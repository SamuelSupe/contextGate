# 云数仓（预览）

[English](cloud-warehouses.md) · [支持矩阵](support-matrix.zh-CN.md) · [配置和模板示例](../examples/cloud-warehouses/README.md)

**ContextGate 0.7.0 已提供：** 已实现 Snowflake、Databricks SQL、Google BigQuery 和 Amazon Redshift 适配器。目前没有真实云环境验证，界面标记 **preview**，实测版本清单为空。已包含在 0.7.0 下载包中，但不计入 18 个产品／20 个版本组合的已验证矩阵。模拟 API 测试不能证明云产品兼容性。

## 配置与认证

在 **数据源 → 添加数据源** 中配置，或调用配置 MCP 的 `create_data_source`。四者均使用现有 `query_sql`，共用 Agent → Data Source 授权、语义模板、本体映射、审计与 OTLP，不需要新的 MCP 端点。

| kind | 连接字段 | 参数 |
|---|---|---|
| `snowflake` | 账号主机、HTTPS 443、database；options 中配置 warehouse、role 和可选 schema。auth_mode 为 token，token 存储 PAT 或 OAuth 访问令牌；token_type 为 PROGRAMMATIC_ACCESS_TOKEN 或 OAUTH。 | `?`，`params` |
| `databricks` | 工作区主机、HTTPS 443；database 表示 Unity Catalog 的 catalog；options.warehouse_id 和可选 schema。使用 token 认证，提供 PAT 或 OAuth 访问令牌。 | `:name`，`named_params` |
| `bigquery` | 固定主机 bigquery.googleapis.com、HTTPS 443；database 为计费项目 ID，options.location 必填、schema 为可选默认 dataset。auth_mode=service_account 时将 Google 服务账号 JSON 作为字符串放在 password；或使用 token 认证。 | `@name`，`named_params` |
| `redshift` | 集群或 Serverless workgroup 主机、database、端口（默认 5439）、用户名和密码，建议启用验证证书的 TLS。 | `$1`，`params` |

三个 HTTPS SQL API 强制证书验证，允许自定义 CA，不跟随重定向、外部结果链接或任意 HTTP 路径。查询 Agent 只能指定数据源 ID，不能指定连接主机。BigQuery 的查询主机和 OAuth Token 端点固定；不读取任意凭证文件，不使用环境默认凭证或工作负载身份。服务账号需要 Google JSON 中的 RSA PKCS8 私钥，至少 2048 位。

BigQuery 服务账号按需换取、缓存并刷新访问令牌；其他访问令牌需要管理员在到期前更换。编辑时留空保留当前认证方式的凭证，切换方式会清除旧方式凭证。暂不实现 Snowflake 密钥对登录、Databricks OAuth 客户端凭证换取、Redshift IAM 临时凭证生成或 Redshift Data API。

## 只读、费用与取消

先在云端创建专用读取身份：Snowflake 授予目标 warehouse/database/schema 的使用权限与目标表/视图 SELECT，并在数据源指定 reader role；Databricks 授予 warehouse 使用、catalog/schema 使用和目标对象 SELECT；BigQuery 需要计费项目的任务创建权限以及目标 dataset 的数据/元数据读取权限；Redshift 使用目标库/schema 使用权限与 SELECT。继承角色、函数、外部表及集成权限需要在云端独立限定。ContextGate 不创建账号或修改授权。

三个 HTTPS 数据源要求 `options.read_only_confirmed: "true"`，这是管理员声明，**不是权限验证**。连接成功后仍显示账号权限“未验证”。Redshift 每次查询开启只读事务并设置事务内 statement_timeout；探测证据只表示 BEGIN READ ONLY 被接受，不声称枚举了账号全部权限。不发送试探性写入。

云端查询只接受保守的 SELECT 子集：将原生参数占位符与标识符引号归一化后，用现有 PostgreSQL 语法树和函数白名单检查；**仅校验副本被归一化**，发给服务端的查询文本与参数保持分离且不变。支持通过校验的关联、子查询、只读 CTE、聚合和窗口；拒绝写入、脚本、写入 CTE、锁定读取、导入导出及危险/外部/限定名称的云函数。HTTPS 连接器还拒绝块注释、hint、末尾分号、歧义转义、会话变量。字符串使用单引号，Snowflake 标识符使用双引号，Databricks/BigQuery 使用反引号。QUALIFY、UDF、高级 GoogleSQL 类型语法等超出校验子集时会拒绝；不宣称完整方言覆盖。

