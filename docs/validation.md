# Validation and reproduction

[简体中文](validation.zh-CN.md)

The records below describe checks performed on 2026-09-11. Database integration tests ran in OrbStack Linux arm64; the administration UI was exercised in local Chrome without adding a browser automation framework. Historical regression reports retain the source digests they actually tested.

## v0.1.1 release scope

v0.1.1 adds OTLP audit log export. The [OTLP checks below](#otlp-audit-export--2026-09-11-after-v010) cover real Collector delivery, failure recovery, administrator boundaries and the English UI. Release-specific native CI and independently unpacked archive results are attached as [VALIDATION.json](https://github.com/SamuelSupe/mcpdbhub/releases/download/v0.1.1/VALIDATION.json). PostgreSQL adapter/MCP checks were rerun for package validation; the full 18-product/20-version matrix remains the historical v0.1.0 evidence and was not rerun for this export-only change.

## v0.1.0 release verification

All **18 products / 20 version combinations** passed, with **132 query/error cases and 89 rejected-operation cases**. Network databases were exercised through their native adapters and MCP HTTP; SQLite/DuckDB used real file engines and MCP. The [machine-readable matrix](verification/matrix.json) identifies the implementation digest and individual reports.

The published release also passed native Linux amd64 and arm64 [GitHub CI](https://github.com/SamuelSupe/mcpdbhub/actions/runs/34586762536). Both actual Linux distribution archives were independently extracted and run in clean Debian containers. After upload, all four assets were downloaded from GitHub and matched their local hashes; both downloaded archives passed the 16-check installation workflow. Local amd64 archive execution used OrbStack emulation on arm64. Native amd64 CI did not repeat the full external database matrix.

The [release validation report](https://github.com/SamuelSupe/mcpdbhub/releases/download/v0.1.0/VALIDATION.json) includes the source commit, matrix, CI results and final archive checks. See [releasing](releasing.md) for packaging and publication steps. This English documentation update does not constitute a new database test run.

## Checks performed

| Scope | Observed behavior and result |
|---|---|
| Go tests | `go test ./...` passed, including real SQLite/DuckDB, official MCP HTTP clients and actual stdio bridge subprocesses |
| Race checks | server, engine, oauth and adapter checks passed; release verification also ran `go test -race ./...` |
| Database matrix | 18 products / 20 independent version combinations passed; see [version records](verification/) and [capability limits](support-matrix.md) |
| Read-only execution | DDL/DML, multiple statements, writing CTEs and dangerous functions/scripts/commands were rejected; target data was unchanged |
| MCP per product | Official SDK HTTP calls covered source configuration, queries, discovery, write rejection, Agent isolation, auditing and revocation for each network product; real-file MCP tests covered SQLite/DuckDB |
| Identity and grants | Per-Agent source isolation, metadata denial, token revocation and cancellation of active/queued work on grant changes |
| OAuth | Authorization code with PKCE S256, invalid verifier/resource, redirect mismatch, consent/denial, empty-selection retry, refresh rotation, replay revocation, expiration and revocation |
| Request and credential boundaries | Undeclared connection fields, CSRF, response redaction, encrypted storage, CIMD private-network/redirect restrictions, HTTPS certificate trust, document-ID path injection rejection and oversized Redis RESP lengths rejected before driver allocation |
| Pagination and values | Complete MongoDB/CQL/Search page sequences compared with baselines; Redis scans; cross-identity/query/tampered cursors rejected; search size:0/small pages/unsorted truncation; Neo4j decimals/nested integers; CQL Decimal/float/double/frozen list/map; MongoDB distinct arrays and missing fields |
| Timeout and recovery | A real PostgreSQL long aggregation hit a one-second timeout; connections recovered after cancellation, database stop/restart and source disable/enable; see [recovery](verification/recovery.json) |
| Persistence | Reopening configuration restored credentials, grants and audit records; image initialization and restart recovery passed; see the historical [image record](verification/package.json) and final release report |
| UI build | TypeScript/Vite production build passed; resources were embedded in Go and runtime images had no Node.js |
| Chrome workflows | Setup, login, empty lists, source add/save/test, discovery, parameterized preview, write errors and stale-result clearing, empty search, one-time Agent tokens/revocation, auditing, support/settings pages and OAuth registration/consent/denial/revocation/expiration |
| Visual and keyboard checks | Sidebar, tables and configuration drawer compared with the compact concept; 390×844 navigation/drawer; Escape, focus restoration and Shift+Tab trapping; no application errors observed in Chrome logs |

Privilege evidence is recorded separately from functional verification. Elasticsearch/OpenSearch fixtures enabled HTTPS and security plugins: dedicated reader accounts received HTTP 403 on direct writes, and untrusted certificates were rejected. Neo4j used Community. These observations do not establish every effective production-account privilege or cluster configuration.

Long-running load tests, every type/operator combination, cluster failover, public OAuth-client interoperability, and native macOS/Windows distributions were not validated. Distribution archives require Linux glibc ≥ 2.36. The external database matrix ran on arm64; see the separate native CI and emulated archive scope above.

## Create the OrbStack development container

Run from the repository root. These names are dedicated to this project:

```sh
docker network create mcpdbhub-test
docker run -d --name mcpdbhub-dev --network mcpdbhub-test \
  --cpus 4 --memory 6g \
  -v "$PWD:/work" \
  -v mcpdbhub-gomod:/go/pkg/mod \
  -v mcpdbhub-gocache:/root/.cache/go-build \
  -w /work golang:1.26-bookworm sleep infinity
# Configure a trusted CA in this container first if your proxy requires one.
npm --prefix web ci
npm --prefix web run build
docker exec mcpdbhub-dev go mod download
docker exec mcpdbhub-dev go test ./...
docker exec mcpdbhub-dev go test -race ./internal/server ./internal/engine ./internal/oauth ./internal/adapter
```

Build the frontend before `go:embed` compilation. The Go image provides the CGO toolchain. Fixture scripts also need curl inside the development container to reach isolated HTTP database APIs; the full Bookworm Go image includes it.

## Reproduce the database matrix

```sh
python3 scripts/matrix.py
# Select individual products.
python3 scripts/matrix.py postgres mariadb redis
# SQLite/DuckDB are included by default; run only real-file engines and MCP:
python3 scripts/matrix.py sqlite duckdb
python3 scripts/publish-verification.py
```

The script creates isolated instances labeled `com.mcpdbhub.fixture=true`, without publishing database ports on the host. It initializes, tests and removes its own containers one at a time, leaving existing same-name containers and unrelated workloads alone. Reserve space for images; initial dependency and database-image downloads can take time.

Set `MCPDBHUB_KEEP_FIXTURES=1` to retain instances for diagnosis. Remove the corresponding fixture before rerunning, without cleaning unrelated resources. Failure logs and manifests containing dedicated fixture credentials stay in the Git-ignored `artifacts/matrix` directory. **Publication copies only credential-free passing reports.** Rerunning a product removes its old record first, so a failure cannot reuse earlier passing evidence.

Publication checks the implementation digest and MCP verification marker; stale results from earlier code cannot be published as current. Report checked_at/verified_at values use UTC. ScyllaDB versions come from system.versions; TimescaleDB records both extension and PostgreSQL versions; Valkey reports its own product version instead of a Redis compatibility version.

## Live-service timeout and recovery

Start Hub and retain a PostgreSQL fixture in the isolated environment:

```sh
MCPDBHUB_KEEP_FIXTURES=1 python3 scripts/matrix.py postgres
MCPDBHUB_TEST_URL=http://127.0.0.1:19840 \
MCPDBHUB_ADMIN_PASSWORD='<CURRENT_TEST_SERVER_ADMIN_PASSWORD>' \
python3 scripts/check-recovery.py
```

The script verifies the fixture label, stops/restarts only `mcpdbhub-it-postgres`, and creates/removes one dedicated data-source configuration. It does not modify other sources. Do not use this test against a production database.

## Docker and distribution checks

After rebuilding an image, retain the PostgreSQL fixture and run `python3 scripts/check-package.py`. It checks non-root execution, a read-only root filesystem, no Node runtime, initialization, MCP queries, restart recovery and token revocation, and cleans up its own container and configuration volume. Use `--archive` to validate the actual distribution instead; see [releasing](releasing.md). BuildKit caches dependencies and downloads only modules needed by the target platform. Supply a trusted proxy CA using `--secret id=build_ca,src=/path/to/trusted-ca.pem` when required.

`Dockerfile` has UI-build, Go-build and Debian-slim runtime stages. The runtime UID/GID is 10001. `compose.yaml` uses loopback publishing, a read-only database directory/root filesystem, dropped capabilities and no-new-privileges. The configuration volume remains writable for settings and auditing.

Production deployments require their own public HTTPS URL, least-privilege database accounts and CA configuration. See the [English README](../README.md) or [Chinese README](../README.zh-CN.md).

## Query correctness regressions

The initial matrix reproduced and fixed Search ignoring size and omitting truncation, Cypher converting numbers to strings, CQL Decimal/float/collection binding failures, and MongoDB distinct semantics for arrays and missing fields. Pagination checks now compare all pages with a complete baseline, rather than checking counts alone.

## Product workflows and English UI regression — 2026-09-11

This incremental round covered authorization lifecycle, connection evidence, credential changes, lossless preview, audit diagnostics and English configuration flows. It did not rerun all 20 version combinations.

- OrbStack `go test ./...` and `go test -race ./...` passed. Regressions covered legacy revocation migration, pause/resume, token rotation and old-token rejection, permanent revocation, authentication changes/credential clearing, connection failure/recovery, request-ID auditing, Agent preview isolation and error redaction.
- The existing `node --test web/test/query.test.mjs` tests confirmed that initial and continuation requests preserve raw integer/Decimal JSON number text. The English TypeScript/Vite build passed.
- Real database/MCP checks passed for PostgreSQL 17.11, MongoDB 7.0.39, Redis 7.4.6, Elasticsearch 8.19.17, OpenSearch 3.2.0, SQLite 3.53.4 and DuckDB 1.5.5. Search TLS trust and reader-account write denial were rechecked through structured `database_permission` / `HTTP 403` errors.
- Chrome kept a failed-password configuration open with a diagnostic; correcting it recovered without duplicating the source. Stopped databases changed checks to failed, then recovered after restart. `9007199254740993` and `0.1234567890123456789012345` survived UI submission unchanged. Discovery, table results, MongoDB native paging, cancellation and subsequent queries passed.
- A Chrome-created token completed a real MCP call. Pause returned 401, resume restored access, rotation rejected the old token while accepting the new one, and permanent revocation remained effective after grant edits. Connect reopened with real client activity. An Agent without grants could not preview structure.
- Request-ID audit filters showed native codes and suggested actions; empty states worked. The 390×844 configuration view had no horizontal overflow. InfluxDB 3 defaulted to port 8181. Escape restored focus and browser logs showed no application errors.
- The final image passed non-root/read-only-root/no-Node checks, setup, MCP, configuration/session restart recovery and revocation. The historical [package record](verification/package.json) records image verification; the local service was updated to the tested binary at that time.

Temporary sources, database containers and token files were removed. Revoked test Agents and audit records were retained; existing source contents were not changed.

## Second product regression round — 2026-09-11

Six fixes covered stale Agent edits overwriting grants, dangling grants after source deletion, renaming interrupting queries, incorrect InfluxDB examples/diagnostics, missing OAuth client lifecycle management and local administrator-password recovery.

- OrbStack full Go and race checks passed. Regressions covered grant revision conflicts, deletion/migration, running queries/cursors during renaming, version-specific InfluxDB discovery, invalidation after OAuth redirect changes/disable/rotation/deletion, legacy OAuth Agent associations, client isolation and recovery preserving source/Agent credentials.
- PostgreSQL 17.11, InfluxDB 1.8.10 / 2.7.12 / 3.11.2, SQLite 3.53.4 and DuckDB 1.5.5 passed real-engine/MCP checks: 28 query/error cases, 27 rejected-operation cases and unchanged data. [Lifecycle records](verification/lifecycle.json) retain this round's implementation digest; it was not a full 20-version rerun.
- Two Chrome tabs reproduced a stale authorization edit. After the second tab removed grants, the first tab's stale save was rejected and reload showed current grants. Migration removed missing-source references while preserving revocations, allowing Agent edits to save.
- Chrome exercised OAuth registration, one-time credentials, retained input on invalid redirects, redirect editing, disable, secret rotation, re-enable, empty search and deletion. Actions were visible without overlap at 390×844; desktop and narrow layouts were visually checked.
- A real InfluxDB 2 source generated Flux using its configured bucket and returned six records. InfluxQL produced `InfluxDB 2.x requires language flux` rather than a misleading permission diagnostic. Settings included English recovery instructions and browser logs showed no application errors.
- TypeScript/Vite and existing numeric-parameter checks passed. The Linux arm64 image additionally passed stopped-service stdin password recovery, rejection of old passwords/sessions, new-password login and continued queries with original database/Agent credentials.

The tested binary replaced the local service at that time. Temporary OAuth clients, sources and containers were cleaned up; existing configuration, revocations and auditing were preserved. Password recovery used an isolated volume and did not change the original local administrator password. Both [English recovery instructions](../README.md#recover-a-forgotten-administrator-password) and [Chinese instructions](../README.zh-CN.md) are available.

## Third query and authorization regression round — 2026-09-11

Five fixes covered unchanged Agent edits extending expiration, one Agent's queued requests exhausting global concurrency, valid SQL functions rejected as keywords, discovery silently losing item 1,001, and empty grants crashing the Agents page.

- OrbStack full Go and race checks passed. Regressions covered Agent/source/global concurrency, queue isolation and capacity release on cancellation, cross-operation cursor rejection, all 1,001 tables, 2 KiB metadata pages, admin API pagination and reading/editing legacy null grants.
- Ten real product/version combinations passed: PostgreSQL, MySQL, MariaDB, TiDB, CockroachDB, TimescaleDB, ClickHouse, SQLite, DuckDB and InfluxDB 3. There were 60 query/error cases and 59 rejected-operation cases, with unchanged data. Nine SQL sources compared complete namespace/table/column paging against baselines. [Query-access records](verification/query-access.json) retain this incremental run's versions and implementation digests.
- MariaDB initially exposed a fixture-startup race: socket queries reached a temporary server about to stop. Waiting for the final TCP server fixed the issue; fresh fixture, adapter and MCP checks passed.
- With 32 controlled slow requests from Agent A, Agent B's independent SQLite query took 2 ms. MCP discovered 1,001 real SQLite tables in 1,000 + 1 pages, ending at `t_1000`. Slow requests used a dedicated HTTP test service; this validates queue isolation, not Elasticsearch performance.
- Chrome preserved `2026-09-12T09:15:00.123456Z` on an unchanged save. Keyboard editing to local `17:16:28` stored `09:16:28Z`. Missing/null grants rendered and edited normally. Narrow 390-pixel layout, Escape and focus restoration passed.
- Chrome's **Next page** reached table 1,001 and discovered its columns. `replace()` / `format()` returned `xbc` / `ok`; `DROP TABLE` was rejected and all 1,001 tables remained. No application errors appeared in browser logs.
- TypeScript/Vite and numeric-parameter checks passed. The final Linux arm64 image passed initialization, HTTP MCP, restart recovery, token revocation and password recovery. The local service on port 19840 was updated while retaining its original session and configuration.

This round used isolated configuration/database files and did not alter the original administrator password, sources or Agent grants. Temporary containers, volumes and token files were removed. Operation-bound cursors invalidate older short-lived cursors on upgrade; clients must restart their first request. SQL metadata pagination has no cross-page snapshot during schema changes.

## Release preparation — 2026-09-11

- After unifying version identifiers and the public Go module path, the full 18-product/20-version OrbStack matrix passed again: 132 query/error cases and 89 rejected-operation cases. [matrix.json](verification/matrix.json) and the individual records carry the implementation digest.
- Full Go and race checks passed, including real SQLite/DuckDB, HTTP MCP, stdio, OAuth, revocation and pagination. PostgreSQL/TimescaleDB fixture initialization also waits for the final TCP server, avoiding premature table creation before extensions are installed.
- README images came from actual local Chrome sessions with an isolated configuration volume, real PostgreSQL/SQLite/DuckDB and sample data. Three Agents completed MCP queries, including authorized revenue aggregation. Captures contain no database or Agent credentials.
- Final archive checks, GitHub download verification and platform scope are listed at the top of this page and in the release's validation attachment. Historical records continue to identify their own source digests.

## OTLP audit export — 2026-09-11 (after v0.1.0)

- `go test -race ./...` passed in the OrbStack Go 1.26 build container. Export regressions use real HTTP, TLS and gRPC receivers plus SQLite: protobuf requests, authentication headers, retry hints, partial/permanent rejection, trusted/untrusted certificates, redirects, invalid/oversized responses, bounded caller fields, in-flight cancellation and pending records across restart.
- An administrator/API/MCP integration test verified default-off behavior, Agent exclusion from all export administration routes, CSRF, stale revisions, invalid destinations/headers, one-way credential handling and real successful/rejected SQLite query logs without query text, values, results or credentials.
- An isolated OrbStack Hub and official `otel/opentelemetry-collector:0.160.0` exercised HTTP/protobuf and gRPC. Chrome sent a synthetic log and saved each protocol. The Collector decoded INFO/ERROR audit events with the expected resource, correlation and typed count/duration fields.
- Stopping the Collector left database queries working (approximately 4 ms in this small local fixture). Three pending events survived Hub restart and were exported after Collector recovery. The receiver ultimately held consecutive audit IDs 1–7 with one stable service instance ID, while pending returned to zero and accepted reached seven. These timings demonstrate isolation in this fixture, not a throughput benchmark.
- Local Chrome verified sign-in, the English Settings form, invalid header feedback, HTTP 404 test feedback, save/reload, clearing stored headers, live delivery status, keyboard focus, and desktop/390×844 layout without horizontal overflow. Stored header values disappeared after saving. No application warnings or errors were recorded in the browser console.
- TypeScript/Vite production build passed. Database adapters were unchanged; this feature round did not rerun the 20-version database compatibility matrix or build a new release archive. The [OTLP guide](audit-export.md) describes at-least-once retries, rejection behavior and the shared 30-day retention limit.
