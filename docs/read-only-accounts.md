# 只读账号配置 / Read-only database accounts

以下命令由数据库管理员在数据库中执行，MCP DB Hub 不创建账号、不修改数据库授权，也不使用写入探测权限。将示例中的数据库、用户名、密码替换为实际值；使用专用账号，避免继承额外角色。表/列/行范围由数据库权限、视图和 RLS 控制。

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

默认权限必须针对实际建表角色配置。不要授予 SUPERUSER、CREATE、角色管理或危险函数执行权限；数据库中已有 PUBLIC 权限也需要管理员评估。TimescaleDB 的 hypertable 读权限遵循 PostgreSQL，服务每次查询另起只读事务。

## MySQL / MariaDB / TiDB

```sql
CREATE USER 'hub_reader'@'%' IDENTIFIED BY 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT SELECT, SHOW VIEW ON app.* TO 'hub_reader'@'%';
SHOW GRANTS FOR 'hub_reader'@'%';
```

实际部署把 `%` 收紧到 Hub 来源网络。MySQL/MariaDB 每次使用只读事务。TiDB 不依赖只读事务提示，连接时要求 SHOW GRANTS 明确仅有 SELECT/SHOW VIEW/USAGE；间接角色、列级复杂授权或无法识别的 grant 会保守拒绝，需要配置直接授权的专用账号。

## CockroachDB

```sql
CREATE USER hub_reader WITH PASSWORD 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT CONNECT ON DATABASE app TO hub_reader;
GRANT USAGE ON SCHEMA app.public TO hub_reader;
GRANT SELECT ON TABLE app.public.orders TO hub_reader;
```

按需授予其他表。验收实例使用隔离的 insecure 单节点；正式部署使用数据库 TLS 和专用证书/密码，UI 选择校验证书。

## ClickHouse

```sql
CREATE USER hub_reader IDENTIFIED BY 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT SELECT ON app.* TO hub_reader;
ALTER USER hub_reader SETTINGS readonly = 1;
```

服务还设置 allow_ddl=0、禁用 introspection、超时和结果大小限制。不能用管理员账号的连通成功证明账号只读。

## SQLite / DuckDB

把已有数据库文件放进 `--database-dir` 指定目录，并只读挂载给服务。UI 填写容器内绝对路径，例如 `/databases/report.duckdb`。禁止 ATTACH、扩展、COPY、CSV/Parquet/网络文件读取；仅当前已配置文件可查。SQLite 活跃 WAL 数据库还应提供数据库需要的同目录文件权限或由管理员生成一致快照；服务不会替用户复制数据库。

## MongoDB

```javascript
use app
db.createUser({user: "hub_reader", pwd: "REPLACE_WITH_RANDOM_PASSWORD",
               roles: [{role: "read", db: "app"}]})
```

UI 的 auth_source 填用户创建所在数据库，副本集可指定 replica_set。Hub 使用 connectionStatus 的有效 actions 检查；证据不完整时显示未验证。

## Redis / Valkey

```text
ACL SETUSER hub_reader on >REPLACE_WITH_RANDOM_PASSWORD ~app:* -@all +@read +ping +info
```

用数据库 ACL 约束 key 范围。Hub 另有更窄的命令白名单，因此 `@read` 中的命令不一定都可调用。若 INFO 被禁，部分版本探测无法完成。集群拓扑、Sentinel 故障切换不在本次实测矩阵中。

## Elasticsearch / OpenSearch

Elasticsearch 的专用角色给目标索引 `read` 与 `view_index_metadata`，版本发现可能需最小的 cluster monitor 权限。OpenSearch Security 的专用角色给目标 index pattern 的 `read` action group 及 mapping/版本发现所需元数据权限。不要授予 write、manage 或任意代理路径权限。

配置用户名/密码或产品支持的 bearer token，通过 UI 配置 CA 并启用 TLS。本轮两个产品均开启安全插件与 HTTPS，使用专用读取角色通过了查询、结构发现和 MCP 调用，并在隔离实例直接验证该账号写入返回 HTTP 403、未信任 CA 的连接失败。测试证据不等于服务能枚举生产账号的全部有效权限，因此 UI 的通用探测仍保留“账号权限未验证”。多节点和其他权限插件组合未验收。

## Neo4j

支持权限角色的发行版可创建用户并仅授予指定数据库的读取角色/图读取权限。Community 的权限能力有限，验收使用 Community 5.26.10，通过引擎 EXPLAIN 分类和查询校验保护读取；不宣称该账号本身只读。需要数据库端强只读隔离时，使用支持细粒度权限的部署或只读实例。

## Cassandra / ScyllaDB

```sql
CREATE ROLE hub_reader WITH PASSWORD = 'REPLACE_WITH_RANDOM_PASSWORD' AND LOGIN = true;
GRANT SELECT ON KEYSPACE app TO hub_reader;
-- ScyllaDB 产品版本发现还需以下元数据读权限：
GRANT SELECT ON TABLE system.versions TO hub_reader;
```

集群需启用 PasswordAuthenticator 与 CassandraAuthorizer；不要授予 MODIFY、CREATE、ALTER、AUTHORIZE、EXECUTE UDF。ScyllaDB 2026.2 首次启动需显式创建 superuser；测试脚本通过独立 scylla.yaml 配置初始哈希，避免镜像启动脚本展开 `$`。CQL 参数按预备语句的原生类型编码，支持精确 Decimal、float/double、列表与 map；超出类型范围的数字拒绝。普通集合的比较仍受 CQL 方言限制，验收使用 frozen 集合验证整体参数绑定。参见 [ScyllaDB 初始账号配置](https://docs.scylladb.com/manual/stable/reference/configuration-parameters.html)。

## InfluxDB 1.x

```sql
CREATE USER hub_reader WITH PASSWORD 'REPLACE_WITH_RANDOM_PASSWORD';
GRANT READ ON app TO hub_reader;
```

先由管理员启用 HTTP 认证，再配置此用户。原生 InfluxQL 绑定参数支持条件值。

## InfluxDB 2.x

在 InfluxDB 管理界面创建只对所需 bucket 授予 read 的 API Token。在 Hub 配置 token、org、bucket。不要使用 all-access Token。Flux 仅支持读取子集与原生字面量参数，禁用 import、网络访问和写入。

## InfluxDB 3 Core

按照已确认的例外，配置 Core 的可用 Token，Hub 仅开放 `/api/v3/query_sql` 与 `/api/v3/query_influxql` 等固定读取接口。**管理员 Token 仍具管理/写入权限；UI 标为“查询 API 隔离”。** 不能将该状态等同数据库只读账号。接口依据 [InfluxDB 3 查询 API](https://docs.influxdata.com/influxdb3/core/api/query-data/)。
