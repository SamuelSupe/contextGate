package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/adapter"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

func (e *Engine) ValidateOntologyMapping(st semantic.State) error {
	b := st.Draft.Ontology
	templates := map[string]bool{}
	for _, en := range st.Draft.Entries {
		if en.Template != nil && en.Template.Enabled {
			templates[en.ID] = true
		}
	}
	visible := map[string]bool{}
	if b != nil {
		v, err := e.Store.OntologyVersion(b.OntologyID, b.Version)
		if err != nil {
			return err
		}
		if err = ontology.ValidateBinding(v.Definition, *b, templates); err != nil {
			return model.Fail("invalid_mapping", err.Error())
		}
		owner, err := e.Store.Ontology(b.OntologyID)
		if err != nil {
			return err
		}
		prior := st.Published.Ontology
		if owner.Archived && (prior == nil || prior.OntologyID != b.OntologyID || prior.Version != b.Version) {
			return model.Fail("invalid_mapping", "Archived ontologies cannot receive new bindings or version adoptions")
		}
		visible = ontology.MappedRefs(*b)
	}
	for _, en := range st.Draft.Entries {
		if en.Template == nil {
			continue
		}
		seen := map[string]bool{}
		for _, ref := range en.Template.ConceptRefs {
			if !visible[ref] || seen[ref] {
				return model.Fail("invalid_mapping", "Template "+en.ID+" references an unmapped or duplicate concept: "+ref)
			}
			seen[ref] = true
		}
	}
	return nil
}

func mappingProof(b *ontology.Binding) string {
	if b == nil {
		return ""
	}
	// Descriptions and associations have no effect on physical discovery evidence.
	copy := *b
	copy.Entities = append([]ontology.EntityMapping(nil), b.Entities...)
	copy.Properties = append([]ontology.PropertyMapping(nil), b.Properties...)
	copy.Relations = append([]ontology.RelationMapping(nil), b.Relations...)
	ontology.NormalizeBinding(&copy)
	for i := range copy.Entities {
		copy.Entities[i].Description = ""
	}
	for i := range copy.Properties {
		copy.Properties[i].Description = ""
		copy.Properties[i].TemplateID = ""
	}
	for i := range copy.Relations {
		copy.Relations[i].Description = ""
		copy.Relations[i].TemplateID = ""
	}
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(raw)
	return "mapping:" + hex.EncodeToString(sum[:])
}

type MappingCheck struct {
	Reference ontology.Reference `json:"reference"`
	Status    string             `json:"status"`
}

