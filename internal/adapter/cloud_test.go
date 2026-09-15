package adapter

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

// These fixtures check the documented wire contracts, not vendor compatibility.
func cloudFixture(t *testing.T, kind string, handle http.HandlerFunc) *cloudConn {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("missing bearer credential")
		}
		w.Header().Set("Content-Type", "application/json")
		handle(w, r)
	}))
	t.Cleanup(srv.Close)
	s := model.Source{Kind: kind, Database: "test-project", Host: "cloud.example.com", Port: 443, TLSMode: "verify", Token: "fixture-secret", AuthMode: "token", Version: "1", Options: map[string]string{"warehouse": "COMPUTE_WH", "warehouse_id": "warehouse1", "schema": "public", "role": "READER", "location": "US", "maximum_bytes_billed": "1073741824", "read_only_confirmed": "true"}}
	s.Limits.Defaults()
	conn, err := openCloud(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	c := conn.(*cloudConn)
	c.http.base = srv.URL
	c.http.client.Transport = srv.Client().Transport
	t.Cleanup(func() { c.Close() })
	return c
}

func readCloudRequest(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	if err := d.Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCloudSQLGuardAndNativeBindings(t *testing.T) {
	for _, kind := range []string{"snowflake", "databricks", "bigquery"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int32
			c := cloudFixture(t, kind, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.Error(w, "should not execute", 500) })
			for _, query := range []string{
				"DELETE FROM customers", "SELECT 1; DELETE FROM customers", "WITH removed AS (DELETE FROM customers RETURNING *) SELECT * FROM removed",
				"SELECT * INTO backup FROM customers", "SELECT * FROM customers FOR UPDATE", "SELECT external_query('secret','DELETE FROM x')", "SELECT ai_query('model','prompt')",
				"SELECT identifier('customers')", "SELECT pg_catalog.abs(1)", "SELECT system$send_email('x')", "SELECT /*+ hint */ 1", "SELECT 'a\\'; DELETE FROM x --'",
			} {
				if _, err := c.Query(context.Background(), model.Query{Query: query}, c.s.Limits); err == nil {
					t.Errorf("accepted %s", query)
				}
			}
			if calls.Load() != 0 {
				t.Fatal("denied SQL reached provider")
			}
			marker := map[string]string{"snowflake": "?", "databricks": ":id", "bigquery": "@id"}[kind]
			query := "WITH totals AS (SELECT customer_id, SUM(amount) AS total FROM orders WHERE customer_id=" + marker + " GROUP BY customer_id) SELECT customer_id, total, RANK() OVER (ORDER BY total DESC) FROM totals"
			if err := guardCloudSQL(kind, query); err != nil {
				t.Fatal(err)
			}
			if err := guardCloudSQL(kind, "SELECT 'DELETE; CALL fake()' AS description"); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, kind := range []string{"snowflake", "bigquery"} {
		_, v, err := cloudParameter(kind, json.Number("12345678901234567890.123456789"))
		if err != nil || v == nil || *v != "12345678901234567890.123456789" {
			t.Fatalf("decimal changed: %s %v", kind, err)
		}
	}
	if _, _, err := cloudParameter("databricks", map[string]any{"type": "DECIMAL(2,3)", "value": "1.234"}); err == nil {
		t.Fatal("decimal scale exceeds precision")
	}
	if _, _, err := cloudParameter("databricks", json.Number("1.234")); err == nil {
		t.Fatal("decimal precision must be explicit")
	}
	typ, v, err := cloudParameter("databricks", map[string]any{"type": "DECIMAL(38,9)", "value": "12345678901234567890.123456789"})
	if err != nil || typ != "DECIMAL(38,9)" || *v != "12345678901234567890.123456789" {
		t.Fatal("typed decimal failed", err)
	}
	for _, query := range []string{"COPY customers FROM 's3://bucket'", "UNLOAD ('SELECT 1') TO 's3://bucket'", "SELECT * FROM customers FOR UPDATE"} {
		if err := guardSQL("redshift", query); err == nil {
			t.Fatalf("Redshift accepted %s", query)
		}
	}
}

func TestSnowflakeStatementPartitionsAndLimits(t *testing.T) {
	var submits atomic.Int32
	c := cloudFixture(t, "snowflake", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			submits.Add(1)
			body := readCloudRequest(t, r)
			if body["statement"] != "SELECT ? AS value" || body["role"] != "READER" || body["parameters"].(map[string]any)["MULTI_STATEMENT_COUNT"] != "1" {
				t.Error("incorrect Snowflake contract")
			}
			binding := body["bindings"].(map[string]any)["1"].(map[string]any)
			if binding["value"] != "9007199254740993" || binding["type"] != "FIXED" {
				t.Error("lossy parameter")
			}
			w.WriteHeader(202)
			w.Write([]byte(`{"statementHandle":"statement1","code":"333334"}`))
		case r.URL.Query().Get("partition") == "1":
			w.Write([]byte(`{"data":[[null]]}`))
		default:
			w.Write([]byte(`{"statementHandle":"statement1","resultSetMetaData":{"numRows":2,"rowType":[{"name":"value","type":"fixed"}],"partitionInfo":[{"rowCount":1},{"rowCount":1}]},"data":[["9007199254740993"]]}`))
		}
	})
	q := model.Query{Query: "SELECT ? AS value", Params: []any{json.Number("9007199254740993")}}
	r, err := c.Query(context.Background(), q, c.s.Limits)
	if err != nil || r.RowCount != 2 || r.Data[0].([]any)[0] != "9007199254740993" || r.Data[1].([]any)[0] != nil {
		t.Fatal("partition decode", r, err)
	}
	l := c.s.Limits
	l.MaxRows = 1
	r, err = c.Query(context.Background(), q, l)
	if err != nil || !r.Truncated || r.NextCursor != "" || r.RowCount != 1 || submits.Load() != 2 {
		t.Fatal("unsafe truncation", r, err)
	}
}

