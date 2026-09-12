package ontology

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strings"
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,95}$`)

func Bounded(d Definition) error {
	b, err := json.Marshal(d)
	if err != nil || len(b) > MaxBytes || len(d.Entities)+len(d.Properties)+len(d.Relations) > MaxItems {
		return fmt.Errorf("ontology exceeds 500 definitions or 512 KiB")
	}
	if len(d.Name) > 256 || len(d.Description) > 32<<10 {
		return fmt.Errorf("ontology name exceeds 256 bytes or description exceeds 32 KiB")
	}
	if d.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported ontology format version")
	}
	return nil
}

func Validate(d Definition) error {
	if err := Bounded(d); err != nil {
		return err
	}
	if strings.TrimSpace(d.Name) == "" || len(d.Name) > 256 {
		return fmt.Errorf("ontology name is required (at most 256 bytes)")
	}
	ids := map[string]bool{}
	check := func(kind, id, name string) error {
		key := kind + ":" + id
		if !identifier.MatchString(id) || ids[key] || strings.TrimSpace(name) == "" || len(name) > 256 {
			return fmt.Errorf("invalid or duplicate %s ID/name: %s", kind, id)
		}
		ids[key] = true
		return nil
	}
	entities := map[string]Entity{}
	props := map[string]Property{}
	for _, en := range d.Entities {
		if err := check("entity", en.ID, en.Name); err != nil {
			return err
		}
		entities[en.ID] = en
	}
	for _, en := range d.Entities {
		seen := map[string]bool{en.ID: true}
		for parent := en.Parent; parent != ""; parent = entities[parent].Parent {
			if _, ok := entities[parent]; !ok {
				return fmt.Errorf("missing parent %s", parent)
			}
			if seen[parent] {
				return fmt.Errorf("inheritance cycle at %s", parent)
			}
			seen[parent] = true
		}
	}
	for _, p := range d.Properties {
		if err := check("property", p.ID, p.Name); err != nil {
			return err
		}
		if _, ok := entities[p.Entity]; !ok {
			return fmt.Errorf("property %s has unknown entity", p.ID)
		}
		if !slices.Contains([]string{"string", "boolean", "integer", "decimal", "number", "date", "datetime", "duration", "binary", "object"}, p.Type) {
			return fmt.Errorf("unsupported logical type on %s", p.ID)
		}
		var bounds [2]*big.Rat
		for i, v := range []string{p.Minimum, p.Maximum} {
			if v == "" {
				continue
			}
			if !slices.Contains([]string{"integer", "decimal", "number"}, p.Type) || len(v) > 256 || strings.ContainsAny(v, "eE/") {
				return fmt.Errorf("invalid numeric range on %s", p.ID)
			}
			n, ok := new(big.Rat).SetString(v)
			if !ok || p.Type == "integer" && !n.IsInt() {
				return fmt.Errorf("invalid numeric bound on %s", p.ID)
			}
			bounds[i] = n
		}
		if bounds[0] != nil && bounds[1] != nil && bounds[0].Cmp(bounds[1]) > 0 {
			return fmt.Errorf("contradictory range on %s", p.ID)
		}
		props[p.ID] = p
	}
	for _, en := range d.Entities {
		chain := Ancestors(d, en.ID)
		names := map[string]bool{}
		for _, p := range d.Properties {
			if !slices.Contains(chain, p.Entity) {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(p.Name))
			if names[name] {
				return fmt.Errorf("inherited property name conflict on %s: %s", en.ID, p.Name)
			}
			names[name] = true
		}
		keys := map[string]bool{}
		for _, id := range en.Identity {
			p, ok := props[id]
			if !ok || !slices.Contains(chain, p.Entity) || p.Multiple || !p.Required || keys[id] {
				return fmt.Errorf("invalid identity property %s on %s (must be effective, required and single-valued)", id, en.ID)
			}
			keys[id] = true
		}
	}
	for _, r := range d.Relations {
		if err := check("relation", r.ID, r.Name); err != nil {
			return err
		}
		if _, ok := entities[r.From]; !ok {
			return fmt.Errorf("unknown relation origin %s", r.ID)
		}
		if _, ok := entities[r.To]; !ok {
			return fmt.Errorf("unknown relation target %s", r.ID)
		}
		for _, c := range []Cardinality{r.FromCardinality, r.ToCardinality} {
			if c.Min < 0 || c.Min > 2147483647 || c.Max != nil && (*c.Max < 0 || *c.Max < c.Min || *c.Max > 2147483647) {
				return fmt.Errorf("contradictory cardinality on %s", r.ID)
			}
		}
	}
	return nil
}

func Ancestors(d Definition, id string) []string {
	parents := map[string]string{}
	for _, e := range d.Entities {
		parents[e.ID] = e.Parent
	}
	out := []string{}
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		out = append(out, id)
		seen[id] = true
		id = parents[id]
	}
	return out
}

func PropertyRef(entity, property string) string { return Ref("property", entity+":"+property) }

func MappedRefs(b Binding) map[string]bool {
	out := map[string]bool{}
	for _, m := range b.Entities {
		out[Ref("entity_type", m.Entity)] = true
	}
	for _, m := range b.Properties {
		out[PropertyRef(m.Entity, m.Property)] = true
	}
	for _, m := range b.Relations {
		out[Ref("relation_type", m.Relation)] = true
	}
	return out
}

// Mapping validation describes references only; it never constructs a query.
func ValidateBinding(d Definition, b Binding, templates map[string]bool) error {
	if err := Validate(d); err != nil {
		return err
	}
	if b.OntologyID == "" || b.Version < 1 {
		return fmt.Errorf("select an immutable ontology version")
	}
	if len(b.Entities)+len(b.Properties)+len(b.Relations) > MaxItems {
		return fmt.Errorf("mapping exceeds 500 entries")
	}
	entities := map[string]Entity{}
	props := map[string]Property{}
	relations := map[string]Relation{}
	for _, v := range d.Entities {
		entities[v.ID] = v
	}
	for _, v := range d.Properties {
		props[v.ID] = v
	}
	for _, v := range d.Relations {
		relations[v.ID] = v
	}
	mapped := map[string]EntityMapping{}
	validRef := func(r Reference, field bool) bool {
		return r.Object != "" && len(r.Object) <= 1024 && len(r.Namespace) <= 1024 && len(r.Field) <= 2048 && (!field || r.Field != "")
	}
	for _, m := range b.Entities {
		if _, ok := entities[m.Entity]; !ok {
			return fmt.Errorf("unknown entity mapping %s", m.Entity)
		}
		if _, ok := mapped[m.Entity]; ok || len(m.Objects) == 0 {
			return fmt.Errorf("entity %s requires unique mapping and physical objects", m.Entity)
		}
		seen := map[Reference]bool{}
		for _, r := range m.Objects {
			if !validRef(r, false) || r.Field != "" || seen[r] {
				return fmt.Errorf("invalid or duplicate object mapping on %s", m.Entity)
			}
			seen[r] = true
		}
		mapped[m.Entity] = m
	}
	belongs := func(entity string, ref Reference) bool {
		ref.Field = ""
		return slices.Contains(mapped[entity].Objects, ref)
	}
	seen := map[string]bool{}
	for _, m := range b.Properties {
		p, ok := props[m.Property]
		key := PropertyRef(m.Entity, m.Property)
		if !ok || len(mapped[m.Entity].Objects) == 0 || !slices.Contains(Ancestors(d, m.Entity), p.Entity) || seen[key] {
			return fmt.Errorf("invalid, duplicate or unmapped property owner: %s", key)
		}
		seen[key] = true
		if (m.Reference == nil) == (m.TemplateID == "") {
			return fmt.Errorf("property %s requires exactly one field or template", key)
		}
		if m.Reference != nil && (!validRef(*m.Reference, true) || !belongs(m.Entity, *m.Reference)) {
			return fmt.Errorf("property field must belong to its entity: %s", key)
		}
		if m.TemplateID != "" && !templates[m.TemplateID] {
			return fmt.Errorf("property %s references missing or disabled template", key)
		}
	}
	for _, m := range b.Relations {
		r, ok := relations[m.Relation]
		key := Ref("relation_type", m.Relation)
		if !ok || len(mapped[r.From].Objects) == 0 || len(mapped[r.To].Objects) == 0 || seen[key] {
			return fmt.Errorf("relation %s requires mapped endpoints and a unique mapping", m.Relation)
		}
		seen[key] = true
		if len(m.Fields) == 0 && m.TemplateID == "" {
			return fmt.Errorf("relation %s requires field pairs or a template", m.Relation)
		}
		if m.TemplateID != "" && !templates[m.TemplateID] {
			return fmt.Errorf("relation %s references missing or disabled template", m.Relation)
		}
		for _, pair := range m.Fields {
			if !validRef(pair.From, true) || !validRef(pair.To, true) || !belongs(r.From, pair.From) || !belongs(r.To, pair.To) {
				return fmt.Errorf("field pair endpoints do not match relation %s", m.Relation)
			}
		}
	}
	return nil
}
