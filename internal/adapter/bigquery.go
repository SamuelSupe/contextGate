package adapter

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
)

type bigField struct {
	Name   string     `json:"name"`
	Type   string     `json:"type"`
	Mode   string     `json:"mode"`
	Fields []bigField `json:"fields"`
}
type bigCell struct {
	V any `json:"v"`
}
type bigRow struct {
	F []bigCell `json:"f"`
}
type bigResults struct {
	Complete bool `json:"jobComplete"`
	Schema   struct {
		Fields []bigField `json:"fields"`
	} `json:"schema"`
	Rows   []bigRow `json:"rows"`
	Next   string   `json:"pageToken"`
	Errors []struct {
		Reason string `json:"reason"`
	} `json:"errors"`
}

func (c *cloudConn) bigQuery(ctx context.Context, q model.Query, l model.Limits) (result *model.Result, err error) {
	parameters, err := namedCloudParameters("bigquery", q)
	if err != nil {
		return nil, err
	}
	query := map[string]any{"query": q.Query, "useLegacySql": false, "parameterMode": "NAMED", "queryParameters": parameters, "maximumBytesBilled": c.s.Options["maximum_bytes_billed"]}
	if schema := c.s.Options["schema"]; schema != "" {
		query["defaultDataset"] = map[string]string{"projectId": c.s.Database, "datasetId": schema}
	}
	configuration := map[string]any{"query": query, "dryRun": true, "jobTimeoutMs": strconv.Itoa(l.TimeoutSeconds * 1000)}
	path := "/bigquery/v2/projects/" + url.PathEscape(c.s.Database)
	location := url.Values{"location": {c.s.Options["location"]}}
	var dry struct {
		Statistics struct {
			Query struct {
				StatementType string `json:"statementType"`
			} `json:"query"`
		} `json:"statistics"`
	}
	body := map[string]any{"configuration": configuration, "jobReference": map[string]string{"projectId": c.s.Database, "location": c.s.Options["location"]}}
	if err = c.json(ctx, http.MethodPost, path+"/jobs", nil, body, &dry, 1<<20); err != nil {
		return nil, err
	}
	if dry.Statistics.Query.StatementType != "SELECT" {
		return nil, model.Fail("query_denied", "BigQuery dry run must classify this job as SELECT")
	}
	// Assign the job ID before submission so cancellation is still possible if
	// a timeout loses the submit response. Never resubmit an ambiguous job.
	id := "contextgate_" + secure.Random(18)
	body["jobReference"] = map[string]string{"projectId": c.s.Database, "location": c.s.Options["location"], "jobId": id}
	configuration["dryRun"] = false
	defer func() {
		if err != nil {
			c.cancel(path+"/jobs/"+id+"/cancel", location)
		}
	}()
	var submitted struct {
		Status struct {
			Error *struct{} `json:"errorResult"`
		} `json:"status"`
	}
	if err = c.json(ctx, http.MethodPost, path+"/jobs", nil, body, &submitted, 1<<20); err != nil {
		return nil, err
	}
	if submitted.Status.Error != nil {
		return nil, model.Fail("database_query", "BigQuery rejected the SELECT job")
	}
	result = model.NewResult("table")
	pageToken := ""
	var fields []bigField
	seen := map[string]bool{}
	for pages := 0; pages < 10000; {
		values := url.Values{"location": location["location"], "maxResults": {strconv.Itoa(l.MaxRows + 1)}, "timeoutMs": {"1000"}}
		if pageToken != "" {
			values.Set("pageToken", pageToken)
		}
		var response bigResults
		if err = c.json(ctx, http.MethodGet, path+"/queries/"+id, values, nil, &response, l.MaxBytes); err != nil {
			return nil, err
		}
		if len(response.Errors) != 0 {
			return nil, model.Fail("database_query", "BigQuery did not complete the SELECT job")
		}
		if !response.Complete {
			if err = cloudPause(ctx); err != nil {
				return nil, err
			}
			continue
		}
		pages++
		if fields == nil {
			fields = response.Schema.Fields
			if len(fields) == 0 {
				return nil, cloudResponseError()
			}
			for _, f := range fields {
				result.Columns = append(result.Columns, model.Column{Name: f.Name, Type: bigFieldType(f)})
			}
		}
		for _, row := range response.Rows {
			if len(row.F) != len(fields) {
				return nil, cloudResponseError()
			}
			values := make([]any, len(fields))
			for i, f := range fields {
				values[i], err = bigValue(f, row.F[i].V)
				if err != nil {
					return nil, err
				}
			}
			var added bool
			added, err = result.Add(values, l)
			if err != nil {
				return nil, err
			}
			if !added {
				return result, nil
			}
		}
		pageToken = response.Next
		if pageToken == "" {
			return result, nil
		}
		if result.RowCount >= l.MaxRows {
			result.Truncated = true
			return result, nil
		}
		if seen[pageToken] || len(pageToken) > 8192 {
			return nil, cloudResponseError()
		}
		seen[pageToken] = true
	}
	return nil, model.Fail("result_too_large", "Too many BigQuery result pages; narrow the query")
}

func bigFieldType(f bigField) string {
	if f.Mode == "REPEATED" {
		return "ARRAY<" + f.Type + ">"
	}
	return f.Type
}

