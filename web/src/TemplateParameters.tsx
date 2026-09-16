import { t } from "./i18n";
import { useState } from "react";
import { Button, Field } from "./components";
import type { SemanticParameter } from "./semantic-types";
import {
  parameterChoices,
  parameterError,
  parameterJSON,
  parameterValues,
} from "./template-parameters";

export function TemplateParameters({
  parameters,
  value,
  example,
  disabled,
  onChange,
  compact = false,
}: {
  compact?: boolean;
  parameters: SemanticParameter[];
  value: string;
  example: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const [advanced, setAdvanced] = useState(false);
  let values: Record<string, string> = {},
    parseError = "";
  try {
    values = parameterValues(value);
    const unknown = Object.keys(values).filter(
      (key) => !parameters.some((p) => p.name === key),
    );
    if (unknown.length)
      parseError = `Unknown parameters: ${unknown.join(", ")}. Edit JSON to remove them.`;
  } catch {
    parseError = "Fix the JSON before returning to the form.";
  }
  const jsonMode = advanced || !!parseError;
  function update(name: string, raw?: string) {
    const next = Object.assign(Object.create(null), values);
    if (raw === undefined) delete next[name];
    else next[name] = raw;
    onChange(parameterJSON(next));
  }
  return (
    <section className="template-parameters">
      {!compact && (
        <div className="section-heading">
          <h3>{t("Parameters")}</h3>
          <div className="button-row">
            <Button disabled={disabled} onClick={() => onChange(example)}>
              {t("Use example values")}
            </Button>
            <Button
              disabled={disabled || !!parseError}
              aria-pressed={jsonMode}
              onClick={() => setAdvanced(!advanced)}
            >
              {jsonMode ? t("Use form") : t("Edit JSON")}
            </Button>
          </div>
        </div>
      )}
      {jsonMode ? (
        <Field
          label={t("Template parameters (JSON)")}
          hint={
            parseError ||
            t(
              "Numbers are sent exactly as entered. Only declared parameters are accepted.",
            )
          }
        >
          <textarea
            disabled={disabled}
            className="query-editor"
            rows={6}
            value={value}
            onChange={(e) => onChange(e.target.value)}
          />
        </Field>
      ) : !parameters.length ? (
        <p className="help">{t("This template needs no parameters.")}</p>
      ) : (
        parameters.map((p) => {
          const raw = values[p.name],
            included = raw !== undefined;
          const error = parameterError(p, raw),
            choices = parameterChoices(p);
          let display = raw || "";
          if (included) {
            try {
              const parsed = JSON.parse(raw);
              if (typeof parsed === "string") display = parsed;
            } catch {
              /* An invalid imported value remains editable. */
            }
          }
          const id = `parameter-${p.name}`;
          const required = p.required && !p.default_json;
          const common = {
            id,
            disabled: disabled || (!included && !required),
            "aria-invalid": !!error,
            "aria-describedby": `${id}-help`,
          };
          return (
            <div className="parameter-field" key={p.name}>
              <div className="parameter-label">
                <label htmlFor={id}>
                  {compact ? t("Example value") : p.name}
                  {p.required && !p.default_json ? " *" : ""}
                </label>
                {!compact && <span className="help">{t(p.type)}</span>}
                {!required && (
                  <label className="check-row">
                    <input
                      type="checkbox"
                      disabled={disabled}
                      checked={included}
                      onChange={(e) =>
                        update(
                          p.name,
                          e.target.checked
                            ? p.default_json ||
                                choices[0] ||
                                (p.type === "string"
                                  ? '""'
                                  : p.type === "boolean"
                                    ? "false"
                                    : p.type === "object"
                                      ? "{}"
                                      : p.type === "array"
                                        ? "[]"
                                        : p.type === "null"
                                          ? "null"
                                          : "0")
                            : undefined,
                        )
                      }
                    />
                    {p.default_json
                      ? t("Override default")
                      : t("Provide value")}
                  </label>
                )}
              </div>
              {choices.length || p.type === "boolean" || p.type === "null" ? (
                <select
                  {...common}
                  value={raw || ""}
                  onChange={(e) => update(p.name, e.target.value)}
                >
                  <option value="" disabled>
                    {p.default_json ? t("Default value") : t("Select a value")}
                  </option>
                  {included &&
                    ![...choices, "true", "false", "null"].includes(raw) && (
                      <option value={raw}>{raw}</option>
                    )}
                  {(choices.length
                    ? choices
                    : p.type === "null"
                      ? ["null"]
                      : ["true", "false"]
                  ).map((v) => (
                    <option key={v} value={v}>
                      {p.type === "string" ? JSON.parse(v) : v}
                    </option>
                  ))}
                </select>
              ) : p.type === "object" || p.type === "array" ? (
                <textarea
                  {...common}
                  rows={3}
                  value={display}
                  onChange={(e) => update(p.name, editableJSON(e.target.value))}
                />
              ) : (
                <input
                  {...common}
                  type="text"
                  inputMode={
                    p.type === "number" || p.type === "integer"
                      ? "decimal"
                      : "text"
                  }
                  value={display}
                  onChange={(e) =>
                    update(
                      p.name,
                      p.type === "string"
                        ? JSON.stringify(e.target.value)
                        : editableJSON(e.target.value),
                    )
                  }
                />
              )}
              <div id={`${id}-help`} className="help">
                {!compact && p.description && (
                  <span className="block">{p.description}</span>
                )}
                {p.default_json && (
                  <span className="block">
                    {included ? t("Default") : t("Using default")}:{" "}
                    {p.default_json}
                  </span>
                )}
                {(p.minimum || p.maximum) && (
                  <span className="block">
                    {p.minimum
                      ? t("Min {minimum}", { minimum: p.minimum })
                      : ""}
                    {p.minimum && p.maximum ? " · " : ""}
                    {p.maximum
                      ? t("Max {maximum}", { maximum: p.maximum })
                      : ""}
                  </span>
                )}
                {error && <span className="field-error">{error}</span>}
              </div>
            </div>
          );
        })
      )}
    </section>
  );
}

function editableJSON(text: string) {
  try {
    JSON.parse(text);
    return text;
  } catch {
    return JSON.stringify(text);
  }
}
