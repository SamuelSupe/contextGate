# OTLP 审计日志上报

[English](audit-export.md) · [README](../README.zh-CN.md)

以**超级管理员**身份进入 **Settings → Audit log export**，把已持久化的查询、配置变更和账号安全审计事件以标准 OTLP Logs protobuf 上报到 OpenTelemetry Collector 或其他兼容接收端，支持 HTTP 和 gRPC。默认关闭，本地审计继续保留。

## 配置

1. 选择 **OTLP HTTP / protobuf** 或 **OTLP gRPC**。
2. HTTP 地址示例为 `http://collector:4318/v1/logs`；仅填写 origin 时自动补 `/v1/logs`。gRPC 使用 `http://collector:4317`，必须包含端口，不能包含路径。两种协议均用 `https://` 开启 TLS。
3. **Service name** 默认 `contextgate`。认证通过 **Headers (JSON)** 配置，例如 `{"Authorization":"Bearer <TOKEN>"}`。Header 使用现有 AES-256-GCM 主密钥加密，API 不回显。留空保留已存内容，填写 JSON 替换全部内容，选择 **Clear stored headers** 清除。
4. 私有 HTTPS 接收端可填写 **CA certificates (PEM)**，在系统信任根之外追加 CA。证书及主机名校验始终开启；当前不支持客户端证书认证。地址不能包含凭证、查询参数或 fragment；HTTP 不跟随重定向。
5. 点击 **Send test log**，使用当前表单发送一个 `mcpdbhub.audit_export.test` 合成事件，不保存配置、不回放历史审计。成功表示接收端接受了事件，不证明其下游存储完成。
6. 开启并点击 **Save export settings**。执行查询后，查看 **Delivery status** 中的待发送、接受、拒收计数和重试状态。计数在配置变更之间累计，不包含合成测试事件。

连接由 Go 服务发起。Docker 内的 `localhost` 指 ContextGate 容器自身，请使用共享网络中的 Collector 服务名或服务端可访问的地址。

## 运行本地 Collector

[示例配置](../examples/otel-collector.yaml)使用 debug exporter 打印解码后的日志，已用 Collector **0.160.0** 实测：

```sh
docker network create hub-telemetry
docker run --rm --name collector --network hub-telemetry \
  -p 127.0.0.1:4317:4317 -p 127.0.0.1:4318:4318 \
  -v "$PWD/examples/otel-collector.yaml:/etc/otelcol/config.yaml:ro" \
  otel/opentelemetry-collector:0.160.0 --config=/etc/otelcol/config.yaml
```

宿主机运行的 ContextGate 可使用 `http://127.0.0.1:4318/v1/logs` 或 `http://127.0.0.1:4317`。容器化 ContextGate 加入 `hub-telemetry` 后使用 `http://collector:4318/v1/logs` 或 `http://collector:4317`。生产环境将 Collector exporter 改为实际观测平台。

## 字段与隐私

Resource 包含 `service.name`、`service.version`、持久化的 `service.instance.id`。时间保留审计事件原始时间；完成及待确认意图为 INFO，失败为 ERROR，正文为固定的完成、待确认或失败说明。意图记录使用 `error_code=operation_pending`，通过相同 `request_id` 关联最终结果。

日志属性以 `mcpdbhub.audit.` 为前缀，包括 `id`、`request_id`、`agent_id`、`source_id`、`operation`、`template_id`、`template_version`、`query_fingerprint`、`elapsed_ms`、`rows`、`preview`、`error_code`、`native_code`。缺失的字符串属性不发送。超出 2,048 字符的字符串会截断，并设置 `attributes_truncated=true`。`event.name` 为 `mcpdbhub.audit`。0.2.0 起，模板执行和试跑包含模板 ID；已发布模板执行还包含执行版本。0.3.0 起，已发布模板执行还包含请求开始时采用的 `ontology_id`、`ontology_version`。本体定义及语义全文不写入日志。完整类型说明见[英文字段表](audit-export.md#log-fields)。

上报包含已持久化的查询、经执行引擎的结构发现、管理员预览、连接检测、管理变更意图及结果，以及已记录的登录失败和凭证变更等账号安全事件。它不是 HTTP 访问日志，不声称捕获每一条被拒绝的请求。不发送完整查询、参数值、查询结果、数据库凭证或上报认证 Header；状态信息不回传接收端原始错误正文。

## 发送语义

- 异步按 PostgreSQL 审计 ID 顺序发送，每批最多 64 条、512 KiB；空闲时每两秒检查一次。单次请求超时十秒，响应最多 64 KiB。接收端故障不进入查询请求的同步执行路径。
- 首次启用和重新启用均从当前尾部开始，不上报历史或关闭期间的事件。关闭会取消发送并清除待上报进度，本地审计不删除。修改已启用的目标会保留积压，随后发往新目标。
- 配置和确认进度一起加密持久化；待发送事件及重试状态在重启后恢复。确认丢失、保存进度失败或发送中切换配置可能产生重复；接收端可按 `service.instance.id` 加 `mcpdbhub.audit.id` 去重，不保证恰好一次。
- 临时网络或服务错误采用指数退避及随机抖动，并遵循接收端重试提示，最长等待 24 小时。HTTP 重试状态为 429、502、503、504；gRPC 按 OTLP 状态和 RetryInfo 规则处理。
- 部分拒收会计数，不重试该批。永久错误拒收当前批，并阻止后续上报，直到管理员修正并保存配置。原始本地审计仍保留；保存后继续后续待发送事件，不回放已拒收批次。遵循 [OTLP 规范](https://opentelemetry.io/docs/specs/otlp/)。
- 队列共用本地审计的 30 天保留期，超期的未发送事件也可能被清理，不保证无限期送达。备份需要同时保存配置数据库和匹配的主密钥。

## 管理 API

`GET /api/settings/audit-export` 返回脱敏配置和实时状态；`PUT` 同一路径保存；`POST /api/settings/audit-export/test` 发送合成日志。均要求超级管理员会话，修改及测试还要求 `X-CSRF-Token`；Agent Token 无权访问。

提交 GET 返回的 `config.revision`，旧版本返回 409。省略 `headers` 保留原值，`{}` 或 `clear_headers: true` 清除。无效配置返回 400。测试返回 `accepted`、`message`、`checked_at`，HTTP 200 本身不代表接收成功。请求示例见[英文说明](audit-export.md#administration-api)。

ContextGate 改名保留已有的服务名配置，`mcpdbhub.audit.*` 属性和事件名继续保持稳定，已有 Collector 规则与仪表盘无需迁移。

### 管理员归属

保留 `mcpdbhub.audit.agent_id` 原语义；同一前缀下新增 `administrator_id`、`administrator_username`、`actor_type`、`configuration_agent_id` 和 `channel`；`event_kind` 表示事件类别，`resource_id`、`revision`、`submitted_fields` 记录资源、版本和提交字段类别，不包含字段值，也不是值级差异。Agent 身份预览同时保留实际管理员及执行所用 Agent。安全事件使用 `event_kind=security`，仅超级管理员可在管理 API 查看或配置上报。Collector 接收安全及业务/查询事件，应相应控制读取权限。旧审计不追溯归属。不导出密码、Token、定义、查询参数或结果。
