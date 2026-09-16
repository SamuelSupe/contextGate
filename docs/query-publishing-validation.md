# Query-publishing workflow validation

Date: 2026-09-16. This record preserves the feature-stage checks after `89cf20d` / 0.7.0 and the subsequent UX checks included in 0.8.0. For final commit, CI and distribution evidence, see [release validation](validation.md#contextgate-080--2026-09-16). [简体中文](query-publishing-validation.zh-CN.md).

## Implemented scope

- A continuous source → native query/fixed HTTP operation → purpose/parameters/results → trial/full-source review → real client confirmation flow.
- Administrator-scoped resume references, transient preview-to-draft copying, navigation protection and explicit conflict recovery.
- Seven primary navigation entries with existing URLs retained. Ontologies keeps a dedicated primary entry; complete semantic editing remains available from each source.
- Distinct publication, execution and business-review states; a 30-day source usage summary and exact source/template/execution-version/Agent call evidence.
- Bilingual guides and a three-query support demo. The demo does not require an ontology.

No semantic file-format change, metadata migration, new query language, template ACL or public MCP tool was introduced. Existing authorization, templates-only restrictions, encrypted drafts and source-snapshot publication remain the execution boundary.

## Checks performed

| Environment / check | Result |
| --- | --- |
| OrbStack Linux arm64: `go test -race ./... -count=1`, `go vet ./...` | Passed |
| Node 24 in OrbStack: 11 UI behavior tests, TypeScript and Vite production build | Passed; existing bundle-size advisory remains |
| Full live open-source database matrix | 18 products / 20 versions, 135 query/error cases, 104 denied operations; all passed |
| Focused readiness regression | Excluded other sources, templates, execution versions, Agents, previews, administrator/system events, failed calls and activity outside the window; a real MCP call counted, saving a draft did not change its published scope, republishing a new execution version reset that scope |
| PostgreSQL UI workflow | Created a native parameterized query, confirmed parameter type/name, saved, refreshed/resumed, trialled against real fixture data and published after acknowledging another saved draft change |
| HTTP API UI workflow | Selected a configured operation, inherited its parameter/enum contract, saved, trialled, reviewed and published |
| Real HTTP MCP clients | Executed both published UI-created templates; PostgreSQL returned two rows and HTTP API one page. Exact request IDs appeared in readiness. Native SQL on the templates-only PostgreSQL source was denied |
| Concurrent editor recovery | A second API session saved another draft entry while the UI was editing. UI save conflicted and kept local edits; explicit review/merge preserved the other entry and did not change publication |
| Credential-change recovery | Rotated a dedicated sample database credential: all three published templates suspended. Fresh trials alone left them suspended. With unchanged draft content, the UI still opened full review and explicit re-publication restored all three at version 2 |
| Demo | Three PostgreSQL templates passed eight example/regression cases, including empty results and a USD 170.00 gross paid-order amount assertion |
| Restart | Isolated instance restarted with the same PostgreSQL metadata and master key; configuration, session and publication remained usable |
| Local Chrome | English and Chinese, desktop and 390px, navigation, resume, parameter forms, preview, full publication review and keyboard confirmation checked. Chinese narrow query workspace measured 390px content in a 390px viewport; no JavaScript console errors observed |
| Documentation | New/updated workflow entry links checked against local files |

The isolated app ran at `http://localhost:19849`; existing user runtimes were retained. Test evidence is under ignored `artifacts/query-publishing-20260916/` (private settings in that directory must not be published). Matrix result files were copied into its `matrix-results/` directory. Their shared backend implementation digest is `e64ae2a8ba84bb51a83687a02deb93a803a340ea02f90afe6a3f303323caa662`.

## Follow-up UX verification (2026-09-16)

This focused pass simplified the existing flow. It did not rerun the full database matrix listed above.

- OrbStack: all 15 frontend behavior tests passed, including account-scoped resume and allowlisted search-return links. TypeScript and Vite production build passed; the existing bundle-size advisory remains.
- OrbStack: `go test -race ./internal/server -run 'BusinessCatalog|Ontology' -count=1` passed. Catalog regressions cover query deduplication through linked term aliases and mapped ontology definitions, hidden-definition isolation, source authorization and snapshot paging.
- Local Chrome: verified search → detail → workspace → return, including browser Back. The keyword, selected Agent, result focus and a 424px reading position were retained. An expired snapshot required an explicit restart; restarting retained search filters. The available fixture had fewer than 20 results, so a live second page was not exercised in Chrome.
- Local Chrome: checked the Home resume priority, direct health check on an isolated sample source, full-width query details, validation drawer, Escape/focus return, and explicit client configuration wording. A read-only trial launched from the drawer passed against the existing PostgreSQL fixture.
- Local Chrome: entity-header click and Enter opened the inspector; dragging moved a node without opening it. Property editing/cancel returned to the inspector; keyboard connection selected both relationship endpoints without saving. Existing ontology definitions and publications were left unchanged.
- English and Chinese UI checked at desktop and 390px. Query workspace and drawers fit the viewport without horizontal page overflow. Property-type labels remain intact on narrow screens.

No new credential, Agent grant or business publication was created for this UX pass. These are targeted interaction checks, not a full accessibility audit or a new distribution validation.

## Final catalog and client UX checks

- OrbStack: all 17 frontend workflow tests and two lossless request tests passed, followed by TypeScript/Vite production build. Full `go test ./... -count=1`, `go test -race ./... -count=1` and `go vet ./...` passed for release preparation.
- Catalog regressions cover administrator-only draft/attention views, Agent projection denial, summary redaction, disabled or stale templates, source authorization and draft-revision pagination.
- Local Chrome: query names opened the workspace directly; draft and attention views, search-return state, bilingual desktop/390px layouts and keyboard controls were checked. A real HTTP MCP call ended the client wait with the matching request ID and execution version. Home retained multiple account-scoped recent queries.
- Current English release screenshots were captured from the running 0.8.0 UI with owned fixtures; historical screenshots remain tied to earlier records. No query definition, credential or publication was changed for the screenshot pass.

The full database matrix above predates these final UX changes and was not repeated in this pass. Dist and independently downloaded package checks are reported separately with their exact source commit.

## Evidence limits and next decisions

These are implementation and fixture checks, not ten independent user setup attempts, a 30-question customer evaluation, sustained real-Agent reuse or two weeks of measured net benefit. Use the [pilot worksheet](query-pilot.md) to collect those observations. Business acceptance remains a separate source-evaluation review; template ownership and question-to-template review references were not added without a pilot need.

Catalog ranking changes, external imports, periodic query regression scheduling and additional connector work remain conditional follow-ups from the plan. The four cloud warehouse adapters retain preview status because no vendor environment was available. At the feature stage, dual-architecture distribution and GitHub download checks had not yet run; their subsequent 0.8.0 records are linked from release validation. No new backup/restore drill was performed for this UX change.

**中文摘要：** 已验证连续发布、断点恢复、冲突合并、完整草稿确认、精确客户端调用证据，以及凭证变更后的重新试跑与发布恢复。真实业务试点与维护收益不包含在本次结果中，0.8.0 发行包验证另附具体提交及报告。
