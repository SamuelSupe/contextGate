package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/adapter"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/testpg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuditExportAdminBoundaryAndMCPRedaction(t *testing.T) {
	h := newHub(t)
	id := h.source()
	agent := h.json("POST", "/api/agents", map[string]any{"name": "Audit reader", "sources": []string{id}, "enabled": true}, 200)
	token := agent["token"].(string)
	semanticPath := "/api/sources/" + id + "/semantics"
	savedDraft := h.json("PUT", semanticPath, semanticInput{Snapshot: semantic.Snapshot{FormatVersion: 1, Entries: []semantic.Entry{semanticFixture()}}}, 200)
	h.json("POST", semanticPath+"/trial", map[string]any{"revision": savedDraft["revision"], "template_id": "amount"}, 200)
	h.json("POST", semanticPath+"/publish", map[string]any{"revision": savedDraft["revision"]}, 200)
	path := "/api/settings/audit-export"
	view := h.json("GET", path, nil, 200)
	config := view["config"].(map[string]any)
	if config["enabled"] != false {
		t.Fatal("export enabled by default")
	}
	for _, route := range []struct{ method, path string }{{"GET", path}, {"PUT", path}, {"POST", path + "/test"}} {
		req, _ := http.NewRequest(route.method, h.http.URL+route.path, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatal("Agent accessed export administration", res.StatusCode)
		}
	}
	csrf := h.csrf
	h.csrf = "invalid"
	h.json("PUT", path, config, 403)
	h.csrf = csrf

	for _, patch := range []map[string]any{
		{"endpoint": "https://user:secret@example.com/v1/logs"},
		{"endpoint": "https://example.com/v1/logs?token=secret"},
		{"endpoint": "file:///tmp/logs"},
		{"endpoint": "http://localhost:4317/v1/logs", "protocol": "grpc"},
		{"headers": map[string]string{"Authorization": "Bearer secret\r\nHost: elsewhere"}},
		{"headers": map[string]string{"Host": "elsewhere"}},
	} {
		input := map[string]any{"revision": config["revision"]}
		for key, value := range patch {
			input[key] = value
		}
		h.json("PUT", path, input, 400)
	}
	configuration := h.json("POST", "/api/configuration-agents", map[string]any{"name": "Audited configuration"}, 200)
	configurationID := configuration["id"].(string)
	received := make(chan *collectorpb.ExportLogsServiceRequest, 16)
	auth := make(chan string, 16)
	testAuth := make(chan string, 16)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		for _, secret := range []string{"secret-query-text", "db-secret-never-echo", "export-secret", token, configuration["token"].(string), "SELECT note", "DELETE FROM"} {
			if bytes.Contains(body, []byte(secret)) {
				t.Error("sensitive data in OTLP payload")
			}
		}
		request := new(collectorpb.ExportLogsServiceRequest)
		if err := proto.Unmarshal(body, request); err != nil {
			t.Error(err)
		}
		synthetic := false
		for _, resource := range request.ResourceLogs {
			for _, scope := range resource.ScopeLogs {
				for _, record := range scope.LogRecords {
					for _, attr := range record.Attributes {
						if attr.Key == "event.name" && attr.Value.GetStringValue() == "mcpdbhub.audit_export.test" {
							synthetic = true
						}
					}
				}
			}
		}
		if synthetic {
			testAuth <- r.Header.Get("Authorization")
		} else {
			received <- request
			auth <- r.Header.Get("Authorization")
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer receiver.Close()
	config["enabled"], config["endpoint"] = true, receiver.URL
	config["headers"] = map[string]string{"Authorization": "Bearer export-secret"}
	view = h.json("PUT", path, config, 200)
	encoded, _ := json.Marshal(view)
	if bytes.Contains(encoded, []byte("export-secret")) || view["headers_configured"] != true {
		t.Fatal("stored headers exposed or missing")
	}
	h.json("PUT", path, config, 409)
	config = view["config"].(map[string]any)

	session := h.mcp(token)
	call(t, session, "query_sql", map[string]any{"source_id": id, "query": "SELECT note FROM events WHERE note = ?", "params": []string{"secret-query-text"}}, false)
	call(t, session, "query_sql", map[string]any{"source_id": id, "query": "DELETE FROM events"}, true)
	call(t, session, "execute_query_template", semantic.Execution{SourceID: id, TemplateID: "amount", ExecutionVersion: "1", Parameters: map[string]any{"id": 1}}, false)
	configurationDraft, _ := h.s.Store.Semantics(id)
	configurationValue(t, configurationClient(t, h, configuration["token"].(string)), "trial_query_template", map[string]any{"source_id": id, "revision": strconv.FormatInt(configurationDraft.Revision, 10), "template_id": "amount"}, false)
	configurationManagementSeen, configurationTrialSeen := false, false
	templateSeen := false
	seen := map[logspb.SeverityNumber]bool{}
	deadline := time.After(8 * time.Second)
	for len(seen) < 2 || !templateSeen || !configurationManagementSeen || !configurationTrialSeen {
		select {
		case request := <-received:
			if <-auth != "Bearer export-secret" {
				t.Fatal("export authentication missing")
			}
			for _, record := range request.ResourceLogs[0].ScopeLogs[0].LogRecords {
				attrs := map[string]string{}
				for _, a := range record.Attributes {
					attrs[a.Key] = a.Value.GetStringValue()
				}
				if attrs["mcpdbhub.audit.agent_id"] == configurationID {
					if attrs["mcpdbhub.audit.operation"] == "configuration.trial_query_template" {
						configurationManagementSeen = true
					}
					if attrs["mcpdbhub.audit.operation"] == "query_sql" && attrs["mcpdbhub.audit.template_id"] == "amount" {
						configurationTrialSeen = true
					}
				}
				if attrs["mcpdbhub.audit.event_kind"] == "management" {
					if attrs["mcpdbhub.audit.request_id"] == "" {
						t.Fatal("management audit request ID missing")
					}
					continue
				}
				seen[record.SeverityNumber] = true
				if attrs["mcpdbhub.audit.operation"] == "execute_query_template" {
					if attrs["mcpdbhub.audit.template_id"] != "amount" || attrs["mcpdbhub.audit.template_version"] != "1" {
						t.Fatal("OTLP template correlation missing")
					}
					templateSeen = true
				}
				if attrs["mcpdbhub.audit.source_id"] != id || attrs["mcpdbhub.audit.request_id"] == "" || attrs["mcpdbhub.audit.query_fingerprint"] == "" {
					t.Fatal("audit correlation fields missing")
				}
			}
		case <-deadline:
			t.Fatal("MCP success/failure audit export missing")
		}
	}
	if !seen[logspb.SeverityNumber_SEVERITY_NUMBER_INFO] || !seen[logspb.SeverityNumber_SEVERITY_NUMBER_ERROR] {
		t.Fatal("incorrect severity")
	}
	// Empty credentials preserve the stored headers until the administrator explicitly clears them.
	view = h.json("PUT", path, config, 200)
	config = view["config"].(map[string]any)
	if h.json("POST", path+"/test", config, 200)["accepted"] != true || <-testAuth != "Bearer export-secret" {
		t.Fatal("stored headers were not reused for test")
	}
	config["clear_headers"] = true
	view = h.json("PUT", path, config, 200)
	if view["headers_configured"] != false {
		t.Fatal("headers not cleared")
	}
	if h.json("POST", path+"/test", view["config"], 200)["accepted"] != true || <-testAuth != "" {
		t.Fatal("cleared headers were sent")
	}
}

