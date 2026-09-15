package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

type businessTemplate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Version    string `json:"execution_version"`
	Status     string `json:"status"`
	Executable bool   `json:"executable"`
}

type businessEntry struct {
	ID               string             `json:"id"`
	Kind             string             `json:"kind"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	SourceID         string             `json:"source_id"`
	SourceName       string             `json:"source_name"`
	SourceKind       string             `json:"source_kind"`
	PublishedVersion string             `json:"published_version"`
	Unit             string             `json:"unit,omitempty"`
	Grain            string             `json:"grain,omitempty"`
	TimeDefinition   string             `json:"time_definition,omitempty"`
	OntologyID       string             `json:"ontology_id,omitempty"`
	OntologyVersion  string             `json:"ontology_version,omitempty"`
	EntityName       string             `json:"entity_name,omitempty"`
	Templates        []businessTemplate `json:"templates"`
	TemplatesLimited bool               `json:"templates_limited"`
}

func catalogText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return text
}

func (s *Server) businessCatalog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, err := strconv.Atoi(q.Get("offset"))
	if q.Get("offset") != "" && (err != nil || offset < 0 || offset > 1000000) || len(q.Get("keyword")) > 256 {
		semanticFailure(w, model.Fail("invalid_input", "Invalid catalog search"))
		return
	}
	kind := q.Get("kind")
	view := q.Get("view")
	if view != "" && view != "queries" && view != "all" {
		semanticFailure(w, model.Fail("invalid_input", "Unknown catalog view"))
		return
	}
	if kind != "" && !slices.Contains([]string{"term", "metric", "object", "field", "relationship", "template", "entity_type", "property", "relation_type"}, kind) {
		semanticFailure(w, model.Fail("invalid_input", "Unknown semantic entry kind"))
		return
	}
	// One response and its page revision share the same publication/grant snapshot.
	// This administrative view never expands an Agent's source authorization.
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	p := model.Principal{AgentID: q.Get("agent_id"), Admin: q.Get("agent_id") == "", Preview: true}
	if !p.Admin {
		a, err := s.Store.Agent(p.AgentID)
		if err != nil || !a.Enabled || a.RevokedAt != nil || !a.ExpiresAt.After(time.Now()) {
			semanticFailure(w, model.Fail("not_found", "Agent unavailable; choose an active Agent"))
			return
		}
	}
	if id := q.Get("entry_id"); id != "" {
		out, err := s.Engine.SemanticEntry(p, q.Get("source_id"), id)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		if q.Get("published_version") != "" && q.Get("published_version") != out["published_version"] {
			semanticFailure(w, model.Fail("conflict", "Catalog changed; refresh before opening this entry"))
			return
		}
		if entry, ok := out["entry"].(semantic.Entry); ok && entry.Kind == "entity_type" {
			src, err := s.Engine.Authorize(p, q.Get("source_id"))
			if err != nil {
				semanticFailure(w, err)
				return
			}
			st, err := s.Store.Semantics(src.ID)
			if err != nil {
				semanticFailure(w, err)
				return
			}
			entries, err := s.Engine.PublishedEntries(src, st)
			if err != nil {
				semanticFailure(w, err)
				return
			}
			entity := entry.Definition.(ontology.Entity)
			contextEntries := []semantic.Entry{}
			bytes := 0
			for _, child := range entries {
				matches := false
				switch def := child.Definition.(type) {
				case ontology.Property:
					matches = def.Entity == entity.ID
				case ontology.Relation:
					matches = def.From == entity.ID || def.To == entity.ID
				}
				if !matches {
					continue
				}
				raw, _ := json.Marshal(child)
				if len(contextEntries) == 50 || bytes+len(raw) > 64<<10 {
					out["context_limited"] = true
					break
				}
				bytes += len(raw)
				contextEntries = append(contextEntries, child)
			}
			out["context_entries"] = contextEntries
		}
		write(w, 200, out)
		return
	}
	sources, err := s.Store.Sources()
	if err != nil {
		semanticFailure(w, err)
		return
	}
	items := []businessEntry{}
	hash := sha256.New()
	json.NewEncoder(hash).Encode([]string{p.AgentID, q.Get("source_id"), kind, q.Get("keyword"), view})
	scanned, limited := 0, false
	for _, src := range sources {
		if r.Context().Err() != nil {
			return
		}
		if q.Get("source_id") != "" && q.Get("source_id") != src.ID {
			continue
		}
		if _, err := s.Engine.Authorize(p, src.ID); err != nil {
			continue
		}
		if scanned == 100 {
			limited = true
			break
		}
		scanned++
		st, err := s.Store.Semantics(src.ID)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		json.NewEncoder(hash).Encode([]any{src.ID, src.Revision, st.PublishedVersion})
		entries, err := s.Engine.PublishedEntries(src, st)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		templates := map[string]businessTemplate{}
		entityNames := map[string]string{}
		for _, en := range entries {
			if entity, ok := en.Definition.(ontology.Entity); ok {
				entityNames[entity.ID] = entity.Name
			}
		}
		for _, en := range st.Published.Entries {
			if en.Template == nil {
				continue
			}
			v := s.Engine.PublishedTemplateValidation(src, en)
			templates[en.ID] = businessTemplate{en.ID, en.Name, en.Template.ExecutionVersion, v.Status, v.Valid}
		}
		for _, en := range entries {
			if en.Kind == "overview" || kind != "" && kind != en.Kind {
				continue
			}
			text := strings.ToLower(en.Name + " " + en.ID + " " + en.Description + " " + strings.Join(en.Aliases, " "))
			if !strings.Contains(text, strings.ToLower(q.Get("keyword"))) {
				continue
			}
			item := businessEntry{ID: en.ID, Kind: en.Kind, Name: en.Name, Description: catalogText(en.Description, 512), SourceID: src.ID, SourceName: src.Name, SourceKind: src.Kind, PublishedVersion: strconv.FormatInt(st.PublishedVersion, 10), Unit: catalogText(en.Unit, 128), Grain: catalogText(en.Grain, 256), TimeDefinition: catalogText(en.TimeDefinition, 256), Templates: []businessTemplate{}}
			if prop, ok := en.Definition.(ontology.Property); ok {
				item.Unit, item.TimeDefinition = catalogText(prop.Unit, 128), catalogText(prop.TimeDefinition, 256)
				item.EntityName = entityNames[prop.Entity]
			}
			if b := st.Published.Ontology; b != nil {
				item.OntologyID, item.OntologyVersion = b.OntologyID, strconv.FormatInt(b.Version, 10)
			}
			for _, other := range st.Published.Entries {
				if other.Template == nil || !(en.ID == other.ID || en.TemplateID == other.ID || slices.Contains(en.TemplateIDs, other.ID)) {
					continue
				}
				if len(item.Templates) == 20 {
					item.TemplatesLimited = true
					break
				}
				item.Templates = append(item.Templates, templates[other.ID])
			}
			if view == "queries" && (!slices.Contains([]string{"template", "metric", "entity_type"}, en.Kind) || !slices.ContainsFunc(item.Templates, func(t businessTemplate) bool { return t.Executable })) {
				continue
			}
			items = append(items, item)
		}
	}
	if !p.Admin {
		a, err := s.Store.Agent(p.AgentID)
		if err != nil || !a.Enabled || a.RevokedAt != nil || !a.ExpiresAt.After(time.Now()) {
			semanticFailure(w, model.Fail("not_found", "Agent unavailable; choose an active Agent"))
			return
		}
	}
	revision := hex.EncodeToString(hash.Sum(nil))
	if (offset > 0 && q.Get("revision") == "") || q.Get("revision") != "" && q.Get("revision") != revision {
		semanticFailure(w, model.Fail("conflict", "Catalog or access changed; restart the search"))
		return
	}
	order := map[string]int{"template": 0, "metric": 1, "entity_type": 2}
	slices.SortFunc(items, func(a, b businessEntry) int {
		if view == "queries" && a.Kind != b.Kind {
			return order[a.Kind] - order[b.Kind]
		}
		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		if n := strings.Compare(a.SourceName, b.SourceName); n != 0 {
			return n
		}
		return strings.Compare(a.SourceID+":"+a.ID, b.SourceID+":"+b.ID)
	})
	end := min(offset+20, len(items))
	page := items[min(offset, len(items)):end]
	write(w, 200, map[string]any{"entries": page, "total": len(items), "revision": revision, "sources_limited": limited, "sources_scanned": scanned})
}
