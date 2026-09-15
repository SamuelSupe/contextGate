package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/testpg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func semanticFixture() semantic.Entry {
	return semantic.Entry{ID: "amount", Kind: "template", Name: "Event amount", Description: "金额查询", Template: &semantic.Template{Enabled: true, Tool: "query_sql", QueryJSON: `{"query":"SELECT amount FROM events WHERE id = ?","params":[0]}`, Parameters: []semantic.Parameter{{Name: "id", Type: "integer", Required: true, Pointers: []string{"/params/0"}, Minimum: "1", Maximum: "100"}}, ExampleJSON: `{"id":1}`}}
}

func TestBusinessCatalogAndPublicationReview(t *testing.T) {
	h := newHub(t)
	id, other := h.source(), h.source()
	path := "/api/sources/" + id + "/semantics"
	a := h.json("POST", "/api/agents", map[string]any{"name": "Catalog reader", "sources": []string{id}, "enabled": true}, 200)
	agentID := a["agent"].(map[string]any)["id"].(string)
	draft := semantic.Empty()
	draft.Entries = []semantic.Entry{semanticFixture()}
	for n := range 22 {
		draft.Entries = append(draft.Entries, semantic.Entry{ID: fmt.Sprintf("term-%02d", n), Kind: "term", Name: fmt.Sprintf("Business term %02d", n), Aliases: []string{"客户"}})
	}
	saved := h.json("PUT", path, semanticInput{Snapshot: draft}, 200)
	impact := h.json("GET", path+"/impact?revision="+saved["revision"].(string), nil, 200)
	if impact["can_publish"] != false || len(impact["issues"].([]any)) == 0 || len(impact["agents"].([]any)) != 1 {
		t.Fatal("review must expose blockers and source grants", impact)
	}
	if page := h.json("GET", "/api/business-catalog?agent_id="+agentID, nil, 200); page["total"] != float64(0) {
		t.Fatal("business catalog leaked a draft", page)
	}
	h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": "amount"}, 200)
	impact = h.json("GET", path+"/impact?revision="+saved["revision"].(string), nil, 200)
	if impact["can_publish"] != true {
		t.Fatal(impact)
	}
	saved = h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	otherPath := "/api/sources/" + other + "/semantics"
	secret := h.json("PUT", otherPath, semanticInput{Snapshot: semantic.Snapshot{FormatVersion: 2, Entries: []semantic.Entry{{ID: "secret", Name: "other-source-private", Kind: "term"}}}}, 200)
	h.json("POST", otherPath+"/publish", map[string]any{"revision": secret["revision"]}, 200)
	base := "/api/business-catalog?agent_id=" + agentID
	queryPage := h.json("GET", base+"&view=queries", nil, 200)
	if queryPage["total"] != float64(1) || queryPage["entries"].([]any)[0].(map[string]any)["id"] != "amount" {
		t.Fatal("query view should include only executable business entries", queryPage)
	}
	h.json("GET", base+"&offset=20&revision="+queryPage["revision"].(string), nil, 409)
	slots := h.json("POST", path+"/bindings", map[string]any{"query_json": draft.Entries[0].Template.QueryJSON}, 200)
	if len(slots["slots"].([]any)) != 1 || slots["slots"].([]any)[0] != "/params/0" {
		t.Fatal(slots)
	}
	page := h.json("GET", base, nil, 200)
	if page["total"] != float64(23) || len(page["entries"].([]any)) != 20 {
		t.Fatal(page)
	}
	raw, _ := json.Marshal(page)
	if bytes.Contains(raw, []byte("other-source-private")) || bytes.Contains(raw, []byte("SELECT amount")) {
		t.Fatal("summary scope or query text leak", string(raw))
	}
	next := h.json("GET", base+"&offset=20&revision="+page["revision"].(string), nil, 200)
	if len(next["entries"].([]any)) != 3 {
		t.Fatal(next)
	}
	h.json("GET", base+"&offset=20", nil, 409)
	h.json("GET", base+"&source_id="+other+"&entry_id=secret", nil, 404)
	if alias := h.json("GET", base+"&keyword=%E5%AE%A2%E6%88%B7", nil, 200); alias["total"] != float64(22) {
		t.Fatal(alias)
	}
	entry := h.json("GET", base+"&source_id="+id+"&entry_id=amount&published_version=1", nil, 200)
	if entry["executable"] != true {
		t.Fatal(entry)
	}
	for _, endpoint := range []string{base, path + "/impact?revision=" + saved["revision"].(string)} {
		req, _ := http.NewRequest("GET", h.http.URL+endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+a["token"].(string))
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatal("Agent accessed administrator workflow", response.StatusCode)
		}
	}
	rev, _ := strconv.ParseInt(saved["revision"].(string), 10, 64)
	draft.Entries[0].Description = "updated business explanation"
	saved = h.json("PUT", path, semanticInput{Revision: rev, Snapshot: draft}, 200)
	h.json("GET", path+"/impact?revision="+strconv.FormatInt(rev, 10), nil, 409)
	impact = h.json("GET", path+"/impact?revision="+saved["revision"].(string), nil, 200)
	if impact["can_publish"] != true || impact["interrupted_templates"] != float64(0) {
		t.Fatal("description edit invalidated execution", impact)
	}
	// Saving drafts does not invalidate published discovery pages.
	h.json("GET", base+"&offset=20&revision="+page["revision"].(string), nil, 200)
	saved = h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	h.json("GET", base+"&offset=20&revision="+page["revision"].(string), nil, 409)
	h.json("GET", base+"&source_id="+id+"&entry_id=amount&published_version=1", nil, 409)
	rev, _ = strconv.ParseInt(saved["revision"].(string), 10, 64)
	draft.Entries[0].Template.QueryJSON = `{"query":"SELECT amount FROM events WHERE id = ? ORDER BY id","params":[0]}`
	saved = h.json("PUT", path, semanticInput{Revision: rev, Snapshot: draft}, 200)
	impact = h.json("GET", path+"/impact?revision="+saved["revision"].(string), nil, 200)
	if impact["can_publish"] != false || impact["interrupted_templates"] != float64(1) {
		t.Fatal("execution change omitted from review", impact)
	}
	h.json("DELETE", "/api/agents/"+agentID, nil, 200)
	h.json("GET", base, nil, 404)
}

