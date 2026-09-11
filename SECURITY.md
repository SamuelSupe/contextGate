# Security policy / 安全反馈

Security fixes target the latest 0.1.x release. This is a self-hosted, single-administrator service; use database accounts with the narrowest privileges required and HTTPS for remote access.

Report authorization bypasses, credential disclosure, read-only boundary escapes or unsafe metadata fetching using [GitHub private vulnerability reporting](https://github.com/SamuelSupe/mcpdbhub/security/advisories/new). Do not publish live credentials or sensitive query data in an issue. Include the affected Hub/database versions, a minimal reproduction using disposable data, impact and relevant safe error codes.

Connectivity is separate from privilege evidence. InfluxDB 3 Core uses query API isolation; its underlying admin token is not a database read-only credential. See the [support matrix](docs/support-matrix.md) for adapter-specific protection and limitations.

安全修复面向最新 0.1.x 版本。越权、凭证泄漏、只读边界绕过和元数据请求风险请通过上面的 GitHub 私密渠道反馈；提供版本、脱敏复现步骤和影响，不要在公开 Issue 中提交凭证或敏感数据。远程访问需配置 HTTPS，数据库账号应按实际范围授予最小权限。
