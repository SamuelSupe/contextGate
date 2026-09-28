# 用户指南：通过 Agent 查询数据

[English](user-guide.md) · [文档首页](README.zh-CN.md)

如果团队已经部署 ContextGate，你希望在 MCP 客户端中查询数据，从这里开始。准备好管理员提供的接入信息和查询权限即可，无需 ContextGate 管理员账号或数据库密码。

Web 界面是管理控制台。这里的“用户”指查询 Agent 的使用者，不是新的控制台账号角色。负责部署服务或为他人发布查询时，请阅读[管理员指南](admin-guide.zh-CN.md)。

**阅读顺序：**[接入客户端](#接入客户端) → [执行已发布查询](#执行已发布查询) → [理解结果与翻页](#理解结果与翻页)。数据源允许时，再使用[原生查询](#原生查询)。

## 接入客户端

向管理员获取以下信息：

- 客户端可访问的 MCP 地址，例如 `https://data.example.com/mcp`。
- 查询 Agent Token，或可由管理员批准的 OAuth 接入配置。
- 用于首次查询的数据源、已发布查询、示例参数及业务限制。

使用 Token 时，将服务加入客户端的 MCP 配置。下面是通用 HTTP 示例；具体客户端格式不同时，使用管理员提供的对应配置：

```json
{
  "mcpServers": {
    "contextgate": {
      "url": "https://data.example.com/mcp",
      "headers": { "Authorization": "Bearer <AGENT_TOKEN>" }
    }
  }
}
```

在本地替换占位值。如果配置引用 `MCPDBHUB_TOKEN`，在启动客户端的环境中设置该变量，再重启客户端。在另一个终端中 export 不会更新已经运行的桌面应用。`localhost` 指客户端所在机器，远程接入需要服务的公开 HTTPS 地址。

使用 OAuth 时，采用不含固定 Bearer Token 的配置，并按客户端提示打开浏览器授权。ContextGate 需要管理员登录并明确同意，请由管理员完成这一步。OAuth 会创建独立的 Agent 身份和数据源授权。

查询入口是 `/mcp`。用于 `/mcp/config` 的配置 MCP 凭证属于管理员，不能替代查询 Token。

<details>
<summary>客户端只支持 stdio 时</summary>

使用兼容本机的 `contextgate` 程序桥接到已有服务：

```json
{
  "mcpServers": {
    "contextgate": {
      "command": "/absolute/path/contextgate",
      "args": ["stdio", "--url", "https://data.example.com/mcp"],
      "env": { "MCPDBHUB_TOKEN": "<AGENT_TOKEN>" }
    }
  }
}
```

桥接与 HTTP 使用相同的授权、执行限制和审计，只向 stdout 写入 MCP 协议消息。

</details>

## 执行已发布查询

在 Agent 客户端中，可以从一个具体问题开始：

> 列出我能访问的数据源，查找已发布的客户订单查询，读取参数契约和业务定义，再用我提供的客户 ID 执行。解释结果，并说明限制或缺失的数据。

Agent 应按以下顺序调用。下面的 JSON 表示 MCP 工具调用，不是可以直接发送的 HTTP 请求；ID、版本和参数均为示例，应使用你的服务实际返回的值。

1. **发现可访问的数据源。** 返回内容包含查询模式、可用工具、示例和执行限制。

   ```json
   {"name":"list_data_sources","arguments":{}}
   ```

2. **在选定来源中查找已发布查询。** 检索匹配名称、别名和说明。`kind: "template"` 只查查询模板，再从详情确认可执行状态；省略 `kind` 可同时查找业务定义。

   ```json
   {"name":"search_semantics","arguments":{"source_id":"src_example","keyword":"orders","kind":"template","limit":25}}
   ```

3. **读取契约。** 确认必填参数、默认值、业务含义、当前可执行状态及 `execution_version`。使用条目 ID `overview` 可读取该来源已发布的业务背景。

   ```json
   {"name":"get_semantic_entry","arguments":{"source_id":"src_example","entry_id":"lookup-orders"}}
   ```

4. **使用当前版本和实际参数执行。** 从上一步复制当前执行版本，不要假定示例中的版本仍然有效。

   ```json
   {"name":"execute_query_template","arguments":{"source_id":"src_example","template_id":"lookup-orders","execution_version":"3","parameters":{"id":42},"max_rows":100}}
   ```

你只能看到 Agent 已获授权数据源中的已发布内容，看不到草稿。来源处于 `templates_only` 模式时，应使用已发布模板，原生查询会被拒绝。模板因发布或验证失效而不可用时，需要管理员处理。

术语、指标和映射的本体概念用于解释查询，不会扩大权限、自动跨库联查或证明答案正确。仍应根据实际业务问题核对结果。

## 填写参数

按 `get_semantic_entry` 返回的名称和 JSON 类型传参，未知参数会被拒绝。省略参数时，有默认值就使用默认值；可选参数没有默认值时保留预设固定值。没有默认值的必填参数不可省略。只有契约允许时才能传 `null`。

准备 JSON 时保留大整数和 Decimal 的精度，避免经 JavaScript `Number` 转换造成舍入。模板参数只能填入预先声明的值位置，不能替换查询文本、表名、命令、HTTP 路径或凭证。

## 理解结果与翻页

结果保留表格、文档、图、时序或 HTTP JSON 的原生结构。MCP 同时返回结构化内容和 JSON 文本形式。

| 字段 | 含义 |
| --- | --- |
| `format`、`data` | 结果格式和原生数据 |
| `row_count`、`elapsed_ms`、`bytes` | 返回数量、执行耗时和响应大小 |
| `truncated` | 结果不完整，不能当作全量数据 |
| `next_cursor` | 当前操作还有可继续读取的页面 |
| `semantic_version`、`template_id`、`template_version` | 本次使用的已发布上下文和模板执行版本 |
| `ontology_context` | 可选的本体版本及显式概念引用，数据仍保留原生结构 |

Int64/uint64 与精确 Decimal 使用无损字符串，二进制值标注 `encoding: base64`，时间戳保留可用精度。MongoDB 使用规范 Extended JSON；Flux 按表保留列和类型信息。没有返回原生类型，不代表它是字符串。云数仓预览适配器另有[结果格式限制](cloud-warehouses.zh-CN.md#类型发现与模板)。

默认查询上限为 30 秒、1,000 行、5 MiB，实际来源可能由管理员设置了不同限制。通过 `timeout_seconds`、`max_rows` 和 `max_bytes` 只能收紧，不能超过来源上限。字节预算覆盖两种 MCP 结果表示，单个值或原生响应过大时可能直接返回大小错误。

返回 `next_cursor` 时，保持身份、操作、查询、参数和限制不变，只增加或替换 `cursor`。游标五分钟后过期，相关授权、连接或发布变更也会使其失效。语义发布变化后，重新发现和分页。

各查询族的翻页方式不同：

- SQL 和 Cypher 在查询中显式分页；结果截断不代表一定存在游标。
- MongoDB 的续页句柄只能使用一次，每个来源最多保留 64 个原生游标。
- Search 的 `search_after` 要求稳定排序，不提供跨页快照。
- Redis 的 `COUNT` 是提示值；批次过大时可能截断且不返回不安全的续页游标。
- SQL 结构发现可能返回 `next_cursor`；页面不是快照，结构变化时应重新发现。

语义搜索每页最多返回 100 条摘要和 64 KiB，游标绑定身份、来源、搜索参数和发布版本。单条详情上限 128 KiB，关联摘要最多 25 条。模板的原生查询游标还绑定模板 ID 和执行版本，不能交给原生查询工具使用。

语义搜索支持 `overview`、`term`、`object`、`field`、`relationship`、`metric`、`template`、`entity_type`、`property`、`relation_type` 类型。使用发现结果返回的概念 ID；只有映射到已授权来源的概念可见。这些说明是业务上下文，不能覆盖 Agent 自身的规则。

## 原生查询

来源允许原生查询时，通过 `list_namespaces`、`list_objects`、`describe_object` 发现结构，再使用 `list_data_sources` 返回的工具和示例：

| 数据类型 | 工具 |
| --- | --- |
| SQL | `query_sql` |
| MongoDB | `query_mongodb` |
| Redis / Valkey | `query_redis` |
| Elasticsearch / OpenSearch | `query_search` |
| Neo4j | `query_cypher` |
| Cassandra / ScyllaDB | `query_cql` |
| InfluxDB | `query_influxdb` |
| 已配置的 HTTP API 操作 | `query_http_api` |

遵循来源的原生方言和参数语法。网关只允许受支持的只读子集，写入、多语句、锁定读取和危险函数即使能被数据库执行，也可能被拒绝。Agent 不能提供连接地址或凭证；HTTP API 只能选择已配置操作，不能传入任意 URL。原生能力限制和已验证版本见[支持矩阵](support-matrix.zh-CN.md)。

## 查询遇到问题

| 现象 | 你可以先做什么 | 何时联系管理员 |
| --- | --- | --- |
| 无法连接或 HTTP 401 | 检查地址、Token 环境和客户端重启情况；OAuth 可重新授权 | Token 过期或撤销、Agent 暂停，或远程服务不可达 |
| 看不到数据源 | 确认使用了正确的 Agent 身份 | 需要来源授权，或来源被停用 |
| 找不到模板 | 换关键词并确认所选来源 | 确认查询是否已发布且可执行 |
| 参数无效 | 读取当前契约，修正名称、类型和值 | 契约无法表达实际业务输入 |
| 执行版本过旧 | 重新读取条目，使用当前版本 | 模板仍然不可用 |
| 游标无效 | 去掉游标，重新开始当前操作 | 参数不变时仍反复失败 |
| `query_denied` | 检查查询模式与支持的只读操作 | 需要发布对应模板或重新审核授权 |
| 结果为空或被截断 | 核对筛选、时间范围和单位；有游标时继续翻页 | 结果仍与预期业务含义不符 |

反馈时提供来源/模板 ID、时间、安全错误分类和可用的请求 ID，不要附上密码或完整 Token。管理员可以关联审计记录；客户端显示已连接，并不代表查询已经成功。