func TestSemanticPublicationAuthorizationAndPersistence(t *testing.T) {
	h := newHub(t)
	id := h.source()
	path := "/api/sources/" + id + "/semantics"
	a := h.json("POST", "/api/agents", map[string]any{"name": "Semantic reader", "sources": []string{id}, "enabled": true}, 200)
	agentID := a["agent"].(map[string]any)["id"].(string)
	session := h.mcp(a["token"].(string))
	empty := h.json("GET", path, nil, 200)
	if empty["published_version"] != "0" {
		t.Fatal(empty)
	}
	draft := semantic.Snapshot{FormatVersion: 1, Overview: "draft-secret-only", Entries: []semantic.Entry{semanticFixture(), {ID: "amount-definition", Kind: "metric", Name: "金额", Description: "金额的定义", TemplateID: "amount", Grain: "event"}}}
	saved := h.json("PUT", path, semanticInput{Revision: 0, Snapshot: draft}, 200)
	revision := saved["revision"]
	h.json("PUT", path, semanticInput{Revision: 0, Snapshot: draft}, 409)
	hidden := call(t, session, "search_semantics", map[string]any{"source_id": id}, false)
	b, _ := json.Marshal(hidden)
	if bytes.Contains(b, []byte("draft-secret-only")) || bytes.Contains(b, []byte("Event amount")) {
		t.Fatal("draft leaked")
	}
	call(t, session, "get_semantic_entry", map[string]any{"source_id": id, "entry_id": "amount"}, true)
	h.json("POST", path+"/publish", map[string]any{"revision": revision}, 400)
	h.json("POST", path+"/trial", map[string]any{"revision": revision, "template_id": "amount"}, 200)
	published := h.json("POST", path+"/publish", map[string]any{"revision": revision}, 200)
	checkReady := func(want float64) {
		t.Helper()
		ready := h.json("GET", "/api/sources/"+id+"/readiness", nil, 200)
		if ready["executable_templates"] != want {
			t.Fatalf("incorrect published readiness: %v", ready)
		}
	}
	checkReady(1)

	execution := map[string]any{"source_id": id, "template_id": "amount", "execution_version": "1", "parameters": map[string]any{"id": 1}}
	result := call(t, session, "execute_query_template", execution, false)
	b, _ = json.Marshal(result)
	if !bytes.Contains(b, []byte(`9007199254740993`)) || !bytes.Contains(b, []byte(`"template_version":"1"`)) {
		t.Fatal("lossless template result missing", string(b))
	}
	page := call(t, session, "search_semantics", map[string]any{"source_id": id, "limit": 1}, false)
	pageJSON, _ := json.Marshal(page.StructuredContent)
	var pageValue map[string]any
	json.Unmarshal(pageJSON, &pageValue)
	cursor := pageValue["next_cursor"].(string)
	if cursor == "" {
		t.Fatal("semantic pagination missing")
	}
	call(t, session, "search_semantics", map[string]any{"source_id": id, "limit": 1, "cursor": cursor}, false)
	for _, params := range []any{map[string]any{"id": "1 OR 1=1"}, map[string]any{"id": 0}, map[string]any{"id": 1, "query": "DELETE"}, map[string]any{"id": map[string]any{"$gt": 0}}} {
		bad := map[string]any{"source_id": id, "template_id": "amount", "execution_version": "1", "parameters": params}
		call(t, session, "execute_query_template", bad, true)
	}
	denied := h.json("POST", "/api/agents", map[string]any{"name": "No semantic grants", "sources": []string{}, "enabled": true}, 200)
	other := h.mcp(denied["token"].(string))
	for _, tool := range []string{"search_semantics", "get_semantic_entry", "execute_query_template"} {
		args := map[string]any{"source_id": id}
		if tool == "get_semantic_entry" {
			args["entry_id"] = "amount"
		}
		if tool == "execute_query_template" {
			args = execution
		}
		call(t, other, tool, args, true)
	}
	req, _ := http.NewRequest("GET", h.http.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+a["token"].(string))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("Agent read draft administration")
	}
	oldCSRF := h.csrf
	h.csrf = "invalid"
	h.json("POST", path+"/publish", map[string]any{"revision": published["revision"]}, 403)
	h.csrf = oldCSRF
	// Restriction is enforced by the shared engine, including Agent identity preview.
	source, _ := h.s.Store.Source(id)
	view := publicSource(source)
	view.QueryAccessMode = "templates_only"
	h.json("PUT", "/api/sources/"+id, view, 200)
	call(t, session, "query_sql", map[string]any{"source_id": id, "query": "SELECT 1"}, true)
	h.json("POST", "/api/query", map[string]any{"source_id": id, "operation": "query_sql", "agent_id": agentID, "query": map[string]any{"query": "SELECT 1"}}, 400)
	h.json("POST", "/api/query", map[string]any{"source_id": id, "operation": "query_sql", "query": map[string]any{"query": "SELECT 1"}}, 200)
	call(t, session, "describe_object", map[string]any{"source_id": id, "object": "events"}, false)
	call(t, session, "execute_query_template", execution, false)
	// Cosmetic source and semantic changes preserve execution versions and evidence.
	source, _ = h.s.Store.Source(id)
	view = publicSource(source)
	view.Name = "Renamed"
	view.Limits.MaxRows = 500
	h.json("PUT", "/api/sources/"+id, view, 200)
	call(t, session, "execute_query_template", execution, false)
	draft.Overview = "Published domain"
	draft.Entries[0].Description = "New description"
	st, _ := h.s.Store.Semantics(id)
	saved = h.json("PUT", path, semanticInput{Revision: st.Revision, Snapshot: draft}, 200)
	published = h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	call(t, session, "execute_query_template", execution, false)
	call(t, session, "search_semantics", map[string]any{"source_id": id, "limit": 1, "cursor": cursor}, true)
	// Connection edits expire both the trial and the published approval.
	source, _ = h.s.Store.Source(id)
	view = publicSource(source)
	view.Password = "new-private-credential"
	h.json("PUT", "/api/sources/"+id, view, 200)
	checkReady(0)
	call(t, session, "execute_query_template", execution, true)
	h.json("POST", path+"/publish", map[string]any{"revision": published["revision"]}, 400)
	h.json("POST", path+"/trial", map[string]any{"revision": published["revision"], "template_id": "amount"}, 200)
	checkReady(0)
	call(t, session, "execute_query_template", execution, true)
	published = h.json("POST", path+"/publish", map[string]any{"revision": published["revision"]}, 200)
	checkReady(1)
	call(t, session, "execute_query_template", execution, true)
	execution["execution_version"] = published["published_version"]
	call(t, session, "execute_query_template", execution, false)
	// Failed trials cannot publish dangerous definitions; source contents stay intact.
	st, _ = h.s.Store.Semantics(id)
	unsafe := semanticFixture()
	unsafe.Template.QueryJSON = `{"query":"DELETE FROM events","params":[0]}`
	saved = h.json("PUT", path, semanticInput{Revision: st.Revision, Snapshot: semantic.Snapshot{FormatVersion: 1, Entries: []semantic.Entry{unsafe}}}, 200)
	h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": "amount"}, 400)
	h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 400)
	call(t, session, "execute_query_template", execution, false)
	h.json("POST", path+"/discard", map[string]any{"revision": saved["revision"]}, 200)
	// The exported document cannot carry trial evidence or connection credentials.
	export := h.json("GET", path+"/export", nil, 200)
	b, _ = json.Marshal(export)
	for _, secret := range []string{"new-private-credential", "db-secret-never-echo", "checked_at", "connection_revision"} {
		if bytes.Contains(b, []byte(secret)) {
			t.Fatal("private evidence or connection exported")
		}
	}
	// Fresh store instances recover snapshots and proofs; all contents are encrypted on disk.
	reopened, err := store.Open(filepath.Join(h.dir, "config"), testpg.DSN(t, h.dir))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Semantics(id)
	reopened.Close()
	if err != nil || restored.PublishedVersion < 3 {
		t.Fatal("semantic recovery failed", err)
	}
	rows, err := h.s.Store.DB.Query("SELECT value FROM semantics_entries")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var value string
		rows.Scan(&value)
		if strings.Contains(value, "SELECT") || strings.Contains(value, "金额") {
			t.Fatal("plaintext semantics stored")
		}
	}
	rows.Close()
	audits, err := h.s.Store.Audits(store.AuditFilter{Source: id}, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range audits {
		if a.Operation == "execute_query_template" && a.ErrorCode == "" {
			found = true
			if a.TemplateID != "amount" || a.TemplateVersion == "" {
				t.Fatal("template audit metadata missing")
			}
		}
	}
	if !found {
		t.Fatal("template audit missing")
	}
	h.json("DELETE", "/api/sources/"+id, nil, 200)
	for _, table := range []string{"semantics_state", "semantics_entries", "semantics_evidence"} {
		var count int
		if err := h.s.Store.DB.QueryRow("SELECT count(*) FROM "+table+" WHERE source_id=$1", id).Scan(&count); err != nil || count != 0 {
			t.Fatal("source semantics not atomically deleted", table, err)
		}
	}
	call(t, session, "execute_query_template", execution, true)
}

