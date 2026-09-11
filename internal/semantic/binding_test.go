package semantic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTemplateBindingPreservesValuesAndOperationStructure(t *testing.T) {
	for _, tc := range []struct {
		tool, query, pointer, typ, value string
		deny                             bool
	}{
		{"query_sql", `{"query":"SELECT ?","params":[0]}`, "/params/0", "integer", `9007199254740993`, false},
		{"query_sql", `{"query":"SELECT ?","params":[0]}`, "/query", "string", `"DELETE FROM events"`, true},
		{"query_cql", `{"query":"SELECT * FROM x WHERE id=?","params":[0]}`, "/params/0", "integer", `9223372036854775807`, false},
		{"query_cypher", `{"query":"RETURN $value","named_params":{"value":null}}`, "/named_params/value", "object", `{"x":[9007199254740993]}`, false},
		{"query_influxdb", `{"language":"influxql","query":"SELECT * FROM x WHERE n=$n","named_params":{"n":0}}`, "/named_params/n", "number", `0.1234567890123456789012345`, false},
		{"query_mongodb", `{"object":"orders","filter":{"status":"paid"}}`, "/filter/status", "string", `"closed"`, false},
		{"query_mongodb", `{"object":"orders","filter":{"status":"paid"}}`, "/filter/status", "object", `{"$ne":null}`, true},
		{"query_mongodb", `{"object":"orders","filter":{"status":"paid"}}`, "/object", "string", `"secret"`, true},
		{"query_mongodb", `{"pipeline":[{"$lookup":{"from":"a"}}]}`, "/pipeline/0/$lookup/from", "string", `"secret"`, true},
		{"query_mongodb", `{"filter":{"$expr":{"$eq":["$field",0]}}}`, "/filter/$expr/$eq/1", "string", `"$secret"`, true},
		{"query_search", `{"body":{"query":{"range":{"total":{"gte":0}}}}}`, "/body/query/range/total/gte", "number", `12.00000000000000001`, false},
		{"query_search", `{"body":{"query":{"terms":{"x":{"index":"a","id":"b","path":"c"}}}}}`, "/body/query/terms/x/index", "string", `"other"`, true},
		{"query_search", `{"body":{"query":{"query_string":{"query":"foo"}}}}`, "/body/query/query_string/query", "string", `"*"`, true},
		{"query_redis", `{"command":"GET","args":["x"]}`, "/args/0", "string", `"fixed key"`, false},
		{"query_redis", `{"command":"SCAN","args":["0","MATCH","a*"]}`, "/args/1", "string", `"COUNT"`, true},
		{"query_redis", `{"command":"GET","args":["x"]}`, "/command", "string", `"SET"`, true},
	} {
		t.Run(tc.tool+tc.pointer+tc.typ, func(t *testing.T) {
			template := Template{Tool: tc.tool, QueryJSON: tc.query, ExampleJSON: `{}`, Parameters: []Parameter{{Name: "value", Type: tc.typ, Required: true, Pointers: []string{tc.pointer}}}}
			value, err := Parse(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			q, err := Bind(template, map[string]any{"value": value}, false)
			if tc.deny {
				if err == nil {
					t.Fatal("unsafe binding accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wire, _ := json.Marshal(q)
			if !strings.Contains(string(wire), tc.value) {
				t.Fatalf("value changed: %s", wire)
			}
		})
	}
}

func TestParameterContractDefaultsEnumsRangesAndDefinition(t *testing.T) {
	tplate := Template{Enabled: true, Tool: "query_sql", QueryJSON: `{"query":"SELECT ?","params":[0]}`, ExampleJSON: `{}`, Parameters: []Parameter{{Name: "n", Type: "integer", Pointers: []string{"/params/0"}, DefaultJSON: `9007199254740993`, Minimum: "9007199254740992", Maximum: "9007199254740994", EnumJSON: `[9007199254740993]`}}}
	q, err := Bind(tplate, nil, true)
	if err != nil || q.Params[0] != json.Number("9007199254740993") {
		t.Fatal(q, err)
	}
	for _, values := range []map[string]any{{"n": json.Number("1e1000000000")}, {"n": json.Number("9007199254740992")}, {"n": json.Number("9007199254740995")}, {"n": "9007199254740993"}, {"unknown": true}} {
		if _, err := Bind(tplate, values, false); err == nil {
			t.Fatal("invalid contract accepted", values)
		}
	}
	original := Definition(tplate)
	tplate.ResultDescription = "说明"
	tplate.ExecutionVersion = "9"
	tplate.ExampleJSON = `{"n":9007199254740993}`
	tplate.Parameters[0].Description = "Amount"
	if Definition(tplate) != original {
		t.Fatal("description changed execution definition")
	}
	tplate.Parameters[0].Maximum = "9007199254740996"
	if Definition(tplate) == original {
		t.Fatal("constraint failed to change execution definition")
	}
}
