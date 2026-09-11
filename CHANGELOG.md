# Changelog

## 0.1.0 — 2026-09-11

First public release of MCP DB Hub.

- Native read-only access across 18 database products and 20 independently tested product/version combinations.
- Eleven MCP tools over Streamable HTTP and an authenticated stdio bridge.
- Embedded English administration UI for data sources, Agent grants, audit records, supported databases and settings.
- Individual tokens and Fosite OAuth with PKCE, administrator consent, refresh rotation and revocation.
- Encrypted credentials, database-specific read-only protection, bounded concurrency, cancellation, lossless values and scoped continuation cursors.
- Precise Agent expiration editing, empty-grant handling, isolated query admission and paged SQL metadata discovery.
- Linux arm64 and amd64 distributions with bilingual instructions, dependency notices and SHA256 checksums.

The service is single-instance and single-administrator. SQL/Cypher query pagination is explicit; SQL metadata supports continuation. It does not provide cross-database federation, arbitrary scripts or automatic database account management. See the [support matrix](docs/support-matrix.md) and [validation record](docs/validation.md) for tested versions and limits.

首个公开版本：提供 18 个数据库产品的原生只读 MCP 查询、内嵌英文管理界面、独立 Agent 授权、OAuth、审计和无损结果；交付 Linux arm64/amd64 发行包。实际版本与验收边界见支持矩阵，不把协议兼容视为已验证支持。
