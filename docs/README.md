# ContextGate documentation

[简体中文](README.zh-CN.md) · [Project](../README.md) · [Download](https://github.com/SamuelSupe/contextGate/releases/latest)

**Semantic Data Gateway for AI Agents.** Connect databases, describe business concepts, publish verified queries, and grant Agents controlled access.

## Start here

| Your task | Guide |
| --- | --- |
| Manage administrator accounts and personal MCP identities | [Accounts and upgrade guide](administrators.md) |
| Install on Linux or Docker / OrbStack | [Installation](install.md) |
| Complete the first source-to-Agent workflow | [Getting started](getting-started.md) |
| Let an Agent configure ContextGate | [Configuration MCP](configuration-mcp.md) |
| Upgrade from ContextGate 0.5.x | [0.6.0 release and upgrade notes](releases/0.6.0.md) |
| Upgrade from MCP DB Hub | [0.4.0 release and upgrade notes](releases/0.4.0.md) |

## Model and query your data

| Area | Guide |
| --- | --- |
| JSON REST APIs, read operations and API-backed templates | [HTTP API sources](http-api.md) |
| Supported databases, versions and limitations | [Support matrix](support-matrix.md) |
| Database permissions | [Read-only account examples](read-only-accounts.md) |
| Business terms, metrics and native query templates | [Semantic catalogs](semantics.md) · [Seven-family examples](../examples/semantics/) |
| Search published business concepts and preview linked queries | [Business catalog workflow](getting-started.md#2-describe-and-verify) |
| Shared entity, property and relationship definitions | [Ontologies and mappings](ontologies.md) |
| Reuse one retail ontology across different databases | [Retail demo](../examples/ontologies/retail-demo/README.md) |
| Client setup, Agent previews and saved evaluations | [Agent workflows](agent-workflows.md) |

## Operate the gateway

| Area | Guide |
| --- | --- |
| Backups, recovery, health, history and regressions | [Operations](operations.md) |
| Review publication impact and separate audit activity | [Review and operations](operations.md#review-before-publishing) |
| Export audit events to your observability stack | [OTLP Logs](audit-export.md) |
| Architecture and execution boundaries | [Architecture](architecture.md) · [Security](../SECURITY.md) |
| Actual test evidence and reproducible checks | [Validation](validation.md) |
| Build and publish distributions | [Release process](releasing.md) |
| Report problems or contribute | [Contributing](../CONTRIBUTING.md) · [Issues](https://github.com/SamuelSupe/contextGate/issues/new/choose) |

The default documentation and UI language is English. Choose **Settings → Language → 简体中文** for the Chinese UI. Business definitions and query results retain their original language.

Configuration credentials, query Agent tokens and administrator sessions have different roles. Follow the guide for your endpoint; a configuration credential is not a data-source-scoped query token.
