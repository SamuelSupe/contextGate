# Changelog

## 0.8.0 — 2026-09-16

- Publish existing native queries and fixed HTTP read operations through a guided source, query, contract, trial and full-source review workflow. Preserve saved drafts, concurrent edits and explicit publication boundaries.
- Link or create queries from mapped ontology concepts, show linked versus executable coverage, and expose authorized published concept/query relationships through existing MCP discovery.
- Manage available queries, drafts, queries needing attention and business definitions in one catalog. Agent visibility stays published-only; draft access is rejected server-side. Query names open the workspace directly, retaining search filters, pagination, position and focus on return.
- Guide client connection and tool invocation in three steps, with a cancellable two-minute wait for exact source/template/version/Agent call evidence. Previews and unrelated calls never confirm setup.
- Keep up to five recent query references per administrator, highlight one next action, and distinguish connection checks, health checks, query validation and business acceptance.
- Simplify English/Chinese page copy, settings, audit filters, responsive actions, parameter forms and ontology inspectors. Use contextual help, concise empty states and pagination only when needed.
- Add bilingual publishing and upgrade guides, a three-query support demo and a business pilot worksheet. No metadata migration, semantic format change, new MCP tool or query authorization model. Cloud warehouses remain unverified previews.

## 0.7.0 — 2026-09-15

- Add preview Snowflake, Databricks SQL, Google BigQuery and Amazon Redshift sources through `query_sql`, with connection forms, Configuration MCP guidance and native semantic-template examples.
- Add bounded cloud SQL execution, native parameter binding, metadata discovery, cancellation and BigQuery service-account token refresh / dry-run SELECT checks / billed-byte limits.
- Preview status remains explicit: no real cloud environment or vendor version has been verified, and the verified product/version list is unchanged. See the [cloud warehouse guide](docs/cloud-warehouses.md).

- Re-run the full open-source matrix on the 0.7.0 implementation: 18 products, 20 versions, 135 query/error cases and 104 denied operations.

## 0.6.0 — 2026-09-15

- Adopt the Apache License 2.0 for ContextGate, with project attribution and retained third-party notices in source, images and distributions.
- Named administrator accounts with Super administrator and Administrator roles, shared business resources and mandatory temporary-password replacement.
- One personal Configuration MCP identity per account, token rotation, targeted session revocation and cancellation of the owner's in-flight work.
- Administrator/configuration identity and entry-point attribution in audit and OTLP Logs; security events restricted to super administrators.
- Transactional migration from 0.5.0 preserves the existing password as `admin`, revokes legacy sessions/configuration tokens and retains query grants. Recovery supports `--username`.
- English and Chinese account-management, personal-token and audit-filter interfaces and documentation.

## 0.5.0 — 2026-09-15

- Add HTTP API sources with fixed, administrator-declared read-only GET/POST operations, scalar parameter contracts, lossless JSON results and token pagination. Reuse Agent grants, Configuration MCP, semantic templates, ontology mappings and sanitized audit/OTLP events.
- Add Home onboarding, a published business catalog with executable-query views and authorized concept details, contextual template previews and actionable health/audit navigation.
- Simplify template authoring with native query text, safe existing parameter-position suggestions and HTTP operation contracts. Keep query values lossless and optional API parameters absent when no value was configured.
- Focus ontology mapping by entity and offer metadata field selection without sample-based inference.
- Add single-answer evaluation with real-client call capture, manual review and independent history summaries; retain optional baseline/guided comparison.
- Add publication review with change impact, focused audit activity views, accurate connection/readiness evidence, credential-expiry links, settings sections, bilingual corrections and keyboard/narrow-screen fixes.
- Preserve PostgreSQL metadata and matching encryption keys when upgrading from 0.4.x; no new metadata backend or automatic upstream API permission claims.

## 0.4.0 — 2026-09-14

- Publish under the renamed `SamuelSupe/contextGate` repository and Go module, with English-first documentation and Linux arm64/amd64 `contextgate` distributions.
- Add Configuration MCP at `/mcp/config`: 22 typed tools for trusted Agents to prepare sources, semantic catalogs, verified query templates and ontology drafts/mappings. Dedicated expiring, revocable credentials are separate from query grants; publication remains an administrator action.

- Renamed the product to **ContextGate — Semantic Data Gateway for AI Agents**, with a new logo, favicon, bilingual positioning and updated client setup. New builds use `contextgate`; the `mcpdbhub` command, existing configuration keys, volumes and telemetry attributes remain compatible.

