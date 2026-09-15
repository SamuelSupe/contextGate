package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/testpg"
)

func eventOntology() ontology.Definition {
	d := ontology.Empty()
	d.Name = "Shared commerce 业务"
	d.Entities = []ontology.Entity{{ID: "record", Name: "Record", Description: "hidden-ancestor-description"}, {ID: "event", Name: "Event", Parent: "record", Identity: []string{"key", "secret"}}, {ID: "private", Name: "unmapped-private-entity"}}
	d.Properties = []ontology.Property{{ID: "key", Name: "ID", Entity: "record", Type: "integer", Required: true}, {ID: "secret", Name: "unmapped-identity-key", Entity: "event", Type: "string", Required: true}, {ID: "amount", Name: "Amount", Entity: "event", Type: "decimal"}}
	d.Relations = []ontology.Relation{{ID: "private-relation", Name: "hidden-relation", From: "event", To: "private", Directed: true}}
	return d
}

func TestOntologyPublicationScopeAndLifecycle(t *testing.T) {
	h := newHub(t)
	source := h.source()
	path := "/api/sources/" + source + "/semantics"
	d := eventOntology()
	created := h.json("POST", "/api/ontologies", ontologyInput{Definition: d}, 200)
	id := created["id"].(string)
	op := "/api/ontologies/" + id
	h.json("POST", op+"/validate", map[string]any{"revision": created["revision"]}, 200)
	pub := h.json("POST", op+"/publish", map[string]any{"revision": created["revision"]}, 200)
	binding := &ontology.Binding{OntologyID: id, Version: 1, Entities: []ontology.EntityMapping{{Entity: "event", Objects: []ontology.Reference{{Namespace: "main", Object: "events"}}}}, Properties: []ontology.PropertyMapping{{Entity: "event", Property: "key", Reference: &ontology.Reference{Namespace: "main", Object: "events", Field: "id"}}, {Entity: "event", Property: "amount", Reference: &ontology.Reference{Namespace: "main", Object: "events", Field: "amount"}}}}
	template := semanticFixture()
	template.Template.ConceptRefs = []string{ontology.Ref("entity_type", "event"), ontology.PropertyRef("event", "amount")}
	draft := semantic.Empty()
	draft.Ontology = binding
	draft.Entries = []semantic.Entry{template}
	saved := h.json("PUT", path, semanticInput{Snapshot: draft}, 200)
	if usage := h.json("GET", op+"/usage-summary", nil, 200); len(usage) != 0 {
		t.Fatal("draft mapping counted as published query usage", usage)
	}
	a := h.json("POST", "/api/agents", map[string]any{"name": "Ontology reader", "sources": []string{source}, "enabled": true}, 200)
	agent := h.mcp(a["token"].(string))
	call(t, agent, "get_semantic_entry", map[string]any{"source_id": source, "entry_id": ontology.Ref("entity_type", "event")}, true)
	h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": "amount"}, 200)
	h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 400)
	checks := h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 200)
	if len(checks["checks"].([]any)) != 3 {
		t.Fatal(checks)
	}
	saved = h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	checkUsage := func(templates int) {
		t.Helper()
		usage := h.json("GET", op+"/usage-summary", nil, 200)
		event := usage["event"].(map[string]any)
		if len(usage) != 1 || event["sources"] != float64(1) || event["templates"] != float64(templates) {
			t.Fatal("usage must count published mappings and deduplicate executable templates", usage)
		}
	}
	checkUsage(1)
	catalog := h.json("GET", "/api/business-catalog?kind=entity_type", nil, 200)
	catalogRaw, _ := json.Marshal(catalog)
	if catalog["total"] != float64(1) || bytes.Contains(catalogRaw, []byte("unmapped-private-entity")) || bytes.Contains(catalogRaw, []byte("hidden-ancestor-description")) {
		t.Fatal("business catalog must project only mapped ontology definitions", string(catalogRaw))
	}
	detail := h.json("GET", "/api/business-catalog?source_id="+source+"&agent_id="+a["agent"].(map[string]any)["id"].(string)+"&entry_id=ontology:entity_type:event&published_version=1", nil, 200)
	contextEntries := detail["context_entries"].([]any)
	if len(contextEntries) != 2 {
		t.Fatal("mapped and inherited properties missing from concept details", detail)
	}
	detailRaw, _ := json.Marshal(detail)
	for _, hidden := range []string{"unmapped-identity-key", "hidden-ancestor-description", "unmapped-private-entity", "private-relation"} {
		if bytes.Contains(detailRaw, []byte(hidden)) {
			t.Fatal("concept context leaked a hidden definition", hidden)
		}
	}
	entry := call(t, agent, "get_semantic_entry", map[string]any{"source_id": source, "entry_id": ontology.Ref("entity_type", "event")}, false)
	raw, _ := json.Marshal(entry)
	for _, secret := range []string{"unmapped-identity-key", "hidden-ancestor-description", "unmapped-private-entity", "private-relation", "\"identity\"", "\"usage\""} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("scope leak", secret, string(raw))
		}
	}
	call(t, agent, "get_semantic_entry", map[string]any{"source_id": source, "entry_id": ontology.PropertyRef("event", "secret")}, true)
	other := h.json("POST", "/api/agents", map[string]any{"name": "No grants", "sources": []string{}, "enabled": true}, 200)
	call(t, h.mcp(other["token"].(string)), "search_semantics", map[string]any{"source_id": source, "kind": "entity_type"}, true)
	execution := semantic.Execution{SourceID: source, TemplateID: "amount", ExecutionVersion: "1", Parameters: map[string]any{"id": 1}}
	result := call(t, agent, "execute_query_template", execution, false)
	raw, _ = json.Marshal(result.StructuredContent)
	if !bytes.Contains(raw, []byte(`"ontology_context"`)) || !bytes.Contains(raw, []byte(id)) || !bytes.Contains(raw, []byte("9007199254740993")) {
		t.Fatal(string(raw))
	}
	audits, err := h.s.Store.Audits(store.AuditFilter{Source: source}, 10)
	if err != nil || audits[0].OntologyID != id || audits[0].OntologyVersion != "1" {
		t.Fatal(audits, err)
	}
	page := call(t, agent, "search_semantics", map[string]any{"source_id": source, "limit": 1}, false)
	raw, _ = json.Marshal(page.StructuredContent)
	var pv map[string]any
	json.Unmarshal(raw, &pv)
	d.Description = "new-definition-version"
	changed := h.json("PUT", op, map[string]any{"revision": pub["revision"], "definition": d}, 200)
	h.json("PUT", op, map[string]any{"revision": pub["revision"], "definition": d}, 409)
	pub = h.json("POST", op+"/publish", map[string]any{"revision": changed["revision"]}, 200)
	h.json("GET", op+"/versions/2/diff?from=1", nil, 200)
	call(t, agent, "search_semantics", map[string]any{"source_id": source, "limit": 1, "cursor": pv["next_cursor"]}, false)
	h.json("DELETE", op+"/versions/1", map[string]any{"revision": pub["revision"]}, 409)
	draft.Ontology.Version = 2
	rev, _ := strconv.ParseInt(saved["revision"].(string), 10, 64)
	saved = h.json("PUT", path, semanticInput{Revision: rev, Snapshot: draft}, 200)
	h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 200)
	saved = h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	call(t, agent, "search_semantics", map[string]any{"source_id": source, "limit": 1, "cursor": pv["next_cursor"]}, true)
	result = call(t, agent, "execute_query_template", execution, false)
	raw, _ = json.Marshal(result.StructuredContent)
	var native model.Result
	json.Unmarshal(raw, &native)
	if native.OntologyContext == nil || native.OntologyContext.Version != "2" || native.TemplateVersion != "1" {
		t.Fatal(string(raw))
	}
	// Retained semantic publications still reference the old ontology version.
	h.json("DELETE", op+"/versions/1", map[string]any{"revision": pub["revision"]}, 409)
	pub = h.json("POST", op+"/archive", map[string]any{"revision": pub["revision"], "archived": true}, 200)
	call(t, agent, "execute_query_template", execution, false)
	h.json("DELETE", op, map[string]any{"revision": pub["revision"]}, 409)
	otherSource := h.source()
	otherPath := "/api/sources/" + otherSource + "/semantics"
	newDraft := semantic.Empty()
	newDraft.Ontology = binding
	newSaved := h.json("PUT", otherPath, semanticInput{Snapshot: newDraft}, 200)
	h.json("POST", otherPath+"/check-mapping", map[string]any{"revision": newSaved["revision"]}, 400)
	// Opening the same persisted database independently exercises decryption and
	// immutable-version recovery without relying on an in-memory ontology cache.
	reopened, err := store.Open(filepath.Join(h.dir, "config"), testpg.DSN(t, h.dir))
	if err != nil {
		t.Fatal(err)
	}
	v, err := reopened.OntologyVersion(id, 2)
	reopened.Close()
	if err != nil || v.Definition.Description != d.Description {
		t.Fatal(v, err)
	}
	rows, err := h.s.Store.DB.Query("SELECT value FROM ontologies UNION ALL SELECT value FROM ontology_versions")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var sealed string
		if err := rows.Scan(&sealed); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if bytes.Contains([]byte(sealed), []byte("new-definition-version")) || bytes.Contains([]byte(sealed), []byte("hidden-ancestor-description")) {
			rows.Close()
			t.Fatal("plaintext ontology persisted")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}

	src, err := h.s.Store.Source(source)
	if err != nil {
		t.Fatal(err)
	}
	view := publicSource(src)
	view.Password = "changed-template-credential"
	h.json("PUT", "/api/sources/"+source, view, 200)
	checkUsage(0)
	h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": "amount"}, 200)
	checkUsage(0)
	h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 200)
	h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
	checkUsage(1)
}

