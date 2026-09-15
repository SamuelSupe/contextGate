# Security policy

Security fixes target the [latest stable release](https://github.com/SamuelSupe/contextGate/releases/latest), currently 0.7.0. ContextGate is a self-hosted, single-instance service with named administrator accounts. Super administrators manage accounts and global settings; all administrators share business configuration. See the [account permissions](docs/administrators.md). Use database accounts with the narrowest privileges required and HTTPS for remote access.

Report authorization bypasses, credential disclosure, read-only boundary escapes or unsafe metadata fetching using [GitHub private vulnerability reporting](https://github.com/SamuelSupe/contextGate/security/advisories/new). Do not publish live credentials or sensitive query data in an issue. Include the affected ContextGate/database versions, a minimal reproduction using disposable data, impact and relevant safe error codes.

Connectivity is separate from privilege evidence. InfluxDB 3 Core uses query API isolation; its underlying admin token is not a database read-only credential. HTTP API operations are administrator-declared read-only; a fixed GET or POST contract does not prove that the upstream service has no side effects. See the [HTTP API boundary](docs/http-api.md) and [support matrix](docs/support-matrix.md) for adapter-specific protection and limitations.
