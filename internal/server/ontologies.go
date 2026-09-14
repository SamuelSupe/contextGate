package server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strconv"

	"github.com/SamuelSupe/contextGate/internal/engine"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/secure"
)

func (s *Server) ontologyRoutes(mux *http.ServeMux) {
	for route, handler := range map[string]http.HandlerFunc{
		"GET /api/ontologies":                                    s.ontologies,
		"POST /api/ontologies":                                   s.createOntology,
		"GET /api/ontologies/{ontology}":                         s.getOntology,
		"GET /api/ontologies/{ontology}/usage-summary":           s.ontologyUsageSummary,
		"PUT /api/ontologies/{ontology}":                         s.changeOntology,
		"DELETE /api/ontologies/{ontology}":                      s.changeOntology,
		"POST /api/ontologies/{ontology}/import":                 s.changeOntology,
		"POST /api/ontologies/{ontology}/validate":               s.validateOntology,
		"POST /api/ontologies/{ontology}/publish":                s.changeOntology,
		"POST /api/ontologies/{ontology}/discard":                s.changeOntology,
		"POST /api/ontologies/{ontology}/archive":                s.changeOntology,
		"GET /api/ontologies/{ontology}/versions":                s.ontologyVersions,
		"GET /api/ontologies/{ontology}/versions/{version}":      s.getOntologyVersion,
		"GET /api/ontologies/{ontology}/versions/{version}/diff": s.ontologyDiff,
		"DELETE /api/ontologies/{ontology}/versions/{version}":   s.changeOntology,
		"GET /api/ontologies/{ontology}/export":                  s.exportOntology,
	} {
		mux.HandleFunc(route, s.requireAdmin(handler))
	}
}

func (s *Server) ontologies(w http.ResponseWriter, r *http.Request) {
	states, err := s.Store.Ontologies()
	if err != nil {
		semanticFailure(w, err)
		return
	}
	out := []map[string]any{}
	for _, st := range states {
		if len(st.Draft.Description) > 1024 {
			st.Draft.Description = string([]rune(st.Draft.Description)[:min(256, len([]rune(st.Draft.Description)))])
		}
		out = append(out, map[string]any{"id": st.ID, "name": st.Draft.Name, "description": st.Draft.Description, "revision": strconv.FormatInt(st.Revision, 10), "latest_version": strconv.FormatInt(st.LatestVersion, 10), "archived": st.Archived})
	}
	write(w, 200, out)
}

type ontologyInput struct {
	ID         string              `json:"id,omitempty"`
	Revision   int64               `json:"revision,string"`
	Definition ontology.Definition `json:"definition"`
	Archived   bool                `json:"archived,omitempty"`
}

