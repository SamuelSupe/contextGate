# ContextGate 文档

[English](README.md) · [项目说明](../README.zh-CN.md) · [下载发行版](https://github.com/SamuelSupe/contextGate/releases/latest)

本文档面向 **ContextGate 0.8.1**。使用其他版本时，请查看对应发行标签中的文档。

## 查询用户

管理员已提供现有服务的访问权限时，从 **[用户指南](user-guide.zh-CN.md)** 开始，按顺序完成接入和查询：

1. [接入 MCP 客户端](user-guide.zh-CN.md#接入客户端)。
2. [查找并执行已发布查询](user-guide.zh-CN.md#执行已发布查询)。
3. [填写参数](user-guide.zh-CN.md#填写参数)，[理解结果与翻页](user-guide.zh-CN.md#理解结果与翻页)。
4. 按需使用[原生查询](user-guide.zh-CN.md#原生查询)，或处理[查询问题](user-guide.zh-CN.md#查询遇到问题)。

查询用户通过自己的 Agent 客户端工作，无需数据库密码或 ContextGate 管理员账号。

## 管理员

负责安装服务、向他人提供查询能力或维护权限与运行状态时，从 **[管理员指南](admin-guide.zh-CN.md)** 开始。

| 任务 | 文档 |
| --- | --- |
| 部署服务、准备第一个来源 | [安装](install.zh-CN.md) → [管理员首次配置](getting-started.zh-CN.md) |
| 发布查询、维护业务定义 | [查询发布](query-publishing.zh-CN.md) · [语义目录](semantics.zh-CN.md) · [本体](ontologies.zh-CN.md) |
| 管理身份与访问权限 | [管理员账号](administrators.zh-CN.md) · [查询 Agent 授权与 OAuth](agent-access.zh-CN.md) |
| 确认真实接入与业务答案 | [接入验证与评估](agent-workflows.zh-CN.md) |
| 维护、恢复与审计 | [运维](operations.zh-CN.md) · [审计上报](audit-export.zh-CN.md) |

管理员指南还提供各类来源配置、配置 MCP、示例和用户交付说明。Web 控制台属于管理流程，在预览中选择 Agent 仍是管理员操作。

## 参考与项目开发

[支持矩阵](support-matrix.zh-CN.md) · [架构](architecture.zh-CN.md) · [安全说明](../SECURITY.md) · [0.8.1 发行说明](releases/0.8.1.zh-CN.md)

构建步骤、可复现验收、发行流程和产品交付记录集中在 **[贡献指南](../CONTRIBUTING.md)**，与日常用户和管理员操作分开查阅。
