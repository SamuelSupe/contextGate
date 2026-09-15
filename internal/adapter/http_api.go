package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
)

type apiConn struct {
	client *http.Client
	source model.Source
}

func openHTTPAPI(_ context.Context, s model.Source) (Connection, error) {
	if err := validateHTTPAPI(&s); err != nil {
		return nil, err
	}
	tc, err := tlsConfig(s)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{TLSClientConfig: tc, Proxy: nil, DisableCompression: true, MaxConnsPerHost: s.Limits.Concurrency, MaxIdleConnsPerHost: s.Limits.Concurrency, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: time.Duration(s.Limits.TimeoutSeconds) * time.Second, MaxResponseHeaderBytes: 64 << 10, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext}
	return &apiConn{source: s, client: &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *apiConn) Close() error { c.client.CloseIdleConnections(); return nil }

func (c *apiConn) operation(id string) (model.HTTPOperation, error) {
	for _, op := range c.source.HTTPAPI.Operations {
		if op.ID == id {
			return op, nil
		}
	}
	return model.HTTPOperation{}, model.Fail("invalid_query", "Unknown HTTP API operation; use list_objects to discover configured operations")
}

func (c *apiConn) Probe(ctx context.Context) (model.Probe, error) {
	p := model.Probe{Protection: "declared_read_api", PermissionStatus: "unverified", CheckedAt: time.Now().UTC(), Evidence: []string{"Connection check executes the configured operation with its example parameters", "GET/POST read-only behavior and response fields are administrator-declared; upstream permissions are not verified", "API contract version is administrator-maintained; upstream changes require a version update and new template trials"}}
	op, err := c.operation(c.source.HTTPAPI.ProbeOperation)
	if err != nil {
		return p, err
	}
	examples, err := apiExamples(op)
	if err != nil {
		return p, err
	}
	_, err = c.Query(ctx, model.Query{Operation: op.ID, NamedParams: examples}, c.source.Limits)
	if err != nil {
		return p, err
	}
	p.Connected, p.ServerVersion = true, "declared-api-contract:"+c.source.Version
	return p, nil
}

func (c *apiConn) Discover(_ context.Context, operation, namespace, object string) ([]model.Object, error) {
	if namespace != "" && namespace != "api" {
		return nil, model.Fail("invalid_query", "HTTP API namespace is api")
	}
	if operation == "namespaces" {
		return []model.Object{{Name: "api", Type: "namespace"}}, nil
	}
	out := []model.Object{}
	for _, op := range c.source.HTTPAPI.Operations {
		if operation == "describe" && op.ID != object {
			continue
		}
		item := model.Object{Name: op.ID, Namespace: "api", Type: "http_operation"}
		// Discovery describes the callable contract, never authentication, network
		// addresses, request paths or administrator-only connection configuration.
		item.Details = map[string]any{"name": op.Name, "description": op.Description, "read_only_status": "administrator_declared", "schema_status": "administrator_declared"}
		if operation == "describe" {
			item.Columns = op.Columns
			parameters := []map[string]any{}
			for _, p := range op.Parameters {
				parameters = append(parameters, map[string]any{"name": p.Name, "type": p.Type, "required": p.Required, "default_json": p.DefaultJSON, "enum_json": p.EnumJSON, "minimum": p.Minimum, "maximum": p.Maximum})
			}
			item.Details.(map[string]any)["parameters"] = parameters
			example, _ := json.Marshal(map[string]any{"operation": op.ID, "named_params": json.RawMessage(op.ExampleJSON)})
			item.Details.(map[string]any)["example_json"] = string(example)
			item.Details.(map[string]any)["pagination"] = op.Pagination != nil
		}
		out = append(out, item)
	}
	if operation == "describe" && len(out) == 0 {
		return nil, model.Fail("invalid_query", "Unknown HTTP API operation")
	}
	return out, nil
}

func (c *apiConn) Query(ctx context.Context, q model.Query, limits model.Limits) (*model.Result, error) {
	if q.Query != "" || len(q.Params) > 0 || q.Namespace != "" || q.Object != "" || q.Command != "" || len(q.Args) > 0 || q.Language != "" || len(q.Body)+len(q.Filter)+len(q.Sort)+len(q.Projection)+len(q.Pipeline) > 0 {
		return nil, model.Fail("invalid_query", "HTTP API accepts only a configured operation ID and named parameter values")
	}
	op, err := c.operation(q.Operation)
	if err != nil {
		return nil, err
	}
	path, query, body, err := bindAPI(op, q.NamedParams)
	if err != nil {
		return nil, err
	}
	if q.Cursor != "" {
		if op.Pagination == nil || len(q.Cursor) > 4096 {
			return nil, model.Fail("invalid_cursor", "This operation has no valid pagination state")
		}
		query.Set(op.Pagination.QueryParameter, q.Cursor)
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	endpoint := strings.TrimSuffix(c.source.HTTPAPI.BaseURL, "/") + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, op.Method, endpoint, reader)
	if err != nil {
		return nil, model.Fail("invalid_query", "Unable to build configured HTTP request")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.source.Token != "" {
		if c.source.HTTPAPI.TokenHeader != "" {
			req.Header.Set(c.source.HTTPAPI.TokenHeader, c.source.Token)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.source.Token)
		}
	} else if c.source.Username != "" {
		req.SetBasicAuth(c.source.Username, c.source.Password)
	}
	res, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// url.Error embeds the request URL (including parameters); never surface it.
		return nil, model.Fail("database_connection", "HTTP API request failed; check the configured address, TLS and server availability")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		code, message := "database_query", "HTTP API rejected the request; check the configured operation and parameters"
		switch {
		case res.StatusCode >= 300 && res.StatusCode < 400:
			code, message = "read_only_violation", "HTTP API redirects are disabled"
		case res.StatusCode == 401:
			code, message = "database_authentication", "HTTP API authentication failed"
		case res.StatusCode == 403:
			code, message = "database_permission", "HTTP API denied access to this operation"
		case res.StatusCode == 429:
			code, message = "rate_limited", "HTTP API rate limit reached; retry later"
		case res.StatusCode >= 500:
			code, message = "database_connection", "HTTP API service is unavailable"
		}
		return nil, &model.Error{Code: code, Message: message, NativeCode: fmt.Sprintf("HTTP %d", res.StatusCode)}
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, int64(limits.MaxBytes)+1))
	if err != nil {
		return nil, model.Fail("database_connection", "HTTP API response could not be read")
	}
	if len(raw) > limits.MaxBytes {
		return nil, model.Fail("result_too_large", "HTTP API response exceeds the byte limit; configure a smaller upstream page")
	}
	result := model.NewResult("documents")
	result.Columns = op.Columns
	if res.StatusCode == http.StatusNoContent {
		return result, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var document any
	if decoder.Decode(&document) != nil {
		return nil, model.Fail("database_query", "HTTP API response is not valid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, model.Fail("database_query", "HTTP API response must contain exactly one JSON value")
	}
	selected, err := apiPointer(document, op.ResponsePointer)
	if err != nil {
		return nil, model.Fail("database_query", "Response data pointer was not found; review the API contract")
	}
	rows, list := selected.([]any)
	if !list {
		rows = []any{selected}
	}
	for _, row := range rows {
		added, err := result.Add(value(row), limits)
		if err != nil {
			return nil, err
		}
		if !added {
			break
		}
	}
	// Do not skip unreturned rows by returning a next-page cursor for a page
	// truncated locally. The caller must reduce upstream page size and restart.
	if !result.Truncated && op.Pagination != nil {
		next, err := apiPointer(document, op.Pagination.NextPointer)
		if err != nil {
			return nil, model.Fail("database_query", "Next-token pointer was not found; return null or an empty string on the final page")
		}
		if next != nil {
			switch v := next.(type) {
			case string:
				result.NextCursor = v
			case json.Number:
				result.NextCursor = string(v)
			default:
				return nil, model.Fail("database_query", "Pagination token must be a string, number or null")
			}
			if len(result.NextCursor) > 4096 || result.NextCursor != "" && result.NextCursor == q.Cursor {
				return nil, model.Fail("database_query", "Invalid or repeated API pagination token")
			}
		}
	}
	return result, nil
}
