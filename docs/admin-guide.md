# Administrator guide

[简体中文](admin-guide.zh-CN.md) · [Documentation](README.md) · [User guide](user-guide.md)

Use this guide to deploy ContextGate, connect data sources, publish queries, grant Agent access and operate the service. The web console is for administrators. People who only use a query Agent should receive the [user guide](user-guide.md) and their connection details.

The console defaults to English. Choose **Settings → Language → 简体中文** to switch this browser to Chinese; business definitions and query results keep their original content.

## Set up your first query

Follow this path for a new instance:

1. **Install and initialize.** Follow [installation](install.md), prepare PostgreSQL metadata and persistent key storage, then use the setup code to create the first super administrator.
2. **Connect and publish.** Follow [first-time administrator setup](getting-started.md). Start at **Home → New query**, choose a source with a dedicated read-only account, provide a working query, define inputs, trial it and review publication. Shared ontologies are optional.
3. **Grant access.** Create a query Agent with the required source grants and lifetime. Use [Agent access and OAuth](agent-access.md) to choose the credential and provide a client configuration.
4. **Confirm actual use.** Ask the user to execute the published query from their real client. Check the selected Agent and exact template version's call in the workspace; administrator previews do not count as client access evidence.

The [three-query support example](../examples/query-publishing/README.md) walks through a small deployment without an ontology. For an existing installation, start with its current task below.

## Configure sources and publish queries

| Task | Guide |
| --- | --- |
| Choose a supported product and database account | [Support matrix](support-matrix.md) · [Read-only accounts](read-only-accounts.md) |
| Publish an existing query and resume saved work | [Query publishing](query-publishing.md) |
| Maintain business definitions, parameters and trials | [Semantic catalogs and templates](semantics.md) · [Example catalogs](../examples/semantics/) |
| Reuse shared concepts across sources | [Ontologies and mappings](ontologies.md) · [Retail example](../examples/ontologies/retail-demo/README.md) |
| Configure fixed JSON API operations | [HTTP API sources](http-api.md) |
| Configure an unverified cloud preview adapter | [Cloud warehouses](cloud-warehouses.md) · [Examples](../examples/cloud-warehouses/README.md) |
| Let a trusted Agent prepare configuration | [Configuration MCP](configuration-mcp.md) |

Query Agent grants are scoped to sources. Database accounts, views and native permissions define table, field and row visibility. Source configuration changes can take effect immediately; semantic drafts require explicit publication. “Connection successful” and “read-only account verified” are different evidence.

## Manage access and confirm business results

| Task | Guide |
| --- | --- |
| Add administrators, assign roles or recover a password | [Administrator accounts](administrators.md) |
| Issue, pause, rotate or revoke query access; manage OAuth clients | [Agent access and OAuth](agent-access.md) |
| Preview as an Agent and inspect actual client activity | [Agent setup and evaluation](agent-workflows.md) |
| Record business questions and review answers | [Evaluation workflow](agent-workflows.md#evaluate-business-questions) · [Pilot worksheet](query-pilot.md) |

**Administrator** and **Super administrator** are the two console roles. Both manage shared business configuration; super administrators additionally manage accounts, global settings and security audit. There is no separate query-user console role or per-administrator resource partition. See the [role table](administrators.md#roles) before assigning an account.

Keep these credentials distinct:

| Credential | Purpose | Boundary |
| --- | --- | --- |
| Administrator session | Web console and management API | Named administrator role; mutations require CSRF protection |
| Personal Configuration MCP token | Trusted setup Agent at `/mcp/config` | All source connections and shared drafts; cannot publish or grant query access |
| Query Agent token / OAuth grant | Data queries at `/mcp` | Authorized sources, query mode, published templates and execution limits |

## Operate and recover

| Task | Guide |
| --- | --- |
| Configure runtime variables, remote HTTPS or upgrades | [Installation and deployment](install.md) · [0.8.0 upgrade notes](releases/0.8.0.md) |
| Review changes, source health and template regressions | [Operations](operations.md) |
| Back up metadata and verify restoration with the matching key | [Backup and recovery](operations.md#back-up-and-verify-recovery) |
| Export audit events to an observability service | [OTLP audit export](audit-export.md) |

ContextGate runs as a single instance with shared business configuration. Preserve both PostgreSQL metadata and its matching encryption key. The [architecture](architecture.md) explains execution and security boundaries; [Contributing](../CONTRIBUTING.md) contains development, validation and release references.

## Hand off to a user

Provide the reachable `/mcp` URL, the chosen client's configuration, its query credential or OAuth consent arrangement, permitted sources, and one published query with sample inputs and business caveats. Tokens are shown once; deliver them through your team's credential-sharing channel.

Link the [user guide](user-guide.md). The user can then connect, discover queries, execute them and report a request ID without learning how to install the gateway or manage accounts. Review business correctness separately from a successful tool call.
