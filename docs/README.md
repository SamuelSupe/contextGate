# ContextGate documentation

[简体中文](README.zh-CN.md) · [Project](../README.md) · [Download](https://github.com/SamuelSupe/contextGate/releases/latest)

These guides describe **ContextGate 0.8.1**. For another installed version, use the documentation at its release tag.

## For query users

Start with the **[user guide](user-guide.md)** when your administrator has provided access to an existing service. It covers the complete path from client connection to a useful query result:

1. [Connect your MCP client](user-guide.md#connect-your-client).
2. [Find and execute a published query](user-guide.md#run-a-published-query).
3. [Supply parameters](user-guide.md#supply-parameters) and [read results or continue pages](user-guide.md#read-results-and-continue-pages).
4. [Use permitted native queries](user-guide.md#native-queries) or [resolve a query problem](user-guide.md#resolve-a-query-problem).

Query users work through their Agent client. They do not need a database password or a ContextGate administrator account.

## For administrators

Start with the **[administrator guide](admin-guide.md)** when you install the service, provide queries to others or maintain access and operations.

| Task | Guide |
| --- | --- |
| Deploy and prepare the first source | [Installation](install.md) → [First-time administrator setup](getting-started.md) |
| Publish queries and maintain business definitions | [Query publishing](query-publishing.md) · [Semantics](semantics.md) · [Ontologies](ontologies.md) |
| Manage identities and access | [Administrator accounts](administrators.md) · [Query Agent access and OAuth](agent-access.md) |
| Confirm real client use and business answers | [Setup and evaluation](agent-workflows.md) |
| Maintain, recover and audit | [Operations](operations.md) · [Audit export](audit-export.md) |

The administrator guide also indexes connector setup, Configuration MCP, examples and user handoff. The web console belongs to this workflow; selecting an Agent in a preview does not turn it into a user console.

## Reference and project development

[Support matrix](support-matrix.md) · [Architecture](architecture.md) · [Security](../SECURITY.md) · [0.8.1 release notes](releases/0.8.1.md)

For build instructions, reproducible validation, release procedures and product delivery records, use **[Contributing](../CONTRIBUTING.md)**. These materials are separate from everyday user and administrator instructions.
