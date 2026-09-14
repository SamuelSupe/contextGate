# Your first ContextGate workflow

[简体中文](getting-started.zh-CN.md) · [Documentation](README.md)

Install ContextGate using the [installation guide](install.md). Open `http://127.0.0.1:8080`, use the one-time setup code from the server log, and choose the administrator password. There is no default administrator password.

## 1. Connect a source

In **Data sources**, add a database connection with a dedicated database read-only account. The gateway's own PostgreSQL metadata account is separate and needs write privileges. Configure TLS, query limits and file roots where relevant. Save, test connectivity, and inspect read-only evidence. A successful connection with **Not verified** permissions is not proof of database read-only privileges.

Keep **Native queries and templates** while preparing the source, or choose **Templates only** when Agents should use exclusively published templates. The latter rejects native Agent queries even when the same Agent has source access.

## 2. Describe and verify

Open the source's **Semantics** area. Import selected objects from structure discovery; this creates field/type skeletons without reading business samples. Add descriptions, terms, metrics, units and caveats. Create native query templates with explicit parameter bindings and example parameters. Run a real read-only trial for each enabled template.

For shared concepts, create an ontology in **Ontologies**, using the graphical or list editor. Define entities such as Customer and Order, their properties and relationships; validate and publish it. In the source's **Ontology mapping**, select that published version and map concepts to physical objects, fields and templates. Check discoverable fields and explicitly declare any unverifiable fields. Mappings do not generate queries or enforce database constraints.

Review validation errors, then publish the source semantics. Saving a draft alone never changes what query Agents see. Connection or credential changes can expire template verification; trial and publish again to restore execution. The [retail demo](../examples/ontologies/retail-demo/README.md) provides concrete definitions and mappings.

## 3. Connect a query Agent

Create an **Agent**, select its permitted data sources and expiry, and save the token displayed once. Open **Connect** for the selected client's HTTP or stdio settings. The query endpoint is `/mcp`; the [example files](../examples/) use placeholder tokens. Use the gateway's reachable address, since `localhost` means the machine running the client.

Ask the Agent:

> List the data sources I can access. Find the published business definitions and templates relevant to customer orders. Read the parameter contract, then execute the matching verified template and explain its result using the published definitions.

The tool sequence is `list_data_sources` → `search_semantics` → `get_semantic_entry` → `execute_query_template`. Use the current template execution version returned by discovery. Raw query tools remain available only where permitted. Check **Audit log** and recorded client activity to confirm the real client called the service; an administrator preview is a different kind of evidence.

## 4. Automate configuration, if needed

In **Settings → Configuration MCP**, issue a short-lived credential to a trusted setup Agent and connect it to `/mcp/config`. This credential covers all sources and shared drafts; source edits take effect immediately. It can discover structure, prepare catalogs/templates/ontologies and run trials. Review its changes in the UI, publish the ontology and source semantics, then grant your separate query Agent access. See the [configuration guide](configuration-mcp.md) for the tool contract and starter prompt.

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
