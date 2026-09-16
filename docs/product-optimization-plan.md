# Product optimization plan

[简体中文: original proposal](product-optimization-plan.zh-CN.md)

Proposed on 2026-09-16 against 0.7.0 (`89cf20d`). This page distinguishes the original hypotheses from the work delivered in [0.8.0](releases/0.8.0.md).

## Product focus

Turn existing business queries into verified, authorized tools that Agents can reuse. The initial audience is an internal data or platform team with working SQL or fixed read APIs. A support team looking up orders and refund status is the example workflow, not evidence of a validated customer segment.

Keep native querying available for exploration and make ontology reuse optional. Maintain the existing source authorization, read-only execution and whole-source publication boundaries. Do not add federation, per-template authorization, model inference or automatically compiled queries.

## Delivered in 0.8.0

- A continuous source → query → parameter/result contract → real trial → full draft review → publication → client confirmation workflow.
- One editor for native and HTTP query drafts; discover existing allowed parameter slots without string interpolation, preserve saved work and explicitly recover editing conflicts.
- A primary Ontologies entry, graphical inspectors and explicit links between mapped concepts and queries. Business definitions remain optional for the first usable query.
- Available queries, administrator drafts and queries needing attention in one catalog. Direct query navigation retains search state; Agent projections exclude management views.
- Exact source/template/execution-version/Agent evidence, separate from previews and business acceptance. Client waiting is bounded and cancellable.
- Up to five account-scoped recent query references, one primary next action, compact forms and contextual help in English and Chinese.

See the [publishing guide](query-publishing.md), [support demo](../examples/query-publishing/README.md) and [validation record](query-publishing-validation.md) for the implemented behavior and its verification limits.

## Decisions requiring a pilot

The original proposal also discusses owners, reviewed question-to-template references, retrieval ranking, external catalog/OpenAPI imports and scheduled regression checks. These are **not delivered in 0.8.0**. Add them only when a measured workflow problem justifies the maintenance cost. Cloud previews require real vendor environments before becoming verified support.

Use the [pilot worksheet](query-pilot.md) to record ten independent setup attempts, a reviewed set of thirty business questions, reuse by another Agent and two weeks of maintenance effort. Compare setup time, abandonment points, reviewed answer quality, human intervention and net time saved. Successful fixture queries do not establish customer retention, willingness to pay, business accuracy or operational savings.
