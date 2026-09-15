package adapter

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/SamuelSupe/contextGate/internal/model"
)

type snowResponse struct {
	Handle   string `json:"statementHandle"`
	Code     string `json:"code"`
	Metadata *struct {
		Rows    int `json:"numRows"`
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"rowType"`
		Partitions []struct {
			Rows int `json:"rowCount"`
		} `json:"partitionInfo"`
	} `json:"resultSetMetaData"`
	Data [][]any `json:"data"`
}

func (c *cloudConn) snowflake(ctx context.Context, q model.Query, l model.Limits) (result *model.Result, err error) {
	if len(q.NamedParams) != 0 {
		return nil, model.Fail("invalid_arguments", "Snowflake uses positional params and ? placeholders")
	}
	bindings := map[string]any{}
	for i, v := range q.Params {
		typ, val, e := cloudParameter("snowflake", v)
		if e != nil {
			return nil, e
		}
		bindings[strconv.Itoa(i+1)] = map[string]any{"type": typ, "value": val}
	}
	body := map[string]any{"statement": q.Query, "timeout": l.TimeoutSeconds, "database": c.s.Database, "warehouse": c.s.Options["warehouse"], "role": c.s.Options["role"], "bindings": bindings, "parameters": map[string]string{"MULTI_STATEMENT_COUNT": "1"}}
	if schema := c.s.Options["schema"]; schema != "" {
		body["schema"] = schema
	}
	var response snowResponse
	err = c.json(ctx, http.MethodPost, "/api/v2/statements", url.Values{"async": {"true"}}, body, &response, l.MaxBytes)
	handle := response.Handle
	if cloudID.MatchString(handle) {
		defer func() {
			if err != nil {
				c.cancel("/api/v2/statements/"+handle+"/cancel", nil)
			}
		}()
	}
	if err != nil {
		return nil, err
	}
	if !cloudID.MatchString(handle) {
		return nil, cloudResponseError()
	}
	for response.Metadata == nil {
		if response.Code != "333334" {
			return nil, model.Fail("database_query", "Snowflake did not complete the SELECT query")
		}
		if err = cloudPause(ctx); err != nil {
			return nil, err
		}
		response = snowResponse{}
		if err = c.json(ctx, http.MethodGet, "/api/v2/statements/"+handle, nil, nil, &response, l.MaxBytes); err != nil {
			return nil, err
		}
	}
	meta := response.Metadata
	if len(meta.Columns) == 0 || meta.Rows < 0 {
		return nil, cloudResponseError()
	}
	result = model.NewResult("table")
	for _, col := range meta.Columns {
		result.Columns = append(result.Columns, model.Column{Name: col.Name, Type: col.Type})
	}
	partitions := len(meta.Partitions)
	if partitions == 0 {
		partitions = 1
	}
	if partitions > 10000 {
		return nil, model.Fail("result_too_large", "Too many Snowflake result partitions; narrow the query")
	}
	for partition := 0; partition < partitions; partition++ {
		if partition > 0 {
			response = snowResponse{}
			if err = c.json(ctx, http.MethodGet, "/api/v2/statements/"+handle, url.Values{"partition": {strconv.Itoa(partition)}}, nil, &response, l.MaxBytes); err != nil {
				return nil, err
			}
		}
		for _, row := range response.Data {
			if len(row) != len(result.Columns) {
				return nil, cloudResponseError()
			}
			var added bool
			added, err = result.Add(value(row), l)
			if err != nil {
				return nil, err
			}
			if !added {
				return result, nil
			}
		}
		if result.RowCount >= l.MaxRows && result.RowCount < meta.Rows {
			result.Truncated = true
			return result, nil
		}
	}
	if result.RowCount != meta.Rows {
		return nil, cloudResponseError()
	}
	return result, nil
}
