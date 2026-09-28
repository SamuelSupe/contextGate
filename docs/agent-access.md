# Query Agent access and OAuth

[简体中文](agent-access.zh-CN.md) · [Administrator guide](admin-guide.md)

This guide is for administrators who grant and maintain query access. For connecting an already authorized client, share the [user guide](user-guide.md).

## Create and hand off an Agent

1. Open **Agents → Create Agent**, name the Agent, select its permitted sources and set its expiration.
2. Save the token shown once. Open **Connect**, select the user's client and copy its configuration.
3. Provide the reachable MCP URL, credential, a published query and example inputs to the user.
4. Confirm a real client query in recorded activity. An administrator preview checks permissions but does not prove the user's client connected.

**Connect** can be reopened later for configuration and the last 30 days of client activity; it cannot retrieve the original token. See [setup and evaluation](agent-workflows.md) for source/template-specific evidence.

## Source grants and query protection

The grant model is Agent → data source. Database users, views and native permissions control database, table, field and row visibility. Configure multiple sources with different database accounts when different visibility is required.

Execution combines statement parsing or engine classification, read-only transactions/files, safe function and command lists, and fixed read APIs. It rejects multiple statements, modifying CTEs, DDL/DML, locking reads, imports/exports, extensions and dangerous scripting. Arbitrary custom functions are outside the supported subset. Read-only access does not imply acceptance of every syntactically valid native query; see the [support matrix](support-matrix.md).

Connectivity and privilege evidence are shown separately. “Account permissions unverified” means database-side grants require independent confirmation. InfluxDB 3 Core is explicitly labeled “query API isolation”: its admin token retains database administration privileges; only the adapter's fixed query APIs are available to agents.

Configuration is stored in PostgreSQL. Source credentials use AES-256-GCM; the master key is held in a separate `master.key` file or the `MCPDBHUB_MASTER_KEY` environment variable (standard Base64 of 32 bytes). Administrator passwords use Argon2id; agent and session tokens are hashed. Admin sessions use HttpOnly/SameSite cookies and CSRF checks. Audit records expire after 30 days and exclude results, plaintext parameters and full query text. Grant changes cancel affected work.

Source limits default to 30 seconds, 1,000 rows and 5 MiB. Administrators may configure up to 120 seconds, 10,000 rows and 20 MiB; query Agents can only tighten those limits.

## Token lifecycle

In **Agents**, **Pause / Resume** temporarily suspends access while retaining the token. **Revoke** permanently retires the credential and cancels in-flight queries. **Rotate token / Issue new token** replaces it while retaining Agent identity, grants and audit history. Save the new token and update your client; the previous token never becomes valid again. On the first upgrade, previously disabled Agents (shown as revoked in older versions) are permanently revoked. Active credentials are preserved.

## OAuth

Ory Fosite implements authorization code grants with mandatory PKCE S256, explicit administrator consent, rotating refresh tokens and revocation. Access tokens last 15 minutes and refresh grants 30 days. OAuth shares the same agent data-source grants as independent tokens. Each consent creates a distinct grant.

Discovery is available at `/.well-known/oauth-protected-resource` and `/.well-known/oauth-authorization-server`. Both authorization and token requests must pass the exact public MCP URL in `resource`, such as `https://db.example.com/mcp`.

Clients can be pre-registered through the UI, use bounded dynamic registration at `/oauth/register`, or supply an HTTPS Client ID Metadata Document. Metadata fetching rejects private addresses, redirects and oversized responses. Redirects require HTTPS, HTTP on a loopback IP, or a reverse-domain native application scheme. Redirect URIs match exactly. Revoke credentials at `/oauth/revoke`.

## Configuration changes and OAuth client management

Agent edits include a revision. A stale save returns HTTP 409 instead of restoring older grants or re-enabling a paused Agent. Close and reload the current grants before editing again. Deleting a data source removes its grants in the same transaction; upgrades also clean up references to previously deleted sources. Renaming a data source preserves running queries, its connection and pagination cursors. Connection, access-state and limit changes invalidate affected execution as before.

Expiration is editable to the second in the browser's local time zone. Leaving the field unchanged preserves the original expiration timestamp in full. Omitted or `null` API `sources` values normalize to `[]`, granting no data source access.

**Agents → OAuth clients** lists registered clients and supports search, name/redirect editing, disabling/enabling, secret rotation and deletion. Redirect changes, disabling, secret rotation and deletion revoke the client's access/refresh tokens, pending authorizations and associated Agent credentials, and cancel running queries. Enabling permits new consent; it does not restore revoked tokens. Changing only the client name preserves credentials. Public PKCE clients have no secret. Metadata document clients obtain their name and redirects from the document; disabling cannot be overridden by a metadata refresh. Deletion releases a registration slot, but a client may register again and request new consent. Use Disable when a metadata document client must stay blocked. Client secrets are shown once and are never included in client listings.
