# ContextGate

![ContextGate — Semantic Data Gateway for AI Agents](docs/images/banner.svg)

[![Release](https://img.shields.io/github/v/release/SamuelSupe/contextGate?color=438c91)](https://github.com/SamuelSupe/contextGate/releases/latest)
[![CI](https://github.com/SamuelSupe/contextGate/actions/workflows/ci.yml/badge.svg)](https://github.com/SamuelSupe/contextGate/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![Databases](https://img.shields.io/badge/Databases-18_products-438c91)](docs/support-matrix.md)

**Semantic Data Gateway for AI Agents**

Understand your business. Query data safely.

ContextGate helps Agents understand business concepts and access real data through controlled, verified queries. Define business terms, metrics and shared ontologies, map them to each data source, then publish tested native query templates. Authorization, read-only enforcement, query limits and audit logs govern execution. Written in Go with an embedded administration UI; no Node.js runtime is required. Agents connect through MCP over HTTP or stdio.

**Context** is the business meaning: terms, metrics, ontologies, mappings and templates. **Gate** is the access boundary: grants, query protection, validation and auditing. Results keep their native structure; ContextGate does not store knowledge-graph instances or provide a reasoning engine.

Formerly **MCP DB Hub**. The repository and Go module are now `SamuelSupe/contextGate`; previous commits and releases remain available. The primary executable is `contextgate`, with `mcpdbhub` retained as an alias. Existing `MCPDBHUB_*` settings and telemetry attribute names remain supported. [Brand and compatibility](docs/brand/README.md).

[简体中文](README.zh-CN.md) · [Download v0.6.0](https://github.com/SamuelSupe/contextGate/releases/tag/v0.6.0) · [Documentation](docs/README.md) · [Installation](docs/install.md) · [Support matrix](docs/support-matrix.md) · [Read-only accounts](docs/read-only-accounts.md) · [Architecture](docs/architecture.md)

![Data sources — actual English administration UI](docs/screenshots/data-sources.png)

*Captured from the running administration UI with isolated databases and sample data.*

## What you get

**New in [0.6.0](docs/releases/0.6.0.md):** work with named administrator accounts, issue a personal Configuration MCP token, and trace changes to their actual operator. Two roles separate shared business administration from account and system security. ContextGate is now licensed under **Apache-2.0**.

| Capability | Behavior |
|---|---|
| Shared business ontologies | Reusable entities, properties and relationships, with explicit per-source mappings and pinned versions |
| Semantic catalogs and templates | Per-source business definitions, verified native templates and optional templates-only Agent access |
| Configuration MCP | Let trusted Agents prepare sources, semantic drafts, templates and ontologies; administrators review and publish |
| Native reads | SQL, MongoDB, Redis, Search DSL, Cypher, CQL, InfluxQL, Flux and fixed JSON HTTP API operations |
| Independent Agent access | Source grants, precise expiration, pause, rotation, revocation and OAuth |
| Enforced read-only execution | Parsers and engine classification, read-only transactions/files, safe commands and fixed query APIs |
| Bounded queries | Timeouts, cancellation, isolated concurrency, response limits and identity-bound cursors |
| Lossless results | Large integers, decimals, binary data and native document, graph and time-series structures |
| Audit log export | Optional OTLP Logs over HTTP/protobuf or gRPC, encrypted headers, durable progress and delivery status |
| Named administrators | Super administrator/Administrator roles, personal Configuration MCP identities, operator audit and targeted credential revocation |
| Built-in administration | Embedded bilingual UI, encrypted credentials, schema preview and named-account password recovery |

## From a business question to a verified query

1. **Connect** a database with its own read-only account and inspect connection evidence.
2. **Describe** its business vocabulary, metrics and fields; optionally map a shared ontology to the source.
3. **Verify and publish** native query templates with real example parameters.
4. **Grant and connect** an Agent, then discover published concepts and execute templates through MCP.
5. **Observe** query and configuration activity in the audit log, with optional OTLP Logs export.

A separate **[Configuration MCP](docs/configuration-mcp.md)** endpoint lets a trusted Agent prepare the first three steps. Create its dedicated, short-lived token in **Settings → My configuration MCP**. Source edits take effect immediately; semantic/ontology publication and query grants remain administrator actions.

<details>
<summary><strong>Explore the UI: ontologies, semantics and Agent grants</strong></summary>

![Graphical ontology editor](docs/screenshots/ontology.png)

![Semantic catalog and verified templates](docs/screenshots/semantics.png)

![Per-Agent data source grants](docs/screenshots/agents.png)

</details>

**HTTP API data sources**, available since 0.5.0, extend the same workflow to fixed GET and administrator-declared read-only POST JSON operations. See the [HTTP API guide](docs/http-api.md) for configuration, authentication, pagination and semantic/ontology examples.

## Operate and recover

**0.4.0** includes management change records, encrypted semantic publication history with restore-to-draft, optional scheduled health checks, template regression cases, and PostgreSQL diagnostics and backup recovery verification. Start with **Health**, **Semantics → Publication history**, and **Settings → Deployment diagnostics**. See the [operations guide](docs/operations.md) for their scope and recovery steps.

## Download

[**ContextGate 0.6.0**](https://github.com/SamuelSupe/contextGate/releases/tag/v0.6.0) provides Linux **arm64 / amd64** archives with the embedded UI, private C++ libraries, bilingual documentation, examples, dependency notices and checksums. The packages require glibc ≥ 2.36 and a PostgreSQL metadata database. Follow the [installation guide](docs/install.md) to download, verify and start the correct archive.

## Run

```sh
git clone https://github.com/SamuelSupe/contextGate.git
cd contextGate
mkdir -p databases
umask 077
printf 'MCPDBHUB_POSTGRES_PASSWORD=%s\n' "$(openssl rand -hex 24)" > .env
docker compose up --build -d
docker compose logs hub
```

Open `http://127.0.0.1:8080`. Use the one-time setup code printed in the logs to create the first super administrator username and password. Add a data source, test its connection and protection evidence, then create an agent and select its data sources. Agent tokens are displayed once.

Compose starts PostgreSQL on its private container network and waits for it to become healthy. Only ContextGate HTTP port is published to loopback. ContextGate runs as UID 10001 and mounts query database files read-only. Metadata persists in `hub-postgres`; the independent encryption key persists in `hub-data`. Preserve both volumes and the existing `.env` when restarting or upgrading; generate `.env` only for a new installation.

**Storage change:** current source uses PostgreSQL exclusively for internal configuration, sessions, grants, catalogs and audit logs. Upgrades from 0.4.x preserve the existing PostgreSQL store and its matching master key. It does not import old SQLite metadata; upgrades from 0.3.x or earlier require a fresh administrator and source configuration. SQLite remains a read-only query data source. Versions through 0.3.0 use SQLite metadata. Read the [0.4.0 upgrade notes](docs/releases/0.4.0.md#upgrading-from-03x-or-earlier) before switching.

To build locally, install Go 1.26, a C/C++ toolchain, and Node.js 24, then run `make build`. CGO is required by SQLite and DuckDB. Build natively for Linux arm64 or amd64; `CGO_ENABLED=0` is unsupported.

```sh
export MCPDBHUB_DATABASE_URL='postgres://mcpdbhub:REPLACE_ME@127.0.0.1:5432/mcpdbhub?sslmode=verify-full'
./bin/contextgate serve --data-dir ./data --database-dir ./databases
```

Create the PostgreSQL database beforehand and use a dedicated owner role with schema/table creation and read/write permissions. URL-encode special characters in connection credentials. For a local isolated database without TLS use `sslmode=disable`; remote deployments should verify the server certificate. `--database-url` is also accepted, but an environment variable avoids putting the connection secret in command arguments. `--data-dir` now holds the encryption key, not metadata.

For a corporate build proxy, pass its CA with `docker build --secret id=build_ca,src=/path/to/ca.pem -t contextgate:local .`. It is used only while downloading dependencies and is excluded from the runtime image. Database TLS certificates are configured separately in the UI.

## Connect an agent

![Verified template query with real sample data](docs/screenshots/query-preview.png)

Streamable HTTP and the stdio bridge share authorization, execution limits, and auditing. Fourteen tools expose four discovery operations, seven native query families, and three semantic catalog/template operations.

```json
{"mcpServers":{"contextgate":{"url":"http://127.0.0.1:8080/mcp","headers":{"Authorization":"Bearer <AGENT_TOKEN>"}}}}
```

For a stdio client:

```json
{"mcpServers":{"contextgate":{"command":"/absolute/path/contextgate","args":["stdio","--url","http://127.0.0.1:8080/mcp"],"env":{"MCPDBHUB_TOKEN":"<AGENT_TOKEN>"}}}}
```

The outer configuration format varies by client. Remote bridge URLs require HTTPS. The bridge writes only MCP protocol messages to stdout.

Start with `list_data_sources`; it returns only authorized sources, with capabilities, tool names, examples and limits. Use `list_namespaces`, `list_objects` and `describe_object` for discovery. Native query tools are `query_sql`, `query_mongodb`, `query_redis`, `query_search`, `query_cypher`, `query_cql`, `query_influxdb` and `query_http_api`. Their input schemas define family-specific arguments. Agents cannot supply connection addresses, credentials or HTTP paths.

```json
{"name":"query_sql","arguments":{"source_id":"src_...","query":"WITH totals AS (SELECT region,sum(amount) AS total FROM orders WHERE created_at >= $1 GROUP BY region) SELECT region,total,rank() OVER (ORDER BY total DESC) FROM totals","params":["2026-09-01"],"max_rows":100,"timeout_seconds":10}}
```

## Database capabilities and results

Adapters cover PostgreSQL, MySQL, MariaDB, TiDB, CockroachDB, TimescaleDB, SQLite, DuckDB, ClickHouse, MongoDB, Redis, Valkey, Elasticsearch, OpenSearch, Neo4j, Cassandra, ScyllaDB, and InfluxDB. InfluxDB 1.x, 2.x and 3 Core are tested separately. **The support matrix identifies actual verified versions; protocol compatibility alone is not a support claim.**

SQL preserves joins, subqueries, read-only CTEs, aggregation and window functions. MongoDB exposes find, aggregation, count and distinct. Redis exposes bounded reads for common structures. Search retains native DSL and aggregations. Graph results retain nodes, relationships and paths. CQL provides native page-state pagination.

Results contain `format`, `data`, available native type information, `row_count`, `elapsed_ms`, `truncated`, `bytes`, and optionally `next_cursor`. MCP provides both structured content and a JSON text fallback. Int64/uint64 and exact decimals use strings; binary values carry `encoding: base64`; timestamps retain available precision. MongoDB uses canonical Extended JSON. Flux preserves column/type information per table. Some native APIs do not report computed-column types; missing type information must not be interpreted as a string type.

Default limits are 30 seconds, 1,000 rows and 5 MiB. Administrators may configure up to 120 seconds, 10,000 rows and 20 MiB. Agents can only tighten limits through `timeout_seconds`, `max_rows` and `max_bytes`. The byte budget reserves room for both MCP result representations, so actual data may be smaller than the configured maximum. Oversized individual values or native responses can produce an explicit size error instead of a partial result.

Cursors expire in five minutes and bind the caller, source revision, operation, complete query and arguments. Reuse identical arguments and limits, changing only `cursor`. SQL and Cypher query pagination is explicit in the query. MongoDB retains native cursors for up to five minutes, with at most 64 per source and single-use continuation handles. Search requires a stable sort for `search_after` and does not provide a snapshot across pages. Redis COUNT is a hint; when a returned batch exceeds limits, the result is marked truncated and an unsafe continuation cursor is withheld.

SQL namespace, table and column discovery returns `next_cursor` when more metadata is available. The UI provides **Next page**. Pages respect row and response size limits, including databases with more than 1,000 tables. Metadata uses ordered offsets without a snapshot across pages; restart discovery if the schema changes.

## Authorization and protection

The grant model is Agent → data source. Database users, views and native permissions control database, table, field and row visibility. Configure multiple sources with different database accounts when different visibility is required.

Execution combines statement parsing or engine classification, read-only transactions/files, safe function and command lists, and fixed read APIs. It rejects multiple statements, modifying CTEs, DDL/DML, locking reads, imports/exports, extensions and dangerous scripting. Arbitrary custom functions are outside the supported subset. Read-only access does not imply acceptance of every syntactically valid native query.

Connectivity and privilege evidence are shown separately. “Account permissions unverified” means database-side grants require independent confirmation. InfluxDB 3 Core is explicitly labeled “query API isolation”: its admin token retains database administration privileges; only the adapter's fixed query APIs are available to agents.

Configuration is stored in PostgreSQL. Source credentials use AES-256-GCM; the master key is held in a separate `master.key` file or the `MCPDBHUB_MASTER_KEY` environment variable (standard Base64 of 32 bytes). Administrator passwords use Argon2id; agent and session tokens are hashed. Admin sessions use HttpOnly/SameSite cookies and CSRF checks. Audit records expire after 30 days and exclude results, plaintext parameters and full query text. Grant changes cancel affected work.


## Configuration MCP

Use **Settings → My configuration MCP** to connect a trusted configuration Agent. A dedicated, expiring token exposes tools for all data source connections, semantic drafts, template trials and shared ontologies/mappings. Source edits are immediate; administrators publish drafts and grant query access. See the [configuration guide](docs/configuration-mcp.md).

## Semantic catalogs and query templates

![Published semantic query templates — actual administration UI](docs/screenshots/semantics.png)

Version 0.2.0 adds independent business catalogs and verified native query templates to every data source. Use **Data sources → Semantics** to import schema skeletons, define terms/fields/relationships/metrics, trial templates against the real database, and publish an Agent-visible snapshot. **Templates only** mode enforces curated query access across HTTP, stdio, OAuth and Agent previews.

`search_semantics`, `get_semantic_entry` and `execute_query_template` expose published content with bounded pagination, typed JSON Pointer bindings and execution versions. Connection, credential or observed database version changes require a new trial and publication. Template audit metadata also flows to OTLP Logs. See the [complete guide](docs/semantics.md) and [examples for all query families](examples/semantics/).

## Shared business ontologies

![Shared business ontology with source and template mappings](docs/screenshots/ontology.png)

Version 0.3.0 adds **Ontologies** and **Semantics → Ontology mapping**. Define Customer, Order, their properties and relationships once; map PostgreSQL tables and MongoDB documents independently to an explicitly selected, immutable ontology version. Agents discover only concepts mapped to their authorized source and receive native template results with `ontology_context`.

Mappings publish atomically with the source catalog. Definition-only upgrades preserve template trial evidence and running queries; existing sources never follow the latest ontology automatically. Inheritance, identity and cardinality are declarative, without fact inference, federated queries or result conversion. See the [ontology guide](docs/ontologies.md), [PostgreSQL/MongoDB examples](examples/ontologies/) and [real verification record](docs/verification/ontology.json).

## OTLP audit logs

Since v0.1.1, **Settings → Audit log export** supports OpenTelemetry Collector and compatible OTLP Logs receivers. Configure HTTP/protobuf or gRPC, authentication headers and optional CA certificates; send a test log and monitor delivery. Export reads the persisted sanitized PostgreSQL audit trail asynchronously, with persisted progress and retries. Full queries, parameters, results and credentials are excluded. See [configuration, Collector example and delivery guarantees](docs/audit-export.md).

## OAuth

Ory Fosite implements authorization code grants with mandatory PKCE S256, explicit administrator consent, rotating refresh tokens and revocation. Access tokens last 15 minutes and refresh grants 30 days. OAuth shares the same agent data-source grants as independent tokens. Each consent creates a distinct grant.

Discovery is available at `/.well-known/oauth-protected-resource` and `/.well-known/oauth-authorization-server`. Both authorization and token requests must pass the exact public MCP URL in `resource`, such as `https://db.example.com/mcp`.

Clients can be pre-registered through the UI, use bounded dynamic registration at `/oauth/register`, or supply an HTTPS Client ID Metadata Document. Metadata fetching rejects private addresses, redirects and oversized responses. Redirects require HTTPS, HTTP on a loopback IP, or a reverse-domain native application scheme. Redirect URIs match exactly. Revoke credentials at `/oauth/revoke`.

**Since 0.6.0:** named administrator accounts, Super administrator/Administrator roles, personal Configuration MCP identities and attributable audit. Upgrading retains the old password under username `admin`, requires signing in again and revokes old configuration MCP tokens. [Upgrade and account guide](docs/administrators.md).

## Deployment and maintenance

| Variable | Default | Purpose |
|---|---|---|
| `MCPDBHUB_LISTEN` | `127.0.0.1:8080` | Listen address; `0.0.0.0:8080` inside the image |
| `MCPDBHUB_PUBLIC_URL` | `http://127.0.0.1:8080` | Canonical public origin; remote origins require HTTPS |
| `MCPDBHUB_DATABASE_URL` | Required | PostgreSQL metadata connection; used by serve and reset-password |
| `MCPDBHUB_DATA_DIR` | `./data` | Local encryption key directory |
| `MCPDBHUB_DATABASE_DIR` | `databases` inside data directory | Allowed SQLite/DuckDB directory |
| `MCPDBHUB_MASTER_KEY` | Separate key file | Optional externally supplied master key |

Use an HTTPS reverse proxy for remote access, preserving the public Host and Authorization headers. Set the exact public URL. The admin UI and OAuth server share an origin. Health checks use `GET /healthz`.

For a consistent backup, stop ContextGate and use `pg_dump` to back up its PostgreSQL database. Back up the matching `master.key` or external master key separately. Restore the database with `pg_restore` and supply the same key; a missing or mismatched key prevents startup. ContextGate remains single-instance, with multiple administrator accounts and two roles sharing the business configuration. It does not introduce multi-instance coordination, resource tenancy, federation or automatic database privilege changes. See [administrator accounts and the upgrade checklist](docs/administrators.md).

See [validation](docs/validation.md) for reproducible OrbStack integration tests and native Chrome UI evidence. Fixture setup writes only to dedicated disposable databases. The service's connection probe never attempts a write.

## Administrator workflows

Use **Agent setup** from a data source to review connection evidence, executable templates, grants and real client query activity. Completed setup becomes a **Query workspace** with the selected Agent visible before previewing. Ontology cards link directly to **Queries and sources**, with published mapping and executable template counts. Save reusable business questions and review paired-run metrics in persistent, encrypted evaluation history. See the [workflow and evaluation guide](docs/agent-workflows.md).

![Per-Agent data source grants](docs/screenshots/agents.png)

The administrator UI defaults to English. In **Settings → Language**, choose **English** or **简体中文**; the change takes effect immediately and is remembered in this browser. Business definitions, query text, parameters and results retain their original content.

In **Data sources**, select an explicit authentication method. A blank credential keeps the value stored for that method; **Clear the stored credential** removes it. Switching methods removes credentials for the previous method. Connection status shows the last completed check and its timestamp; it is not a live health monitor. A saved configuration whose connection check fails stays open with the failure reason.

In **Agents**, **Pause / Resume** temporarily suspends access while retaining the token. **Revoke** permanently retires the credential and cancels in-flight queries. **Rotate token / Issue new token** replaces it while retaining Agent identity, grants and audit history. Save the new token and update your client; the previous token never becomes valid again. On the first upgrade, previously disabled Agents (shown as revoked in older versions) are permanently revoked. Active credentials are preserved.

**Connect** remains available after creation, with HTTP/stdio connection values and recorded client activity. Activity covers the last 30 days and excludes administrator previews. **Explore data** supports clickable structure discovery, table/JSON results, native next-page cursors and cancellation. Select an Agent to check its data source grants; this is an administrator preview, not proof that a client has connected. Numeric query parameters are sent without JavaScript numeric conversion.

**Audit log** filters by caller, data source, result, date range and request ID. Query failures and connection checks expose safe error categories and native codes when available. Request IDs correlate with audit details. Credentials, raw database error text, query text, parameters and results are never stored in the audit log.

### Configuration changes and OAuth client management

Agent edits include a revision. A stale save returns HTTP 409 instead of restoring older grants or re-enabling a paused Agent. Close and reload the current grants before editing again. Deleting a data source removes its grants in the same transaction; upgrades also clean up references to previously deleted sources. Renaming a data source preserves running queries, its connection and pagination cursors. Connection, access-state and limit changes invalidate affected execution as before.

Expiration is editable to the second in the browser's local time zone. Leaving the field unchanged preserves the original expiration timestamp in full. Omitted or `null` API `sources` values normalize to `[]`, granting no data source access.

**Agents → OAuth clients** lists registered clients and supports search, name/redirect editing, disabling/enabling, secret rotation and deletion. Redirect changes, disabling, secret rotation and deletion revoke the client's access/refresh tokens, pending authorizations and associated Agent credentials, and cancel running queries. Enabling permits new consent; it does not restore revoked tokens. Changing only the client name preserves credentials. Public PKCE clients have no secret. Metadata document clients obtain their name and redirects from the document; disabling cannot be overridden by a metadata refresh. Deletion releases a registration slot, but a client may register again and request new consent. Use Disable when a metadata document client must stay blocked. Client secrets are shown once and are never included in client listings.

InfluxDB discovery and the query preview use the configured 1.x, 2.x or 3 Core version. Version 2 examples use Flux with the configured bucket; unsupported languages produce a local `invalid_query` diagnostic.

### Recover a forgotten administrator password

Recovery requires local server access, the same `MCPDBHUB_DATABASE_URL`, the existing key directory and its matching master key. Stop the service first. The command reads the new password from standard input (12–256 bytes), never from a command argument. It preserves data sources, Agent credentials and audit history, and revokes only the selected account’s sessions and configuration MCP token. Use `--username`; omission is allowed only with one account. The role and other administrators remain unchanged.

For Docker Compose, run these commands in Bash or Zsh. The password prompt does not echo input or place the password in shell history:

```sh
docker compose stop hub
read -r -s hub_new_password
printf '%s' "$hub_new_password" | docker compose run --rm -T hub reset-password --username admin --password-stdin
unset hub_new_password
docker compose up -d hub
```

Recovery must use the same `MCPDBHUB_DATABASE_URL` as the running service, in addition to the existing master key. For a standalone binary, pipe the password to `contextgate reset-password --data-dir /path/to/existing-data --username admin --password-stdin`, then restart the service. If you configured `MCPDBHUB_MASTER_KEY`, supply the same environment configuration to the recovery command. Do not delete the configuration database or master key to recover a password.

## Get involved

[Report an issue](https://github.com/SamuelSupe/contextGate/issues/new/choose) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md) · [Third-party notices](THIRD_PARTY_NOTICES.md)

ContextGate is licensed under the [Apache License, Version 2.0](LICENSE). See [NOTICE](NOTICE) for attribution and [third-party notices](THIRD_PARTY_NOTICES.md) for dependencies, which retain their respective licenses.