// CheckOntologyMapping uses metadata APIs only. In particular, Neo4j property
// discovery is excluded because its existing describe operation reads nodes.
func (e *Engine) CheckOntologyMapping(ctx context.Context, source string, revision int64) ([]MappingCheck, error) {
	src, err := e.Authorize(model.AdministratorPrincipal(ctx), source)
	if err != nil {
		return nil, err
	}
	st, err := e.Store.Semantics(source)
	if err != nil {
		return nil, err
	}
	if st.Revision != revision {
		return nil, model.Fail("conflict", "Mapping draft changed; reload before checking")
	}
	if err = e.ValidateOntologyMapping(st); err != nil {
		return nil, err
	}
	checks := []MappingCheck{}
	b := st.Draft.Ontology
	if b == nil {
		return checks, nil
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(src.Limits.TimeoutSeconds)*time.Second)
	defer cancel()
	objects := map[ontology.Reference]bool{}
	fields := map[ontology.Reference]bool{}
	namespaces := map[string]bool{}
	for _, m := range b.Entities {
		for _, ref := range m.Objects {
			namespaces[ref.Namespace] = true
		}
	}
	for ns := range namespaces {
		cursor := ""
		for page := 0; page < 20; page++ {
			res, err := e.Execute(ctx, model.AdministratorPrincipal(ctx), "objects", model.Query{SourceID: source, Namespace: ns, Cursor: cursor})
			if err != nil {
				return nil, err
			}
			for _, raw := range res.Data {
				var obj model.Object
				data, _ := json.Marshal(raw)
				if json.Unmarshal(data, &obj) != nil {
					return nil, model.Fail("invalid_mapping", "Unreadable structure metadata")
				}
				objects[ontology.Reference{Namespace: ns, Object: obj.Name}] = true
			}
			if res.Truncated {
				return nil, model.Fail("limit_exceeded", "Object discovery was truncated; raise source limits before checking mappings")
			}
			cursor = res.NextCursor
			if cursor == "" {
				break
			}
			if page == 19 {
				return nil, model.Fail("limit_exceeded", "Mapping discovery exceeds 20 pages")
			}
		}
	}
	checkedObjects := map[ontology.Reference]bool{}
	for _, m := range b.Entities {
		for _, ref := range m.Objects {
			if !objects[ref] {
				return nil, model.Fail("invalid_mapping", "Physical object was not discovered: "+ref.Namespace+"."+ref.Object)
			}
			if checkedObjects[ref] {
				continue
			}
			checkedObjects[ref] = true
			checks = append(checks, MappingCheck{Reference: ref, Status: "verified"})
			if src.Kind == "neo4j" {
				continue
			}
			res, err := e.Execute(ctx, model.AdministratorPrincipal(ctx), "describe", model.Query{SourceID: source, Namespace: ref.Namespace, Object: ref.Object})
			if err != nil {
				return nil, err
			}
			if res.Truncated || res.NextCursor != "" {
				return nil, model.Fail("limit_exceeded", "Field discovery was truncated; raise source limits before checking mappings")
			}
			for _, raw := range res.Data {
				var obj model.Object
				data, _ := json.Marshal(raw)
				if err = json.Unmarshal(data, &obj); err != nil {
					return nil, err
				}
				cols := obj.Columns
				if adapter.ForSource(src).Tool == "query_sql" || adapter.ForSource(src).Tool == "query_cql" || src.Kind == "influxdb" {
					cols = append(cols, model.Column{Name: obj.Name})
				}
				for _, col := range cols {
					r := ref
					r.Field = col.Name
					fields[r] = true
				}
			}
		}
	}
	checkField := func(ref ontology.Reference, declared bool) error {
		status := "verified"
		if !fields[ref] {
			if !declared {
				return model.Fail("invalid_mapping", "Field was not discovered; explicitly declare it unverified or correct it: "+ref.Object+"."+ref.Field)
			}
			status = "unverified"
		}
		checks = append(checks, MappingCheck{Reference: ref, Status: status})
		return nil
	}
	for _, m := range b.Properties {
		if m.Reference != nil {
			if err = checkField(*m.Reference, m.Declared); err != nil {
				return nil, err
			}
		}
	}
	for _, m := range b.Relations {
		for _, pair := range m.Fields {
			if err = checkField(pair.From, pair.Declared); err != nil {
				return nil, err
			}
			if err = checkField(pair.To, pair.Declared); err != nil {
				return nil, err
			}
		}
	}
	e.Store.Mutations.Lock()
	defer e.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(ctx); err != nil {
		return nil, err
	}
	fresh, err := e.Store.Source(source)
	if err != nil {
		return nil, err
	}
	latest, err := e.Store.Semantics(source)
	if err != nil {
		return nil, err
	}
	if latest.Revision != revision || e.connectionProof(src) != e.connectionProof(fresh) {
		return nil, model.Fail("conflict", "Mapping or connection changed during discovery")
	}
	details, _ := json.Marshal(checks)
	err = e.Store.SaveSemanticEvidence(source, semantic.Evidence{Definition: mappingProof(b), Connection: e.connectionProof(src), CheckedAt: time.Now().UTC(), DetailsJSON: string(details)})
	return checks, err
}

func ontologyContext(snapshot semantic.Snapshot, refs []string) *ontology.Context {
	b := snapshot.Ontology
	if b == nil {
		return nil
	}
	return &ontology.Context{OntologyID: b.OntologyID, Version: strconv.FormatInt(b.Version, 10), ConceptRefs: append([]string{}, refs...)}
}

func (e *Engine) ontologySummary(snapshot semantic.Snapshot) any {
	b := snapshot.Ontology
	if b == nil {
		return nil
	}
	_, err := e.Store.OntologyVersion(b.OntologyID, b.Version)
	return map[string]any{"id": b.OntologyID, "version": strconv.FormatInt(b.Version, 10), "available": err == nil && len(b.Entities) > 0}
}

