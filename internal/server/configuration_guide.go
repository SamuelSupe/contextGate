package server

import "github.com/SamuelSupe/contextGate/internal/ontology"

func (s *Server) configurationGuide() any {
	return map[string]any{
		"scope":       "All data source configurations, semantic drafts, query templates and shared ontology drafts. Separate from query Agent permissions.",
		"publication": "administrator_only",
		"review_urls": map[string]string{"sources": s.PublicURL + "/sources", "ontologies": s.PublicURL + "/ontologies", "credentials": s.PublicURL + "/settings#configuration-mcp"},
		"workflow": []string{
			"1. List supported databases and configured sources. Reuse existing IDs. Ask the user for business meaning and read-only database credentials; never invent either.",
			"2. Create or update a source. Source edits are immediately effective; connection changes expire trial evidence and cancel affected queries. Query Agent grants are managed separately in the UI.",
			"3. Test the connection, then discover namespaces, objects and fields. Read permission evidence literally; connected is not the same as verified read-only permissions.",
			"4. Get the semantic draft and import selected structure objects. Add business terms, field descriptions and metrics from user-provided definitions, without sampling data to infer semantics.",
			"5. Reuse a published ontology or create/save/validate a draft. For a new or changed ontology, give the administrator its ID, revision and review URL to publish in the UI. Then read that immutable version.",
			"6. Save the semantic snapshot with an ontology binding to that published version. Map only this source's entities, fields and relations. Mark undiscoverable fields declared, which remains unverified. Mapping does not generate queries.",
			"7. Upsert native read-only query templates and explicit JSON Pointer parameter bindings. Add concept_refs and examples. Validate the semantic draft, check the ontology mapping, and trial every enabled template with current revision.",
			"8. Return a concise change summary, current semantic revision, validation status and source review URL. Administrator reviews and publishes Semantics in the UI, then grants the intended query Agent access.",
			"9. The query Agent connects to /mcp using its own query credential and discovers published semantics and templates. Configuration tokens are not query tokens.",
		},
		"editing_rules": []string{
			"Read current revision before each edit; reuse the revision returned by the previous edit. On conflict, reload and merge intentionally; never blindly overwrite.",
			"Whole-draft saves replace content. Preserve unrelated entries, ontology mappings and stable IDs; use entry-level upsert for small changes.",
			"Template query_json, example_json and default_json are JSON-encoded strings so integers and decimals remain exact. Parameter types: string, integer, number, boolean, null, object, array (subject to native slot restrictions).",
			"SQL/CQL/Cypher/InfluxDB use native parameter slots; Redis binds fixed command argument elements; MongoDB/Search bind document value locations. Never bind query text, commands, object names, keys or operators, and never interpolate strings.",
			"The tool descriptions and database support catalog cover all seven query families. Trial uses the existing read-only engine. Template trial evidence excludes returned rows.",
			"Never store credentials in descriptions, options, query text or sample parameters. Only source password/token fields accept secrets, and they are never returned.",
			"Configuration tools cannot publish, delete data sources or ontologies, alter Agent grants, create their own credentials, or change system settings. Use administrator UI for those actions.",
		},
		"limits": map[string]any{"per_credential_concurrency": 2, "global_configuration_concurrency": 8, "call_timeout_seconds": 120, "response_max_bytes": 4 << 20, "request_max_bytes": 1 << 20, "token_max_days": 30},
		"examples": map[string]any{
			"create_data_source": map[string]any{"configuration": map[string]any{"name": "Retail read-only", "kind": "postgres", "host": "db.example.internal", "port": 5432, "database": "retail", "username": "retail_reader", "password": "REPLACE_WITH_READ_ONLY_SECRET", "tls_mode": "verify", "enabled": true, "query_access_mode": "templates_only", "auth_mode": "password"}},
			"create_ontology":    map[string]any{"id": "retail", "definition": ontology.Definition{FormatVersion: 1, Name: "Retail", Entities: []ontology.Entity{{ID: "Customer", Name: "Customer", Identity: []string{"customer_id"}}}, Properties: []ontology.Property{{ID: "customer_id", Entity: "Customer", Name: "Customer ID", Type: "integer", Required: true}}, Relations: []ontology.Relation{}}},
			"ontology_binding":   ontology.Binding{OntologyID: "retail", Version: 1, Entities: []ontology.EntityMapping{{Entity: "Customer", Objects: []ontology.Reference{{Namespace: "public", Object: "customers"}}}}, Properties: []ontology.PropertyMapping{{Entity: "Customer", Property: "customer_id", Reference: &ontology.Reference{Namespace: "public", Object: "customers", Field: "id"}}}, Relations: []ontology.RelationMapping{}},
			"template_entry":     map[string]any{"id": "customer-by-id", "kind": "template", "name": "Customer by ID", "template": map[string]any{"enabled": true, "tool": "query_sql", "query_json": `{"query":"SELECT id, name FROM public.customers WHERE id = $1","params":[0]}`, "parameters": []any{map[string]any{"name": "customer_id", "type": "integer", "required": true, "pointers": []string{"/params/0"}}}, "example_json": `{"customer_id":1}`, "concept_refs": []string{"ontology:entity_type:Customer", ontology.PropertyRef("Customer", "customer_id")}, "result_description": "Native customer rows; an unknown customer produces an empty result."}},
		},
	}
}
