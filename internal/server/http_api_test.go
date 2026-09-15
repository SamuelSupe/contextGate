package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
	"github.com/SamuelSupe/contextGate/internal/store"
)

func TestHTTPAPIMCPConfigurationSemanticsAndLifecycle(t *testing.T) {
	h := newHub(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "http-api-private-token" || r.Method != "GET" || r.URL.Path != "/customers" {
			t.Error("unexpected upstream request")
			w.WriteHeader(403)
			return
		}
		if r.URL.Query().Get("cursor") != "" {
			io.WriteString(w, `{"data":[],"next":null}`)
			return
		}
		if r.URL.Query().Get("region") == "fail" {
			w.WriteHeader(422)
			return
		}
		io.WriteString(w, `{"data":[{"id":9007199254740993,"amount":12.00000000000000001}],"next":"page-2"}`)
	}))
	defer upstream.Close()
	config := h.json("POST", "/api/configuration-agents", map[string]any{"name": "HTTP API configurator"}, 200)
	session := configurationClient(t, h, config["token"].(string))
	source := model.Source{Name: "Customer HTTP API", Kind: "http_api", Version: "1", Enabled: true, TLSMode: "disable", AuthMode: "token", Token: "http-api-private-token", HTTPAPI: &model.HTTPAPIConfig{BaseURL: upstream.URL, TokenHeader: "X-API-Key", ProbeOperation: "customers", Operations: []model.HTTPOperation{{ID: "customers", Name: "Customers", Method: "GET", Path: "/customers", ReadOnly: true, ExampleJSON: `{"region":"east"}`, ResponsePointer: "/data", Parameters: []model.HTTPParameter{{Name: "region", In: "query", Target: "region", Type: "string", Required: true}}, Columns: []model.Column{{Name: "id", Type: "integer"}, {Name: "amount", Type: "decimal"}}, Pagination: &model.HTTPPagination{QueryParameter: "cursor", NextPointer: "/next"}}}}}
	// Use the actual configuration MCP schema, not an internal store shortcut.
	raw, _ := json.Marshal(source)
	var editable configurationSource
	json.Unmarshal(raw, &editable)
	created := configurationValue(t, session, "create_data_source", map[string]any{"configuration": editable}, false)
	id := created["id"].(string)
	read := configurationValue(t, session, "get_source_configuration", map[string]any{"source_id": id}, false)
	raw, _ = json.Marshal(read)
	if bytes.Contains(raw, []byte(source.Token)) || read["configuration"].(map[string]any)["http_api"] == nil {
		t.Fatal("HTTP configuration not round-trippable or credential leaked")
	}
	probe := configurationValue(t, session, "test_data_source", map[string]any{"source_id": id}, false)
	if probe["permission_status"] != "unverified" {
		t.Fatal("API permission overstated", probe)
	}
	reader := h.json("POST", "/api/agents", map[string]any{"name": "API reader", "sources": []string{id}, "enabled": true}, 200)
	agentID := reader["agent"].(map[string]any)["id"].(string)
	agent := h.mcp(reader["token"].(string))
	args := map[string]any{"source_id": id, "operation": "customers", "named_params": map[string]any{"region": "east"}}
	native := call(t, agent, "query_http_api", args, false)
	nativeRaw, _ := json.Marshal(native.StructuredContent)
	var result model.Result
	json.Unmarshal(nativeRaw, &result)
	if result.NextCursor == "" || result.RowCount != 1 || !bytes.Contains(nativeRaw, []byte("9007199254740993")) {
		t.Fatal("native result", string(nativeRaw))
	}
	for _, tool := range []string{"list_data_sources", "list_objects", "describe_object"} {
		input := map[string]any{"source_id": id, "namespace": "api"}
		if tool == "list_data_sources" {
			input = map[string]any{}
		}
		if tool == "describe_object" {
			input["object"] = "customers"
		}
		response := call(t, agent, tool, input, false)
		b, _ := json.Marshal(response)
		if bytes.Contains(b, []byte(source.Token)) || bytes.Contains(b, []byte(upstream.URL)) {
			t.Fatal("Agent discovery leaked connection details")
		}
	}
	unauthorized := h.json("POST", "/api/agents", map[string]any{"name": "No API grant", "sources": []string{}, "enabled": true}, 200)
	call(t, h.mcp(unauthorized["token"].(string)), "query_http_api", args, true)
	for key, value := range map[string]any{"url": upstream.URL, "method": "DELETE", "body": map[string]any{}, "headers": map[string]any{}} {
		bad := map[string]any{}
		for k, v := range args {
			bad[k] = v
		}
		bad[key] = value
		call(t, agent, "query_http_api", bad, true)
	}
	path := "/api/sources/" + id + "/semantics"
	template := semantic.Entry{ID: "customers-by-region", Kind: "template", Name: "Customers by region", Template: &semantic.Template{Enabled: true, Tool: "query_http_api", QueryJSON: `{"operation":"customers","named_params":{"region":""}}`, Parameters: []semantic.Parameter{{Name: "region", Type: "string", Required: true, Pointers: []string{"/named_params/region"}}}, ExampleJSON: `{"region":"east"}`, ConceptRefs: []string{ontology.Ref("entity_type", "event")}}}
	definition := h.json("POST", "/api/ontologies", ontologyInput{Definition: eventOntology()}, 200)
	ontologyID := definition["id"].(string)
	h.json("POST", "/api/ontologies/"+ontologyID+"/publish", map[string]any{"revision": definition["revision"]}, 200)
	draft := semantic.Empty()
	draft.Entries = []semantic.Entry{template}
	draft.Ontology = &ontology.Binding{OntologyID: ontologyID, Version: 1, Entities: []ontology.EntityMapping{{Entity: "event", Objects: []ontology.Reference{{Namespace: "api", Object: "customers"}}}}, Properties: []ontology.PropertyMapping{{Entity: "event", Property: "key", Reference: &ontology.Reference{Namespace: "api", Object: "customers", Field: "id"}}}}
	saved := h.json("PUT", path, semanticInput{Snapshot: draft}, 200)
	call(t, agent, "get_semantic_entry", map[string]any{"source_id": id, "entry_id": template.ID}, true)
	h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 400)
	h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": template.ID}, 200)
	checks := h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 200)
	items := checks["checks"].([]any)
	if items[len(items)-1].(map[string]any)["status"] != "unverified" {
		t.Fatal("declared HTTP fields claimed independently verified", checks)
	}
	published := h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	execution := semantic.Execution{SourceID: id, TemplateID: template.ID, ExecutionVersion: "1", Parameters: map[string]any{"region": "east"}}
	response := call(t, agent, "execute_query_template", execution, false)
	raw, _ = json.Marshal(response.StructuredContent)
	var templated model.Result
	json.Unmarshal(raw, &templated)
	a, _ := json.Marshal(result.Data)
	b, _ := json.Marshal(templated.Data)
	if !bytes.Equal(a, b) || templated.OntologyContext == nil || templated.OntologyContext.OntologyID != ontologyID {
		t.Fatal("template/native or ontology mismatch", string(raw))
	}
	execution.Cursor = templated.NextCursor
	call(t, agent, "execute_query_template", execution, false)
	execution.Parameters = map[string]any{"region": "west"}
	call(t, agent, "execute_query_template", execution, true)
	execution.Cursor = ""
	execution.Parameters = map[string]any{"region": "east"}
	stored, _ := h.s.Store.Source(id)
	stored.QueryAccessMode = "templates_only"
	update := h.json("PUT", "/api/sources/"+id, stored.Public(), 200)
	call(t, agent, "query_http_api", args, true)
	h.json("POST", "/api/query", map[string]any{"source_id": id, "agent_id": agentID, "operation": "query_http_api", "query": map[string]any{"operation": "customers", "named_params": map[string]any{"region": "east"}}}, 400)
	call(t, agent, "execute_query_template", execution, false)
	// Publication proof must expire after the declared API contract changes.
	update["version"] = "2"
	update = h.json("PUT", "/api/sources/"+id, update, 200)
	call(t, agent, "execute_query_template", execution, true)
	h.json("POST", path+"/trial", map[string]any{"revision": published["revision"], "template_id": template.ID}, 200)
	call(t, agent, "execute_query_template", execution, true)
	h.json("POST", path+"/check-mapping", map[string]any{"revision": published["revision"]}, 200)
	h.json("POST", path+"/publish", map[string]any{"revision": published["revision"]}, 200)
	st, _ := h.s.Store.Semantics(id)
	execution.ExecutionVersion = st.Published.Entries[0].Template.ExecutionVersion
	call(t, agent, "execute_query_template", execution, false)
	// Bad binding definitions never publish or reach the upstream.
	bad := *template.Template
	bad.Parameters = []semantic.Parameter{{Name: "op", Type: "string", Required: true, Pointers: []string{"/operation"}}}
	bad.ExampleJSON = `{"op":"delete"}`
	if _, err := semantic.Bind(bad, nil, true); err == nil {
		t.Fatal("template can replace operation")
	}
	audit, err := h.s.Store.Audits(store.AuditFilter{Source: id}, 100)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(audit)
	if bytes.Contains(raw, []byte(source.Token)) || bytes.Contains(raw, []byte("east")) || bytes.Contains(raw, []byte("9007199254740993")) {
		t.Fatal("audit leaked data")
	}
	var encrypted string
	if err := h.s.Store.DB.QueryRow("SELECT value FROM sources WHERE id=$1", id).Scan(&encrypted); err != nil || strings.Contains(encrypted, source.Token) {
		t.Fatal("source secret not encrypted", err)
	}
	h.json("DELETE", "/api/agents/"+agentID, nil, 200)
	if _, err := h.s.Engine.ExecuteTemplate(context.Background(), model.Principal{AgentID: agentID}, execution); err == nil {
		t.Fatal("revoked reader executed template")
	}
}
