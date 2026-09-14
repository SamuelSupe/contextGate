import { t } from "./i18n";
import { useState } from "react";
import { Field } from "./components";

type Cardinality = { min: number; max: number | null };
const choices = [
  { name: "Optional one (0…1)", min: 0, max: 1 },
  { name: "Exactly one (1)", min: 1, max: 1 },
  { name: "Zero or more (0…many)", min: 0, max: null },
  { name: "One or more (1…many)", min: 1, max: null },
];

export function cardinalityLabel(value: Cardinality) {
  if (value.max === null)
    return value.min === 0
      ? t("zero or more")
      : value.min === 1
        ? t("one or more")
        : t("at least {value1}", { value1: value.min });
  if (value.min === value.max)
    return value.min === 1
      ? t("exactly one")
      : t("exactly {value1}", { value1: value.min });
  return value.min === 0 && value.max === 1
    ? t("at most one")
    : t("{value1} to {value2}", { value1: value.min, value2: value.max });
}

export function OntologyCardinality({
  label,
  value,
  onChange,
}: {
  label: string;
  value: Cardinality;
  onChange: (value: Cardinality) => void;
}) {
  const [custom, setCustom] = useState(false);
  const index = choices.findIndex(
    (v) => v.min === value.min && v.max === value.max,
  );
  const showCustom = custom || index === -1;
  return (
    <section className="ontology-cardinality">
      <Field label={label}>
        <select
          value={showCustom ? "custom" : String(index)}
          onChange={(e) => {
            setCustom(e.target.value === "custom");
            if (e.target.value !== "custom") {
              const next = choices[Number(e.target.value)];
              onChange({ min: next.min, max: next.max });
            }
          }}
        >
          {choices.map((v, i) => (
            <option key={t(v.name)} value={i}>
              {t(v.name)}
            </option>
          ))}
          <option value="custom">{t("Custom range…")}</option>
        </select>
      </Field>
      {showCustom && (
        <div className="field-grid">
          <Field label={t("{label}: minimum", { label: label })}>
            <input
              required
              type="number"
              min={0}
              max={2147483647}
              value={value.min}
              onChange={(e) =>
                onChange({ ...value, min: Number(e.target.value) })
              }
            />
          </Field>
          <Field
            label={t("{label}: maximum", { label: label })}
            hint={t("Leave blank for unbounded.")}
          >
            <input
              type="number"
              min={value.min}
              max={2147483647}
              value={value.max ?? ""}
              onChange={(e) =>
                onChange({
                  ...value,
                  max: e.target.value === "" ? null : Number(e.target.value),
                })
              }
            />
          </Field>
        </div>
      )}
    </section>
  );
}
