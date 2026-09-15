import { useState } from "react";
import { Button, Field } from "./components";
import { t } from "./i18n";
import { parameterJSON, parameterValues } from "./template-parameters";

export function NativeQueryEditor({
  tool,
  value,
  onChange,
}: {
  tool: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const [advanced, setAdvanced] = useState(false);
  let fields: Record<string, string> = {};
  let query: unknown;
  try {
    fields = parameterValues(value);
    query = fields.query === undefined ? "" : JSON.parse(fields.query);
  } catch {
    query = undefined;
  }
  const textMode =
    ["query_sql", "query_cql", "query_cypher", "query_influxdb"].includes(
      tool,
    ) && typeof query === "string";
  return (
    <div className="native-query-editor">
      {textMode && (
        <div className="button-row">
          <Button aria-pressed={!advanced} onClick={() => setAdvanced(false)}>
            {t("Query text")}
          </Button>
          <Button aria-pressed={advanced} onClick={() => setAdvanced(true)}>
            {t("Advanced JSON")}
          </Button>
        </div>
      )}
      {textMode && !advanced ? (
        <>
          <Field
            label={t("Native query")}
            required
            hint={t(
              "Use native parameter placeholders. Values are bound separately; no string interpolation.",
            )}
          >
            <textarea
              className="query-editor"
              required
              rows={9}
              spellCheck={false}
              value={query as string}
              onChange={(e) =>
                onChange(
                  parameterJSON({
                    ...fields,
                    query: JSON.stringify(e.target.value),
                  }),
                )
              }
            />
          </Field>
          <details className="advanced">
            <summary>{t("Fixed parameter values and query options")}</summary>
            <p className="help">
              {t(
                "Add parameter slots here before choosing their bindings. Values remain lossless JSON.",
              )}
            </p>
            <Field label={t("Query options (JSON)")}>
              <textarea
                className="query-editor"
                rows={4}
                key={tool}
                defaultValue={parameterJSON(
                  Object.fromEntries(
                    Object.entries(fields).filter(([key]) => key !== "query"),
                  ),
                )}
                onChange={(e) => {
                  try {
                    const options = parameterValues(e.target.value);
                    if (Object.hasOwn(options, "query")) throw new Error();
                    onChange(
                      parameterJSON({
                        ...options,
                        query: fields.query || '""',
                      }),
                    );
                    e.target.setCustomValidity("");
                  } catch {
                    e.target.setCustomValidity(t("Enter a valid JSON object."));
                  }
                }}
                onBlur={(e) => e.target.reportValidity()}
              />
            </Field>
          </details>
        </>
      ) : (
        <Field
          label={t("Native query (JSON)")}
          required
          hint={t(
            "Use the native tool's query structure. Do not include source_id, cursor or query limits.",
          )}
        >
          <textarea
            className="query-editor"
            required
            spellCheck={false}
            rows={10}
            value={value}
            onChange={(e) => onChange(e.target.value)}
          />
        </Field>
      )}
    </div>
  );
}
