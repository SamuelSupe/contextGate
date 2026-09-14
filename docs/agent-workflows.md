# From a connection to a useful Agent query

[简体中文](agent-workflows.zh-CN.md)

## Agent setup

In **Data sources**, click a source name to open its query workspace or **Agent setup**. The status below the name shows the next step, executable published templates and active authorized Agents. **More actions → Structure & native preview** opens the administrator exploration panel. Follow these steps:

1. Test the connection and inspect read-only protection evidence. Connectivity, engine protection, account permissions and query API isolation remain distinct. No write probes are sent.
2. Select an active authorized Agent in the identity panel above the queries and open its connection guide. **Create Agent for this source** preselects the source but requires explicitly saving the grant and securely recording the new token.
3. Prepare and preview an allowed query with the selected identity. Native sources can use native previews or verified templates. Templates-only sources require an executable published template. A new successful trial does not restore a suspended template until publication.
4. Complete a query in the real MCP client and refresh activity. Only a successful non-preview query on this source counts as query evidence; structure discovery and administrator previews do not count. Historical success does not prove that current credentials or connectivity still work.

The setup page reports an observed configuration snapshot, not a guarantee of future execution. Connection and last-query evidence show their timestamps. Authorization, query-mode restrictions, limits and template proofs are rechecked by the existing execution engine.

After connection, authorization, query availability and real-client query evidence are present, the page becomes **Query workspace**. Completed setup sections collapse while queries remain open. The selected Agent stays visible before previewing; expand a completed section to inspect or update its evidence.

The Agent connection guide explains common failure categories and links back to source setup or templates. For `templates_only`, use `search_semantics`, inspect a template contract and execute it with `execute_query_template` and the current execution version.

## Follow a business concept

In **Ontologies → Model**, entity cards show published source mapping counts and currently executable template counts. Open **Queries →** directly to inspect **Queries and sources**; use **Definition** to switch back. The inspector header keeps the entity name visible. List mode exposes the same information. Choose an adopted source to inspect:

- Its immutable ontology version and source publication.
- Physical objects and mapped properties for this entity.
- Templates explicitly linked through concept references or property/relationship mappings, with current executable status.
- Active Agents authorized for that source and a direct template preview with the chosen Agent identity.

The display uses that source's published ontology version, even when the shared draft or latest version has advanced. An entity absent from the adopted mapping is explicitly marked not visible. Links open the actual mapping or setup workflow. These are administrator views; no shared management endpoints are exposed to Agent credentials.

## Evaluate business questions

Open **Agent setup → Evaluate a business question**. This is a paired-run worksheet for real client use, not an embedded model or automatic correctness evaluator.

1. Choose a dedicated Agent that has no unrelated calls during the evaluation. Record a question name, client/model/settings, business question and acceptance criteria. Use **Save question for reuse** to add it to **Saved questions**, or start an ad-hoc evaluation. Each run provides its own copyable prompt; acceptance criteria remain separate so the client does not receive the expected answer.
2. Start **Baseline** capture, ask the question using the existing workflow in a fresh client conversation, wait for all tool calls to finish, then collect the completed calls.
3. Start **With semantic guidance** capture and repeat with the same model and data, explicitly using semantic discovery and verified templates when applicable.
4. Review each answer manually as correct, partially correct or incorrect, record the reason and click **Save review**. Captures save automatically; unsaved manual edits are protected when navigating away. Open **History** to resume a capture, review a completed evaluation or repeat it. Export JSON when you need a portable copy.

Questions and evaluations are encrypted in ContextGate configuration database. Capture timestamps, metrics and configuration snapshots come from the server. Editing or deleting a reusable question does not rewrite existing evaluations. Completed measurements remain available after audit retention removes the underlying events. Active captures survive restarts and can be resumed from History; abandon an expired capture before repeating it. Concurrent edits return a conflict, preserving local edits until you save a new question or explicitly discard them and reload.

