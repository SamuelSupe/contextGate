# ContextGate

![ContextGate — Semantic Data Gateway for AI Agents](docs/images/banner.svg)

[![Release](https://img.shields.io/github/v/release/SamuelSupe/contextGate?color=438c91)](https://github.com/SamuelSupe/contextGate/releases/latest)
[![CI](https://github.com/SamuelSupe/contextGate/actions/workflows/ci.yml/badge.svg)](https://github.com/SamuelSupe/contextGate/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![Databases](https://img.shields.io/badge/Databases-18_products-438c91)](docs/support-matrix.md)

[简体中文](README.zh-CN.md) · [Documentation](docs/README.md) · [Download v0.8.0](https://github.com/SamuelSupe/contextGate/releases/tag/v0.8.0)

**Semantic Data Gateway for AI Agents**

ContextGate turns existing business queries into verified, authorized and traceable tools for AI Agents. Administrators connect sources, explain queries, verify them and publish. Users connect their existing Agent client, find an authorized query and run it with their own inputs.

Business definitions and shared ontologies are optional context. Queries keep their native results; ContextGate does not store knowledge-graph instances or provide a reasoning engine. It is written in Go with an embedded bilingual administration UI; no Node.js runtime is required.

## Choose your guide

| What you want to do | Start here |
| --- | --- |
| Use an Agent to query an existing service | [User guide](docs/user-guide.md) — connect, find queries, read results and resolve errors |
| Deploy the service or provide queries to others | [Administrator guide](docs/admin-guide.md) — install, publish, grant access and operate |
| Change the code or reproduce verification | [Contributing](CONTRIBUTING.md) — development, validation and release references |

The web UI is an administration console. Query users work through MCP at `/mcp` and do not need a console account. Configuration MCP at `/mcp/config` is a separate administrator capability.

![Published query catalog in the ContextGate 0.8.0 administration UI](docs/screenshots/0.8.0/query-tools.png)

*Captured from a running administration console with isolated databases and sample data.*

## Capabilities

| Capability | What it provides |
| --- | --- |
| Verified queries | Publish working SQL or fixed read API operations with parameter contracts, trials and explicit versions |
| Business context | Describe terms and metrics; optionally reuse shared entities and source mappings |
| Controlled access | Per-Agent source grants, expiry, pause, rotation, revocation and OAuth; optional templates-only mode |
| Bounded native reads | Read-only protection, timeouts, cancellation and result limits across SQL, document, key-value, search, graph, time-series and HTTP API sources |
| Native results | Preserve large integers, decimals, binary values and native result structures |
| Administration and operations | Named accounts, encrypted credentials, audit/OTLP export, health checks and backup recovery |

The [support matrix](docs/support-matrix.md) lists verified products, versions and limitations. Snowflake, Databricks SQL, BigQuery and Redshift are [cloud previews](docs/cloud-warehouses.md) without real cloud-environment verification.

## Set up a service

[ContextGate 0.8.0](https://github.com/SamuelSupe/contextGate/releases/tag/v0.8.0) provides Linux **arm64 / amd64** packages with the UI, private C++ runtime libraries, documentation, examples and checksums. They require glibc ≥ 2.36 and a PostgreSQL metadata database. macOS users can run the service through Docker / OrbStack.

Follow [installation](docs/install.md), then [first-time administrator setup](docs/getting-started.md). The [three-query support example](examples/query-publishing/README.md) is a small starting point. Existing deployments should read the [upgrade notes](docs/releases/0.8.0.md#compatibility-and-upgrade).

## Project

Formerly **MCP DB Hub**. The primary executable is `contextgate`; `mcpdbhub`, existing `MCPDBHUB_*` settings and telemetry names remain compatible. See [brand and compatibility](docs/brand/README.md).

[Report an issue](https://github.com/SamuelSupe/contextGate/issues/new/choose) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md) · [Architecture](docs/architecture.md)

ContextGate is licensed under the [Apache License, Version 2.0](LICENSE). See [NOTICE](NOTICE) for attribution and [third-party notices](THIRD_PARTY_NOTICES.md) for dependency licenses.
