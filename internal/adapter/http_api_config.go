package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

var apiIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)
var apiHeader = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)
var apiSegment = regexp.MustCompile(`^[A-Za-z0-9_~-][A-Za-z0-9_.~-]{0,511}$`)

func validateHTTPAPI(s *model.Source) error {
	invalid := func(message string) error { return errors.New("HTTP API: " + message) }
	c := s.HTTPAPI
	if c == nil {
		return invalid("configuration is required")
	}
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > 256<<10 {
		return invalid("configuration exceeds 256 KiB")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || strings.ContainsAny(c.BaseURL, "?#\\\r\n\t ") {
		return invalid("base URL must be HTTP(S), without credentials, query or fragment")
	}
	if (u.Scheme == "https") != (s.TLSMode == "verify") {
		return invalid("base URL scheme must match TLS mode")
	}
	if u.Port() != "" {
		if n, e := strconv.Atoi(u.Port()); e != nil || n < 1 || n > 65535 {
			return invalid("invalid URL port")
		}
	}
	if u.Path != "" && u.Path != "/" && !safeAPIPath(strings.TrimSuffix(u.Path, "/"), false) || u.RawPath != "" {
		return invalid("base path must use literal safe segments")
	}
	if s.Version == "" || len(s.Version) > 120 || strings.ContainsAny(s.Version, "\r\n") {
		return invalid("an administrator-maintained API contract version is required")
	}
	if c.TokenHeader != "" {
		if !apiHeader.MatchString(c.TokenHeader) || !strings.HasPrefix(strings.ToLower(c.TokenHeader), "x-") || slices.Contains([]string{"x-http-method", "x-http-method-override", "x-method-override", "x-forwarded-host", "x-forwarded-for", "x-forwarded-proto", "x-original-url", "x-rewrite-url"}, strings.ToLower(c.TokenHeader)) {
			return invalid("API key header must be a non-routing X- header; leave blank for Bearer authentication")
		}
	}
	if strings.ContainsAny(s.Token, "\r\n\x00") {
		return invalid("invalid token")
	}
	if len(c.Operations) == 0 || len(c.Operations) > 40 {
		return invalid("configure 1–40 read operations")
	}
	ids := map[string]bool{}
	for _, op := range c.Operations {
		if !apiIdentifier.MatchString(op.ID) || ids[op.ID] {
			return invalid("operation IDs must be unique identifiers")
		}
		ids[op.ID] = true
		if strings.TrimSpace(op.Name) == "" || len(op.Name) > 120 || len(op.Description) > 4000 {
			return invalid("operation needs a name of 1–120 bytes and a description of at most 4000 bytes")
		}
		if !op.ReadOnly || (op.Method != "GET" && op.Method != "POST") {
			return invalid("only explicitly declared read-only GET and POST operations are allowed")
		}
		if !safeAPIPath(op.Path, true) {
			return invalid("operation path must be a fixed absolute path with optional whole-segment {parameters}")
		}
		if len(op.Parameters) > 32 || len(op.Columns) > 200 {
			return invalid("an operation supports up to 32 parameters and 200 declared fields")
		}
		if op.Method == "GET" && op.BodyJSON != "" {
			return invalid("GET cannot have a request body")
		}
		if op.BodyJSON != "" {
			v, e := semantic.Parse(op.BodyJSON)
			if e != nil {
				return invalid("invalid JSON body")
			}
			if _, ok := v.(map[string]any); !ok {
				return invalid("POST body must be a JSON object")
			}
		}
		if _, e := apiPointerParts(op.ResponsePointer); e != nil {
			return e
		}
		names, targets, pathParams := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, p := range op.Parameters {
			if !apiIdentifier.MatchString(p.Name) || names[p.Name] || targets[p.In+":"+p.Target] {
				return invalid("parameters need unique names and destinations")
			}
			names[p.Name], targets[p.In+":"+p.Target] = true, true
			if !slices.Contains([]string{"string", "integer", "number", "boolean"}, p.Type) {
				return invalid("HTTP parameters must be scalar strings, integers, numbers or booleans")
			}
			switch p.In {
			case "path":
				if !apiIdentifier.MatchString(p.Target) || !p.Required || !strings.Contains(op.Path, "{"+p.Target+"}") {
					return invalid("path parameters must be required and match a placeholder")
				}
				pathParams[p.Target] = true
			case "query":
				if !apiIdentifier.MatchString(p.Target) {
					return invalid("query parameter target must be an identifier")
				}
			case "body":
				if op.Method != "POST" || p.Target == "" {
					return invalid("body parameters need a POST JSON value pointer")
				}
				v, e := semantic.Parse(op.BodyJSON)
				if e != nil {
					return e
				}
				old, e := apiPointer(v, p.Target)
				if e != nil {
					return e
				}
				if !apiScalar(old) {
					return invalid("body bindings can only replace scalar values")
				}
			default:
				return invalid("parameter location must be path, query or body")
			}
			contract := apiContract(p)
			if p.DefaultJSON != "" {
				v, e := semantic.Parse(p.DefaultJSON)
				if e != nil {
					return e
				}
				if e = semantic.ValidateValue(contract, v); e != nil {
					return e
				}
			}
			for _, bound := range []string{p.Minimum, p.Maximum} {
				if bound != "" {
					rangeContract := contract
					rangeContract.EnumJSON = ""
					if e := semantic.ValidateValue(rangeContract, json.Number(bound)); e != nil {
						return e
					}
				}
			}
			if p.EnumJSON != "" {
				v, e := semantic.Parse(p.EnumJSON)
				if e != nil {
					return e
				}
				list, ok := v.([]any)
				if !ok || len(list) == 0 || len(list) > 100 {
					return invalid("enum must have 1–100 scalar values")
				}
				contract.EnumJSON = ""
				for _, item := range list {
					if e = semantic.ValidateValue(contract, item); e != nil {
						return e
					}
				}
			}
		}
		for _, segment := range strings.Split(op.Path, "/") {
			if strings.HasPrefix(segment, "{") && !pathParams[strings.Trim(segment, "{}")] {
				return invalid("every path placeholder needs a required parameter")
			}
		}
		columns := map[string]bool{}
		for _, col := range op.Columns {
			if col.Name == "" || len(col.Name) > 256 || len(col.Type) > 120 || col.Type == "" || columns[col.Name] {
				return invalid("response fields need unique names and types")
			}
			columns[col.Name] = true
		}
		if p := op.Pagination; p != nil {
			if !apiIdentifier.MatchString(p.QueryParameter) || targets["query:"+p.QueryParameter] || p.NextPointer == "" {
				return invalid("pagination needs a reserved query parameter and a next-token JSON pointer")
			}
			if _, e := apiPointerParts(p.NextPointer); e != nil {
				return e
			}
		}
		examples, e := apiExamples(op)
		if e != nil {
			return e
		}
		if _, _, _, e = bindAPI(op, examples); e != nil {
			return e
		}
	}
	if !ids[c.ProbeOperation] {
		return invalid("choose an existing operation for connection checks")
	}
	// Host fields are derived, never alternate network destinations.
	s.Host, s.Port, s.Database, s.Path = u.Hostname(), 443, "", ""
	if u.Scheme == "http" {
		s.Port = 80
	}
	if u.Port() != "" {
		s.Port, _ = strconv.Atoi(u.Port())
	}
	return nil
}

func safeAPIPath(path string, placeholders bool) bool {
	if !strings.HasPrefix(path, "/") || len(path) > 2048 {
		return false
	}
	if path == "/" {
		return true
	}
	for _, part := range strings.Split(strings.TrimSuffix(path[1:], "/"), "/") {
		if placeholders && strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && apiIdentifier.MatchString(part[1:len(part)-1]) {
			continue
		}
		if !apiSegment.MatchString(part) || strings.Contains(part, "..") {
			return false
		}
	}
	return true
}

func apiContract(p model.HTTPParameter) semantic.Parameter {
	return semantic.Parameter{Name: p.Name, Type: p.Type, Required: p.Required, DefaultJSON: p.DefaultJSON, EnumJSON: p.EnumJSON, Minimum: p.Minimum, Maximum: p.Maximum}
}

func apiExamples(op model.HTTPOperation) (map[string]any, error) {
	v, err := semantic.Parse(op.ExampleJSON)
	if err != nil {
		return nil, model.Fail("invalid_query", "Operation examples must be a JSON object")
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, model.Fail("invalid_query", "Operation examples must be a JSON object")
	}
	return m, nil
}

func apiScalar(v any) bool {
	switch v.(type) {
	case nil, string, json.Number, bool:
		return true
	}
	return false
}

func apiPointerParts(ptr string) ([]string, error) {
	if ptr == "" {
		return nil, nil
	}
	if !strings.HasPrefix(ptr, "/") || len(ptr) > 1024 {
		return nil, model.Fail("invalid_query", "Expected a bounded JSON Pointer")
	}
	parts := strings.Split(ptr[1:], "/")
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] == '~' {
				j++
				if j == len(part) || (part[j] != '0' && part[j] != '1') {
					return nil, model.Fail("invalid_query", "Invalid JSON Pointer escape")
				}
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func apiChild(v any, key string) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		if val, ok := x[key]; ok {
			return val, nil
		}
	case []any:
		if n, err := strconv.Atoi(key); err == nil && n >= 0 && n < len(x) && strconv.Itoa(n) == key {
			return x[n], nil
		}
	}
	return nil, model.Fail("invalid_query", "Configured JSON Pointer was not found")
}

