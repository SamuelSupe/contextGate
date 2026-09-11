# OTLP audit log export

[简体中文](audit-export.zh-CN.md) · [README](../README.md)

Available since v0.1.1. Enable **Settings → Audit log export** to send persisted database audit events to an OpenTelemetry Collector or another OTLP Logs receiver. Export is disabled by default and does not change the local audit log. It uses standard OTLP protobuf messages over HTTP or gRPC.

## Configure

1. Select **OTLP HTTP / protobuf** or **OTLP gRPC**.
2. Enter the receiver endpoint. For HTTP, use `http://collector:4318/v1/logs`; an origin without a path automatically gets `/v1/logs`. For gRPC, use `http://collector:4317`, with an explicit port and no path. `https://` enables TLS for either protocol.
3. Set **Service name** (`mcpdbhub` by default). Optionally supply **Headers (JSON)**, for example `{"Authorization":"Bearer <TOKEN>"}`. Values are encrypted with the existing AES-256-GCM master key and never returned by the API. Leave the field blank to retain stored headers, provide JSON to replace them, or select **Clear stored headers** to remove them.
4. For private HTTPS receivers, paste trusted **CA certificates (PEM)**. These extend system trust; certificate and hostname verification stay enabled. Client certificate authentication is not currently supported. URLs cannot contain credentials, query parameters or fragments; use headers for authentication. HTTP redirects are rejected.
5. Choose **Send test log** to send one synthetic `mcpdbhub.audit_export.test` event using the current form. This does not save settings or replay audit records. A successful response proves receiver acceptance, not final storage in its downstream backend.
6. Enable export and **Save export settings**. Generate a query and check **Delivery status** for pending, accepted and rejected event counts, last success and retry information. Counters are cumulative for this installation, across configuration changes, and exclude synthetic test logs.

Endpoints are resolved and contacted by the **Go server**, not the browser. With Docker, `localhost` refers to the Hub container. Use a shared Docker network and the Collector's container/service name, or an address reachable from the server.

## Try a local Collector

The [example configuration](../examples/otel-collector.yaml) prints decoded logs for development. It was tested with Collector **0.160.0**:

```sh
docker network create hub-telemetry
docker run --rm --name collector --network hub-telemetry \
  -p 127.0.0.1:4317:4317 -p 127.0.0.1:4318:4318 \
  -v "$PWD/examples/otel-collector.yaml:/etc/otelcol/config.yaml:ro" \
  otel/opentelemetry-collector:0.160.0 --config=/etc/otelcol/config.yaml
```

For a host-run Hub, configure `http://127.0.0.1:4318/v1/logs` or `http://127.0.0.1:4317`. For a containerized Hub, attach it to `hub-telemetry` and configure `http://collector:4318/v1/logs` or `http://collector:4317`. Use a Collector exporter for your actual observability backend in production.

## Log fields

Resource attributes are `service.name`, `service.version` and a persistent `service.instance.id`. Each log preserves the audit event timestamp; successful operations use INFO and failures use ERROR. The body is a fixed success/failure message.

| Attribute | Value |
|---|---|
| `event.name` | `mcpdbhub.audit` |
| `mcpdbhub.audit.id` | Local audit record ID, int64 |
| `mcpdbhub.audit.request_id` | Request correlation ID, when available |
| `mcpdbhub.audit.agent_id` | Calling identity, when available |
| `mcpdbhub.audit.source_id` | Configured source ID |
| `mcpdbhub.audit.operation` | Query, discovery or connection-check operation |
| `mcpdbhub.audit.template_id` | Template ID for execution or trial (since 0.2.0) |
| `mcpdbhub.audit.template_version` | Published template execution version, when applicable |
| `mcpdbhub.audit.query_fingerprint` | Existing keyed fingerprint, when available |
| `mcpdbhub.audit.elapsed_ms` | Duration in milliseconds, int64 |
| `mcpdbhub.audit.rows` | Returned row/item count, int64 |
| `mcpdbhub.audit.preview` | Whether this was an administrator preview |
| `mcpdbhub.audit.error_code` | Safe error category, on failure |
| `mcpdbhub.audit.native_code` | Sanitized database error code, when available |
| `mcpdbhub.audit.attributes_truncated` | Present and true if a string exceeded 2,048 characters |

Export follows the existing database audit scope: query execution, discovery through the engine, administrator previews and connection checks. Login failures, rejected HTTP authentication and configuration changes are not a separate security-event feed. Full queries, parameter values, query results, database credentials and exporter authentication headers are excluded. Receiver error bodies are not exposed in status messages.

## Delivery and retention

- The worker reads committed SQLite audit records in ID order, up to 64 events and 512 KiB per batch. It polls every two seconds when idle and drains a backlog in smaller intervals. Receiver requests time out after ten seconds and responses are limited to 64 KiB. Remote delivery never runs in the query request path.
- First enable and every re-enable start at the current audit tail: historical and disabled-period events are not exported. Disabling cancels the active send and clears the export backlog without deleting local audit records. Updating an enabled destination preserves the queue, so pending events go to the new destination.
- Configuration and the acknowledged position are encrypted and persisted together. Pending events and retry state survive restarts. Events already accepted remotely can be repeated if an acknowledgement is lost, a checkpoint cannot be saved, or configuration changes while sending. Deduplicate by `service.instance.id` plus `mcpdbhub.audit.id`; this is not exactly-once delivery.
- Retryable network/server failures use exponential backoff with jitter, honoring receiver retry hints (capped at 24 hours). HTTP retries cover 429, 502, 503 and 504; gRPC follows OTLP status and RetryInfo rules.
- OTLP partial rejections are counted and not retried. A permanent failure rejects the current batch and blocks subsequent exports until corrected settings are saved. Original local audit records remain available; saving resumes subsequent pending events and does not replay the rejected batch. These rules follow the [OTLP specification](https://opentelemetry.io/docs/specs/otlp/).
- The queue shares local audit retention: events older than 30 days may be removed, including unsent ones. This is bounded retention, not an indefinite delivery guarantee. Back up both the configuration database and matching master key as described in the README.

## Administration API

These endpoints require the administrator session; mutations also require `X-CSRF-Token`. Agent tokens cannot access them.

| Method | Endpoint | Behavior |
|---|---|---|
| GET | `/api/settings/audit-export` | Redacted configuration and live delivery status |
| PUT | `/api/settings/audit-export` | Validate and save configuration |
| POST | `/api/settings/audit-export/test` | Send one synthetic log without saving |

Use `config.revision` from GET in PUT or POST. Stale revisions return HTTP 409. Omit `headers` to keep existing values; `{}` or `clear_headers: true` removes them. A successful PUT returns the updated redacted view. Invalid configuration returns HTTP 400. The test endpoint returns `accepted`, `message` and `checked_at`; HTTP 200 alone does not mean the test was accepted.

```json
{
  "revision": 0,
  "enabled": true,
  "protocol": "http/protobuf",
  "endpoint": "https://collector.example.com/v1/logs",
  "service_name": "mcpdbhub",
  "headers": {"Authorization": "Bearer <TOKEN>"},
  "ca_pem": ""
}
```
