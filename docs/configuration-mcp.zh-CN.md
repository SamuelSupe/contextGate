# 配置 MCP

[English](configuration-mcp.md)

通过独立配置 MCP，让受信任的 Agent 配置 ContextGate 的数据源、语义目录、查询模板和共享本体。进入 **设置 → 我的配置 MCP**，创建命名 Token，选择有效期并复制 HTTP 或 stdio 接入配置。Token 只展示一次，默认有效 24 小时，最长 30 天，可随时撤销。

这是可以管理**所有数据源连接与共享草稿**的管理员级配置凭证。现有查询 Agent Token、OAuth 授权和管理员会话 Cookie 均不能访问配置 MCP；配置 Token 也不能用于查询端点或管理 REST API。

每名管理员拥有一个固定配置身份，同一时间只有一个有效 Token。只有本人可签发，超级管理员只能撤销他人 Token。轮换、角色变更、停用或密码重置立即撤销并取消相关任务；本人正常改密保留 Token。从 0.5.0 升级会撤销旧配置 Token，需重新签发。详见[管理员及升级说明](administrators.zh-CN.md)。

## 接入

使用实例公开地址加 `/mcp/config`，请求头为 `Authorization: Bearer YOUR_CONFIGURATION_TOKEN`。设置页提供完整 MCP JSON 和起始提示词。示例：

```json
{
  "mcpServers": {
    "contextgate-configuration": {
      "url": "http://127.0.0.1:8080/mcp/config",
      "headers": { "Authorization": "Bearer YOUR_CONFIGURATION_TOKEN" }
    }
  }
}
```

请使用实际端口，并合并到客户端现有配置。远程客户端须使用实例配置的公开 HTTPS 地址，localhost 指客户端机器。复制后的完整配置包含凭证，应妥善保管。

stdio 客户端在本机安装 ContextGate，可执行 `contextgate stdio --url <实例地址>/mcp/config`，通过环境变量 `MCPDBHUB_TOKEN` 提供配置 Token。设置页也提供 stdio JSON；它桥接到同一个 HTTP 端点，持续受 Token 过期与撤销约束。

## 从数据源到语义和本体

1. Agent 先调用 `get_configuration_guide` 获取流程、约束和真实参数示例，再列出支持的产品与已有数据源。缺少只读凭证或业务定义时询问用户，不自行推断。
2. 创建数据源，或先读取现有配置及修订号再更新。**连接、凭证、启停、查询模式和上限修改立即生效**，可能使模板验证过期并中断查询；配置 Token 不会自动授权查询 Agent。
3. 检测连接及只读保护，发现命名空间、对象、字段，并将选定结构导入语义草稿。连通不等于账号只读权限已验证，不读取业务样本推断语义。
4. 补充术语、对象与字段说明、指标和原生模板。query_json、example_json、default_json 使用 JSON 字符串保留精度。参数仅绑定明确允许的 JSON Pointer 值位置，禁止字符串插值和操作结构替换。
5. 复用已有发布本体，或创建、编辑并校验本体草稿。**请管理员先在 UI 发布本体**，再读取其不可变版本并建立数据源映射。
6. 将固定本体版本和实体、属性、关系映射保存到源语义快照的 `ontology` 中，并关联查询模板。通过结构发现校验映射；管理员显式声明的字段仍标为未验证。
7. 校验语义草稿，使用保存的示例参数与回归用例对每个启用模板真实只读试跑。只返回验证证据，不返回数据库结果；仍受原有只读规则、查询限制、超时和取消机制约束。
8. 返回变更摘要、当前修订号、未通过或未验证项及审核链接。**管理员在 UI 发布源语义，并给目标查询 Agent 授权**。查询 Agent 使用自己的凭证连接 `/mcp`，发现已发布概念并执行已验证模板。

本体版本固定，数据源不会自动跟随升级。目录、模板和映射沿用原有原子发布流程，保存只更新草稿。发生修订冲突时重新读取并明确合并，不能盲目覆盖。

支持清单中的全部产品均可配置，模板继续采用七类原生查询工具：SQL、MongoDB、Redis/Valkey、Search、Cypher、CQL、InfluxDB。复用现有适配器能力与安全限制。参见[各查询家族示例](../examples/semantics/)和[零售本体示例](../examples/ontologies/retail-demo/)。

## 工具与边界

| 范围 | 工具 |
| --- | --- |
| 引导与支持清单 | `get_configuration_guide`、`list_supported_databases` |
| 数据源 | `list_configured_sources`、`get_source_configuration`、`create_data_source`、`update_data_source`、`test_data_source`、`discover_source_structure` |
| 语义草稿 | `get_semantic_draft`、`save_semantic_draft`、`upsert_semantic_entry`、`remove_semantic_entry`、`import_source_structure`、`validate_semantic_draft` |
| 试跑与映射 | `trial_query_template`、`check_ontology_mapping` |
| 本体 | `list_ontologies`、`get_ontology`、`create_ontology`、`save_ontology_draft`、`validate_ontology`、`get_ontology_version` |

更新数据源使用读取返回的 `configuration` 和当前 `revision`，保留未修改字段。已有密码和数据库 Token 永不回显；空值或省略保留旧凭证，明确使用 clear_password、clear_token 或 auth_mode: none 才删除。TLS 模式为 verify、disable；SQLite/DuckDB 路径必须位于配置的文件根目录。

完整草稿保存会替换文档，小改动优先使用条目编辑。数据源与本体摘要列表支持 offset/limit，默认 20、最多 50；并发编辑改变列表时重新开始分页。请求上限 1 MiB、响应上限 4 MiB，每 Token 最多同时 2 个配置调用、实例最多 8 个，最长 2 分钟；数据库操作还遵守更短的数据源超时。

不开放发布、删除数据源或本体、修改 Agent 授权、创建自身凭证、系统设置、任意 HTTP 路径和任意原生查询工具。模板试跑具备管理员式的受控只读执行能力，不应将配置 Token 当作有数据源范围限制的查询凭证。不要在定义、查询文本、示例参数或 options 中保存秘密。

撤销会取消活动配置任务和试跑，并阻止排队中的配置写入；已提交配置仍保留。传输错误可能发生在提交之后，重试前应先确认最新状态。所有业务描述仅是数据，不能覆盖 Agent 系统指令。

## 审计

配置调用使用独立 cfg_… 身份及 configuration.<tool> 操作名；连接检测、结构发现和模板试跑保留同一身份。Token 创建和撤销也写入审计，沿用现有 OTLP Logs 上报。不记录凭证、完整定义、查询文本、参数和结果。配置 Token 仅保存哈希，数据库凭证及语义／本体内容沿用原有加密存储。

## 起始提示词

> 请使用 ContextGate 配置 MCP 帮我配置数据源、语义目录、查询模板和本体映射。先调用 get_configuration_guide 并检查已有配置。缺少只读凭证或业务定义时向我询问，保留无关配置。校验本体后请管理员发布，再检查映射并试跑启用的模板。最后给出变更摘要、当前修订号和供管理员审核发布的链接。

HTTP API 数据源也可以通过配置 MCP 创建和维护，查询工具为 `query_http_api`。固定请求契约、只读声明、API 版本与参数配置见 [HTTP API 数据源](http-api.zh-CN.md)。
