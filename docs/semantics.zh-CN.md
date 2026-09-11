# 数据源语义目录与查询模板

[English](semantics.md) · [示例目录](../examples/semantics/) · [架构](architecture.zh-CN.md)

**0.2.0** 为每个数据源提供独立的业务术语、对象、字段、关系、指标和原生查询模板。继续采用 **Agent → Data Source** 授权。语义只描述业务，不承担权限控制，也不能覆盖 Agent 的系统指令。界面使用英文；名称、别名和说明支持中文及其他语言。

## 配置和发布

1. 打开 **Data sources → Semantics**，在 **Overview** 填写业务背景、数据来源和约定。
2. 在 **Catalog** 从结构发现选择对象导入骨架，或新建术语、对象、字段、关系和指标。补充单位、枚举含义、时间口径、粒度和注意事项。对象引用使用 `namespace`、`object`、可选 `field`，关系限定当前数据源。
3. 在 **Query templates** 编写原生查询 JSON，设置参数类型、绑定位置、约束、示例参数和结果说明。指标编辑器可以关联一个模板。
4. 保存草稿后运行 **Validate draft**；新增或修改的启用模板必须点击 **Trial**，使用保存的示例参数对真实数据库完成只读试跑。
5. 点击 **Publish** 并确认，原子替换 Agent 可见的完整快照。**Discard draft** 用已发布快照恢复草稿。旧修订保存或发布返回 HTTP 409，不覆盖其他页面的更新。

保存、JSON 导入和结构导入都只修改草稿。试跑不保存结果，只保留脱敏审计与加密的验证证据。仅说明变更不会修改模板执行版本，也不会取消正在运行的模板查询；执行定义变更、禁用或删除发布后，会取消受影响任务。

数据源编辑器的 **Query access** 默认是 **Native queries and templates**。选择 **Templates only** 后，所有 Agent 原生查询入口都被共享执行层拒绝，包括 HTTP、stdio、OAuth 和 Agent 身份预览。结构发现与语义读取仍可用。没有有效已发布模板时，Agent 不能查询。管理员保留原生预览与草稿试跑能力。

## 验证证据与连接生命周期

验证证据绑定规范化的执行定义、连接配置、凭证和实际数据库版本，保存在不可导入的独立存储中。名称、说明、查询上限修改不会让证据过期；查询始终遵守最新上限。

连接或凭证变更会使证据过期，即使随后恢复旧值也需要重新验证。每次模板查询会在取得的连接上读取数据库版本。发现已知版本变化会暂停模板并取消该数据源任务；必须允许读取版本元数据，否则模板不能完成验证或执行。首次记录版本不会修改数据源编辑修订，已知版本发生变化才会修改。

失效后需要 **重新 Trial，再 Publish**。单独试跑不会恢复旧发布模板；重新发布会生成新的执行版本。连接检查不会发送试探性写入，也不会把账号权限“未验证”误报成“已只读验证”。

## 参数契约

模板包含 `tool`、`query_json`、`parameters`、`example_json`、`enabled`，以及可选 `result_description`。配置中的原生查询和 JSON 参数值使用字符串保存，避免浏览器通过 JavaScript Number 损失大整数或 Decimal 精度。Agent 调用时提交真实 JSON 参数值，Go 使用无损数字解码。

每个参数包含唯一 `name`、`type`、`required`、一个或多个 JSON Pointer `pointers`。类型支持 `string`、`integer`、`number`、`boolean`、`object`、`array`、`null`。可选约束为 `default_json`、`enum_json`（JSON 数组）、`minimum`、`maximum`（十进制字符串）。省略参数时先采用默认值；无默认值的非必填参数保留模板固定槽位值。声明默认值的必填参数也可以采用默认值。额外参数被拒绝。

| 查询族 | 允许绑定的位置 |
|---|---|
| SQL | 驱动支持的直接 `/params/0` 或 `/named_params/name` 原生参数槽 |
| CQL | 直接 `/params/0` 原生参数槽 |
| Cypher / InfluxDB | 直接 `/named_params/name` 原生参数槽 |
| Redis / Valkey | `/args/N` 的现有字符串元素，命令和选项关键字固定 |
| MongoDB | `/filter/...` 和 `$match` 聚合阶段的标量叶子值 |
| Elasticsearch / OpenSearch | `/body/query/...` 下 term、match、match_phrase、range、terms 数组、ids 数组的标量值 |

不能绑定到查询文本、命令、语言、数据源、namespace、对象名、JSON 键、操作符、限制或游标。对象和数组参数只允许用于原生数据库参数槽；文档模板不能通过绑定改变操作结构。MongoDB 参数字符串不能以 `$` 开头，Search 不能绑定查询字符串、脚本或 terms lookup 索引。Flux 的 bucket、Cypher 动态标签/关系目标和 ClickHouse Identifier 参数不能用于模板动态对象选择。

