# ContextGate 品牌

[English](README.md) · [项目说明](../../README.zh-CN.md)

**ContextGate**

Agent 语义数据网关

让 Agent 理解业务，安全查询数据。

![ContextGate Logo](contextgate-logo.svg)

Context 代表业务术语、指标、共享本体、数据源映射和查询模板；Gate 代表授权、只读执行、查询限制、发布验证和审计。MCP 作为接入协议展示，数据库清单作为连接能力展示。产品不提供知识图谱实例存储或事实推理引擎。

## Logo

开放的几何 C 形代表入口，中央节点代表业务概念，水平路径代表受控的数据连接。主色为深青 `#183F43`，强调色为青绿 `#32B6A0`，背景为暖白 `#F3F8F6`。

- [完整 SVG Logo](contextgate-logo.svg)、[独立图标](contextgate-mark.svg)、[单色图标](contextgate-mark-mono.svg)、[README 横幅](../images/banner.svg)。
- [imagegen 原始设计稿](contextgate-concept.png)与[生成提示词](logo-prompt.md)。SVG 为根据设计稿整理的小尺寸生产版本。

保留至少一个节点直径的留白，保持比例和对比度。图标建议不小于 20 px，同时作为浏览器 favicon；小尺寸省略定位文字。SVG 字标采用查看设备上可用的无衬线字体。应用图标源文件为 `web/public/contextgate-mark.svg`，文档副本需与其同步。

## 改名与兼容

当前源码构建、Docker 镜像与后续发行包采用 `contextgate` 主命令，并保留等价的 `mcpdbhub` 兼容入口。MCP 服务标识与新生成的客户端配置名使用 `contextgate`；已有客户端配置可以保留旧名称。

已有 `MCPDBHUB_*` 环境变量、Compose 服务与数据卷、PostgreSQL 标识、加密标识、浏览器偏好和 `mcpdbhub.audit.*` 遥测属性不变。新增或留空的 OTLP 服务名默认为 `contextgate`，已保存的服务名不自动修改。

GitHub 仓库和 Go 模块已更名为 `github.com/SamuelSupe/contextGate`，请将本地 Git remote 更新到新地址。提交历史和历史发行版继续保留，旧版本的下载文件名及行为不变。

[实际构建与浏览器验证记录](../verification/contextgate-brand.json)。本轮验证覆盖 OrbStack Linux arm64 和本机 Chrome 的品牌改动，不代表重新执行全部数据库矩阵或 amd64 发行验证。
