import type { QueryResult } from "./types";
import { t } from "./i18n";
import { useState } from "react";
import { api, message } from "./api";
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
  sourceID,
  sourceKind,
  label,
  value,
  objects,
  onChange,
}: {
  sourceID: string;
  sourceKind: string;
  label: string;
  value: ObjectReference;
  objects: ObjectReference[];
  onChange: (v: ObjectReference) => void;
}) {
  const [discovery, setDiscovery] = useState<{
    object: string;
    fields: { name: string; type: string }[];
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const discoverable = [
    "postgresql",
    "postgres",
    "mysql",
    "mariadb",
    "tidb",
    "cockroachdb",
    "timescaledb",
    "sqlite",
    "duckdb",
    "clickhouse",
    "cassandra",
    "scylladb",
    "http_api",
  ].includes(sourceKind);
  const selected = JSON.stringify({
    namespace: value.namespace,
    object: value.object,
  });
  return (
    <div className="field-grid">
      <Field label={t("{label} object", { label: label })} required>
        <select
          required
          value={selected}
          onChange={(e) =>
            onChange({ ...JSON.parse(e.target.value), field: value.field })
          }
        >
          <option value={JSON.stringify({ namespace: "", object: "" })}>
            {t("Select mapped object")}
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
      <Field label={t("{label} field path", { label: label })} required>
        <input
          required
          value={value.field || ""}
          onChange={(e) => onChange({ ...value, field: e.target.value })}
        />
      </Field>
      {discoverable && (
        <div className="field-discovery">
          <Button
            busy={busy}
            disabled={!value.object}
            onClick={async () => {
              setBusy(true);
              setError("");
              try {
                const params = new URLSearchParams({
                  operation: "describe",
                  namespace: value.namespace,
                  object: value.object,
                });
                const result = await api<QueryResult>(
                  "/api/sources/" + sourceID + "/objects?" + params,
                );
                const data = result.data as {
                  name: string;
                  type: string;
                  columns?: { name: string; type: string }[];
                }[];
                setDiscovery({
                  object: selected,
                  fields:
                    sourceKind === "http_api"
                      ? data.flatMap((o) => o.columns || [])
                      : data,
                });
              } catch (e) {
                setError(message(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            {t("Discover fields")}
          </Button>
          {discovery?.object === selected && (
            <Field label={t("Available fields")}>
              <select
                value=""
                onChange={(e) =>
                  e.target.value &&
                  onChange({ ...value, field: e.target.value })
                }
              >
                <option value="">{t("Choose a field")}</option>
                {discovery.fields.map((f) => (
                  <option key={f.name} value={f.name}>
                    {f.name} · {f.type}
                  </option>
                ))}
              </select>
              <p className="help">
                {sourceKind === "http_api"
                  ? t(
                      "API fields are administrator declared, not verified against response samples.",
                    )
                  : t(
                      "Choose metadata or enter a field path manually. Check the saved mapping before publishing.",
                    )}
              </p>
              {!discovery.fields.length && (
                <p className="help">
                  {t(
                    "No discoverable fields. Enter a path and explicitly allow a declaration if needed.",
                  )}
                </p>
              )}
            </Field>
          )}
          <ErrorNote error={error} />
        </div>
      )}
    </div>
  );
}

export function OntologyMappingEditor({
  sourceID,
  sourceKind,
  kind,
  item,
  existing,
  definition,
  binding,
  entries,
  onSave,
  onClose,
}: {
  sourceID: string;
  sourceKind: string;
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
    <Field label={t("Read-only query template")} required>
      <select
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">{t("Select template")}</option>
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
      title={t("{value1} {value2} mapping", {
        value1: existing ? t("Edit") : t("Add"),
        value2:
          kind === "entities"
            ? t("entity")
            : kind === "properties"
              ? t("property")
              : t("relation"),
      })}
      subtitle={t(
        "Mappings describe this data source. They do not generate joins or query text.",
      )}
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={busy} onClick={onClose}>
            {t("Cancel")}
          </Button>
          <Button primary form="mapping-form" type="submit" busy={busy}>
            {t("Save mapping to draft")}
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
            <Field label={t("Entity type")} required>
              <select
                required
                disabled={existing}
                value={entity.entity}
                onChange={(e) => update({ entity: e.target.value })}
              >
                <option value="">{t("Select entity type")}</option>
                {entityOptions}
              </select>
            </Field>
            <h3>{t("Physical objects")}</h3>
            <p className="help">
              {t(
                "An entity can map to multiple objects in this source. Use the source's native namespace and object names.",
              )}
            </p>
            {entity.objects.map((o, i) => (
              <section className="semantic-parameter" key={i}>
                <div className="field-grid">
                  <Field
                    label={t("Object {value1} namespace", { value1: i + 1 })}
                  >
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
                  <Field
                    label={t("Object {value1} name", { value1: i + 1 })}
                    required
                  >
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
                  {t("Remove object")}
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
              {t("Add physical object")}
            </Button>
          </>
        )}
        {property && (
          <>
            <Field label={t("Mapped entity")} required>
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
                <option value="">{t("Select mapped entity")}</option>
                {binding.entities.map((m) => (
                  <option key={m.entity} value={m.entity}>
                    {definition.entities.find((e) => e.id === m.entity)?.name ||
                      m.entity}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={t("Effective property")} required>
              <select
                required
                disabled={existing}
                value={property.property}
                onChange={(e) => update({ property: e.target.value })}
              >
                <option value="">{t("Select property")}</option>
                {definition.properties
                  .filter((p) =>
                    effectiveEntities(definition, property.entity).includes(
                      p.entity,
                    ),
                  )
                  .map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({p.id})
                      {p.entity !== property.entity ? t(" · inherited") : ""}
                    </option>
                  ))}
              </select>
            </Field>
            <Field label={t("Mapping method")}>
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
                <option value="field">{t("Physical field")}</option>
                <option value="template">
                  {t("Computed by query template")}
                </option>
              </select>
            </Field>
            {property.reference ? (
              <>
                <PhysicalField
                  sourceID={sourceID}
                  sourceKind={sourceKind}
                  label={t("Property")}
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
                  <span>
                    {t(
                      "Allow administrator declaration if this field cannot be discovered",
                    )}
                    <small>
                      {t(
                        "Undiscoverable fields remain unverified. No business samples are inspected.",
                      )}
                    </small>
                  </span>
                </label>
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
            <Field label={t("Relation type")} required>
              <select
                required
                disabled={existing}
                value={relation.relation}
                onChange={(e) =>
                  update({ relation: e.target.value, fields: [] })
                }
              >
                <option value="">{t("Select relation")}</option>
                {definition.relations.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name} ({r.from} → {r.to})
                  </option>
                ))}
              </select>
            </Field>
            <p className="help">
              {t(
                "Both endpoint entities must be mapped. Add field pairs, a query template, or both.",
              )}
            </p>
            {(relation.fields || []).map((pair, i) => (
              <section className="semantic-parameter" key={i}>
                <PhysicalField
                  sourceID={sourceID}
                  sourceKind={sourceKind}
                  label={t("Origin {value1}", { value1: i + 1 })}
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
                  sourceID={sourceID}
                  sourceKind={sourceKind}
                  label={t("Target {value1}", { value1: i + 1 })}
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
                  {t("Allow undiscoverable fields as unverified declarations")}
                </label>
                <Button
                  type="button"
                  onClick={() =>
                    update({
                      fields: relation.fields!.filter((_, n) => n !== i),
                    })
                  }
                >
                  {t("Remove field pair")}
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
              {t("Add field pair")}
            </Button>
            {templateSelect(relation.template_id || "", (v) =>
              update({ template_id: v }),
            )}
          </>
        )}
        <Field label={t("Mapping description")}>
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
