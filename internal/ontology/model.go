package ontology

import "time"

const FormatVersion = 1
const MaxItems = 500
const MaxBytes = 512 << 10

type Entity struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases,omitempty"`
	Description string   `json:"description,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Identity    []string `json:"identity,omitempty"`
}

type Property struct {
	ID             string   `json:"id"`
	Entity         string   `json:"entity"`
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases,omitempty"`
	Description    string   `json:"description,omitempty"`
	Type           string   `json:"type"`
	Required       bool     `json:"required"`
	Multiple       bool     `json:"multiple"`
	Unique         bool     `json:"unique,omitempty"`
	Unit           string   `json:"unit,omitempty"`
	Enums          []string `json:"enums,omitempty"`
	Minimum        string   `json:"minimum,omitempty"`
	Maximum        string   `json:"maximum,omitempty"`
	TimeDefinition string   `json:"time_definition,omitempty"`
}

type Cardinality struct {
	Min int  `json:"min"`
	Max *int `json:"max"`
}

type Relation struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Aliases         []string    `json:"aliases,omitempty"`
	Description     string      `json:"description,omitempty"`
	From            string      `json:"from"`
	To              string      `json:"to"`
	Directed        bool        `json:"directed"`
	FromCardinality Cardinality `json:"from_cardinality"`
	ToCardinality   Cardinality `json:"to_cardinality"`
}

type Definition struct {
	FormatVersion int        `json:"format_version"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	Entities      []Entity   `json:"entities"`
	Properties    []Property `json:"properties"`
	Relations     []Relation `json:"relations"`
}

type State struct {
	ID            string     `json:"id"`
	Revision      int64      `json:"revision,string"`
	LatestVersion int64      `json:"latest_version,string"`
	Archived      bool       `json:"archived"`
	Draft         Definition `json:"draft"`
}

type Version struct {
	OntologyID  string     `json:"ontology_id"`
	Version     int64      `json:"version,string"`
	PublishedAt time.Time  `json:"published_at"`
	Definition  Definition `json:"definition"`
}

type Reference struct {
	Namespace string `json:"namespace"`
	Object    string `json:"object"`
	Field     string `json:"field,omitempty"`
}

type EntityMapping struct {
	Entity      string      `json:"entity"`
	Objects     []Reference `json:"objects"`
	Description string      `json:"description,omitempty"`
}

type PropertyMapping struct {
	Property string `json:"property"`
	// Entity identifies the effective owner when mapping an inherited property.
	Entity      string     `json:"entity"`
	Reference   *Reference `json:"reference,omitempty"`
	TemplateID  string     `json:"template_id,omitempty"`
	Declared    bool       `json:"declared,omitempty"`
	Description string     `json:"description,omitempty"`
}

type FieldPair struct {
	From     Reference `json:"from"`
	To       Reference `json:"to"`
	Declared bool      `json:"declared,omitempty"`
}

type RelationMapping struct {
	Relation    string      `json:"relation"`
	Fields      []FieldPair `json:"fields,omitempty"`
	TemplateID  string      `json:"template_id,omitempty"`
	Description string      `json:"description,omitempty"`
}

type Binding struct {
	OntologyID string            `json:"ontology_id"`
	Version    int64             `json:"version,string"`
	Entities   []EntityMapping   `json:"entities"`
	Properties []PropertyMapping `json:"properties"`
	Relations  []RelationMapping `json:"relations"`
}

type Context struct {
	OntologyID  string   `json:"ontology_id"`
	Version     string   `json:"version"`
	ConceptRefs []string `json:"concept_refs"`
}

func Empty() Definition {
	return Definition{FormatVersion: FormatVersion, Entities: []Entity{}, Properties: []Property{}, Relations: []Relation{}}
}

// References occupy a namespace that legacy catalog IDs cannot use.
func Ref(kind, id string) string { return "ontology:" + kind + ":" + id }

func NormalizeDefinition(d *Definition) {
	if d.Entities == nil {
		d.Entities = []Entity{}
	}
	if d.Properties == nil {
		d.Properties = []Property{}
	}
	if d.Relations == nil {
		d.Relations = []Relation{}
	}
}

func NormalizeBinding(b *Binding) {
	if b == nil {
		return
	}
	if b.Entities == nil {
		b.Entities = []EntityMapping{}
	}
	if b.Properties == nil {
		b.Properties = []PropertyMapping{}
	}
	if b.Relations == nil {
		b.Relations = []RelationMapping{}
	}
	for i := range b.Entities {
		if b.Entities[i].Objects == nil {
			b.Entities[i].Objects = []Reference{}
		}
	}
}
