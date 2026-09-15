# Cloud warehouse examples (preview)

These are configuration examples, not compatibility evidence. All four sources start **disabled** and in **Templates only** mode. No secrets are supplied.

1. Copy the source JSON into `create_data_source.configuration` on Configuration MCP, or the body of `POST /api/sources`. Replace every placeholder with administrator-provided values. Configure a dedicated reader in the cloud service first. BigQuery also supports `auth_mode: "service_account"` with the Google JSON credential encoded as a string in `password`; remove `token` when using that method.
2. Open the source in the UI, enable it, test the connection and inspect the permission evidence. No grants are inferred from a successful connection.
3. Adapt the matching semantic JSON's table, columns and business definition. Import it under **Semantics**, run the example trial and publish the draft. The example's large integer intentionally tests precision and may not exist in your data.
4. Grant a query Agent access. It can discover this template through `search_semantics` and call `execute_query_template` with its current execution version. Bind your published ontology using the usual source mapping workflow if needed; these files do not create or assume a shared ontology.

Snowflake uses positional `?`; Redshift uses positional `$1`; Databricks uses `:customer_id`; BigQuery uses `@customer_id`. Values are bound separately, never interpolated into SQL.

See [connection fields, credentials and limits](../../docs/cloud-warehouses.md) · [中文指南](../../docs/cloud-warehouses.zh-CN.md).
