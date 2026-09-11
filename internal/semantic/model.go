package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const FormatVersion = 1
const MaxEntries = 500
const MaxSnapshotBytes = 768 << 10

type Reference struct {
	Namespace string `json:"namespace"`
	Object    string `json:"object"`
	Field     string `json:"field,omitempty"`
}

type Entry struct {
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	Name           string            `json:"name"`
	Aliases        []string          `json:"aliases,omitempty"`
	Description    string            `json:"description,omitempty"`
	Reference      *Reference        `json:"reference,omitempty"`
	DataType       string            `json:"data_type,omitempty"`
	Unit           string            `json:"unit,omitempty"`
	Enums          map[string]string `json:"enums,omitempty"`
	TimeDefinition string            `json:"time_definition,omitempty"`
	Grain          string            `json:"grain,omitempty"`
	Caveats        string            `json:"caveats,omitempty"`
	Related        []Reference       `json:"related,omitempty"`
	TemplateID     string            `json:"template_id,omitempty"`
	Template       *Template         `json:"template,omitempty"`
}

// JSON documents are strings in configuration APIs so editing in a browser does
// not round database integers or decimals through JavaScript's number type.
type Template struct {
	Enabled           bool        `json:"enabled"`
	Tool              string      `json:"tool"`
	QueryJSON         string      `json:"query_json"`
	Parameters        []Parameter `json:"parameters"`
	ExampleJSON       string      `json:"example_json"`
	ResultDescription string      `json:"result_description,omitempty"`
	ExecutionVersion  string      `json:"execution_version,omitempty"`
}

type Parameter struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Pointers    []string `json:"pointers"`
	DefaultJSON string   `json:"default_json,omitempty"`
	EnumJSON    string   `json:"enum_json,omitempty"`
	Minimum     string   `json:"minimum,omitempty"`
	Maximum     string   `json:"maximum,omitempty"`
}

type Snapshot struct {
	FormatVersion int     `json:"format_version"`
	Overview      string  `json:"overview"`
	Entries       []Entry `json:"entries"`
}

type State struct {
	Revision         int64    `json:"revision,string"`
	PublishedVersion int64    `json:"published_version,string"`
	Draft            Snapshot `json:"draft"`
	Published        Snapshot `json:"published"`
}

type Evidence struct {
	Definition string    `json:"definition"`
	Connection string    `json:"connection"`
	CheckedAt  time.Time `json:"checked_at"`
}

type Execution struct {
	SourceID         string         `json:"source_id"`
	TemplateID       string         `json:"template_id"`
	ExecutionVersion string         `json:"execution_version"`
	Parameters       map[string]any `json:"parameters"`
	Cursor           string         `json:"cursor,omitempty"`
	MaxRows          int            `json:"max_rows,omitempty"`
	MaxBytes         int            `json:"max_bytes,omitempty"`
	TimeoutSeconds   int            `json:"timeout_seconds,omitempty"`
}

func Definition(t Template) string {
	t.ExecutionVersion, t.ResultDescription, t.ExampleJSON = "", "", ""
	// Description and examples do not change executable behavior.
	t.Parameters = append([]Parameter(nil), t.Parameters...)
	for i := range t.Parameters {
		t.Parameters[i].Description = ""
	}
	if v, err := Parse(t.QueryJSON); err == nil {
		b, _ := json.Marshal(v)
		t.QueryJSON = string(b)
	}
	b, _ := json.Marshal(t)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func Empty() Snapshot { return Snapshot{FormatVersion: FormatVersion, Entries: []Entry{}} }
