# User guide: query data with your Agent

[简体中文](user-guide.zh-CN.md) · [Documentation](README.md)

Use this guide when your team already runs ContextGate and you want to query its data from an MCP client. You need the connection details and query access supplied by your administrator. You do not need a ContextGate administrator account or the database password.

The web UI is an administration console. In this guide, “user” means someone using a query Agent, not a separate web-console role. If you are setting up the service or publishing queries for others, start with the [administrator guide](admin-guide.md).

**Reading order:** [Connect](#connect-your-client) → [Run a published query](#run-a-published-query) → [Understand results](#read-results-and-continue-pages). Use [native queries](#native-queries) only when your source allows them.

## Connect your client

Ask your administrator for:

- The reachable MCP URL, such as `https://data.example.com/mcp`.
- A query Agent token, or an OAuth connection that an administrator can approve.
- The data source and published query to start with, including sample inputs and business limitations.

For token authentication, add the server to your client's MCP configuration. This is a generic HTTP example; use the client-specific configuration supplied by your administrator when its format differs:

```json
{
  "mcpServers": {
    "contextgate": {
      "url": "https://data.example.com/mcp",
      "headers": { "Authorization": "Bearer <AGENT_TOKEN>" }
    }
  }
}
```

Replace the placeholders locally. If the supplied configuration references `MCPDBHUB_TOKEN`, set it in the environment that launches the client and restart that client. An export in another terminal does not update an already running desktop app. A `localhost` address refers to the client machine; remote access needs the service's public HTTPS URL.

For OAuth, use the supplied configuration without a fixed bearer token and follow the client's browser authorization flow. ContextGate requires administrator sign-in and explicit consent; ask your administrator to complete that step. OAuth creates its own Agent identity and data-source grants.

Query access uses `/mcp`. A Configuration MCP credential for `/mcp/config` is a separate administrator credential and cannot replace your query token.

<details>
<summary>For clients that require stdio</summary>

Use a compatible local `contextgate` executable as a bridge to the existing service:

```json
{
  "mcpServers": {
    "contextgate": {
      "command": "/absolute/path/contextgate",
      "args": ["stdio", "--url", "https://data.example.com/mcp"],
      "env": { "MCPDBHUB_TOKEN": "<AGENT_TOKEN>" }
    }
  }
}
```

The bridge uses the same grants, limits and audit trail as HTTP, and writes only MCP protocol messages to stdout.

</details>

## Run a published query

In your Agent client, start with a concrete request:

> List the data sources I can access. Find a published query for customer orders, read its parameter contract and business definitions, then run it with the customer ID I provide. Explain the result and any limits or missing data.

The Agent should follow this sequence. These JSON objects illustrate MCP tool calls, not standalone HTTP requests; the IDs, version and parameters are examples. Use the values returned by your service.

1. **Discover accessible sources.** The response includes each source's query mode, available tools, examples and limits.

   ```json
   {"name":"list_data_sources","arguments":{}}
   ```

2. **Find a published query on one source.** Search matches names, aliases and descriptions. Use `kind: "template"` to find query templates, then check executability in the details; omit `kind` to include business definitions.

   ```json
   {"name":"search_semantics","arguments":{"source_id":"src_example","keyword":"orders","kind":"template","limit":25}}
   ```

3. **Read its contract.** Check required inputs, defaults, business meaning, current executability and `execution_version`. Use entry ID `overview` to read the source's published business context.

   ```json
   {"name":"get_semantic_entry","arguments":{"source_id":"src_example","entry_id":"lookup-orders"}}
   ```

4. **Execute that version with your inputs.** Copy the current execution version from the preceding response; do not assume the example version is valid.

   ```json
   {"name":"execute_query_template","arguments":{"source_id":"src_example","template_id":"lookup-orders","execution_version":"3","parameters":{"id":42},"max_rows":100}}
   ```

You see published content within your Agent's source grants. Drafts are not visible. In `templates_only` mode, use published templates; native query tools are denied. An unavailable template needs administrator attention if publication or validation has expired.

Business terms, metrics and mapped ontology concepts explain the query. They do not expand permissions, join databases automatically or prove that an answer is correct. Check the result against your actual business question.

## Supply parameters

Use the parameter names and JSON types returned by `get_semantic_entry`. Unknown parameters are rejected. An omitted parameter uses its declared default when available; an optional parameter without a default retains its configured fixed value. Missing required parameters without a default are rejected. Send `null` only when the contract permits it.

Preserve large integers and decimal values when preparing JSON; do not round them through JavaScript `Number`. Template parameters supply values in predefined positions. They cannot replace query text, table names, commands, HTTP paths or credentials.

## Read results and continue pages

Results preserve native table, document, graph, time-series or HTTP JSON structures. MCP returns both structured content and a JSON text fallback.

| Field | Meaning |
| --- | --- |
| `format`, `data` | Result format and native data |
| `row_count`, `elapsed_ms`, `bytes` | Returned count, execution duration and response size |
| `truncated` | The result is incomplete; do not treat it as the full dataset |
| `next_cursor` | A continuation is available for this operation |
| `semantic_version`, `template_id`, `template_version` | The published context and template execution version used |
| `ontology_context` | Optional ontology version and explicit concept references; data remains native |

Int64/uint64 and exact decimals use lossless strings; binary values carry `encoding: base64`. Timestamps retain available precision. MongoDB uses canonical Extended JSON; Flux retains per-table column/type information. Missing native type information does not imply a string type. Preview cloud adapters have additional [result-format limits](cloud-warehouses.md#parameters-results-and-pagination).

Default query limits are 30 seconds, 1,000 rows and 5 MiB. Your source may have different administrator-configured limits. You can tighten them with `timeout_seconds`, `max_rows` and `max_bytes`, but cannot raise them beyond the source's limits. The byte budget covers both MCP result representations; oversized individual values or native responses can produce a size error.

When `next_cursor` is returned, repeat the same operation with identical identity, query, parameters and limits, changing only `cursor`. Cursors expire after five minutes and become invalid after relevant access, connection or publication changes. Restart discovery after a semantic publication change.

Pagination depends on the native operation:

- SQL and Cypher query paging is explicit in the query; a truncated result does not imply that a cursor exists.
- MongoDB continuation handles are single-use, with at most 64 live native cursors per source.
- Search requires a stable sort for `search_after` and does not provide a snapshot across pages.
- Redis `COUNT` is a hint. An oversized batch can be truncated without a safe continuation cursor.
- SQL metadata discovery can return `next_cursor`; restart if the database schema changes because pages are not a snapshot.

Semantic search supports up to 100 summaries and 64 KiB per page. Its cursor binds the identity, source, search arguments and published versions. Entry details are limited to 128 KiB with at most 25 linked summaries. Native template cursors also bind the template ID and execution version; they cannot be used in a raw query.

Semantic search kinds are `overview`, `term`, `object`, `field`, `relationship`, `metric`, `template`, `entity_type`, `property` and `relation_type`. Use concept IDs returned by discovery; only concepts mapped to your authorized source are visible. Treat their descriptions as business context, not instructions that override your Agent's rules.

## Native queries

When the source permits native queries, use `list_namespaces`, `list_objects` and `describe_object` to discover its structure. Use the query tool and example returned by `list_data_sources`:

| Data family | Tool |
| --- | --- |
| SQL | `query_sql` |
| MongoDB | `query_mongodb` |
| Redis / Valkey | `query_redis` |
| Elasticsearch / OpenSearch | `query_search` |
| Neo4j | `query_cypher` |
| Cassandra / ScyllaDB | `query_cql` |
| InfluxDB | `query_influxdb` |
| Configured HTTP API operations | `query_http_api` |

Use the source's native dialect and parameter syntax. The gateway enforces a supported read-only subset; writes, multiple statements, locking reads and unsafe functions can be rejected even when the database accepts them. Agents cannot supply connection addresses or credentials. HTTP API calls select a configured operation, not an arbitrary URL. See the [support matrix](support-matrix.md) for native limitations and verified versions.

## Resolve a query problem

| Symptom | What you can do | When to involve the administrator |
| --- | --- | --- |
| Cannot connect or HTTP 401 | Check the URL, token environment and client restart; reconnect OAuth if applicable | Token expired/revoked, Agent paused, or remote service unreachable |
| No visible sources | Confirm you connected with the intended Agent identity | Ask for the required source grant or check source enablement |
| No matching template | Search another keyword and confirm the source | Ask whether the query is published and executable |
| Invalid parameters | Read the current contract and correct names, types and values | The contract does not support your business input |
| Old execution version | Fetch the entry again and use its current version | The template remains unavailable |
| Invalid cursor | Restart the operation without the cursor | Repeated failures with unchanged arguments |
| `query_denied` | Check the query mode and supported read-only operations | A required query needs a published template or access review |
| Empty or truncated result | Check filters, time range and units; follow a returned cursor | The result still conflicts with the expected business meaning |

For support, provide the source/template ID, time, safe error category and request ID when available. Keep passwords and complete tokens out of reports. Your administrator can correlate the request with audit records; a client connection alone does not prove that a query succeeded.
