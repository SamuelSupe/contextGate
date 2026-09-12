# Semantic catalogs and query templates

Since 0.3.0, source JSON exports use format version 2 and may include a shared ontology mapping; v1 imports and existing trial evidence remain compatible. See [shared ontologies](ontologies.md).

[简体中文](semantics.zh-CN.md) · [Examples](../examples/semantics/) · [Architecture](architecture.md)

Available in **0.2.0**. Each data source owns an independent catalog of business terms, objects, fields, relationships, metrics and executable native query templates. An Agent's existing data source grant controls access to the published catalog and its templates. Business descriptions are context, never instructions or permission rules.

## Administrator workflow

1. Open **Data sources → Semantics**. Write the source's business context in **Overview**.
2. In **Catalog**, import selected schema objects or create structured entries. Complete definitions, aliases, units, enum meanings, time conventions, metric grain and caveats. References use explicit `namespace`, `object` and optional `field` paths. Relationships stay within this data source.
3. In **Query templates**, author the native query JSON and define parameters. Link metrics to a template in the metric editor. Save updates to the draft.
4. Run **Validate draft** and **Trial** for each new or changed enabled template. Trials use the saved example parameters against the real database through the read-only engine. A failed trial cannot establish publication approval. Result rows are discarded; the sanitized audit record and encrypted validation proof remain.
5. Select **Publish** and confirm. Publication replaces the complete Agent-visible snapshot atomically. **Discard draft** restores the published content. Other administrators' tabs receive a conflict when saving an old revision.

Saving, JSON import and schema import do not publish. Descriptive changes preserve an existing template's execution version and do not cancel its running queries. Changed, disabled or deleted executable templates cancel affected queries after publication. Old execution versions must be refreshed.

**Query access** in the source editor defaults to **Native queries and templates**. Select **Templates only** to prohibit Agent native queries through HTTP, stdio, OAuth and Agent-identity previews. Discovery and semantic reads remain available. With no executable published templates, all Agent queries are denied. Administrators retain native preview and draft trial access.

## Validation and connection changes

Successful trial evidence binds the normalized executable definition and the source's connection configuration, credentials and observed database version. It is separate from importable catalog content. Renaming a source, changing descriptions or editing query limits does not invalidate evidence. Execution uses the latest configured limits.

A connection or credential edit invalidates previous proof even if the same settings are later restored. The acquired connection's server version is checked before every template query. An observed upgrade expires approval and cancels source work. Version metadata must be readable; unavailable version information pauses template execution. The initial version observation does not change the source's editing revision. A changed known version does.

After an invalidating change, **run a new trial and publish again**. A trial alone does not reactivate the published template. The new publication assigns a new execution version when connection approval changes. A connection check does not attempt writes or turn unverified database grants into verified read-only permissions.

## Parameter contract

Templates contain `tool`, `query_json`, `parameters`, `example_json`, `enabled` and optional `result_description`. `query_json` and JSON-valued configuration fields are strings so browser editing never rounds a large integer or decimal. Agent calls send actual JSON parameter values, decoded without a float64 conversion.

Each parameter has a unique `name`, `type`, `required` flag and one or more JSON Pointer `pointers`. Supported types are `string`, `integer`, `number`, `boolean`, `object`, `array` and `null`. Optional constraints are `default_json`, `enum_json` (a JSON array), `minimum` and `maximum` (decimal strings). An omitted optional parameter uses its default, or the existing fixed slot value. Required parameters with defaults use that default when omitted. Unknown parameters are rejected. Structured objects and arrays are only permitted in native database parameter slots.

| Query family | Permitted binding positions |
|---|---|
| SQL | Existing direct `/params/0` or `/named_params/name` slots supported by the driver |
| CQL | Existing direct `/params/0` slots |
| Cypher / InfluxDB | Existing direct native `/named_params/name` slots |
| Redis / Valkey | Existing string elements under `/args/N`; command and option keywords remain fixed |
| MongoDB | Scalar leaves in `/filter/...` and `$match` pipeline stages; no expression/script, collection or operator replacement |
| Elasticsearch / OpenSearch | Scalar term, match, match_phrase, range, terms-array and ids-array values beneath `/body/query/...`; no lookup index, script or query-string replacement |

Bind to **values**, not JSON keys. Query text, command names, language, namespace, object name, connection, limits and cursor are fixed outside the parameter contract. MongoDB parameter strings cannot start with `$`, preventing expression references. Search DSL structure and Redis option positions cannot be changed by parameters. Flux bucket targets, Cypher dynamic labels/relationships and ClickHouse Identifier parameters cannot select template objects dynamically. Parameter substitution never concatenates or interpolates query strings. The resulting native request passes the same read-only validation as a direct native call.