// Run the same real fixtures through the public Agent protocol, so adapter-only
// success cannot hide schema, authorization, encoding or cursor wrapper defects.
func TestDatabaseMCPMatrix(t *testing.T) {
	path := os.Getenv("MCPDBHUB_MATRIX")
	if path == "" {
		t.Skip("set MCPDBHUB_MATRIX to a fixture manifest")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name      string         `json:"name"`
		Source    model.Source   `json:"source"`
		Namespace string         `json:"namespace"`
		Object    string         `json:"object"`
		Baseline  map[string]any `json:"baseline"`
		Queries   []struct {
			Query     map[string]any  `json:"query"`
			Rows      int             `json:"rows"`
			Error     bool            `json:"error"`
			Contains  string          `json:"contains"`
			Data      json.RawMessage `json:"data"`
			Truncated *bool           `json:"truncated"`
		} `json:"queries"`
		Denied []map[string]any `json:"denied"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err = d.Decode(&fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			h := newHub(t)
			f.Source.Name, f.Source.Enabled = f.Name, true
			saved := h.json("POST", "/api/sources", f.Source, 200)
			id := saved["id"].(string)
			if saved["password"] != nil || saved["token"] != nil {
				t.Fatal("source credential leaked")
			}
			h.json("POST", "/api/sources/"+id+"/test", nil, 200)
			a := h.json("POST", "/api/agents", map[string]any{"name": "Fixture reader", "sources": []string{id}, "enabled": true}, 200)
			s := h.mcp(a["token"].(string))
			capability, _ := adapter.Get(f.Source.Kind)
			call(t, s, "list_data_sources", map[string]any{}, false)
			matrixOntology(t, h, id, f.Namespace, f.Object)
			for _, tool := range []string{"list_namespaces", "list_objects", "describe_object"} {
				args := map[string]any{"source_id": id, "namespace": f.Namespace}
				if tool == "describe_object" {
					args["object"] = f.Object
				}
				call(t, s, tool, args, false)
			}
			read := func(q map[string]any, denied bool) *model.Result {
				q["source_id"] = id
				r := call(t, s, capability.Tool, q, denied)
				if denied {
					return nil
				}
				var result model.Result
				wire, _ := json.Marshal(r.StructuredContent)
				d := json.NewDecoder(bytes.NewReader(wire))
				d.UseNumber()
				if err := d.Decode(&result); err != nil {
					t.Fatal(err)
				}
				return &result
			}
			before, _ := json.Marshal(read(f.Baseline, false).Data)
			for i, q := range f.Queries {
				r := read(q.Query, q.Error)
				matrixTemplate(t, h, s, id, capability.Tool, q.Query, r, q.Error)
				if q.Error {
					continue
				}
				wire, _ := json.Marshal(r)
				data, _ := json.Marshal(r.Data)
				if r.RowCount != q.Rows || q.Truncated != nil && r.Truncated != *q.Truncated || q.Contains != "" && !bytes.Contains(wire, []byte(q.Contains)) {
					t.Fatalf("query %d unexpected result: %s", i, wire)
				}
				if len(q.Data) > 0 {
					var expected bytes.Buffer
					json.Compact(&expected, q.Data)
					if string(data) != expected.String() {
						t.Fatalf("query %d data=%s want=%s", i, data, expected.Bytes())
					}
				}
			}
			for _, q := range f.Denied {
				read(q, true)
				matrixTemplate(t, h, s, id, capability.Tool, q, nil, true)
			}
			after, _ := json.Marshal(read(f.Baseline, false).Data)
			if string(before) != string(after) {
				t.Fatal("target data changed through MCP")
			}
			f.Baseline["max_rows"] = 1
			r := read(f.Baseline, false)
			all := append([]any{}, r.Data...)
			for pages := 0; r.NextCursor != ""; pages++ {
				if pages > 100 {
					t.Fatal("MCP cursor did not terminate")
				}
				f.Baseline["cursor"] = r.NextCursor
				r = read(f.Baseline, false)
				all = append(all, r.Data...)
			}
			if capability.Family == "mongodb" || capability.Family == "cql" || capability.Family == "search" {
				wire, _ := json.Marshal(all)
				if string(wire) != string(before) {
					t.Fatal("MCP pagination lost or repeated rows")
				}
			}
			empty := h.json("POST", "/api/agents", map[string]any{"name": "No grants", "sources": []string{}, "enabled": true}, 200)
			other := h.mcp(empty["token"].(string))
			call(t, other, "list_objects", map[string]any{"source_id": id}, true)
			call(t, other, capability.Tool, f.Baseline, true)
			var auditCount int
			if err := h.s.Store.DB.QueryRow("SELECT count(*) FROM audit WHERE source_id=$1", id).Scan(&auditCount); err != nil || auditCount < len(f.Queries) {
				t.Fatal("MCP audit missing", err)
			}
			agentID := a["agent"].(map[string]any)["id"].(string)
			h.json("DELETE", "/api/agents/"+agentID, nil, 200)
			if _, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_data_sources", Arguments: map[string]any{}}); err == nil {
				t.Fatal("revoked token reached MCP")
			}
		})
	}
}

func TestDuckDBMCPReadOnly(t *testing.T) {
	h := newHub(t)
	path := filepath.Join(h.dir, "fixture.duckdb")
	db, err := sql.Open("duckdb", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("CREATE TABLE events(id BIGINT); INSERT INTO events VALUES(1),(2),(3)")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	source := h.json("POST", "/api/sources", map[string]any{"name": "DuckDB fixture", "kind": "duckdb", "path": path, "enabled": true}, 200)
	id := source["id"].(string)
	a := h.json("POST", "/api/agents", map[string]any{"name": "DuckDB reader", "sources": []string{id}, "enabled": true}, 200)
	s := h.mcp(a["token"].(string))
	localTemplateMatrix(t, h, s, id)
	call(t, s, "describe_object", map[string]any{"source_id": id, "namespace": "main", "object": "events"}, false)
	r := call(t, s, "query_sql", map[string]any{"source_id": id, "query": "SELECT ?::BIGINT AS large_value", "params": []any{int64(9007199254740993)}}, false)
	wire, _ := json.Marshal(r.StructuredContent)
	if !bytes.Contains(wire, []byte(`"9007199254740993"`)) {
		t.Fatal("DuckDB MCP integer precision lost")
	}
	for _, q := range []string{"DELETE FROM events", "SELECT 1; DELETE FROM events", "SELECT * FROM read_csv('/etc/passwd')", "ATTACH '/tmp/another.db' AS other"} {
		call(t, s, "query_sql", map[string]any{"source_id": id, "query": q}, true)
	}
	r = call(t, s, "query_sql", map[string]any{"source_id": id, "query": "SELECT * FROM events", "max_rows": 1}, false)
	wire, _ = json.Marshal(r.StructuredContent)
	if !bytes.Contains(wire, []byte(`"truncated":true`)) {
		t.Fatal("DuckDB MCP truncation missing")
	}
	r = call(t, s, "query_sql", map[string]any{"source_id": id, "query": "SELECT count(*) FROM events"}, false)
	wire, _ = json.Marshal(r.StructuredContent)
	if !bytes.Contains(wire, []byte(`"data":[["3"]]`)) {
		t.Fatal("DuckDB data changed")
	}
}

type hubTest struct {
	t      *testing.T
	s      *Server
	http   *httptest.Server
	client *http.Client
	csrf   string
	dir    string
}

func TestAdminPasswordChangeRollsBackIfSessionRevocationFails(t *testing.T) {
	h := newHub(t)
	previous, err := h.s.Store.Get("admin_password")
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce a storage failure between updating the password and revoking
	// sessions. The security change must either commit completely or roll back.
	if _, err = h.s.Store.DB.Exec(`CREATE FUNCTION fail_revocation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'simulated revocation failure'; END $$; CREATE TRIGGER fail_session_revocation BEFORE DELETE ON sessions FOR EACH STATEMENT EXECUTE FUNCTION fail_revocation()`); err != nil {
		t.Fatal(err)
	}
	h.json("POST", "/api/password", map[string]any{"current_password": "test-password-123456", "password": "replacement-password-123456"}, 500)
	current, err := h.s.Store.Get("admin_password")
	if err != nil || current != previous {
		t.Fatal("failed password change left a new password with unrevoked sessions", err)
	}
	if h.json("GET", "/api/session", nil, 200)["authenticated"] != true {
		t.Fatal("rollback did not retain the existing session")
	}
}

func TestAdminPasswordChangeRejectsStaleAuthentication(t *testing.T) {
	h := newHub(t)
	source := h.source()
	agent := h.json("POST", "/api/agents", map[string]any{"name": "Retained reader", "sources": []string{source}, "enabled": true}, 200)
	oldHash, err := h.s.Store.Get("admin_password")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(h.http.URL)
	oldCookies := h.client.Jar.Cookies(u)
	changed := h.json("POST", "/api/password", map[string]any{"current_password": "test-password-123456", "password": "replacement-password-123456"}, 200)
	h.csrf = changed["csrf"].(string)
	for _, cookie := range oldCookies {
		if cookie.Name == "hub_session" {
			if _, err := h.s.Store.CheckSession(cookie.Value); err == nil {
				t.Fatal("old administrator session survived password change")
			}
		}
	}
	if h.json("GET", "/api/session", nil, 200)["authenticated"] != true {
		t.Fatal("password change did not issue a valid replacement session")
	}

	// Pause at the verification/issuance boundary deterministically: this login
	// verified the former hash before the concurrent password change committed.
	w := httptest.NewRecorder()
	h.s.createSession(w, httptest.NewRequest("POST", "/api/login", nil), oldHash)
	if w.Code != http.StatusUnauthorized || w.Header().Get("Set-Cookie") != "" {
		t.Fatal("stale login issued an administrator session", w.Code)
	}
	currentHash, err := h.s.Store.Get("admin_password")
	if err != nil {
		t.Fatal(err)
	}
	if err = h.s.Store.ChangeAdminPassword(oldHash, oldHash); model.ErrorCode(err) != "conflict" {
		t.Fatal("stale password edit was not rejected", err)
	}
	fresh, err := h.s.Store.Get("admin_password")
	if err != nil || fresh != currentHash || h.json("GET", "/api/session", nil, 200)["authenticated"] != true {
		t.Fatal("stale password edit modified current credentials or sessions", err)
	}
	h.json("POST", "/api/login", map[string]any{"password": "test-password-123456"}, 401)
	login := h.json("POST", "/api/login", map[string]any{"password": "replacement-password-123456"}, 200)
	h.csrf = login["csrf"].(string)
	if _, err := h.s.Store.TokenAgent(agent["token"].(string)); err != nil {
		t.Fatal("password change affected the Agent credential", err)
	}
	if stored, err := h.s.Store.Source(source); err != nil || stored.Password != "db-secret-never-echo" {
		t.Fatal("password change affected database credentials", err)
	}
}

func newHub(t *testing.T) *hubTest {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "config"), testpg.DSN(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	base := "http://" + ts.Listener.Addr().String()
	s, err := New(st, base, dir)
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s.Handler()
	ts.Start()
	jar, _ := cookiejar.New(nil)
	h := &hubTest{t: t, s: s, http: ts, client: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, dir: dir}
	t.Cleanup(func() { ts.Close(); s.Close(); st.DB.Close() })
	code, err := s.EnsureSetup()
	if err != nil {
		t.Fatal(err)
	}
	v := h.json("POST", "/api/setup", map[string]any{"token": code, "password": "test-password-123456"}, 200)
	h.csrf = v["csrf"].(string)
	return h
}
func (h *hubTest) req(method, path string, body io.Reader, typ string, want int) *http.Response {
	h.t.Helper()
	r, _ := http.NewRequest(method, h.http.URL+path, body)
	r.Header.Set("Content-Type", typ)
	r.Header.Set("X-CSRF-Token", h.csrf)
	res, e := h.client.Do(r)
	if e != nil {
		h.t.Fatal(e)
	}
	if res.StatusCode != want {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		h.t.Fatalf("%s %s: status %d want %d: %s", method, path, res.StatusCode, want, b)
	}
	return res
}
func (h *hubTest) json(method, path string, v any, want int) map[string]any {
	h.t.Helper()
	b, _ := json.Marshal(v)
	res := h.req(method, path, bytes.NewReader(b), "application/json", want)
	defer res.Body.Close()
	out := map[string]any{}
	json.NewDecoder(res.Body).Decode(&out)
	return out
}
func (h *hubTest) form(path string, v url.Values, want int) map[string]any {
	h.t.Helper()
	res := h.req("POST", path, strings.NewReader(v.Encode()), "application/x-www-form-urlencoded", want)
	defer res.Body.Close()
	out := map[string]any{}
	json.NewDecoder(res.Body).Decode(&out)
	return out
}
func (h *hubTest) source() string {
	h.t.Helper()
	path := filepath.Join(h.dir, "fixture.sqlite")
	db, e := sql.Open("sqlite3", path)
	if e != nil {
		h.t.Fatal(e)
	}
	_, e = db.Exec("CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY,amount INTEGER,note TEXT); DELETE FROM events; INSERT INTO events VALUES(1,9007199254740993,'secret-query-text'),(2,4,'second'),(3,5,'third')")
	db.Close()
	if e != nil {
		h.t.Fatal(e)
	}
	v := h.json("POST", "/api/sources", map[string]any{"name": "SQLite fixture", "kind": "sqlite", "path": path, "enabled": true, "password": "db-secret-never-echo"}, 200)
	if v["password"] != nil {
		h.t.Fatal("credential leaked")
	}
	return v["id"].(string)
}

type testBearer struct{ token string }

func (b testBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}
func (h *hubTest) mcp(token string) *mcp.ClientSession {
	h.t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "1"}, nil)
	s, e := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: h.http.URL + "/mcp", HTTPClient: &http.Client{Transport: testBearer{token}}}, nil)
	if e != nil {
		h.t.Fatal(e)
	}
	h.t.Cleanup(func() { s.Close() })
	return s
}
func call(t *testing.T, s *mcp.ClientSession, tool string, args any, wantError bool) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, e := s.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if e != nil {
		t.Fatal(e)
	}
	if r.IsError != wantError {
		wire, _ := json.Marshal(r)
		t.Fatalf("%s: unexpected result %s", tool, wire)
	}
	return r
}
func TestLifecycleAuthorizationReadOnlyAndPersistence(t *testing.T) {
	h := newHub(t)
	id := h.source()
	h.json("POST", "/api/sources/"+id+"/test", nil, 200)
	h.json("GET", "/api/sources/"+id+"/objects?operation=describe&object=events", nil, 200)
	savedCSRF := h.csrf
	h.csrf = "bad"
	h.json("POST", "/api/agents", map[string]any{}, 403)
	h.csrf = savedCSRF
	a := h.json("POST", "/api/agents", map[string]any{"name": "Reader", "sources": []string{id}, "enabled": true}, 200)
	token := a["token"].(string)
	agent := a["agent"].(map[string]any)
	s := h.mcp(token)
	localTemplateMatrix(t, h, s, id)
	args := map[string]any{"source_id": id, "query": "WITH totals AS (SELECT id, amount FROM events WHERE id >= ?) SELECT id, amount, sum(amount) OVER () FROM totals ORDER BY id", "params": []any{1}}
	r := call(t, s, "query_sql", args, false)
	encoded, _ := json.Marshal(r.StructuredContent)
	if !bytes.Contains(encoded, []byte("9007199254740993")) {
		t.Fatalf("lossless integer missing: %s", encoded)
	}
	for _, q := range []string{"DELETE FROM events", "SELECT 1; DELETE FROM events", "WITH changed AS (DELETE FROM events RETURNING *) SELECT * FROM changed", "SELECT load_extension('bad')", "ATTACH '/tmp/bad' AS other"} {
		denied := call(t, s, "query_sql", map[string]any{"source_id": id, "query": q}, true)
		b, _ := json.Marshal(denied.Content)
		if bytes.Contains(b, []byte("cancelled")) {
			t.Fatal("read-only rejection was incorrectly classified as cancellation")
		}
	}
	call(t, s, "query_sql", map[string]any{"source_id": id, "query": "SELECT * FROM events WHERE id=99"}, false)
	r = call(t, s, "query_sql", map[string]any{"source_id": id, "query": "SELECT * FROM events", "max_rows": 1}, false)
	encoded, _ = json.Marshal(r.StructuredContent)
	if !bytes.Contains(encoded, []byte(`"truncated":true`)) {
		t.Fatalf("not truncated: %s", encoded)
	}
	call(t, s, "list_objects", map[string]any{"source_id": "invisible-source"}, true)
	call(t, s, "query_sql", map[string]any{"source_id": id, "query": "SELECT 1", "host": "injected-host"}, true)
	r = call(t, s, "query_sql", map[string]any{"source_id": id, "query": "SELECT '" + strings.Repeat("x", 20000) + "' AS payload", "max_bytes": 4096}, false)
	wire, _ := json.Marshal(r)
	if len(wire) > 4096 {
		t.Fatalf("MCP response exceeds byte budget: %d", len(wire))
	}

	empty := h.json("POST", "/api/agents", map[string]any{"name": "No grants", "sources": []string{}, "enabled": true}, 200)
	other := h.mcp(empty["token"].(string))
	call(t, other, "list_objects", map[string]any{"source_id": id}, true)
	fresh, e := store.Open(filepath.Join(h.dir, "config"), testpg.DSN(t, h.dir))
	if e != nil {
		t.Fatal(e)
	}
	src, e := fresh.Source(id)
	fresh.DB.Close()
	if e != nil || src.Password != "db-secret-never-echo" {
		t.Fatalf("restart persistence: %v", e)
	}
	var rows int
	if e = h.s.Store.DB.QueryRow("SELECT count(*) FROM audit WHERE agent_id=$1", agent["id"]).Scan(&rows); e != nil || rows < 8 {
		t.Fatalf("audit not persisted: %d %v", rows, e)
	}
	var raw string
	h.s.Store.DB.QueryRow("SELECT value FROM sources WHERE id=$1", id).Scan(&raw)
	if strings.Contains(raw, "db-secret") {
		t.Fatal("plaintext source stored")
	}
	h.json("DELETE", "/api/agents/"+agent["id"].(string), nil, 200)
	if _, e = h.s.principal(context.Background(), token); e == nil {
		t.Fatal("revoked token accepted")
	}
	db, _ := sql.Open("sqlite3", src.Path)
	defer db.Close()
	db.QueryRow("SELECT count(*) FROM events").Scan(&rows)
	if rows != 3 {
		t.Fatal("target database changed")
	}
}

func TestOAuthPKCERotationReplayAndRevocation(t *testing.T) {
	h := newHub(t)
	id := h.source()
	ctx := context.Background()
	redirect := "http://127.0.0.1:41999/callback"
	reg := h.json("POST", "/oauth/register", map[string]any{"client_name": "OAuth reader", "redirect_uris": []string{redirect}, "token_endpoint_auth_method": "none"}, 201)
	clientID := reg["client_id"].(string)
	verifier := strings.Repeat("v", 64)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	authorize := func(allow bool) string {
		t.Helper()
		v := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect}, "resource": {h.http.URL + "/mcp"}, "scope": {"db:read offline_access"}, "state": {"state-long-enough"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
		res := h.req("GET", "/oauth/authorize?"+v.Encode(), nil, "", 303)
		loc, _ := url.Parse(res.Header.Get("Location"))
		res.Body.Close()
		request := loc.Query().Get("request")
		if request == "" {
			t.Fatalf("no consent: %s", loc)
		}
		if allow {
			h.json("POST", "/api/oauth/consent", map[string]any{"request": request, "allow": true, "sources": []string{}}, 400)
		}
		consent := h.json("POST", "/api/oauth/consent", map[string]any{"request": request, "allow": allow, "sources": []string{id}}, 200)
		u, _ := url.Parse(consent["redirect"].(string))
		if u.Query().Get("state") != "state-long-enough" {
			t.Fatal("state lost")
		}
		if !allow {
			if u.Query().Get("error") != "access_denied" {
				t.Fatal("denial missing")
			}
			return ""
		}
		return u.Query().Get("code")
	}
	badRedirect := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {"http://127.0.0.1:41999/another-path"}, "resource": {h.http.URL + "/mcp"}, "scope": {"db:read"}, "state": {"long-enough-state"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	rejected := h.req("GET", "/oauth/authorize?"+badRedirect.Encode(), nil, "", 400)
	rejected.Body.Close()
	authorize(false)
	code := authorize(true)
	v := url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "redirect_uri": {redirect}, "resource": {h.http.URL + "/mcp"}, "code": {code}, "code_verifier": {verifier}}
	wrong := url.Values{}
	for k, values := range v {
		wrong[k] = append([]string{}, values...)
	}
	wrong.Set("resource", "https://different.example/mcp")
	h.form("/oauth/token", wrong, 400)
	wrong.Set("resource", h.http.URL+"/mcp")
	wrong.Set("code_verifier", strings.Repeat("x", 64))
	h.form("/oauth/token", wrong, 400)
	// A failed PKCE exchange may consume the code. Start a fresh authorization.
	v.Set("code", authorize(true))
	tokens := h.form("/oauth/token", v, 200)
	access, ok := tokens["access_token"].(string)
	if !ok {
		t.Fatalf("no access token: %v", tokens)
	}
	call(t, h.mcp(access), "query_sql", map[string]any{"source_id": id, "query": "SELECT count(*) FROM events"}, false)
	semanticPath := "/api/sources/" + id + "/semantics"
	draft := h.json("PUT", semanticPath, map[string]any{"revision": "0", "snapshot": map[string]any{"format_version": 1, "entries": []any{semanticFixture()}}}, 200)
	h.json("POST", semanticPath+"/trial", map[string]any{"revision": draft["revision"], "template_id": "amount"}, 200)
	h.json("POST", semanticPath+"/publish", map[string]any{"revision": draft["revision"]}, 200)
	source, _ := h.s.Store.Source(id)
	view := publicSource(source)
	view.QueryAccessMode = "templates_only"
	h.json("PUT", "/api/sources/"+id, view, 200)
	call(t, h.mcp(access), "query_sql", map[string]any{"source_id": id, "query": "SELECT count(*) FROM events"}, true)
	call(t, h.mcp(access), "search_semantics", map[string]any{"source_id": id}, false)
	call(t, h.mcp(access), "execute_query_template", map[string]any{"source_id": id, "template_id": "amount", "execution_version": "1", "parameters": map[string]any{"id": 1}}, false)
	refresh := tokens["refresh_token"].(string)
	rotated := h.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "resource": {h.http.URL + "/mcp"}, "refresh_token": {refresh}}, 200)
	if rotated["refresh_token"] == refresh {
		t.Fatal("refresh was not rotated")
	}
	h.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "resource": {h.http.URL + "/mcp"}, "refresh_token": {refresh}}, 400)
	if _, e := h.s.OAuth.Principal(ctx, rotated["access_token"].(string)); e == nil {
		t.Fatal("refresh replay did not revoke token family")
	}
	v.Set("code", authorize(true))
	tokens = h.form("/oauth/token", v, 200)
	h.form("/oauth/revoke", url.Values{"client_id": {clientID}, "token": {tokens["refresh_token"].(string)}, "token_type_hint": {"refresh_token"}}, 200)
	if _, e := h.s.OAuth.Principal(ctx, tokens["access_token"].(string)); e == nil {
		t.Fatal("revocation failed")
	}
	v.Set("code", authorize(true))
	tokens = h.form("/oauth/token", v, 200)
	h.form("/oauth/token", v, 400)
	if _, e := h.s.OAuth.Principal(ctx, tokens["access_token"].(string)); e == nil {
		t.Fatal("code replay did not revoke token family")
	}
	v.Set("code", authorize(true))
	expired := h.form("/oauth/token", v, 200)
	if _, e := h.s.Store.DB.Exec("UPDATE oauth SET expires=$1 WHERE kind IN ('access','refresh')", time.Now().Add(-time.Minute).Unix()); e != nil {
		t.Fatal(e)
	}
	if _, e := h.s.OAuth.Principal(ctx, expired["access_token"].(string)); e == nil {
		t.Fatal("expired access token accepted")
	}
	h.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "resource": {h.http.URL + "/mcp"}, "refresh_token": {expired["refresh_token"].(string)}}, 400)
	h.json("POST", "/oauth/register", map[string]any{"client_name": "bad", "redirect_uris": []string{"http://example.com/callback"}}, 400)
}

func TestPauseRevocationAndTokenReplacement(t *testing.T) {
	h := newHub(t)
	sourceID := h.source()
	created := h.json("POST", "/api/agents", map[string]any{"name": "Lifecycle reader", "sources": []string{sourceID}, "enabled": true}, 200)
	original := created["token"].(string)
	agent := created["agent"].(map[string]any)
	id := agent["id"].(string)
	retained, err := h.s.principal(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	agent["enabled"] = false
	agent = h.json("PUT", "/api/agents/"+id, agent, 200)["agent"].(map[string]any)
	if _, err = h.s.principal(context.Background(), original); err == nil {
		t.Fatal("paused credential accepted")
	}
	agent["enabled"] = true
	agent = h.json("PUT", "/api/agents/"+id, agent, 200)["agent"].(map[string]any)
	if _, err = h.s.principal(context.Background(), original); err != nil {
		t.Fatal("pause did not retain credential", err)
	}
	rotated := h.json("POST", "/api/agents/"+id+"/token", nil, 200)
	if _, err = h.s.principal(context.Background(), original); err == nil {
		t.Fatal("old token revived after rotation")
	}
	if _, err = h.s.Engine.Execute(context.Background(), retained, "query_sql", model.Query{SourceID: sourceID, Query: "SELECT 1"}); err == nil {
		t.Fatal("retained principal bypassed token rotation")
	}
	replacement := rotated["token"].(string)
	call(t, h.mcp(replacement), "query_sql", map[string]any{"source_id": sourceID, "query": "SELECT count(*) FROM events"}, false)
	h.json("DELETE", "/api/agents/"+id, nil, 200)
	h.json("PUT", "/api/agents/"+id, agent, 409)
	if _, err = h.s.principal(context.Background(), replacement); err == nil {
		t.Fatal("revoked token accepted")
	}
	reissued := h.json("POST", "/api/agents/"+id+"/token", nil, 200)
	if reissued["agent"].(map[string]any)["id"] != id {
		t.Fatal("replacement lost Agent identity")
	}
	if _, err = h.s.principal(context.Background(), replacement); err == nil {
		t.Fatal("reissuing revived revoked token")
	}
	call(t, h.mcp(reissued["token"].(string)), "query_sql", map[string]any{"source_id": sourceID, "query": "SELECT count(*) FROM events"}, false)
}

func TestCredentialMethodsProbeFailureAndDiagnostics(t *testing.T) {
	h := newHub(t)
	var status atomic.Int32
	status.Store(200)
	var auth atomic.Value
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(int(status.Load()))
		io.WriteString(w, `{"version":{"number":"8.19.14"}}`)
	}))
	defer target.Close()
	u, _ := url.Parse(target.URL)
	port, _ := strconv.Atoi(u.Port())
	src := h.json("POST", "/api/sources", map[string]any{"name": "Authentication fixture", "kind": "elasticsearch", "host": u.Hostname(), "port": port, "tls_mode": "disable", "token": "old-secret-token", "auth_mode": "token", "enabled": true}, 200)
	id := src["id"].(string)
	h.json("POST", "/api/sources/"+id+"/test", nil, 200)
	if auth.Load() != "Bearer old-secret-token" {
		t.Fatal("token not sent")
	}
	src["auth_mode"] = "password"
	src["username"] = "reader"
	src["password"] = "new-secret-password"
	src = h.json("PUT", "/api/sources/"+id, src, 200)
	h.json("POST", "/api/sources/"+id+"/test", nil, 200)
	if auth.Load() != "Basic "+base64.StdEncoding.EncodeToString([]byte("reader:new-secret-password")) {
		t.Fatal("old token still overrides password")
	}
	stored, _ := h.s.Store.Source(id)
	if stored.Token != "" {
		t.Fatal("old token was not removed")
	}
	src["clear_password"] = true
	src = h.json("PUT", "/api/sources/"+id, src, 200)
	stored, _ = h.s.Store.Source(id)
	if stored.Password != "" {
		t.Fatal("credential clear was ignored")
	}
	src["auth_mode"] = "none"
	h.json("PUT", "/api/sources/"+id, src, 200)
	h.json("POST", "/api/sources/"+id+"/test", nil, 200)
	if auth.Load() != "" {
		t.Fatal("none authentication sent a credential")
	}
	status.Store(401)
	failure := h.json("POST", "/api/sources/"+id+"/test", nil, 400)
	diag := failure["error"].(map[string]any)
	if diag["code"] != "database_authentication" || diag["native_code"] != "HTTP 401" || diag["request_id"] == "" {
		t.Fatalf("missing diagnostic: %v", diag)
	}
	stored, _ = h.s.Store.Source(id)
	if stored.Probe == nil || stored.Probe.Connected || stored.Probe.Error == nil {
		t.Fatal("failed check retained success")
	}
	response := h.req("GET", "/api/audit?request_id="+diag["request_id"].(string)+"&status=error&source_id="+id, nil, "", 200)
	defer response.Body.Close()
	var audits []model.Audit
	json.NewDecoder(response.Body).Decode(&audits)
	if len(audits) != 1 || audits[0].ErrorCode != "database_authentication" {
		t.Fatal("diagnostic audit correlation failed")
	}
	status.Store(200)
	h.json("POST", "/api/sources/"+id+"/test", nil, 200)
	stored, _ = h.s.Store.Source(id)
	if !stored.Probe.Connected || stored.Probe.Error != nil {
		t.Fatal("recovery did not replace failed check")
	}
}

func TestAdminPreviewPrecisionAndAgentIsolation(t *testing.T) {
	h := newHub(t)
	id := h.source()
	created := h.json("POST", "/api/agents", map[string]any{"name": "Preview reader", "sources": []string{id}, "enabled": true}, 200)
	agentID := created["agent"].(map[string]any)["id"].(string)
	raw := `{"source_id":"` + id + `","operation":"query_sql","agent_id":"` + agentID + `","query":{"query":"SELECT ? AS amount","params":[9007199254740993]}}`
	res := h.req("POST", "/api/query", strings.NewReader(raw), "application/json", 200)
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(b, []byte(`"9007199254740993"`)) {
		t.Fatalf("preview lost integer precision: %s", b)
	}
	activity, err := h.s.Store.AgentActivity()
	if err != nil || activity[agentID].LastSuccess != nil {
		t.Fatal("preview counted as client connectivity", err)
	}
	none := h.json("POST", "/api/agents", map[string]any{"name": "No grants", "sources": []string{}, "enabled": true}, 200)
	deniedID := none["agent"].(map[string]any)["id"].(string)
	h.json("POST", "/api/query", map[string]any{"agent_id": deniedID, "source_id": id, "operation": "query_sql", "query": map[string]any{"query": "SELECT 1"}}, 400)
	h.json("GET", "/api/sources/"+id+"/objects?agent_id="+deniedID, nil, 400)
	call(t, h.mcp(created["token"].(string)), "query_sql", map[string]any{"source_id": id, "query": "SELECT 1"}, false)
	activity, err = h.s.Store.AgentActivity()
	if err != nil || activity[agentID].LastSuccess == nil {
		t.Fatal("client success not recorded", err)
	}
}

func TestAgentEditsRejectStaleGrantsAndCleanDeletedSources(t *testing.T) {
	h := newHub(t)
	id, other := h.source(), h.source()
	created := h.json("POST", "/api/agents", map[string]any{"name": "Reader", "enabled": true, "sources": []string{id, other}}, 200)
	stale := created["agent"].(map[string]any)
	path := "/api/agents/" + stale["id"].(string)
	update := map[string]any{}
	for k, v := range stale {
		update[k] = v
	}
	update["sources"] = []string{id}
	current := h.json("PUT", path, update, 200)["agent"].(map[string]any)
	stale["name"] = "Stale rename"
	h.json("PUT", path, stale, 409)
	principal, err := h.s.principal(context.Background(), created["token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.s.Engine.Authorize(principal, other); err == nil {
		t.Fatal("stale edit restored removed access")
	}
	current["enabled"] = false
	h.json("PUT", path, current, 200)
	current["enabled"] = true
	h.json("PUT", path, current, 409)
	if _, err = h.s.principal(context.Background(), created["token"].(string)); err == nil {
		t.Fatal("stale edit resumed a paused Agent")
	}
	h.json("DELETE", "/api/sources/"+id, nil, 200)
	fresh, err := h.s.Store.Agent(stale["id"].(string))
	if err != nil || len(fresh.Sources) != 0 {
		t.Fatal("deleted source grant retained", err)
	}
	fresh.Name = "Editable after deletion"
	h.json("PUT", path, fresh, 200)
}

func TestAgentEmptyGrantsRemainEditable(t *testing.T) {
	h := newHub(t)
	for _, explicitNull := range []bool{false, true} {
		input := map[string]any{"name": "Empty grants", "enabled": true}
		if explicitNull {
			input["sources"] = nil
		}
		created := h.json("POST", "/api/agents", input, 200)
		a := created["agent"].(map[string]any)
		if grants, ok := a["sources"].([]any); !ok || len(grants) != 0 {
			t.Fatal("empty grants were not encoded as an array")
		}
		id := a["id"].(string)
		// Simulate an existing row from a version that persisted omitted grants as null.
		a["sources"] = nil
		legacy, _ := json.Marshal(a)
		if _, err := h.s.Store.DB.Exec("UPDATE agents SET value=$1 WHERE id=$2", string(legacy), id); err != nil {
			t.Fatal(err)
		}
		response := h.req("GET", "/api/agents", nil, "application/json", 200)
		var listed []model.Agent
		err := json.NewDecoder(response.Body).Decode(&listed)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, agent := range listed {
			if agent.Sources == nil {
				t.Fatal("legacy empty grants can still crash the UI")
			}
			if agent.ID == id {
				agent.Name = "Editable empty grants"
				h.json("PUT", "/api/agents/"+id, agent, 200)
			}
		}
		principal, err := h.s.principal(context.Background(), created["token"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if sources, err := h.s.Engine.Sources(principal); err != nil || len(sources) != 0 {
			t.Fatal("empty grants changed authorization", err)
		}
	}
}

func TestSQLMetadataPaginationThroughMCPAndPreview(t *testing.T) {
	h := newHub(t)
	id := h.source()
	db, err := sql.Open("sqlite3", filepath.Join(h.dir, "fixture.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 1000; i++ {
		if _, err = tx.Exec(fmt.Sprintf("CREATE TABLE t_%04d(id INTEGER)", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	a := h.json("POST", "/api/agents", map[string]any{"name": "Schema reader", "enabled": true, "sources": []string{id}}, 200)
	client := h.mcp(a["token"].(string))
	page := func(args map[string]any) model.Result {
		t.Helper()
		r := call(t, client, "list_objects", args, false)
		wire, _ := json.Marshal(r.StructuredContent)
		var result model.Result
		if err := json.Unmarshal(wire, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	args := map[string]any{"source_id": id, "namespace": "main"}
	first := page(args)
	if first.RowCount != 1000 || first.NextCursor == "" {
		t.Fatal("schema discovery silently lost its 1001st table")
	}
	args["cursor"] = first.NextCursor
	last := page(args)
	if last.RowCount != 1 || last.NextCursor != "" || last.Truncated {
		t.Fatal("metadata continuation did not finish")
	}
	all := append(first.Data, last.Data...)
	for i, item := range all {
		want := "events"
		if i > 0 {
			want = fmt.Sprintf("t_%04d", i-1)
		}
		if item.(map[string]any)["name"] != want {
			t.Fatal("metadata pages omitted or repeated tables")
		}
	}
	call(t, client, "list_namespaces", args, true)
	args["namespace"] = "other"
	call(t, client, "list_objects", args, true)
	args = map[string]any{"source_id": id, "namespace": "main", "max_bytes": 2048}
	small := page(args)
	if small.RowCount == 0 || small.NextCursor == "" {
		t.Fatal("byte-limited metadata cannot continue")
	}
	args["cursor"] = small.NextCursor
	next := page(args)
	if next.Data[0].(map[string]any)["name"] != fmt.Sprintf("t_%04d", small.RowCount-1) {
		t.Fatal("byte-limited continuation skipped an object")
	}
	path := "/api/sources/" + id + "/objects?operation=objects&namespace=main"
	preview := h.json("GET", path, nil, 200)
	cursor, _ := preview["next_cursor"].(string)
	if cursor == "" {
		t.Fatal("admin preview did not expose metadata pagination")
	}
	end := h.json("GET", path+"&cursor="+url.QueryEscape(cursor), nil, 200)
	if end["row_count"] != float64(1) || end["next_cursor"] != nil {
		t.Fatal("admin preview continuation failed")
	}
}

func TestSourceRenamePreservesRunningQueryAndCursor(t *testing.T) {
	h := newHub(t)
	started, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			io.WriteString(w, `{"hits":{"hits":[{"_id":"1","sort":[1],"_source":{"value":1}},{"_id":"2","sort":[2],"_source":{"value":2}}]}}`)
		} else {
			io.WriteString(w, `{"hits":{"hits":[]}}`)
		}
	}))
	defer remote.Close()
	u, _ := url.Parse(remote.URL)
	port, _ := strconv.Atoi(u.Port())
	source := h.json("POST", "/api/sources", map[string]any{"name": "Search fixture", "kind": "elasticsearch", "host": u.Hostname(), "port": port, "database": "events", "tls_mode": "disable", "enabled": true}, 200)
	sourceID := source["id"].(string)
	query := model.Query{SourceID: sourceID, Operation: "search", MaxRows: 1, Body: json.RawMessage(`{"sort":[{"value":"asc"}]}`)}
	type outcome struct {
		result *model.Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := h.s.Engine.Execute(context.Background(), model.Principal{Admin: true}, "query_search", query)
		done <- outcome{result, err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("query did not reach the database")
	}
	source["name"] = "Renamed while reading"
	updated := h.json("PUT", "/api/sources/"+sourceID, source, 200)
	close(release)
	result := <-done
	if result.err != nil || result.result.NextCursor == "" {
		t.Fatal("rename interrupted query or pagination", result.err)
	}
	query.Cursor = result.result.NextCursor
	if _, err := h.s.Engine.Execute(context.Background(), model.Principal{Admin: true}, "query_search", query); err != nil {
		t.Fatal("rename invalidated cursor", err)
	}
	updated["enabled"] = false
	h.json("PUT", "/api/sources/"+sourceID, updated, 200)
	if _, err := h.s.Engine.Execute(context.Background(), model.Principal{Admin: true}, "query_search", query); err == nil {
		t.Fatal("disabled source retained cursor access")
	}
	h.json("PUT", "/api/sources/"+sourceID, source, 409)
}

func TestInfluxSourceAdvertisesExecutableVersionedExample(t *testing.T) {
	h := newHub(t)
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/v2/query" || r.URL.Query().Get("org") != "review" {
			t.Errorf("unexpected query API %s", r.URL)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body["query"].(string), `from(bucket: "configured-bucket")`) {
			t.Error("example ignored configured bucket")
		}
		io.WriteString(w, "#datatype,string,long,dateTime:RFC3339,double\n,result,table,_time,_value\n,,0,2026-09-11T00:00:00Z,42\n")
	}))
	defer remote.Close()
	u, _ := url.Parse(remote.URL)
	port, _ := strconv.Atoi(u.Port())
	source := h.json("POST", "/api/sources", map[string]any{"name": "Influx 2", "kind": "influxdb", "version": "2", "host": u.Hostname(), "port": port, "tls_mode": "disable", "enabled": true, "options": map[string]string{"org": "review", "bucket": "configured-bucket"}}, 200)
	sourceID := source["id"].(string)
	cap := source["capability"].(map[string]any)
	result := h.json("POST", "/api/query", map[string]any{"source_id": sourceID, "operation": "query_influxdb", "query": cap["example"]}, 200)
	if result["row_count"] != float64(1) {
		t.Fatal("default example returned no rows")
	}
	denied := h.json("POST", "/api/query", map[string]any{"source_id": sourceID, "operation": "query_influxdb", "query": map[string]any{"language": "influxql", "query": "SELECT * FROM events"}}, 400)
	if denied["error"].(map[string]any)["code"] != "invalid_query" || calls.Load() != 1 {
		t.Fatal("wrong dialect did not produce a local query diagnostic")
	}
	discovery, err := h.s.Engine.Sources(model.Principal{Admin: true})
	if err != nil {
		t.Fatal(err)
	}
	advertised := discovery[0]["capability"].(adapter.SourceCapability)
	if advertised.Version != "2" || len(advertised.Languages) != 1 || advertised.Languages[0] != "flux" {
		t.Fatal("MCP discovery omitted the source dialect")
	}
}

func TestOAuthClientManagementRevokesCredentialsAndPreservesOtherClients(t *testing.T) {
	h := newHub(t)
	sourceID := h.source()
	ctx := context.Background()
	redirect := "http://127.0.0.1:41998/callback"
	register := func(name string) map[string]any {
		return h.json("POST", "/api/oauth/clients", map[string]any{"client_name": name, "redirect_uris": []string{redirect}, "token_endpoint_auth_method": "client_secret_post"}, 201)
	}
	registration, unrelated := register("Managed client"), register("Unrelated client")
	clientID, secret := registration["client_id"].(string), registration["client_secret"].(string)
	clientPath := "/api/oauth/clients/" + url.PathEscape(clientID)
	clients := func() []map[string]any {
		response := h.req("GET", "/api/oauth/clients", nil, "", 200)
		defer response.Body.Close()
		var list []map[string]any
		json.NewDecoder(response.Body).Decode(&list)
		for _, c := range list {
			if c["client_secret"] != nil || c["secret"] != nil {
				t.Fatal("client secret or hash leaked")
			}
		}
		return list
	}
	current := func() map[string]any {
		for _, c := range clients() {
			if c["client_id"] == clientID {
				return c
			}
		}
		t.Fatal("managed client missing")
		return nil
	}
	verifier := strings.Repeat("v", 64)
	hash := sha256.Sum256([]byte(verifier))
	code := func(id, uri string) string {
		values := url.Values{"response_type": {"code"}, "client_id": {id}, "redirect_uri": {uri}, "resource": {h.http.URL + "/mcp"}, "scope": {"db:read offline_access"}, "state": {"client-lifecycle-test"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}, "code_challenge_method": {"S256"}}
		response := h.req("GET", "/oauth/authorize?"+values.Encode(), nil, "", 303)
		location, _ := url.Parse(response.Header.Get("Location"))
		response.Body.Close()
		consent := h.json("POST", "/api/oauth/consent", map[string]any{"request": location.Query().Get("request"), "allow": true, "sources": []string{sourceID}}, 200)
		callback, _ := url.Parse(consent["redirect"].(string))
		return callback.Query().Get("code")
	}
	exchange := func(id, password, uri, authorizationCode string, status int) map[string]any {
		return h.form("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "client_id": {id}, "client_secret": {password}, "redirect_uri": {uri}, "resource": {h.http.URL + "/mcp"}, "code": {authorizationCode}, "code_verifier": {verifier}}, status)
	}
	issue := func(id, password, uri string) map[string]any { return exchange(id, password, uri, code(id, uri), 200) }
	edit := func(c map[string]any, enabled bool, uri string) map[string]any {
		return h.json("PUT", clientPath, map[string]any{"revision": c["revision"], "client_name": c["client_name"], "redirect_uris": []string{uri}, "enabled": enabled}, 200)["client"].(map[string]any)
	}
	tokens := issue(clientID, secret, redirect)
	otherTokens := issue(unrelated["client_id"].(string), unrelated["client_secret"].(string), redirect)
	if _, err := h.s.principal(ctx, tokens["access_token"].(string)); err != nil {
		t.Fatal(err)
	}
	// Old deployments identified the client through OAuth requests, not Agent.client_id.
	principal, _ := h.s.principal(ctx, tokens["access_token"].(string))
	legacy, _ := h.s.Store.Agent(principal.AgentID)
	legacy.ClientID = ""
	h.s.Store.SaveAgent(legacy, "")
	c := current()
	c["client_name"] = "Renamed client"
	c = edit(c, true, redirect)
	if _, err := h.s.principal(ctx, tokens["access_token"].(string)); err != nil {
		t.Fatal("rename revoked client", err)
	}
	pending := code(clientID, redirect)
	replacementRedirect := "http://127.0.0.1:41998/new-callback"
	c = edit(c, true, replacementRedirect)
	if _, err := h.s.principal(ctx, tokens["access_token"].(string)); err == nil {
		t.Fatal("redirect update retained token")
	}
	legacy, _ = h.s.Store.Agent(principal.AgentID)
	if legacy.RevokedAt == nil {
		t.Fatal("legacy client grant was not retired")
	}
	exchange(clientID, secret, redirect, pending, 400)
	h.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "client_secret": {secret}, "resource": {h.http.URL + "/mcp"}, "refresh_token": {tokens["refresh_token"].(string)}}, 400)
	tokens = issue(clientID, secret, replacementRedirect)
	rotated := h.json("POST", clientPath+"/secret", map[string]any{"revision": c["revision"]}, 200)
	if _, err := h.s.principal(ctx, tokens["access_token"].(string)); err == nil {
		t.Fatal("secret rotation retained token")
	}
	h.json("POST", clientPath+"/secret", map[string]any{"revision": c["revision"]}, 409)
	exchange(clientID, secret, replacementRedirect, code(clientID, replacementRedirect), 401)
	secret = rotated["client_secret"].(string)
	c = rotated["client"].(map[string]any)
	tokens = issue(clientID, secret, replacementRedirect)
	c = edit(c, false, replacementRedirect)
	if _, err := h.s.principal(ctx, tokens["access_token"].(string)); err == nil {
		t.Fatal("disabled client retained token")
	}
	h.form("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "client_secret": {secret}, "resource": {h.http.URL + "/mcp"}, "refresh_token": {tokens["refresh_token"].(string)}}, 401)
	c = edit(c, true, replacementRedirect)
	if _, err := h.s.principal(ctx, tokens["access_token"].(string)); err == nil {
		t.Fatal("enable resurrected token")
	}
	tokens = issue(clientID, secret, replacementRedirect)
	h.json("DELETE", clientPath, map[string]any{"revision": c["revision"]}, 200)
	if _, err := h.s.principal(ctx, tokens["access_token"].(string)); err == nil {
		t.Fatal("deleted client retained token")
	}
	if len(clients()) != 1 {
		t.Fatal("client registration slot was not released")
	}
	if _, err := h.s.principal(ctx, otherTokens["access_token"].(string)); err != nil {
		t.Fatal("unrelated client revoked", err)
	}
}

func TestEvaluationActivityIsolationAndReadiness(t *testing.T) {
	h := newHub(t)
	source := h.source()
	a := h.json("POST", "/api/agents", map[string]any{"name": "Evaluation reader", "sources": []string{source}, "enabled": true}, 200)
	agent := a["agent"].(map[string]any)["id"].(string)
	now := time.Now().Add(-time.Minute)
	for _, row := range []model.Audit{
		{AgentID: agent, SourceID: source, Operation: "query_sql", ElapsedMS: 12},
		{AgentID: agent, SourceID: source, Operation: "execute_query_template", ElapsedMS: 18, ErrorCode: "template_changed"},
		{AgentID: agent, SourceID: source, Operation: "namespaces", ElapsedMS: 2},
		{AgentID: agent, SourceID: source, Operation: "query_sql", ElapsedMS: 999, Preview: true},
		{AgentID: "another-agent", SourceID: source, Operation: "query_sql", ElapsedMS: 999},
		{AgentID: agent, SourceID: "another-source", Operation: "query_sql", ElapsedMS: 999},
	} {
		row.At = now
		if err := h.s.Store.Audit(row); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/sources/" + source + "/evaluation?agent_id=" + agent
	capture := h.json("GET", path+"&from="+now.Add(-time.Second).UTC().Format(time.RFC3339), nil, 200)
	stats := capture["stats"].(map[string]any)
	if stats["calls"] != float64(3) || stats["queries"] != float64(2) || stats["successful_queries"] != float64(1) || stats["errors"] != float64(1) || stats["elapsed_ms"] != float64(32) {
		t.Fatal(stats)
	}
	empty := h.json("GET", path, nil, 200)["stats"].(map[string]any)
	if empty["calls"] != float64(0) {
		t.Fatal("new capture includes prior activity", empty)
	}
	h.json("GET", path+"&from=invalid", nil, 400)
	h.json("GET", path+"&from="+now.Add(-25*time.Hour).UTC().Format(time.RFC3339), nil, 400)
	ready := h.json("GET", "/api/sources/"+source+"/readiness", nil, 200)
	if ready["last_query"] == nil || len(ready["active_agents"].([]any)) != 1 {
		t.Fatal(ready)
	}
	// Query success is not inferred from previews or metadata discovery.
	if _, err := h.s.Store.DB.Exec("DELETE FROM audit WHERE preview=FALSE AND operation IN ('query_sql','execute_query_template')"); err != nil {
		t.Fatal(err)
	}
	ready = h.json("GET", "/api/sources/"+source+"/readiness", nil, 200)
	if ready["last_query"] != nil {
		t.Fatal("preview/discovery counted as a real query", ready)
	}
	for _, endpoint := range []string{path, "/api/sources/" + source + "/readiness"} {
		req, _ := http.NewRequest("GET", h.http.URL+endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+a["token"].(string))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("Agent accessed administrator workflow: %d", res.StatusCode)
		}
	}
}

func TestEvaluationHistoryLifecycle(t *testing.T) {
	h := newHub(t)
	source, otherSource := h.source(), h.source()
	base := "/api/sources/" + source + "/evaluation"
	a := h.json("POST", "/api/agents", map[string]any{"name": "Dedicated evaluator", "sources": []string{source}, "enabled": true}, 200)
	agent := a["agent"].(map[string]any)["id"].(string)
	question := map[string]any{"name": "Revenue 业务", "question": "private-business-question", "criteria": "private-acceptance-criteria", "client": "test client"}
	saved := h.json("POST", base+"/questions", question, 200)
	questionPath := base + "/questions/" + saved["id"].(string)
	input := map[string]any{"name": question["name"], "question": question["question"], "criteria": question["criteria"], "client": question["client"], "case_id": saved["id"], "case_revision": saved["revision"], "agent_id": agent, "kind": "baseline"}
	input["stats"] = map[string]any{"queries": 99}
	h.json("POST", base+"/history", input, 400)
	delete(input, "stats")
	input["agent_id"] = "admin"
	h.json("POST", base+"/history", input, 404)
	input["agent_id"] = agent
	v := h.json("POST", base+"/history", input, 200)
	id := v["id"].(string)
	path := base + "/history/" + id
	h.json("GET", "/api/sources/"+otherSource+"/evaluation/history/"+id, nil, 404)
	h.json("DELETE", path, map[string]any{"revision": v["revision"]}, 409)
	h.json("POST", path+"/capture", map[string]any{"revision": v["revision"], "kind": "guided", "action": "start"}, 409)
	h.json("PUT", path+"/review", map[string]any{"revision": v["revision"], "reviews": map[string]any{"baseline": map[string]any{"verdict": "correct"}}}, 400)

	question["revision"] = saved["revision"]
	question["question"] = "changed business question"
	updated := h.json("PUT", questionPath, question, 200)
	h.json("PUT", questionPath, question, 409)
	h.json("POST", base+"/history", input, 409)
	retained := h.json("GET", path, nil, 200)
	if retained["question"] != input["question"] || retained["case_revision"] != saved["revision"] {
		t.Fatal("editing a reusable question rewrote history", retained)
	}

	client := h.mcp(a["token"].(string))
	call(t, client, "query_sql", map[string]any{"source_id": source, "query": "SELECT amount FROM events WHERE id=?", "params": []any{1}}, false)
	for _, row := range []model.Audit{
		{AgentID: agent, SourceID: source, Operation: "query_sql", Preview: true},
		{AgentID: "another-agent", SourceID: source, Operation: "query_sql"},
		{AgentID: agent, SourceID: otherSource, Operation: "query_sql"},
	} {
		row.At = time.Now()
		if err := h.s.Store.Audit(row); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(2 * time.Millisecond)
	v = h.json("POST", path+"/capture", map[string]any{"revision": v["revision"], "kind": "baseline", "action": "collect"}, 200)
	baseline := v["runs"].(map[string]any)["baseline"].(map[string]any)
	stats := baseline["stats"].(map[string]any)
	if stats["calls"] != float64(1) || stats["successful_queries"] != float64(1) {
		t.Fatal("capture includes calls from another identity, source or preview", stats)
	}
	oldRevision := v["revision"]
	review := map[string]any{"baseline": map[string]any{"verdict": "correct", "notes": "private-review-notes"}}
	v = h.json("PUT", path+"/review", map[string]any{"revision": v["revision"], "reviews": review}, 200)
	h.json("PUT", path+"/review", map[string]any{"revision": oldRevision, "reviews": review}, 409)
	h.json("PUT", path+"/review", map[string]any{"revision": v["revision"], "reviews": map[string]any{"baseline": map[string]any{"verdict": "correct", "stats": stats}}}, 400)
	v = h.json("POST", path+"/capture", map[string]any{"revision": v["revision"], "kind": "guided", "action": "start"}, 200)
	h.json("DELETE", "/api/agents/"+agent, map[string]any{"revision": a["agent"].(map[string]any)["revision"]}, 200)
	v = h.json("POST", path+"/capture", map[string]any{"revision": v["revision"], "kind": "guided", "action": "collect"}, 200)
	if !v["runs"].(map[string]any)["guided"].(map[string]any)["configuration_changed"].(bool) {
		t.Fatal("grant revocation did not mark the changed evaluation conditions")
	}
	delete(input, "case_id")
	delete(input, "case_revision")
	h.json("POST", base+"/history", input, 404)
	h.json("DELETE", questionPath, map[string]any{"revision": updated["revision"]}, 200)
	if _, err := h.s.Store.DB.Exec("DELETE FROM audit"); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(filepath.Join(h.dir, "config"), testpg.DSN(t, h.dir))
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Evaluation(source, id)
	reopened.Close()
	if err != nil || recovered.Runs["baseline"].Stats.SuccessfulQueries != 1 || recovered.Runs["baseline"].Notes != "private-review-notes" || recovered.Question != input["question"] {
		t.Fatal("history did not survive reopening, case deletion and audit retention", recovered, err)
	}
	var sealed string
	if err := h.s.Store.DB.QueryRow("SELECT value FROM evaluations WHERE source_id=$1 AND id=$2", source, id).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-business-question", "private-acceptance-criteria", "private-review-notes"} {
		if strings.Contains(sealed, secret) {
			t.Fatal("evaluation persisted in plaintext", secret)
		}
	}

	for _, route := range []string{base + "/questions", base + "/history", path, path + "/capture", path + "/review"} {
		method := "GET"
		if strings.HasSuffix(route, "/capture") {
			method = "POST"
		}
		if strings.HasSuffix(route, "/review") {
			method = "PUT"
		}
		req, _ := http.NewRequest(method, h.http.URL+route, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+a["token"].(string))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatal("Agent accessed evaluation administration", route, res.StatusCode)
		}
	}
	csrf := h.csrf
	h.csrf = "invalid"
	h.json("POST", base+"/questions", question, 403)
	h.csrf = csrf
	if err := h.s.Store.DeleteSource(source); err != nil {
		t.Fatal(err)
	}
	h.json("GET", path, nil, 404)
	for _, table := range []string{"evaluation_questions", "evaluations"} {
		var count int
		if err := h.s.Store.DB.QueryRow("SELECT count(*) FROM "+table+" WHERE source_id=$1", source).Scan(&count); err != nil || count != 0 {
			t.Fatal("source deletion retained evaluation data", table, count, err)
		}
	}
}

func TestEvaluationPagingAndExpiredCapture(t *testing.T) {
	h := newHub(t)
	source := h.source()
	base := "/api/sources/" + source + "/evaluation"
	for n := range 21 {
		q := model.EvaluationQuestion{ID: fmt.Sprint(n), Revision: 1, Name: "Question", Question: "Q", Criteria: "C", Created: time.Now()}
		if err := h.s.Store.SaveEvaluationQuestion(source, q, 0); err != nil {
			t.Fatal(err)
		}
	}
	page := h.json("GET", base+"/questions", nil, 200)
	if len(page["items"].([]any)) != 20 || page["next_cursor"] == "" {
		t.Fatal(page)
	}
	next := h.json("GET", base+"/questions?cursor="+page["next_cursor"].(string), nil, 200)
	if len(next["items"].([]any)) != 1 || next["next_cursor"] != "" || next["items"].([]any)[0].(map[string]any)["id"] != "0" {
		t.Fatal(next)
	}
	h.json("GET", base+"/questions?cursor=-1", nil, 400)
	a := h.json("POST", "/api/agents", map[string]any{"name": "Expired evaluation", "sources": []string{source}, "enabled": true}, 200)
	v := h.json("POST", base+"/history", map[string]any{"name": "Expired", "question": "Q", "criteria": "C", "agent_id": a["agent"].(map[string]any)["id"], "kind": "baseline"}, 200)
	id := v["id"].(string)
	record, err := h.s.Store.Evaluation(source, id)
	if err != nil {
		t.Fatal(err)
	}
	record.Runs["baseline"].Started = time.Now().Add(-25 * time.Hour)
	record.Revision++
	if err := h.s.Store.SaveEvaluation(record, record.Revision-1); err != nil {
		t.Fatal(err)
	}
	action := map[string]any{"revision": strconv.FormatInt(record.Revision, 10), "kind": "baseline", "action": "collect"}
	h.json("POST", base+"/history/"+id+"/capture", action, 400)
	action["action"] = "abandon"
	v = h.json("POST", base+"/history/"+id+"/capture", action, 200)
	h.json("DELETE", base+"/history/"+id, map[string]any{"revision": v["revision"]}, 200)
	if err := h.s.Store.DeleteSource(source); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := h.s.Store.DB.QueryRow("SELECT count(*) FROM evaluation_questions WHERE source_id=$1", source).Scan(&count); err != nil || count != 0 {
		t.Fatal("question cleanup failed", count, err)
	}
}

func TestEvaluationHistoryFiltersAndSummary(t *testing.T) {
	h := newHub(t)
	source, other := h.source(), h.source()
	created := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	for n := range 26 {
		v := model.Evaluation{ID: fmt.Sprint(n), SourceID: source, Revision: 1, Name: "Customer orders 客户", Question: "Question", AgentID: "agent-a", AgentName: "Reader", Created: created, Runs: map[string]*model.EvaluationCapture{}}
		for _, kind := range []string{"baseline", "guided"} {
			v.Runs[kind] = &model.EvaluationCapture{State: "completed", Verdict: "correct", Stats: &model.EvaluationStats{Queries: 1}}
		}
		switch n {
		case 0:
			v.Runs["guided"].ConfigurationChanged = true
		case 1:
			v.Runs["guided"].Stats.Queries = 0
		case 2:
			v.Runs["guided"].Verdict = "unrated"
		case 3:
			v.Runs["guided"].Verdict = "incorrect"
		case 22:
			v.SourceID = other
		case 23:
			v.AgentID = "agent-b"
		case 24:
			v.Created = created.Add(-48 * time.Hour)
		case 25:
			v.Question, v.Name = "Unrelated", "Other question"
		}
		if err := h.s.Store.SaveEvaluation(v, 0); err != nil {
			t.Fatal(err)
		}
	}
	base := "/api/sources/" + source + "/evaluation/history"
	query := "?search=customer&agent=agent-a&from=2026-09-12T00:00:00Z&until=2026-09-13T00:00:00Z"
	page := h.json("GET", base+query, nil, 200)
	summary := page["summary"].(map[string]any)
	if summary["total"] != float64(22) || summary["completed_pairs"] != float64(22) || summary["reviewed_pairs"] != float64(19) || summary["baseline_correct"] != float64(19) || summary["guided_correct"] != float64(18) || summary["changed_pairs"] != float64(1) {
		t.Fatal(summary)
	}
	if len(page["items"].([]any)) != 20 || page["next_cursor"] == "" {
		t.Fatal(page)
	}
	next := h.json("GET", base+query+"&cursor="+page["next_cursor"].(string), nil, 200)
	if len(next["items"].([]any)) != 2 || next["next_cursor"] != "" || next["summary"].(map[string]any)["total"] != float64(22) {
		t.Fatal(next)
	}
	if len(h.json("GET", base+"?search="+url.QueryEscape("客户")+"&agent=agent-b", nil, 200)["items"].([]any)) != 1 {
		t.Fatal("Unicode or Agent filtering failed")
	}
	if len(h.json("GET", base+"?search=no-match", nil, 200)["items"].([]any)) != 0 {
		t.Fatal("Unmatched records returned")
	}
	for _, q := range []string{"?from=bad", "?until=2026-01-01", "?from=2026-09-13T00:00:00Z&until=2026-09-12T00:00:00Z"} {
		h.json("GET", base+q, nil, 400)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.s.Store.SearchEvaluations(ctx, source, false, 0, store.EvaluationFilter{}); err == nil {
		t.Fatal("Cancelled search continued")
	}
}

func TestHealthDriftDiagnosticsAndManagementAudit(t *testing.T) {
	h := newHub(t)
	id := h.source()
	path := "/api/sources/" + id + "/semantics"
	draft := semantic.Snapshot{FormatVersion: semantic.FormatVersion, Entries: []semantic.Entry{{ID: "events", Kind: "object", Name: "Events", Reference: &semantic.Reference{Object: "events"}}}}
	saved := h.json("PUT", path, semanticInput{Snapshot: draft}, 200)
	h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	cfg := h.json("GET", "/api/settings/health", nil, 200)
	if cfg["enabled"] != false {
		t.Fatal("periodic checks must be opt-in")
	}
	cfg["enabled"], cfg["interval_minutes"] = true, 5
	h.json("PUT", "/api/settings/health", cfg, 200)
	h.json("PUT", "/api/settings/health", cfg, 409)
	initial := h.json("POST", "/api/health/"+id+"/check", nil, 200)
	if initial["structure"].([]any)[0].(map[string]any)["status"] != "baseline" {
		t.Fatal(initial)
	}
	db, err := sql.Open("sqlite3", filepath.Join(h.dir, "fixture.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("ALTER TABLE events ADD COLUMN extra TEXT"); err != nil {
		t.Fatal(err)
	}
	changed := h.json("POST", "/api/health/"+id+"/check", nil, 200)
	if changed["structure"].([]any)[0].(map[string]any)["status"] != "changed" {
		t.Fatal(changed)
	}
	changes := changed["structure"].([]any)[0].(map[string]any)["changes"].([]any)
	if len(changes) != 1 || changes[0].(map[string]any)["column"] != "extra" {
		t.Fatal("field-level difference missing", changed)
	}
	h.json("POST", "/api/health/"+id+"/baseline", map[string]any{"checked_at": initial["checked_at"]}, 409)
	accepted := h.json("POST", "/api/health/"+id+"/baseline", map[string]any{"checked_at": changed["checked_at"]}, 200)
	if accepted["structure"].([]any)[0].(map[string]any)["status"] != "unchanged" {
		t.Fatal(accepted)
	}
	src, _ := h.s.Store.Source(id)
	view := publicSource(src)
	view.Password = "rotated-private-credential"
	h.json("PUT", "/api/sources/"+id, view, 200)
	status := h.json("GET", "/api/health", nil, 200)
	if status["sources"].([]any)[0].(map[string]any)["status"] != "stale" {
		t.Fatal(status)
	}
	diagnostics := h.json("GET", "/api/settings/diagnostics", nil, 200)
	if diagnostics["checks"].([]any)[1].(map[string]any)["status"] != "passed" {
		t.Fatal(diagnostics)
	}
	audits, err := h.s.Store.Audits(store.AuditFilter{EventKind: "management"}, 100)
	if err != nil {
		t.Fatal(err)
	}
	operations := map[string]bool{}
	for _, audit := range audits {
		operations[audit.Operation] = true
		if audit.Operation == "health.accept_baseline" && audit.SourceID != id {
			t.Fatal("baseline audit lost its source", audit)
		}
		if audit.RequestID == "" || audit.ErrorCode == "operation_pending" {
			t.Fatal("completed change was not correlated", audit)
		}
	}
	if !operations["source.create"] || !operations["source.update"] || !operations["health.accept_baseline"] {
		t.Fatal("management change not audited", operations)
	}
	raw, _ := json.Marshal(struct {
		Audits      []model.Audit
		Diagnostics any
	}{audits, diagnostics})
	for _, secret := range []string{"rotated-private-credential", "db-secret-never-echo", "secret-query-text", "SELECT", h.dir} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("private input leaked to audit or diagnostics")
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM events").Scan(&count); err != nil || count != 3 {
		t.Fatal("health check modified source data", count, err)
	}
	// A missing audit sink must prevent the configuration change itself.
	if _, err := h.s.Store.DB.Exec("ALTER TABLE audit RENAME TO audit_unavailable"); err != nil {
		t.Fatal(err)
	}
	h.json("DELETE", "/api/sources/"+id, nil, 503)
	if _, err := h.s.Store.Source(id); err != nil {
		t.Fatal("source changed without audit", err)
	}
	if _, err := h.s.Store.DB.Exec("ALTER TABLE audit_unavailable RENAME TO audit"); err != nil {
		t.Fatal(err)
	}
	h.json("DELETE", "/api/sources/"+id, nil, 200)
	if _, err := h.s.Store.SourceHealth(id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("source health was not deleted", err)
	}
}
