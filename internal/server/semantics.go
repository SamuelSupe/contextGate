package server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/adapter"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

func (s *Server) semanticRoutes(mux *http.ServeMux) {
	for route, handler := range map[string]http.HandlerFunc{
		"GET /api/sources/{id}/semantics/versions":              s.semanticVersions,
		"GET /api/sources/{id}/semantics/versions/{version}":    s.semanticVersion,
		"DELETE /api/sources/{id}/semantics/versions/{version}": s.deleteSemanticVersion,
		"POST /api/sources/{id}/semantics/restore":              s.restoreSemantics,
		"POST /api/sources/{id}/semantics/trial-all":            s.trialAllSemantics,
		"GET /api/sources/{id}/semantics":                       s.semantics,
		"PUT /api/sources/{id}/semantics":                       s.saveSemantics,
		"GET /api/sources/{id}/semantics/entries":               s.semanticEntries,
		"PUT /api/sources/{id}/semantics/entries/{entry}":       s.saveSemanticEntry,
		"DELETE /api/sources/{id}/semantics/entries/{entry}":    s.saveSemanticEntry,
		"POST /api/sources/{id}/semantics/preview":              s.previewSemanticVisibility,
		"POST /api/sources/{id}/semantics/check-mapping":        s.checkOntologyMapping,
		"POST /api/sources/{id}/semantics/validate":             s.validateSemantics,
		"POST /api/sources/{id}/semantics/trial":                s.trialSemantics,
		"POST /api/sources/{id}/semantics/publish":              s.publishSemantics,
		"POST /api/sources/{id}/semantics/discard":              s.discardSemantics,
		"POST /api/sources/{id}/semantics/import":               s.saveSemantics,
		"GET /api/sources/{id}/semantics/export":                s.exportSemantics,
		"POST /api/sources/{id}/semantics/import-structure":     s.importSemanticStructure,
		"POST /api/sources/{id}/semantics/execute":              s.executeSemanticTemplate,
	} {
		mux.HandleFunc(route, s.requireAdmin(handler))
	}
}

func semanticFailure(w http.ResponseWriter, err error) {
	status := 400
	if model.ErrorCode(err) == "conflict" {
		status = 409
	}
	if model.ErrorCode(err) == "not_found" {
		status = 404
	}
	fail(w, status, err)
}

func (s *Server) semanticState(id string) (model.Source, semantic.State, error) {
	src, err := s.Store.Source(id)
	if err != nil {
		return src, semantic.State{}, model.Fail("not_found", "Data source not found")
	}
	st, err := s.Store.Semantics(id)
	return src, st, err
}

func (s *Server) semanticView(src model.Source, st semantic.State) map[string]any {
	validation := []any{}
	for _, en := range st.Draft.Entries {
		if en.Template != nil {
			validation = append(validation, s.Engine.DraftTemplateValidation(src, st, en))
		}
	}
	a, b := st.Draft, st.Published
	// Execution versions are assigned at publication and are not draft content.
	strip := func(v semantic.Snapshot) semantic.Snapshot {
		raw, _ := json.Marshal(v)
		var out semantic.Snapshot
		json.Unmarshal(raw, &out)
		for i := range out.Entries {
			if out.Entries[i].Template != nil {
				out.Entries[i].Template.ExecutionVersion = ""
			}
		}
		return out
	}
	mapping := s.Engine.MappingValidation(src, st.Draft)
	if st.TrialAfter != nil {
		if checked, ok := mapping["checked_at"].(time.Time); ok && checked.Before(*st.TrialAfter) {
			mapping["status"] = "check_required"
		}
	}
	return map[string]any{"revision": strconv.FormatInt(st.Revision, 10), "published_version": strconv.FormatInt(st.PublishedVersion, 10), "draft": st.Draft, "published": st.Published, "changed": !reflect.DeepEqual(strip(a), strip(b)), "validation": validation, "mapping_validation": mapping}
}

func (s *Server) semantics(w http.ResponseWriter, r *http.Request) {
	src, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, s.semanticView(src, st))
}

type semanticInput struct {
	Revision int64             `json:"revision,string"`
	Snapshot semantic.Snapshot `json:"snapshot"`
}