func TestDatabricksChunksAndCancellation(t *testing.T) {
	var canceled atomic.Bool
	var pending atomic.Bool
	c := cloudFixture(t, "databricks", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cancel"):
			canceled.Store(true)
			w.Write([]byte(`{}`))
		case r.Method == http.MethodPost:
			body := readCloudRequest(t, r)
			if body["disposition"] != "INLINE" || body["warehouse_id"] != "warehouse1" || body["statement"] != "SELECT :id AS id" {
				t.Error("incorrect Databricks contract")
			}
			p := body["parameters"].([]any)[0].(map[string]any)
			if p["name"] != "id" || p["type"] != "BIGINT" || p["value"] != "9007199254740993" {
				t.Error("lossy named parameter")
			}
			w.Write([]byte(`{"statement_id":"statement1","status":{"state":"PENDING"}}`))
		case strings.HasSuffix(r.URL.Path, "/chunks/1"):
			w.Write([]byte(`{"chunk_index":1,"data_array":[["2"]]}`))
		case pending.Load():
			w.Write([]byte(`{"statement_id":"statement1","status":{"state":"RUNNING"}}`))
		default:
			w.Write([]byte(`{"statement_id":"statement1","status":{"state":"SUCCEEDED"},"manifest":{"schema":{"columns":[{"name":"id","type_text":"BIGINT"}]},"total_row_count":2},"result":{"chunk_index":0,"next_chunk_index":1,"next_chunk_internal_link":"https://evil.invalid/credentials","data_array":[["9007199254740993"]]}}`))
		}
	})
	q := model.Query{Query: "SELECT :id AS id", NamedParams: map[string]any{"id": json.Number("9007199254740993")}}
	r, err := c.Query(context.Background(), q, c.s.Limits)
	if err != nil || r.RowCount != 2 || r.Data[0].([]any)[0] != "9007199254740993" {
		t.Fatal(r, err)
	}
	pending.Store(true)
	ctx, done := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer done()
	if _, err = c.Query(ctx, q, c.s.Limits); err == nil || !canceled.Load() {
		t.Fatal("remote statement was not canceled", err)
	}
}

