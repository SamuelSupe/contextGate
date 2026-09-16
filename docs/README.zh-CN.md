# ContextGate 帮助文档

[English](README.md) · [项目说明](../README.zh-CN.md) · [下载发行版](https://github.com/SamuelSupe/contextGate/releases/latest)

- [查询发布与恢复](query-publishing.zh-CN.md) · [业务试点工作表](query-pilot.zh-CN.md)

**Agent 语义数据网关：** 连接数据库与 HTTP API，描述业务概念，发布经过验证的查询，为 Agent 授予受控访问权限。

本文档面向 **ContextGate 0.8.0**，包含查询发布、本体概念关联和统一查询工作区；旧版本请查看对应发行标签中的文档。[验收记录](validation.zh-CN.md)区分最终发行验证与历史功能证据。

| 使用目标 | 文档 |
| --- | --- |
| 管理员账号、个人 MCP 身份及升级 | [账号管理指南](administrators.zh-CN.md) |
| Linux / Docker / OrbStack 安装 | [安装指南](install.zh-CN.md) |
| 从数据源配置到 Agent 查询 | [首次使用](getting-started.zh-CN.md) |
| 让 Agent 配置 ContextGate | [配置 MCP](configuration-mcp.zh-CN.md) |
| 从 ContextGate 0.4.x / 0.5.x / 0.6.x / 0.7.x 升级 | [0.8.0 发行与升级说明](releases/0.8.0.zh-CN.md) |
| 从 0.3.x 或更早的 SQLite 元数据升级 | [0.4.0 发行与升级说明（英文）](releases/0.4.0.md) |
| JSON REST API、只读操作与查询模板 | [HTTP API 数据源](http-api.zh-CN.md) |
| 云数仓预览配置 | [Snowflake、Databricks、BigQuery、Redshift](cloud-warehouses.zh-CN.md) · [示例](../examples/cloud-warehouses/README.md) |
| 数据库版本与读取能力 | [支持矩阵](support-matrix.zh-CN.md) · [只读账号](read-only-accounts.zh-CN.md) |
| 术语、指标与原生模板 | [语义目录](semantics.zh-CN.md) · [数据库示例](../examples/semantics/) · [HTTP API 示例](../examples/http-api/semantics.json) |
| 查找查询、管理草稿与关联业务概念 | [业务目录与首次使用](getting-started.zh-CN.md) |
| 实体、属性、关系与独立源映射 | [本体与映射](ontologies.zh-CN.md) · [零售示例](../examples/ontologies/retail-demo/README.zh-CN.md) |
| 客户端、查询预览与评估 | [Agent 工作流](agent-workflows.zh-CN.md) |
| 备份恢复、健康、历史与回归 | [运维指南](operations.zh-CN.md) |
| 发布影响与审计活动分类 | [发布前审阅](operations.zh-CN.md#发布前审核) |
| 审计上报 | [OTLP Logs](audit-export.zh-CN.md) |
| 系统设计与测试证据 | [架构](architecture.zh-CN.md) · [验收](validation.zh-CN.md) |

默认 UI 为英文，在 **Settings → Language → 简体中文** 切换并记住当前浏览器的语言偏好。业务文本和数据不会被自动翻译。

[产品方案与交付状态](product-optimization-plan.zh-CN.md) · [流程验证记录](query-publishing-validation.zh-CN.md)
