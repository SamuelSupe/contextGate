import { Button, Field } from "./components";
import { t } from "./i18n";
import type { HTTPParameter } from "./types";

export function HTTPAPIParameters({
  parameters,
  method,
  onChange,
}: {
  parameters: HTTPParameter[];
  method: "GET" | "POST";
  onChange: (parameters: HTTPParameter[]) => void;
}) {
  function parameter(at: number, patch: Partial<HTTPParameter>) {
    onChange(parameters.map((p, i) => (i === at ? { ...p, ...patch } : p)));
  }
  return (
    <>
      <div className="http-section-heading">
        <h4>{t("Request parameters")}</h4>
        <Button
          disabled={parameters.length >= 32}
          onClick={() =>
            onChange([
              ...parameters,
              {
                name: "",
                in: "query",
                target: "",
                type: "string",
                required: true,
              },
            ])
          }
        >
          {t("Add parameter")}
        </Button>
      </div>
      {!parameters?.length && (
        <p className="help">
          {t("No parameters. The operation sends its fixed request.")}
        </p>
      )}
      {parameters.map((p, i) => (
        <div className="http-parameter" key={i}>
          <div className="field-grid">
            <Field label={t("Parameter name")} required>
              <input
                required
                value={p.name}
                onChange={(e) =>
                  parameter(i, {
                    name: e.target.value,
                    target:
                      !p.target || p.target === p.name
                        ? e.target.value
                        : p.target,
                  })
                }
              />
            </Field>
            <Field label={t("Value type")}>
              <select
                value={p.type}
                onChange={(e) =>
                  parameter(i, {
                    type: e.target.value as HTTPParameter["type"],
                  })
                }
              >
                {["string", "integer", "number", "boolean"].map((type) => (
                  <option key={type} value={type}>
                    {t(type)}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={t("Send value in")}>
              <select
                value={p.in}
                onChange={(e) =>
                  parameter(i, {
                    in: e.target.value as HTTPParameter["in"],
                    required: e.target.value === "path" || p.required,
                  })
                }
              >
                <option value="query">{t("Query parameter")}</option>
                <option value="path">{t("Path segment")}</option>
                {method === "POST" && (
                  <option value="body">{t("JSON body value")}</option>
                )}
              </select>
            </Field>
            <Field
              label={t("Destination")}
              required
              hint={
                p.in === "body"
                  ? t("JSON Pointer, for example /filter/customer_id")
                  : t("Query key or path placeholder name")
              }
            >
              <input
                required
                value={p.target}
                onChange={(e) => parameter(i, { target: e.target.value })}
              />
            </Field>
          </div>
          <div className="http-section-heading">
            <label className="check-row">
              <input
                type="checkbox"
                checked={p.required}
                disabled={p.in === "path"}
                onChange={(e) => parameter(i, { required: e.target.checked })}
              />
              {t("Required")}
            </label>
            <Button
              onClick={() => onChange(parameters.filter((_, at) => i !== at))}
            >
              {t("Remove parameter")}
            </Button>
          </div>
          <details className="advanced">
            <summary>{t("Defaults and constraints")}</summary>
            <Field label={t("Default value (JSON)")}>
              <input
                value={p.default_json || ""}
                onChange={(e) => parameter(i, { default_json: e.target.value })}
              />
            </Field>
            <Field label={t("Allowed values (JSON array)")}>
              <input
                placeholder='["active", "inactive"]'
                value={p.enum_json || ""}
                onChange={(e) => parameter(i, { enum_json: e.target.value })}
              />
            </Field>
            {["integer", "number"].includes(p.type) && (
              <div className="field-grid">
                <Field label={t("Minimum")}>
                  <input
                    value={p.minimum || ""}
                    onChange={(e) => parameter(i, { minimum: e.target.value })}
                  />
                </Field>
                <Field label={t("Maximum")}>
                  <input
                    value={p.maximum || ""}
                    onChange={(e) => parameter(i, { maximum: e.target.value })}
                  />
                </Field>
              </div>
            )}
          </details>
        </div>
      ))}
    </>
  );
}
