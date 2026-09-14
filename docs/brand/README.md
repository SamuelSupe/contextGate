# ContextGate brand

[简体中文](README.zh-CN.md) · [Project](../../README.md)

**ContextGate**

Semantic Data Gateway for AI Agents

Understand your business. Query data safely.

![ContextGate logo](contextgate-logo.svg)

ContextGate connects business meaning to controlled access to real data. Context covers business terms, metrics, shared ontologies, data-source mappings and query templates. Gate covers authorization, read-only execution, query limits, publication validation and audit. MCP describes the supported protocol; database products describe connection capabilities. The brand does not imply knowledge-graph instance storage or a reasoning engine.

## Logo assets

The open geometric C is a gateway. The central node represents a business concept, and the horizontal path represents its controlled connection to data. The design uses deep teal `#183F43`, signal teal `#32B6A0` and warm white `#F3F8F6`.

- [Primary vector logo](contextgate-logo.svg): wordmark with the English descriptor.
- [Symbol](contextgate-mark.svg): transparent SVG for navigation and small icons; recommended minimum 20 px; also used as the browser favicon.
- [Monochrome symbol](contextgate-mark-mono.svg): single-color use.
- [README banner](../images/banner.svg): English default product positioning.
- [Generated concept](contextgate-concept.png): original concept created with the built-in imagegen tool; the SVG assets are the manually refined production geometry.
- [Generation prompt](logo-prompt.md).

Keep at least one node diameter of clear space around the symbol. Preserve proportions and contrast; omit the descriptor at small sizes. SVG wordmark text uses the viewer's available system sans-serif font. The application's canonical symbol is `web/public/contextgate-mark.svg`; keep the documentation copy identical when changing its geometry.

## Rename compatibility

New source builds, Docker images and future distributions use `contextgate` as the primary executable. `make build`, the Docker image and packaged archives also provide `mcpdbhub` as an equivalent compatibility entry point. MCP server discovery advertises `contextgate`; newly generated client configuration uses that server key. Existing client configurations can keep their old names.

To preserve existing deployments, `MCPDBHUB_*` environment variables, Compose service names and volume names, PostgreSQL identifiers, encryption identifiers, browser preferences and `mcpdbhub.audit.*` telemetry attributes remain unchanged. The default OTLP `service.name` is now `contextgate` for an empty/new configuration; saved service names are not rewritten.

The repository and Go module are now `github.com/SamuelSupe/contextGate`. Update local Git remotes to that URL. The repository retains its history and previous releases; those versions keep their original filenames and behavior.

[Actual build and browser verification](../verification/contextgate-brand.json). This covers the rebrand in a Linux arm64 OrbStack build and local Chrome; it is not a new full database matrix or amd64 release verification.
