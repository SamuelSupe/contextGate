package adapter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
	"unicode/utf8"
)

// Integers and exact decimals are strings so JSON consumers never lose precision.
func value(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		if !utf8.ValidString(x) {
			return value([]byte(x))
		}
		return x
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case []byte:
		return map[string]any{"encoding": "base64", "data": base64.StdEncoding.EncodeToString(x)}
	case json.Number:
		return string(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case int:
		return strconv.Itoa(x)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case uint32:
		return strconv.FormatUint(uint64(x), 10)
	case float64:
		if x != x || x > 1.7976931348623157e308 || x < -1.7976931348623157e308 {
			return fmt.Sprint(x)
		}
		return x
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = value(v)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			out[k] = value(v)
		}
		return out
	case fmt.Stringer:
		return x.String()
	}
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Map {
		if rv.Type().Key().Kind() == reflect.String {
			out := map[string]any{}
			it := rv.MapRange()
			for it.Next() {
				out[it.Key().String()] = value(it.Value().Interface())
			}
			return out
		}
		entries := []any{}
		it := rv.MapRange()
		for it.Next() {
			entries = append(entries, map[string]any{"key": value(it.Key().Interface()), "value": value(it.Value().Interface())})
		}
		return map[string]any{"encoding": "map", "entries": entries}
	}
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = value(rv.Index(i).Interface())
		}
		return out
	}
	return v
}
func params(in []any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		if n, ok := v.(json.Number); ok {
			if x, e := n.Int64(); e == nil {
				v = x
			} else {
				v = string(n)
			}
		}
		out[i] = v
	}
	return out
}