func TestBigQueryDryRunTypesPagingAndMetadata(t *testing.T) {
	var reject atomic.Bool
	var executions atomic.Int32
	c := cloudFixture(t, "bigquery", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/jobs"):
			body := readCloudRequest(t, r)
			cfg := body["configuration"].(map[string]any)
			if cfg["dryRun"] == true {
				if reject.Load() {
					w.Write([]byte(`{"statistics":{"query":{"statementType":"SCRIPT"}}}`))
					return
				}
				w.Write([]byte(`{"statistics":{"query":{"statementType":"SELECT"}}}`))
				return
			}
			executions.Add(1)
			query := cfg["query"].(map[string]any)
			if query["useLegacySql"] != false || query["maximumBytesBilled"] != "1073741824" {
				t.Error("missing billing or GoogleSQL boundary")
			}
			param := query["queryParameters"].([]any)[0].(map[string]any)
			if param["parameterValue"].(map[string]any)["value"] != "9007199254740993" {
				t.Error("lossy BigQuery parameter")
			}
			if body["jobReference"].(map[string]any)["location"] != "US" {
				t.Error("missing location")
			}
			w.Write([]byte(`{"status":{"state":"RUNNING"}}`))
		case strings.Contains(r.URL.Path, "/queries/"):
			if r.URL.Query().Get("pageToken") == "next-page" {
				w.Write([]byte(`{"jobComplete":true,"rows":[{"f":[{"v":"2"},{"v":null},{"v":[]}]}]}`))
				return
			}
			w.Write([]byte(`{"jobComplete":true,"schema":{"fields":[{"name":"id","type":"INTEGER"},{"name":"record","type":"RECORD","fields":[{"name":"amount","type":"NUMERIC"}]},{"name":"labels","type":"STRING","mode":"REPEATED"}]},"rows":[{"f":[{"v":"9007199254740993"},{"v":{"f":[{"v":"1234567890.123456789"}]}},{"v":[{"v":"one"},{"v":"two"}]}]}],"pageToken":"next-page"}`))
		case strings.HasSuffix(r.URL.Path, "/datasets"):
			w.Write([]byte(`{"datasets":[{"datasetReference":{"datasetId":"sales"}}],"nextPageToken":"more"}`))
		default:
			t.Error("unexpected path", r.URL.Path)
			http.Error(w, "fixture-secret", 500)
		}
	})
	q := model.Query{Query: "SELECT @id AS id", NamedParams: map[string]any{"id": json.Number("9007199254740993")}}
	r, err := c.Query(context.Background(), q, c.s.Limits)
	if err != nil || r.RowCount != 2 {
		t.Fatal(r, err)
	}
	row := r.Data[0].([]any)
	if row[0] != "9007199254740993" || row[1].(map[string]any)["amount"] != "1234567890.123456789" || len(row[2].([]any)) != 2 {
		t.Fatal("nested types lost", row)
	}
	reject.Store(true)
	if _, err = c.Query(context.Background(), q, c.s.Limits); err == nil || executions.Load() != 1 {
		t.Fatal("non-SELECT dry run executed")
	}
	r, err = c.DiscoverPage(context.Background(), "namespaces", model.Query{}, c.s.Limits)
	if err != nil || r.NextCursor != "more" || r.Data[0].(model.Object).Name != "sales" {
		t.Fatal(r, err)
	}
}

func TestCloudSourceCredentialAndEndpointBoundary(t *testing.T) {
	s := model.Source{Name: "BigQuery", Kind: "bigquery", Database: "test-project", Token: "secret", AuthMode: "token", Options: map[string]string{"location": "US", "read_only_confirmed": "true"}}
	if err := ValidateSource(&s, ""); err != nil {
		t.Fatal(err)
	}
	if s.Host != "bigquery.googleapis.com" || s.Port != 443 || s.Options["maximum_bytes_billed"] != "1073741824" {
		t.Fatal("missing defaults")
	}
	s.Host = "evil.invalid"
	if err := ValidateSource(&s, ""); err == nil {
		t.Fatal("BigQuery credential destination changed")
	}
	if _, err := bigQueryCredentials(`{"type":"service_account","client_email":"example","private_key":"PRIVATE KEY","token_uri":"http://169.254.169.254/token"}`); err == nil {
		t.Fatal("untrusted token URI accepted")
	}
	s.Kind = "snowflake"
	s.Host = "account.snowflakecomputing.com"
	s.Options = map[string]string{"warehouse": "wh", "role": "reader"}
	if err := ValidateSource(&s, ""); err == nil {
		t.Fatal("read-only role confirmation missing")
	}
}

type cloudTransport func(*http.Request) (*http.Response, error)

