package semantic

import (
	"slices"
	"strconv"
	"strings"
)

// BindingSlots describes existing positions using Bind's restrictions. It does
// not validate read-only execution or grant permission to run a query.
func BindingSlots(tool, query string) ([]string, error) {
	if len(query) > 128<<10 {
		return nil, invalid("Query definition exceeds 128 KiB")
	}
	value, err := Parse(query)
	if err != nil {
		return nil, err
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, invalid("Native query must be a JSON object")
	}
	slots := []string{}
	visited := 0
	var walk func(any, []string) error
	walk = func(value any, path []string) error {
		visited++
		if visited > 8192 || len(path) > 64 {
			return invalid("Query definition is too complex to list parameter positions")
		}
		native, allowedErr := allowed(tool, path)
		_, object := value.(map[string]any)
		_, array := value.([]any)
		if allowedErr == nil && (native || !object && !array) && (tool != "query_redis" || redisValueSlot(root, path)) {
			if len(slots) == 128 {
				return invalid("More than 128 parameter positions; use advanced bindings")
			}
			escaped := make([]string, len(path))
			for i, part := range path {
				escaped[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
			}
			slots = append(slots, "/"+strings.Join(escaped, "/"))
			return nil
		}
		switch v := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, key := range keys {
				if err := walk(v[key], append(slices.Clone(path), key)); err != nil {
					return err
				}
			}
		case []any:
			for i, item := range v {
				if err := walk(item, append(slices.Clone(path), strconv.Itoa(i))); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root, nil); err != nil {
		return nil, err
	}
	return slots, nil
}
