package adapter

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type matrixCase struct {
	Name      string        `json:"name"`
	Source    model.Source  `json:"source"`
	Namespace string        `json:"namespace"`
	Object    string        `json:"object"`
	Queries   []queryCheck  `json:"queries"`
	Denied    []model.Query `json:"denied"`
	Baseline  model.Query   `json:"baseline"`
}
type queryCheck struct {
	Query     model.Query     `json:"query"`
	Rows      int             `json:"rows"`
	Contains  string          `json:"contains"`
	Error     bool            `json:"error"`
	Truncated *bool           `json:"truncated,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// This suite runs against independently provisioned engines. Passing results, not
// driver compatibility, are the evidence used in the published support matrix.
func TestDatabaseMatrix(t *testing.T) {
	path := os.Getenv("MCPDBHUB_MATRIX")
	if path == "" {
		t.Skip("set MCPDBHUB_MATRIX to a fixture manifest")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var cases []matrixCase
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if e = d.Decode(&cases); e != nil {
		t.Fatal(e)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) { testDatabase(t, c) })
	}
}
func testDatabase(t *testing.T, c matrixCase) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c.Source.Limits.Defaults()
	conn, e := Open(ctx, c.Source)
	if e != nil {
		t.Fatalf("connect: %v", e)
	}
	defer conn.Close()
	p, e := conn.Probe(ctx)
	if e != nil || !p.Connected {
		t.Fatalf("probe: %+v %v", p, e)
	}
	if c.Source.Kind == "elasticsearch" || c.Source.Kind == "opensearch" {
		if c.Source.TLSMode != "verify" {
			t.Fatal("search fixture must use verified TLS")
		}
		untrusted := c.Source
		untrusted.CACert = ""
		bad, err := Open(ctx, untrusted)
		if err != nil {
			t.Fatal(err)
		}
		_, err = bad.Probe(ctx)
		bad.Close()
		if err == nil {
			t.Fatal("untrusted database certificate accepted")
		}
		// Only isolated fixtures receive this permission test. Production probes never write.
		httpDB := conn.(*httpConn)
		res, err := httpDB.request(ctx, http.MethodPut, "/events/_doc/permission-check", nil, map[string]any{"id": 99})
		if res != nil {
			res.Body.Close()
		}
		var denied *model.Error
		if !errors.As(err, &denied) || denied.Code != "database_permission" || denied.NativeCode != "HTTP 403" {
			t.Fatalf("database account did not deny writes: %v", err)
		}
	}
	for _, op := range []string{"namespaces", "objects", "describe"} {
		objects, e := conn.Discover(ctx, op, c.Namespace, c.Object)
		if e != nil || len(objects) == 0 {
			t.Fatalf("%s: %v %+v", op, e, objects)
		}
		if paged, ok := conn.(DiscoveryPager); ok {
			limits := c.Source.Limits
			limits.MaxRows = 1
			q := model.Query{Namespace: c.Namespace, Object: c.Object}
			var all []any
			for pages := 0; ; pages++ {
				if pages > len(objects) {
					t.Fatal("metadata cursor did not terminate")
				}
				r, err := paged.DiscoverPage(ctx, op, q, limits)
				if err != nil {
					t.Fatalf("%s metadata page: %v", op, err)
				}
				all = append(all, r.Data...)
				if r.NextCursor == "" {
					break
				}
				q.Cursor = r.NextCursor
			}
			want, _ := json.Marshal(objects)
			got, _ := json.Marshal(all)
			if !bytes.Equal(got, want) {
				t.Fatalf("%s metadata pagination lost or repeated objects", op)
			}
		}
	}
	before, e := conn.Query(ctx, c.Baseline, c.Source.Limits)
	if e != nil {
		t.Fatalf("baseline: %v", e)
	}
	beforeJSON, _ := json.Marshal(before.Data)
	for i, q := range c.Queries {
		limits := c.Source.Limits
		if q.Query.MaxRows > 0 {
			limits.MaxRows = min(limits.MaxRows, q.Query.MaxRows)
		}
		r, e := conn.Query(ctx, q.Query, limits)
		if q.Error {
			if e == nil {
				t.Errorf("query %d: expected error", i)
			}
			continue
		}
		if e != nil {
			t.Fatalf("query %d: %v", i, e)
		}
		if r.RowCount != q.Rows {
			t.Errorf("query %d rows=%d want=%d: %+v", i, r.RowCount, q.Rows, r)
		}
		if q.Truncated != nil && r.Truncated != *q.Truncated {
			t.Errorf("query %d truncated=%v want=%v", i, r.Truncated, *q.Truncated)
		}
		if len(q.Data) > 0 {
			actual, _ := json.Marshal(r.Data)
			var expected bytes.Buffer
			if err := json.Compact(&expected, q.Data); err != nil {
				t.Fatal(err)
			}
			if string(actual) != expected.String() {
				t.Errorf("query %d data=%s want=%s", i, actual, expected.Bytes())
			}
		}
		b, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		if q.Contains != "" && !strings.Contains(string(b), q.Contains) {
			t.Errorf("query %d missing %q: %s", i, q.Contains, b)
		}
	}
	for i, q := range c.Denied {
		if _, e := conn.Query(ctx, q, c.Source.Limits); e == nil {
			t.Errorf("dangerous query %d accepted", i)
		}
	}
	after, e := conn.Query(ctx, c.Baseline, c.Source.Limits)
	if e != nil {
		t.Fatal(e)
	}
	afterJSON, _ := json.Marshal(after.Data)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatal("target data changed after denied queries")
	}
	limited := c.Source.Limits
	limited.MaxRows = 1
	r, e := conn.Query(ctx, c.Baseline, limited)
	if e != nil {
		t.Fatal(e)
	}
	if r.RowCount > 1 {
		t.Fatal("row limit exceeded")
	}
	capability, _ := Get(c.Source.Kind)
	if before.RowCount > r.RowCount && r.NextCursor == "" && !r.Truncated {
		t.Fatal("incomplete result has neither continuation nor truncation")
	}
	if before.RowCount > 1 && (capability.Family == "mongodb" || capability.Family == "cql" || capability.Family == "search") && r.NextCursor == "" {
		t.Fatal("native pagination did not return a cursor")
	}
	if r.NextCursor != "" {
		nextQuery := c.Baseline
		all := append([]any{}, r.Data...)
		for pages := 0; r.NextCursor != ""; pages++ {
			if pages > before.RowCount {
				t.Fatal("pagination did not terminate")
			}
			nextQuery.Cursor = r.NextCursor
			next, err := conn.Query(ctx, nextQuery, limited)
			if err != nil {
				t.Fatalf("native cursor: %v", err)
			}
			if next.RowCount > 1 {
				t.Fatal("page row limit exceeded")
			}
			if next.Truncated {
				t.Fatal("native pagination discarded rows")
			}
			all = append(all, next.Data...)
			r = next
		}
		allJSON, _ := json.Marshal(all)
		if string(allJSON) != string(beforeJSON) {
			t.Fatalf("pagination changed data: %s want %s", allJSON, beforeJSON)
		}
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, e := conn.Query(cancelled, c.Baseline, c.Source.Limits); e == nil {
		t.Error("cancelled query accepted")
	}
	if !t.Failed() {
		t.Logf("VERIFIED %s %s", c.Name, p.ServerVersion)
		if dir := os.Getenv("MCPDBHUB_MATRIX_REPORT"); dir != "" {
			os.MkdirAll(dir, 0755)
			b, _ := json.MarshalIndent(map[string]any{"name": c.Name, "kind": c.Source.Kind, "version": p.ServerVersion, "query_cases": len(c.Queries), "denied_cases": len(c.Denied), "probe": p, "verified_at": time.Now().UTC(), "checks": []string{"connection", "namespaces", "objects", "describe", "queries", "parameters", "types", "empty", "error", "denied_operations", "unchanged_data", "row_limit", "cancel"}}, "", "  ")
			if e := os.WriteFile(filepath.Join(dir, c.Name+".json"), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestLocalDatabaseReadOnly(t *testing.T) {
	for _, kind := range []string{"sqlite", "duckdb"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "fixture.db")
			driver := kind
			if kind == "sqlite" {
				driver = "sqlite3"
			}
			db, e := sql.Open(driver, path)
			if e != nil {
				t.Fatal(e)
			}
			_, e = db.Exec("CREATE TABLE events(id BIGINT, amount DECIMAL(20,2), note VARCHAR); INSERT INTO events VALUES (1,12.50,'one'),(2,20.25,'two'),(3,4.00,'three')")
			db.Close()
			if e != nil {
				t.Fatal(e)
			}
			formatQuery := "SELECT format('%s','ok') AS value"
			if kind == "duckdb" {
				formatQuery = "SELECT format('{}','ok') AS value"
			}
			testDatabase(t, matrixCase{Name: kind, Source: model.Source{Kind: kind, Path: path, TLSMode: "disable"}, Namespace: "main", Object: "events", Baseline: model.Query{Query: "SELECT * FROM events ORDER BY id"}, Queries: []queryCheck{
				{Query: model.Query{Query: "WITH totals AS (SELECT id,amount FROM events WHERE id>=?) SELECT id, sum(amount) OVER () AS total FROM totals ORDER BY id", Params: []any{int64(1)}}, Rows: 3},
				{Query: model.Query{Query: "SELECT ? AS large_value", Params: []any{int64(9007199254740993)}}, Rows: 1, Contains: "9007199254740993"},
				{Query: model.Query{Query: "SELECT ? AS binary_value", Params: []any{[]byte{0, 255}}}, Rows: 1, Contains: "AP8="},
				{Query: model.Query{Query: "SELECT * FROM events WHERE id=99"}, Rows: 0},
				{Query: model.Query{Query: "SELECT replace('abc','a','x') AS value"}, Rows: 1, Contains: "xbc"},
				{Query: model.Query{Query: formatQuery}, Rows: 1, Contains: "ok"},
				{Query: model.Query{Query: "SELECT missing FROM events"}, Error: true},
			}, Denied: []model.Query{{Query: "DELETE FROM events"}, {Query: "SELECT 1; DELETE FROM events"}, {Query: "WITH t AS (DELETE FROM events RETURNING *) SELECT * FROM t"}, {Query: "SELECT load_extension('evil')"}, {Query: "ATTACH '/tmp/other.db' AS other"}, {Query: "COPY events TO '/tmp/escape.csv'"}, {Query: "SELECT * FROM read_csv('/etc/passwd')"}}})
		})
	}
}

func TestQueryGuards(t *testing.T) {
	for _, kind := range []string{"mysql", "mariadb", "tidb"} {
		if err := guardSQL(kind, "WITH totals AS (SELECT id FROM events) SELECT id, 'FOR SHARE' AS note FROM totals"); err != nil {
			t.Errorf("%s rejected a non-locking CTE: %v", kind, err)
		}
		for _, query := range []string{
			"SELECT * FROM events FOR SHARE",
			"SELECT * FROM events FOR SHARE NOWAIT",
			"SELECT * FROM events FOR SHARE SKIP LOCKED",
			"SELECT * FROM events WHERE id IN (SELECT id FROM events FOR SHARE)",
			"WITH locked AS (SELECT * FROM events FOR SHARE) SELECT * FROM locked",
		} {
			if err := guardSQL(kind, query); err == nil {
				t.Errorf("%s accepted a locking read: %s", kind, query)
			}
		}
	}
	for _, kind := range []string{"postgres", "mysql", "clickhouse", "sqlite", "duckdb"} {
		for _, query := range []string{"SELECT replace('abc','a','x')", "SELECT format('%s','ok')", "SELECT truncate(1.25,1)"} {
			if err := guardSQL(kind, query); err != nil {
				t.Errorf("%s rejected scalar function %s: %v", kind, query, err)
			}
		}
	}
	for _, kind := range []string{"postgres", "mysql", "clickhouse", "cassandra"} {
		for _, query := range []string{"SELECT 1;DELETE FROM events", "WITH t AS (DELETE FROM events RETURNING *) SELECT * FROM t", "SELECT * FROM events FOR UPDATE", "SELECT dangerous_function()", "SELECT 1 INTO OUTFILE '/tmp/file'", "SELECT 1 /*!; DELETE FROM events */", "REPLACE INTO events VALUES(1)", "TRUNCATE TABLE events", "SELECT 1 FORMAT JSON", "SELECT replace('abc','a','x'); DELETE FROM events"} {
			if e := guardSQL(kind, query); e == nil {
				t.Errorf("%s accepted %s", kind, query)
			}
		}
	}
	for _, q := range []string{"MATCH (n) DELETE n", "CALL dbms.shutdown()", "MATCH (n) RETURN n; CREATE ()", "LOAD CSV FROM 'https://evil' AS row RETURN row"} {
		if e := guardCypher(q); e == nil {
			t.Errorf("Cypher accepted %s", q)
		}
	}
	for _, q := range []string{`import "http" from(bucket:"x")`, `from(bucket:"x") |> to(bucket:"out")`, `requests.get(url:"http://evil")`, `from(bucket:"x",host:"http://evil",token:"x")`, `from(bucket:"${die(msg: \"oops\")}")`} {
		if e := guardFlux(q); e == nil {
			t.Errorf("Flux accepted %s", q)
		}
	}
}

func TestFluxParametersRemainLiteral(t *testing.T) {
	payload := `${die(msg:"executed")}`
	ast, err := fluxExtern(map[string]any{"region": payload, "large": json.Number("9007199254740993")})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(ast)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Body []struct {
			Init struct {
				Properties []struct {
					Key   struct{ Name string }
					Value struct {
						Type  string
						Value any
					}
				}
			}
		}
	}
	if err = json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	for _, p := range file.Body[0].Init.Properties {
		if p.Key.Name == "region" && (p.Value.Type != "StringLiteral" || p.Value.Value != payload) {
			t.Fatal("parameter was interpreted as code")
		}
		if p.Key.Name == "large" && (p.Value.Type != "IntegerLiteral" || p.Value.Value != "9007199254740993") {
			t.Fatal("integer precision lost")
		}
	}
	if _, err = fluxExtern(map[string]any{"large": json.Number("9223372036854775808")}); err == nil {
		t.Fatal("overflow must not become a rounded float")
	}
}

func TestHTTPSCertificateAndFixedReadPaths(t *testing.T) {
	calls := 0
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":{"number":"fixture"}}`))
	}))
	defer remote.Close()
	u, _ := url.Parse(remote.URL)
	port, _ := strconv.Atoi(u.Port())
	source := model.Source{Kind: "elasticsearch", Host: u.Hostname(), Port: port, Database: "events", TLSMode: "verify"}
	source.Limits.Defaults()
	ctx := context.Background()
	conn, err := Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Probe(ctx); err == nil {
		t.Fatal("untrusted certificate accepted")
	}
	conn.Close()
	source.CACert = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: remote.Certificate().Raw}))
	conn, err = Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.Probe(ctx); err != nil {
		t.Fatal(err)
	}
	before := calls
	for _, id := range []string{"../_delete_by_query", "..", "%2e%2e", "a/b", "a?refresh=true"} {
		if _, err = conn.Query(ctx, model.Query{Object: "events", Operation: "get", Query: id}, source.Limits); err == nil {
			t.Fatal("unsafe document path accepted")
		}
	}
	if calls != before {
		t.Fatal("denied path reached database")
	}
}

