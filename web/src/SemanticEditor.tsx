import { useState } from "react";
import { Button, Drawer, ErrorNote, Field } from "./components";
import { message } from "./api";
import {
  entryKinds,
  type ObjectReference,
  type SemanticEntry,
  type SemanticParameter,
} from "./semantic-types";

function ReferenceFields({
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
      <Field label={`${prefix} namespace`}>
        <input
          value={value.namespace}
          onChange={(e) => onChange({ ...value, namespace: e.target.value })}
        />
      </Field>
      <Field label={`${prefix} object`} required>
        <input
          required
          value={value.object}
          onChange={(e) => onChange({ ...value, object: e.target.value })}
        />
      </Field>
      <Field label={`${prefix} field path`}>
        <input
          value={value.field || ""}
          onChange={(e) => onChange({ ...value, field: e.target.value })}
        />
      </Field>
    </div>
  );
}

function ParameterFields({
  value,
  onChange,
  remove,
}: {
  value: SemanticParameter;
  onChange: (v: SemanticParameter) => void;
  remove: () => void;
}) {
  const field = <K extends keyof SemanticParameter>(
    key: K,
    v: SemanticParameter[K],
  ) => onChange({ ...value, [key]: v });
  return (
    <section className="semantic-parameter">
      <div className="field-grid">
        <Field label="Parameter name" required>
          <input
            required
            value={value.name}
            onChange={(e) => field("name", e.target.value)}
          />
        </Field>
        <Field label="Parameter type">
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
              <option key={v}>{v}</option>
            ))}
          </select>
        </Field>
      </div>
      <Field label="Parameter description">
        <input
          value={value.description || ""}
          onChange={(e) => field("description", e.target.value)}
        />
      </Field>
      <Field
        label="JSON Pointer bindings"
        required
        hint="One existing value position per line, for example /params/0 or /filter/status. Query text and object names cannot be parameters."
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
        Required parameter
      </label>
      <div className="field-grid">
        <Field
          label="Default value (JSON)"
          hint="Leave blank to use the template's fixed value for an optional parameter."
        >
          <input
            value={value.default_json || ""}
            onChange={(e) => field("default_json", e.target.value)}
          />
        </Field>
        <Field label="Allowed values (JSON array)">
          <input
            value={value.enum_json || ""}
            onChange={(e) => field("enum_json", e.target.value)}
          />
        </Field>
        <Field label="Minimum">
          <input
            inputMode="decimal"
            value={value.minimum || ""}
            onChange={(e) => field("minimum", e.target.value)}
          />
        </Field>
        <Field label="Maximum">
          <input
            inputMode="decimal"
            value={value.maximum || ""}
            onChange={(e) => field("maximum", e.target.value)}
          />
        </Field>
      </div>
      <Button type="button" className="danger" onClick={remove}>
        Remove parameter
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
  const template = form.template;
  const editTemplate = (
    patch: Partial<NonNullable<SemanticEntry["template"]>>,
  ) => setForm((f) => ({ ...f, template: { ...f.template!, ...patch } }));
  return (
    <Drawer
      wide
      title={template ? "Edit query template" : "Edit catalog entry"}
      subtitle="Save changes to the draft. Agents only see published content."
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button type="button" disabled={busy} onClick={onClose}>
            Cancel
          </Button>
          <Button primary form="semantic-entry-form" type="submit" busy={busy}>
            Save draft entry
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
                "Enum meanings must be a JSON object with string values.",
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
          <Field label="Entry ID" hint="Stable reference used by Agents.">
            <input value={form.id} readOnly />
          </Field>
          {!template && (
            <Field label="Entry kind">
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
                  <option key={v}>{v}</option>
                ))}
              </select>
            </Field>
          )}
        </div>
        <Field label="Name" required>
          <input
            required
            maxLength={256}
            value={form.name}
            onChange={(e) => update("name", e.target.value)}
          />
        </Field>
        <Field
          label="Aliases"
          hint="One alias per line. Business content may use any language."
        >
          <textarea
            rows={2}
            value={(form.aliases || []).join("\n")}
            onChange={(e) => update("aliases", e.target.value.split("\n"))}
          />
        </Field>
        <Field label={template ? "Purpose" : "Definition"}>
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
            prefix="Reference"
          />
        )}
        {!template && (
          <>
            <div className="field-grid">
              <Field label="Data type">
                <input
                  value={form.data_type || ""}
                  onChange={(e) => update("data_type", e.target.value)}
                />
              </Field>
              <Field label="Unit">
                <input
                  value={form.unit || ""}
                  onChange={(e) => update("unit", e.target.value)}
                />
              </Field>
            </div>
            <Field
              label="Enum meanings (JSON)"
              hint={'For example {"paid":"Payment completed"}'}
            >
              <textarea
                rows={3}
                value={enumText}
                onChange={(e) => setEnumText(e.target.value)}
              />
            </Field>
            <Field label="Time definition">
              <textarea
                rows={2}
                value={form.time_definition || ""}
                onChange={(e) => update("time_definition", e.target.value)}
              />
            </Field>
            {form.kind === "metric" && (
              <>
                <Field label="Grain">
                  <input
                    value={form.grain || ""}
                    onChange={(e) => update("grain", e.target.value)}
                  />
                </Field>
                <Field label="Query template">
                  <select
                    value={form.template_id || ""}
                    onChange={(e) => update("template_id", e.target.value)}
                  >
                    <option value="">No linked template</option>
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
                <h3>Related objects in this data source</h3>
                {(form.related || []).map((ref, i) => (
                  <div key={i}>
                    <ReferenceFields
                      prefix={`Reference ${i + 1}`}
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
                      Remove reference {i + 1}
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
                  Add reference
                </Button>
              </section>
            )}
          </>
        )}
        <Field label="Caveats">
          <textarea
            rows={2}
            value={form.caveats || ""}
            onChange={(e) => update("caveats", e.target.value)}
          />
        </Field>
        {template && (
          <>
            <section className="form-section">
              <h3>
                Executable definition · <code>{template.tool}</code>
              </h3>
              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={template.enabled}
                  onChange={(e) => editTemplate({ enabled: e.target.checked })}
                />{" "}
                Enabled after publication
              </label>
              <Field
                label="Native query (JSON)"
                required
                hint="Use the native tool's query structure. Do not include source_id, cursor or query limits."
              >
                <textarea
                  className="query-editor"
                  required
                  spellCheck={false}
                  rows={10}
                  value={template.query_json}
                  onChange={(e) => editTemplate({ query_json: e.target.value })}
                />
              </Field>
              <h3>Parameters</h3>
              {(template.parameters || []).map((param, i) => (
                <ParameterFields
                  key={i}
                  value={param}
                  onChange={(v) =>
                    editTemplate({
                      parameters: template.parameters.map((p, n) =>
                        n === i ? v : p,
                      ),
                    })
                  }
                  remove={() =>
                    editTemplate({
                      parameters: template.parameters.filter((_, n) => n !== i),
                    })
                  }
                />
              ))}
              <Button
                type="button"
                onClick={() =>
                  editTemplate({
                    parameters: [
                      ...(template.parameters || []),
                      {
                        name: "",
                        type: "string",
                        required: true,
                        pointers: [""],
                      },
                    ],
                  })
                }
              >
                Add parameter
              </Button>
              <Field
                label="Example parameters (JSON)"
                required
                hint="A real read-only trial uses these values. Successful trial evidence stores no result data."
              >
                <textarea
                  className="query-editor"
                  required
                  spellCheck={false}
                  rows={4}
                  value={template.example_json}
                  onChange={(e) =>
                    editTemplate({ example_json: e.target.value })
                  }
                />
              </Field>
              <Field label="Result description">
                <textarea
                  rows={3}
                  value={template.result_description || ""}
                  onChange={(e) =>
                    editTemplate({ result_description: e.target.value })
                  }
                />
              </Field>
            </section>
          </>
        )}
      </form>
    </Drawer>
  );
}