func TestOntologyDefinitionAndMappingValidation(t *testing.T) {
	for name, change := range map[string]func(*ontology.Definition){
		"cycle":               func(d *ontology.Definition) { d.Entities[0].Parent = "event" },
		"property conflict":   func(d *ontology.Definition) { d.Properties[2].Name = "ID" },
		"identity":            func(d *ontology.Definition) { d.Entities[1].Identity = []string{"absent"} },
		"multivalue identity": func(d *ontology.Definition) { d.Properties[0].Multiple = true },
		"endpoint":            func(d *ontology.Definition) { d.Relations[0].To = "absent" },
		"cardinality": func(d *ontology.Definition) {
			n := 1
			d.Relations[0].FromCardinality = ontology.Cardinality{Min: 2, Max: &n}
		},
		"range": func(d *ontology.Definition) { d.Properties[2].Minimum = "2"; d.Properties[2].Maximum = "1" },
	} {
		t.Run(name, func(t *testing.T) {
			d := eventOntology()
			change(&d)
			if ontology.Validate(d) == nil {
				t.Fatal("invalid definition accepted")
			}
		})
	}
	h := newHub(t)
	source := h.source()
	d := eventOntology()
	c := h.json("POST", "/api/ontologies", ontologyInput{Definition: d}, 200)
	id := c["id"].(string)
	h.json("POST", "/api/ontologies/"+id+"/publish", map[string]any{"revision": c["revision"]}, 200)
	b := &ontology.Binding{OntologyID: id, Version: 1, Entities: []ontology.EntityMapping{{Entity: "event", Objects: []ontology.Reference{{Namespace: "main", Object: "events"}}}}, Properties: []ontology.PropertyMapping{{Entity: "event", Property: "amount", Reference: &ontology.Reference{Namespace: "main", Object: "events", Field: "unknown"}}}}
	draft := semantic.Empty()
	draft.Ontology = b
	path := "/api/sources/" + source + "/semantics"
	saved := h.json("PUT", path, semanticInput{Snapshot: draft}, 200)
	h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 400)
	b.Properties[0].Declared = true
	rev, _ := strconv.ParseInt(saved["revision"].(string), 10, 64)
	saved = h.json("PUT", path, semanticInput{Revision: rev, Snapshot: draft}, 200)
	checked := h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 200)
	raw, _ := json.Marshal(checked)
	if !bytes.Contains(raw, []byte(`"status":"unverified"`)) {
		t.Fatal(string(raw))
	}
	h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
}