- Added administrator change auditing with durable intent/outcome correlation and OTLP export; encrypted semantic publication history, diffs and restore-to-draft; opt-in health checks and schema baselines; deterministic template regression cases; PostgreSQL diagnostics and isolated backup recovery scripts.
- Added language selection and setup-code help before sign-in. Local build identity is visible in the CLI and settings.

- **Breaking:** Replaced internal SQLite metadata storage with PostgreSQL. Set `MCPDBHUB_DATABASE_URL`; Compose includes PostgreSQL. Existing SQLite metadata is not imported; initialize a fresh store. SQLite/DuckDB remain supported read-only data sources. PostgreSQL transactions preserve session revocation, atomic semantic snapshots, encrypted recovery and audit export ordering.

- Added English and Simplified Chinese UI languages in Settings, with immediate switching and a browser-local preference. English remains the default; business content, query values and database identifiers retain their original text.
- Fixed OAuth-wide blocking from slow token, revocation and client-management request bodies, including multipart parsing and consent replay of the submitted body. Token mutations and client revision checks remain serialized.
- Rejected MySQL-family locking reads throughout the SQL syntax tree, including `FOR SHARE` in subqueries and CTEs. MySQL read-only transactions can otherwise acquire shared locks and block application writes.
- Fixed administrator password-change atomicity and stale authentication: password updates and session revocation now commit together, concurrent outdated edits are rejected, and session issuance checks the password hash that was verified.
- Added client-specific Codex/Cursor/VS Code connection guides, lossless contract-driven template parameter forms and readable result previews.
- Evaluation history now supports source-wide question/Agent/date filters, retained filter state, honest paired-review summaries and results-first evaluation details.
- Simplified workspace navigation, ontology usage, Agent actions and optional OTLP settings; improved copy feedback, empty/error recovery, schema import defaults and keyboard navigation on narrow screens.

- Completed source setup now opens a query workspace with persistent Agent context and collapsible evidence. Ontology cards show published mapping/executable template counts and direct query navigation with a named entity inspector.
- Added encrypted reusable evaluation questions and paginated history, server-generated capture metrics and configuration snapshots, resume after restart, explicit manual review saving, optimistic edit conflicts and source deletion cleanup.

- Added source-level Agent setup, executable-template readiness with publication proof, contextual connection recovery, concept-to-source/template navigation, and a real-client paired evaluation worksheet with source-scoped audit metrics and manual answer review.

- Graph-mode entity details and relationship maps now open in a contextual side panel, with related-entity navigation and a return path from editing to the panel and canvas.

- Fixed ontology editor state surviving navigation into another ontology, and protected unsaved edits from sidebar navigation, browser history and page unload. Save operations lock form inputs; conflicts preserve the edit with export and explicit reload actions.
- Improved graph selection and full-detail navigation, and made Enter save an entry without unintentionally starting another.
- Entity-centered ontology editing with contextual property/relationship forms, inherited-property navigation, readable relationship maps and count presets, generated stable IDs, continuous entry and unsaved-edit protection.
- Graphical ontology editing with movable entity cards, drag or click connections, editable relationship lines, inheritance links, zoom, automatic layout, keyboard controls and browser-local layout persistence. Graph and list modes share the same draft and publication flow.

## 0.3.0 — 2026-09-12

- Shared, encrypted business ontologies with immutable versions, single inheritance, entity/property/relation validation, explicit version adoption and reference-protected deletion.
- Independent source mappings publish atomically with semantic catalogs; schema checks distinguish verified fields from administrator declarations.
- Source-authorized ontology discovery through the existing 14 MCP tools, with filtered definitions, snapshot-bound cursors and native template results carrying ontology context.
- Definition-only changes preserve query trial evidence and in-flight execution; audit and OTLP Logs carry ontology ID/version without definitions or results.
- English ontology management and mapping UI, bilingual guides, PostgreSQL/MongoDB reuse examples, semantic JSON v2 with v1 import compatibility, and verified templates across all 18 products / 20 version combinations.

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

At 0.1.0, the service was single-instance and single-administrator. Current account behavior is documented in the [administrator guide](docs/administrators.md). SQL/Cypher query pagination is explicit; SQL metadata supports continuation. It does not provide cross-database federation, arbitrary scripts or automatic database account management. See the [support matrix](docs/support-matrix.md) and [validation record](docs/validation.md) for tested versions and limits.
