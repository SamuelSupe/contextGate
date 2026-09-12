# Shared business ontologies

[简体中文](ontologies.zh-CN.md) · [Commerce examples](../examples/ontologies/) · [Semantic catalogs and templates](semantics.md)

Available in **0.3.0**. Define business entities, properties and relations once, then map each data source to one immutable ontology version. PostgreSQL can map `Customer` to `public.crm_customers`; MongoDB can map the same concept to `hubtest.buyers`. Agents use these definitions to find a source's native query templates. Results keep their native rows, documents, graph or time-series structure.

This is a definition and query-guidance feature. It does not store entity instances, infer facts, compile business concepts into queries, join across sources or normalize native results into unified entities. It does not implement full [OWL 2](https://www.w3.org/TR/owl2-overview/) or [SHACL](https://www.w3.org/TR/shacl/) conformance.

## Define and publish

1. Open **Ontologies → Create ontology**. Give the ontology a name; business names, aliases and descriptions may use any language.
2. Add **Entities**, **Properties** and **Relations**. Entity and definition IDs are stable references. Entities support one parent. The effective properties include inherited properties; cycles and conflicting inherited property names are rejected.
3. After adding properties, choose an entity's identity property combination. Identity properties must belong to its effective definition, be required and be single-valued. Properties support logical types, units, enums, exact numeric range text, uniqueness declarations and time conventions.
4. Define relation endpoints, direction and cardinality. **Origins per target** describes how many origin entities may relate to one target; **Targets per origin** describes the reverse. Blank maximum means unbounded.
5. **Validate**, then **Publish version**. Saving only changes the draft. Publishing creates an immutable version. **Discard draft** restores the latest published definition.

These checks validate definition consistency. They do not establish that every database row satisfies identity, requiredness, uniqueness, ranges or cardinality. Numeric range bounds are decimal strings without exponent notation. Cardinality counts range from zero to 2,147,483,647.

## Map a data source

Open **Data sources → Semantics → Ontology mapping**. Choose an ontology and a published version, then **Save binding draft**.

- **Entity mappings** select one or more physical objects in the current source. Each reference has an explicit namespace and object name.
- **Property mappings** select an effective entity property and either a physical field path or an enabled read-only query template. Inherited properties can be mapped separately for different effective entities.
- **Relation mappings** describe field pairs between the two mapped endpoint entities, an enabled association query template, or both. They never generate joins, query text or parameter bindings.
- In **Query templates**, select the mapped concepts that a template explicitly represents. These references appear in its execution results. A relation or computed-property mapping also guides discovery to its associated template.

Run the normal real **Trial** for each new or changed enabled template, then **Check structure**. The check reads existing metadata APIs, not business samples. Objects must be discoverable. Fields absent from metadata require explicit administrator declaration and remain **unverified**. For example, MongoDB index metadata exposes indexed document paths; other document paths need declarations. Neo4j labels can be discovered, but this feature does not run the existing node-content property scan.

Finally **Publish** the source semantics. The catalog, mapping and template concept associations are committed together. Missing ontology versions, missing definitions, invalid references, unmapped relation endpoints and missing or disabled linked templates block publication. Connection changes expire metadata-check evidence as well as normal query-trial evidence.

The source's database account or views continue to control table, field and row access. Mapping a concept does not grant a database permission. Unmapping a concept hides its ontology definition; native reads still follow the source's configured query-access mode and database permissions.

## Versions and visibility

Sources stay pinned when a new ontology version is published. Select another version in **Ontology mapping**, use **Compare with adopted version**, repair mappings and publish explicitly. Archiving blocks new bindings and new version adoption; existing published bindings continue. Referenced ontology versions cannot be deleted, including references retained by drafts. The latest version is retained as the discard target; an entirely unreferenced ontology can be deleted.

All Agent reads authorize the data source before constructing its visible ontology subset:

- Only mapped entities, properties and relations are returned. Both endpoints of a relation must be visible.
- Ancestors expose only the necessary ID, name and parent chain. Their other descriptions and unmapped properties are omitted.
- An entity's identity combination is omitted if any member is not mapped for that effective entity.
- Other sources' mappings, ontology usage lists and unpublished definitions are never returned. Shared ontology administration APIs require the administrator session.

**Preview Agent visibility** uses the selected Agent's current source grant and the published snapshot. It can inspect definitions and template guidance without exposing the shared administrator view.

Definition, mapping-description and concept-association publications preserve query-trial evidence and do not cancel running queries. Every returned result describes the ontology snapshot captured when that request began. Changed executable definitions, disabled templates and revoked grants retain the existing cancellation behavior. Semantic cursors bind the source publication and ontology version. Native template cursors on ontology-bound sources also bind that context; restart pagination after publication changes.

## MCP contract

There are still **14 MCP tools**. The three semantic tools are extended:

| Tool | Ontology behavior |
|---|---|
| `list_data_sources` | Adds `ontology: {id, version, available}` for the adopted version; no bulk ontology contents |
| `search_semantics` | Adds kinds `entity_type`, `property`, `relation_type`; bounded, snapshot-bound pages |
| `get_semantic_entry` | Returns the visible definition, current source mapping, structure evidence status and associated templates |
| `execute_query_template` | Existing input contract; adds request-start `ontology_context` to the native result |

Ontology references have a separate namespace:

```text
ontology:entity_type:Customer
ontology:property:Customer:customer_id
ontology:relation_type:places
```

Property references contain the effective entity ID and the property ID, allowing different mappings of inherited properties. Legacy catalog IDs cannot contain the separating colons.

Example result metadata (the native `data` remains unchanged):

```json
{
  "semantic_version": "3",
  "template_id": "customer-orders",
  "template_version": "1",
  "ontology_context": {
    "ontology_id": "commerce",
    "version": "2",
    "concept_refs": ["ontology:entity_type:Customer", "ontology:relation_type:places"]
  }
}
```

Business descriptions are untrusted context. They cannot override Agent instructions or the service's authorization and read-only policies.

## Import, APIs and storage

Source semantic JSON is now **format version 2** and includes optional `ontology` and template `concept_refs`. Version 1 files remain readable and importable. Existing sources stay unbound and retain native query access; existing template trials remain valid. Imports always create drafts and never import validation proof. Missing referenced ontology versions must be repaired before publication.

Ontology JSON exports contain `{id, definition}`. Create an ontology using that ID to preserve references in a new deployment; importing into an existing ontology retains the destination ID. Immutable version numbers are local to each ontology's publication history. When moving configurations between deployments, explicitly choose the corresponding destination version. Exports contain definitions and mappings, never database credentials, trials, rows or audit data.

Administrator endpoints, all protected by session cookies and CSRF on mutations:

| Path | Operations |
|---|---|
| `/api/ontologies` | `GET` summaries; `POST` create/import `{id?, definition}` |
| `/api/ontologies/{id}` | `GET` draft/status/usage; `PUT` `{revision, definition}`; `DELETE` `{revision}` |
| `/api/ontologies/{id}/{validate,publish,discard}` | `POST` `{revision}` |
| `/api/ontologies/{id}/archive` | `POST` `{revision, archived}` |
| `/api/ontologies/{id}/versions` | `GET`, optional `before` version, at most 100 IDs |
| `/api/ontologies/{id}/versions/{version}` | `GET` immutable definition; `DELETE` `{revision}` if unreferenced and not latest |
| `/api/ontologies/{id}/versions/{version}/diff?from=1` | `GET` definition differences |
| `/api/ontologies/{id}/export?version=1` | `GET` version export; omit `version` for draft |
| `/api/ontologies/{id}/import` | `POST` `{revision, definition}` |
| `/api/sources/{id}/semantics/check-mapping` | `POST` `{revision}`; metadata evidence only |
| `/api/sources/{id}/semantics/preview` | `POST` `{agent_id, keyword?, kind?, limit?, cursor?, entry_id?}` |

Revisions and versions are JSON strings to avoid browser integer rounding. Stale writes/publishes return HTTP 409. Ontologies and immutable versions are AES-GCM encrypted in SQLite, using the existing independent master key. Source mappings share the encrypted semantic snapshot transaction; source deletion atomically clears their references. Limits are 200 ontologies, 500 definitions and 512 KiB per ontology; a source semantic snapshot remains capped at 768 KiB and 500 catalog entries.

Template audit records and OTLP Logs add `ontology_id` and `ontology_version`. Definitions, query text, parameters and results are not logged. See [audit export](audit-export.md).

## Reproduce the commerce example

The [example files](../examples/ontologies/) map the same Customer/Order ontology to PostgreSQL tables and MongoDB collections with different physical names. With the existing `mcpdbhub-dev` OrbStack container and `mcpdbhub-test` network:

```sh
python3 scripts/verify-ontology.py --keep
# Existing matrix-owned fixtures can be reused without replacing their containers:
python3 scripts/verify-ontology.py --reuse
MCPDBHUB_REUSE_FIXTURES=postgres,mongodb python3 scripts/matrix.py
```

The script seeds explicitly isolated fixtures, uses read-only database accounts for the Hub, and verifies complex native queries, exact decimals, empty results, parameter rejection, source projection, version lifecycle, cursor invalidation and request-start context. It writes a sanitized [verification record](verification/ontology.json). The [full matrix](verification/matrix.json) independently exercises all seven native query families with ontology-associated templates.