func (s *Server) createOntology(w http.ResponseWriter, r *http.Request) {
	var in ontologyInput
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid ontology document"))
		return
	}
	if in.ID == "" {
		in.ID = "ont_" + secure.Random(12)
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,95}$`).MatchString(in.ID) || in.Revision != 0 {
		semanticFailure(w, model.Fail("invalid_input", "Invalid ontology ID or initial revision"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	st := ontology.State{ID: in.ID, Draft: in.Definition}
	if err := s.Store.WriteOntology(st, 0, false); err != nil {
		semanticFailure(w, err)
		return
	}
	r.SetPathValue("ontology", in.ID)
	s.getOntology(w, r)
}

func (s *Server) getOntology(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Ontology(r.PathValue("ontology"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	versions, err := s.Store.OntologyVersions(st.ID, 0)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	usage, err := s.Store.OntologyUsage(st.ID)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	changed := true
	if st.LatestVersion > 0 {
		v, err := s.Store.OntologyVersion(st.ID, st.LatestVersion)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		changed = !reflect.DeepEqual(st.Draft, v.Definition)
	}
	write(w, 200, map[string]any{"id": st.ID, "revision": strconv.FormatInt(st.Revision, 10), "latest_version": strconv.FormatInt(st.LatestVersion, 10), "archived": st.Archived, "draft": st.Draft, "versions": versions, "usage": usage, "changed": changed})
}

func (s *Server) changeOntology(w http.ResponseWriter, r *http.Request) {
	var in ontologyInput
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid ontology revision or document"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	id := r.PathValue("ontology")
	st, err := s.Store.Ontology(id)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if in.Revision != st.Revision {
		semanticFailure(w, model.Fail("conflict", "Ontology draft changed; reload before continuing"))
		return
	}
	if r.Method == http.MethodDelete {
		version := int64(0)
		if r.PathValue("version") != "" {
			version, err = strconv.ParseInt(r.PathValue("version"), 10, 64)
			if err != nil || version < 1 {
				semanticFailure(w, model.Fail("invalid_input", "Invalid version"))
				return
			}
		}
		if err = s.Store.DeleteOntologyVersion(id, version, in.Revision); err != nil {
			semanticFailure(w, err)
			return
		}
		write(w, 200, map[string]bool{"deleted": true})
		return
	}
	publish := r.Pattern == "POST /api/ontologies/{ontology}/publish"
	switch r.Pattern {
	case "POST /api/ontologies/{ontology}/publish":
		if st.Archived {
			semanticFailure(w, model.Fail("invalid_ontology", "Unarchive the ontology before publishing new versions"))
			return
		}
	case "POST /api/ontologies/{ontology}/discard":
		st.Draft = ontology.Empty()
		if st.LatestVersion > 0 {
			v, err := s.Store.OntologyVersion(id, st.LatestVersion)
			if err != nil {
				semanticFailure(w, err)
				return
			}
			st.Draft = v.Definition
		}
	case "POST /api/ontologies/{ontology}/archive":
		st.Archived = in.Archived
	default:
		st.Draft = in.Definition
	}
	if err = s.Store.WriteOntology(st, in.Revision, publish); err != nil {
		semanticFailure(w, err)
		return
	}
	s.getOntology(w, r)
}

func (s *Server) validateOntology(w http.ResponseWriter, r *http.Request) {
	var in ontologyInput
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid validation request"))
		return
	}
	st, err := s.Store.Ontology(r.PathValue("ontology"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if st.Revision != in.Revision {
		semanticFailure(w, model.Fail("conflict", "Draft changed; reload before validating"))
		return
	}
	if err = ontology.Validate(st.Draft); err != nil {
		semanticFailure(w, model.Fail("invalid_ontology", err.Error()))
		return
	}
	write(w, 200, map[string]any{"valid": true, "revision": strconv.FormatInt(st.Revision, 10), "scope": "Definition consistency only; database values and declared constraints have not been validated"})
}

func (s *Server) ontologyVersions(w http.ResponseWriter, r *http.Request) {
	before, err := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if r.URL.Query().Get("before") != "" && (err != nil || before < 1) {
		semanticFailure(w, model.Fail("invalid_input", "Invalid version page"))
		return
	}
	versions, err := s.Store.OntologyVersions(r.PathValue("ontology"), before)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, versions)
}

func (s *Server) getOntologyVersion(w http.ResponseWriter, r *http.Request) {
	v, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || v < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Invalid ontology version"))
		return
	}
	version, err := s.Store.OntologyVersion(r.PathValue("ontology"), v)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, version)
}

func (s *Server) exportOntology(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("ontology")
	st, err := s.Store.Ontology(id)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	d := st.Draft
	if raw := r.URL.Query().Get("version"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 1 {
			semanticFailure(w, model.Fail("invalid_input", "Invalid version"))
			return
		}
		version, err := s.Store.OntologyVersion(id, v)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		d = version.Definition
	}
	w.Header().Set("Content-Disposition", `attachment; filename="ontology.json"`)
	write(w, 200, map[string]any{"id": id, "definition": d})
}

func (s *Server) ontologyDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("ontology")
	from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if err != nil || from < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Select a previous version"))
		return
	}
	to, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || to < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Select a target version"))
		return
	}
	a, err := s.Store.OntologyVersion(id, from)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	b, err := s.Store.OntologyVersion(id, to)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	index := func(d ontology.Definition) map[string]json.RawMessage {
		out := map[string]json.RawMessage{}
		raw, _ := json.Marshal(d)
		var groups map[string]json.RawMessage
		json.Unmarshal(raw, &groups)
		for _, kind := range []string{"entities", "properties", "relations"} {
			var items []map[string]any
			json.Unmarshal(groups[kind], &items)
			for _, item := range items {
				key := kind + ":" + item["id"].(string)
				out[key], _ = json.Marshal(item)
			}
		}
		out["name"], _ = json.Marshal(d.Name)
		out["description"], _ = json.Marshal(d.Description)
		return out
	}
	old, new := index(a.Definition), index(b.Definition)
	changes := []map[string]any{}
	for key, value := range new {
		if !reflect.DeepEqual(value, old[key]) {
			changes = append(changes, map[string]any{"reference": key, "before": old[key], "after": value})
		}
		delete(old, key)
	}
	for key, value := range old {
		changes = append(changes, map[string]any{"reference": key, "before": value, "after": nil})
	}
	write(w, 200, map[string]any{"ontology_id": id, "from": strconv.FormatInt(from, 10), "to": strconv.FormatInt(to, 10), "changes": changes})
}

func (s *Server) checkOntologyMapping(w http.ResponseWriter, r *http.Request) {
	var in semanticAction
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid mapping check request"))
		return
	}
	checks, err := s.Engine.CheckOntologyMapping(r.Context(), r.PathValue("id"), in.Revision)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, map[string]any{"checks": checks, "revision": strconv.FormatInt(in.Revision, 10), "scope": "Structure metadata only; declared identity and cardinality constraints are not data validation"})
}

func (s *Server) previewSemanticVisibility(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AgentID string `json:"agent_id"`
		EntryID string `json:"entry_id,omitempty"`
		Keyword string `json:"keyword,omitempty"`
		Kind    string `json:"kind,omitempty"`
		Cursor  string `json:"cursor,omitempty"`
		Limit   int    `json:"limit,omitempty"`
	}
	if err := decode(r, &in); err != nil || in.AgentID == "" {
		semanticFailure(w, model.Fail("invalid_input", "Select an Agent identity"))
		return
	}
	p := model.Principal{AgentID: in.AgentID, Preview: true}
	var out map[string]any
	var err error
	if in.EntryID != "" {
		out, err = s.Engine.SemanticEntry(p, r.PathValue("id"), in.EntryID)
	} else {
		out, err = s.Engine.SearchSemantics(p, engine.SemanticSearch{SourceID: r.PathValue("id"), Keyword: in.Keyword, Kind: in.Kind, Cursor: in.Cursor, Limit: in.Limit})
	}
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, out)
}
