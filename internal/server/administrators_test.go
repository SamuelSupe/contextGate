package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/store"
)

func administratorBrowser(t *testing.T, h *hubTest, username, password string) *hubTest {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	browser := &hubTest{t: t, s: h.s, http: h.http, dir: h.dir, client: &http.Client{Jar: jar}}
	response := browser.json("POST", "/api/login", map[string]any{"username": username, "password": password}, 200)
	browser.csrf = response["csrf"].(string)
	return browser
}

// Exercise the HTTP/MCP boundaries together: shared configuration does not imply
// permission to administer accounts, impersonate another owner or read security events.
func TestAdministratorsRolesPersonalTokensAndAudit(t *testing.T) {
	h := newHub(t)
	sourceID := h.source()
	owner, _ := h.s.Store.AdministratorByUsername("admin")
	created := h.json("POST", "/api/administrators", map[string]any{"username": "Analyst", "display_name": "分析管理员", "role": "admin"}, 200)
	adminID := created["id"].(string)
	temporary := created["temporary_password"].(string)
	creationAudit, err := h.s.Store.Audits(store.AuditFilter{Administrator: owner.ID, EventKind: "security"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	recorded := false
	for _, event := range creationAudit {
		if event.Operation == "administrator.create" && event.ResourceID == adminID && event.Revision == "1" {
			recorded = true
		}
	}
	if !recorded {
		t.Fatal("administrator creation audit lost its committed revision")
	}

	h.json("POST", "/api/administrators", map[string]any{"username": "ANALYST", "display_name": "Duplicate", "role": "admin"}, 409)
	reader := administratorBrowser(t, h, "ANALYST", temporary)
	reader.json("GET", "/api/sources", nil, 403)
	reader.json("POST", "/api/configuration-agents", map[string]any{}, 403)
	changed := reader.json("POST", "/api/password", map[string]any{"current_password": temporary, "password": "personal-password-123"}, 200)
	if changed["administrator"].(map[string]any)["must_change_password"] != false {
		t.Fatal("first password did not unlock account")
	}
	reader.json("GET", "/api/sources", nil, 200)
	for _, path := range []string{"/api/administrators", "/api/settings/diagnostics", "/api/settings/health", "/api/settings/audit-export", "/api/audit?event_kind=security"} {
		reader.json("GET", path, nil, 403)
	}
	for _, path := range []string{"/api/administrators", "/api/settings/health", "/api/settings/audit-export"} {
		method := "PUT"
		if path == "/api/administrators" {
			method = "POST"
		}
		reader.json(method, path, map[string]any{}, 403)
	}
	own := reader.json("POST", "/api/configuration-agents", map[string]any{}, 200)
	ownID, ownToken := own["id"].(string), own["token"].(string)
	other := h.json("POST", "/api/configuration-agents", map[string]any{}, 200)
	otherID := other["id"].(string)
	reader.json("POST", "/api/configuration-agents/"+otherID+"/token", map[string]any{"revision": "2"}, 403)
	reader.json("DELETE", "/api/configuration-agents/"+otherID, nil, 403)
	h.json("POST", "/api/configuration-agents/"+ownID+"/token", map[string]any{"revision": "2"}, 403)
	listed := reader.json("GET", "/api/configuration-agents", nil, 200)["agents"].([]any)
	if len(listed) != 1 || listed[0].(map[string]any)["id"] != ownID {
		t.Fatal("other administrator identities leaked")
	}
	queryAgent := h.json("POST", "/api/agents", map[string]any{"name": "reader", "sources": []string{sourceID}, "enabled": true}, 200)
	effectiveID := queryAgent["agent"].(map[string]any)["id"].(string)
	reader.json("POST", "/api/query", map[string]any{"operation": "query_sql", "agent_id": effectiveID, "source_id": sourceID, "query": map[string]any{"query": "SELECT 1"}}, 200)
	cfg := configurationClient(t, h, ownToken)
	definition := eventOntology()
	configurationValue(t, cfg, "create_ontology", map[string]any{"id": "attributed", "definition": definition}, false)
	updated := reader.json("PUT", "/api/ontologies/attributed", map[string]any{"revision": "1", "definition": definition}, 200)
	if updated["revision"] != "2" {
		t.Fatal("UI draft edit failed")
	}
	audits, err := h.s.Store.Audits(store.AuditFilter{Administrator: adminID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	preview, ui, mcp := false, false, false
	for _, a := range audits {
		if a.AdministratorUsername != "analyst" {
			t.Fatal("missing verified username")
		}
		if a.Operation == "query_sql" {
			preview = a.AgentID == effectiveID && a.Preview && a.Channel == "ui"
		}
		if a.Operation == "configuration.create_ontology" {
			mcp = a.ConfigurationAgentID == ownID && a.Channel == "configuration_mcp" && a.ActorType == "configuration_agent"
		}
		if strings.HasPrefix(a.Operation, "ontology.put") {
			ui = a.Channel == "ui" && a.ActorType == "administrator"
		}
	}
	if !preview || !ui || !mcp {
		t.Fatalf("actor attribution missing: preview=%v ui=%v mcp=%v", preview, ui, mcp)
	}
	response := reader.req("GET", "/api/audit?administrator_id="+owner.ID, nil, "application/json", 200)
	var public []model.Audit
	if err := json.NewDecoder(response.Body).Decode(&public); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(public) == 0 {
		t.Fatal("ordinary administrator cannot read business audit")
	}
	raw, _ := json.Marshal(public)
	for _, event := range public {
		if event.EventKind == "security" {
			t.Fatal("security events leaked via ordinary audit filter")
		}
	}
	for _, secret := range []string{temporary, ownToken, other["token"].(string), "personal-password-123"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("audit leaked credential")
		}
	}
	identity, _ := h.s.Store.ConfigurationAgent(ownID)
	job, finish, err := h.s.startConfigurationCall(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	rotated := reader.json("POST", "/api/configuration-agents/"+ownID+"/token", map[string]any{"revision": strconv.FormatInt(identity.Revision, 10)}, 200)
	if rotated["id"] != ownID || job.Err() == nil {
		t.Fatal("rotation changed identity or failed to cancel existing work")
	}
	if _, err = h.s.Store.ConfigurationToken(ownToken); err == nil {
		t.Fatal("rotated token still valid")
	}
	if _, _, err = h.s.startConfigurationCall(context.Background(), identity); model.ErrorCode(err) != "unauthorized" {
		t.Fatal("captured MCP identity survived rotation", err)
	}
	reader.json("POST", "/api/configuration-agents/"+ownID+"/token", map[string]any{"revision": strconv.FormatInt(identity.Revision, 10)}, 409)
	token := rotated["token"].(string)
	secondBrowser := administratorBrowser(t, h, "analyst", "personal-password-123")
	reader.json("POST", "/api/password", map[string]any{"current_password": "personal-password-123", "password": "new-personal-password-456"}, 200)
	secondBrowser.json("GET", "/api/sources", nil, 401)
	reader.json("GET", "/api/sources", nil, 200)
	h.json("GET", "/api/sources", nil, 200)
	if _, err = h.s.Store.ConfigurationToken(token); err != nil {
		t.Fatal("personal password change revoked configuration token", err)
	}
	account, _ := h.s.Store.Administrator(adminID)
	disabled := h.json("PUT", "/api/administrators/"+adminID, map[string]any{"display_name": account.DisplayName, "role": account.Role, "enabled": false, "revision": strconv.FormatInt(account.Revision, 10)}, 200)
	reader.json("GET", "/api/sources", nil, 401)
	if _, err = h.s.Store.ConfigurationToken(token); err == nil {
		t.Fatal("disabled account token survived")
	}
	h.json("PUT", "/api/administrators/"+adminID, map[string]any{"display_name": account.DisplayName, "role": account.Role, "enabled": true, "revision": disabled["revision"]}, 200)
	if _, err = h.s.Store.ConfigurationToken(token); err == nil {
		t.Fatal("reenabling restored old token")
	}
	reader = administratorBrowser(t, h, "analyst", "new-personal-password-456")
	issued := reader.json("POST", "/api/configuration-agents", map[string]any{}, 200)
	identity, _ = h.s.Store.ConfigurationAgent(ownID)
	job, finishRole, err := h.s.startConfigurationCall(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer finishRole()
	account, _ = h.s.Store.Administrator(adminID)
	h.json("PUT", "/api/administrators/"+adminID, map[string]any{"display_name": account.DisplayName, "role": "super_admin", "enabled": true, "revision": strconv.FormatInt(account.Revision, 10)}, 200)
	reader.json("GET", "/api/sources", nil, 401)
	if job.Err() == nil {
		t.Fatal("role change did not cancel configuration work")
	}
	if _, err = h.s.Store.ConfigurationToken(issued["token"].(string)); err == nil {
		t.Fatal("role change retained configuration token")
	}
	reader = administratorBrowser(t, h, "analyst", "new-personal-password-456")
	reader.json("GET", "/api/administrators", nil, 200)
	h.json("GET", "/api/sources", nil, 200)
	call(t, h.mcp(queryAgent["token"].(string)), "query_sql", map[string]any{"source_id": sourceID, "query": "SELECT 1"}, false)
	h.json("PUT", "/api/administrators/"+owner.ID, map[string]any{"display_name": owner.DisplayName, "role": "admin", "enabled": true, "revision": strconv.FormatInt(owner.Revision, 10)}, 403)
}

func TestAdministratorRevocationRejectsCapturedUIContext(t *testing.T) {
	h := newHub(t)
	created := h.json("POST", "/api/administrators", map[string]any{"username": "operator", "display_name": "Operator", "role": "admin"}, 200)
	browser := administratorBrowser(t, h, "operator", created["temporary_password"].(string))
	browser.json("POST", "/api/password", map[string]any{"current_password": created["temporary_password"], "password": "operator-password-123"}, 200)
	req, _ := http.NewRequest("GET", h.http.URL+"/api/sources", nil)
	for _, cookie := range browser.client.Jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}
	account, _, ok := h.s.adminSession(req)
	if !ok {
		t.Fatal("missing session")
	}
	captured, finish := h.s.administratorContext(req, account)
	defer finish()
	id := created["id"].(string)
	h.json("POST", "/api/administrators/"+id+"/reset-password", map[string]any{"revision": strconv.FormatInt(account.Revision, 10)}, 200)
	if captured.Context().Err() == nil || model.CheckConfigurationContext(captured.Context()) == nil {
		t.Fatal("reset did not invalidate in-flight UI work")
	}
	if _, err := h.s.Store.CreateAdministrator(captured.Context(), "forbidden", "Forbidden", "admin", "hash"); err == nil {
		t.Fatal("revoked request committed a change")
	}
	current, _ := h.s.Store.Administrator(id)
	if !current.MustChangePassword || current.TemporaryExpiresAt == nil || time.Until(*current.TemporaryExpiresAt) > 24*time.Hour {
		t.Fatal("password reset did not require a bounded temporary password")
	}
	h.s.Store.DB.Exec("UPDATE administrators SET temporary_expires_at=$1 WHERE id=$2", time.Now().Add(-time.Minute), id)
	if current, err := h.s.Store.Administrator(id); err != nil || current.Active() {
		t.Fatal("expired temporary account is active")
	}
}
