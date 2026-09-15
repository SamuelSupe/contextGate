# Administrator accounts and personal Configuration MCP

[简体中文](administrators.zh-CN.md) · [Documentation](README.md)

ContextGate supports multiple named administrator accounts on one instance. All administrators share data sources, semantic catalogs, ontologies, query Agents and OAuth query clients. The two roles control account and system administration; they do not partition business resources.

## Sign in and add a colleague

Initialization requires the one-time setup code and creates the first **Super administrator** with your chosen username and password. Usernames are unique without case distinctions, contain 3–64 ASCII letters, numbers, dots, underscores or hyphens, and cannot be renamed. Display names may use other languages.

1. Sign in and open **Settings → Administrators**.
2. Select **Add administrator**, enter a username and display name, and choose **Administrator** or **Super administrator**.
3. Save the temporary password shown once and give it directly to the account owner. It expires after 24 hours.
4. The owner signs in with that username and temporary password, then chooses a different personal password. Until then, the account can only change its password or sign out. Expired temporary passwords require a super administrator to reset them.

Accounts can be disabled and reenabled, but not deleted. You cannot disable yourself or change your own role. Concurrent account edits cannot remove the last enabled super administrator. An outdated revision returns HTTP 409; reload before editing again.

## Roles

| Capability | Administrator | Super administrator |
| --- | --- | --- |
| Sources, semantics, ontologies and publication | Yes | Yes |
| Query Agents, grants and OAuth clients/consent | Yes | Yes |
| Business configuration/query audit | Yes | Yes |
| Personal password, language and own Configuration MCP token | Yes | Yes |
| Accounts, roles, disablement and password resets | No | Yes |
| OTLP, diagnostics and health-check policy | No | Yes |
| Security audit and revoking another owner's configuration token | No | Yes |

Role checks apply to the API as well as the UI. Super administrators cannot issue another person's Configuration MCP token or retrieve a password/token.

## My configuration MCP

Every account has one fixed configuration identity. Creating the account does **not** issue a token. Open **Settings → My configuration MCP**, select **Issue my token**, choose a lifetime and copy the HTTP or stdio configuration. The default lifetime is 24 hours; the maximum is 30 days. Store the token when shown: it cannot be retrieved later.

**Rotate my token** keeps the identity ID, invalidates the old token immediately and cancels its running configuration work. Update every client using that identity. **Revoke** leaves the identity available for a later issue. All actions are attributed to the owning administrator.

Configuration MCP can edit every source connection, prepare semantic/ontology drafts, discover structure and trial templates. It cannot publish, grant query access, administer accounts or change global settings. Query Agent credentials and OAuth query grants remain separate. See the [connection examples and tool contract](configuration-mcp.md).

## Credential lifecycle

| Change | Sessions | Configuration token | Query Agents/OAuth query grants |
| --- | --- | --- | --- |
| Own password change | Keep this session; revoke own other sessions | Retained | Retained |
| Role change or account disablement | Revoke that account's sessions | Revoked | Retained |
| Super administrator password reset | Revoke that account's sessions; issue temporary password | Revoked | Retained |
| Local CLI password recovery | Revoke selected account's sessions | Revoked | Retained |
| Configuration token rotation/revocation | Retained | Old token invalid immediately | Retained |

Account revocation cancels that person's configuration jobs and UI previews. Unrelated administrators and query Agents continue working. Reenabling an account never restores old sessions/tokens. Execution rechecks credentials and binds relevant cursors to the acting administrator, session or configuration credential version. Already committed changes remain committed.

## Audit

Select **Audit log → All activity** or **Management changes** to inspect business changes. Filter by administrator, configuration identity ID, entry point and outcome. When previewing as a query Agent, details retain both the actual administrator and the execution Agent. The administrator directory shown for filtering contains names only.

Only super administrators can read **Account security**, including account creation, role/status edits, password/token changes and authentication events. Ordinary administrators cannot retrieve these events using alternate filters or direct API calls. A local password recovery is attributed to `local_operator` through `cli`, with the target account as the resource; server access does not prove a named UI identity.

Records contain IDs, verified usernames, action, submitted field categories, revisions, request IDs, timing and outcome. They do not contain passwords, tokens, full definitions, query parameters or results. Changes record intent first and outcome second. If intent cannot be written, the change is refused. A failed outcome write leaves an identifiable pending event for investigation. New actor fields are also delivered through [OTLP Logs](audit-export.md).

## Upgrade from 0.5.0 or an earlier PostgreSQL release

This account model is available in 0.6.0. Back up the PostgreSQL metadata database and its matching master key before upgrading.

On first startup, one transaction:

- Migrates the old password hash unchanged into username **`admin`**, role **Super administrator**.
- Invalidates old browser sessions: sign in again with `admin` and the existing password.
- Revokes **all old Configuration MCP tokens**. Each owner must issue a new personal token and update their clients.
- Retains sources, credentials, semantic/ontology content, query Agent/OAuth grants, evaluations and audit history.
- Keeps legacy configuration identities and historical `admin` events as historical identities, without guessing a person.

The migration is idempotent and rolls back on failure. Do not run old and new server versions against the same metadata database. To roll back the application, restore the pre-upgrade database backup and matching key. SQLite metadata from 0.3.x or earlier still requires the separate [PostgreSQL migration procedure](releases/0.4.0.md#upgrading-from-03x-or-earlier).

## Recover a forgotten password

Stop ContextGate. Keep the same `MCPDBHUB_DATABASE_URL` and matching master key, and pass the new password through stdin:

```sh
read -r -s contextgate_new_password
printf '%s' "$contextgate_new_password" | contextgate reset-password \
  --data-dir /path/to/data --username alice --password-stdin
unset contextgate_new_password
```

Restart the service afterward. `--username` may be omitted only when exactly one account exists. Recovery preserves the account's role and enabled status, revokes only that account's sessions/configuration token, and records a security event. It does not enable a disabled account.

## Management API

All management APIs use the administrator session cookie. Mutations also require `X-CSRF-Token` from `/api/session`. Revisions are JSON strings.

| Method and path | Contract |
| --- | --- |
| `POST /api/setup` | `token`, `username`, optional `display_name`, `password` |
| `POST /api/login` | `username`, `password`; uniform credential error |
| `GET /api/session` | Authentication, CSRF token and current `administrator` |
| `POST /api/password` | `current_password`, `password`; retains current session |
| `GET /api/administrators` | Super only; account list with configuration identity status |
| `POST /api/administrators` | Super only; `username`, `display_name`, `role`; one-time `temporary_password` |
| `PUT /api/administrators/{id}` | Super only; `display_name`, `role`, `enabled`, `revision` |
| `POST /api/administrators/{id}/reset-password` | Super only; `revision`; one-time `temporary_password` |
| `GET /api/configuration-agents` | Current owner's one identity and MCP endpoint |
| `POST /api/configuration-agents/{id}/token` | Owner only; `revision`, optional `name` and `expires_at`; issue/rotate |
| `DELETE /api/configuration-agents/{id}` | Owner or super; revoke only |
| `GET /api/audit` | `administrator_id`, `configuration_agent_id`, `channel`, existing event/outcome/date filters |

`POST /api/configuration-agents` can issue a token for the current owner's inactive identity. It returns 409 if an active token already exists; rotate the fixed identity with its current revision instead. Responses never include stored password/token hashes.
