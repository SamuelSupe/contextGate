# Query reuse pilot worksheet

[简体中文](query-pilot.zh-CN.md) · [Query publishing](query-publishing.md)

This is a collection template, not a performance report. Identify a configuration owner, a business reviewer, the existing Agent workflow and a fixed sample/data revision. Record changes and missing evidence rather than estimating them as measured results.

| Attempt | Source/template/version | Started → first matching client call | Hands-on minutes | Completed/abandoned/failed | Blocker and request ID |
| --- | --- | --- | --- | --- | --- |
| Fill for each valid attempt, including failures | | | | | |

Observe at least ten valid setup attempts before calculating a completion rate or median activation time. Count all valid attempts in the denominator. Measure later-query setup separately. Record whether credentials, a working query and the client were ready before starting; waiting time and hands-on time are different quantities.

Collect 30 **real** pilot questions: 20 development questions and 10 independent acceptance questions. The demo's questions are examples, not a substitute for this sample. Include synonyms, refund/time boundaries, empty results, unsupported requests, ambiguous queries and unauthorized identities. Repeat each question three times when practical. Do not place secrets, sensitive parameters or gold answers in public semantic descriptions.

| Question ID / split | Acceptance criteria / data revision | Model/client settings | Baseline/guided evaluation IDs | Actual tools used | Verdict / reviewer / time | Duration / interventions / missing evidence |
| --- | --- | --- | --- | --- | --- | --- |
| | | | | | | |

Use existing **Single answer check** and optional comparison evaluations. Human reviews stay attached to the captured run and its configuration. A native baseline must actually use native queries; if it calls templates, label it accordingly. Separate configurations with equivalent permissions are needed when query mode or catalogs differ; do not suppress configuration-change warnings. Execution time is not end-to-end answer time or data freshness.

Report correct / partially correct / incorrect / unreviewed / failed / unfinished separately. Correctness uses reviewed, completed runs; review coverage uses all valid initiated runs. Publish both denominators and sample size. Do not derive all user questions from successful-call audits. Runtime checks and human verdicts must remain distinct.

| Week / team | Query/Agent reuse on separate dates | Saved work (manual estimate) | Added maintenance (manual estimate) | Setup cost to date | Net benefit / blockers / deployment willingness |
| --- | --- | --- | --- | --- | --- |
| | | | | | |

Require sustained use by two real Agents, excluding previews and test clients, before reporting reuse. Two consecutive weeks of positive net benefit are a pilot decision criterion, not an implemented product claim. Model costs require credible client usage data; otherwise mark them missing. Select the next feature only after identifying a recurring blocker. Ownership fields, automatic regressions, catalog integrations and ranking remain separate follow-up decisions.
