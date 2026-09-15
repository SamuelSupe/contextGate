package adapter

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/model"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Capability struct {
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Family      string   `json:"family"`
	Tool        string   `json:"tool"`
	Port        int      `json:"port"`
	Protection  string   `json:"protection"`
	Parameters  bool     `json:"parameters"`
	Pagination  string   `json:"pagination"`
	Example     any      `json:"example"`
	Limitations []string `json:"limitations"`
	Verified    []string `json:"verified_versions"`
}

//go:embed verified.json
var verifiedJSON []byte

func Catalog() []Capability {
	out := []Capability{
		{"http_api", "HTTP API", "http_api", "query_http_api", 443, "declared_read_api", true, "configured opaque token", map[string]any{"operation": "list_customers", "named_params": map[string]any{}}, []string{"Administrator-declared GET and read-only POST JSON operations only; redirects and arbitrary URLs are denied", "Read-only behavior and response fields are declared by the administrator, not verified against upstream permissions", "API contract version must be updated when the upstream contract changes"}, nil},
		{"postgres", "PostgreSQL", "sql", "query_sql", 5432, "read_only_transaction", true, "explicit SQL", map[string]any{"query": "SELECT $1::bigint AS value", "params": []any{42}}, []string{"Custom functions and external access are denied"}, nil},
		{"mysql", "MySQL", "sql", "query_sql", 3306, "read_only_transaction", true, "explicit SQL", map[string]any{"query": "SELECT ? AS value", "params": []any{42}}, nil, nil},
		{"mariadb", "MariaDB", "sql", "query_sql", 3306, "read_only_transaction", true, "explicit SQL", map[string]any{"query": "SELECT ? AS value", "params": []any{42}}, nil, nil},
		{"tidb", "TiDB", "sql", "query_sql", 4000, "select_privileges", true, "explicit SQL", map[string]any{"query": "SELECT ? AS value", "params": []any{42}}, []string{"A SELECT-only database account is required"}, nil},
		{"cockroachdb", "CockroachDB", "sql", "query_sql", 26257, "read_only_transaction", true, "explicit SQL", map[string]any{"query": "SELECT $1::int AS value", "params": []any{42}}, nil, nil},
		{"timescaledb", "TimescaleDB", "sql", "query_sql", 5432, "read_only_transaction", true, "explicit SQL", map[string]any{"query": "SELECT now() AS time"}, nil, nil},
		{"sqlite", "SQLite", "sql", "query_sql", 0, "read_only_file", true, "explicit SQL", map[string]any{"query": "SELECT ? AS value", "params": []any{42}}, []string{"Only files in the configured database directory; ATTACH and extensions denied"}, nil},
		{"duckdb", "DuckDB", "sql", "query_sql", 0, "read_only_file", true, "explicit SQL", map[string]any{"query": "SELECT ?::BIGINT AS value", "params": []any{42}}, []string{"CGO required; external files and extensions disabled"}, nil},
		{"clickhouse", "ClickHouse", "sql", "query_sql", 9000, "readonly_setting", true, "explicit SQL", map[string]any{"query": "SELECT {value:Int64} AS value", "named_params": map[string]any{"value": 42}}, []string{"External table functions and user-defined functions denied"}, nil},
		{"mongodb", "MongoDB", "mongodb", "query_mongodb", 27017, "read_role", true, "native cursor (5 minute TTL)", map[string]any{"operation": "find", "object": "events", "filter": map[string]any{}}, []string{"No JavaScript, $out or $merge"}, nil},
		{"redis", "Redis", "redis", "query_redis", 6379, "command_allowlist", true, "native scan", map[string]any{"command": "SCAN", "args": []string{"0", "COUNT", "100"}}, []string{"Explicit safe command list; no scripts, KEYS or blocking commands"}, nil},
		{"valkey", "Valkey", "redis", "query_redis", 6379, "command_allowlist", true, "native scan", map[string]any{"command": "SCAN", "args": []string{"0", "COUNT", "100"}}, nil, nil},
		{"elasticsearch", "Elasticsearch", "search", "query_search", 9200, "query_api", true, "search_after", map[string]any{"operation": "search", "object": "events", "body": map[string]any{"query": map[string]any{"match_all": map[string]any{}}}}, []string{"Read APIs only; scripts, remote indexes and arbitrary paths denied"}, nil},
		{"opensearch", "OpenSearch", "search", "query_search", 9200, "query_api", true, "search_after", map[string]any{"operation": "search", "object": "events", "body": map[string]any{"query": map[string]any{"match_all": map[string]any{}}}}, nil, nil},
		{"neo4j", "Neo4j", "cypher", "query_cypher", 7687, "engine_classification", true, "explicit Cypher", map[string]any{"query": "MATCH (n) RETURN n LIMIT $limit", "named_params": map[string]any{"limit": 10}}, []string{"EXPLAIN must classify the query as read-only; procedures and LOAD CSV denied"}, nil},
		{"cassandra", "Cassandra", "cql", "query_cql", 9042, "select_privileges", true, "native page state", map[string]any{"query": "SELECT * FROM events LIMIT 10"}, []string{"SELECT-only account required; no user-defined functions"}, nil},
		{"scylladb", "ScyllaDB", "cql", "query_cql", 9042, "select_privileges", true, "native page state", map[string]any{"query": "SELECT * FROM events LIMIT 10"}, nil, nil},
		{"influxdb", "InfluxDB", "influxdb", "query_influxdb", 8086, "query_api", true, "explicit query", map[string]any{"language": "influxql", "query": "SELECT * FROM events LIMIT 10"}, []string{"1.x: InfluxQL; 2.x: restricted Flux; 3 Core: SQL/InfluxQL query API isolation, not a read-only account"}, nil},
	}
	var verified map[string][]string
	_ = json.Unmarshal(verifiedJSON, &verified)
	for i := range out {
		out[i].Verified = verified[out[i].Kind]
		if out[i].Verified == nil {
			out[i].Verified = []string{}
		}
		if out[i].Limitations == nil {
			out[i].Limitations = []string{}
		}
	}
	return out
}
func Get(kind string) (Capability, bool) {
	for _, c := range Catalog() {
		if c.Kind == kind {
			return c, true
		}
	}
	return Capability{}, false
}

