package model

// HTTP API configuration is administrator-owned. Query callers select an
// operation ID and typed values; they cannot supply URLs, methods or headers.
type HTTPAPIConfig struct {
	BaseURL        string          `json:"base_url"`
	TokenHeader    string          `json:"token_header,omitempty"`
	ProbeOperation string          `json:"probe_operation"`
	Operations     []HTTPOperation `json:"operations"`
}

type HTTPOperation struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Method          string          `json:"method"`
	Path            string          `json:"path"`
	ReadOnly        bool            `json:"read_only"`
	BodyJSON        string          `json:"body_json,omitempty"`
	Parameters      []HTTPParameter `json:"parameters,omitempty"`
	ExampleJSON     string          `json:"example_json"`
	ResponsePointer string          `json:"response_pointer,omitempty"`
	Columns         []Column        `json:"columns,omitempty"`
	Pagination      *HTTPPagination `json:"pagination,omitempty"`
}

type HTTPParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Target      string `json:"target"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	DefaultJSON string `json:"default_json,omitempty"`
	EnumJSON    string `json:"enum_json,omitempty"`
	Minimum     string `json:"minimum,omitempty"`
	Maximum     string `json:"maximum,omitempty"`
}

// Only opaque tokens are followed, never upstream-supplied links or URLs.
type HTTPPagination struct {
	QueryParameter string `json:"query_parameter"`
	NextPointer    string `json:"next_pointer"`
}