// Each matrix query is authored and trialled as a template, then compared through
// the public MCP API with the equivalent native call, including cursor pages.
func matrixTemplate(t *testing.T, h *hubTest, session *mcp.ClientSession, source, tool string, native map[string]any, want *model.Result, denied bool) {
	t.Helper()
	query := map[string]any{}
	input := semantic.Execution{SourceID: source, TemplateID: "matrix-template", Parameters: map[string]any{}}
	for key, value := range native {
		switch key {
		case "source_id", "cursor":
		case "max_rows":
			input.MaxRows = intValue(value)
		case "max_bytes":
			input.MaxBytes = intValue(value)
		case "timeout_seconds":
			input.TimeoutSeconds = intValue(value)
		default:
			query[key] = value
		}
	}
	b, _ := json.Marshal(query)
	template := semantic.Template{Enabled: true, Tool: tool, QueryJSON: string(b), Parameters: []semantic.Parameter{}, ExampleJSON: `{}`}
	add := func(name, pointer string, value any) {
		typ := ""
		switch value.(type) {
		case string:
			typ = "string"
		case json.Number:
			typ = "number"
		case float64:
			typ = "number"
		case bool:
			typ = "boolean"
		case []any:
			typ = "array"
		case map[string]any:
			typ = "object"
		case nil:
			typ = "null"
		default:
			typ = "integer"
		}
		wire, _ := json.Marshal(value)
		parsed, _ := semantic.Parse(string(wire))
		p := semantic.Parameter{Name: name, Type: typ, Required: true, Pointers: []string{pointer}}
		candidate := template
		candidate.Parameters = append(append([]semantic.Parameter{}, template.Parameters...), p)
		values := map[string]any{}
		for k, v := range input.Parameters {
			values[k] = v
		}
		values[name] = parsed
		if _, err := semantic.Bind(candidate, values, false); err == nil {
			template = candidate
			input.Parameters = values
		}
	}
	if params, ok := query["params"].([]any); ok {
		for i, v := range params {
			add(fmt.Sprintf("p%d", i), fmt.Sprintf("/params/%d", i), v)
		}
	}
	if params, ok := query["named_params"].(map[string]any); ok {
		for k, v := range params {
			add(k, "/named_params/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"), v)
		}
	}
	if tool == "query_redis" {
		if args, ok := query["args"].([]any); ok && len(args) > 0 {
			add("key", "/args/0", args[0])
		}
	}
	var leaves func(any, string)
	leaves = func(v any, path string) {
		switch node := v.(type) {
		case map[string]any:
			for k, v := range node {
				leaves(v, path+"/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"))
			}
		case []any:
			for i, v := range node {
				leaves(v, fmt.Sprintf("%s/%d", path, i))
			}
		default:
			add(fmt.Sprintf("value%d", len(template.Parameters)), path, node)
		}
	}
	for _, key := range []string{"filter", "pipeline", "body"} {
		if v, exists := query[key]; exists {
			leaves(v, "/"+key)
		}
	}
	examples, _ := json.Marshal(input.Parameters)
	template.ExampleJSON = string(examples)
	st, err := h.s.Store.Semantics(source)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/sources/" + source + "/semantics"
	template.ConceptRefs = []string{ontology.Ref("entity_type", "record")}
	draft := semantic.Snapshot{FormatVersion: semantic.FormatVersion, Ontology: st.Draft.Ontology, Entries: []semantic.Entry{{ID: input.TemplateID, Kind: "template", Name: "Matrix native equivalence", Template: &template}}}
	saved := h.json("PUT", path, semanticInput{Revision: st.Revision, Snapshot: draft}, 200)
	status := 200
	if denied {
		status = 400
	}
	h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": input.TemplateID}, status)
	if denied {
		h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 400)
		return
	}
	h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 200)
	published := h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	state, _ := h.s.Store.Semantics(source)
	input.ExecutionVersion = state.Published.Entries[0].Template.ExecutionVersion
	result := call(t, session, "execute_query_template", input, false)
	raw, _ := json.Marshal(result.StructuredContent)
	var got model.Result
	json.Unmarshal(raw, &got)
	actual := equivalentData(got.Data, tool, query)
	expected := equivalentData(want.Data, tool, query)
	if !bytes.Equal(actual, expected) || got.Format != want.Format || got.RowCount != want.RowCount || got.Truncated != want.Truncated || got.SemanticVersion != published["published_version"] || got.OntologyContext == nil || got.OntologyContext.Version != "1" || len(got.OntologyContext.ConceptRefs) != 1 {
		t.Fatalf("template differs from native: got %s want %s", actual, expected)
	}
	// Page both paths in lockstep; opaque template and native cursors are distinct.
	nativeCursor := want.NextCursor
	for pages := 0; got.NextCursor != ""; pages++ {
		if pages > 100 || nativeCursor == "" {
			t.Fatal("template cursor diverged")
		}
		input.Cursor = got.NextCursor
		rawQuery := map[string]any{}
		for k, v := range native {
			rawQuery[k] = v
		}
		rawQuery["source_id"] = source
		rawQuery["cursor"] = nativeCursor
		n := call(t, session, tool, rawQuery, false)
		b, _ := json.Marshal(n.StructuredContent)
		var next model.Result
		json.Unmarshal(b, &next)
		nativeCursor = next.NextCursor
		r := call(t, session, "execute_query_template", input, false)
		b, _ = json.Marshal(r.StructuredContent)
		got = model.Result{}
		json.Unmarshal(b, &got)
		a, _ := json.Marshal(got.Data)
		b, _ = json.Marshal(next.Data)
		if !bytes.Equal(a, b) {
			t.Fatal("template pagination differs")
		}
	}
	if nativeCursor != "" {
		t.Fatal("template stopped before native pagination ended")
	}
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func TestStructureImportPreservesDescriptionsAndRejectsStaleDraft(t *testing.T) {
	h := newHub(t)
	id := h.source()
	path := "/api/sources/" + id + "/semantics"
	body := map[string]any{"revision": "0", "objects": []semantic.Reference{{Namespace: "main", Object: "events"}}}
	saved := h.json("POST", path+"/import-structure", body, 200)
	st, _ := h.s.Store.Semantics(id)
	if len(st.Draft.Entries) != 4 {
		t.Fatal("object and field skeleton missing", st.Draft)
	}
	st.Draft.Entries[0].Description = "Manual business meaning"
	saved = h.json("PUT", path, semanticInput{Revision: st.Revision, Snapshot: st.Draft}, 200)
	body["revision"] = saved["revision"]
	h.json("POST", path+"/import-structure", body, 200)
	st, _ = h.s.Store.Semantics(id)
	if len(st.Draft.Entries) != 4 || st.Draft.Entries[0].Description != "Manual business meaning" {
		t.Fatal("reimport replaced description")
	}
	h.json("POST", path+"/import-structure", body, 409)
}

