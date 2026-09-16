# Cloud warehouses (preview)

[简体中文](cloud-warehouses.zh-CN.md) · [Support matrix](support-matrix.md) · [Configuration examples](../examples/cloud-warehouses/README.md)

**Available since ContextGate 0.7.0:** Snowflake, Databricks SQL, Google BigQuery and Amazon Redshift adapters are implemented, with no real vendor environment verified. They are displayed as **preview**, with no tested versions in the support catalog. They are included in current downloads, but not in the 18-product / 20-version verified matrix. Mock contract tests do not establish cloud compatibility.

## Connection setup

Create a data source in **Data sources → Add data source**, or use the existing Configuration MCP `create_data_source` tool. All four use `query_sql`, the same Agent → Data Source grants, semantic catalogs, query templates, ontology mappings, previews, cancellation and sanitized audit/OTLP pipeline. No new MCP endpoint or query tool is required.

| Kind | Connection and credentials | Native parameters | Discovery |
|---|---|---|---|
| `snowflake` | Account host, HTTPS 443, database; `options.warehouse`, `options.role`, optional `options.schema`. `auth_mode: "token"`; store a PAT or OAuth access token in `token`. `options.token_type`: `PROGRAMMATIC_ACCESS_TOKEN` or `OAUTH`. | `?` with `params` | Current database's information_schema schemas, tables and columns |
| `databricks` | Workspace host, HTTPS 443; `database` is the Unity Catalog catalog. `options.warehouse_id`, optional `options.schema`. `auth_mode: "token"`, PAT or OAuth access token in `token`. | `:name` with `named_params` | Current catalog's information_schema schemas, tables and columns; a catalog exposing these views is required |
| `bigquery` | Fixed `bigquery.googleapis.com`, HTTPS 443; `database` is the billing project ID. Required `options.location`, optional `options.schema` default dataset. Google service account JSON in `password` with `auth_mode: "service_account"`, or access token in `token` with `auth_mode: "token"`. | `@name` with `named_params` | Datasets, tables and nested schema field paths within the configured project |
| `redshift` | Cluster or Serverless workgroup host, database, TCP port (default 5439), username/password. Use verified TLS. | `$1` with `params` | Native SVV_REDSHIFT views, scoped to the connected database; external catalog discovery is outside this preview |

For the three HTTPS SQL APIs, certificate verification is mandatory; custom CA certificates are supported. Redirects, arbitrary HTTP paths and external result URLs are not followed. Only administrators configure the host; query Agents provide a source ID. BigQuery's API and OAuth token hosts are fixed. No service account credential file path, custom token endpoint, application-default credential discovery or workload identity is used. Private key JSON must be a Google service account with an RSA PKCS8 key of at least 2048 bits.

Google service account access tokens are obtained and cached on demand, then refreshed when expired. Other access tokens must be replaced by the administrator before expiry. Blank credential updates preserve the existing secret for the selected method; switching methods clears the old method's secret. No Snowflake key-pair login, Databricks OAuth client-credential exchange, Redshift IAM credential generation or Redshift Data API is implemented.

## Read-only and cost boundaries

Use dedicated database/cloud principals with only the required data-read permissions. For Snowflake grant warehouse/database/schema usage and SELECT on the intended tables or views; select that reader role in the source. For Databricks grant warehouse use, catalog/schema use and SELECT only. BigQuery needs query-job creation in the billing project and data/metadata read permissions on the intended datasets. Redshift needs database/schema usage and SELECT for the intended objects. Scope inherited roles, functions, external tables and cloud integrations separately in the provider. ContextGate does not create these accounts or modify grants.

The three HTTP connectors require `options.read_only_confirmed: "true"`. This is an administrator declaration, **not permission verification**. A successful SELECT probe still returns `permission_status: "unverified"`. Redshift opens a read-only transaction for every query and sets a transaction-local statement timeout; its probe reports acceptance of `BEGIN READ ONLY`, separately from unverified account grants. No connector tests permissions by attempting writes.

Cloud SQL accepts a conservative SELECT subset: joins, subqueries, read-only CTEs, aggregation and windows accepted by the existing PostgreSQL syntax/function checks after normalizing native placeholders and identifier quotes **for validation only**. Original query text and separate parameters go to the vendor. Writes, scripts, modifying CTEs, locks, import/export, dangerous/external functions and qualified cloud functions are rejected. The HTTPS connectors also reject block comments, hints, trailing semicolons, ambiguous escaped literals and session variables. Use single-quoted literals; Snowflake double-quoted identifiers; Databricks/BigQuery backtick identifiers. Native dialect features outside this subset (for example QUALIFY, UDFs, external table functions or advanced GoogleSQL type syntax) may be rejected. Prefer SQL CAST to native type syntax where appropriate. This is not full dialect coverage.

