# Read-only database accounts

[简体中文](read-only-accounts.zh-CN.md)

A database administrator runs these commands in the database. ContextGate does not create accounts, modify grants or probe privileges by attempting writes. Replace the example database, username and password with your own values. Use dedicated accounts without inherited extra roles. Database permissions, views and RLS control table, column and row visibility.

## PostgreSQL / TimescaleDB

```sql
CREATE ROLE hub_reader LOGIN PASSWORD 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT CONNECT ON DATABASE app TO hub_reader;
GRANT USAGE ON SCHEMA public TO hub_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO hub_reader;
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA public
  GRANT SELECT ON TABLES TO hub_reader;
ALTER ROLE hub_reader SET default_transaction_read_only = on;
```

Configure default privileges for the role that actually creates tables. Do not grant SUPERUSER, CREATE, role administration or execution of dangerous functions; administrators must also assess existing PUBLIC privileges. TimescaleDB hypertable read permissions follow PostgreSQL, and the service creates a separate read-only transaction for every query.

## MySQL / MariaDB / TiDB

```sql
CREATE USER 'hub_reader'@'%' IDENTIFIED BY 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT SELECT, SHOW VIEW ON app.* TO 'hub_reader'@'%';
SHOW GRANTS FOR 'hub_reader'@'%';
```

Restrict `%` to ContextGate network in an actual deployment. MySQL/MariaDB use a read-only transaction for each query. TiDB does not rely on a read-only transaction hint: SHOW GRANTS must explicitly contain only SELECT/SHOW VIEW/USAGE. Indirect roles, complex column grants and unrecognized grants are conservatively rejected; configure a dedicated account with direct grants.

## CockroachDB

```sql
CREATE USER hub_reader WITH PASSWORD 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT CONNECT ON DATABASE app TO hub_reader;
GRANT USAGE ON SCHEMA app.public TO hub_reader;
GRANT SELECT ON TABLE app.public.orders TO hub_reader;
```

Grant access to additional tables as needed. Validation used an isolated insecure single-node fixture. Production deployments should use database TLS and dedicated certificates/passwords, with certificate verification enabled in the UI.

## ClickHouse

```sql
CREATE USER hub_reader IDENTIFIED BY 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT SELECT ON app.* TO hub_reader;
ALTER USER hub_reader SETTINGS readonly = 1;
```

The service also sets allow_ddl=0, disables introspection, and applies timeout and result-size limits. A successful connection with an administrator account does not establish that the account is read-only.

## SQLite / DuckDB

Place existing database files in the directory selected by `--database-dir` and mount it read-only for the service. Enter an absolute container path in the UI, such as `/databases/report.duckdb`. ATTACH, extensions, COPY and CSV/Parquet/network file access are rejected; queries can access only the configured database file. An active SQLite WAL database also needs the appropriate access to adjacent files, or an administrator-created consistent snapshot. The service does not copy user databases.

## MongoDB

```javascript
use app
db.createUser({user: "hub_reader", pwd: "REPLACE_WITH_RANDOM_PASSWORD",
               roles: [{role: "read", db: "app"}]})
```

Set auth_source to the database where the user was created; replica sets can specify replica_set. ContextGate checks effective actions from connectionStatus and reports unverified privileges when evidence is incomplete.

## Redis / Valkey

```text
ACL SETUSER hub_reader on >REPLACE_WITH_RANDOM_PASSWORD ~app:* -@all +@read +ping +info
```

Use database ACLs to restrict key patterns. ContextGate applies a narrower command allowlist, so not every command in `@read` is available. Disabling INFO can prevent some version discovery. Cluster topologies and Sentinel failover are outside the tested matrix.

## Elasticsearch / OpenSearch

For Elasticsearch, grant a dedicated role `read` and `view_index_metadata` on the target indexes; version discovery may require minimal cluster-monitor privileges. For OpenSearch Security, grant the `read` action group on the target index pattern and the metadata privileges needed for mapping/version discovery. Do not grant write, manage or arbitrary proxy-path privileges.

Configure a username/password or a product-supported bearer token, enable TLS and provide the CA in the UI. Both products were tested with security plugins and HTTPS. Dedicated reader roles passed queries, discovery and MCP calls; direct writes returned HTTP 403, and connections with an untrusted CA failed. This fixture evidence does not imply that ContextGate can enumerate every effective production-account privilege, so generic UI probes retain an account-permissions-unverified status. Multi-node and other authorization-plugin combinations were not validated.

## Neo4j

In editions supporting permission roles, create a user with only the required database/graph read permissions. Community has limited authorization capabilities. Validation used Community 5.26.10 with engine EXPLAIN classification and query validation; it does not claim the account itself is read-only. For stronger database-side isolation, use a deployment with granular permissions or a read-only instance.

## Cassandra / ScyllaDB

```sql
CREATE ROLE hub_reader WITH PASSWORD = 'REPLACE_WITH_RANDOM_PASSWORD' AND LOGIN = true;
GRANT SELECT ON KEYSPACE app TO hub_reader;
-- ScyllaDB product-version discovery also requires this metadata read permission:
GRANT SELECT ON TABLE system.versions TO hub_reader;
```

Enable PasswordAuthenticator and CassandraAuthorizer in the cluster. Do not grant MODIFY, CREATE, ALTER, AUTHORIZE or EXECUTE UDF. ScyllaDB 2026.2 requires explicit initial superuser creation; the fixture script supplies its initial hash through a separate scylla.yaml to avoid `$` expansion by the image entrypoint. CQL parameters use prepared-statement native types, including exact Decimal, float/double, lists and maps; out-of-range numbers are rejected. Ordinary collection comparisons remain subject to CQL dialect restrictions; validation used frozen collections for whole-value binding. See [ScyllaDB initial account configuration](https://docs.scylladb.com/manual/stable/reference/configuration-parameters.html).

## InfluxDB 1.x

```sql
CREATE USER hub_reader WITH PASSWORD 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT READ ON app TO hub_reader;
```

An administrator must enable HTTP authentication before configuring this user. Native InfluxQL parameter binding supports condition values.

## InfluxDB 2.x

Create an API token in the InfluxDB UI with read permission only for the required bucket. Configure token, org and bucket in ContextGate. Do not use an all-access token. Flux supports a read-only subset with native literal parameters; imports, network access and writes are disabled.

## InfluxDB 3 Core

Under the explicit Core exception, configure an available Core token. ContextGate exposes only fixed read endpoints such as `/api/v3/query_sql` and `/api/v3/query_influxql`. **An administrator token retains administration/write privileges; the UI labels this query API isolation.** This status is not equivalent to a read-only database account. See the [InfluxDB 3 query API](https://docs.influxdata.com/influxdb3/core/api/query-data/).
