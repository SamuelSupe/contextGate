package semantic

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,95}$`)

func invalid(message string) error { return model.Fail("invalid_semantics", message) }

func Parse(text string) (any, error) {
	var v any
	d := json.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, invalid("Invalid JSON document")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, invalid("Expected one JSON document")
	}
	return v, nil
}

func Validate(s Snapshot, tool string) error {
	if s.FormatVersion != FormatVersion && s.FormatVersion != 1 {
		return invalid("Unsupported semantic format version")
	}
	b, _ := json.Marshal(s)
	if len(s.Entries) > MaxEntries || len(b) > MaxSnapshotBytes {
		return invalid("Catalog exceeds 500 entries or 768 KiB")
	}
	ids := map[string]Entry{}
	for _, en := range s.Entries {
		if en.Definition != nil || en.Mapping != nil || len(en.Ancestors) > 0 || len(en.TemplateIDs) > 0 {
			return invalid("Projected ontology entries cannot be saved in the catalog")
		}
		if en.ID == "overview" || !identifier.MatchString(en.ID) || strings.TrimSpace(en.Name) == "" || len(en.Name) > 256 {
			return invalid("Entries need a stable ID and a name of at most 256 bytes")
		}
		if _, exists := ids[en.ID]; exists {
			return invalid("Duplicate entry ID: " + en.ID)
		}
		ids[en.ID] = en
		switch en.Kind {
		case "term", "metric", "object", "field", "relationship", "template":
		default:
			return invalid("Unknown entry kind: " + en.Kind)
		}
		if (en.Kind == "object" || en.Kind == "field") && en.Reference == nil {
			return invalid("Object and field entries require an explicit reference")
		}
		refs := append([]Reference(nil), en.Related...)
		if en.Reference != nil {
			refs = append(refs, *en.Reference)
		}
		for _, ref := range refs {
			if ref.Object == "" {
				return invalid("References require an object; use an empty namespace only for databases without namespaces")
			}
		}
		if en.Kind == "field" && en.Reference.Field == "" {
			return invalid("Field entries require a field path")
		}
		if en.Kind == "relationship" && len(en.Related) < 2 {
			return invalid("Relationships require at least two references in this data source")
		}
		if en.Kind == "template" {
			if en.Template == nil || en.Template.Tool != tool {
				return invalid("Template must use this data source's native query tool")
			}
			if _, err := Bind(*en.Template, nil, true); err != nil {
				return fmt.Errorf("%s: %w", en.ID, err)
			}
			if err := ValidateRegression(*en.Template); err != nil {
				return fmt.Errorf("%s: %w", en.ID, err)
			}
		} else if en.Template != nil {
			return invalid("Only template entries can contain executable definitions")
		}
	}
	for _, en := range s.Entries {
		if en.TemplateID != "" && ids[en.TemplateID].Kind != "template" {
			return invalid("Linked template does not exist: " + en.TemplateID)
		}
	}
	return nil
}

func pointerParts(ptr string) ([]string, error) {
	if !strings.HasPrefix(ptr, "/") {
		return nil, invalid("Bindings require an absolute JSON Pointer")
	}
	p := strings.Split(ptr[1:], "/")
	for i, v := range p {
		for j := 0; j < len(v); j++ {
			if v[j] == '~' {
				j++
				if j == len(v) || (v[j] != '0' && v[j] != '1') {
					return nil, invalid("Invalid JSON Pointer escape")
				}
			}
		}
		p[i] = strings.ReplaceAll(strings.ReplaceAll(v, "~1", "/"), "~0", "~")
	}
	return p, nil
}

func child(parent any, key string) (any, error) {
	switch v := parent.(type) {
	case map[string]any:
		if val, ok := v[key]; ok {
			return val, nil
		}
	case []any:
		n, err := strconv.Atoi(key)
		if err == nil && strconv.Itoa(n) == key && n >= 0 && n < len(v) {
			return v[n], nil
		}
	}
	return nil, invalid("Binding must target an existing value")
}

func allowed(tool string, parts []string) (native bool, err error) {
	if len(parts) < 2 {
		return false, invalid("Binding cannot replace a query document")
	}
	switch tool {
	case "query_sql":
		if len(parts) == 2 && (parts[0] == "params" || parts[0] == "named_params") {
			return true, nil
		}
	case "query_cql":
		if len(parts) == 2 && parts[0] == "params" {
			return true, nil
		}
	case "query_http_api":
		if len(parts) == 2 && parts[0] == "named_params" {
			return false, nil
		}
	case "query_cypher", "query_influxdb":
		if len(parts) == 2 && parts[0] == "named_params" {
			return true, nil
		}
	case "query_redis":
		if len(parts) == 2 && parts[0] == "args" {
			return false, nil
		}
	case "query_mongodb":
		if parts[0] == "filter" || len(parts) >= 4 && parts[0] == "pipeline" && parts[2] == "$match" {
			for _, p := range parts {
				if slices.Contains([]string{"$expr", "$where", "$function", "$accumulator", "$jsonSchema", "$text"}, p) {
					return false, invalid("Binding into expressions or scripts is forbidden")
				}
			}
			return false, nil
		}
	case "query_search":
		if parts[0] == "body" && len(parts) > 3 && parts[1] == "query" && searchValueSlot(parts[2:]) {
			return false, nil
		}
	}
	return false, invalid("Binding destination is not a supported native parameter or document value position")
}

// Traverse query clauses explicitly: field names can themselves be named term,
// range or match, and must never be mistaken for an operator in a lookup object.
func searchValueSlot(parts []string) bool {
	arrayIndex := func(s string) bool {
		n, err := strconv.Atoi(s)
		return err == nil && n >= 0 && strconv.Itoa(n) == s
	}
	for len(parts) >= 2 {
		switch parts[0] {
		case "term", "match", "match_phrase":
			option := "query"
			if parts[0] == "term" {
				option = "value"
			}
			return len(parts) == 2 || len(parts) == 3 && parts[2] == option
		case "range":
			return len(parts) == 3 && slices.Contains([]string{"gt", "gte", "lt", "lte"}, parts[2])
		case "terms":
			return len(parts) == 3 && arrayIndex(parts[2])
		case "ids":
			return len(parts) == 3 && parts[1] == "values" && arrayIndex(parts[2])
		case "bool":
			if !slices.Contains([]string{"must", "filter", "should", "must_not"}, parts[1]) {
				return false
			}
			parts = parts[2:]
			if len(parts) > 0 && arrayIndex(parts[0]) {
				parts = parts[1:]
			}
		case "nested", "has_child", "has_parent", "function_score":
			if parts[1] != "query" {
				return false
			}
			parts = parts[2:]
		case "constant_score":
			if parts[1] != "filter" {
				return false
			}
			parts = parts[2:]
		case "dis_max":
			if len(parts) < 3 || parts[1] != "queries" || !arrayIndex(parts[2]) {
				return false
			}
			parts = parts[3:]
		case "boosting":
			if parts[1] != "positive" && parts[1] != "negative" {
				return false
			}
			parts = parts[2:]
		default:
			return false
		}
	}
	return false
}

func number(v any) (*big.Rat, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return nil, false
	}
	text := string(n)
	if len(text) > 1024 || !json.Valid([]byte(text)) {
		return nil, false
	}
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		exponent, err := strconv.Atoi(text[i+1:])
		if err != nil || exponent > 10000 || exponent < -10000 {
			return nil, false
		}
	}
	r, ok := new(big.Rat).SetString(text)
	return r, ok
}

// ValidateValue checks a typed parameter without converting exact JSON numbers.
func ValidateValue(p Parameter, v any) error {
	ok := false
	switch p.Type {
	case "string":
		_, ok = v.(string)
	case "boolean":
		_, ok = v.(bool)
	case "number", "integer":
		n, yes := number(v)
		ok = yes && (p.Type == "number" || n.IsInt())
	case "object":
		_, ok = v.(map[string]any)
	case "array":
		_, ok = v.([]any)
	case "null":
		ok = v == nil
	}
	if !ok {
		return invalid("Parameter " + p.Name + " must be " + p.Type)
	}
	for _, bound := range []struct {
		text    string
		minimum bool
	}{{p.Minimum, true}, {p.Maximum, false}} {
		if bound.text == "" {
			continue
		}
		n, ok := number(v)
		limit, valid := number(json.Number(bound.text))
		if !ok || !valid || (bound.minimum && n.Cmp(limit) < 0) || (!bound.minimum && n.Cmp(limit) > 0) {
			return invalid("Parameter " + p.Name + " is outside its numeric range")
		}
	}
	if p.EnumJSON != "" {
		choices, err := Parse(p.EnumJSON)
		if err != nil {
			return err
		}
		list, ok := choices.([]any)
		if !ok || len(list) == 0 {
			return invalid("Enum must be a non-empty JSON array")
		}
		match := false
		for _, option := range list {
			if reflect.DeepEqual(option, v) {
				match = true
			}
			if a, yes := number(option); yes {
				if b, yes := number(v); yes && a.Cmp(b) == 0 {
					match = true
				}
			}
		}
		if !match {
			return invalid("Parameter " + p.Name + " is not in its enum")
		}
	}
	return nil
}

// Bind only replaces existing value slots; neither object keys nor any part of
// a query string is interpreted as a substitution expression.
func Bind(t Template, supplied map[string]any, examples bool) (model.Query, error) {
	var q model.Query
	v, err := Parse(t.QueryJSON)
	if err != nil {
		return q, err
	}
	if len(t.Parameters) > 128 {
		return q, invalid("Templates support at most 128 parameters")
	}
	root, ok := v.(map[string]any)
	if !ok {
		return q, invalid("Native query must be a JSON object")
	}
	fields := map[string]string{
		"query_sql": "query params named_params", "query_cql": "query params", "query_cypher": "query named_params", "query_influxdb": "query named_params language", "query_redis": "command args", "query_mongodb": "namespace object operation query filter projection sort pipeline", "query_search": "object operation query body", "query_http_api": "operation named_params",
	}
	for key := range root {
		if !slices.Contains(strings.Fields(fields[t.Tool]), key) {
			return q, invalid("Unsupported field in native template query")
		}
	}
	for _, key := range []string{"source_id", "cursor", "max_rows", "max_bytes", "timeout_seconds"} {
		if _, exists := root[key]; exists {
			return q, invalid("Template cannot set " + key)
		}
	}
	if examples {
		ev, err := Parse(t.ExampleJSON)
		if err != nil {
			return q, err
		}
		supplied, ok = ev.(map[string]any)
		if !ok {
			return q, invalid("Example parameters must be a JSON object")
		}
	}
	names, pointers := map[string]bool{}, map[string]bool{}
	for _, p := range t.Parameters {
		if !identifier.MatchString(p.Name) || names[p.Name] || len(p.Pointers) == 0 || len(p.Pointers) > 32 {
			return q, invalid("Parameters need unique names and at least one binding")
		}
		names[p.Name] = true
		if !slices.Contains([]string{"string", "integer", "number", "boolean", "object", "array", "null"}, p.Type) {
			return q, invalid("Unknown parameter type")
		}
		if p.EnumJSON != "" {
			choices, err := Parse(p.EnumJSON)
			if err != nil {
				return q, err
			}
			list, ok := choices.([]any)
			if !ok || len(list) == 0 {
				return q, invalid("Enum must be a non-empty JSON array")
			}
			contract := p
			contract.EnumJSON = ""
			for _, value := range list {
				if err = ValidateValue(contract, value); err != nil {
					return q, err
				}
			}
		}
		if p.DefaultJSON != "" {
			value, err := Parse(p.DefaultJSON)
			if err != nil {
				return q, err
			}
			if err = ValidateValue(p, value); err != nil {
				return q, err
			}
		}
		val, present := supplied[p.Name]
		if !present && p.DefaultJSON != "" {
			val, err = Parse(p.DefaultJSON)
			if err != nil {
				return q, err
			}
			present = true
		}
		if !present && p.Required {
			return q, invalid("Missing required parameter: " + p.Name)
		}
		if present {
			if err = ValidateValue(p, val); err != nil {
				return q, err
			}
		}
		for _, ptr := range p.Pointers {
			if pointers[ptr] {
				return q, invalid("Two parameters cannot bind the same value")
			}
			pointers[ptr] = true
			parts, err := pointerParts(ptr)
			if err != nil {
				return q, err
			}
			if t.Tool == "query_redis" && !redisValueSlot(root, parts) {
				return q, invalid("Redis bindings cannot change command options")
			}
			native, err := allowed(t.Tool, parts)
			if err != nil {
				return q, err
			}
			if !native && (p.Type == "object" || p.Type == "array") {
				return q, invalid("Structured values require a native database parameter slot")
			}
			parent := any(root)
			for _, part := range parts[:len(parts)-1] {
				parent, err = child(parent, part)
				if err != nil {
					return q, err
				}
			}
			key := parts[len(parts)-1]
			original, err := child(parent, key)
			if err != nil {
				return q, err
			}
			if !native {
				switch original.(type) {
				case map[string]any, []any:
					return q, invalid("Document binding cannot replace operation structure")
				}
			}
			if !present {
				val = original
				if err = ValidateValue(p, val); err != nil {
					return q, err
				}
			}
			if !native {
				if str, ok := val.(string); ok && t.Tool == "query_mongodb" && strings.HasPrefix(str, "$") {
					return q, invalid("Document parameters cannot introduce field or variable references")
				}
			}
			if t.Tool == "query_redis" && p.Type != "string" {
				return q, invalid("Redis argument parameters must be strings")
			}
			switch obj := parent.(type) {
			case map[string]any:
				obj[key] = val
			case []any:
				index, _ := strconv.Atoi(key)
				obj[index] = val
			}
		}
	}
	for name := range supplied {
		if !names[name] {
			return q, invalid("Unknown template parameter")
		}
	}
	b, err := json.Marshal(root)
	if err != nil {
		return q, err
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	d.DisallowUnknownFields()
	if err = d.Decode(&q); err != nil {
		return q, invalid("Invalid native query structure")
	}
	return q, nil
}

func redisValueSlot(root map[string]any, parts []string) bool {
	if len(parts) != 2 || parts[0] != "args" {
		return false
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 {
		return false
	}
	args, ok := root["args"].([]any)
	if !ok || index >= len(args) {
		return false
	}
	command, _ := root["command"].(string)
	command = strings.ToUpper(command)
	switch command {
	case "SCAN", "HSCAN", "SSCAN", "ZSCAN":
		offset := 0
		if command != "SCAN" {
			offset = 1
		}
		if index <= offset {
			return true
		}
		return index > offset+1 && (index-offset)%2 == 0
	case "ZRANGE", "ZREVRANGE", "ZRANGEBYSCORE", "ZREVRANGEBYSCORE", "XRANGE", "XREVRANGE":
		if index < 3 {
			return true
		}
		previous, _ := args[index-1].(string)
		return strings.EqualFold(previous, "COUNT") || strings.EqualFold(previous, "LIMIT") || index > 4 && strings.EqualFold(model.String(args[index-2]), "LIMIT")
	default:
		return true
	}
}
