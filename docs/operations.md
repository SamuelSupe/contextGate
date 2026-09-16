# Operations and safe changes

[简体中文](operations.zh-CN.md)

This guide describes the current source tree; use the documentation at your release tag when running an older build. The service uses PostgreSQL metadata and an independent encryption key. Settings and `contextgate version` show the build commit; local Go builds also mark a modified checkout when VCS metadata is available.

## Review before publishing

**Data source → Semantics → Review and publish** combines validation, entry changes, explicitly linked queries and current Agent grants. It can run a missing template trial (the example and all configured regression cases) or check mappings without leaving the review. Failed or stale evidence blocks publication. The final action rechecks the draft revision and evidence; concurrent edits require reloading the review.

The review distinguishes business-context changes from query execution changes. Description-only changes retain execution versions and running queries. Changing or revalidating a published execution definition follows the existing cancellation/version rules. Linked-query impact uses explicit catalog and ontology references; it does not infer complete dependencies from native query text. The listed Agents have active source grants, which does not prove that they have used a particular template.

The source connection editor also explains live impact before saving connection, credential, enablement or query-mode changes. Connection changes invalidate template evidence. Name, description and limit edits do not invalidate it; execution still observes current limits.

## Administrator responsibilities

All administrators share business configuration. Super administrators additionally manage accounts, global settings, health-check policy and OTLP. Account/password/personal Configuration MCP token events appear in the restricted **Account security** audit category. See [administrator accounts](administrators.md) for onboarding, personal tokens and upgrade behavior.

## Focus audit activity

**Audit log** starts with **Real Agent calls**. **Manual previews** includes administrator and selected-Agent previews; **System checks** contains new background/manual health-discovery events; **All activity** also exposes management changes. Combine these views with administrator, configuration identity, entry point, source, caller, result and time filters. Account-security events are visible only to super administrators. Health checks never count as successful Agent onboarding.

System-discovery events use `event_kind=system`, including the existing OTLP Logs attribute. This classification applies to newly recorded events; historical records retain their original classification. Event content and retention remain unchanged, and no query text or results are added to audit records.

## Review and recover configuration changes

**Audit log → Event type → Management changes** includes source configuration, query Agent grants and credentials, ontology and semantic edits/publications, OAuth client administration and operational settings. Records contain the administrator identity, resource ID, operation, revision, submitted field categories, time and outcome. Categories describe submitted fields, not a value-level diff. Query text, business definitions, parameter values, passwords and results are excluded. Management events use the existing 30-day retention and OTLP Logs export.

An intent is durably recorded before a configuration handler runs. If this fails, the change is rejected. A second record reports completion or failure. The UI hides an intent after its outcome arrives; OTLP receives both with the same request ID. A crash or failed outcome write leaves **Outcome pending**, not a false success. Check the current resource before retrying; the Health page highlights intents without an outcome after three minutes.

**Data source → Semantics → Publication history** lists immutable encrypted snapshots. Select a version to compare entry changes, native definitions and test contracts with the current draft. **Restore as draft** requires confirmation and the current revision. It replaces only the saved draft. Enabled templates must pass new trials, and mappings must be checked again before publication. Agent calls continue using the current publication throughout this process.

Restoring the same execution definition retains its execution version when published; actual query changes follow the existing cancellation and cursor rules. Trial results do not become query results or Agent instructions. Historical snapshots retain references to ontology versions, so those definitions cannot disappear before recovery. Delete an obsolete historical snapshot explicitly to release its reference; the current publication cannot be deleted. Source deletion atomically removes its history. Upgrading an older store can retain only its current publication, marked **Retained during upgrade**; earlier versions cannot be reconstructed.

## Check health

**Health** brings together source connection evidence, changes to published object metadata, template validation, credentials expired or expiring within seven days, and OTLP delivery errors. Refresh reads saved evidence; **Check now** performs an actual read-only check. Only super administrators configure **Scheduled checks**. They are off by default, configurable from 5 to 1,440 minutes, and begin on the worker's next one-minute tick when due.

Checks are sequential, respect source execution limits and use at most 30 seconds per source. Each check probes the connection and compares discoverable column/field names and native types for up to 20 explicitly published object references, including ontology mappings. They do not execute query templates, retrieve business samples, enumerate every database object, or prove business correctness. Redis/Valkey, Neo4j and InfluxDB field checks remain unverified because their discovery paths may inspect data; MongoDB evidence covers indexed fields, not all document fields. Coverage and check time are visible. A change includes up to ten added, removed or type-changed fields per object; large or unavailable field details are explicitly marked as limited. Configuration changes mark saved evidence stale; overdue periodic checks are labeled explicitly.

A detected server version change expires template connection evidence through the existing mechanism. A structure difference remains visible until you explicitly **Accept baseline**. Accepting a baseline does not approve a query, revalidate a template or change permissions. Changes to objects outside the published references are not monitored. Checks report stable error codes; they never try a write to test permissions.

## Add template regression cases

