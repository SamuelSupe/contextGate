package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SamuelSupe/mcpdbhub/internal/ontology"
	"reflect"
	"time"
)

type Limits struct {
	TimeoutSeconds int `json:"timeout_seconds"`
	MaxRows        int `json:"max_rows"`
	MaxBytes       int `json:"max_bytes"`
	Concurrency    int `json:"concurrency"`
}

func (l *Limits) Defaults() {
	if l.TimeoutSeconds == 0 {
		l.TimeoutSeconds = 30
	}
	if l.MaxRows == 0 {
		l.MaxRows = 1000
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 5 << 20
	}
	if l.Concurrency == 0 {
		l.Concurrency = 4
	}
}
func (l Limits) Validate() error {
	if l.TimeoutSeconds < 1 || l.TimeoutSeconds > 120 || l.MaxRows < 1 || l.MaxRows > 10000 || l.MaxBytes < 1024 || l.MaxBytes > 20<<20 || l.Concurrency < 1 || l.Concurrency > 16 {
		return errors.New("limits outside allowed range")
	}
	return nil
}

type Source struct {
	ObservedVersion    string            `json:"observed_version,omitempty"`
	QueryAccessMode    string            `json:"query_access_mode"`
	ConnectionRevision int64             `json:"connection_revision,string"`
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Kind               string            `json:"kind"`
	Version            string            `json:"version,omitempty"`
	Host               string            `json:"host,omitempty"`
	Port               int               `json:"port,omitempty"`
	Database           string            `json:"database,omitempty"`
	Username           string            `json:"username,omitempty"`
	Password           string            `json:"password,omitempty"`
	Token              string            `json:"token,omitempty"`
	Path               string            `json:"path,omitempty"`
	TLSMode            string            `json:"tls_mode"`
	CACert             string            `json:"ca_cert,omitempty"`
	Options            map[string]string `json:"options,omitempty"`
	Enabled            bool              `json:"enabled"`
	Limits             Limits            `json:"limits"`
	Revision           int64             `json:"revision,string"`
	QueryRevision      int64             `json:"query_revision,string"`
	HasSecret          bool              `json:"has_secret"`
	AuthMode           string            `json:"auth_mode,omitempty"`
	Probe              *Probe            `json:"probe,omitempty"`
}

// Display edits have their own revision; active queries and cursors only depend
// on changes to the connection, access state or execution limits.
func (s Source) ExecutionRevision() int64 {
	if s.QueryRevision != 0 {
		return s.QueryRevision
	}
	return s.Revision
}

func (s Source) SameQueryConfig(other Source) bool {
	s.Name, other.Name = "", ""
	s.ConnectionRevision, other.ConnectionRevision = 0, 0
	s.ObservedVersion, other.ObservedVersion = "", ""
	s.QueryAccessMode, other.QueryAccessMode = s.QueryMode(), other.QueryMode()
	s.Revision, other.Revision = 0, 0
	s.QueryRevision, other.QueryRevision = 0, 0
	s.Probe, other.Probe = nil, nil
	s.HasSecret, other.HasSecret = false, false
	s.AuthMode, other.AuthMode = s.Public().AuthMode, other.Public().AuthMode
	if len(s.Options) == 0 {
		s.Options = nil
	}
	if len(other.Options) == 0 {
		other.Options = nil
	}
	return reflect.DeepEqual(s, other)
}

func (s Source) QueryMode() string {
	if s.QueryAccessMode == "" {
		return "native_and_templates"
	}
	return s.QueryAccessMode
}

func (s Source) SameConnection(other Source) bool {
	s.Limits, other.Limits = Limits{}, Limits{}
	s.Enabled, other.Enabled = false, false
	s.QueryAccessMode, other.QueryAccessMode = "", ""
	return s.SameQueryConfig(other)
}

func (s Source) Public() Source {
	s.QueryAccessMode = s.QueryMode()
	s.HasSecret = s.Password != "" || s.Token != ""
	if s.AuthMode == "" {
		s.AuthMode = "none"
		if s.Token != "" {
			s.AuthMode = "token"
		} else if s.Username != "" || s.Password != "" {
			s.AuthMode = "password"
		}
	}
	s.Password = ""
	s.Token = ""
	return s
}

