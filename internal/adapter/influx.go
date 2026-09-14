package adapter

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func guardInfluxQL(q string) error {
	ts, e := lex(q)
	if e != nil {
		return e
	}
	if ts[0].text != "select" {
		return model.Fail("query_denied", "InfluxQL requires SELECT")
	}
	for _, t := range ts {
		if !t.quoted && deniedSQL[t.text] {
			return model.Fail("query_denied", "InfluxQL writes and SELECT INTO are denied")
		}
	}
	return guardCallsAllowed(ts, wordset("count distinct integral mean median mode spread stddev sum bottom first last max min percentile sample top abs acos asin atan atan2 ceil cos cumulative_sum derivative difference elapsed exp floor histogram holt_winters holt_winters_with_fit ln log log2 log10 moving_average non_negative_derivative non_negative_difference pow round sin sqrt tan now time fill"))
}
func guardFlux(q string) error {
	if strings.Contains(q, "${") || strings.Contains(q, "--") || strings.Contains(q, "/*") || strings.Contains(q, "#") {
		return model.Fail("query_denied", "Flux interpolation and non-Flux comments are denied; use named parameters")
	}
	ts, e := lex(q)
	if e != nil {
		return e
	}
	allowed := wordset("from range filter map reduce group aggregatewindow window mean sum count min max first last median quantile percentile distinct unique sort limit top bottom tail yield join pivot union difference derivative integral elapsed timedmovingaverage movingaverage cumulativesum spread covariance pearsonr statecount stateduration fill duplicate drop keep rename set exists contains length die bool int uint float string time duration bytes display")
	for i, t := range ts {
		if i+1 < len(ts) && ts[i+1].text == ":" && wordset("host url token")[t.text] {
			return model.Fail("query_denied", "Flux cannot supply connection addresses or credentials")
		}
		if !t.quoted && wordset("import package option to experimental http requests secrets socket sql")[t.text] {
			return model.Fail("query_denied", "Flux imports, writes and external access are denied")
		}
		if i+1 < len(ts) && ts[i+1].text == "(" && !t.quoted && len(t.text) > 0 && ((t.text[0] >= 'a' && t.text[0] <= 'z') || t.text[0] == '_') {
			if !allowed[t.text] && t.text != "if" {
				return model.Fail("query_denied", "Flux call is outside the read-only subset")
			}
		}
	}
	return nil
}
func (c *httpConn) queryInflux(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	switch c.s.Version {
	case "1":
		if q.Language != "" && q.Language != "influxql" {
			return nil, model.Fail("invalid_query", "InfluxDB 1.x requires language influxql. Use the data source query example.")
		}
		if e := guardInfluxQL(q.Query); e != nil {
			return nil, e
		}
		return c.influxQL(ctx, q, l)
	case "2":
		if q.Language != "" && q.Language != "flux" {
			return nil, model.Fail("invalid_query", "InfluxDB 2.x requires language flux. Use the data source query example.")
		}
		if e := guardFlux(q.Query); e != nil {
			return nil, e
		}
		return c.runFlux(ctx, q, l)
	case "3":
		lang := q.Language
		if lang == "" {
			lang = "sql"
		}
		if lang != "sql" && lang != "influxql" {
			return nil, model.Fail("invalid_query", "InfluxDB 3 Core requires language sql or influxql.")
		}
		if lang == "sql" {
			if e := guardSQL("postgres", regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*`).ReplaceAllString(q.Query, "$$1")); e != nil {
				return nil, e
			}
		} else if e := guardInfluxQL(q.Query); e != nil {
			return nil, e
		}
		v, e := c.json(ctx, http.MethodPost, "/api/v3/query_"+lang, nil, map[string]any{"db": c.s.Database, "q": q.Query, "params": q.NamedParams, "format": "json"}, l.MaxBytes)
		if e != nil {
			return nil, e
		}
		r := model.NewResult("documents")
		items, ok := v.([]any)
		if !ok {
			return nil, errors.New("invalid InfluxDB query response")
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
		return r, nil
	default:
		return nil, errors.New("InfluxDB version must be 1, 2 or 3")
	}
}
func (c *httpConn) runFlux(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	org := c.s.Options["org"]
	if org == "" {
		return nil, model.Fail("invalid_input", "Configure the InfluxDB organization on this data source.")
	}
	body := map[string]any{"query": q.Query, "type": "flux", "dialect": map[string]any{"annotations": []string{"datatype"}, "dateTimeFormat": "RFC3339Nano"}}
	if len(q.NamedParams) > 0 {
		extern, err := fluxExtern(q.NamedParams)
		if err != nil {
			return nil, err
		}
		body["extern"] = extern
	}
	res, e := c.request(ctx, http.MethodPost, "/api/v2/query", url.Values{"org": {org}}, body)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	bounded := &io.LimitedReader{R: res.Body, N: int64(l.MaxBytes) + 1}
	result, err := fluxCSV(bounded, l)
	if bounded.N == 0 {
		return nil, model.Fail("result_too_large", "Flux response exceeds byte limit")
	}
	return result, err
}

func (c *httpConn) influxQL(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	p := url.Values{"db": {c.s.Database}, "q": {q.Query}, "epoch": {"ns"}}
	if len(q.NamedParams) > 0 {
		b, _ := json.Marshal(q.NamedParams)
		p.Set("params", string(b))
	}
	v, e := c.json(ctx, http.MethodGet, "/query", p, nil, l.MaxBytes)
	if e != nil {
		return nil, e
	}
	r := model.NewResult("timeseries")
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("invalid InfluxQL response")
	}
	results, _ := m["results"].([]any)
	for _, item := range results {
		result, _ := item.(map[string]any)
		if result["error"] != nil {
			return nil, model.Fail("database_error", "InfluxQL query failed")
		}
		series, _ := result["series"].([]any)
		for _, sv := range series {
			s, _ := sv.(map[string]any)
			rows, _ := s["values"].([]any)
			for _, row := range rows {
				ok, e := r.Add(map[string]any{"measurement": s["name"], "tags": value(s["tags"]), "columns": s["columns"], "values": value(row)}, l)
				if e != nil {
					return nil, e
				}
				if !ok {
					return r, nil
				}
			}
		}
	}
	return r, nil
}
func fluxCSV(rd io.Reader, l model.Limits) (*model.Result, error) {
	reader := csv.NewReader(rd)
	reader.FieldsPerRecord = -1
	r := model.NewResult("tables")
	var cols, types []string
	for {
		row, e := reader.Read()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return nil, e
		}
		if len(row) == 0 {
			continue
		}
		if row[0] == "#datatype" {
			types = append([]string{}, row...)
			continue
		}
		if strings.HasPrefix(row[0], "#") {
			continue
		}
		if len(row) > 1 && (row[1] == "result" || row[1] == "error") {
			cols = append([]string{}, row...)
			if row[1] == "error" {
				return nil, model.Fail("database_error", "Flux query failed")
			}
			continue
		}
		if cols == nil {
			return nil, errors.New("invalid Flux CSV header")
		}
		ok, e := r.Add(map[string]any{"columns": cols, "types": types, "values": row}, l)
		if e != nil {
			return nil, e
		}
		if !ok {
			break
		}
	}
	return r, nil
}
func (c *httpConn) discoverInflux(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	if op == "namespaces" {
		name := c.s.Database
		if c.s.Version == "2" {
			name = c.s.Options["bucket"]
		}
		return []model.Object{{Name: name, Type: "database"}}, nil
	}
	l := c.s.Limits
	l.MaxRows = 1000
	if c.s.Version == "2" {
		bucket := c.s.Options["bucket"]
		if bucket == "" {
			return nil, errors.New("configure options.bucket")
		}
		if ns != "" && ns != bucket {
			return nil, model.Fail("query_denied", "namespace outside configured bucket")
		}
		if strings.Contains(bucket, "${") || strings.Contains(obj, "${") {
			return nil, model.Fail("invalid_object", "Flux interpolation is not allowed in metadata identifiers")
		}
		// Use InfluxDB's schema functions; no measurement values leave the engine.
		q := "import \"influxdata/influxdb/schema\"\n" + `schema.measurements(bucket: ` + strconv.Quote(bucket) + `, start: -30d)`
		field := "_measurement"
		if op == "describe" {
			q = "import \"influxdata/influxdb/schema\"\n" + `schema.measurementFieldKeys(bucket: ` + strconv.Quote(bucket) + `, measurement: ` + strconv.Quote(obj) + `, start: -30d)`
			field = "_field"
		}
		q += ` |> limit(n: 1000)`
		r, e := c.runFlux(ctx, model.Query{Language: "flux", Query: q}, l)
		if e != nil {
			return nil, e
		}
		out := []model.Object{}
		for _, row := range r.Data {
			m := row.(map[string]any)
			cols := m["columns"].([]string)
			vals := m["values"].([]string)
			for i, k := range cols {
				if (k == field || k == "_value") && i < len(vals) {
					out = append(out, model.Object{Name: vals[i], Type: field, Namespace: bucket})
				}
			}
		}
		return out, nil
	}
	if c.s.Version == "3" {
		q := "SELECT table_name FROM information_schema.tables WHERE table_schema='iox'"
		if op == "describe" {
			q = "SELECT column_name,data_type FROM information_schema.columns WHERE table_name=$name"
		}
		r, e := c.queryInflux(ctx, model.Query{Language: "sql", Query: q, NamedParams: map[string]any{"name": obj}}, l)
		if e != nil {
			return nil, e
		}
		out := []model.Object{}
		for _, row := range r.Data {
			m, _ := row.(map[string]any)
			name := model.String(m["table_name"])
			typ := "measurement"
			if op == "describe" {
				name = model.String(m["column_name"])
				typ = model.String(m["data_type"])
			}
			out = append(out, model.Object{Name: name, Type: typ, Namespace: c.s.Database})
		}
		return out, nil
	}
	query := "SHOW MEASUREMENTS LIMIT 1000"
	if op == "describe" {
		if strings.ContainsAny(obj, "\\\r\n\x00") {
			return nil, model.Fail("invalid_object", "unsupported measurement identifier")
		}
		query = "SHOW FIELD KEYS FROM " + `"` + strings.ReplaceAll(obj, `"`, `\"`) + `"`
	}
	r, e := c.influxQL(ctx, model.Query{Query: query}, l)
	if e != nil {
		return nil, e
	}
	out := []model.Object{}
	for _, row := range r.Data {
		m, _ := row.(map[string]any)
		vals, _ := m["values"].([]any)
		if len(vals) > 0 {
			typ := "measurement"
			if len(vals) > 1 {
				typ = model.String(vals[1])
			}
			out = append(out, model.Object{Name: model.String(vals[0]), Type: typ, Namespace: c.s.Database})
		}
	}
	return out, nil
}
