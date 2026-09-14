package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

type semanticChange struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Change string   `json:"change"`
	Fields []string `json:"fields"`
}

func semanticDiff(old, next semantic.Snapshot) []semanticChange {
	entries := func(s semantic.Snapshot) map[string]semantic.Entry {
		out := map[string]semantic.Entry{"overview": {ID: "overview", Name: "Overview", Description: s.Overview, Definition: s.Ontology}}
		for _, e := range s.Entries {
			if e.Template != nil {
				template := *e.Template
				template.ExecutionVersion = ""
				e.Template = &template
			}
			out[e.ID] = e
		}
		return out
	}
	a, b := entries(old), entries(next)
	ids := []string{}
	for id := range a {
		ids = append(ids, id)
	}
	for id := range b {
		if _, ok := a[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	out := []semanticChange{}
	for _, id := range ids {
		before, was := a[id]
		after, is := b[id]
		change := semanticChange{ID: id, Name: after.Name, Change: "changed", Fields: []string{}}
		if !was {
			change.Change = "added"
		} else if !is {
			change.Change = "removed"
			change.Name = before.Name
		} else {
			x, y := map[string]json.RawMessage{}, map[string]json.RawMessage{}
			raw, _ := json.Marshal(before)
			json.Unmarshal(raw, &x)
			raw, _ = json.Marshal(after)
			json.Unmarshal(raw, &y)
			for key, val := range x {
				if string(val) != string(y[key]) {
					change.Fields = append(change.Fields, key)
				}
			}
			for key := range y {
				if _, ok := x[key]; !ok {
					change.Fields = append(change.Fields, key)
				}
			}
			if len(change.Fields) == 0 {
				continue
			}
			slices.Sort(change.Fields)
		}
		out = append(out, change)
	}
	return out
}

func (s *Server) semanticVersions(w http.ResponseWriter, r *http.Request) {
	if _, _, err := s.semanticState(r.PathValue("id")); err != nil {
		semanticFailure(w, err)
		return
	}
	before, err := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if r.URL.Query().Get("before") != "" && (err != nil || before < 1) {
		semanticFailure(w, model.Fail("invalid_input", "Invalid version cursor"))
		return
	}
	versions, err := s.Store.SemanticVersions(r.PathValue("id"), before)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, versions)
}

func (s *Server) semanticVersion(w http.ResponseWriter, r *http.Request) {
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || version < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Invalid semantic version"))
		return
	}
	v, err := s.Store.SemanticVersion(r.PathValue("id"), version)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, map[string]any{"version": v, "changes": semanticDiff(*v.Snapshot, st.Draft), "revision": strconv.FormatInt(st.Revision, 10)})
}

func (s *Server) restoreSemantics(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int64 `json:"revision,string"`
		Version  int64 `json:"version,string"`
	}
	if err := decode(r, &in); err != nil || in.Version < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Invalid restore request"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if st.Revision != in.Revision {
		semanticFailure(w, model.Fail("conflict", "Draft changed; reload before restoring"))
		return
	}
	v, err := s.Store.SemanticVersion(r.PathValue("id"), in.Version)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	st.Draft = *v.Snapshot
	for i := range st.Draft.Entries {
		if st.Draft.Entries[i].Template != nil {
			st.Draft.Entries[i].Template.ExecutionVersion = ""
		}
	}
	now := time.Now().UTC()
	st.TrialAfter = &now
	if err = s.Store.WriteSemantics(r.PathValue("id"), in.Revision, st); err != nil {
		semanticFailure(w, err)
		return
	}
	s.semantics(w, r)
}

func (s *Server) deleteSemanticVersion(w http.ResponseWriter, r *http.Request) {
	var in semanticAction
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid history deletion request"))
		return
	}
	version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || version < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Invalid semantic version"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	_, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if st.Revision != in.Revision {
		semanticFailure(w, model.Fail("conflict", "Draft changed; reload before deleting history"))
		return
	}
	if version == st.PublishedVersion {
		semanticFailure(w, model.Fail("conflict", "The current publication cannot be deleted from history"))
		return
	}
	result, err := s.Store.DB.Exec("DELETE FROM semantics_versions WHERE source_id=$1 AND version=$2", r.PathValue("id"), version)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if n == 0 {
		semanticFailure(w, model.Fail("not_found", "Semantic version not found"))
		return
	}
	write(w, 200, map[string]any{"ok": true})
}
