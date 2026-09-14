package adapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var indexName = regexp.MustCompile(`^[a-zA-Z0-9_.*-]+$`)

func (c *httpConn) searchIndex(object string) (string, error) {
	if object == "" {
		object = c.s.Database
	}
	if object == "" {
		return "", errors.New("configure an index or index pattern in database")
	}
	if !indexName.MatchString(object) || strings.Contains(object, "..") {
		return "", errors.New("invalid index name")
	}
	if c.s.Database != "" {
		ok, e := path.Match(c.s.Database, object)
		if e != nil || !ok {
			return "", model.Fail("query_denied", "index outside configured pattern")
		}
	}
	return object, nil
}
func guardSearch(v any) error {
	switch n := v.(type) {
	case map[string]any:
		for k, x := range n {
			if wordset("script script_fields runtime_mappings pit scroll profile indices_boost")[strings.ToLower(k)] {
				return model.Fail("query_denied", "search scripts and stateful operations are denied")
			}
			if e := guardSearch(x); e != nil {
				return e
			}
		}
	case []any:
		for _, x := range n {
			if e := guardSearch(x); e != nil {
				return e
			}
		}
	}
	return nil
}
func (c *httpConn) querySearch(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	index, e := c.searchIndex(q.Object)
	if e != nil {
		return nil, e
	}
	op := q.Operation
	if op == "" {
		op = "search"
	}
	body := map[string]any{}
	if len(q.Body) > 0 {
		d := json.NewDecoder(bytes.NewReader(q.Body))
		d.UseNumber()
		if e = d.Decode(&body); e != nil {
			return nil, e
		}
	}
	if e = guardSearch(body); e != nil {
		return nil, e
	}
	r := model.NewResult("documents")
	switch op {
	case "get":
		if q.Query == "" || len(q.Query) > 512 || q.Query == "." || q.Query == ".." || strings.ContainsAny(q.Query, "/\\?#%\x00") {
			return nil, errors.New("query must contain document ID")
		}
		v, e := c.json(ctx, http.MethodGet, "/"+url.PathEscape(index)+"/_doc/"+url.PathEscape(q.Query), nil, nil, l.MaxBytes)
		if e != nil {
			return nil, e
		}
		_, e = r.Add(value(v), l)
		return r, e
	case "count":
		v, e := c.json(ctx, http.MethodPost, "/"+url.PathEscape(index)+"/_count", nil, body, l.MaxBytes)
		if e != nil {
			return nil, e
		}
		_, e = r.Add(value(v), l)
		return r, e
	case "search":
		for k := range body {
			if !wordset("query aggs aggregations sort _source fields docvalue_fields from size highlight track_total_hits search_after timeout stored_fields post_filter min_score collapse explain")[k] {
				return nil, model.Fail("query_denied", "unsupported search option")
			}
		}
		size := l.MaxRows
		if v, exists := body["size"]; exists {
			n, ok := v.(json.Number)
			requested, err := n.Int64()
			if !ok || err != nil || requested < 0 {
				return nil, model.Fail("invalid_query", "search size must be a non-negative integer")
			}
			size = int(min(requested, int64(size)))
		}
		from := int64(0)
		if v, exists := body["from"]; exists {
			n, ok := v.(json.Number)
			var err error
			from, err = n.Int64()
			if !ok || err != nil || from < 0 {
				return nil, model.Fail("invalid_query", "search from must be a non-negative integer")
			}
		}
		// One extra hit proves continuation, including when track_total_hits is disabled.
		// Stay inside the engines' default result window; a full boundary page is conservative.
		fetchSize := size
		if size > 0 && size < 10000 && (q.Cursor != "" || from < 10000-int64(size)) {
			fetchSize++
		}
		body["size"] = fetchSize
		body["timeout"] = strconv.Itoa(l.TimeoutSeconds) + "s"
		if q.Cursor != "" {
			b, e := base64.RawURLEncoding.DecodeString(q.Cursor)
			if e != nil {
				return nil, e
			}
			var after []any
			d := json.NewDecoder(bytes.NewReader(b))
			d.UseNumber()
			if e = d.Decode(&after); e != nil {
				return nil, e
			}
			body["search_after"] = after
			delete(body, "from")
		}
		v, e := c.json(ctx, http.MethodPost, "/"+url.PathEscape(index)+"/_search", nil, body, l.MaxBytes)
		if e != nil {
			return nil, e
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("invalid search response")
		}
		if timeout, _ := m["timed_out"].(bool); timeout {
			return nil, model.Fail("timeout", "search engine timed out")
		}
		if shards, ok := m["_shards"].(map[string]any); ok && model.String(shards["failed"]) != "0" && shards["failed"] != nil {
			return nil, model.Fail("database_error", "search returned failed shards")
		}
		hits, _ := m["hits"].(map[string]any)
		items, _ := hits["hits"].([]any)
		more := size > 0 && (len(items) > size || (fetchSize == size && len(items) == size))
		if len(items) > size {
			items = items[:size]
		}
		for _, item := range items {
			ok, e := r.Add(value(item), l)
			if e != nil {
				return nil, e
			}
			if !ok {
				break
			}
		}
		if more && !r.Truncated {
			r.Truncated = true
			last, _ := items[len(items)-1].(map[string]any)
			if sort, ok := last["sort"].([]any); ok && len(sort) > 0 {
				b, _ := json.Marshal(sort)
				r.NextCursor = base64.RawURLEncoding.EncodeToString(b)
				r.Truncated = false
			}
		}
		if aggs, ok := m["aggregations"]; ok {
			r.Format = "search"
			r.Data = []any{map[string]any{"hits": r.Data, "aggregations": value(aggs), "total": value(hits["total"])}}
			encoded, err := json.Marshal(r.Data)
			if err != nil {
				return nil, err
			}
			r.Bytes = len(encoded)
		}
		return r, nil
	default:
		return nil, model.Fail("query_denied", "unsupported search operation")
	}
}
func (c *httpConn) discoverSearch(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	if op == "namespaces" {
		return []model.Object{{Name: c.s.Database, Type: "index_pattern"}}, nil
	}
	index, e := c.searchIndex(obj)
	if e != nil {
		return nil, e
	}
	v, e := c.json(ctx, http.MethodGet, "/"+url.PathEscape(index)+"/_mapping", nil, nil, c.s.Limits.MaxBytes)
	if e != nil {
		return nil, e
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("invalid mapping response")
	}
	out := []model.Object{}
	for name, mapping := range m {
		o := model.Object{Name: name, Namespace: c.s.Database, Type: "index"}
		if op == "describe" {
			o.Details = mapping
			if m, ok := mapping.(map[string]any); ok {
				if schema, ok := m["mappings"].(map[string]any); ok {
					o.Columns = mappingColumns(schema, "")
				}
			}
		}
		out = append(out, o)
		if len(out) >= 1000 {
			break
		}
	}
	return out, nil
}

func mappingColumns(schema map[string]any, prefix string) []model.Column {
	out := []model.Column{}
	for _, group := range []string{"properties", "fields"} {
		properties, _ := schema[group].(map[string]any)
		names := make([]string, 0, len(properties))
		for name := range properties {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			definition, ok := properties[name].(map[string]any)
			if !ok {
				continue
			}
			path := prefix + name
			typ, _ := definition["type"].(string)
			if typ == "" {
				typ = "object"
			}
			out = append(out, model.Column{Name: path, Type: typ})
			out = append(out, mappingColumns(definition, path+".")...)
		}
	}
	return out
}
