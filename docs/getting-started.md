# Your first ContextGate workflow

[简体中文](getting-started.zh-CN.md) · [Documentation](README.md)

Install ContextGate using the [installation guide](install.md). Open `http://127.0.0.1:8080`, use the one-time setup code from the server log, and choose the first super administrator username and password. There is no default administrator password.

**Home → New query** opens a continuous five-step workflow: choose a source, provide an existing query, explain its contract, trial/review/publish, and confirm the exact version's real Agent call. **Continue last setup** restores saved references from the server. Read the [publishing guide](query-publishing.md) and try the [support demo without an ontology](../examples/query-publishing/README.md). New sources in this flow recommend Templates only; existing modes stay unchanged. Native exploration and the advanced paths below remain available.

## 1. Connect a source

In **Data sources**, add a database connection with a dedicated database read-only account. The gateway's own PostgreSQL metadata account is separate and needs write privileges. Configure TLS, query limits and file roots where relevant. Save, test connectivity, and inspect read-only evidence. A successful connection with **Not verified** permissions is not proof of database read-only privileges.

For a JSON REST API, choose **HTTP API**, declare fixed read-only operations and scalar parameters, then test the configured check operation. HTTP read-only behavior remains administrator-declared; follow the [HTTP API guide](http-api.md).

Keep **Native queries and templates** while preparing the source, or choose **Templates only** when Agents should use exclusively published templates. The latter rejects native Agent queries even when the same Agent has source access.

## 2. Describe and verify

Open the source's **Semantics** area. Import selected objects from structure discovery; this creates field/type skeletons without reading business samples. Add descriptions, terms, metrics, units and caveats. Create native query templates with explicit parameter bindings and example parameters. Run a real read-only trial for each enabled template.

For shared concepts, create an ontology in **Ontologies**, using the graphical or list editor. Define entities such as Customer and Order, their properties and relationships; validate and publish it. In the source's **Ontology mapping**, select that published version and map concepts to physical objects, fields and templates. Check discoverable fields and explicitly declare any unverifiable fields. Mappings do not generate queries or enforce database constraints.

Open **Review and publish** to see required trials or mapping checks, changed entries, explicitly linked templates and authorized Agents that may be affected. Run missing checks directly from the review, or open the relevant editor to repair them. Publication stays blocked until validation is current. Saving a draft alone never changes what query Agents see. Connection or credential changes can expire template verification; trial and publish again to restore execution. The [retail demo](../examples/ontologies/retail-demo/README.md) provides concrete definitions and mappings.

**Query tools → Available queries** lists executable published templates with related concepts as context. **Drafts** lists new, changed and pending-removal queries; **Needs attention** lists enabled published queries blocked by invalid evidence or a disabled source. These two management views are administrator-only. Selecting an Agent immediately switches to its published visible subset. **All definitions** includes published terms, metrics and mapped ontology definitions. Search by name, alias or description and filter by source. A query name opens its workspace; **Preview query** reads the published version. Returning restores the search, page, position and focus. Concept details and query previews remain linked. A successful trial proves the configured checks passed, not that every business answer is correct.

The template editor provides a native query text view for text-based query families and an advanced JSON view for the full contract. **Find parameter positions** lists existing allowed value positions; it does not create placeholders or interpolate text. Add native parameter slots in the query options first, then choose their bindings. For HTTP APIs, select a configured operation to copy its parameter contract and examples. In **Ontology mapping**, select an entity to focus its fields and relationships; **Discover fields** offers available physical fields without reading business samples. Declared HTTP API fields remain unverified.

The catalog returns 20 entries per page across up to 100 accessible sources. For larger installations, select a source explicitly. Publication or access changes require restarting pagination. Agent views exclude drafts and unmapped ontology definitions. Administrator draft pages also bind the draft revision; a draft save does not invalidate published-only pages. Queries still execute against one authorized source.

## 3. Connect a query Agent

Create an **Agent**, select its permitted data sources and expiry, and save the token displayed once. Open **Connect** for the selected client's HTTP or stdio settings. The query endpoint is `/mcp`; the [example files](../examples/) use placeholder tokens. Use the gateway's reachable address, since `localhost` means the machine running the client.

Ask the Agent:

> List the data sources I can access. Find the published business definitions and templates relevant to customer orders. Read the parameter contract, then execute the matching verified template and explain its result using the published definitions.

The tool sequence is `list_data_sources` → `search_semantics` → `get_semantic_entry` → `execute_query_template`. Use the current template execution version returned by discovery. Raw query tools remain available only where permitted. Check **Audit log** and recorded client activity to confirm the real client called the service; an administrator preview is a different kind of evidence. The query workspace offers a cancellable **Wait for client call** action, bounded to two minutes and scoped to the selected Agent and execution version.

## 4. Check a business answer

Open **Evaluate** for a source and use the default **Single answer check** mode. Record a business question and acceptance criteria, select the Agent that will answer it, and start a capture. Ask that client to run the task, collect its calls, then record your verdict and notes. **Compare baseline and semantic guidance** is optional when you want separate baseline and guided runs. Captures summarize observed calls; they do not automatically grade business correctness or retain query results.

## 5. Automate configuration, if needed

In **Settings → My configuration MCP**, issue a short-lived credential to a trusted setup Agent and connect it to `/mcp/config`. This credential covers all sources and shared drafts; source edits take effect immediately. It can discover structure, prepare catalogs/templates/ontologies and run trials. Review its changes in the UI, publish the ontology and source semantics, then grant your separate query Agent access. See the [configuration guide](configuration-mcp.md) for the tool contract and starter prompt.

## When something fails

| Symptom | Next action |
| --- | --- |
| No sources visible | Inspect the Agent's grants, expiry, pause/revocation and source enablement |
| Template unavailable | Check publication, enabled state and current connection-bound trial evidence |
| Publish rejected | Repair validation errors, missing mappings or failed example/regression trials |
| Revision conflict | Reload current configuration, merge intentionally and save again |
| Old execution version or cursor | Refresh published discovery; restart paging with the current contract |
| HTTP 401 on Configuration MCP | Use its dedicated configuration credential, not a query token or browser cookie |
| Remote connection failure | Configure HTTPS and a fixed public URL reachable by the client |

Use safe error categories and request IDs from the UI to locate audit records. Do not paste passwords, database connection secrets or complete token-bearing configurations into issue reports.
