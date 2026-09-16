import { t } from "./i18n";
import { useState, type ReactNode } from "react";
import { Button, Drawer, ErrorNote, Field } from "./components";
import { message } from "./api";
import {
  entryKinds,
  type ObjectReference,
  type SemanticEntry,
  type SemanticParameter,
} from "./semantic-types";

export function ReferenceFields({
  value,
  onChange,
  prefix,
}: {
  value: ObjectReference;
  onChange: (v: ObjectReference) => void;
  prefix: string;
}) {
  return (
    <div className="field-grid semantic-reference">
      <Field label={t("{prefix} namespace", { prefix: prefix })}>
        <input
          value={value.namespace}
          onChange={(e) => onChange({ ...value, namespace: e.target.value })}
        />
      </Field>
      <Field label={t("{prefix} object", { prefix: prefix })} required>
        <input
          required
          value={value.object}
          onChange={(e) => onChange({ ...value, object: e.target.value })}
        />
      </Field>
      <Field label={t("{prefix} field path", { prefix: prefix })}>
        <input
          value={value.field || ""}
          onChange={(e) => onChange({ ...value, field: e.target.value })}
        />
      </Field>
    </div>
  );
}

export function ParameterFields({
  value,
  onChange,
  remove,
  slots,
  children,
}: {
  value: SemanticParameter;
  onChange: (v: SemanticParameter) => void;
  remove: () => void;
  slots: string[];
  children?: ReactNode;
}) {
  const field = <K extends keyof SemanticParameter>(
    key: K,
    v: SemanticParameter[K],
  ) => onChange({ ...value, [key]: v });
  return (
    <section className="semantic-parameter">
      <div className="field-grid">
        <Field label={t("Parameter name")} required>
          <input
            required
            value={value.name}
            onChange={(e) => field("name", e.target.value)}
          />
        </Field>
        <Field label={t("Parameter type")}>
          <select
            value={value.type}
            onChange={(e) => field("type", e.target.value)}
          >
            {[
              "string",
              "integer",
              "number",
              "boolean",
              "object",
              "array",
              "null",
            ].map((v) => (
              <option key={v} value={v}>
                {t(v)}
              </option>
            ))}
          </select>
        </Field>
      </div>
      <Field label={t("Parameter description")}>
        <input
          value={value.description || ""}
          onChange={(e) => field("description", e.target.value)}
        />
      </Field>
      {children}
      <details
        className="advanced"
        open={value.pointers.every((pointer) => !pointer)}
      >
        <summary>{t("Advanced bindings and constraints")}</summary>
        {slots.length > 0 && (
          <Field label={t("Bind to a query value")}>
            <select
              value=""
              onChange={(e) => {
                if (e.target.value && !value.pointers.includes(e.target.value))
                  field("pointers", [
                    ...value.pointers.filter(Boolean),
                    e.target.value,
                  ]);
              }}
            >
              <option value="">{t("Choose a parameter position")}</option>
              {slots.map((slot) => (
                <option key={slot} value={slot}>
                  {slot}
                </option>
              ))}
            </select>
          </Field>
        )}

        <Field
          label={t("Selected bindings")}
          hint={t(
            "Choose existing value positions above, or enter JSON Pointers. Query text and object names cannot be parameters.",
          )}
        >
          <textarea
            required
            rows={2}
            value={value.pointers.join("\n")}
            onChange={(e) => field("pointers", e.target.value.split("\n"))}
          />
        </Field>
        <label className="checkbox-row">
          <input
            type="checkbox"
            checked={value.required}
            onChange={(e) => field("required", e.target.checked)}
          />{" "}
          {t("Required parameter")}
        </label>
        <div className="field-grid">
          <Field
            label={t("Default value (JSON)")}
            hint={t(
              "Leave blank to use the template's fixed value for an optional parameter.",
            )}
          >
            <input
              value={value.default_json || ""}
              onChange={(e) => field("default_json", e.target.value)}
            />
          </Field>
          <Field label={t("Allowed values (JSON array)")}>
            <input
              value={value.enum_json || ""}
              onChange={(e) => field("enum_json", e.target.value)}
            />
          </Field>
          <Field label={t("Minimum")}>
            <input
              inputMode="decimal"
              value={value.minimum || ""}
              onChange={(e) => field("minimum", e.target.value)}
            />
          </Field>
          <Field label={t("Maximum")}>
            <input
              inputMode="decimal"
              value={value.maximum || ""}
              onChange={(e) => field("maximum", e.target.value)}
            />
          </Field>
        </div>
      </details>
      <Button type="button" className="danger" onClick={remove}>
        {t("Remove parameter")}
      </Button>
    </section>
  );
}

