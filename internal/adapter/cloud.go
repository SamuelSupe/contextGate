package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"golang.org/x/oauth2"
)

type cloudConn struct {
	http      *httpConn
	s         model.Source
	tokenGate chan struct{}
	token     *oauth2.Token
}

func openCloud(ctx context.Context, s model.Source) (Connection, error) {
	h, err := openHTTP(ctx, s)
	if err != nil {
		return nil, err
	}
	client := h.(*httpConn)
	// BigQuery authentication and queries use different Google hosts. Let the
	// transport verify each request host instead of pinning the query API SNI.
	client.client.Transport.(*http.Transport).TLSClientConfig.ServerName = ""
	return &cloudConn{http: client, s: s, tokenGate: make(chan struct{}, 1)}, nil
}

func (c *cloudConn) Close() error { return c.http.Close() }

func (c *cloudConn) json(ctx context.Context, method, path string, values url.Values, body, out any, maxBytes int) error {
	h := *c.http
	if c.s.AuthMode == "service_account" {
		select {
		case c.tokenGate <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		if !c.token.Valid() {
			cfg, err := bigQueryCredentials(c.s.Password)
			if err != nil {
				<-c.tokenGate
				return model.Fail("database_authentication", "Invalid Google service account credential")
			}
			token, err := cfg.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, h.client)).Token()
			if err != nil {
				<-c.tokenGate
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return model.Fail("database_authentication", "Google service account authentication failed")
			}
			c.token = token
		}
		h.s.Token = c.token.AccessToken
		<-c.tokenGate
	}
	resp, err := h.request(ctx, method, path, values, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)+1))
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return model.Fail("result_too_large", "Cloud SQL response exceeds the byte limit")
	}
	if len(data) == 0 && out == nil {
		return nil
	}
	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return cloudResponseError()
	}
	if decoder.Decode(new(any)) != io.EOF {
		return cloudResponseError()
	}
	return nil
}

func cloudResponseError() error {
	return model.Fail("database_response", "The cloud SQL service returned an invalid or incomplete response")
}

func cloudPause(ctx context.Context) error {
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *cloudConn) cancel(path string, values url.Values) {
	ctx, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	_ = c.json(ctx, http.MethodPost, path, values, map[string]any{}, nil, 64<<10)
}

func (c *cloudConn) Probe(ctx context.Context) (model.Probe, error) {
	l := c.s.Limits
	l.MaxRows = 1
	_, err := c.Query(ctx, model.Query{Query: "SELECT 1 AS contextgate_probe"}, l)
	return model.Probe{Connected: err == nil, Protection: "select_guard_reader_role", PermissionStatus: "unverified", ServerVersion: c.s.Kind + "-declared-contract:" + c.s.Version, CheckedAt: time.Now(), Evidence: []string{"SELECT probe only; no write was attempted", "SELECT syntax is checked; the administrator confirms a dedicated reader role, account grants are not verified", "Cloud engine upgrades are not automatically detected; update the connection contract version and revalidate templates"}}, err
}

func (c *cloudConn) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if err := guardCloudSQL(c.s.Kind, q.Query); err != nil {
		return nil, err
	}
	if q.Cursor != "" {
		return nil, model.Fail("invalid_cursor", "Cloud SQL pagination is explicit in the query; result chunks are collected within request limits")
	}
	ctx, done := context.WithTimeout(ctx, time.Duration(l.TimeoutSeconds)*time.Second)
	defer done()
	switch c.s.Kind {
	case "snowflake":
		return c.snowflake(ctx, q, l)
	case "databricks":
		return c.databricks(ctx, q, l)
	case "bigquery":
		return c.bigQuery(ctx, q, l)
	}
	return nil, cloudResponseError()
}

func (c *cloudConn) Discover(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	l := c.s.Limits
	l.MaxRows = 10000
	r, err := c.DiscoverPage(ctx, op, model.Query{Namespace: ns, Object: obj}, l)
	if err != nil {
		return nil, err
	}
	if r.NextCursor != "" || r.Truncated {
		return nil, model.Fail("result_too_large", "Metadata exceeds import limits; select a smaller schema")
	}
	objects := make([]model.Object, 0, len(r.Data))
	for _, item := range r.Data {
		objects = append(objects, item.(model.Object))
	}
	return objects, nil
}

func (c *cloudConn) DiscoverPage(ctx context.Context, op string, q model.Query, l model.Limits) (*model.Result, error) {
	if c.s.Kind == "bigquery" {
		return c.bigQueryDiscover(ctx, op, q, l)
	}
	offset := uint64(0)
	if q.Cursor != "" {
		var err error
		offset, err = strconv.ParseUint(q.Cursor, 10, 32)
		if err != nil {
			return nil, model.Fail("invalid_cursor", "Invalid metadata cursor")
		}
	}
	ns := q.Namespace
	if ns == "" {
		ns = c.s.Options["schema"]
	}
	if ns == "" {
		if c.s.Kind == "snowflake" {
			ns = "PUBLIC"
		} else {
			ns = "default"
		}
	}
	bind := "?"
	query := model.Query{}
	add := func(name, val string) string {
		if c.s.Kind == "snowflake" {
			query.Params = append(query.Params, val)
			return bind
		}
		if query.NamedParams == nil {
			query.NamedParams = map[string]any{}
		}
		query.NamedParams[name] = val
		return ":" + name
	}
	switch op {
	case "namespaces":
		query.Query = "SELECT schema_name, 'schema' FROM information_schema.schemata ORDER BY schema_name"
	case "objects":
		query.Query = "SELECT table_name, table_type FROM information_schema.tables WHERE table_schema=" + add("schema", ns) + " ORDER BY table_name"
	case "describe":
		query.Query = "SELECT column_name, data_type FROM information_schema.columns WHERE table_schema=" + add("schema", ns) + " AND table_name=" + add("object", q.Object) + " ORDER BY ordinal_position"
	default:
		return nil, model.Fail("invalid_arguments", "Unknown discovery operation")
	}
	query.Query += " LIMIT " + strconv.Itoa(l.MaxRows+1) + " OFFSET " + strconv.FormatUint(offset, 10)
	limit := l
	limit.MaxRows++
	rows, err := c.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	result := model.NewResult("metadata")
	budget := l
	budget.MaxBytes -= 768
	for _, item := range rows.Data {
		row, ok := item.([]any)
		if !ok || len(row) != 2 {
			return nil, cloudResponseError()
		}
		name, ok := row[0].(string)
		typ, okType := row[1].(string)
		if !ok || !okType {
			return nil, cloudResponseError()
		}
		object := model.Object{Name: name, Namespace: ns, Type: typ}
		if op == "namespaces" {
			object.Namespace = ""
		}
		added, err := result.Add(object, budget)
		if err != nil {
			return nil, err
		}
		if !added {
			if result.RowCount == 0 {
				return nil, model.Fail("result_too_large", "One metadata object exceeds the byte limit")
			}
			result.NextCursor = strconv.FormatUint(offset+uint64(result.RowCount), 10)
			result.Truncated = false
			return result, nil
		}
	}
	if rows.Truncated {
		if result.RowCount == 0 {
			return nil, model.Fail("result_too_large", "Metadata page cannot make progress within the byte limit")
		}
		result.NextCursor = strconv.FormatUint(offset+uint64(result.RowCount), 10)
	}
	return result, nil
}
