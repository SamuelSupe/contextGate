package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
)

func (s *Server) ontologyUsageSummary(w http.ResponseWriter, r *http.Request) {
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	id := r.PathValue("ontology")
	if _, err := s.Store.Ontology(id); err != nil {
		semanticFailure(w, model.Fail("not_found", "Ontology not found"))
		return
	}
	usage, err := s.Store.OntologyUsage(id)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	type counts struct {
		Sources   int `json:"sources"`
		Templates int `json:"templates"`
	}
	out := map[string]counts{}
	for _, u := range usage {
		if u.Phase != "published" {
			continue
		}
		if r.Context().Err() != nil {
			return
		}
		src, st, err := s.semanticState(u.SourceID)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		b := st.Published.Ontology
		if b == nil || b.OntologyID != id {
			continue
		}
		version, err := s.Store.OntologyVersion(id, b.Version)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		valid := map[string]bool{}
		for _, entry := range st.Published.Entries {
			if entry.Template != nil {
				valid[entry.ID] = src.Enabled && s.Engine.PublishedTemplateValidation(src, entry).Valid
			}
		}
		for _, entity := range b.Entities {
			rels := map[string]bool{}
			for _, rel := range version.Definition.Relations {
				if rel.From == entity.Entity || rel.To == entity.Entity {
					rels[rel.ID] = true
				}
			}
			templates := map[string]bool{}
			for _, p := range b.Properties {
				if p.Entity == entity.Entity && valid[p.TemplateID] {
					templates[p.TemplateID] = true
				}
			}
			mappedRels := []string{}
			for _, rel := range b.Relations {
				if rels[rel.Relation] {
					mappedRels = append(mappedRels, "ontology:relation_type:"+rel.Relation)
					if valid[rel.TemplateID] {
						templates[rel.TemplateID] = true
					}
				}
			}
			for _, entry := range st.Published.Entries {
				if !valid[entry.ID] {
					continue
				}
				for _, ref := range entry.Template.ConceptRefs {
					if ref == "ontology:entity_type:"+entity.Entity || strings.HasPrefix(ref, "ontology:property:"+entity.Entity+":") || slices.Contains(mappedRels, ref) {
						templates[entry.ID] = true
					}
				}
			}
			count := out[entity.Entity]
			count.Sources++
			count.Templates += len(templates)
			out[entity.Entity] = count
		}
	}
	write(w, 200, out)
}