func TestRedisLengthBudgetBeforeAllocation(t *testing.T) {
	for _, wire := range []string{"$1000000000\r\n", "*1000000000\r\n"} {
		a, b := net.Pipe()
		go func() { defer b.Close(); io.WriteString(b, wire) }()
		guarded := newReadBudgetConn(a, 4096)
		header := make([]byte, 64)
		if n, err := guarded.Read(header); n != 0 || err == nil {
			t.Fatal("oversized header reached driver")
		}
		a.Close()
	}
	a, b := net.Pipe()
	wire := "*2\r\n$6\r\n$100\r\n\r\n:1\r\n"
	go func() { defer b.Close(); io.WriteString(b, wire) }()
	defer a.Close()
	data, err := io.ReadAll(newReadBudgetConn(a, 4096))
	if err != nil || string(data) != wire {
		t.Fatalf("bulk content was interpreted as a RESP header: %q %v", data, err)
	}
}

func TestTemplateTargetsCannotBeParameters(t *testing.T) {
	for _, test := range []struct {
		kind, version, query string
		denied               bool
	}{
		{"influxdb", "2", `from(bucket:params.bucket) |> range(start:-1h)`, true},
		{"influxdb", "2", `from(bucket:"orders" + params.suffix) |> range(start:-1h)`, true},
		{"influxdb", "2", `from(bucket:"orders") |> range(start:-1h) |> filter(fn:(r)=>r.region == params.region)`, false},
		{"neo4j", "", `MATCH (n:$($label)) RETURN n`, true},
		{"neo4j", "", `MATCH (n:Event {id:$id}) RETURN n`, false},
		{"clickhouse", "", `SELECT * FROM {table:Identifier}`, true},
		{"clickhouse", "", `SELECT * FROM orders WHERE id={id:Int64}`, false},
	} {
		err := CheckTemplateTargets(model.Source{Kind: test.kind, Version: test.version}, model.Query{Query: test.query})
		if (err != nil) != test.denied {
			t.Fatalf("%s: %v", test.query, err)
		}
	}
}