type SourceCapability struct {
	ExampleJSON string `json:"example_json,omitempty"`
	Capability
	Version   string   `json:"version,omitempty"`
	Languages []string `json:"languages,omitempty"`
}

func ForSource(s model.Source) SourceCapability {
	c, _ := Get(s.Kind)
	out := SourceCapability{Capability: c, Version: s.Version}
	if s.Kind == "http_api" && s.HTTPAPI != nil && len(s.HTTPAPI.Operations) > 0 {
		op := s.HTTPAPI.Operations[0]
		var params any
		d := json.NewDecoder(strings.NewReader(op.ExampleJSON))
		d.UseNumber()
		_ = d.Decode(&params)
		out.Example = map[string]any{"operation": op.ID, "named_params": params}
		raw, _ := json.Marshal(out.Example)
		out.ExampleJSON = string(raw)
	}
	if s.Kind != "influxdb" {
		return out
	}
	if out.Version == "" {
		out.Version = "2"
	}
	switch out.Version {
	case "1":
		out.Name = "InfluxDB 1.x"
		out.Languages = []string{"influxql"}
		out.Limitations = []string{"InfluxQL SELECT queries; writes and SELECT INTO denied"}
	case "2":
		out.Name = "InfluxDB 2.x"
		out.Languages = []string{"flux"}
		bucket := s.Options["bucket"]
		if bucket == "" {
			bucket = "your-bucket"
		}
		out.Example = map[string]any{"language": "flux", "query": "from(bucket: " + strconv.Quote(bucket) + ") |> range(start: -1h) |> limit(n: 10)"}
		out.Limitations = []string{"Restricted Flux; imports, writes and external access denied"}
	case "3":
		out.Name, out.Port = "InfluxDB 3 Core", 8181
		out.Languages = []string{"sql", "influxql"}
		out.Example = map[string]any{"language": "sql", "query": "SELECT * FROM events LIMIT 10"}
		out.Limitations = []string{"Query API isolation; the configured token is not claimed to have database read-only permissions"}
	}
	out.Verified = []string{}
	for _, version := range c.Verified {
		if strings.HasPrefix(version, out.Version+".") {
			out.Verified = append(out.Verified, version)
		}
	}
	return out
}

