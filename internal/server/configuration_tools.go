package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/engine"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Configuration tools dispatch to fixed administrator workflows, never to a
// caller-provided HTTP route. Authentication and audit surround every dispatch.
func configurationTool[T any](s *Server, server *mcp.Server, agent model.ConfigurationAgent, name, description string, readOnly bool, run func(context.Context, T) (any, error)) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	configurationStringFields(schema, reflect.TypeFor[T]())
	resolved, err := schema.Resolve(nil)
	if err != nil {
		panic(err)
	}
	server.AddTool(&mcp.Tool{Name: name, Description: description, InputSchema: schema, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly}}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, done, err := s.startConfigurationCall(ctx, agent)
		if err != nil {
			return configurationResult(nil, err), nil
		}
		defer done()
		audit := model.AdministratorPrincipal(ctx).AttributeAudit(model.Audit{At: time.Now().UTC(), AgentID: agent.ID, EventKind: "management", Operation: "configuration." + name, RequestID: secure.Random(16), ErrorCode: "operation_pending"})
		var in T
		var plain any
		raw := request.Params.Arguments
		if len(raw) == 0 {
			raw = json.RawMessage(`{}`)
		}
		// Schema validation uses numbers only for shape checks. The actual input
		// decoder and the fixed-handler bridge retain json.Number without rounding.
		if len(raw) > 1<<20 || json.Unmarshal(raw, &plain) != nil || resolved.Validate(plain) != nil {
			err = model.Fail("invalid_input", "Arguments do not match the tool schema; check required fields, types and unknown keys")
		} else {
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			decoder.DisallowUnknownFields()
			if decoder.Decode(&in) != nil {
				err = model.Fail("invalid_input", "Invalid configuration arguments")
			}
		}
		audit.ChangedFields = managementFields(raw)
		var result any
		if err == nil {
			var refs struct {
				SourceID   string `json:"source_id"`
				OntologyID string `json:"ontology_id"`
				Revision   string `json:"revision"`
				TemplateID string `json:"template_id"`
			}
			json.Unmarshal(raw, &refs)
			audit.SourceID, audit.OntologyID, audit.Revision, audit.TemplateID = refs.SourceID, refs.OntologyID, refs.Revision, refs.TemplateID
			audit.ResourceID = refs.SourceID
			if refs.OntologyID != "" {
				audit.ResourceID = refs.OntologyID
			}
		}
		if auditErr := s.Store.Audit(audit); auditErr != nil {
			return configurationResult(nil, model.Fail("audit_unavailable", "Configuration call was not started because the audit log is unavailable")), nil
		}
		if err == nil {
			if err = model.CheckConfigurationContext(ctx); err == nil {
				result, err = run(ctx, in)
			}
			if err == nil {
				err = model.CheckConfigurationContext(ctx)
			}
		}
		audit.ErrorCode = ""
		if err != nil {
			audit.ErrorCode = engine.PublicError(err).Code
		}
		audit.ElapsedMS = time.Since(audit.At).Milliseconds()
		if out, ok := result.(map[string]any); ok {
			if id, ok := out["id"].(string); ok {
				audit.ResourceID = id
			}
			if revision, ok := out["revision"].(string); ok {
				audit.Revision = revision
			}
		}
		if name == "create_data_source" {
			audit.SourceID = audit.ResourceID
		}
		if name == "create_ontology" {
			audit.OntologyID = audit.ResourceID
		}
		if e := s.Store.Audit(audit); e != nil {
			err = model.Fail("audit_unavailable", "The call may have completed; its audit intent remains pending. Read the current revision before retrying")
		}
		return configurationResult(result, err), nil
	})
}

// jsonschema.For does not implement encoding/json's ,string option. Revisions
// and pinned versions must advertise the same lossless contract as the API.
func configurationStringFields(schema *jsonschema.Schema, typ reflect.Type) {
	if schema == nil {
		return
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		for _, field := range reflect.VisibleFields(typ) {
			name, opts, _ := strings.Cut(field.Tag.Get("json"), ",")
			property := schema.Properties[name]
			if property == nil {
				continue
			}
			if strings.Contains(","+opts+",", ",string,") {
				property.Type, property.Types = "string", nil
				property.Pattern = `^[0-9]+$`
			} else {
				configurationStringFields(property, field.Type)
			}
		}
	case reflect.Slice, reflect.Array:
		configurationStringFields(schema.Items, typ.Elem())
	case reflect.Map:
		configurationStringFields(schema.AdditionalProperties, typ.Elem())
	}
}

func configurationResult(value any, err error) *mcp.CallToolResult {
	if err != nil {
		value = map[string]any{"error": engine.PublicError(err)}
	}
	raw, marshalErr := json.Marshal(value)
	if marshalErr != nil || len(raw) > 4<<20 {
		err = model.Fail("response_too_large", "Configuration response exceeds 4 MiB; request a smaller page")
		value = map[string]any{"error": err}
		raw, _ = json.Marshal(value)
	}
	return &mcp.CallToolResult{IsError: err != nil, StructuredContent: value, Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}
}

func configurationHandler(ctx context.Context, handler http.HandlerFunc, pattern string, values map[string]string, body any) (any, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	method, _, _ := strings.Cut(pattern, " ")
	r := httptest.NewRequest(method, "/", bytes.NewReader(raw)).WithContext(ctx)
	r.Pattern = pattern
	for key, value := range values {
		r.SetPathValue(key, value)
	}
	w := httptest.NewRecorder()
	handler(w, r)
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(w.Body.Bytes()), 4<<20))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, model.Fail("response_too_large", "Cannot return configuration response; use a smaller page")
	}
	if w.Code >= 400 {
		var failure struct {
			Error *model.Error `json:"error"`
		}
		json.Unmarshal(w.Body.Bytes(), &failure)
		if failure.Error != nil {
			return nil, failure.Error
		}
		return nil, model.Fail("configuration_failed", "Configuration operation failed")
	}
	return value, nil
}