不进行字符串拼接或插值。绑定后的查询仍进入现有原生只读校验、账号授权、并发限制、取消、超时和结果编码。

目录上限为 500 条、768 KiB；单条上限 128 KiB；Overview 上限 32 KiB；每模板最多 128 个参数，每参数最多 32 个绑定。数字契约限制为 1,024 字符、指数绝对值不超过 10,000，防止无界解析。查询默认仍是 30 秒、1,000 条、5 MiB，管理员上限沿用原配置。

## MCP 调用

`list_data_sources` 新增 `query_access_mode`、`semantics_available`、`semantic_version`、`available_tools`，不一次返回目录。

```json
{"name":"search_semantics","arguments":{"source_id":"src_example","keyword":"金额","kind":"metric","limit":25}}
```

按名称、别名、说明关键词匹配。类型包括 `overview`、`term`、`object`、`field`、`relationship`、`metric`、`template`。每页最多 100 条摘要、64 KiB 摘要数据。游标有效期 5 分钟，绑定身份、数据源、关键词、类型、分页大小和发布版本；发布版本改变后重新开始检索。

```json
{"name":"get_semantic_entry","arguments":{"source_id":"src_example","entry_id":"lookup-orders"}}
```

保留条目 ID **`overview`** 用于读取已发布数据源背景。模板详情包含参数契约、调用示例、执行版本、是否可执行及关联指标摘要。关联摘要最多返回 25 条，超出时标记截断。

```json
{"name":"execute_query_template","arguments":{"source_id":"src_example","template_id":"lookup-orders","execution_version":"3","parameters":{"id":42},"max_rows":100}}
```

Agent 只能提交模板标识、精确执行版本、参数，以及可选游标和更严格的上限。结果保留原生结构，并追加 `semantic_version`、`template_id`、`template_version`。执行前和返回前检查授权及模板有效性。原生分页游标额外绑定模板 ID、执行版本、查询、参数、身份；原生工具不能消费模板游标。SQL 继续由显式查询分页。

## 管理 API

基路径为 `/api/sources/{id}/semantics`，全部要求管理员会话，修改请求还要求 CSRF Token。修订和执行版本为字符串。

| 方法和路径后缀 | 内容 |
|---|---|
| GET 基路径 | 有界草稿、发布快照、修订、修改状态、试跑状态 |
| PUT 基路径 | `{revision, snapshot}` 替换草稿 |
| GET `/entries` | 草稿分页，支持 `keyword`、`kind`、`offset`、`limit`（1–100）、`revision` |
| PUT `/entries/{entry}` | `{revision, entry}` 保存条目 |
| DELETE `/entries/{entry}` | `{revision}` 删除草稿条目 |
| POST `/import-structure` | `{revision, objects:[{namespace,object}]}`，1–20 个对象，不覆盖人工说明 |
| GET `/export` | 导出草稿 JSON；`phase=published` 导出发布配置 |
| POST `/import` | `{revision, snapshot}` 替换草稿，不导入凭证或证据 |
| POST `/validate` | 校验已保存完整草稿，不执行查询 |
| POST `/trial` | `{revision, template_id}` 真实只读试跑 |
| POST `/publish` | `{revision}` 校验并原子发布 |
| POST `/discard` | `{revision}` 恢复发布快照 |
| POST `/execute` | 已发布模板预览，参数与 MCP 执行相同，可增加 `agent_id` |

导出 JSON 包含 `format_version: 1`、`overview`、`entries`；不含连接配置、凭证、验证证据或执行批准。导入使用明确的替换流程，只影响草稿。删除数据源会在事务内删除草稿、发布内容和验证证据。

## 结构导入边界

SQL/CQL、InfluxDB 1/3 使用元数据中的字段名和类型。Search 使用 mapping，包括嵌套字段和 multi-fields。MongoDB 导入集合和索引字段名，类型标记未知，不抽样文档推断完整结构。Redis 导入选中 Key 的名称及类型元数据。Neo4j 导入标签骨架，属性由管理员定义，避免读取节点内容。

InfluxDB 2 使用数据库的[结构发现函数](https://docs.influxdata.com/influxdb/v2/query-data/flux/explore-schema/)，范围为最近 30 天；字段类型不进行猜测。上述结构只用于生成骨架，均不读取业务样本来推断业务含义，不连接额外模型。

## 审计和示例

模板执行和管理员试跑沿用脱敏审计。已发布执行记录操作 `execute_query_template`、模板 ID、执行版本；OTLP 属性为 `mcpdbhub.audit.template_id`、`mcpdbhub.audit.template_version`。审计不保存目录全文、原生查询文本、参数、结果或连接秘密。

[示例目录](../examples/semantics/) 覆盖 PostgreSQL SQL、位置参数 SQL、ClickHouse、MongoDB、Redis、Search、Cypher、CQL 及三个 InfluxDB 版本。请按实际结构修改固定查询和业务定义，导入后试跑并发布。实际数据库验证结果见[验证记录](validation.zh-CN.md)。