type Probe struct {
	Connected        bool      `json:"connected"`
	Protection       string    `json:"protection"`
	PermissionStatus string    `json:"permission_status"`
	Evidence         []string  `json:"evidence"`
	ServerVersion    string    `json:"server_version,omitempty"`
	CheckedAt        time.Time `json:"checked_at"`
	Error            *Error    `json:"error,omitempty"`
}
type Agent struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Sources   []string   `json:"sources"`
	Enabled   bool       `json:"enabled"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	AuthType  string     `json:"auth_type"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	Revision  int64      `json:"revision,string"`
	ClientID  string     `json:"client_id,omitempty"`
}
type Principal struct {
	AgentID         string
	Admin           bool
	Preview         bool
	CredentialValid func() bool
}
type Query struct {
	SourceID       string          `json:"source_id"`
	Query          string          `json:"query,omitempty"`
	Params         []any           `json:"params,omitempty"`
	NamedParams    map[string]any  `json:"named_params,omitempty"`
	Namespace      string          `json:"namespace,omitempty"`
	Object         string          `json:"object,omitempty"`
	Operation      string          `json:"operation,omitempty"`
	Filter         json.RawMessage `json:"filter,omitempty"`
	Projection     json.RawMessage `json:"projection,omitempty"`
	Sort           json.RawMessage `json:"sort,omitempty"`
	Pipeline       json.RawMessage `json:"pipeline,omitempty"`
	Body           json.RawMessage `json:"body,omitempty"`
	Command        string          `json:"command,omitempty"`
	Args           []string        `json:"args,omitempty"`
	Language       string          `json:"language,omitempty"`
	Cursor         string          `json:"cursor,omitempty"`
	MaxRows        int             `json:"max_rows,omitempty"`
	TimeoutSeconds int             `json:"timeout_seconds,omitempty"`
	MaxBytes       int             `json:"max_bytes,omitempty"`
}
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type Result struct {
	OntologyContext *ontology.Context `json:"ontology_context,omitempty"`
	SemanticVersion string            `json:"semantic_version,omitempty"`
	TemplateID      string            `json:"template_id,omitempty"`
	TemplateVersion string            `json:"template_version,omitempty"`
	RequestID       string            `json:"request_id,omitempty"`
	Format          string            `json:"format"`
	Columns         []Column          `json:"columns,omitempty"`
	Data            []any             `json:"data"`
	RowCount        int               `json:"row_count"`
	ElapsedMS       int64             `json:"elapsed_ms"`
	Truncated       bool              `json:"truncated"`
	NextCursor      string            `json:"next_cursor,omitempty"`
	Bytes           int               `json:"bytes"`
}

func NewResult(format string) *Result { return &Result{Format: format, Data: []any{}} }
func (r *Result) Add(v any, l Limits) (bool, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return false, err
	}
	// Reserve room for MCP's structured result and its JSON text fallback.
	if r.RowCount >= l.MaxRows || (r.Bytes+len(b))*3 > l.MaxBytes-1024 {
		r.Truncated = true
		return false, nil
	}
	r.Data = append(r.Data, v)
	r.RowCount++
	r.Bytes += len(b)
	return true, nil
}

type Object struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace,omitempty"`
	Type      string   `json:"type"`
	Columns   []Column `json:"columns,omitempty"`
	Details   any      `json:"details,omitempty"`
}
type Audit struct {
	TemplateID      string    `json:"template_id,omitempty"`
	TemplateVersion string    `json:"template_version,omitempty"`
	OntologyID      string    `json:"ontology_id,omitempty"`
	OntologyVersion string    `json:"ontology_version,omitempty"`
	RequestID       string    `json:"request_id"`
	NativeCode      string    `json:"native_code,omitempty"`
	Preview         bool      `json:"preview"`
	ID              int64     `json:"id"`
	At              time.Time `json:"at"`
	AgentID         string    `json:"agent_id"`
	SourceID        string    `json:"source_id"`
	Operation       string    `json:"operation"`
	Fingerprint     string    `json:"fingerprint"`
	ElapsedMS       int64     `json:"elapsed_ms"`
	Rows            int       `json:"rows"`
	ErrorCode       string    `json:"error_code,omitempty"`
}
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
	NativeCode string `json:"native_code,omitempty"`
}

func (e *Error) Error() string    { return e.Code + ": " + e.Message }
func Fail(code, msg string) error { return &Error{Code: code, Message: msg} }
func ErrorCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "query_failed"
}
func String(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
