package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/model"
	"io"
	"net/http"
	"net/url"
	"time"
)

type httpConn struct {
	client *http.Client
	s      model.Source
	base   string
}

func openHTTP(ctx context.Context, s model.Source) (Connection, error) {
	tc, e := tlsConfig(s)
	if e != nil {
		return nil, e
	}
	tr := &http.Transport{TLSClientConfig: tc, DisableCompression: true, MaxConnsPerHost: s.Limits.Concurrency, MaxIdleConnsPerHost: s.Limits.Concurrency, ResponseHeaderTimeout: time.Duration(s.Limits.TimeoutSeconds) * time.Second}
	return &httpConn{&http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("database redirects are disabled") }}, s, baseURL(s)}, nil
}
func (c *httpConn) Close() error { c.client.CloseIdleConnections(); return nil }
func (c *httpConn) request(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		rd = bytes.NewReader(b)
	}
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, e := http.NewRequestWithContext(ctx, method, u, rd)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.s.Kind == "influxdb" && c.s.Version == "2" {
		req.Header.Set("Accept", "application/csv")
	}
	if c.s.Token != "" {
		prefix := "Bearer "
		if c.s.Kind == "influxdb" && c.s.Version == "2" {
			prefix = "Token "
		}
		req.Header.Set("Authorization", prefix+c.s.Token)
	} else if c.s.Username != "" {
		req.SetBasicAuth(c.s.Username, c.s.Password)
	}
	res, e := c.client.Do(req)
	if e != nil {
		return nil, e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		code, message := "database_error", "The database returned an unsuccessful HTTP response."
		switch res.StatusCode {
		case 401:
			code, message = "database_authentication", "Database authentication failed. Check the authentication method and credential."
		case 403:
			code, message = "database_permission", "The database denied access. Check the account read permissions."
		case 400, 404, 422:
			code, message = "database_query", "The database rejected the query or object name. Check the query and selected database."
		case 502, 503, 504:
			code, message = "database_connection", "The database service is unavailable. Check server availability."
		}
		return nil, &model.Error{Code: code, Message: message, NativeCode: fmt.Sprintf("HTTP %d", res.StatusCode)}
	}
	return res, nil
}
func (c *httpConn) json(ctx context.Context, method, path string, query url.Values, body any, limit int) (any, error) {
	res, e := c.request(ctx, method, path, query, body)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, int64(limit)+1))
	if e != nil {
		return nil, e
	}
	if len(b) > limit {
		return nil, model.Fail("result_too_large", "database response exceeds byte limit")
	}
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	e = dec.Decode(&v)
	return v, e
}
func (c *httpConn) Probe(ctx context.Context) (model.Probe, error) {
	p := model.Probe{Connected: true, Protection: "query_api", PermissionStatus: "unverified", Evidence: []string{"Only fixed read endpoints are exposed; database account grants are not inferred"}, CheckedAt: time.Now()}
	path := "/"
	if c.s.Kind == "influxdb" {
		path = "/health"
		if c.s.Version == "3" {
			p.PermissionStatus = "api_isolated"
			p.Evidence = []string{"InfluxDB 3 Core admin token is restricted by this adapter to query_sql/query_influxql; this is not a read-only database account"}
		}
	}
	v, e := c.json(ctx, http.MethodGet, path, nil, nil, 1<<20)
	if e != nil && c.s.Kind == "influxdb" {
		res, err := c.request(ctx, http.MethodGet, "/ping", nil, nil)
		if err != nil {
			return p, e
		}
		defer res.Body.Close()
		p.ServerVersion = res.Header.Get("X-Influxdb-Version")
		return p, nil
	}
	if e != nil {
		return p, e
	}
	if m, ok := v.(map[string]any); ok {
		switch ver := m["version"].(type) {
		case map[string]any:
			p.ServerVersion = model.String(ver["number"])
		case string:
			p.ServerVersion = ver
		}
	}
	return p, nil
}
func (c *httpConn) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if c.s.Kind == "influxdb" {
		return c.queryInflux(ctx, q, l)
	}
	return c.querySearch(ctx, q, l)
}
func (c *httpConn) Discover(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	if c.s.Kind == "influxdb" {
		return c.discoverInflux(ctx, op, ns, obj)
	}
	return c.discoverSearch(ctx, op, ns, obj)
}