Limits: 500 entries and 768 KiB per snapshot; 128 KiB per entry; 32 KiB for the overview; 128 parameters per template and 32 pointers per parameter. Numeric contract values are limited to 1,024 characters and exponent magnitude 10,000, preventing unbounded numeric parsing. Query limits remain 30 seconds / 1,000 rows / 5 MiB by default, with the existing administrator bounds.

## MCP usage

`list_data_sources` includes `query_access_mode`, `semantics_available`, `semantic_version` and `available_tools`. It does not return the catalog.

```json
{"name":"search_semantics","arguments":{"source_id":"src_example","keyword":"revenue","kind":"metric","limit":25}}
```

Search matches names, aliases and descriptions. Kinds are `overview`, `term`, `object`, `field`, `relationship`, `metric` and `template`. `next_cursor` binds identity, source, keyword, kind, page size and publication version for five minutes. Restart after a publication change. Pages have at most 100 summaries and 64 KiB of summary data.

```json
{"name":"get_semantic_entry","arguments":{"source_id":"src_example","entry_id":"lookup-orders"}}
```

Use reserved entry ID **`overview`** to read the published source context. Template details include parameter contracts, a call example, execution version, current executability and linked metric summaries. Each entry is limited to 128 KiB; linked summaries are capped at 25. Nothing from the draft is returned.

```json
{"name":"execute_query_template","arguments":{"source_id":"src_example","template_id":"lookup-orders","execution_version":"3","parameters":{"id":42},"max_rows":100}}
```

Execution accepts only these identifiers, parameter values, an optional continuation cursor and tighter row/byte/time limits. Results retain native tabular, document, graph or time-series structure and append `semantic_version`, `template_id` and `template_version`. Authentication and template validity are checked before execution and before returning results. Native pagination cursors additionally bind template ID and execution version; a raw query cannot consume a template cursor. SQL paging remains explicit native SQL.

## Management API

All routes below require an administrator session; mutations require the session's CSRF token. The base is `/api/sources/{id}/semantics`.

| Method / suffix | Contract |
|---|---|
| GET base | Bounded draft/published snapshots, revisions, change status, trial evidence status |
| PUT base | `{revision, snapshot}` replaces the draft |
| GET `/entries` | Draft entries; `keyword`, `kind`, `offset`, `limit` (1–100), optional `revision` rejects stale pages |
| PUT `/entries/{entry}` | `{revision, entry}` saves a draft entry |
| DELETE `/entries/{entry}` | `{revision}` deletes a draft entry |
| POST `/import-structure` | `{revision, objects:[{namespace,object}]}`; 1–20 objects, preserves existing descriptions |
| GET `/export` | Versioned draft JSON; `phase=published` exports the published configuration |
| POST `/import` | `{revision, snapshot}` replaces draft configuration; no credentials or evidence |
| POST `/validate` | Validates the complete saved draft without running queries |
| POST `/trial` | `{revision, template_id}` performs a real read-only trial |
| POST `/publish` | `{revision}` atomically publishes after validation and trial checks |
| POST `/discard` | `{revision}` restores the published snapshot to the draft |
| POST `/execute` | Published template preview; execution arguments plus optional `agent_id` |

Revisions and execution versions are strings. Stale updates/publishes return HTTP 409. JSON export contains `format_version: 1`, `overview` and `entries`; it excludes source configuration, validation proof and execution approval. Import is replace-only and explicit in the UI. Data source deletion atomically removes both snapshots and validation evidence.

## Schema import boundaries

SQL/CQL and InfluxDB 1/3 import metadata field names/types. Search imports mapping properties and multi-fields. MongoDB imports collections and indexed field names with an unknown type; it does not sample documents to invent a complete schema. Redis imports selected key names and their metadata. Neo4j imports label skeletons only; property definitions are manual because the existing property discovery reads node contents. InfluxDB 2 uses the database's [schema metadata functions](https://docs.influxdata.com/influxdb/v2/query-data/flux/explore-schema/) with a 30-day scope; field types remain unspecified. No business sample values are used to infer semantic meanings, and no external model is connected.

## Audit and examples

Template queries and administrator trials use the existing sanitized audit pipeline. Executions record operation `execute_query_template`, template ID and execution version; OTLP attributes are `mcpdbhub.audit.template_id` and `mcpdbhub.audit.template_version`. Catalog text, query text, parameters, results and connection secrets are never added to audit records.

The [example catalogs](../examples/semantics/) cover PostgreSQL-style SQL, positional SQL, ClickHouse, MongoDB, Redis, Search, Cypher, CQL and all three InfluxDB generations. Adapt the query to the actual schema and native dialect, import the JSON, trial it, then publish. SQL protocol compatibility alone is not a new product verification claim. See the [actual verification record](validation.md).
