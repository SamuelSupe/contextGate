# Support matrix

[简体中文](support-matrix.zh-CN.md)

The supported database versions cover **18 products and 20 product/version combinations**, counting InfluxDB 1.x, 2.x and 3 Core separately. The latest full matrix ran in isolated OrbStack Linux arm64 instances on **2026-09-15**, during 0.6.0 administrator feature verification: 135 query/error cases and 104 denied operations. It covers connectivity, discovery, parameters/types, empty/error results, rejected operations, unchanged fixture data, limits, cancellation and native/template equivalence. The [feature record](verification/administrators.json) preserves its exact digest. This records the 0.6.0 release baseline, not the later preview adapters. Final release checks and reproduction instructions are in [validation](validation.md).

**HTTP API sources** are an additional connector, not a nineteenth database product. Fixed GET/POST JSON operations were verified against isolated fixtures; read-only behavior of arbitrary upstream APIs is not certified. See [HTTP API capabilities and limits](http-api.md).

Verification covers the versions and capabilities below. It does not establish compatibility with other versions, distributions, authorization plugins or cluster topologies.

| Product | Tested version | Native reads and parameters | Pagination | Protection and account evidence |
|---|---|---|---|---|
| PostgreSQL | 17.11 | SQL, CTEs, windows, aggregation; `$1` | Explicit SQL | AST validation and a read-only transaction per query; `engine_enforced` |
| MySQL | 8.4.11 | SQL, CTEs, windows, aggregation; `?` | Explicit SQL | AST validation and a read-only transaction per query; `engine_enforced` |
| MariaDB | 10.11.18 | SQL, CTEs, windows, aggregation; `?` | Explicit SQL | AST validation and a read-only transaction per query; `engine_enforced` |
| TiDB | 8.5.1 | Native SQL; `?` | Explicit SQL | AST validation; SHOW GRANTS must contain only SELECT/SHOW VIEW/USAGE; `verified` |
| CockroachDB | 24.3.15 | Native SQL; `$1` | Explicit SQL | PostgreSQL AST and read-only transactions; `engine_enforced` |
| TimescaleDB | 2.25.2 / PostgreSQL 17.7 | Native SQL, hypertables, time_bucket; `$1` | Explicit SQL | Independent extension-version discovery and read-only transactions; `engine_enforced` |
| SQLite | 3.53.4 | SQL, CTEs, windows; `?` | Explicit SQL | mode=ro, query_only and authorizer; `engine_enforced` |
| DuckDB | 1.5.5 | SQL, CTEs, windows; `?` | Explicit SQL | READ_ONLY, engine StatementType, external access and extensions disabled; `engine_enforced` |
| ClickHouse | 24.8.14.39 | Analytical SQL; `{name:Type}` and named_params | Explicit SQL | Parser, readonly=1, allow_ddl=0; `engine_enforced` |
| MongoDB | 7.0.39 | find/filter/projection/sort, aggregate, count, distinct; Extended JSON | Native cursors, five minutes, up to 64 per source | Fixed read operations, recursive stage/script checks; dedicated read role |
| Redis | 7.4.6 | String/Hash/List/Set/ZSet/Stream; separate string argument array | SCAN family | Read-command allowlist and dedicated ACL; complete effective privileges unverified |
| Valkey | 8.1.9 | Same restricted read families as Redis, independently tested | SCAN family | Read-command allowlist and dedicated ACL; complete effective privileges unverified |
| Elasticsearch | 8.19.17 | Search DSL, aggregation, get, count, mapping; JSON arguments | search_after | Fixed read paths, no scripts; HTTPS and dedicated reader role; native writes returned 403 |
| OpenSearch | 3.2.0 | Search DSL, aggregation, get, count, mapping; JSON arguments | search_after | Fixed read paths, no scripts; HTTPS and dedicated reader role; native writes returned 403 |
| Neo4j | Community 5.26.10 | MATCH, aggregation, nodes/relationships/paths; `$name` | Explicit Cypher | Read-only EXPLAIN classification and complete statement checks; account privileges unverified |
| Cassandra | 5.0.8 | SELECT, TTL, writetime; `?` | Native PageState | SELECT syntax restrictions and dedicated SELECT role; complete privileges unverified |
| ScyllaDB | 2026.2.5 | SELECT, TTL, writetime; `?`; independent product-version discovery | Native PageState | SELECT syntax restrictions and dedicated role; metadata read permission on system.versions |
| InfluxDB 1.x | 1.8.10 | InfluxQL, aggregation; `$name` condition parameters | Explicit query | SELECT only and fixed /query path; dedicated READ user |
| InfluxDB 2.x | 2.7.12 | Read-only Flux, aggregation; `params.name` literal AST | Explicit query | Imports/network/interpolation/writes denied; bucket-read token |
| InfluxDB 3 Core | 3.11.2 | Fixed SQL and InfluxQL APIs; `$name` | Explicit query | **Query API isolation**; the administrator token itself retains administration privileges |