func localTemplateMatrix(t *testing.T, h *hubTest, session *mcp.ClientSession, id string) {
	matrixOntology(t, h, id, "main", "events")
	for _, tc := range []struct {
		q      map[string]any
		denied bool
	}{
		{map[string]any{"query": "WITH base AS (SELECT id FROM events WHERE id>=?) SELECT id,sum(id) OVER () FROM base ORDER BY id", "params": []any{1}}, false},
		{map[string]any{"query": "SELECT CAST(? AS BIGINT)", "params": []any{int64(9007199254740993)}}, false},
		{map[string]any{"query": "SELECT * FROM events WHERE id=?", "params": []any{99}}, false},
		{map[string]any{"query": "SELECT * FROM events ORDER BY id", "max_rows": 1}, false},
		{map[string]any{"query": "SELECT missing_column FROM events"}, true},
		{map[string]any{"query": "DELETE FROM events"}, true},
		{map[string]any{"query": "SELECT 1; DELETE FROM events"}, true},
	} {
		tc.q["source_id"] = id
		response := call(t, session, "query_sql", tc.q, tc.denied)
		var want model.Result
		if !tc.denied {
			b, _ := json.Marshal(response.StructuredContent)
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
		}
		matrixTemplate(t, h, session, id, "query_sql", tc.q, &want, tc.denied)
	}
}

