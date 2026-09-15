# Implementation tracking

Approved scope: 18 products; Go MCP HTTP + stdio bridge; encrypted PostgreSQL configuration;
named administrators with super/admin roles; per-Agent datasource grants; Fosite OAuth; compact bilingual React UI (English default).
InfluxDB 3 Core explicitly permits query-API isolation instead of a read-only account.

## Work

- [x] Configuration, authentication, query execution, MCP and stdio
- [x] Multiple administrators, personal Configuration MCP identities and attributable audit (0.6.0)
- [x] Open-source database adapters and discovery
- [x] Cloud warehouse preview adapters, UI, templates and bilingual guides (0.7.0); real vendor validation pending
- [x] Fosite OAuth, consent, refresh, revocation, client metadata
- [x] Embedded React management UI
- [x] OrbStack real database matrix and security behavior checks
- [x] Chrome functional and visual verification
- [x] Docker packaging and bilingual documentation

No product compatibility is counted as verified until its recorded runtime checks pass.

Validation detail and remaining deployment combinations: [validation.md](validation.md).