BigQuery 先 dry run，必须得到 SELECT 语句分类才提交执行任务。`options.maximum_bytes_billed` 为正 int64 十进制字符串，默认 `1073741824`（1 GiB），同时用于 dry run 和实际任务。这限制扫描计费字节，不是结果条数，也不保证没有云费用。超时/取消会对已知任务发起有界取消请求，但断网或云端行为可能导致无法确认。BigQuery 提交前生成任务 ID；Snowflake/Databricks 若提交响应丢失且未取得句柄，则不能取消该任务，应同时配置云端超时和资源策略。

## 类型、发现与模板

JSON 数值参数不经浮点数往返；显式类型使用 `{"type":"TYPE","value":"精确文本"}`，仅接受允许的标量类型。暂不支持数组/结构体原生参数。未指定类型的 null 使用字符串类型，需要其他类型时显式指定。

- Snowflake 整数/小数默认 FIXED，可用 `{"type":"FIXED","value":"12345678901234567890.123456789"}`。
- Databricks 整数默认 BIGINT；小数必须指定，例如 `{"type":"DECIMAL(38,9)","value":"12345678901234567890.123456789"}`。
- BigQuery 整数默认 INT64、小数 NUMERIC，更大范围用 BIGNUMERIC。
- Redshift 使用现有 PostgreSQL 标量绑定和结果解码；SUPER 等专有类型尚未实测。

精确数值结果使用字符串并附带原生列类型。HTTP 返回的时间文本保持原精度，可能是 epoch 文本而不是 RFC3339（例如 BigQuery TIMESTAMP 通常是 Unix 秒）。Snowflake/Databricks 半结构化和二进制值保留 API 字符串表示与原生列类型，不会将二进制文本直接标为 base64。BigQuery repeated/record 转为数组/对象，布尔转为布尔，BYTES 使用 base64 对象。

原生结果分区/分块/分页在单次调用的限制内收集；SQL 查询分页仍需显式编写，截断不返回可能跳过数据的游标。单个原生响应过大时可返回大小错误。结构发现支持身份和数据源修订绑定的有界游标，但跨页不构成结构快照。Snowflake/Databricks 从当前 database/catalog 的 information_schema 发现结构，Databricks 需要暴露该结构的 catalog。BigQuery 发现当前项目的 dataset、table 和嵌套字段。Redshift 使用当前数据库的 SVV_REDSHIFT 视图，外部 catalog 发现不在本次预览范围。

导入结构后维护业务语义和模板，试跑、发布，再授予 Agent 访问。示例提供四份数据源配置及对应 v2 语义 JSON，默认停用且仅模板模式；请替换真实对象和业务口径。模板只绑定 `/params/0` 或 `/named_params/name` 等原生参数槽；类型/值对可以作为 object 合同占据整个原生槽，仍经过适配器类型检查。本体按现有流程独立绑定，不生成 SQL 或转换查询结果。

三个 HTTPS 数据源的 version 是管理员维护的**连接约定版本**，初始为 `"1"`，不会自动探测云引擎升级。相关升级、接口/权限变化后修改版本，重新试跑并发布。凭证与连接变更已会使证据过期。Redshift 沿用实际版本探测。试跑仅证明当前配置下的示例执行，不表示所有参数和云版本均验证。

## 验证范围

2026-09-15 在 OrbStack Linux arm64 执行 `go test -race ./...`、`go vet ./...`、前端测试和 UI/Go 生产构建。本机 Chrome 检查四种数据源表单、BigQuery 认证切换和无效凭证错误、预览标记、中英文界面与 390px 布局。随后为 0.7.0 发行重跑原有 18 产品／20 版本数据库矩阵，135 个查询/错误用例、104 个拒绝用例全部通过。[当前记录](verification/matrix.json)覆盖开源产品，不证明这些云数仓预览的兼容性。

本地自动化覆盖危险查询提交前拒绝、精确参数、模板绑定、异步轮询、结果分块/分页、空结果、dry run 拒绝、取消、凭证脱敏、服务账号令牌缓存，以及配置 MCP/UI API 的加密保存和凭证更新。真实云认证、授权、引擎行为、私网、费用和生产兼容性尚未验证。Redshift 本地覆盖查询校验，实际执行/发现仍需 AWS 环境。

正式标为支持前，逐产品在真实环境验证连接、发现、复杂查询、参数和类型、空结果和错误、拒绝操作且测试数据未变、截断/取消、原生与模板等价，单独记录引擎/runtime 和部署信息；不能用模拟测试更新 verified.json。

接口依据：[Snowflake SQL API](https://docs.snowflake.com/en/developer-guide/sql-api/reference)、[Databricks Statement Execution](https://docs.databricks.com/api/statement-execution/v1/statement-execution)、[BigQuery Jobs](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/Job)、[Redshift 事务](https://docs.aws.amazon.com/redshift/latest/dg/r_BEGIN.html)、[Redshift 元数据](https://docs.aws.amazon.com/redshift/latest/dg/r_SVV_REDSHIFT_TABLES.html)。
