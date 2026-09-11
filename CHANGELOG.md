# Changelog

## 0.2.0 — 2026-09-11

- Independent semantic catalogs for each data source: business context, terms, objects, fields, relationships and metrics. Includes metadata skeleton import, structured English editors, multilingual business content and versioned JSON import/export.
- Native query templates across all seven query families, with typed JSON Pointer value bindings, defaults, enums, ranges and lossless numbers. Real read-only trials are required before publication; connection, credential and observed database version changes expire approval.
- Atomic draft/publication snapshots, optimistic editing, independent template execution versions and selective cancellation. Agents see only published content through `search_semantics`, `get_semantic_entry` and `execute_query_template` (14 MCP tools total).
- Optional **Templates only** access enforced across HTTP, stdio, OAuth and Agent previews. Template IDs and execution versions appear in results, audit records and OTLP Logs.
- Encrypted catalog and validation storage, bounded semantic paging, publication-bound discovery cursors and template-bound query cursors. Existing sources retain native query access after migration.

## 0.1.1 — 2026-09-11

- Optional OTLP audit Logs export over HTTP/protobuf and gRPC, configured through the English Settings page. Includes TLS/custom CA, encrypted headers, a synthetic connection test, persisted delivery progress, retry and rejection status, and cancellation on configuration changes.

## 0.1.0 — 2026-09-11

First public release of MCP DB Hub.

- Native read-only access across 18 database products and 20 independently tested product/version combinations.
- Eleven MCP tools over Streamable HTTP and an authenticated stdio bridge.
- Embedded English administration UI for data sources, Agent grants, audit records, supported databases and settings.
- Individual tokens and Fosite OAuth with PKCE, administrator consent, refresh rotation and revocation.
- Encrypted credentials, database-specific read-only protection, bounded concurrency, cancellation, lossless values and scoped continuation cursors.
- Precise Agent expiration editing, empty-grant handling, isolated query admission and paged SQL metadata discovery.
- Linux arm64 and amd64 distributions with bilingual instructions, dependency notices and SHA256 checksums.

The service is single-instance and single-administrator. SQL/Cypher query pagination is explicit; SQL metadata supports continuation. It does not provide cross-database federation, arbitrary scripts or automatic database account management. See the [support matrix](docs/support-matrix.md) and [validation record](docs/validation.md) for tested versions and limits.
