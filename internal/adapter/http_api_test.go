package adapter

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
)

func apiTestSource(base string) model.Source {
	s := model.Source{Name: "Customer API", Kind: "http_api", Version: "2026-09", TLSMode: "disable", Enabled: true, HTTPAPI: &model.HTTPAPIConfig{BaseURL: base, ProbeOperation: "customers", Operations: []model.HTTPOperation{{ID: "customers", Name: "Customer lookup", Method: "GET", Path: "/customers/{id}", ReadOnly: true, ExampleJSON: `{"id":"42","region":"east"}`, Parameters: []model.HTTPParameter{{Name: "id", In: "path", Target: "id", Type: "string", Required: true}, {Name: "region", In: "query", Target: "region", Type: "string", Required: true}}, ResponsePointer: "/data", Columns: []model.Column{{Name: "id", Type: "integer"}}, Pagination: &model.HTTPPagination{QueryParameter: "cursor", NextPointer: "/next"}}}}}
	s.Limits.Defaults()
	return s
}

func TestHTTPAPIRequestBoundariesAndResults(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("X-API-Key") != "fixture-secret" || r.URL.Path != "/v1/customers/42" || r.URL.Query().Get("region") != "east&admin=true" || r.URL.Query().Get("admin") != "" {
			t.Error("request escaped its fixed contract")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			io.WriteString(w, `{"data":[],"next":null}`)
			return
		}
		io.WriteString(w, `{"data":[{"id":9007199254740993,"amount":12.00000000000000001},{"id":2}],"next":"opaque&secret=value"}`)
	}))
	defer server.Close()
	s := apiTestSource(server.URL + "/v1")
	s.Token = "fixture-secret"
	s.HTTPAPI.TokenHeader = "X-API-Key"
	if err := ValidateSource(&s, ""); err != nil {
		t.Fatal(err)
	}
	c, err := Open(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	q := model.Query{Operation: "customers", NamedParams: map[string]any{"id": "42", "region": "east&admin=true"}}
	result, err := c.Query(context.Background(), q, s.Limits)
	if err != nil || result.RowCount != 2 || result.NextCursor != "opaque&secret=value" {
		t.Fatal(result, err)
	}
	first := result.Data[0].(map[string]any)
	if first["id"] != "9007199254740993" || first["amount"] != "12.00000000000000001" {
		t.Fatal("numbers lost precision", first)
	}
	q.Cursor = result.NextCursor
	result, err = c.Query(context.Background(), q, s.Limits)
	if err != nil || result.RowCount != 0 || result.NextCursor != "" {
		t.Fatal(result, err)
	}
	q.Cursor = ""
	limits := s.Limits
	limits.MaxRows = 1
	result, err = c.Query(context.Background(), q, limits)
	if err != nil || !result.Truncated || result.NextCursor != "" {
		t.Fatal("truncation must not skip rows", result, err)
	}
	before := calls.Load()
	for _, invalid := range []model.Query{
		{Operation: "https://example.com"}, {Operation: "customers", Body: json.RawMessage(`{}`)},
		{Operation: "customers", NamedParams: map[string]any{"id": "../write", "region": "east"}},
		{Operation: "customers", NamedParams: map[string]any{"id": "%2e%2e", "region": "east"}},
		{Operation: "customers", NamedParams: map[string]any{"id": "42/other", "region": "east"}},
		{Operation: "customers", NamedParams: map[string]any{"id": "42", "region": map[string]any{"$gt": 0}}},
		{Operation: "customers", NamedParams: map[string]any{"id": "42", "region": "east", "url": "http://elsewhere"}},
	} {
		if _, err := c.Query(context.Background(), invalid, s.Limits); err == nil {
			t.Fatal("unsafe query accepted", invalid)
		}
	}
	if calls.Load() != before {
		t.Fatal("invalid query reached upstream")
	}
	meta, err := c.Discover(context.Background(), "describe", "api", "customers")
	raw, _ := json.Marshal(meta)
	if err != nil || strings.Contains(string(raw), server.URL) || strings.Contains(string(raw), "fixture-secret") || !strings.Contains(string(raw), "administrator_declared") {
		t.Fatal("discovery boundary", string(raw), err)
	}
}