## Unreleased cloud previews

| Connector | Interface / binding | Real vendor verification |
|---|---|---|
| Snowflake | SQL API; `?` and positional params | Not verified |
| Databricks SQL | Statement Execution API; `:name` and named_params | Not verified |
| Google BigQuery | Jobs API, dry-run SELECT; `@name` and named_params | Not verified |
| Amazon Redshift | PostgreSQL protocol, read-only transactions; `$1` and positional params | Not verified |

These four previews are implemented but excluded from the verified counts and existing v0.6.0 downloads. Cloud REST fixtures validate request/response handling, not vendor compatibility. Discovery, auth methods, conservative SQL subsets, billing limits and manual cloud contract versions are documented in the [cloud warehouse guide](cloud-warehouses.md).

## Read boundaries

- SQL preserves joins, subqueries, read-only CTEs, aggregation and windows without rewriting everything into a common dialect. Syntax outside the parser or function allowlist is explicitly rejected. Custom functions/procedures, external table functions, locks and import/export are outside the supported subset.
- Explicit SQL query pagination does not restrict metadata discovery. SQL namespace, table and column lists use ordered offset cursors and enforce per-page row/byte limits. Restart discovery when the schema changes; pages do not share a snapshot.
- SQLite/DuckDB query only existing database files within the administrator-configured directory. Queries cannot select other CSV, Parquet, URL, attached-database or extension resources.
- MongoDB distinct uses bounded aggregation, deduplicates array elements and excludes missing fields. Scripts, writing stages and disk spill are disabled. Continuation handles are single-use; expiration or a server restart requires a new query. A cross-page snapshot is not guaranteed while data changes.
- Redis/Valkey do not expose KEYS, scripts, blocking reads or administration commands. SCAN COUNT is a hint: oversized batches are explicitly truncated without returning a cursor that would skip results.
- Search preserves `size:0` aggregation and smaller requested sizes; limits only tighten the request. More hits without sorting produce a truncated result; a stable sort enables continuation. Deep pagination requires an explicit stable sort and has no PIT/scroll snapshot. Aggregations retain their native structure; oversized native responses produce a size error.
- Neo4j does not expose APOC, custom procedures or LOAD CSV. Driver read routing is not proof of database account privileges.
- CQL accepts only a complete SELECT token stream and leaves actual syntax parsing to the database. UDFs and batches are unavailable. Native PageState is not exposed directly to agents.
- Flux does not expose import/package/option or network access. Supported built-ins are defined by `guardFlux`. OSS 2.7.12 parameters use an extern literal AST; tests confirmed that interpolation text in parameter values does not execute. Integers outside int64 are rejected to preserve precision.
- Every family has timeouts and row/byte limits. A native response that cannot be safely truncated by row returns an explicit error. APIs that omit exact native type information do not receive invented types.

## Interpreting evidence

`engine_enforced` confirms the observed engine/file read-only mechanism; it does not mean every account privilege was enumerated. `verified` is reserved for sufficient observed privilege evidence. Other connections retain an account-permissions-unverified status; InfluxDB 3 Core uses `api_isolated` separately.

The matrix covers core reads on single-node/single-replica fixtures. It does not cover every type/operator combination, distributed failover, long-running load tests, production networks or all TLS/account-plugin combinations. Shared HTTP TLS handling also has real certificate-trust tests. Single-node Elasticsearch/OpenSearch HTTPS, security plugins and reader roles were tested; other products' TLS cluster deployments require deployment-specific validation.
