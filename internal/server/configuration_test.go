package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/testpg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func configurationClient(t *testing.T, h *hubTest, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "configuration-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: h.http.URL + "/mcp/config", HTTPClient: &http.Client{Transport: testBearer{token}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func configurationValue(t *testing.T, session *mcp.ClientSession, tool string, args any, wantError bool) map[string]any {
	t.Helper()
	result := call(t, session, tool, args, wantError)
	raw, _ := json.Marshal(result.StructuredContent)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestConfigurationMCPSourceToPublishedOntologyTemplate(t *testing.T) {
	h := newHub(t)
	h.source() // A real read-only database fixture; the new source is created via MCP.
	credential := h.json("POST", "/api/configuration-agents", map[string]any{"name": "Configuration fixture"}, 200)
	session := configurationClient(t, h, credential["token"].(string))
	invoke := func(tool string, args any) map[string]any { return configurationValue(t, session, tool, args, false) }
	guide := invoke("get_configuration_guide", map[string]any{})
	if guide["publication"] != "administrator_only" {
		t.Fatal("publication boundary missing")
	}
	invoke("list_supported_databases", map[string]any{})
	created := invoke("create_data_source", map[string]any{"configuration": map[string]any{"name": "配置测试", "kind": "sqlite", "path": filepath.Join(h.dir, "fixture.sqlite"), "tls_mode": "disable", "enabled": true, "password": "configuration-private-password", "query_access_mode": "templates_only"}})
	id := created["id"].(string)
	read := invoke("get_source_configuration", map[string]any{"source_id": id})
	raw, _ := json.Marshal(read)
	if bytes.Contains(raw, []byte("configuration-private-password")) || read["has_secret"] != true {
		t.Fatal("configuration read leaked credentials")
	}
	editable := read["configuration"].(map[string]any)
	editable["name"] = "配置测试 renamed"
	update := map[string]any{"source_id": id, "revision": read["revision"], "configuration": editable}
	invoke("update_data_source", update)
	conflict := configurationValue(t, session, "update_data_source", update, true)
	if conflict["error"].(map[string]any)["code"] != "conflict" {
		t.Fatal("revision conflict lost", conflict)
	}
	stored, _ := h.s.Store.Source(id)
	if stored.Password != "configuration-private-password" {
		t.Fatal("omitted credential was cleared")
	}
	invoke("test_data_source", map[string]any{"source_id": id})
	invoke("discover_source_structure", map[string]any{"source_id": id, "operation": "describe", "namespace": "main", "object": "events"})
	page := invoke("list_configured_sources", map[string]any{"limit": 1})
	if page["has_more"] != true {
		t.Fatal("source pagination missing")
	}
	saved := invoke("import_source_structure", map[string]any{"source_id": id, "revision": "0", "objects": []ontology.Reference{{Namespace: "main", Object: "events"}}})
	if len(saved["draft"].(map[string]any)["entries"].([]any)) < 2 {
		t.Fatal("structure fields not imported")
	}
	definition := eventOntology()
	ont := invoke("create_ontology", map[string]any{"id": "config-example", "definition": definition})
	invoke("validate_ontology", map[string]any{"ontology_id": ont["id"], "revision": ont["revision"]})
	definition.Name = "Updated shared definition"
	ont = invoke("save_ontology_draft", map[string]any{"ontology_id": ont["id"], "revision": ont["revision"], "definition": definition})
	invoke("list_ontologies", map[string]any{})
	invoke("get_ontology", map[string]any{"ontology_id": ont["id"]})
	if result, err := configurationClient(t, h, credential["token"].(string)).CallTool(context.Background(), &mcp.CallToolParams{Name: "publish_ontology", Arguments: map[string]any{"ontology_id": ont["id"], "revision": ont["revision"]}}); err == nil && !result.IsError {
		t.Fatal("configuration Agent published an ontology")
	}
	h.json("POST", "/api/ontologies/config-example/publish", map[string]any{"revision": ont["revision"]}, 200)
	invoke("get_ontology_version", map[string]any{"ontology_id": ont["id"], "version": "1"})
	draft := semantic.Empty()
	draft.Overview = "private draft business description"
	draft.Ontology = &ontology.Binding{OntologyID: "config-example", Version: 1, Entities: []ontology.EntityMapping{{Entity: "event", Objects: []ontology.Reference{{Namespace: "main", Object: "events"}}}}, Properties: []ontology.PropertyMapping{{Entity: "event", Property: "amount", Reference: &ontology.Reference{Namespace: "main", Object: "events", Field: "amount"}}}, Relations: []ontology.RelationMapping{}}
	template := semanticFixture()
	template.Template.ConceptRefs = []string{ontology.Ref("entity_type", "event"), ontology.PropertyRef("event", "amount")}
	draft.Entries = []semantic.Entry{template}
	saved = invoke("save_semantic_draft", map[string]any{"source_id": id, "revision": saved["revision"], "snapshot": draft})
	template.Description = "Authored description preserved"
	saved = invoke("upsert_semantic_entry", map[string]any{"source_id": id, "revision": saved["revision"], "entry": template})
	invoke("get_semantic_draft", map[string]any{"source_id": id})
	invoke("validate_semantic_draft", map[string]any{"source_id": id})
	invoke("check_ontology_mapping", map[string]any{"source_id": id, "revision": saved["revision"]})
	trial := invoke("trial_query_template", map[string]any{"source_id": id, "revision": saved["revision"], "template_id": "amount"})
	raw, _ = json.Marshal(trial)
	if bytes.Contains(raw, []byte("9007199254740993")) || bytes.Contains(raw, []byte("secret-query-text")) {
		t.Fatal("trial returned query data")
	}
	a := h.json("POST", "/api/agents", map[string]any{"name": "Published reader", "sources": []string{id}, "enabled": true}, 200)
	reader := h.mcp(a["token"].(string))
	call(t, reader, "get_semantic_entry", map[string]any{"source_id": id, "entry_id": "amount"}, true)
	h.json("POST", "/api/sources/"+id+"/semantics/publish", map[string]any{"revision": saved["revision"]}, 200)
	result := call(t, reader, "execute_query_template", semantic.Execution{SourceID: id, TemplateID: "amount", ExecutionVersion: "1", Parameters: map[string]any{"id": 1}}, false)
	raw, _ = json.Marshal(result)
	if !bytes.Contains(raw, []byte("9007199254740993")) || !bytes.Contains(raw, []byte("ontology_context")) {
		t.Fatal("native result or ontology context lost")
	}
	call(t, reader, "query_sql", map[string]any{"source_id": id, "query": "SELECT 1"}, true)
	audits, err := h.s.Store.Audits(store.AuditFilter{Agent: credential["id"].(string)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	trialAudit, configurationAudit := false, false
	for _, audit := range audits {
		if audit.TemplateID == "amount" && audit.Operation == "query_sql" && audit.Preview {
			trialAudit = true
		}
		if audit.Operation == "configuration.save_semantic_draft" && audit.SourceID == id {
			configurationAudit = true
		}
	}
	if !trialAudit || !configurationAudit {
		t.Fatal("configuration identity missing from nested audit")
	}
	raw, _ = json.Marshal(audits)
	for _, secret := range []string{credential["token"].(string), "configuration-private-password", "private draft business description", "SELECT amount", "9007199254740993"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("audit contains configuration payload")
		}
	}
	reopened, err := store.Open(filepath.Join(h.dir, "config"), testpg.DSN(t, h.dir))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if restored, err := reopened.ConfigurationToken(credential["token"].(string)); err != nil || restored.ID != credential["id"] {
		t.Fatal("configuration token did not survive restart")
	}
}

func TestConfigurationCredentialsBoundariesRevocationAndValidation(t *testing.T) {
	h := newHub(t)
	id := h.source()
	a := h.json("POST", "/api/agents", map[string]any{"name": "Query only", "sources": []string{id}, "enabled": true}, 200)
	credential := h.json("POST", "/api/configuration-agents", map[string]any{"name": "Config only"}, 200)
	token, cfgID := credential["token"].(string), credential["id"].(string)
	for _, test := range []struct{ token, path string }{{token, "/mcp"}, {a["token"].(string), "/mcp/config"}, {"", "/mcp/config"}, {token, "/api/sources"}, {token, "/api/configuration-agents"}} {
		req, _ := http.NewRequest("GET", h.http.URL+test.path, nil)
		req.Header.Set("Authorization", "Bearer "+test.token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("credential boundary: %s returned %d", test.path, res.StatusCode)
		}
	}
	h.json("GET", "/mcp/config", nil, 401) // Even an administrator Cookie is not a configuration token.
	csrf := h.csrf
	h.csrf = "bad"
	h.json("POST", "/api/configuration-agents", map[string]any{"name": "No CSRF"}, 403)
	h.json("DELETE", "/api/configuration-agents/"+cfgID, nil, 403)
	h.csrf = csrf
	h.json("POST", "/api/configuration-agents", map[string]any{"name": "Expired", "expires_at": time.Now().Add(-time.Hour)}, 400)
	h.json("POST", "/api/configuration-agents", map[string]any{"name": "Too long", "expires_at": time.Now().Add(31 * 24 * time.Hour)}, 400)
	session := configurationClient(t, h, token)
	for _, args := range []map[string]any{{"source_id": id, "operation": "query_sql"}, {"source_id": id, "operation": "objects", "query": "DELETE"}, {"source_id": id, "operation": "objects", "agent_id": "admin"}} {
		configurationValue(t, session, "discover_source_structure", args, true)
	}
	bad := semanticFixture()
	bad.Template.QueryJSON = `{"query":"DELETE FROM events","params":[0]}`
	saved := configurationValue(t, session, "upsert_semantic_entry", map[string]any{"source_id": id, "revision": "0", "entry": bad}, false)
	configurationValue(t, session, "trial_query_template", map[string]any{"source_id": id, "revision": saved["revision"], "template_id": "amount"}, true)
	h.json("POST", "/api/sources/"+id+"/semantics/publish", map[string]any{"revision": saved["revision"]}, 400)
	db, err := sql.Open("sqlite3", filepath.Join(h.dir, "fixture.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	var count int
	err = db.QueryRow("SELECT count(*) FROM events").Scan(&count)
	db.Close()
	if err != nil || count != 3 {
		t.Fatal("configuration trial changed source data", count, err)
	}
	identity, err := h.s.Store.ConfigurationAgent(cfgID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, finish, err := h.s.startConfigurationCall(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	second, secondFinish, err := h.s.startConfigurationCall(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer secondFinish()
	if _, _, err := h.s.startConfigurationCall(context.Background(), identity); model.ErrorCode(err) != "busy" {
		t.Fatal("configuration concurrency is unbounded")
	}
	h.json("DELETE", "/api/configuration-agents/"+cfgID, nil, 200)
	if ctx.Err() == nil || second.Err() == nil {
		t.Fatal("revocation did not cancel active calls")
	}
	if _, err := h.s.Engine.Authorize(model.AdministratorPrincipal(ctx), id); model.ErrorCode(err) != "unauthorized" {
		t.Fatal("administrator execution bypassed credential revocation")
	}
	value, err := configurationHandler(ctx, h.s.saveSource, "POST /api/sources", nil, map[string]any{"name": "Must not commit", "kind": "sqlite", "path": filepath.Join(h.dir, "fixture.sqlite"), "enabled": true})
	if err == nil {
		t.Fatal("revoked queued configuration committed", value)
	}
	if result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_configuration_guide", Arguments: map[string]any{}}); err == nil && !result.IsError {
		t.Fatal("revoked HTTP configuration token accepted")
	}
	if _, err := h.s.Store.ConfigurationToken(token); err == nil {
		t.Fatal("revoked token hash retained")
	}
	view := h.json("GET", "/api/configuration-agents", nil, 200)
	raw, _ := json.Marshal(view)
	if bytes.Contains(raw, []byte(token)) || bytes.Contains(raw, []byte("token_hash")) {
		t.Fatal("configuration listing exposed credential")
	}
	expired, expiredToken, err := h.s.Store.CreateConfigurationAgent("Expired", time.Now().Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.s.startConfigurationCall(context.Background(), expired); err == nil {
		t.Fatal("expired credential accepted")
	}
	req, _ := http.NewRequest("POST", h.http.URL+"/mcp/config", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("expired HTTP credential accepted")
	}
}

func TestConfigurationMCPPostgresLosslessTemplateAndMapping(t *testing.T) {
	h := newHub(t)
	var namespace string
	if err := h.s.Store.DB.QueryRow("SELECT current_schema()").Scan(&namespace); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.Store.DB.Exec("CREATE TABLE configuration_events(id BIGINT PRIMARY KEY, amount NUMERIC); INSERT INTO configuration_events VALUES(9007199254740993,0.12345678901234567890123456789)"); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(os.Getenv("MCPDBHUB_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	credential := h.json("POST", "/api/configuration-agents", map[string]any{"name": "PostgreSQL setup"}, 200)
	session := configurationClient(t, h, credential["token"].(string))
	invoke := func(tool string, args any) map[string]any { return configurationValue(t, session, tool, args, false) }
	created := invoke("create_data_source", map[string]any{"configuration": map[string]any{"name": "PostgreSQL configuration fixture", "kind": "postgres", "host": u.Hostname(), "database": strings.TrimPrefix(u.Path, "/"), "username": u.User.Username(), "password": password, "tls_mode": "disable", "enabled": true}})
	id := created["id"].(string)
	invoke("test_data_source", map[string]any{"source_id": id})
	invoke("discover_source_structure", map[string]any{"source_id": id, "operation": "describe", "namespace": namespace, "object": "configuration_events"})
	ont := invoke("create_ontology", map[string]any{"definition": eventOntology()})
	h.json("POST", "/api/ontologies/"+ont["id"].(string)+"/publish", map[string]any{"revision": ont["revision"]}, 200)
	draft := semantic.Empty()
	draft.Ontology = &ontology.Binding{OntologyID: ont["id"].(string), Version: 1, Entities: []ontology.EntityMapping{{Entity: "event", Objects: []ontology.Reference{{Namespace: namespace, Object: "configuration_events"}}}}, Properties: []ontology.PropertyMapping{{Entity: "event", Property: "amount", Reference: &ontology.Reference{Namespace: namespace, Object: "configuration_events", Field: "amount"}}}, Relations: []ontology.RelationMapping{}}
	query, _ := json.Marshal(map[string]any{"query": "SELECT id, amount FROM " + namespace + ".configuration_events WHERE id = $1", "params": []int{0}})
	draft.Entries = []semantic.Entry{{ID: "amount", Kind: "template", Name: "Precise amount", Template: &semantic.Template{Enabled: true, Tool: "query_sql", QueryJSON: string(query), Parameters: []semantic.Parameter{{Name: "id", Type: "integer", Required: true, Pointers: []string{"/params/0"}}}, ExampleJSON: `{"id":9007199254740993}`, ConceptRefs: []string{ontology.PropertyRef("event", "amount")}}}}
	saved := invoke("save_semantic_draft", map[string]any{"source_id": id, "revision": "0", "snapshot": draft})
	invoke("check_ontology_mapping", map[string]any{"source_id": id, "revision": saved["revision"]})
	invoke("trial_query_template", map[string]any{"source_id": id, "revision": saved["revision"], "template_id": "amount"})
	h.json("POST", "/api/sources/"+id+"/semantics/publish", map[string]any{"revision": saved["revision"]}, 200)
	a := h.json("POST", "/api/agents", map[string]any{"name": "Postgres reader", "sources": []string{id}, "enabled": true}, 200)
	result := call(t, h.mcp(a["token"].(string)), "execute_query_template", semantic.Execution{SourceID: id, TemplateID: "amount", ExecutionVersion: "1", Parameters: map[string]any{"id": json.Number("9007199254740993")}}, false)
	raw, _ := json.Marshal(result)
	for _, want := range []string{"9007199254740993", "0.12345678901234567890123456789", "ontology_context"} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Fatal("PostgreSQL configuration lost native value or ontology context", want)
		}
	}
}
