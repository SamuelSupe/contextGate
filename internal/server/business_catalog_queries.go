package server

import (
	"strconv"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

func catalogEntry(src model.Source, st semantic.State, en semantic.Entry) businessEntry {
	return businessEntry{
		ID: en.ID, Kind: en.Kind, Name: en.Name, Description: catalogText(en.Description, 512),
		SourceID: src.ID, SourceName: src.Name, SourceKind: src.Kind,
		PublishedVersion: strconv.FormatInt(st.PublishedVersion, 10),
		Unit:             catalogText(en.Unit, 128), Grain: catalogText(en.Grain, 256), TimeDefinition: catalogText(en.TimeDefinition, 256),
		Templates: []businessTemplate{},
	}
}

// Only the administrator catalog calls this projection. It never returns query
// definitions, parameters or trial results, and never expands Agent discovery.
func (s *Server) managementQueryEntries(src model.Source, st semantic.State, view, keyword string) []businessEntry {
	changes := map[string]string{}
	for _, change := range semanticDiff(st.Published, st.Draft) {
		changes[change.ID] = change.Change
	}
	entries := map[string]semantic.Entry{}
	published := map[string]semantic.Entry{}
	for _, en := range st.Published.Entries {
		if en.Template != nil {
			entries[en.ID], published[en.ID] = en, en
		}
	}
	if view == "drafts" {
		for _, en := range st.Draft.Entries {
			if en.Template != nil {
				entries[en.ID] = en
			}
		}
	}
	items := []businessEntry{}
	for _, en := range entries {
		if !strings.Contains(strings.ToLower(en.Name+" "+en.ID+" "+en.Description+" "+strings.Join(en.Aliases, " ")), strings.ToLower(keyword)) {
			continue
		}
		if view == "drafts" && changes[en.ID] == "" {
			continue
		}
		item := catalogEntry(src, st, en)
		if view == "drafts" {
			item.DraftChange = changes[en.ID]
		}
		if pub, ok := published[en.ID]; ok {
			validation := s.Engine.PublishedTemplateValidation(src, pub)
			item.QueryStatus = validation.Status
			if !src.Enabled {
				item.QueryStatus = "source_disabled"
			}
			item.Templates = []businessTemplate{{pub.ID, pub.Name, pub.Template.ExecutionVersion, item.QueryStatus, validation.Valid && src.Enabled}}
			if view == "attention" && (!pub.Template.Enabled || validation.Valid && src.Enabled) {
				continue
			}
		}
		items = append(items, item)
	}
	return items
}