func equivalentData(data []any, tool string, query map[string]any) []byte {
	if tool == "query_mongodb" && query["operation"] == "distinct" {
		values := []string{}
		for _, v := range data {
			b, _ := json.Marshal(v)
			values = append(values, string(b))
		}
		sort.Strings(values)
		b, _ := json.Marshal(values)
		return b
	}
	b, _ := json.Marshal(data)
	return b
}

func matrixOntology(t *testing.T, h *hubTest, source, namespace, object string) {
	t.Helper()
	d := ontology.Empty()
	d.Name = "Matrix business ontology"
	d.Entities = []ontology.Entity{{ID: "record", Name: "Record"}}
	created := h.json("POST", "/api/ontologies", ontologyInput{Definition: d}, 200)
	id := created["id"].(string)
	h.json("POST", "/api/ontologies/"+id+"/publish", map[string]any{"revision": created["revision"]}, 200)
	st, err := h.s.Store.Semantics(source)
	if err != nil {
		t.Fatal(err)
	}
	st.Draft.Ontology = &ontology.Binding{OntologyID: id, Version: 1, Entities: []ontology.EntityMapping{{Entity: "record", Objects: []ontology.Reference{{Namespace: namespace, Object: object}}}}}
	h.json("PUT", "/api/sources/"+source+"/semantics", semanticInput{Revision: st.Revision, Snapshot: st.Draft}, 200)
}

