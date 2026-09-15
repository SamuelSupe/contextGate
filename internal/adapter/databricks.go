package adapter

import (
	"context"
	"net/http"
	"strconv"

	"github.com/SamuelSupe/contextGate/internal/model"
)

type bricksChunk struct {
	Index int     `json:"chunk_index"`
	Next  *int    `json:"next_chunk_index"`
	Data  [][]any `json:"data_array"`
}
type bricksResponse struct {
	ID     string `json:"statement_id"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	Manifest *struct {
		Schema struct {
			Columns []struct {
				Name string `json:"name"`
				Type string `json:"type_text"`
			} `json:"columns"`
		} `json:"schema"`
		Rows      int  `json:"total_row_count"`
		Truncated bool `json:"truncated"`
	} `json:"manifest"`
	Result bricksChunk `json:"result"`
}

func (c *cloudConn) databricks(ctx context.Context, q model.Query, l model.Limits) (result *model.Result, err error) {
	parameters, err := namedCloudParameters("databricks", q)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"statement": q.Query, "warehouse_id": c.s.Options["warehouse_id"], "catalog": c.s.Database, "parameters": parameters, "format": "JSON_ARRAY", "disposition": "INLINE", "wait_timeout": "0s", "on_wait_timeout": "CONTINUE", "row_limit": l.MaxRows + 1, "byte_limit": l.MaxBytes}
	if schema := c.s.Options["schema"]; schema != "" {
		body["schema"] = schema
	}
	var response bricksResponse
	err = c.json(ctx, http.MethodPost, "/api/2.0/sql/statements", nil, body, &response, l.MaxBytes)
	id := response.ID
	if cloudID.MatchString(id) {
		defer func() {
			if err != nil {
				c.cancel("/api/2.0/sql/statements/"+id+"/cancel", nil)
			}
		}()
	}
	if err != nil {
		return nil, err
	}
	if !cloudID.MatchString(id) {
		return nil, cloudResponseError()
	}
	for response.Status.State == "PENDING" || response.Status.State == "RUNNING" {
		if err = cloudPause(ctx); err != nil {
			return nil, err
		}
		response = bricksResponse{}
		if err = c.json(ctx, http.MethodGet, "/api/2.0/sql/statements/"+id, nil, nil, &response, l.MaxBytes); err != nil {
			return nil, err
		}
	}
	if response.Status.State != "SUCCEEDED" {
		return nil, model.Fail("database_query", "Databricks did not complete the SELECT query")
	}
	if response.Manifest == nil || len(response.Manifest.Schema.Columns) == 0 {
		return nil, cloudResponseError()
	}
	result = model.NewResult("table")
	for _, col := range response.Manifest.Schema.Columns {
		result.Columns = append(result.Columns, model.Column{Name: col.Name, Type: col.Type})
	}
	chunk := response.Result
	for {
		for _, row := range chunk.Data {
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
		if chunk.Next == nil {
			break
		}
		if result.RowCount >= l.MaxRows {
			result.Truncated = true
			return result, nil
		}
		next := *chunk.Next
		if next <= chunk.Index || next > 10000 {
			return nil, cloudResponseError()
		}
		chunk = bricksChunk{}
		if err = c.json(ctx, http.MethodGet, "/api/2.0/sql/statements/"+id+"/result/chunks/"+strconv.Itoa(next), nil, nil, &chunk, l.MaxBytes); err != nil {
			return nil, err
		}
		if chunk.Index != next {
			return nil, cloudResponseError()
		}
	}
	if result.RowCount != response.Manifest.Rows {
		return nil, cloudResponseError()
	}
	result.Truncated = response.Manifest.Truncated
	return result, nil
}
