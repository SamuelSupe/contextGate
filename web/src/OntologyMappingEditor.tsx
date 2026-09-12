import { useState } from "react";
import { message } from "./api";
import { Button, Drawer, ErrorNote, Field } from "./components";
import {
  effectiveEntities,
  type EntityMapping,
  type OntologyBinding,
  type OntologyDefinition,
  type PropertyMapping,
  type RelationMapping,
} from "./ontology-types";
import type { ObjectReference, SemanticEntry } from "./semantic-types";

export type MappingKind = "entities" | "properties" | "relations";
export type MappingItem = EntityMapping | PropertyMapping | RelationMapping;
export const mappingKey = (value: MappingItem) =>
  "property" in value
    ? `${value.entity}:${value.property}`
    : "entity" in value
      ? value.entity
      : value.relation;

function PhysicalField({
  label,
  value,
  objects,
  onChange,
}: {
  label: string;
  value: ObjectReference;
  objects: ObjectReference[];
  onChange: (v: ObjectReference) => void;
}) {
  const selected = JSON.stringify({
    namespace: value.namespace,
    object: value.object,
  });
  return (
    <div className="field-grid">
      <Field label={`${label} object`} required>
        <select
          required
          value={selected}
          onChange={(e) =>
            onChange({ ...JSON.parse(e.target.value), field: value.field })
          }
        >
          <option value={JSON.stringify({ namespace: "", object: "" })}>
            Select mapped object
          </option>
          {objects.map((o) => (
            <option
              key={JSON.stringify(o)}
              value={JSON.stringify({
                namespace: o.namespace,
                object: o.object,
              })}
            >
              {o.namespace ? o.namespace + "." : ""}
              {o.object}
            </option>
          ))}
        </select>
      </Field>
      <Field label={`${label} field path`} required>
        <input
          required
          value={value.field || ""}
          onChange={(e) => onChange({ ...value, field: e.target.value })}
        />
      </Field>
    </div>
  );
}

