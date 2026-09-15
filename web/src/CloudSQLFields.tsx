import { Field } from "./components";
import { t } from "./i18n";
import type { Source } from "./types";

export function isCloudSQL(kind: string) {
  return ["snowflake", "databricks", "bigquery"].includes(kind);
}

export function CloudSQLFields({
  source,
  option,
  version,
}: {
  source: Source;
  option: (key: string, value: string) => void;
  version: (value: string) => void;
}) {
  const fields =
    source.kind === "snowflake"
      ? ([
          ["warehouse", "Warehouse", true],
          ["role", "Read-only role", true],
          ["schema", "Default schema", false],
        ] as const)
      : source.kind === "databricks"
        ? ([
            ["warehouse_id", "SQL warehouse ID", true],
            ["schema", "Default schema", false],
          ] as const)
        : ([
            ["location", "BigQuery location", true],
            ["schema", "Default dataset", false],
            ["maximum_bytes_billed", "Maximum bytes billed per query", true],
          ] as const);
  return (
    <>
      <div className="field-grid">
        {fields.map(([key, label, required]) => (
          <Field
            key={key}
            label={t(label)}
            required={required}
            hint={
              key === "maximum_bytes_billed"
                ? t(
                    "Default: 1 GiB (1073741824 bytes). Limits billed scanning, not returned rows; jobs may still incur cloud charges.",
                  )
                : undefined
            }
          >
            <input
              required={required}
              value={source.options?.[key] || ""}
              maxLength={1024}
              inputMode={key === "maximum_bytes_billed" ? "numeric" : undefined}
              placeholder={key === "location" ? "US / europe-west1" : undefined}
              onChange={(e) => option(key, e.target.value)}
            />
          </Field>
        ))}
      </div>
      {source.kind === "snowflake" && (
        <Field label={t("Snowflake token type")}>
          <select
            value={source.options?.token_type || "PROGRAMMATIC_ACCESS_TOKEN"}
            onChange={(e) => option("token_type", e.target.value)}
          >
            <option value="PROGRAMMATIC_ACCESS_TOKEN">
              {t("Programmatic access token (PAT)")}
            </option>
            <option value="OAUTH">{t("OAuth access token")}</option>
          </select>
        </Field>
      )}
      <Field
        label={t("Connection contract version")}
        required
        hint={t(
          "Cloud engine upgrades are not automatically detected. Change this version after an upstream upgrade, then trial and publish templates again.",
        )}
      >
        <input
          required
          maxLength={120}
          value={source.version || "1"}
          onChange={(e) => version(e.target.value)}
        />
      </Field>
      <label className="check-row">
        <input
          type="checkbox"
          required
          checked={source.options?.read_only_confirmed === "true"}
          onChange={(e) =>
            option("read_only_confirmed", String(e.target.checked))
          }
        />
        {t(
          "I have configured a dedicated read-only data role for this credential.",
        )}
      </label>
      <p className="help">
        {t(
          "This declaration does not verify cloud permissions. Only a restricted SELECT subset is accepted; unsupported dialect features are rejected.",
        )}
      </p>
    </>
  );
}
