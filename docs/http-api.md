# HTTP API data sources

[简体中文](http-api.zh-CN.md) · [Documentation](README.md)

HTTP API sources connect JSON REST APIs to the same Agent grants, query limits, audit trail, semantic catalog, query templates and ontology mappings as database sources. Available in ContextGate 0.5.0 and later.

## Configure a source

Choose **Data sources → Add data source → Source type → HTTP API**.

1. Set the base URL, authentication and API contract version. HTTPS requires certificate and hostname verification; a custom CA is supported. HTTP requires explicitly disabling TLS. URLs must not contain credentials, query strings or fragments.
2. Define named **GET** or **POST** operations. Confirm that each only reads data and has no side effects. This confirmation is required for GET too: a method name does not prove read-only behavior.
3. Define scalar parameters, their request destinations, optional defaults, enums and numeric bounds. Provide real example parameters for connection checks.
4. Set the response data JSON Pointer, such as `/data/items`, or leave it blank to retain the complete response. An array becomes result documents; an object becomes one document. HTTP 204 returns no documents.
5. Optionally declare response fields and token pagination. Select a connection-check operation, then save and test.

Authentication supports no credentials, Basic Auth, Bearer tokens, and API keys in non-routing `X-` headers such as `X-API-Key`. The source's **Access token** holds the key; leave **API key header** blank for Bearer. Secrets use existing encrypted storage and write-only credential updates. Do not put secrets in paths, descriptions, request bodies, defaults or examples.

**The API is administrator-declared read-only.** ContextGate enforces the fixed request contract, not arbitrary upstream business behavior. Use read-scoped credentials and operations verified by the API owner. Do not configure generic write, script, SQL execution or remote-fetch endpoints as read operations. Connection success does not verify permissions. Only trusted administrators and configuration Agents should manage these sources; their fixed destinations may include private services.

## Request contracts

Agents call `query_http_api` with `source_id`, `operation` and `named_params`; the standard optional query limits and cursor also apply. They cannot supply URLs, methods, headers or request-body fragments.

| Parameter location | Allowed binding |
| --- | --- |
| `query` | A fixed query-string key, encoded as one scalar value |
| `path` | One required whole path segment, e.g. `{customer_id}`; slashes, percent encoding and traversal are rejected |
| `body` | An existing scalar JSON Pointer in a fixed POST body, e.g. `/filter/region` |

Parameter types are `string`, `integer`, `number` and `boolean`. Objects and arrays cannot replace operation structures. Request JSON is stored as text, parsed with exact numbers and never string-interpolated. Responses retain document structure; JSON numbers are encoded as exact strings, following ContextGate's existing lossless convention. Declared columns describe original types and are not inferred from results.

A source supports up to 40 operations, each with 32 parameters and 200 declared fields. Configuration is limited to 256 KiB, individual parameter values to 8 KiB and pagination tokens to 4 KiB. Existing timeout, byte, row and concurrency limits apply. There are no automatic retries; 429 is returned as `rate_limited` with `HTTP 429` and no upstream error body.

## Pagination and discovery

Token pagination reserves one query-string key and reads the next token from a fixed response JSON Pointer. The final page must contain `null` or an empty string there. ContextGate never follows next-page URLs, HTTP redirects or environment proxies. Tokens are opaque values sent only to the same configured operation. Engine cursors bind identity, source revision, query values and template execution version and expire after five minutes.

If a response exceeds the byte limit, the call fails with `result_too_large`. If the returned array is locally truncated, no next-page cursor is issued: lower the upstream page size and restart to avoid skipping unread rows. Offset/page-number APIs can expose their page parameter explicitly; automatic Link-header, offset increment and nested pagination are not supported.

Discovery uses namespace `api`; operations are objects, and declared response fields are columns. `describe_object` exposes the operation's business name, parameter contract and exact `example_json` text, without URLs, paths, credentials or headers. It does not fetch samples. HTTP ontology field checks remain **unverified**, even when a field is in the declared schema.

## Semantics and ontologies

Import an operation through **Semantics → Catalog → Import structure**, then describe its business meaning. Map ontology entities to `api.<operation_id>` and properties to the declared response field paths. No entity instances or relationships are inferred.

In **Query templates**, select a configured API operation to fill its query, parameter definitions, examples and result description. Required parameters and optional parameters with an example or default are included. Other optional parameters stay absent to preserve upstream behavior; add an example or default in the source operation before exposing them in a generated template. Review the generated contract, then trial and publish it.

A query template fixes the operation and uses `/named_params/<name>` bindings:

```json
{
  "operation": "list_customers",
  "named_params": {"region": "east"}
}
```

See [source configuration](../examples/http-api/source.json) and [importable semantic catalog](../examples/http-api/semantics.json). Trial and publish the template before query Agents execute it. **Templates only** blocks native HTTP API calls across MCP, stdio and Agent identity previews. Results keep native documents and add the existing semantic/template/ontology context. Audits and OTLP use `query_http_api` or `execute_query_template`; request parameters, response data and headers are not recorded.

API contract versions are maintained by the administrator. The probe reports `declared-api-contract:<version>`; this is not a detected server version. Update it whenever the upstream contract changes. Changes to API configuration, credentials or version invalidate template evidence and mapping checks; trial, recheck and publish to resume. An upstream change cannot be detected automatically without a version update. Name-only and limit-only source edits retain trial validity.

## Run the local example

From the repository root, using Docker or OrbStack:

```sh
docker run -d --name contextgate-http-api-demo \
  -p 127.0.0.1:19845:8080 \
  -v "$PWD/examples/http-api:/example:ro" \
  python:3.13-alpine python /example/server.py
```

For ContextGate in OrbStack use `http://host.docker.internal:19845`; for a host process use `http://localhost:19845`. The example has no authentication and no mutation routes. Configure the two operations from `source.json`, or use Configuration MCP `create_data_source` with that file as its `configuration` value. Import `semantics.json` through Semantics JSON import, trial, review and publish.

For MCP query clients:

```json
{"source_id":"YOUR_SOURCE_ID","operation":"list_customers","named_params":{"region":"east"}}
```

The first response includes Acme with the exact ID `9007199254740993`; the returned cursor retrieves Northwind. POST `/customers/search` uses the same filter and returns equivalent documents.

Outbound OAuth flows, arbitrary headers, cookies, OpenAPI import, XML, binary responses and streaming/SSE are outside this first HTTP API implementation.