func (e *Engine) ontologyEntries(src model.Source, snapshot semantic.Snapshot) ([]semantic.Entry, error) {
	b := snapshot.Ontology
	if b == nil {
		return nil, nil
	}
	v, err := e.Store.OntologyVersion(b.OntologyID, b.Version)
	if err != nil {
		return nil, err
	}
	d := v.Definition
	out := []semantic.Entry{}
	entities := map[string]ontology.Entity{}
	props := map[string]ontology.Property{}
	relations := map[string]ontology.Relation{}
	for _, x := range d.Entities {
		entities[x.ID] = x
	}
	for _, x := range d.Properties {
		props[x.ID] = x
	}
	for _, x := range d.Relations {
		relations[x.ID] = x
	}
	visible := ontology.MappedRefs(*b)
	for _, m := range b.Entities {
		x, ok := entities[m.Entity]
		if !ok {
			continue
		}
		for _, key := range x.Identity {
			if !visible[ontology.PropertyRef(m.Entity, key)] {
				x.Identity = nil
				break
			}
		}
		ancestors := []ontology.Entity{}
		for _, id := range ontology.Ancestors(d, x.ID)[1:] {
			a := entities[id]
			ancestors = append(ancestors, ontology.Entity{ID: a.ID, Name: a.Name, Parent: a.Parent})
		}
		out = append(out, semantic.Entry{ID: ontology.Ref("entity_type", x.ID), Kind: "entity_type", Name: x.Name, Aliases: x.Aliases, Description: x.Description, Definition: x, Mapping: m, Ancestors: ancestors})
	}
	for _, m := range b.Properties {
		x, ok := props[m.Property]
		if !ok || !visible[ontology.Ref("entity_type", m.Entity)] {
			continue
		}
		x.Entity = m.Entity
		out = append(out, semantic.Entry{ID: ontology.PropertyRef(m.Entity, x.ID), Kind: "property", Name: x.Name, Aliases: x.Aliases, Description: x.Description, Definition: x, Mapping: m, TemplateID: m.TemplateID})
	}
	for _, m := range b.Relations {
		x, ok := relations[m.Relation]
		if !ok || !visible[ontology.Ref("entity_type", x.From)] || !visible[ontology.Ref("entity_type", x.To)] {
			continue
		}
		out = append(out, semantic.Entry{ID: ontology.Ref("relation_type", x.ID), Kind: "relation_type", Name: x.Name, Aliases: x.Aliases, Description: x.Description, Definition: x, Mapping: m, TemplateID: m.TemplateID})
	}
	validation := e.MappingValidation(src, snapshot)
	fieldStatus := map[ontology.Reference]string{}
	if checks, ok := validation["checks"].([]MappingCheck); ok {
		for _, check := range checks {
			fieldStatus[check.Reference] = check.Status
		}
	}
	for i := range out {
		raw, _ := json.Marshal(out[i].Mapping)
		var mapping map[string]any
		json.Unmarshal(raw, &mapping)
		status := validation["status"].(string)
		if status == "checked" {
			status = "verified"
			switch m := out[i].Mapping.(type) {
			case ontology.PropertyMapping:
				if m.Reference == nil {
					status = "query_template"
				} else {
					status = fieldStatus[*m.Reference]
				}
			case ontology.RelationMapping:
				for _, pair := range m.Fields {
					if fieldStatus[pair.From] != "verified" || fieldStatus[pair.To] != "verified" {
						status = "unverified"
					}
				}
				if len(m.Fields) == 0 {
					status = "query_template"
				}
			}
		}
		mapping["verification_status"] = status
		if checked, ok := validation["checked_at"]; ok {
			mapping["checked_at"] = checked
		}
		out[i].Mapping = mapping
		for _, en := range snapshot.Entries {
			if en.Template != nil && (en.ID == out[i].TemplateID || slices.Contains(en.Template.ConceptRefs, out[i].ID)) {
				out[i].TemplateIDs = append(out[i].TemplateIDs, en.ID)
			}
		}
	}
	return out, nil
}

func (e *Engine) MappingValidation(src model.Source, snapshot semantic.Snapshot) map[string]any {
	if snapshot.Ontology == nil {
		return map[string]any{"status": "unbound"}
	}
	ev, err := e.Store.SemanticEvidence(src.ID, mappingProof(snapshot.Ontology))
	if err != nil {
		return map[string]any{"status": "check_required"}
	}
	if ev.Connection != e.connectionProof(src) {
		return map[string]any{"status": "expired", "checked_at": ev.CheckedAt}
	}
	var checks []MappingCheck
	if json.Unmarshal([]byte(ev.DetailsJSON), &checks) != nil {
		return map[string]any{"status": "check_required"}
	}
	return map[string]any{"status": "checked", "checked_at": ev.CheckedAt, "checks": checks}
}
