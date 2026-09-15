# 支持与验收矩阵 / Support matrix

[English](support-matrix.md)

支持的数据库版本覆盖 **18 个产品、20 个产品/版本组合**，InfluxDB 1.x、2.x、3 Core 分开计算。最近一次完整矩阵于 **2026-09-15** 在 OrbStack Linux arm64 隔离实例中执行，属于 0.6.0 多管理员功能阶段：135 个查询/错误用例、104 个拒绝操作用例，覆盖连接、结构发现、参数与类型、空结果和错误、危险操作拒绝、测试数据未改变、限制、取消及原生/模板等价。[功能记录](verification/administrators.json)保留实际摘要；这是 0.6.0 发行基线记录，不覆盖后续预览适配器。最终发行检查及复现步骤见[验收记录](validation.zh-CN.md)。

**HTTP API 数据源**是额外的连接类型，不计为第 19 个数据库产品。固定 GET/POST JSON 操作通过隔离实例验证，不代表认证任意上游 API 的只读行为。详见 [HTTP API 能力与限制](http-api.zh-CN.md)。

“已验收”仅指表中版本及所述能力，不推断其他版本、发行版、权限插件或集群拓扑兼容性。

| 产品 | 实测版本 | 原生读取能力与参数 | 分页 | 实际保护 / 账号证据 |
|---|---|---|---|---|
| PostgreSQL | 17.11 | SQL、CTE、窗口、聚合；`$1` | 显式 SQL | AST + 每次只读事务；engine_enforced |
| MySQL | 8.4.11 | SQL、CTE、窗口、聚合；`?` | 显式 SQL | AST + 每次只读事务；engine_enforced |
| MariaDB | 10.11.18 | SQL、CTE、窗口、聚合；`?` | 显式 SQL | AST + 每次只读事务；engine_enforced |
| TiDB | 8.5.1 | 原生 SQL；`?` | 显式 SQL | AST + 检查 SHOW GRANTS 仅 SELECT/SHOW VIEW/USAGE；verified |
| CockroachDB | 24.3.15 | 原生 SQL；`$1` | 显式 SQL | PostgreSQL AST + 只读事务；engine_enforced |
| TimescaleDB | 2.25.2 / PostgreSQL 17.7 | 原生 SQL、hypertable、time_bucket；`$1` | 显式 SQL | 独立扩展版本探测 + 只读事务；engine_enforced |
| SQLite | 3.53.4 | SQL、CTE、窗口；`?` | 显式 SQL | mode=ro、query_only、authorizer；engine_enforced |
| DuckDB | 1.5.5 | SQL、CTE、窗口；`?` | 显式 SQL | READ_ONLY、引擎 StatementType、禁外部访问/扩展；engine_enforced |
| ClickHouse | 24.8.14.39 | 分析 SQL；`{name:Type}` + named_params | 显式 SQL | parser、readonly=1、allow_ddl=0；engine_enforced |
| MongoDB | 7.0.39 | find/filter/projection/sort、aggregate、count、distinct；Extended JSON | 原生游标，5 分钟、每源最多 64 个 | 固定读取操作、递归阶段/脚本检查；专用 read 角色 |
| Redis | 7.4.6 | String/Hash/List/Set/ZSet/Stream；独立字符串参数数组 | SCAN 家族 | 读取命令白名单 + 专用 ACL；权限全集未验证 |
| Valkey | 8.1.9 | 与 Redis 相同的受限读取族，独立实测 | SCAN 家族 | 读取命令白名单 + 专用 ACL；权限全集未验证 |
| Elasticsearch | 8.19.17 | Search DSL、聚合、get、count、mapping；JSON 参数 | search_after | 固定读路径、禁脚本；HTTPS + 专用读取角色，实测原生写入 403 |
| OpenSearch | 3.2.0 | Search DSL、聚合、get、count、mapping；JSON 参数 | search_after | 固定读路径、禁脚本；HTTPS + 专用读取角色，实测原生写入 403 |
| Neo4j | Community 5.26.10 | MATCH、聚合、节点/关系/路径；`$name` | 显式 Cypher | EXPLAIN 分类只读 + 全语句检查；账号权限未验证 |
| Cassandra | 5.0.8 | SELECT、TTL、writetime；`?` | 原生 PageState | SELECT 语法限制 + 专用 SELECT 角色；权限全集未验证 |
| ScyllaDB | 2026.2.5 | SELECT、TTL、writetime；`?`；独立产品版本探测 | 原生 PageState | SELECT 语法限制 + 专用角色；system.versions 元数据读权限 |
| InfluxDB 1.x | 1.8.10 | InfluxQL、聚合；`$name` 条件绑定 | 显式查询 | 仅 SELECT + 固定 /query；专用 READ 用户 |
| InfluxDB 2.x | 2.7.12 | 只读 Flux、聚合；`params.name` 字面量 AST | 显式查询 | 禁导入/网络/插值/写入；bucket read Token |
| InfluxDB 3 Core | 3.11.2 | SQL 与 InfluxQL 固定 API；`$name` | 显式查询 | **查询 API 隔离**；管理员 Token 本身具有管理权限 |


