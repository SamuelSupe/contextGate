# HTTP API 数据源

[English](http-api.md) · [文档目录](README.zh-CN.md)

HTTP API 数据源让 JSON REST API 复用现有的 Agent 授权、限流、审计、语义目录、查询模板和本体映射。自 ContextGate 0.5.0 起提供。

## 配置流程

进入 **数据源 → 添加数据源 → 数据源类型 → HTTP API**：

1. 设置基础 URL、认证方式和 API 契约版本。HTTPS 必须校验证书和主机名，可配置自定义 CA；HTTP 需要明确禁用 TLS。URL 不允许包含凭证、查询参数或片段。
2. 添加有稳定 ID 的 GET 或 POST 操作，声明其只读取数据且无副作用。GET 同样需要声明，方法名不能证明只读性。
3. 设置标量参数、目标位置、必填性、默认值、枚举和数值范围，填写连接检测所需的真实示例参数。
4. 设置响应数据位置，例如 `/data/items`；留空保留完整 JSON。数组逐项返回，对象作为一个文档，HTTP 204 返回空结果。
5. 可选声明响应字段和游标分页，选择连接检测操作，保存并检测。

认证支持无认证、Basic Auth、Bearer Token、以及 `X-API-Key` 等非路由用途的 `X-` 请求头。密钥填写在“访问令牌”中，API Key 请求头留空即使用 Bearer。凭证加密存储，读取配置时不返回。不要在路径、说明、请求体、默认值或示例中保存凭证。

**只读性由管理员声明。** ContextGate 限制固定请求契约，无法证明任意上游业务操作不产生副作用。请使用只读凭证并由接口负责人确认操作；不要把通用写入、脚本执行、SQL 执行或远程抓取接口配置为读取操作。连通成功不代表权限已验证。配置者属于受信任管理边界，固定地址可以是内网服务。

## 参数和结果

Agent 使用 `query_http_api`，只提交 `source_id`、`operation`、`named_params`，以及可选的查询上限与游标，不能指定地址、方法、请求头或请求体片段。

| 参数位置 | 支持的绑定 |
| --- | --- |
| query | 固定查询参数键，值单独编码 |
| path | 整个必填路径段，例如 `{customer_id}`；拒绝斜杠、百分号编码和目录穿越 |
| body | 固定 POST JSON 中已存在的标量值，例如 `/filter/region` |

支持 string、integer、number、boolean；不能传入对象或数组替换操作结构。不进行字符串插值。请求体以 JSON 文本存储，数值无损解析；结果中的 JSON 数字按现有约定编码为精确字符串，文档结构保留。

每个数据源最多 40 个操作，每个操作 32 个参数、200 个声明字段。配置最多 256 KiB，单参数值 8 KiB，分页 Token 4 KiB；同时遵守现有超时、响应大小、行数和并发上限。429 返回 `rate_limited` 与 `HTTP 429`，不自动重试，也不暴露上游错误体。

## 分页与结构发现

游标分页固定一个查询参数键和响应 JSON Pointer；末页必须在该位置返回 null 或空字符串。不跟随分页 URL、HTTP 重定向或环境代理。外层加密游标绑定身份、数据源修订、查询参数和模板执行版本，5 分钟过期。

上游响应超过字节上限会报错；本地行数截断时不返回下一页游标，以免跳过未返回的数据。请调小上游每页数量后重新查询。页码和 offset 可配置为普通参数，不自动递增；暂不支持 Link Header 或嵌套分页。

结构发现使用 `api` namespace，将操作视为对象，将声明响应字段视为列。操作详情包含参数契约和无损的 `example_json` 示例文本，不包含 URL、请求路径、凭证和请求头，也不会读取业务样本。HTTP 本体映射的字段检查始终标为“未验证”，不把人工声明说成真实模式验证。

## 语义、本体与模板

在 **语义 → 目录 → 导入结构** 导入操作，补充业务说明；本体实体映射到 `api.<操作 ID>`，属性映射到声明字段路径。不会推断实体实例或关系。

在“查询模板”中选择已配置的 API 操作，可自动填入查询、参数定义、示例及结果说明。生成模板包含必填参数，以及已设置示例或默认值的可选参数；其他可选参数保持缺省，避免改变上游行为。如需暴露它们，请先在数据源操作中设置示例或默认值，再检查生成的契约。

模板固定 `operation`，参数绑定 `/named_params/<参数名>`。参考[数据源配置](../examples/http-api/source.json)和[可导入语义文件](../examples/http-api/semantics.json)。完成真实试跑、审查和发布后 Agent 才能执行。“仅模板”模式统一限制 HTTP MCP、stdio 和 Agent 身份预览。

API 契约版本由管理员维护。检测结果 `declared-api-contract:<版本>` 不代表自动发现的服务端版本。上游契约变更后必须更新版本；API 配置、凭证或版本变化会使模板试跑和映射检查过期，需要重新试跑、检查映射并发布。无法自动发现没有更新版本的上游变更。只改数据源名称或查询上限不会使试跑过期。

结果附带现有语义、模板、本体上下文，审计与 OTLP 记录 `query_http_api` 或 `execute_query_template`，不保存请求参数、请求头和结果。

## 本地示例

从仓库根目录运行：

```sh
docker run -d --name contextgate-http-api-demo \
  -p 127.0.0.1:19845:8080 \
  -v "$PWD/examples/http-api:/example:ro" \
  python:3.13-alpine python /example/server.py
```

OrbStack 中的 ContextGate 使用 `http://host.docker.internal:19845`，宿主机进程使用 `http://localhost:19845`。示例无认证、无写入路由。按 source.json 配置两个操作，或用配置 MCP 的 `create_data_source` 传入 `configuration`。在语义页面导入 semantics.json，完成试跑与发布。

`list_customers` 携带 `{"region":"east"}` 返回 Acme（精确 ID 为 `9007199254740993`），下一页为 Northwind。POST `/customers/search` 的查询结果等价。

首版不包含出站 OAuth 流程、任意请求头、Cookie、OpenAPI 导入、XML、二进制响应、流式/SSE。
