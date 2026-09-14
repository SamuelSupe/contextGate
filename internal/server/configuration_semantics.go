package server

import (
	"context"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type configurationDraft struct {
	SourceID string `json:"source_id"`
	Revision string `json:"revision" jsonschema:"Current semantic draft revision, returned by get_semantic_draft or the preceding edit."`
}

func (s *Server) configurationSemanticTools(server *mcp.Server, agent model.ConfigurationAgent) {
	configurationTool(s, server, agent, "get_semantic_draft", "Read current draft, published snapshot, revision, template validation and ontology mapping status. Only published content is visible to query Agents.", true, func(ctx context.Context, in configurationSourceID) (any, error) {
		return configurationHandler(ctx, s.semantics, "GET /api/sources/{id}/semantics", map[string]string{"id": in.SourceID}, nil)
	})
	configurationTool(s, server, agent, "save_semantic_draft", "Replace the complete semantic draft, including ontology binding. Preserve existing entries. Use format_version 2; binding pins an already published ontology version. Does not publish.", false, func(ctx context.Context, in struct {
		configurationDraft
		Snapshot semantic.Snapshot `json:"snapshot"`
	}) (any, error) {
		return configurationHandler(ctx, s.saveSemantics, "PUT /api/sources/{id}/semantics", map[string]string{"id": in.SourceID}, map[string]any{"revision": in.Revision, "snapshot": in.Snapshot})
	})
	configurationTool(s, server, agent, "upsert_semantic_entry", "Create or replace one draft term, object, field, relationship, metric or query template. Query and example JSON are strings to preserve exact numbers. Bind values through allowed JSON Pointers, never string interpolation. No publication.", false, func(ctx context.Context, in struct {
		configurationDraft
		Entry semantic.Entry `json:"entry"`
	}) (any, error) {
		if in.Entry.ID == "" {
			return nil, model.Fail("invalid_input", "An entry ID is required")
		}
		return configurationHandler(ctx, s.saveSemanticEntry, "PUT /api/sources/{id}/semantics/entries/{entry}", map[string]string{"id": in.SourceID, "entry": in.Entry.ID}, map[string]any{"revision": in.Revision, "entry": in.Entry})
	})
	configurationTool(s, server, agent, "remove_semantic_entry", "Remove a draft entry by ID and revision. Published content remains unchanged until administrator publication.", false, func(ctx context.Context, in struct {
		configurationDraft
		EntryID string `json:"entry_id"`
	}) (any, error) {
		return configurationHandler(ctx, s.saveSemanticEntry, "DELETE /api/sources/{id}/semantics/entries/{entry}", map[string]string{"id": in.SourceID, "entry": in.EntryID}, map[string]any{"revision": in.Revision})
	})
	configurationTool(s, server, agent, "import_source_structure", "Import 1–20 discovered objects into the semantic draft. Generates field/type skeletons without sampling data; repeated imports preserve authored descriptions.", false, func(ctx context.Context, in struct {
		configurationDraft
		Objects []semantic.Reference `json:"objects"`
	}) (any, error) {
		return configurationHandler(ctx, s.importSemanticStructure, "POST /api/sources/{id}/semantics/import-structure", map[string]string{"id": in.SourceID}, map[string]any{"revision": in.Revision, "objects": in.Objects})
	})
	configurationTool(s, server, agent, "validate_semantic_draft", "Validate current catalog, parameter contracts and ontology references. This does not execute templates or prove database constraints.", true, func(ctx context.Context, in configurationSourceID) (any, error) {
		return configurationHandler(ctx, s.validateSemantics, "POST /api/sources/{id}/semantics/validate", map[string]string{"id": in.SourceID}, nil)
	})
	configurationTool(s, server, agent, "trial_query_template", "Run a saved draft template with its example and regression cases against the real database, using shared read-only validation and limits. Returns validation evidence, not query results. Required before administrator publication of enabled templates.", false, func(ctx context.Context, in struct {
		configurationDraft
		TemplateID string `json:"template_id"`
	}) (any, error) {
		return configurationHandler(ctx, s.trialSemantics, "POST /api/sources/{id}/semantics/trial", map[string]string{"id": in.SourceID}, map[string]any{"revision": in.Revision, "template_id": in.TemplateID})
	})
	configurationTool(s, server, agent, "check_ontology_mapping", "Verify draft mappings against discoverable objects/fields. Explicit declared fields remain unverified. Does not validate all database values or infer business meaning.", false, func(ctx context.Context, in configurationDraft) (any, error) {
		return configurationHandler(ctx, s.checkOntologyMapping, "POST /api/sources/{id}/semantics/check-mapping", map[string]string{"id": in.SourceID}, map[string]any{"revision": in.Revision})
	})
}

type configurationOntologyID struct {
	OntologyID string `json:"ontology_id"`
}
type configurationOntologyRevision struct {
	OntologyID string `json:"ontology_id"`
	Revision   string `json:"revision"`
}

func (s *Server) configurationOntologyTools(server *mcp.Server, agent model.ConfigurationAgent) {
	configurationTool(s, server, agent, "list_ontologies", "List shared ontology summaries and immutable latest publication numbers, including archived definitions.", true, func(ctx context.Context, in configurationPage) (any, error) {
		result, err := configurationHandler(ctx, s.ontologies, "GET /api/ontologies", nil, nil)
		if err != nil {
			return nil, err
		}
		return configurationSlice(result.([]any), in)
	})
	configurationTool(s, server, agent, "get_ontology", "Read a shared ontology draft, revision, recent version summaries and usage. Other sources may share this ontology; changes remain drafts.", true, func(ctx context.Context, in configurationOntologyID) (any, error) {
		return configurationHandler(ctx, s.getOntology, "GET /api/ontologies/{ontology}", map[string]string{"ontology": in.OntologyID}, nil)
	})
	configurationTool(s, server, agent, "create_ontology", "Create a shared business ontology draft with stable entity/property/relation IDs. Single inheritance, declarative constraints and no instance storage or inference. Administrator publishes before mapping can adopt it.", false, func(ctx context.Context, in struct {
		ID         string              `json:"id,omitempty"`
		Definition ontology.Definition `json:"definition"`
	}) (any, error) {
		return configurationHandler(ctx, s.createOntology, "POST /api/ontologies", nil, map[string]any{"id": in.ID, "revision": "0", "definition": in.Definition})
	})
	configurationTool(s, server, agent, "save_ontology_draft", "Replace a complete shared ontology draft at its current revision. Preserve stable IDs and existing definitions. Does not update published versions or existing source bindings.", false, func(ctx context.Context, in struct {
		configurationOntologyRevision
		Definition ontology.Definition `json:"definition"`
	}) (any, error) {
		return configurationHandler(ctx, s.changeOntology, "PUT /api/ontologies/{ontology}", map[string]string{"ontology": in.OntologyID}, map[string]any{"revision": in.Revision, "definition": in.Definition})
	})
	configurationTool(s, server, agent, "validate_ontology", "Validate the saved ontology revision for inheritance cycles, property conflicts, identities, relationship endpoints and cardinality. No database facts are asserted.", true, func(ctx context.Context, in configurationOntologyRevision) (any, error) {
		return configurationHandler(ctx, s.validateOntology, "POST /api/ontologies/{ontology}/validate", map[string]string{"ontology": in.OntologyID}, map[string]any{"revision": in.Revision})
	})
	configurationTool(s, server, agent, "get_ontology_version", "Read an immutable published ontology definition to use in snapshot.ontology. Version must be a positive integer encoded as a string.", true, func(ctx context.Context, in struct {
		OntologyID string `json:"ontology_id"`
		Version    string `json:"version"`
	}) (any, error) {
		return configurationHandler(ctx, s.getOntologyVersion, "GET /api/ontologies/{ontology}/versions/{version}", map[string]string{"ontology": in.OntologyID, "version": in.Version}, nil)
	})
}