## 尚未发布的云数仓预览

| 连接器 | 接口 / 参数 | 真实云环境验证 |
|---|---|---|
| Snowflake | SQL API；`?` 与 params | 未验证 |
| Databricks SQL | Statement Execution API；`:name` 与 named_params | 未验证 |
| Google BigQuery | Jobs API、dry-run SELECT；`@name` 与 named_params | 未验证 |
| Amazon Redshift | PostgreSQL 协议、只读事务；`$1` 与 params | 未验证 |

四个预览适配器已实现，但不计入上述已验证数量，不包含在现有 v0.6.0 下载包中。云 REST 模拟测试验证请求/响应处理，不能证明产品兼容。结构发现、认证方式、受限 SQL 子集、计费上限和手工维护的云连接版本见[云数仓指南](cloud-warehouses.zh-CN.md)。

## 读取边界

- SQL 读取子集保留关联、子查询、只读 CTE、聚合、窗口函数，不做统一方言重写。解析器或函数白名单无法接受的语法会明确拒绝；自定义函数/过程、外部表函数、锁、导入导出不在支持范围。
- 上表 SQL 查询的“显式 SQL”分页不限制结构发现。SQL 命名空间、表和字段列表使用有序偏移游标续页，遵守每页行数和响应大小限制；结构发生变化时需重新开始，不提供跨页快照。
- SQLite/DuckDB 仅查管理员配置目录内已存在的数据库文件。不能从查询中指定其他 CSV、Parquet、URL、附件数据库或扩展。
- MongoDB distinct 用受限聚合返回去重值，数组按元素去重并排除缺失字段；不允许脚本、写入阶段、磁盘溢出。跨页游标一次消费；到期或服务重启后需重新查询。数据变化时不承诺跨页快照。
- Redis/Valkey 不开放 KEYS、脚本、阻塞读取和管理命令。SCAN COUNT 是提示；一次批量超出限制时明确截断，不返回会跳过结果的游标。
- Search 保留 `size:0` 聚合及较小的 size，上限只会收紧；有更多命中且无排序时标记截断，有稳定排序时返回续页游标。Search 的深分页需要显式稳定排序；无 PIT/scroll 快照。聚合数据作为原生结果保留，原生响应过大时返回大小错误。
- Neo4j 不开放 APOC、自定义过程、LOAD CSV；驱动读取路由不是数据库端权限证明。
- CQL 只接受完整 SELECT token 流，由数据库解析实际语法；不接受 UDF/批处理。原生 PageState 不直接暴露给 Agent。
- Flux 不开放 import/package/option 和网络访问；支持的内置函数见 guardFlux。OSS 2.7.12 的参数通过 extern 字面量 AST 绑定，已验证字符串插值内容不会执行；超出 int64 的整数参数拒绝，避免精度损失。
- 所有族都有请求超时、行数/字节限制；某些原生响应不能安全按行流式截断时返回明确错误。未提供精确原生类型的 API 不伪造类型。

## 证据解释

`engine_enforced` 说明本次已确认引擎/文件只读机制，不说明账号所有权限已被枚举。`verified` 仅用于实际读到的权限证据足够的场景。其他连接显示账号权限未验证；InfluxDB 3 Core 单独显示 api_isolated。

矩阵为单节点/单副本核心读取验收，不包含每个数据类型的全量交叉组合、所有 SQL/DSL 操作符、分布式故障切换、长期压力测试、生产网络和所有 TLS/账号插件组合。共享 HTTP TLS 层另有真实证书信任验证测试；Elasticsearch/OpenSearch 单节点 HTTPS、安全插件与读取角色已实测；其他产品的 TLS 集群部署仍需部署方验收。