export function SemanticEditor({
  entry,
  entries,
  onClose,
  onSave,
}: {
  entry: SemanticEntry;
  entries: SemanticEntry[];
  onClose: () => void;
  onSave: (v: SemanticEntry) => Promise<void>;
}) {
  const [form, setForm] = useState(() => structuredClone(entry));
  const [enumText, setEnumText] = useState(
    JSON.stringify(entry.enums || {}, null, 2),
  );
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const update = <K extends keyof SemanticEntry>(
    key: K,
    value: SemanticEntry[K],
  ) => setForm((f) => ({ ...f, [key]: value }));
  return (
    <Drawer
      wide
      title={t("Edit catalog entry")}
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button type="button" disabled={busy} onClick={onClose}>
            {t("Cancel")}
          </Button>
          <Button primary form="semantic-entry-form" type="submit" busy={busy}>
            {t("Save draft entry")}
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      <form
        id="semantic-entry-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            const enums = JSON.parse(enumText);
            if (
              !enums ||
              typeof enums !== "object" ||
              Array.isArray(enums) ||
              Object.values(enums).some((v) => typeof v !== "string")
            )
              throw new Error(
                t("Enum meanings must be a JSON object with string values."),
              );
            await onSave({ ...form, enums });
          } catch (e) {
            setError(message(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <div className="field-grid">
          <Field
            label={t("Entry ID")}
            hint={t("Stable reference used by Agents.")}
          >
            <input value={form.id} readOnly />
          </Field>
          <Field label={t("Entry kind")}>
            <select
              value={form.kind}
              onChange={(e) => {
                const kind = e.target.value;
                setForm((f) => ({
                  ...f,
                  kind,
                  reference: ["object", "field"].includes(kind)
                    ? f.reference || { namespace: "", object: "" }
                    : undefined,
                  related:
                    kind === "relationship"
                      ? f.related || [
                          { namespace: "", object: "" },
                          { namespace: "", object: "" },
                        ]
                      : f.related,
                }));
              }}
            >
              {entryKinds.map((v) => (
                <option key={v} value={v}>
                  {t(v)}
                </option>
              ))}
            </select>
          </Field>
        </div>
        <Field label={t("Name")} required>
          <input
            required
            maxLength={256}
            value={form.name}
            onChange={(e) => update("name", e.target.value)}
          />
        </Field>
        <Field
          label={t("Aliases")}
          hint={t("One alias per line. Business content may use any language.")}
        >
          <textarea
            rows={2}
            value={(form.aliases || []).join("\n")}
            onChange={(e) => update("aliases", e.target.value.split("\n"))}
          />
        </Field>
        <Field label={t("Definition")}>
          <textarea
            rows={3}
            value={form.description || ""}
            onChange={(e) => update("description", e.target.value)}
          />
        </Field>
        {["object", "field"].includes(form.kind) && (
          <ReferenceFields
            value={form.reference || { namespace: "", object: "" }}
            onChange={(v) => update("reference", v)}
            prefix={t("Reference")}
          />
        )}
        <>
          <div className="field-grid">
            <Field label={t("Data type")}>
              <input
                value={form.data_type || ""}
                onChange={(e) => update("data_type", e.target.value)}
              />
            </Field>
            <Field label={t("Unit")}>
              <input
                value={form.unit || ""}
                onChange={(e) => update("unit", e.target.value)}
              />
            </Field>
          </div>
          <Field
            label={t("Enum meanings (JSON)")}
            hint={t('For example {"paid":"Payment completed"}')}
          >
            <textarea
              rows={3}
              value={enumText}
              onChange={(e) => setEnumText(e.target.value)}
            />
          </Field>
          <Field label={t("Time definition")}>
            <textarea
              rows={2}
              value={form.time_definition || ""}
              onChange={(e) => update("time_definition", e.target.value)}
            />
          </Field>
          {form.kind === "metric" && (
            <>
              <Field label={t("Grain")}>
                <input
                  value={form.grain || ""}
                  onChange={(e) => update("grain", e.target.value)}
                />
              </Field>
              <Field label={t("Query template")}>
                <select
                  value={form.template_id || ""}
                  onChange={(e) => update("template_id", e.target.value)}
                >
                  <option value="">{t("No linked template")}</option>
                  {entries
                    .filter((v) => v.template)
                    .map((v) => (
                      <option value={v.id} key={v.id}>
                        {v.name}
                      </option>
                    ))}
                </select>
              </Field>
            </>
          )}
          {form.kind === "relationship" && (
            <section className="form-section">
              <h3>{t("Related objects in this data source")}</h3>
              {(form.related || []).map((ref, i) => (
                <div key={i}>
                  <ReferenceFields
                    prefix={t("Reference {value1}", { value1: i + 1 })}
                    value={ref}
                    onChange={(v) =>
                      update(
                        "related",
                        form.related!.map((r, n) => (n === i ? v : r)),
                      )
                    }
                  />
                  <Button
                    type="button"
                    className="danger"
                    onClick={() =>
                      update(
                        "related",
                        form.related!.filter((_, n) => n !== i),
                      )
                    }
                  >
                    {t("Remove reference ")}
                    {i + 1}
                  </Button>
                </div>
              ))}
              <Button
                type="button"
                onClick={() =>
                  update("related", [
                    ...(form.related || []),
                    { namespace: "", object: "" },
                  ])
                }
              >
                {t("Add reference")}
              </Button>
            </section>
          )}
        </>
        <Field label={t("Caveats")}>
          <textarea
            rows={2}
            value={form.caveats || ""}
            onChange={(e) => update("caveats", e.target.value)}
          />
        </Field>
      </form>
    </Drawer>
  );
}