func (f cloudTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBigQueryServiceAccountTokenLifecycle(t *testing.T) {
	c := cloudFixture(t, "bigquery", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := json.Marshal(map[string]string{"type": "service_account", "client_email": "reader@test-project.iam.gserviceaccount.com", "private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))})
	c.s.AuthMode, c.s.Password, c.s.Token = "service_account", string(secret), ""
	var exchanges atomic.Int32
	var fail atomic.Bool
	transport := c.http.client.Transport
	c.http.client.Transport = cloudTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "oauth2.googleapis.com" {
			return transport.RoundTrip(r)
		}
		exchanges.Add(1)
		if r.URL.Path != "/token" || r.URL.Scheme != "https" {
			t.Error("unexpected credential destination")
		}
		r.ParseForm()
		parts := strings.Split(r.Form.Get("assertion"), ".")
		if len(parts) != 3 {
			t.Error("missing signed JWT")
		} else {
			raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
			var claims map[string]any
			json.Unmarshal(raw, &claims)
			if claims["aud"] != "https://oauth2.googleapis.com/token" || claims["iss"] != "reader@test-project.iam.gserviceaccount.com" {
				t.Error("wrong assertion audience or identity")
			}
		}
		code, body := 200, `{"access_token":"fixture-secret","token_type":"Bearer","expires_in":3600}`
		if fail.Load() {
			code, body = 401, `{"error":"invalid_grant","error_description":"private_key fixture-secret"}`
		}
		return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			var out any
			if err := c.json(context.Background(), "GET", "/fixture", nil, nil, &out, 4096); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if exchanges.Load() != 1 {
		t.Fatal("token refresh was not shared across requests")
	}
	c.token.Expiry = time.Now().Add(-time.Hour)
	fail.Store(true)
	var out any
	if err := c.json(context.Background(), "GET", "/fixture", nil, nil, &out, 4096); err == nil || strings.Contains(err.Error(), "private_key") || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatal("credential error was not redacted", err)
	}
	c.tokenGate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.json(ctx, "GET", "/fixture", nil, nil, &out, 4096); err == nil {
		t.Fatal("canceled request waited for credential refresh")
	}
	<-c.tokenGate
}

func TestCloudTemplatesEmptyResultsAndFailures(t *testing.T) {
	for _, kind := range []string{"snowflake", "databricks", "bigquery"} {
		t.Run(kind, func(t *testing.T) {
			var mode atomic.Int32
			var canceled atomic.Bool
			c := cloudFixture(t, kind, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/cancel") {
					canceled.Store(true)
					w.Write([]byte(`{}`))
					return
				}
				if mode.Load() == 1 {
					http.Error(w, "secret-sql secret-parameter fixture-secret", 403)
					return
				}
				if kind == "bigquery" && strings.HasSuffix(r.URL.Path, "/jobs") {
					body := readCloudRequest(t, r)
					if body["configuration"].(map[string]any)["dryRun"] == true {
						w.Write([]byte(`{"statistics":{"query":{"statementType":"SELECT"}}}`))
					} else {
						w.Write([]byte(`{"status":{"state":"RUNNING"}}`))
					}
					return
				}
				if mode.Load() == 2 {
					switch kind {
					case "snowflake":
						w.Write([]byte(`{"statementHandle":"statement1","code":"333334"}`))
					case "databricks":
						w.Write([]byte(`{"statement_id":"statement1","status":{"state":"PENDING"}}`))
					case "bigquery":
						w.Write([]byte(`{"jobComplete":false}`))
					}
					return
				}
				switch kind {
				case "snowflake":
					w.Write([]byte(`{"statementHandle":"statement1","resultSetMetaData":{"numRows":0,"rowType":[{"name":"id","type":"fixed"}]},"data":[]}`))
				case "databricks":
					w.Write([]byte(`{"statement_id":"statement1","status":{"state":"SUCCEEDED"},"manifest":{"schema":{"columns":[{"name":"id","type_text":"BIGINT"}]},"total_row_count":0},"result":{"chunk_index":0,"data_array":[]}}`))
				case "bigquery":
					w.Write([]byte(`{"jobComplete":true,"schema":{"fields":[{"name":"id","type":"INTEGER"}]},"rows":[]}`))
				}
			})
			cap, _ := Get(kind)
			raw, _ := json.Marshal(cap.Example)
			pointer := "/named_params/value"
			if kind == "snowflake" {
				pointer = "/params/0"
			}
			template := semantic.Template{Tool: "query_sql", QueryJSON: string(raw), Parameters: []semantic.Parameter{{Name: "value", Type: "integer", Required: true, Pointers: []string{pointer}}}}
			q, err := semantic.Bind(template, map[string]any{"value": json.Number("9007199254740993")}, false)
			if err != nil {
				t.Fatal(err)
			}
			r, err := c.Query(context.Background(), q, c.s.Limits)
			if err != nil || r.RowCount != 0 || r.Truncated || len(r.Columns) != 1 {
				t.Fatal("empty template result", r, err)
			}
			mode.Store(1)
			if _, err = c.Query(context.Background(), q, c.s.Limits); err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatal("cloud error leaked query/credential", err)
			}
			mode.Store(2)
			ctx, done := context.WithTimeout(context.Background(), 80*time.Millisecond)
			defer done()
			if _, err = c.Query(ctx, q, c.s.Limits); err == nil || !canceled.Load() {
				t.Fatal("pending remote operation not canceled", err)
			}
		})
	}
}