func bigValue(f bigField, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if f.Mode == "REPEATED" {
		array, ok := v.([]any)
		if !ok {
			return nil, cloudResponseError()
		}
		out := make([]any, 0, len(array))
		f.Mode = "NULLABLE"
		for _, item := range array {
			cell, ok := item.(map[string]any)
			if !ok {
				return nil, cloudResponseError()
			}
			val, err := bigValue(f, cell["v"])
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		}
		return out, nil
	}
	if f.Type == "RECORD" || f.Type == "STRUCT" {
		record, ok := v.(map[string]any)
		if !ok {
			return nil, cloudResponseError()
		}
		cells, ok := record["f"].([]any)
		if !ok || len(cells) != len(f.Fields) {
			return nil, cloudResponseError()
		}
		out := map[string]any{}
		for i, field := range f.Fields {
			cell, ok := cells[i].(map[string]any)
			if !ok {
				return nil, cloudResponseError()
			}
			val, err := bigValue(field, cell["v"])
			if err != nil {
				return nil, err
			}
			out[field.Name] = val
		}
		return out, nil
	}
	if f.Type == "BOOLEAN" || f.Type == "BOOL" {
		if text, ok := v.(string); ok {
			b, err := strconv.ParseBool(text)
			if err != nil {
				return nil, cloudResponseError()
			}
			return b, nil
		}
		if b, ok := v.(bool); ok {
			return b, nil
		}
		return nil, cloudResponseError()
	}
	if f.Type == "BYTES" {
		return map[string]any{"encoding": "base64", "data": v}, nil
	}
	return value(v), nil
}

func (c *cloudConn) bigQueryDiscover(ctx context.Context, op string, q model.Query, l model.Limits) (*model.Result, error) {
	base := "/bigquery/v2/projects/" + url.PathEscape(c.s.Database)
	ns := q.Namespace
	if ns == "" {
		ns = c.s.Options["schema"]
	}
	if op != "namespaces" && !cloudID.MatchString(ns) {
		return nil, model.Fail("invalid_arguments", "Select a valid BigQuery dataset namespace")
	}
	result := model.NewResult("metadata")
	budget := l
	budget.MaxBytes -= 768
	if op == "describe" {
		if q.Object == "" || len(q.Object) > 1024 || strings.ContainsAny(q.Object, "/\\\x00") {
			return nil, model.Fail("invalid_arguments", "Invalid table name")
		}
		var table struct {
			Schema struct {
				Fields []bigField `json:"fields"`
			} `json:"schema"`
		}
		if err := c.json(ctx, http.MethodGet, base+"/datasets/"+ns+"/tables/"+url.PathEscape(q.Object), nil, nil, &table, l.MaxBytes); err != nil {
			return nil, err
		}
		offset := uint64(0)
		if q.Cursor != "" {
			var err error
			offset, err = strconv.ParseUint(q.Cursor, 10, 32)
			if err != nil {
				return nil, model.Fail("invalid_cursor", "Invalid column cursor")
			}
		}
		objects := []model.Object{}
		var fields func([]bigField, string)
		fields = func(items []bigField, prefix string) {
			for _, f := range items {
				name := prefix + f.Name
				objects = append(objects, model.Object{Name: name, Namespace: ns, Type: bigFieldType(f)})
				fields(f.Fields, name+".")
			}
		}
		fields(table.Schema.Fields, "")
		for i := offset; i < uint64(len(objects)); i++ {
			added, err := result.Add(objects[i], budget)
			if err != nil {
				return nil, err
			}
			if !added {
				result.NextCursor = strconv.FormatUint(i, 10)
				result.Truncated = false
				break
			}
		}
	} else {
		values := url.Values{"maxResults": {strconv.Itoa(l.MaxRows)}}
		if q.Cursor != "" {
			if len(q.Cursor) > 8192 {
				return nil, model.Fail("invalid_cursor", "Invalid metadata cursor")
			}
			values.Set("pageToken", q.Cursor)
		}
		path := base + "/datasets"
		if op == "objects" {
			path += "/" + ns + "/tables"
		} else if op != "namespaces" {
			return nil, model.Fail("invalid_arguments", "Unknown discovery operation")
		}
		var page struct {
			Next     string `json:"nextPageToken"`
			Datasets []struct {
				Ref struct {
					ID string `json:"datasetId"`
				} `json:"datasetReference"`
			} `json:"datasets"`
			Tables []struct {
				Ref struct {
					ID string `json:"tableId"`
				} `json:"tableReference"`
				Type string `json:"type"`
			} `json:"tables"`
		}
		if err := c.json(ctx, http.MethodGet, path, values, nil, &page, l.MaxBytes); err != nil {
			return nil, err
		}
		objects := []model.Object{}
		for _, d := range page.Datasets {
			objects = append(objects, model.Object{Name: d.Ref.ID, Type: "dataset"})
		}
		for _, t := range page.Tables {
			objects = append(objects, model.Object{Name: t.Ref.ID, Namespace: ns, Type: t.Type})
		}
		for _, object := range objects {
			added, err := result.Add(object, budget)
			if err != nil {
				return nil, err
			}
			if !added {
				return nil, model.Fail("result_too_large", "Metadata page exceeds limits; retry with a smaller max_rows")
			}
		}
		if len(page.Next) > 8192 {
			return nil, cloudResponseError()
		}
		result.NextCursor = page.Next
	}
	if result.NextCursor != "" && result.RowCount == 0 {
		return nil, model.Fail("result_too_large", "No metadata progress within current limits")
	}
	return result, nil
}