type Connection interface {
	Probe(context.Context) (model.Probe, error)
	Discover(context.Context, string, string, string) ([]model.Object, error)
	Query(context.Context, model.Query, model.Limits) (*model.Result, error)
	Close() error
}

// DiscoveryPager returns a bounded metadata page and an adapter continuation
// state. The execution layer binds that state to identity, operation and query.
type DiscoveryPager interface {
	DiscoverPage(context.Context, string, model.Query, model.Limits) (*model.Result, error)
}

func Open(ctx context.Context, s model.Source) (Connection, error) {
	c, ok := Get(s.Kind)
	if !ok {
		return nil, errors.New("unsupported database")
	}
	switch c.Family {
	case "sql":
		return openSQL(ctx, s)
	case "mongodb":
		return openMongo(ctx, s)
	case "redis":
		return openRedis(ctx, s)
	case "http_api":
		return openHTTPAPI(ctx, s)
	case "search", "influxdb":
		return openHTTP(ctx, s)
	case "cypher":
		return openNeo4j(ctx, s)
	case "cql":
		return openCQL(ctx, s)
	}
	return nil, errors.New("unsupported adapter")
}
func ValidateSource(s *model.Source, fileRoot string) error {
	c, ok := Get(s.Kind)
	if !ok {
		return errors.New("unsupported database kind")
	}
	if s.Kind == "influxdb" {
		if s.Version == "" {
			s.Version = "2"
		}
		if s.Version != "1" && s.Version != "2" && s.Version != "3" {
			return errors.New("InfluxDB version must be 1, 2 or 3")
		}
		c = ForSource(*s).Capability
	}
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" || len(s.Name) > 120 {
		return errors.New("name must contain 1-120 bytes")
	}
	s.Limits.Defaults()
	if e := s.Limits.Validate(); e != nil {
		return e
	}
	if s.Options == nil {
		s.Options = map[string]string{}
	}
	for k, v := range s.Options {
		if !wordset("org bucket auth_source replica_set sentinel_master")[k] || len(v) > 1024 {
			return errors.New("unsupported connection option")
		}
	}
	if s.TLSMode == "" {
		s.TLSMode = "verify"
	}
	if s.TLSMode != "verify" && s.TLSMode != "disable" {
		return errors.New("tls_mode must be verify or disable")
	}
	if s.Kind == "http_api" {
		return validateHTTPAPI(s)
	}
	if s.HTTPAPI != nil {
		return errors.New("http_api configuration requires an HTTP API source")
	}
	if s.Kind == "sqlite" || s.Kind == "duckdb" {
		root, e := filepath.EvalSymlinks(fileRoot)
		if e != nil {
			return fmt.Errorf("database directory: %w", e)
		}
		path, e := filepath.EvalSymlinks(s.Path)
		if e != nil {
			return fmt.Errorf("database file: %w", e)
		}
		path, e = filepath.Abs(path)
		if e != nil {
			return e
		}
		root, e = filepath.Abs(root)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, path)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return errors.New("database file must be inside configured database directory")
		}
		info, e := os.Stat(path)
		if e != nil || !info.Mode().IsRegular() {
			return errors.New("database file must exist and be regular")
		}
		s.Path = path
		s.Host = ""
		s.Port = 0
	} else {
		if s.Host == "" || strings.ContainsAny(s.Host, "/\\?#@\r\n\t ") {
			return errors.New("host must be a hostname or IP, without a URL, path or credentials")
		}
		if s.Port == 0 {
			s.Port = c.Port
		}
		if s.Port < 1 || s.Port > 65535 {
			return errors.New("invalid port")
		}
	}
	if strings.ContainsAny(s.Database, "\x00\r\n") {
		return errors.New("invalid database")
	}
	if s.Kind == "influxdb" && s.Version == "" {
		s.Version = "2"
	}
	if s.Kind == "influxdb" && s.Version != "1" && s.Version != "2" && s.Version != "3" {
		return errors.New("InfluxDB version must be 1, 2 or 3")
	}
	return nil
}
func address(s model.Source) string { return net.JoinHostPort(s.Host, fmt.Sprint(s.Port)) }
func baseURL(s model.Source) string {
	scheme := "https"
	if s.TLSMode == "disable" {
		scheme = "http"
	}
	return (&url.URL{Scheme: scheme, Host: address(s)}).String()
}
