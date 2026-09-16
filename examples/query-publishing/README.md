# Internal support query demo

[简体中文](README.zh-CN.md) · [Publishing guide](../../docs/query-publishing.md)

This fictional PostgreSQL example supplies three existing queries, business contracts and regression cases without an ontology. Use a **disposable sample database**, never a production database, to run `psql "$DEMO_DATABASE_URL" -f setup.sql`. The script creates a dedicated schema and fails if it already exists. Configure a separate login with `USAGE` on `support_demo` and `SELECT` on its tables; follow the [read-only account guide](../../docs/read-only-accounts.md). The login must have no inherited write privileges. Set its ContextGate source to **Templates only**.

Choose **Publish a query**, select the source, and paste the `query_json` of a template from `postgres-semantics.json` into **Advanced JSON**. Find its parameter slots, name them using the provided contract, fill the purpose/results and save. Trial and review before publication. Alternatively import the complete semantic file in the source's Semantics to create a draft, then open a saved query in the publishing flow. Importing does not publish or transfer trial evidence.

| Client task and input | Expected observable behavior |
| --- | --- |
| “Show customer 1's orders.” `customer_id=1` | Two rows: orders 101 and 102, including the cancelled order. Customer 999 produces an empty result. |
| “Has order 101 been refunded?” `order_id=101` | One settled USD 10.00 refund. Order 102 has no recorded refund. Do not equate absence with a completed zero refund. |
| “What is paid gross order amount from Sep 1 inclusive to Sep 4 exclusive, UTC?” | Two paid orders, USD 170.00; cancelled orders excluded, the USD 10.00 refund **not deducted**. This is not net revenue. |

Configure a query Agent and discover the templates with `search_semantics`/`get_semantic_entry`; use the returned execution version in `execute_query_template`. Confirm its request ID in the query workspace. A second Agent can reuse the same published templates after a source grant. This demo does not claim real-team reuse, answer-quality improvements or a production permission model.

For the HTTP branch, run the [existing HTTP API demo](../../docs/http-api.md), create its source, and select a configured operation in **Publish a query**. Parameter contracts and examples are copied without changing the fixed HTTP operation or substituting a URL. No customer samples are read to infer business meaning.

The customer/order identifiers in this example are query inputs, **not row-level authorization**. Use database accounts/views or a separately designed trusted end-user identity boundary for customer-facing access. Each query runs against one source; separate Agent calls have no shared snapshot guarantee.