BigQuery additionally performs a dry run and requires `statistics.query.statementType` to be `SELECT` before submitting an execution job. `options.maximum_bytes_billed` is a positive decimal int64 string, default `1073741824` (1 GiB). The same limit is sent on dry-run and execution requests. This limits billed scan bytes, not returned rows or all possible cloud charges. Statement timeouts and cancellation are best effort at the provider: requests to cancel known jobs/statements are bounded, but connection loss can prevent confirmation. BigQuery job IDs are allocated before submission to permit cancellation after an ambiguous response. Snowflake/Databricks cannot cancel a statement whose handle never reached the gateway; configure provider-side timeout/resource policies too.

## Parameters, results and pagination

Scalar parameters preserve JSON numbers without a floating-point round trip. For explicit native types use a value slot containing `{"type":"TYPE","value":"exact text"}`. Only known scalar type names are allowed; arrays/structs as native parameters are not implemented in this preview. Untyped null defaults to the provider's string type; use an explicit typed null when needed.

| Connector | Inferred types | Explicit examples |
|---|---|---|
| Snowflake | TEXT, BOOLEAN, FIXED | `{"type":"FIXED","value":"12345678901234567890.123456789"}` |
| Databricks | STRING, BOOLEAN, BIGINT; decimals require an explicit type | `{"type":"DECIMAL(38,9)","value":"12345678901234567890.123456789"}` |
| BigQuery | STRING, BOOL, INT64, NUMERIC | `{"type":"BIGNUMERIC","value":"123456789012345678901234567890.123456789"}` |
| Redshift | Existing PostgreSQL scalar binding | `params: [9007199254740993]`, SQL `CAST($1 AS BIGINT)` |

Exact numeric values remain strings in results, accompanied by native column types. The HTTP APIs' timestamp, date and time text is retained without precision-losing conversion; this can be epoch-based text (BigQuery TIMESTAMP is normally Unix seconds), not RFC3339. Snowflake and Databricks semi-structured and binary values retain their API string representation and native column types; binary text is not automatically relabeled as base64. BigQuery repeated/record values are decoded to arrays/objects, booleans to booleans, and BYTES to base64 objects. Redshift uses the existing PostgreSQL wire decoding; vendor-specific types such as SUPER are not separately certified.

Provider partitions/chunks/pages are collected within one request's limits. SQL query pagination remains **explicit in SQL**; truncated queries do not issue continuation cursors that would skip unread rows. Oversized native pages can return a size error even when a smaller row count was requested. Use selective queries and deterministic keyset pagination for large results. Metadata discovery has bounded continuation cursors, wrapped by the shared execution layer and bound to identity and source revision; it is not a schema snapshot across pages.

## Semantics and ontology workflow

Import discovered structure into a semantic draft, add business descriptions and native templates, trial enabled templates, then publish and grant Agent access. The [four examples](../examples/cloud-warehouses/README.md) show source configuration and importable v2 semantic snapshots. Templates bind only native parameter slots (`/params/0` or `/named_params/name`); never bind SQL text or object names. A typed pair can occupy a native slot using an object parameter contract; its type is still checked by the adapter. Ontology bindings remain source-scoped and do not rewrite SQL or convert results.

For HTTPS cloud sources, `version` is an administrator-maintained **connection contract version**, initially `"1"`. Cloud engine upgrades are not automatically detected. Change it after relevant provider upgrades or contract/privilege changes, then trial and publish affected templates again. Connection/credential edits already expire trial evidence. Redshift uses its reported engine version through existing version observation. Trial evidence proves the executed example under that configuration; it does not establish every parameter value or vendor-version combination.

## Validation and references

Local checks on 2026-09-15 used OrbStack Linux arm64: `go test -race ./...`, `go vet ./...`, frontend tests and the production UI/Go build. Local Chrome checked all four source forms, BigQuery credential-method switching and invalid-credential errors, preview labels, English/Chinese UI and a 390px layout. The original 18-product / 20-version database matrix was subsequently rerun for the 0.7.0 release and passed 135 query/error cases and 104 denied operations. Its [current report](verification/matrix.json) covers the open-source products, not these cloud previews.

The local automated fixtures cover SQL rejection before HTTP submission, exact parameters, template binding, asynchronous polling, chunk/page collection, empty results, dry-run rejection, cancellation, credential redaction and service account token caching. Configuration tests exercise encrypted source persistence and credential edits through Configuration MCP and the UI API. These tests simulate the documented API responses; real authentication, account grants, engine behavior, private networking, billing and production compatibility remain unverified. Redshift's SQL guard is tested locally; native Redshift execution/discovery awaits an AWS environment.

Before promoting any connector to supported, run connection/discovery, complex SELECTs, parameters/types, empty/error cases, denied operations with unchanged fixture data, truncation/cancellation, and native/template equivalence in its real environment. Record provider engine/runtime and deployment details separately; do not update `verified.json` from mock results.

Implementation contracts: [Snowflake SQL API](https://docs.snowflake.com/en/developer-guide/sql-api/reference), [Databricks Statement Execution](https://docs.databricks.com/api/statement-execution/v1/statement-execution), [BigQuery Jobs](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/Job), [Redshift transactions](https://docs.aws.amazon.com/redshift/latest/dg/r_BEGIN.html) and [Redshift metadata](https://docs.aws.amazon.com/redshift/latest/dg/r_SVV_REDSHIFT_TABLES.html).
