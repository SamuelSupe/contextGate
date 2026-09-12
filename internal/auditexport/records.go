package auditexport

import (
	"strings"
	"time"

	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/version"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func textAttribute(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
}
func intAttribute(key string, value int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: value}}}
}
func boolAttribute(key string, value bool) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: value}}}
}

func envelope(config Config, instance string, records []*logspb.LogRecord) *collectorpb.ExportLogsServiceRequest {
	return &collectorpb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
			textAttribute("service.name", config.ServiceName), textAttribute("service.version", version.Version), textAttribute("service.instance.id", instance),
		}},
		ScopeLogs: []*logspb.ScopeLogs{{Scope: &commonpb.InstrumentationScope{Name: "github.com/SamuelSupe/mcpdbhub/audit", Version: version.Version}, LogRecords: records}},
	}}}
}

func auditRequest(config Config, instance string, audits []model.Audit) (*collectorpb.ExportLogsServiceRequest, int) {
	request := envelope(config, instance, nil)
	scope := request.ResourceLogs[0].ScopeLogs[0]
	size := proto.Size(request)
	for _, a := range audits {
		attrs := []*commonpb.KeyValue{
			textAttribute("event.name", "mcpdbhub.audit"), intAttribute("mcpdbhub.audit.id", a.ID),
			intAttribute("mcpdbhub.audit.elapsed_ms", a.ElapsedMS), intAttribute("mcpdbhub.audit.rows", int64(a.Rows)), boolAttribute("mcpdbhub.audit.preview", a.Preview),
		}
		truncated := false
		for _, entry := range []struct{ key, value string }{
			{"request_id", a.RequestID}, {"agent_id", a.AgentID}, {"source_id", a.SourceID}, {"operation", a.Operation},
			{"template_id", a.TemplateID}, {"template_version", a.TemplateVersion}, {"ontology_id", a.OntologyID}, {"ontology_version", a.OntologyVersion},
			{"query_fingerprint", a.Fingerprint}, {"error_code", a.ErrorCode}, {"native_code", a.NativeCode},
		} {
			value := []rune(strings.ToValidUTF8(entry.value, "�"))
			if len(value) > 2048 {
				value = value[:2048]
				truncated = true
			}
			if len(value) > 0 {
				attrs = append(attrs, textAttribute("mcpdbhub.audit."+entry.key, string(value)))
			}
		}
		if truncated {
			attrs = append(attrs, boolAttribute("mcpdbhub.audit.attributes_truncated", true))
		}
		record := &logspb.LogRecord{TimeUnixNano: uint64(a.At.UnixNano()), ObservedTimeUnixNano: uint64(time.Now().UnixNano()), SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO, SeverityText: "INFO", Attributes: attrs, Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Database audit: operation completed"}}}
		if a.ErrorCode != "" {
			record.SeverityNumber = logspb.SeverityNumber_SEVERITY_NUMBER_ERROR
			record.SeverityText = "ERROR"
			record.Body = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Database audit: operation failed"}}
		}
		// Include protobuf length prefixes, leaving ample room below the transport limit.
		size += proto.Size(record) + 16
		if size > 512<<10 {
			break
		}
		scope.LogRecords = append(scope.LogRecords, record)
	}
	return request, len(scope.LogRecords)
}

func testRequest(config Config, instance string) *collectorpb.ExportLogsServiceRequest {
	now := uint64(time.Now().UnixNano())
	return envelope(config, instance, []*logspb.LogRecord{{TimeUnixNano: now, ObservedTimeUnixNano: now, SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO, SeverityText: "INFO", Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Audit log export connection test"}}, Attributes: []*commonpb.KeyValue{textAttribute("event.name", "mcpdbhub.audit_export.test")}}})
}