Open a query template and expand **Regression checks**. Add up to ten named cases:

- `parameters_json`: a JSON object, bound through the existing parameter contract.
- Optional `min_rows` / `max_rows`: inclusive bounds from 0 to 10,000. Set both to zero for an empty result.
- Optional `columns`: required column names, with an optional exact native type.
- Optional `values`: exact JSON expectations at JSON Pointers within `result.data`. Table rows are arrays (`/0/0`); documents retain their fields (`/0/total`). Use the native lossless encoding for large integers and decimals. String and number values are distinct, and numeric lexemes compare exactly.

The normal example and all configured cases run in one template trial. The current timeout covers the entire trial; **Run all template checks** shares a 120-second batch budget and reports templates not run. A failed check blocks draft publication, while an existing published template remains available. Query results are discarded; only case names, pass/fail codes, counts and duration are recorded in encrypted trial evidence. Case parameters and expectations remain administrator-only and are omitted from Agent semantic details.

Adding or editing a regression contract requires a new successful trial. The contract does not change the native query definition or cancel running calls. Partial/truncated results and cases with a next-page cursor fail as `incomplete_result`; use a bounded query that returns the full tested dataset. This is a deterministic check of administrator-specified expectations, not model grading or automatic business inference. Publication history includes contract diffs for reviewing changes.

## Diagnose PostgreSQL

For super administrators, **Settings → Diagnostics → Run diagnostics** checks the metadata connection, observed PostgreSQL version and TLS transport, key match, schema and pool usage. The copyable report contains no database address, credentials, source names or query data. The key check verifies the loaded key against metadata; it does not prove the key has been backed up. TLS transport evidence alone does not establish certificate verification policy. An unavailable metadata database can prevent administrator login; `/healthz`, container state and service startup logs remain the first checks in that case.

## Back up and verify recovery

Keep the configured key directory on persistent storage, never in `/tmp` or another directory cleared on restart. The PostgreSQL metadata database and its matching `master.key` are both required for recovery; generating a new key cannot decrypt existing records.

Use a dedicated metadata database: `pg_dump` includes the whole configured database. For the bundled Compose deployment, keep the `.env` and deployment configuration in protected storage. Never regenerate its database password on restart. Run the scripts from the directory containing `compose.yaml`:

```sh
bash scripts/backup-metadata.sh /secure/backups/hub-20260914
MCPDBHUB_VERIFY_IMAGE=contextgate:local bash scripts/verify-backup.sh /secure/backups/hub-20260914
```

The destination must be new. The backup contains a consistent custom-format `pg_dump`, the matching 32-byte key (including environment-supplied keys), a build identifier and checksums. A failed backup does not produce a valid final checksum manifest. Keep it on restricted, encrypted storage; it contains recoverable credentials. The scripts do not back up attached SQLite/DuckDB source files or external databases. Avoid key rotation or database reconfiguration during the backup. Use a PostgreSQL client at least as new as the server; the bundled image is PostgreSQL 17.11.

Verification checks the manifest, restores into a new temporary PostgreSQL container on a dedicated internal network, then runs the **same compatible ContextGate image** with `verify-metadata`. This command uses a read-only transaction to validate the key, decrypt each encrypted metadata record, check JSON and count restored records. It does not initialize/migrate storage, start the server, launch exporters, or connect to a source. The script removes only its own temporary database and network. A successful JSON report states `source_connections_opened: 0`. Use `MCPDBHUB_PG_IMAGE` for a compatible PostgreSQL restore image. PostgreSQL dumps are trusted administrator artifacts; verify only backups from your deployment.

For an actual recovery, stop ContextGate; restore the verified dump into a **new empty database** with `pg_restore --exit-on-error --no-owner --no-acl`; provide its saved `master.key` in the configured key directory, or restore the matching `MCPDBHUB_MASTER_KEY`; point `MCPDBHUB_DATABASE_URL` at the restored database and start the matching ContextGate build. Restore the original key ownership and file permissions (0600), with the directory accessible only to the service user. Preserve the old database until recovery is verified. Restoring also restores sessions, Agent/OAuth grants and export progress as of the backup, so review subsequent revocations and OTLP delivery before exposing the recovered service. Then sign in, run diagnostics and explicitly test selected source connections and Agent authorization.

Backup freshness is not monitored by the service. Retain the verification output in your deployment records; a green diagnostics panel is not a restore rehearsal.

## Verification record

See [0.8.0 release validation](validation.md#contextgate-080--2026-09-16) for final CI and archive/download gates. The feature-stage records below retain their original scopes.

[UX fix validation](verification/product-ux-fixes.json) records the follow-up catalog, authoring, single-answer evaluation and responsive checks, including the isolated runtime recovery and remaining browser limitations.

[Product workflow validation](verification/product-workflows.json) records the Home, business catalog, publication review and audit-view checks during the earlier 0.5.0 feature stage, including tests not rerun.

[Earlier operations validation](verification/operations.json) records the previous backup, health and regression rollout, with its separate verification scope.