func boundedDraft(draft semantic.Snapshot) error {
	if draft.FormatVersion != semantic.FormatVersion && draft.FormatVersion != 1 {
		return model.Fail("invalid_semantics", "Unsupported semantic format version")
	}
	b, _ := json.Marshal(draft)
	if len(draft.Overview) > 32<<10 || len(b) > semantic.MaxSnapshotBytes || len(draft.Entries) > semantic.MaxEntries {
		return model.Fail("invalid_semantics", "Catalog exceeds 500 entries or 768 KiB")
	}
	ids := map[string]bool{}
	for _, en := range draft.Entries {
		if en.ID == "overview" || en.ID == "" || len(en.ID) > 96 || ids[en.ID] {
			return model.Fail("invalid_semantics", "Entries require unique IDs of at most 96 bytes")
		}
		ids[en.ID] = true
		eb, _ := json.Marshal(en)
		if len(eb) > 128<<10 {
			return model.Fail("invalid_semantics", "Entry exceeds 128 KiB")
		}
	}
	return nil
}

func (s *Server) writeDraft(id string, revision int64, draft semantic.Snapshot) error {
	if err := boundedDraft(draft); err != nil {
		return err
	}
	_, st, err := s.semanticState(id)
	if err != nil {
		return err
	}
	if st.Revision != revision {
		return model.Fail("conflict", "Draft changed; reload before saving")
	}
	for i := range draft.Entries {
		if draft.Entries[i].Template != nil {
			draft.Entries[i].Template.ExecutionVersion = ""
		}
	}
	draft.FormatVersion = semantic.FormatVersion
	st.Draft = draft
	return s.Store.WriteSemantics(id, revision, st)
}

func (s *Server) saveSemantics(w http.ResponseWriter, r *http.Request) {
	var in semanticInput
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid semantic document"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	if err := s.writeDraft(r.PathValue("id"), in.Revision, in.Snapshot); err != nil {
		semanticFailure(w, err)
		return
	}
	s.semantics(w, r)
}

func (s *Server) saveSemanticEntry(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int64          `json:"revision,string"`
		Entry    semantic.Entry `json:"entry"`
	}
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid semantic entry"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	id := r.PathValue("entry")
	index := slices.IndexFunc(st.Draft.Entries, func(en semantic.Entry) bool { return en.ID == id })
	if r.Method == http.MethodDelete {
		if index >= 0 {
			st.Draft.Entries = slices.Delete(st.Draft.Entries, index, index+1)
		}
	} else {
		in.Entry.ID = id
		if index >= 0 {
			st.Draft.Entries[index] = in.Entry
		} else {
			st.Draft.Entries = append(st.Draft.Entries, in.Entry)
		}
	}
	if err = s.writeDraft(r.PathValue("id"), in.Revision, st.Draft); err != nil {
		semanticFailure(w, err)
		return
	}
	s.semantics(w, r)
}

func (s *Server) semanticEntries(w http.ResponseWriter, r *http.Request) {
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if offset < 0 || limit < 0 || limit > 100 {
		semanticFailure(w, model.Fail("invalid_input", "Invalid page"))
		return
	}
	if limit == 0 {
		limit = 25
	}
	if revision := q.Get("revision"); revision != "" && revision != strconv.FormatInt(st.Revision, 10) {
		semanticFailure(w, model.Fail("conflict", "Draft changed; restart entry listing"))
		return
	}
	out := []semantic.Entry{}
	matches := 0
	for _, en := range st.Draft.Entries {
		if (q.Get("kind") == "" || en.Kind == q.Get("kind")) && strings.Contains(strings.ToLower(en.Name+" "+en.Description+" "+strings.Join(en.Aliases, " ")), strings.ToLower(q.Get("keyword"))) {
			if matches >= offset && len(out) < limit {
				out = append(out, en)
			}
			matches++
		}
	}
	write(w, 200, map[string]any{"entries": out, "total": matches, "revision": strconv.FormatInt(st.Revision, 10)})
}

func (s *Server) validateSemantics(w http.ResponseWriter, r *http.Request) {
	src, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if err = semantic.Validate(st.Draft, adapter.ForSource(src).Tool); err != nil {
		semanticFailure(w, err)
		return
	}
	if err = s.Engine.ValidateOntologyMapping(st); err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, s.semanticView(src, st))
}

type semanticAction struct {
	Revision   int64  `json:"revision,string"`
	TemplateID string `json:"template_id,omitempty"`
}

