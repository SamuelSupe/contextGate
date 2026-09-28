# 管理员指南

[English](admin-guide.md) · [文档首页](README.zh-CN.md) · [用户指南](user-guide.zh-CN.md)

本指南面向负责部署 ContextGate、接入数据源、发布查询、授予 Agent 权限和维护服务的管理员。Web 控制台用于管理；只使用查询 Agent 的同事，获得接入信息后阅读[用户指南](user-guide.zh-CN.md)即可。

控制台默认英文，可在 **Settings → Language → 简体中文** 切换当前浏览器的语言；业务定义和查询结果保留原始内容。

## 配置第一个可用查询

新实例按以下顺序操作：

1. **安装并初始化。** 按[安装指南](install.zh-CN.md)准备 PostgreSQL 元数据库与持久化密钥目录，使用设置码创建首个超级管理员。
2. **接入并发布。** 按[管理员首次配置](getting-started.zh-CN.md)，从 **首页 → 新建查询** 开始：选择使用专用只读账号的来源，提供已有查询，说明参数，试跑并审阅发布。共享本体为可选项。
3. **授予查询权限。** 创建查询 Agent，选择来源和有效期。按 [Agent 授权与 OAuth](agent-access.zh-CN.md)选择凭证方式，提供客户端配置。
4. **确认实际使用。** 让用户从真实客户端执行已发布查询，在工作区核对所选 Agent 与当前模板执行版本的调用。管理员预览不能作为客户端已接入的证据。

[三个查询的客服示例](../examples/query-publishing/README.zh-CN.md)演示了不需要本体的小规模配置。已有实例可直接按下方任务查阅。

## 接入数据源与发布查询

| 任务 | 文档 |
| --- | --- |
| 选择支持的产品、准备数据库账号 | [支持矩阵](support-matrix.zh-CN.md) · [只读账号](read-only-accounts.zh-CN.md) |
| 发布已有查询、继续未完成的配置 | [查询发布](query-publishing.zh-CN.md) |
| 维护业务定义、参数和试跑证据 | [语义目录与模板](semantics.zh-CN.md) · [示例目录](../examples/semantics/) |
| 在不同来源复用业务概念 | [本体与映射](ontologies.zh-CN.md) · [零售示例](../examples/ontologies/retail-demo/README.zh-CN.md) |
| 配置固定 JSON API 操作 | [HTTP API 数据源](http-api.zh-CN.md) |
| 配置尚无真实云环境验证的预览适配器 | [云数仓](cloud-warehouses.zh-CN.md) · [示例](../examples/cloud-warehouses/README.md) |
| 让可信 Agent 准备配置 | [配置 MCP](configuration-mcp.zh-CN.md) |

查询 Agent 按数据源授权；表、字段和行范围由数据库账号、视图及原生权限决定。来源配置变更可能立即生效，语义草稿需要显式发布。“连接成功”和“只读账号已验证”是不同证据。

## 管理访问与核对业务结果

| 任务 | 文档 |
| --- | --- |
| 添加管理员、分配角色或恢复密码 | [管理员账号](administrators.zh-CN.md) |
| 签发、暂停、轮换、撤销查询权限，管理 OAuth 客户端 | [Agent 授权与 OAuth](agent-access.zh-CN.md) |
| 以 Agent 身份预览、查看实际客户端活动 | [接入验证与评估](agent-workflows.zh-CN.md) |
| 保存业务问题并人工评审答案 | [评估流程](agent-workflows.zh-CN.md#验证业务问题的效果) · [试点工作表](query-pilot.zh-CN.md) |

控制台只有**管理员**和**超级管理员**两种角色，都能管理共享业务配置；超级管理员额外负责账号、全局设置和安全审计。没有独立的普通查询用户控制台角色，也没有按管理员划分的资源隔离。分配账号前请查看[权限表](administrators.zh-CN.md#权限)。

三类凭证分别用于不同入口：

| 凭证 | 用途 | 权限边界 |
| --- | --- | --- |
| 管理员会话 | Web 控制台和管理 API | 具名管理员角色；写操作需要 CSRF 保护 |
| 个人配置 MCP Token | 可信配置 Agent 访问 `/mcp/config` | 全部来源连接及共享草稿；不能发布或授予查询权限 |
| 查询 Agent Token / OAuth 授权 | 通过 `/mcp` 查询数据 | 已授权来源、查询模式、已发布模板和执行限制 |

## 部署维护与恢复

| 任务 | 文档 |
| --- | --- |
| 设置运行变量、远程 HTTPS 或升级 | [安装部署](install.zh-CN.md) · [0.8.0 升级说明](releases/0.8.0.zh-CN.md) |
| 审阅变更、检查来源健康与模板回归 | [运维指南](operations.zh-CN.md) |
| 备份元数据，使用匹配密钥验证恢复 | [备份与恢复](operations.zh-CN.md#pg-诊断与备份恢复) |
| 将审计发送到可观测平台 | [OTLP 审计上报](audit-export.zh-CN.md) |

ContextGate 以单实例运行，管理员共享业务配置。备份必须同时保留 PostgreSQL 元数据及其匹配的加密密钥。[架构说明](architecture.zh-CN.md)介绍执行与安全边界；开发、验收和发行资料集中在[贡献指南](../CONTRIBUTING.md)。

## 向用户交付接入信息

提供可访问的 `/mcp` 地址、所选客户端的配置、查询凭证或 OAuth 同意安排、可访问来源，以及一个包含示例参数和业务注意事项的已发布查询。Token 只显示一次，请通过团队的凭证交付渠道传递。

附上[用户指南](user-guide.zh-CN.md)，用户即可完成接入、发现、执行及反馈请求 ID，无需先学习安装和账号管理。业务答案正确性应与工具调用成功分别核对。