func TestHTTPAPIPOSTTLSAndFailures(t *testing.T) {
	var mode atomic.Int64
	var cancelObserved atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("missing bearer")
		}
		if mode.Load() == 5 {
			io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				cancelObserved.Store(true)
			case <-time.After(2 * time.Second):
			}
			return
		}
		switch mode.Load() {
		case 1:
			w.Header().Set("Location", "http://elsewhere.invalid")
			w.WriteHeader(307)
			return
		case 2:
			w.WriteHeader(429)
			io.WriteString(w, "secret-in-upstream-error")
			return
		case 3:
			io.WriteString(w, strings.Repeat("x", 2049))
			return
		case 4:
			io.WriteString(w, "{}{}")
			return
		}
		raw, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || string(raw) != `{"filter":{"id":9007199254740993},"operation":"search"}` {
			t.Errorf("incorrect fixed POST body: %s", raw)
		}
		io.WriteString(w, `{"id":9007199254740993}`)
	}))
	defer server.Close()
	s := apiTestSource(server.URL)
	s.TLSMode = "verify"
	s.Token = "fixture-secret"
	s.CACert = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	op := &s.HTTPAPI.Operations[0]
	op.Method = "POST"
	op.Path = "/search"
	op.BodyJSON = `{"filter":{"id":0},"operation":"search"}`
	op.Parameters = []model.HTTPParameter{{Name: "id", In: "body", Target: "/filter/id", Type: "integer", Required: true}}
	op.ExampleJSON = `{"id":9007199254740993}`
	op.ResponsePointer = ""
	op.Pagination = nil
	c, err := Open(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	q := model.Query{Operation: "customers", NamedParams: map[string]any{"id": json.Number("9007199254740993")}}
	if res, err := c.Query(context.Background(), q, s.Limits); err != nil || res.RowCount != 1 {
		t.Fatal(res, err)
	}
	p, err := c.Probe(context.Background())
	if err != nil || !p.Connected || p.PermissionStatus != "unverified" || p.ServerVersion != "declared-api-contract:2026-09" {
		t.Fatal(p, err)
	}
	for n := int64(1); n <= 4; n++ {
		mode.Store(n)
		limits := s.Limits
		limits.MaxBytes = 2048
		_, err := c.Query(context.Background(), q, limits)
		if err == nil || strings.Contains(err.Error(), "secret-in-upstream-error") || strings.Contains(err.Error(), server.URL) {
			t.Fatal("unsafe or missing failure", n, err)
		}
	}
	mode.Store(5)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.Query(ctx, q, s.Limits); err == nil {
		t.Fatal("request ignored cancellation")
	}
	deadline := time.Now().Add(time.Second)
	for !cancelObserved.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !cancelObserved.Load() {
		t.Fatal("upstream connection was not cancelled")
	}
}

func TestHTTPAPIConfigurationRejectsUnsafeDefinitions(t *testing.T) {
	for _, mutate := range []func(*model.Source){
		func(s *model.Source) { s.HTTPAPI.BaseURL = "http://user:secret@example.com" },
		func(s *model.Source) { s.HTTPAPI.BaseURL = "http://example.com?key=secret" },
		func(s *model.Source) { s.HTTPAPI.BaseURL = "file:///tmp/data" },
		func(s *model.Source) { s.HTTPAPI.TokenHeader = "X-HTTP-Method-Override" },
		func(s *model.Source) { s.HTTPAPI.Operations[0].ReadOnly = false },
		func(s *model.Source) { s.HTTPAPI.Operations[0].Method = "DELETE" },
		func(s *model.Source) { s.HTTPAPI.Operations[0].Path = "//elsewhere" },
		func(s *model.Source) { s.HTTPAPI.Operations[0].Path = "/../write" },
		func(s *model.Source) { s.HTTPAPI.Operations[0].Path = "/%2e%2e/write" },
		func(s *model.Source) { s.HTTPAPI.Operations[0].Parameters[0].Type = "object" },
		func(s *model.Source) { s.HTTPAPI.Operations[0].Pagination.QueryParameter = "region" },
	} {
		s := apiTestSource("http://example.com")
		mutate(&s)
		if err := ValidateSource(&s, ""); err == nil {
			t.Fatal("unsafe API definition accepted", s)
		}
	}
}
