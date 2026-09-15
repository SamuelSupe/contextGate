package server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/adapter"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

type impactChange struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Change    string   `json:"change"`
	Effect    string   `json:"effect"`
	Templates []string `json:"templates"`
}
type impactIssue struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Action string `json:"action"`
	Status string `json:"status"`
}

func mappingItems(b *ontology.Binding) map[string]any {
	out := map[string]any{}
	if b == nil {
		return out
	}
	for _, x := range b.Entities {
		out[ontology.Ref("entity_type", x.Entity)] = x
	}
	for _, x := range b.Properties {
		out[ontology.PropertyRef(x.Entity, x.Property)] = x
	}
	for _, x := range b.Relations {
		out[ontology.Ref("relation_type", x.Relation)] = x
	}
	return out
}

func (s *Server) semanticImpact(w http.ResponseWriter, r *http.Request) {
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	src, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	revision := strconv.FormatInt(st.Revision, 10)
	if r.URL.Query().Get("revision") != revision {
		semanticFailure(w, model.Fail("conflict", "Draft changed; reload before reviewing publication"))
		return
	}
	before, after := map[string]semantic.Entry{}, map[string]semantic.Entry{}
	for _, en := range st.Published.Entries {
		before[en.ID] = en
	}
	for _, en := range st.Draft.Entries {
		after[en.ID] = en
	}
	changes := []impactChange{}
	seen := map[string]bool{}
	for _, diff := range semanticDiff(st.Published, st.Draft) {
		x, exists := after[diff.ID]
		if !exists {
			x = before[diff.ID]
		}
		effect := "context"
		if x.Template != nil || before[diff.ID].Template != nil {
			old, next := before[diff.ID].Template, after[diff.ID].Template
			if old == nil || next == nil || semantic.Definition(*old) != semantic.Definition(*next) {
				effect = "execution"
			}
		}
		changes = append(changes, impactChange{diff.ID, diff.Name, x.Kind, diff.Change, effect, []string{}})
		seen[diff.ID] = true
	}
	oldMapping, newMapping := mappingItems(st.Published.Ontology), mappingItems(st.Draft.Ontology)
	keys := []string{}
	for id := range oldMapping {
		keys = append(keys, id)
	}
	for id := range newMapping {
		if _, ok := oldMapping[id]; !ok {
			keys = append(keys, id)
		}
	}
	slices.Sort(keys)
	for _, id := range keys {
		if reflect.DeepEqual(oldMapping[id], newMapping[id]) {
			continue
		}
		change := "changed"
		if oldMapping[id] == nil {
			change = "added"
		} else if newMapping[id] == nil {
			change = "removed"
		}
		changes = append(changes, impactChange{id, id, "mapping", change, "context", []string{}})
	}
	issues := []impactIssue{}
	for _, en := range st.Draft.Entries {
		if en.Template == nil || !en.Template.Enabled {
			continue
		}
		if !s.Engine.DraftTemplateValidation(src, st, en).Valid {
			issues = append(issues, impactIssue{en.ID, en.Name, "trial", "trial_required"})
		}
		if old := before[en.ID]; old.Template != nil && semantic.Definition(*old.Template) == semantic.Definition(*en.Template) && !s.Engine.PublishedTemplateValidation(src, old).Valid {
			if !seen[en.ID] {
				changes = append(changes, impactChange{en.ID, en.Name, "template", "revalidated", "execution", []string{en.ID}})
			} else {
				for i := range changes {
					if changes[i].ID == en.ID {
						changes[i].Effect = "execution"
					}
				}
			}
		}
	}
	validationError := ""
	if err := semantic.Validate(st.Draft, adapter.ForSource(src).Tool); err != nil {
		validationError = err.Error()
		issues = append(issues, impactIssue{"", "Catalog and parameters", "catalog", "invalid_semantics"})
	}
	if err := s.Engine.ValidateOntologyMapping(st); err != nil {
		if validationError == "" {
			validationError = err.Error()
		}
		issues = append(issues, impactIssue{"", "Ontology mapping", "mapping", "invalid_mapping"})
	}
	view := s.semanticView(src, st)
	if mapping := view["mapping_validation"].(map[string]any); st.Draft.Ontology != nil && mapping["status"] != "checked" {
		issues = append(issues, impactIssue{"", "Ontology mapping", "check_mapping", "check_required"})
	}
	queries := map[string]bool{}
	for i := range changes {
		c := &changes[i]
		linked := map[string]bool{}
		for _, snapshot := range []semantic.Snapshot{st.Published, st.Draft} {
			for _, en := range snapshot.Entries {
				if en.Template != nil && (en.ID == c.ID || slices.Contains(en.Template.ConceptRefs, c.ID)) {
					linked[en.ID] = true
				}
				if en.ID == c.ID && en.TemplateID != "" {
					linked[en.TemplateID] = true
				}
			}
			if b := snapshot.Ontology; b != nil {
				for _, p := range b.Properties {
					if ontology.PropertyRef(p.Entity, p.Property) == c.ID && p.TemplateID != "" {
						linked[p.TemplateID] = true
					}
				}
				for _, rel := range b.Relations {
					if ontology.Ref("relation_type", rel.Relation) == c.ID && rel.TemplateID != "" {
						linked[rel.TemplateID] = true
					}
				}
			}
		}
		for id := range linked {
			c.Templates = append(c.Templates, id)
		}
		slices.Sort(c.Templates)
		c.Templates = slices.Compact(c.Templates)
		if c.Effect == "execution" && before[c.ID].Template != nil {
			queries[c.ID] = true
		}
	}
	agents, err := s.Store.Agents()
	if err != nil {
		semanticFailure(w, err)
		return
	}
	affected := []map[string]string{}
	for _, a := range agents {
		if a.Enabled && a.RevokedAt == nil && a.ExpiresAt.After(time.Now()) && slices.Contains(a.Sources, src.ID) {
			affected = append(affected, map[string]string{"id": a.ID, "name": a.Name})
		}
	}
	out := map[string]any{"revision": revision, "published_version": strconv.FormatInt(st.PublishedVersion, 10), "changes": changes, "issues": issues, "validation_error": validationError, "agents": affected, "interrupted_templates": len(queries), "can_publish": len(issues) == 0}
	raw, err := json.Marshal(out)
	if err != nil || len(raw) > 1<<20 {
		semanticFailure(w, model.Fail("limit_exceeded", "Change review exceeds response limit"))
		return
	}
	write(w, 200, out)
}
