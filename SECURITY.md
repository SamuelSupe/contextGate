# Security policy

Security fixes target the latest 0.1.x release. This is a self-hosted, single-administrator service; use database accounts with the narrowest privileges required and HTTPS for remote access.

Report authorization bypasses, credential disclosure, read-only boundary escapes or unsafe metadata fetching using [GitHub private vulnerability reporting](https://github.com/SamuelSupe/contextGate/security/advisories/new). Do not publish live credentials or sensitive query data in an issue. Include the affected ContextGate/database versions, a minimal reproduction using disposable data, impact and relevant safe error codes.

Connectivity is separate from privilege evidence. InfluxDB 3 Core uses query API isolation; its underlying admin token is not a database read-only credential. See the [support matrix](docs/support-matrix.md) for adapter-specific protection and limitations.