func (s *Server) trialSemantics(w http.ResponseWriter, r *http.Request) {
	var in semanticAction
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid trial request"))
		return
	}
	v, err := s.Engine.TrialTemplate(r.Context(), r.PathValue("id"), in.TemplateID, in.Revision)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) publishSemantics(w http.ResponseWriter, r *http.Request) {
	var in semanticAction
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid publication request"))
		return
	}
	_, err := s.Engine.PublishSemantics(r.PathValue("id"), in.Revision)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	s.semantics(w, r)
}

func (s *Server) discardSemantics(w http.ResponseWriter, r *http.Request) {
	var in semanticAction
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid discard request"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	st.Draft = st.Published
	st.TrialAfter = nil
	if err = s.Store.WriteSemantics(r.PathValue("id"), in.Revision, st); err != nil {
		semanticFailure(w, err)
		return
	}
	s.semantics(w, r)
}

func (s *Server) exportSemantics(w http.ResponseWriter, r *http.Request) {
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	snapshot := st.Draft
	if r.URL.Query().Get("phase") == "published" {
		snapshot = st.Published
	}
	for i := range snapshot.Entries {
		if snapshot.Entries[i].Template != nil {
			snapshot.Entries[i].Template.ExecutionVersion = ""
		}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="semantics.json"`)
	write(w, 200, snapshot)
}

func (s *Server) importSemanticStructure(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int64                `json:"revision,string"`
		Objects  []semantic.Reference `json:"objects"`
	}
	if err := decode(r, &in); err != nil || len(in.Objects) == 0 || len(in.Objects) > 20 {
		semanticFailure(w, model.Fail("invalid_input", "Select 1–20 structure objects"))
		return
	}
	id := r.PathValue("id")
	src, st, err := s.semanticState(id)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	for _, ref := range in.Objects {
		if ref.Object == "" || ref.Field != "" {
			semanticFailure(w, model.Fail("invalid_input", "Select objects, not field paths"))
			return
		}
		obj := model.Object{Name: ref.Object, Namespace: ref.Namespace, Type: "object"}
		// Neo4j property discovery reads node contents. Import labels only; their
		// property semantics must be supplied explicitly by the administrator.
		if src.Kind != "neo4j" {
			res, err := s.Engine.Execute(r.Context(), model.AdministratorPrincipal(r.Context()), "describe", model.Query{SourceID: id, Namespace: ref.Namespace, Object: ref.Object})
			if err != nil {
				semanticFailure(w, err)
				return
			}
			if res.Truncated || res.NextCursor != "" {
				semanticFailure(w, model.Fail("limit_exceeded", "Structure exceeds source limits; raise limits before importing"))
				return
			}
			for _, raw := range res.Data {
				b, _ := json.Marshal(raw)
				var found model.Object
				if json.Unmarshal(b, &found) == nil {
					if adapter.ForSource(src).Tool == "query_sql" || adapter.ForSource(src).Tool == "query_cql" || src.Kind == "influxdb" {
						obj.Columns = append(obj.Columns, model.Column{Name: found.Name, Type: found.Type})
					} else {
						obj.Columns = append(obj.Columns, found.Columns...)
					}
				}
			}
		}
		add := func(kind string, r semantic.Reference, typ string) {
			for _, en := range st.Draft.Entries {
				if en.Kind == kind && en.Reference != nil && *en.Reference == r {
					return
				}
			}
			name := r.Object
			if r.Field != "" {
				name = r.Field
			}
			st.Draft.Entries = append(st.Draft.Entries, semantic.Entry{ID: "entry_" + secure.Random(12), Kind: kind, Name: name, Reference: &r, DataType: typ})
		}
		add("object", ref, obj.Type)
		for _, col := range obj.Columns {
			field := ref
			field.Field = col.Name
			add("field", field, col.Type)
		}
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	fresh, err := s.Store.Source(id)
	if err != nil || fresh.ExecutionRevision() != src.ExecutionRevision() {
		semanticFailure(w, model.Fail("conflict", "Data source changed during structure import"))
		return
	}
	if err = s.writeDraft(id, in.Revision, st.Draft); err != nil {
		semanticFailure(w, err)
		return
	}
	s.semantics(w, r)
}

func (s *Server) executeSemanticTemplate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		semantic.Execution
		AgentID string `json:"agent_id,omitempty"`
	}
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid template execution request"))
		return
	}
	in.SourceID = r.PathValue("id")
	res, err := s.Engine.ExecuteTemplate(r.Context(), model.Principal{Admin: in.AgentID == "", AgentID: in.AgentID, Preview: true}, in.Execution)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, res)
}
