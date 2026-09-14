package adapter

import (
	"encoding/json"
	"github.com/SamuelSupe/contextGate/internal/model"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var fluxParamName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// OSS 2.x ignores the Cloud-only params body. Literal AST nodes in extern bind
// values without interpreting strings as Flux code or interpolation expressions.
func fluxExtern(params map[string]any) (any, error) {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	properties := []any{}
	for _, key := range keys {
		if !fluxParamName.MatchString(key) {
			return nil, model.Fail("invalid_parameters", "Flux parameter names must be identifiers")
		}
		v := params[key]
		typ := ""
		switch x := v.(type) {
		case string:
			typ = "StringLiteral"
		case bool:
			typ = "BooleanLiteral"
		case json.Number:
			if n, e := x.Int64(); e == nil {
				typ = "IntegerLiteral"
				v = strconv.FormatInt(n, 10)
			} else {
				if !strings.ContainsAny(string(x), ".eE") {
					return nil, model.Fail("invalid_parameters", "Flux integer exceeds int64 range")
				}
				n, e := x.Float64()
				if e != nil || math.IsInf(n, 0) || math.IsNaN(n) {
					return nil, model.Fail("invalid_parameters", "Flux number is outside supported range")
				}
				typ = "FloatLiteral"
				v = n
			}
		case int:
			typ = "IntegerLiteral"
			v = strconv.Itoa(x)
		case int64:
			typ = "IntegerLiteral"
			v = strconv.FormatInt(x, 10)
		case float64:
			if math.IsInf(x, 0) || math.IsNaN(x) {
				return nil, model.Fail("invalid_parameters", "Flux number must be finite")
			}
			typ = "FloatLiteral"
		default:
			return nil, model.Fail("invalid_parameters", "Flux parameters support string, boolean, int64 and float64 values")
		}
		properties = append(properties, map[string]any{"type": "Property", "key": map[string]any{"type": "Identifier", "name": key}, "value": map[string]any{"type": typ, "value": v}})
	}
	return map[string]any{"type": "File", "name": "parameters", "imports": []any{}, "body": []any{map[string]any{"type": "VariableAssignment", "id": map[string]any{"type": "Identifier", "name": "params"}, "init": map[string]any{"type": "ObjectExpression", "properties": properties}}}}, nil
}