func TestSemanticHistoryRestoreAndRegressionGates(t *testing.T) {
	h := newHub(t)
	id := h.source()
	path := "/api/sources/" + id + "/semantics"
	one, zero := 1, 0
	entry := semanticFixture()
	entry.Template.QueryJSON = `{"query":"SELECT 'ok' AS status WHERE ? = 1","params":[0]}`
	entry.Template.Tests = []semantic.RegressionCase{
		{Name: "Matching row", ParametersJSON: `{"id":1}`, MinRows: &one, MaxRows: &one, Columns: []model.Column{{Name: "status"}}, Values: []semantic.ExpectedValue{{Pointer: "/0/0", ExpectedJSON: `"ok"`}}},
		{Name: "Empty result", ParametersJSON: `{"id":2}`, MinRows: &zero, MaxRows: &zero},
	}
	draft := semantic.Snapshot{FormatVersion: semantic.FormatVersion, Entries: []semantic.Entry{entry}}
	saved := h.json("PUT", path, semanticInput{Snapshot: draft}, 200)
	trial := h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": "amount"}, 200)
	if trial["valid"] != true || len(trial["report"].(map[string]any)["cases"].([]any)) != 3 {
		t.Fatal(trial)
	}
	published := h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	v1 := h.json("GET", path+"/versions/1", nil, 200)
	if v1["version"].(map[string]any)["snapshot"] == nil {
		t.Fatal("publication history missing snapshot")
	}
	agent := h.json("POST", "/api/agents", map[string]any{"name": "History reader", "sources": []string{id}, "enabled": true}, 200)
	session := h.mcp(agent["token"].(string))
	execution := map[string]any{"source_id": id, "template_id": "amount", "execution_version": "1", "parameters": map[string]any{"id": 1}}
	call(t, session, "execute_query_template", execution, false)
	details := call(t, session, "get_semantic_entry", map[string]any{"source_id": id, "entry_id": "amount"}, false)
	raw, _ := json.Marshal(details.StructuredContent)
	if bytes.Contains(raw, []byte("Matching row")) {
		t.Fatal("administrator regression parameters leaked to Agent")
	}
	two := 2
	draft.Entries[0].Template.Tests[0].MinRows = &two
	draft.Entries[0].Template.Tests[0].MaxRows = &two
	revision, _ := strconv.ParseInt(published["revision"].(string), 10, 64)
	saved = h.json("PUT", path, semanticInput{Revision: revision, Snapshot: draft}, 200)
	h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": "amount"}, 400)
	h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 400)
	status := h.json("GET", path, nil, 200)
	if status["validation"].([]any)[0].(map[string]any)["status"] != "regression_failed" {
		t.Fatal(status)
	}
	call(t, session, "execute_query_template", execution, false)
	restored := h.json("POST", path+"/restore", map[string]any{"revision": saved["revision"], "version": "1"}, 200)
	h.json("POST", path+"/restore", map[string]any{"revision": saved["revision"], "version": "1"}, 409)
	// A restore must not reuse old success evidence, or disrupt the current publication.
	h.json("POST", path+"/publish", map[string]any{"revision": restored["revision"]}, 400)
	call(t, session, "execute_query_template", execution, false)
	h.json("POST", path+"/trial-all", map[string]any{"revision": restored["revision"]}, 200)
	published = h.json("POST", path+"/publish", map[string]any{"revision": restored["revision"]}, 200)
	if published["published_version"] != "2" {
		t.Fatal(published)
	}
	// The restored query definition is unchanged, so execution versions stay stable.
	call(t, session, "execute_query_template", execution, false)
	other := h.source()
	h.json("GET", "/api/sources/"+other+"/semantics/versions/1", nil, 404)
	for _, route := range []string{path + "/versions", path + "/versions/1", "/api/health", "/api/settings/diagnostics"} {
		req, _ := http.NewRequest("GET", h.http.URL+route, nil)
		req.Header.Set("Authorization", "Bearer "+agent["token"].(string))
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatal("Agent accessed administration", route)
		}
	}
	reopened, err := store.Open(filepath.Join(h.dir, "config"), testpg.DSN(t, h.dir))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	versions, err := reopened.SemanticVersions(id, 0)
	if err != nil || len(versions) != 2 {
		t.Fatal("history did not survive reopening", versions, err)
	}
	h.json("DELETE", "/api/sources/"+id, nil, 200)
	versions, err = reopened.SemanticVersions(id, 0)
	if err != nil || len(versions) != 0 {
		t.Fatal("source history not deleted atomically", err)
	}
}
