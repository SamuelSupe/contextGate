package server

import (
	"encoding/json"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/version"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (s *Server) agents(w http.ResponseWriter, r *http.Request) {
	a, e := s.Store.Agents()
	if e != nil {
		fail(w, 500, e)
		return
	}
	activity, e := s.Store.AgentActivity()
	if e != nil {
		fail(w, 500, e)
		return
	}
	out := []struct {
		model.Agent
		Activity store.Activity `json:"activity"`
	}{}
	for _, agent := range a {
		out = append(out, struct {
			model.Agent
			Activity store.Activity `json:"activity"`
		}{agent, activity[agent.ID]})
	}
	write(w, 200, out)
}
func (s *Server) saveAgent(w http.ResponseWriter, r *http.Request) {
	var a model.Agent
	if e := decode(r, &a); e != nil {
		fail(w, 400, model.Fail("invalid_input", e.Error()))
		return
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Sources == nil {
		a.Sources = []string{}
	}
	if len(a.Name) == 0 || len(a.Name) > 120 {
		fail(w, 400, model.Fail("invalid_input", "Agent name must contain 1-120 bytes"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	id := r.PathValue("id")
	token := ""
	var old model.Agent
	if id != "" {
		var e error
		old, e = s.Store.Agent(id)
		if e != nil {
			fail(w, 404, model.Fail("not_found", "Agent not found"))
			return
		}
		if a.Revision != old.Revision {
			fail(w, 409, model.Fail("conflict", "Agent changed. Reload its current grants before saving."))
			return
		}
		a.ID = old.ID
		a.AuthType = old.AuthType
		a.CreatedAt = old.CreatedAt
		a.RevokedAt = old.RevokedAt
		a.ClientID = old.ClientID
		a.Revision = old.Revision + 1
		if old.RevokedAt != nil && a.Enabled {
			fail(w, 409, model.Fail("credential_revoked", "This credential is permanently revoked. Issue a new token or authorize OAuth again."))
			return
		}
	} else {
		a.ID = "agent_" + secure.Random(16)
		a.AuthType = "token"
		a.RevokedAt = nil
		a.CreatedAt = time.Now()
		a.Revision = 1
		a.ClientID = ""
		token = "hub_" + secure.Random(32)
	}
	if a.ExpiresAt.IsZero() {
		a.ExpiresAt = time.Now().Add(90 * 24 * time.Hour)
	}
	if !a.ExpiresAt.Equal(old.ExpiresAt) && (!a.ExpiresAt.After(time.Now()) || a.ExpiresAt.After(time.Now().Add(366*24*time.Hour))) {
		fail(w, 400, model.Fail("invalid_input", "expiration must be in the next 366 days"))
		return
	}
	for _, id := range a.Sources {
		if _, e := s.Store.Source(id); e != nil {
			fail(w, 400, model.Fail("invalid_input", "unknown data source"))
			return
		}
	}
	if e := s.Store.SaveAgent(a, token); e != nil {
		fail(w, 500, e)
		return
	}
	if a.Enabled != old.Enabled || !a.ExpiresAt.Equal(old.ExpiresAt) || !slices.Equal(a.Sources, old.Sources) {
		s.Engine.InvalidateAgent(a.ID)
	}
	out := map[string]any{"agent": a}
	if token != "" {
		out["token"] = token
	}
	write(w, 200, out)
}
func (s *Server) revokeAgent(w http.ResponseWriter, r *http.Request) {
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	a, e := s.Store.Agent(r.PathValue("id"))
	if e != nil {
		fail(w, 404, model.Fail("not_found", "Agent not found"))
		return
	}
	a.Enabled = false
	now := time.Now()
	a.RevokedAt = &now
	a.Revision++
	b, e := json.Marshal(a)
	if e == nil {
		_, e = s.Store.DB.Exec("UPDATE agents SET value=$1,token_hash=NULL WHERE id=$2", string(b), a.ID)
	}
	if e != nil {
		fail(w, 500, e)
		return
	}
	s.Engine.InvalidateAgent(a.ID)
	write(w, 200, map[string]any{"ok": true})
}
func (s *Server) rotateAgent(w http.ResponseWriter, r *http.Request) {
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	a, e := s.Store.Agent(r.PathValue("id"))
	if e != nil {
		fail(w, 404, model.Fail("not_found", "Agent not found"))
		return
	}
	if a.AuthType != "token" {
		fail(w, 400, model.Fail("invalid_input", "OAuth clients must authorize again to obtain new credentials."))
		return
	}
	if !a.ExpiresAt.After(time.Now()) {
		fail(w, 400, model.Fail("invalid_input", "Extend the Agent expiration before issuing a new token."))
		return
	}
	if a.RevokedAt != nil {
		a.Enabled = true
	}
	a.RevokedAt = nil
	a.Revision++
	token := "hub_" + secure.Random(32)
	if e = s.Store.SaveAgent(a, token); e != nil {
		fail(w, 500, e)
		return
	}
	s.Engine.InvalidateAgent(a.ID)
	write(w, 200, map[string]any{"agent": a, "token": token})
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	before, e := strconv.ParseInt(q.Get("before"), 10, 64)
	if q.Get("before") != "" && (e != nil || before < 1) {
		fail(w, 400, model.Fail("invalid_input", "Invalid audit cursor"))
		return
	}
	f := store.AuditFilter{Administrator: q.Get("administrator_id"), ConfigurationAgent: q.Get("configuration_agent_id"), Channel: q.Get("channel"), ExcludeSecurity: model.AdministratorPrincipal(r.Context()).AdministratorRole != model.RoleSuperAdministrator, Before: before, View: q.Get("view"), EventKind: q.Get("event_kind"), Agent: q.Get("agent_id"), Source: q.Get("source_id"), Status: q.Get("status"), RequestID: q.Get("request_id")}
	if f.EventKind != "" && f.EventKind != "query" && f.EventKind != "management" && f.EventKind != "system" && f.EventKind != "security" {
		fail(w, 400, model.Fail("invalid_input", "Invalid audit event kind"))
		return
	}
	if f.EventKind == "security" && f.ExcludeSecurity {
		fail(w, 403, model.Fail("forbidden", "Security audit requires super administrator access"))
		return
	}
	if f.View != "" && f.View != "client" && f.View != "preview" && f.View != "system" {
		fail(w, 400, model.Fail("invalid_input", "Invalid audit view"))
		return
	}
	if f.Status != "" && f.Status != "success" && f.Status != "error" {
		fail(w, 400, model.Fail("invalid_input", "Invalid audit status"))
		return
	}
	for key, target := range map[string]*time.Time{"from": &f.From, "until": &f.Until} {
		if v := q.Get(key); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				fail(w, 400, model.Fail("invalid_input", "Audit dates must be RFC3339 timestamps"))
				return
			}
			*target = t
		}
	}
	a, e := s.Store.Audits(f, 100)
	if e != nil {
		fail(w, 500, e)
		return
	}
	write(w, 200, a)
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"version": version.Version, "commit": version.BuildCommit(), "metadata_storage": "postgresql", "public_url": s.PublicURL, "mcp_url": s.PublicURL + "/mcp", "database_directory": s.FileRoot, "audit_retention_days": 30, "default_timeout_seconds": 30, "default_max_rows": 1000, "default_max_bytes": 5 << 20, "global_concurrency": 32, "agent_concurrency": 4, "oauth_access_token_minutes": 15, "oauth_refresh_token_days": 30})
}