export function OntologyMappingEditor({
  kind,
  item,
  existing,
  definition,
  binding,
  entries,
  onSave,
  onClose,
}: {
  kind: MappingKind;
  item: MappingItem;
  existing: boolean;
  definition: OntologyDefinition;
  binding: OntologyBinding;
  entries: SemanticEntry[];
  onSave: (item: MappingItem) => Promise<void>;
  onClose: () => void;
}) {
  const [form, setForm] = useState(() => structuredClone(item));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const update = (
    patch: Partial<EntityMapping & PropertyMapping & RelationMapping>,
  ) => setForm((f) => ({ ...f, ...patch }));
  const entity = kind === "entities" ? (form as EntityMapping) : null;
  const property = kind === "properties" ? (form as PropertyMapping) : null;
  const relation = kind === "relations" ? (form as RelationMapping) : null;
  const entityOptions = definition.entities.map((e) => (
    <option key={e.id} value={e.id}>
      {e.name} ({e.id})
    </option>
  ));
  const objects = (id: string) =>
    binding.entities.find((m) => m.entity === id)?.objects || [];
  const templates = entries.filter((en) => en.template?.enabled);
  const relationType = definition.relations.find(
    (r) => r.id === relation?.relation,
  );
  const emptyField = () => ({ namespace: "", object: "", field: "" });
  const templateSelect = (
    value: string,
    onChange: (v: string) => void,
    required = false,
  ) => (
    <Field label="Read-only query template" required>
      <select
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">Select template</option>
        {templates.map((t) => (
          <option value={t.id} key={t.id}>
            {t.name} ({t.id})
          </option>
        ))}
      </select>
    </Field>
  );
  return (
    <Drawer
      wide
      title={`${existing ? "Edit" : "Add"} ${kind === "entities" ? "entity" : kind === "properties" ? "property" : "relation"} mapping`}
      subtitle="Mappings describe this data source. They do not generate joins or query text."
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={busy} onClick={onClose}>
            Cancel
          </Button>
          <Button primary form="mapping-form" type="submit" busy={busy}>
            Save mapping to draft
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      <form
        id="mapping-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            await onSave(form);
          } catch (e) {
            setError(message(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        {entity && (
          <>
            <Field label="Entity type" required>
              <select
                required
                disabled={existing}
                value={entity.entity}
                onChange={(e) => update({ entity: e.target.value })}
              >
                <option value="">Select entity type</option>
                {entityOptions}
              </select>
            </Field>
            <h3>Physical objects</h3>
            <p className="help">
              An entity can map to multiple objects in this source. Use the
              source's native namespace and object names.
            </p>
            {entity.objects.map((o, i) => (
              <section className="semantic-parameter" key={i}>
                <div className="field-grid">
                  <Field label={`Object ${i + 1} namespace`}>
                    <input
                      value={o.namespace}
                      onChange={(e) =>
                        update({
                          objects: entity.objects.map((v, n) =>
                            n === i ? { ...v, namespace: e.target.value } : v,
                          ),
                        })
                      }
                    />
                  </Field>
                  <Field label={`Object ${i + 1} name`} required>
                    <input
                      required
                      value={o.object}
                      onChange={(e) =>
                        update({
                          objects: entity.objects.map((v, n) =>
                            n === i ? { ...v, object: e.target.value } : v,
                          ),
                        })
                      }
                    />
                  </Field>
                </div>
                <Button
                  type="button"
                  disabled={entity.objects.length === 1}
                  onClick={() =>
                    update({
                      objects: entity.objects.filter((_, n) => n !== i),
                    })
                  }
                >
                  Remove object
                </Button>
              </section>
            ))}
            <Button
              type="button"
              onClick={() =>
                update({
                  objects: [...entity.objects, { namespace: "", object: "" }],
                })
              }
            >
              Add physical object
            </Button>
          </>
        )}
        {property && (
          <>
            <Field label="Mapped entity" required>
              <select
                required
                disabled={existing}
                value={property.entity}
                onChange={(e) =>
                  update({
                    entity: e.target.value,
                    property: "",
                    reference: emptyField(),
                    template_id: "",
                  })
                }
              >
                <option value="">Select mapped entity</option>
                {binding.entities.map((m) => (
                  <option key={m.entity} value={m.entity}>
                    {definition.entities.find((e) => e.id === m.entity)?.name ||
                      m.entity}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Effective property" required>
              <select
                required
                disabled={existing}
                value={property.property}
                onChange={(e) => update({ property: e.target.value })}
              >
                <option value="">Select property</option>
                {definition.properties
                  .filter((p) =>
                    effectiveEntities(definition, property.entity).includes(
                      p.entity,
                    ),
                  )
                  .map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({p.id})
                      {p.entity !== property.entity ? " · inherited" : ""}
                    </option>
                  ))}
              </select>
            </Field>
            <Field label="Mapping method">
              <select
                value={property.reference ? "field" : "template"}
                onChange={(e) =>
                  update(
                    e.target.value === "field"
                      ? { reference: emptyField(), template_id: "" }
                      : { reference: undefined, template_id: "" },
                  )
                }
              >
                <option value="field">Physical field</option>
                <option value="template">Computed by query template</option>
              </select>
            </Field>
            {property.reference ? (
              <>
                <PhysicalField
                  label="Property"
                  value={property.reference}
                  objects={objects(property.entity)}
                  onChange={(v) => update({ reference: v })}
                />
                <label className="checkbox-row">
                  <input
                    type="checkbox"
                    checked={!!property.declared}
                    onChange={(e) => update({ declared: e.target.checked })}
                  />
                  Allow administrator declaration if this field cannot be
                  discovered
                </label>
                <p className="help">
                  Undiscoverable fields remain unverified. No business samples
                  are inspected.
                </p>
              </>
            ) : (
              templateSelect(
                property.template_id || "",
                (v) => update({ template_id: v }),
                true,
              )
            )}
          </>
        )}
        {relation && (
          <>
            <Field label="Relation type" required>
              <select
                required
                disabled={existing}
                value={relation.relation}
                onChange={(e) =>
                  update({ relation: e.target.value, fields: [] })
                }
              >
                <option value="">Select relation</option>
                {definition.relations.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name} ({r.from} → {r.to})
                  </option>
                ))}
              </select>
            </Field>
            <p className="help">
              Both endpoint entities must be mapped. Add field pairs, a query
              template, or both.
            </p>
            {(relation.fields || []).map((pair, i) => (
              <section className="semantic-parameter" key={i}>
                <PhysicalField
                  label={`Origin ${i + 1}`}
                  value={pair.from}
                  objects={objects(relationType?.from || "")}
                  onChange={(v) =>
                    update({
                      fields: relation.fields!.map((p, n) =>
                        n === i ? { ...p, from: v } : p,
                      ),
                    })
                  }
                />
                <PhysicalField
                  label={`Target ${i + 1}`}
                  value={pair.to}
                  objects={objects(relationType?.to || "")}
                  onChange={(v) =>
                    update({
                      fields: relation.fields!.map((p, n) =>
                        n === i ? { ...p, to: v } : p,
                      ),
                    })
                  }
                />
                <label className="checkbox-row">
                  <input
                    type="checkbox"
                    checked={!!pair.declared}
                    onChange={(e) =>
                      update({
                        fields: relation.fields!.map((p, n) =>
                          n === i ? { ...p, declared: e.target.checked } : p,
                        ),
                      })
                    }
                  />
                  Allow undiscoverable fields as unverified declarations
                </label>
                <Button
                  type="button"
                  onClick={() =>
                    update({
                      fields: relation.fields!.filter((_, n) => n !== i),
                    })
                  }
                >
                  Remove field pair
                </Button>
              </section>
            ))}
            <Button
              type="button"
              onClick={() =>
                update({
                  fields: [
                    ...(relation.fields || []),
                    { from: emptyField(), to: emptyField() },
                  ],
                })
              }
            >
              Add field pair
            </Button>
            {templateSelect(relation.template_id || "", (v) =>
              update({ template_id: v }),
            )}
          </>
        )}
        <Field label="Mapping description">
          <textarea
            rows={3}
            value={form.description || ""}
            onChange={(e) => update({ description: e.target.value })}
          />
        </Field>
      </form>
    </Drawer>
  );
}
