package adapter

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
)

var parameterName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
var decimalType = regexp.MustCompile(`^DECIMAL\(([1-9]|[12][0-9]|3[0-8]),([0-9]|[12][0-9]|3[0-8])\)$`)

func cloudParameter(kind string, input any) (string, *string, error) {
	var typ string
	if typed, ok := input.(map[string]any); ok {
		var valid bool
		typ, valid = typed["type"].(string)
		v, present := typed["value"]
		if !valid || !present || len(typed) != 2 {
			return "", nil, model.Fail("invalid_arguments", "Typed cloud parameters require exactly type and value")
		}
		input = v
		typ = strings.ToUpper(typ)
	}
	var val *string
	base := "STRING"
	var text string
	switch v := input.(type) {
	case nil:
	case string:
		text = v
		val = &text
	case bool:
		base = "BOOLEAN"
		text = strconv.FormatBool(v)
		val = &text
	case json.Number:
		base = "INTEGER"
		if strings.ContainsAny(string(v), ".eE") {
			base = "DECIMAL"
		}
		text = string(v)
		val = &text
	case int:
		base = "INTEGER"
		text = strconv.Itoa(v)
		val = &text
	case int64:
		base = "INTEGER"
		text = strconv.FormatInt(v, 10)
		val = &text
	case float64:
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return "", nil, model.Fail("invalid_arguments", "Invalid numeric parameter")
		}
		base = "DECIMAL"
		text = strconv.FormatFloat(v, 'g', -1, 64)
		val = &text
	default:
		return "", nil, model.Fail("invalid_arguments", "Cloud SQL parameters must be scalar or explicit type/value pairs; arrays and structs are not supported")
	}
	if typ == "" {
		switch kind {
		case "snowflake":
			typ = map[string]string{"STRING": "TEXT", "BOOLEAN": "BOOLEAN", "INTEGER": "FIXED", "DECIMAL": "FIXED"}[base]
		case "databricks":
			typ = map[string]string{"STRING": "STRING", "BOOLEAN": "BOOLEAN", "INTEGER": "BIGINT"}[base]
			if base == "DECIMAL" {
				return "", nil, model.Fail("invalid_arguments", "Databricks exact decimals require an explicit DECIMAL(precision,scale) type and string value")
			}
		case "bigquery":
			typ = map[string]string{"STRING": "STRING", "BOOLEAN": "BOOL", "INTEGER": "INT64", "DECIMAL": "NUMERIC"}[base]
		}
	}
	allowed := map[string]map[string]bool{
		"snowflake":  wordset("TEXT FIXED REAL BOOLEAN DATE TIME TIMESTAMP_NTZ TIMESTAMP_LTZ TIMESTAMP_TZ BINARY"),
		"databricks": wordset("STRING BOOLEAN TINYINT SMALLINT INT BIGINT FLOAT DOUBLE DATE TIMESTAMP TIMESTAMP_NTZ BINARY"),
		"bigquery":   wordset("STRING BOOL INT64 FLOAT64 NUMERIC BIGNUMERIC DATE TIME DATETIME TIMESTAMP BYTES JSON GEOGRAPHY"),
	}
	decimal := false
	if kind == "databricks" {
		if match := decimalType.FindStringSubmatch(typ); match != nil {
			precision, _ := strconv.Atoi(match[1])
			scale, _ := strconv.Atoi(match[2])
			decimal = scale <= precision
		}
	}
	if !allowed[kind][strings.ToLower(typ)] && !decimal {
		return "", nil, model.Fail("invalid_arguments", "Unsupported cloud parameter type")
	}
	return typ, val, nil
}

func namedCloudParameters(kind string, q model.Query) ([]map[string]any, error) {
	if len(q.Params) != 0 {
		return nil, model.Fail("invalid_arguments", "Use named_params for this cloud SQL source")
	}
	names := make([]string, 0, len(q.NamedParams))
	for name := range q.NamedParams {
		names = append(names, name)
	}
	sort.Strings(names)
	params := make([]map[string]any, 0, len(names))
	for _, name := range names {
		if !parameterName.MatchString(name) {
			return nil, model.Fail("invalid_arguments", "Invalid cloud parameter name")
		}
		typ, val, err := cloudParameter(kind, q.NamedParams[name])
		if err != nil {
			return nil, err
		}
		if kind == "bigquery" {
			params = append(params, map[string]any{"name": name, "parameterType": map[string]any{"type": typ}, "parameterValue": map[string]any{"value": val}})
		} else {
			p := map[string]any{"name": name, "type": typ}
			if val != nil {
				p["value"] = *val
			}
			params = append(params, p)
		}
	}
	return params, nil
}