Lists use 20-record pages. Each source can retain up to 1,000 questions and 10,000 evaluations; export and delete records when reaching the limit. Nothing is silently pruned. Deleting a source also deletes its questions and evaluation history. Back up the configuration database and master key together. These APIs require an administrator session and CSRF protection, and do not expose evaluation content through MCP.

Automatic measurements include completed audited calls, query calls, successful queries, errors, template calls and the sum of ContextGate call durations for the selected Agent and source. Capture uses server timestamps, accepts windows up to 24 hours, and excludes previews. ContextGate cannot observe other sources, model reasoning, token usage or final-answer correctness. Discovery operations without a source-specific audit record are outside these counters; they are not a total of all MCP invocations. Sum of call durations is not end-to-end answer latency and may overlap when calls run concurrently.

Use consistent conditions and separate conversations. The worksheet does not hide tools or change permissions. Configuration versions are captured at the start and checked after collection; differing source execution settings or publications within or between runs produce a comparison warning. Do not change grants, data, source settings or definitions during a paired evaluation. Stop after calls have completed: calls still running when collected have not yet written their audit record.

Exports include the question, criteria, client/model notes, manual assessments, captured configuration metadata and aggregate metrics. They do not automatically include query text, parameters, results or credentials. Review any business information you enter before sharing the exported file.

Start with representative questions such as revenue excluding refunds, customer identity across physical schemas, and time-window aggregation. Use multiple cases and repetitions before attributing a quality improvement to semantics. A successful SQL call is not proof that the business answer is correct.

## Client-specific connection guides

**Agents → Connect** offers Codex, Cursor and VS Code configurations with the exact file or command to open, a copy button, and a connection check. Copy feedback confirms the action. HTTP presets reference `MCPDBHUB_TOKEN`, so they can be copied without embedding the credential. Set the variable in the environment that launches the client and restart it; exporting in an unrelated terminal does not update a running desktop app. OAuth presets omit bearer headers and use browser consent. The stdio bridge remains under Advanced.

Configuration formats were checked against the official [Codex](https://developers.openai.com/codex/mcp), [Cursor](https://cursor.com/docs/context/mcp) and [VS Code](https://code.visualstudio.com/docs/agents/reference/mcp-configuration) references. This documents the configuration contract, not certification of every installed client version. Verify an actual query and recorded client activity. For remote clients, use ContextGate's configured public HTTPS URL; localhost refers to the client machine.

## Template parameter forms

**Preview published** starts with a form generated from the template's parameter contract. Strings, booleans, null and enums have simple controls; native object/array parameters retain JSON editors. Required fields and numeric bounds are validated inline. Optional parameters can be omitted; **Override default** explicitly replaces the server's default. **Use example values** resets the inputs. **Edit JSON** retains the complete parameter object and refuses unknown or duplicate keys instead of silently dropping them.

Integer and decimal values are kept as raw JSON text, including values beyond JavaScript's safe integer range. The execution engine still performs the authoritative binding, permission, read-only and limit checks. Changing parameters or identity clears the previous result and cursor. SQL/CQL results open as a table; document and graph results default to JSON. Both views remain available, and successful empty queries have a separate empty state.

## Reading evaluation history

Opening an evaluation shows the baseline/guided comparison first. Question, expected answer, conditions and review controls are expandable. Manual review changes must be saved; a confirmation appears after saving. Repeat creates a new evaluation using the existing inputs.

History filters question name/text, Agent and creation date across the entire source history, with an inclusive end day in the browser's local time zone. Applied filters and pagination are retained while opening a record and returning to History within the same workspace. Summary counts cover every matching page. The manual-correctness comparison includes only pairs with completed captures, at least one query in each capture, two reviews and unchanged source/Agent configuration. Unrated, empty and changed-configuration pairs are excluded. Different questions, models or underlying data can still affect outcomes; the summary does not establish a causal model-quality improvement.

Business questions remain encrypted; filtering decrypts records individually without creating a plaintext search index. The source's retention cap and a 10-second request deadline bound a search. Filters are held in page memory and are not added to the page URL or local storage.
