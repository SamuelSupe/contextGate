import { useState } from "react";
import { Button, Drawer, ErrorNote, Field } from "./components";
import { message } from "./api";
import {
  effectiveEntities,
  type EntityType,
  type OntologyDefinition,
  type Property,
  type RelationType,
} from "./ontology-types";

export type OntologyItem = EntityType | Property | RelationType;
export type OntologyKind = "entities" | "properties" | "relations";

export function OntologyEditor({
  kind,
  item,
  definition,
  existing,
  onSave,
  onClose,
}: {
  kind: OntologyKind;
  item: OntologyItem;
  definition: OntologyDefinition;
  existing: boolean;
  onSave: (item: OntologyItem) => Promise<void>;
  onClose: () => void;
}) {
  const [form, setForm] = useState(() => structuredClone(item));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const update = (patch: Partial<EntityType & Property & RelationType>) =>
    setForm((f) => ({ ...f, ...patch }));
  const entity = kind === "entities" ? (form as EntityType) : null;
  const property = kind === "properties" ? (form as Property) : null;
  const relation = kind === "relations" ? (form as RelationType) : null;
  const entityOptions = definition.entities.map((e) => (
    <option key={e.id} value={e.id}>
      {e.name} ({e.id})
    </option>
  ));
  const identityOptions = entity
    ? definition.properties.filter((p) =>
        effectiveEntities(
          {
            ...definition,
            entities: [
              ...definition.entities.filter((e) => e.id !== entity.id),
              entity,
            ],
          },
          entity.id,
        ).includes(p.entity),
      )
    : [];
  return (
    <Drawer
      wide
      title={`${existing ? "Edit" : "Add"} ${kind === "entities" ? "entity type" : kind === "properties" ? "property" : "relation type"}`}
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={busy} onClick={onClose}>
            Cancel
          </Button>
          <Button primary busy={busy} type="submit" form="ontology-item-form">
            Save draft definition
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      <form
        id="ontology-item-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setError("");
          setBusy(true);
          try {
            await onSave(form);
          } catch (e) {
            setError(message(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <div className="field-grid">
          <Field
            label="Stable ID"
            required
            hint="Letters, digits, periods, underscores and hyphens. Published mappings reference this ID."
          >
            <input
              required
              pattern="[a-zA-Z0-9][a-zA-Z0-9_.-]*"
              maxLength={96}
              readOnly={existing}
              value={form.id}
              onChange={(e) => update({ id: e.target.value })}
            />
          </Field>
          <Field label="Name" required>
            <input
              required
              maxLength={256}
              value={form.name}
              onChange={(e) => update({ name: e.target.value })}
            />
          </Field>
        </div>
        <Field
          label="Aliases"
          hint="One alias per line. Business content may use any language."
        >
          <textarea
            rows={2}
            value={(form.aliases || []).join("\n")}
            onChange={(e) => update({ aliases: e.target.value.split("\n") })}
          />
        </Field>
        <Field label="Description">
          <textarea
            rows={3}
            value={form.description || ""}
            onChange={(e) => update({ description: e.target.value })}
          />
        </Field>
        {entity && (
          <>
            <Field label="Parent entity">
              <select
                value={entity.parent || ""}
                onChange={(e) => update({ parent: e.target.value })}
              >
                <option value="">No parent</option>
                {definition.entities
                  .filter((e) => e.id !== entity.id)
                  .map((e) => (
                    <option key={e.id} value={e.id}>
                      {e.name}
                    </option>
                  ))}
              </select>
            </Field>
            <section className="form-section">
              <h3>Identity property combination</h3>
              <p className="help">
                Choose required, single-valued properties. This declares
                identity; it does not verify database uniqueness. Save the
                entity and add its properties first.
              </p>
              {identityOptions.map((p) => (
                <label className="checkbox-row" key={p.id}>
                  <input
                    type="checkbox"
                    checked={(entity.identity || []).includes(p.id)}
                    onChange={(e) =>
                      update({
                        identity: e.target.checked
                          ? [...(entity.identity || []), p.id]
                          : entity.identity?.filter((id) => id !== p.id),
                      })
                    }
                  />
                  {p.name}
                  {p.entity !== entity.id ? " (inherited)" : ""}
                </label>
              ))}
              {(entity.identity || [])
                .filter((id) => !identityOptions.some((p) => p.id === id))
                .map((id) => (
                  <p key={id} className="help">
                    Missing identity property: {id}{" "}
                    <Button
                      type="button"
                      onClick={() =>
                        update({
                          identity: entity.identity?.filter((v) => v !== id),
                        })
                      }
                    >
                      Remove reference
                    </Button>
                  </p>
                ))}
            </section>
          </>
        )}
        {property && (
          <>
            <div className="field-grid">
              <Field label="Owner entity" required>
                <select
                  required
                  value={property.entity}
                  onChange={(e) => update({ entity: e.target.value })}
                >
                  <option value="">Select entity</option>
                  {entityOptions}
                </select>
              </Field>
              <Field label="Logical type">
                <select
                  value={property.type}
                  onChange={(e) => update({ type: e.target.value })}
                >
                  {[
                    "string",
                    "boolean",
                    "integer",
                    "decimal",
                    "number",
                    "date",
                    "datetime",
                    "duration",
                    "binary",
                    "object",
                  ].map((v) => (
                    <option key={v}>{v}</option>
                  ))}
                </select>
              </Field>
            </div>
            <div className="button-row">
              {(["required", "multiple", "unique"] as const).map((key) => (
                <label key={key} className="checkbox-row">
                  <input
                    type="checkbox"
                    checked={!!property[key]}
                    onChange={(e) => update({ [key]: e.target.checked })}
                  />
                  {key === "multiple"
                    ? "Multiple values"
                    : key === "unique"
                      ? "Declared unique"
                      : "Required"}
                </label>
              ))}
            </div>
            <Field label="Unit">
              <input
                value={property.unit || ""}
                onChange={(e) => update({ unit: e.target.value })}
              />
            </Field>
            <Field label="Enumeration" hint="One logical value per line.">
              <textarea
                rows={3}
                value={(property.enums || []).join("\n")}
                onChange={(e) => update({ enums: e.target.value.split("\n") })}
              />
            </Field>
            <div className="field-grid">
              <Field label="Minimum" hint="Exact decimal or integer text.">
                <input
                  value={property.minimum || ""}
                  onChange={(e) => update({ minimum: e.target.value })}
                />
              </Field>
              <Field label="Maximum">
                <input
                  value={property.maximum || ""}
                  onChange={(e) => update({ maximum: e.target.value })}
                />
              </Field>
            </div>
            <Field label="Time convention">
              <textarea
                rows={2}
                value={property.time_definition || ""}
                onChange={(e) => update({ time_definition: e.target.value })}
              />
            </Field>
          </>
        )}
        {relation && (
          <>
            <div className="field-grid">
              <Field label="From entity" required>
                <select
                  required
                  value={relation.from}
                  onChange={(e) => update({ from: e.target.value })}
                >
                  <option value="">Select entity</option>
                  {entityOptions}
                </select>
              </Field>
              <Field label="To entity" required>
                <select
                  required
                  value={relation.to}
                  onChange={(e) => update({ to: e.target.value })}
                >
                  <option value="">Select entity</option>
                  {entityOptions}
                </select>
              </Field>
            </div>
            <label className="checkbox-row">
              <input
                type="checkbox"
                checked={relation.directed}
                onChange={(e) => update({ directed: e.target.checked })}
              />
              Directed from origin to target
            </label>
            {(["from_cardinality", "to_cardinality"] as const).map((key) => (
              <section className="form-section" key={key}>
                <h3>
                  {key === "from_cardinality"
                    ? "Origins per target"
                    : "Targets per origin"}
                </h3>
                <div className="field-grid">
                  <Field label="Minimum count">
                    <input
                      type="number"
                      min={0}
                      max={2147483647}
                      required
                      value={relation[key].min}
                      onChange={(e) =>
                        update({
                          [key]: {
                            ...relation[key],
                            min: Number(e.target.value),
                          },
                        })
                      }
                    />
                  </Field>
                  <Field
                    label="Maximum count"
                    hint="Leave blank for unbounded."
                  >
                    <input
                      type="number"
                      min={0}
                      max={2147483647}
                      value={relation[key].max ?? ""}
                      onChange={(e) =>
                        update({
                          [key]: {
                            ...relation[key],
                            max:
                              e.target.value === ""
                                ? null
                                : Number(e.target.value),
                          },
                        })
                      }
                    />
                  </Field>
                </div>
              </section>
            ))}
          </>
        )}
      </form>
    </Drawer>
  );
}