func TestCloudHTTPVerifiesEachRequestHost(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	defer server.Close()
	source := model.Source{Kind: "bigquery", Host: "bigquery.googleapis.com", Port: 443, TLSMode: "verify", CACert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))}
	source.Limits.Defaults()
	connection, err := openCloud(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	c := connection.(*cloudConn)
	c.http.base = server.URL
	var out any
	if err := c.json(context.Background(), "GET", "/token", nil, nil, &out, 4096); err != nil {
		t.Fatal("cloud transport used API hostname when checking a different TLS endpoint", err)
	}
}

func TestCloudMetadataBindingAndPaging(t *testing.T) {
	for _, kind := range []string{"snowflake", "databricks"} {
		t.Run(kind, func(t *testing.T) {
			c := cloudFixture(t, kind, func(w http.ResponseWriter, r *http.Request) {
				body := readCloudRequest(t, r)
				query := body["statement"].(string)
				if strings.Contains(query, "DROP") {
					t.Error("metadata namespace was interpolated")
				}
				var namespace any
				if kind == "snowflake" {
					namespace = body["bindings"].(map[string]any)["1"].(map[string]any)["value"]
				} else {
					namespace = body["parameters"].([]any)[0].(map[string]any)["value"]
				}
				if namespace != "public'; DROP TABLE x --" {
					t.Error("namespace was not bound as a value")
				}
				rows := [][]string{{"orders", "BASE TABLE"}, {"customers", "BASE TABLE"}}
				if strings.Contains(query, "OFFSET 1") {
					rows = rows[1:]
				}
				if kind == "snowflake" {
					json.NewEncoder(w).Encode(map[string]any{"statementHandle": "metadata1", "resultSetMetaData": map[string]any{"numRows": len(rows), "rowType": []map[string]string{{"name": "table_name", "type": "text"}, {"name": "table_type", "type": "text"}}}, "data": rows})
				} else {
					json.NewEncoder(w).Encode(map[string]any{"statement_id": "metadata1", "status": map[string]string{"state": "SUCCEEDED"}, "manifest": map[string]any{"schema": map[string]any{"columns": []map[string]string{{"name": "table_name", "type_text": "STRING"}, {"name": "table_type", "type_text": "STRING"}}}, "total_row_count": len(rows)}, "result": map[string]any{"chunk_index": 0, "data_array": rows}})
				}
			})
			l := c.s.Limits
			l.MaxRows = 1
			q := model.Query{Namespace: "public'; DROP TABLE x --"}
			first, err := c.DiscoverPage(context.Background(), "objects", q, l)
			if err != nil || first.NextCursor != "1" || first.RowCount != 1 || first.Truncated {
				t.Fatal(first, err)
			}
			q.Cursor = first.NextCursor
			next, err := c.DiscoverPage(context.Background(), "objects", q, l)
			if err != nil || next.NextCursor != "" || next.Data[0].(model.Object).Name != "customers" {
				t.Fatal(next, err)
			}
		})
	}
}
