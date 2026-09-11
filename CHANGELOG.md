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