func TestSharedOntologyDatabaseMapping(t *testing.T) {
	manifest := os.Getenv("MCPDBHUB_ONTOLOGY_FIXTURES")
	if manifest == "" {
		t.Skip("set MCPDBHUB_ONTOLOGY_FIXTURES to the PostgreSQL/MongoDB commerce fixture manifest")
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name   string       `json:"name"`
		Source model.Source `json:"source"`
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil || len(fixtures) != 2 {
		t.Fatal("expected two commerce fixtures", err)
	}
	h := newHub(t)
	raw, err = os.ReadFile("../../examples/ontologies/commerce.json")
	if err != nil {
		t.Fatal(err)
	}
	var document ontologyInput
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	created := h.json("POST", "/api/ontologies", document, 200)
	h.json("POST", "/api/ontologies/commerce/publish", map[string]any{"revision": created["revision"]}, 200)
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			scoped := *h
			scoped.t = t
			h := &scoped
			fixture.Source.Name = "Commerce " + fixture.Name
			fixture.Source.Enabled = true
			source := h.json("POST", "/api/sources", fixture.Source, 200)["id"].(string)
			path := "/api/sources/" + source + "/semantics"
			raw, err := os.ReadFile("../../examples/ontologies/" + fixture.Source.Kind + "-semantics.json")
			if err != nil {
				t.Fatal(err)
			}
			var draft semantic.Snapshot
			if err = json.Unmarshal(raw, &draft); err != nil {
				t.Fatal(err)
			}
			if fixture.Source.Kind == "postgres" {
				draft.Ontology.Properties = append(draft.Ontology.Properties[:1], draft.Ontology.Properties[2:]...)
			}
			saved := h.json("PUT", path, semanticInput{Snapshot: draft}, 200)
			h.json("POST", path+"/trial", map[string]any{"revision": saved["revision"], "template_id": "customer-orders"}, 200)
			h.json("POST", path+"/check-mapping", map[string]any{"revision": saved["revision"]}, 200)
			h.json("POST", path+"/publish", map[string]any{"revision": saved["revision"]}, 200)
			a := h.json("POST", "/api/agents", map[string]any{"name": "Commerce reader", "sources": []string{source}, "enabled": true}, 200)
			reader := h.mcp(a["token"].(string))
			concept := call(t, reader, "get_semantic_entry", map[string]any{"source_id": source, "entry_id": "ontology:relation_type:places"}, false)
			b, _ := json.Marshal(concept)
			if !bytes.Contains(b, []byte("customer-orders")) {
				t.Fatal("relation did not guide Agent to template", string(b))
			}
			call(t, reader, "get_semantic_entry", map[string]any{"source_id": source, "entry_id": "ontology:property:Customer:customer_name"}, fixture.Source.Kind == "postgres")
			template := draft.Entries[0].Template
			query, err := semantic.Parse(template.QueryJSON)
			if err != nil {
				t.Fatal(err)
			}
			native := query.(map[string]any)
			native["source_id"] = source
			nativeResult := call(t, reader, template.Tool, native, false)
			result := call(t, reader, "execute_query_template", semantic.Execution{SourceID: source, TemplateID: "customer-orders", ExecutionVersion: "1", Parameters: map[string]any{"customer_id": "C-100"}}, false)
			var want, got model.Result
			b, _ = json.Marshal(nativeResult.StructuredContent)
			json.Unmarshal(b, &want)
			b, _ = json.Marshal(result.StructuredContent)
			json.Unmarshal(b, &got)
			if !bytes.Equal(equivalentData(want.Data, template.Tool, native), equivalentData(got.Data, template.Tool, native)) || got.RowCount != 2 || got.OntologyContext == nil || got.OntologyContext.OntologyID != "commerce" || got.OntologyContext.Version != "1" || len(got.OntologyContext.ConceptRefs) != 4 {
				t.Fatal("shared ontology changed native results", string(b))
			}
			empty := call(t, reader, "execute_query_template", semantic.Execution{SourceID: source, TemplateID: "customer-orders", ExecutionVersion: "1", Parameters: map[string]any{"customer_id": "C-404"}}, false)
			b, _ = json.Marshal(empty.StructuredContent)
			got = model.Result{}
			json.Unmarshal(b, &got)
			if got.RowCount != 0 {
				t.Fatal("empty lookup returned data")
			}
			call(t, reader, "execute_query_template", semantic.Execution{SourceID: source, TemplateID: "customer-orders", ExecutionVersion: "1", Parameters: map[string]any{"customer_id": map[string]any{"$ne": nil}}}, true)
		})
	}
	usage, err := h.s.Store.OntologyUsage("commerce")
	if err != nil || len(usage) != 4 {
		t.Fatal("shared version did not retain independent source bindings", usage, err)
	}
}