func apiPointer(v any, ptr string) (any, error) {
	parts, err := apiPointerParts(ptr)
	if err != nil {
		return nil, err
	}
	for _, p := range parts {
		v, err = apiChild(v, p)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

func bindAPI(op model.HTTPOperation, supplied map[string]any) (string, url.Values, any, error) {
	path, query := op.Path, url.Values{}
	var body any
	if op.BodyJSON != "" {
		var err error
		body, err = semantic.Parse(op.BodyJSON)
		if err != nil {
			return "", nil, nil, err
		}
	}
	known := map[string]bool{}
	for _, p := range op.Parameters {
		known[p.Name] = true
		v, present := supplied[p.Name]
		if !present && p.DefaultJSON != "" {
			var err error
			v, err = semantic.Parse(p.DefaultJSON)
			if err != nil {
				return "", nil, nil, err
			}
			present = true
		}
		if !present {
			if p.Required {
				return "", nil, nil, model.Fail("invalid_query", "Missing API parameter: "+p.Name)
			}
			continue
		}
		// Normalize Go callers through the same lossless JSON contract as MCP/API.
		raw, err := json.Marshal(v)
		if err != nil || len(raw) > 8192 {
			return "", nil, nil, model.Fail("invalid_query", "API parameter exceeds 8 KiB")
		}
		v, err = semantic.Parse(string(raw))
		if err != nil {
			return "", nil, nil, err
		}
		if err = semantic.ValidateValue(apiContract(p), v); err != nil {
			return "", nil, nil, err
		}
		text := fmt.Sprint(v)
		switch p.In {
		case "path":
			if !apiSegment.MatchString(text) || strings.Contains(text, "..") {
				return "", nil, nil, model.Fail("invalid_query", "Path parameter must be one safe literal segment")
			}
			path = strings.ReplaceAll(path, "{"+p.Target+"}", text)
		case "query":
			query.Set(p.Target, text)
		case "body":
			parts, err := apiPointerParts(p.Target)
			if err != nil || len(parts) == 0 {
				return "", nil, nil, model.Fail("invalid_query", "Invalid body binding")
			}
			parent := body
			for _, part := range parts[:len(parts)-1] {
				parent, err = apiChild(parent, part)
				if err != nil {
					return "", nil, nil, err
				}
			}
			key := parts[len(parts)-1]
			old, err := apiChild(parent, key)
			if err != nil || !apiScalar(old) {
				return "", nil, nil, model.Fail("invalid_query", "Body binding requires an existing scalar value")
			}
			switch x := parent.(type) {
			case map[string]any:
				x[key] = v
			case []any:
				n, _ := strconv.Atoi(key)
				x[n] = v
			}
		}
	}
	for name := range supplied {
		if !known[name] {
			return "", nil, nil, model.Fail("invalid_query", "Unknown API parameter")
		}
	}
	return path, query, body, nil
}
