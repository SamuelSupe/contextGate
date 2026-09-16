import { t } from "./i18n";
import { useEffect, useRef, useState } from "react";
import { Button, Drawer, ErrorNote, Field } from "./components";
import { APIError, message } from "./api";
import { useNavigationGuard } from "./useNavigationGuard";
import { OntologyCardinality } from "./OntologyCardinality";
import {
  effectiveEntities,
  downloadJSON,
  type EntityType,
  type OntologyDefinition,
  type Property,
  type RelationType,
} from "./ontology-types";

export type OntologyItem = EntityType | Property | RelationType;
export type OntologyKind = "entities" | "properties" | "relations";

export function newOntologyItem(kind: OntologyKind, owner = ""): OntologyItem {
  if (kind === "entities") return { id: "", name: "" };
  if (kind === "properties")
    return {
      id: "",
      name: "",
      entity: owner,
      type: "string",
      required: false,
      multiple: false,
    };
  return {
    id: "",
    name: "",
    from: owner,
    to: "",
    directed: true,
    from_cardinality: { min: 1, max: 1 },
    to_cardinality: { min: 0, max: null },
  };
}

function suggestedID(
  name: string,
  kind: OntologyKind,
  definition: OntologyDefinition,
  owner: string,
) {
  const slug = name
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  const fallback =
    kind === "entities"
      ? "entity"
      : kind === "properties"
        ? "property"
        : "relationship";
  const base = `${owner ? owner + "." : ""}${slug || fallback}`.slice(0, 88);
  const used = new Set(definition[kind].map((v) => v.id));
  let id = base;
  for (let i = 2; used.has(id); i++) id = `${base}-${i}`;
  return id;
}

export function OntologyEditor({
  kind,
  item,
  definition,
  existing,
  onSave,
  onClose,
  onReload,
}: {
  kind: OntologyKind;
  item: OntologyItem;
  definition: OntologyDefinition;
  existing: boolean;
  onSave: (item: OntologyItem, keepOpen: boolean) => Promise<void>;
  onClose: () => void;
  onReload: () => Promise<void>;
}) {
  const [form, setForm] = useState(() => structuredClone(item));
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [reloadRequested, setReloadRequested] = useState(false);
  const [manualID, setManualID] = useState(existing);
  const [closing, setClosing] = useState(false);
  const [constraintsOpen, setConstraintsOpen] = useState(() => {
    const p = kind === "properties" ? (item as Property) : null;
    return !!(
      p?.enums?.length ||
      p?.minimum ||
      p?.maximum ||
      p?.time_definition
    );
  });
  const nameInput = useRef<HTMLInputElement>(null);
  useEffect(() => {
    nameInput.current?.focus();
  }, []);
  const initial = useRef(structuredClone(item));
  const dirty = JSON.stringify(form) !== JSON.stringify(initial.current);
  const [busy, setBusy] = useState(false);
  useNavigationGuard(dirty, busy);
  const update = (patch: Partial<EntityType & Property & RelationType>) =>
    setForm((f) => ({ ...f, ...patch }));
  const entity = kind === "entities" ? (form as EntityType) : null;
  const property = kind === "properties" ? (form as Property) : null;
  const relation = kind === "relations" ? (form as RelationType) : null;
  function close() {
    if (busy) return;
    if (dirty) setClosing(true);
    else onClose();
  }
  function rename(name: string) {
    update({
      name,
      ...(!manualID
        ? {
            id: name.trim()
              ? suggestedID(
                  name,
                  kind,
                  definition,
                  property?.entity || relation?.from || "",
                )
              : "",
          }
        : {}),
    });
  }
  const entityName = (id: string) =>
    definition.entities.find((e) => e.id === id)?.name || "entity";
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
      title={t("{action} {kind}", {
        action: t(existing ? "Edit" : "Add"),
        kind: t(
          kind === "entities"
            ? "entity"
            : kind === "properties"
              ? "property"
              : "relationship",
        ),
      })}
      onClose={close}
      footer={
        <>
          <Button disabled={busy} onClick={close}>
            {t("Cancel")}
          </Button>
          {!existing && (
            <Button
              disabled={busy}
              type="submit"
              form="ontology-item-form"
              name="save-mode"
              value="another"
            >
              {t("Save & add another")}
            </Button>
          )}
          <Button
            id="ontology-save-entry"
            primary
            busy={busy}
            type="submit"
            form="ontology-item-form"
          >
            {t("Save to draft")}
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      {conflict && (
        <div className="notice warning">
          <div>
            <p>
              {t(
                "Your edit is still here. Export it before reloading the latest draft; reloading discards this form's changes.",
              )}
            </p>
            <div className="button-row">
              <Button
                disabled={busy}
                onClick={() =>
                  downloadJSON(
                    { kind, entry: form },
                    "unsaved-ontology-entry.json",
                  )
                }
              >
                {t("Export unsaved entry")}
              </Button>
              <Button disabled={busy} onClick={() => setReloadRequested(true)}>
                {t("Reload latest draft")}
              </Button>
            </div>
            {reloadRequested && (
              <div role="alert">
                <p>
                  {t("Discard this form's changes and load the latest draft?")}
                </p>
                <div className="button-row">
                  <Button
                    autoFocus
                    disabled={busy}
                    onClick={() => setReloadRequested(false)}
                  >
                    {t("Keep editing")}
                  </Button>
                  <Button
                    busy={busy}
                    onClick={async () => {
                      setBusy(true);
                      try {
                        await onReload();
                      } catch (e) {
                        setError(message(e));
                      } finally {
                        setBusy(false);
                      }
                    }}
                  >
                    {t("Discard edit and reload")}
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
      {closing && (
        <div className="notice warning" role="alert">
          <div>
            <strong>{t("You have unsaved changes.")}</strong>
            <p>{t("Keep editing or discard the changes in this form.")}</p>
            <div className="button-row">
              <Button autoFocus type="button" onClick={() => setClosing(false)}>
                {t("Keep editing")}
              </Button>
              <Button
                disabled={busy}
                type="button"
                className="danger"
                onClick={onClose}
              >
                {t("Discard changes")}
              </Button>
            </div>
          </div>
        </div>
      )}
      <form
        id="ontology-item-form"
        aria-busy={busy}
        onKeyDown={(e) => {
          if (
            e.key === "Enter" &&
            e.target instanceof HTMLInputElement &&
            !e.nativeEvent.isComposing
          ) {
            e.preventDefault();
            if (!busy)
              e.currentTarget.requestSubmit(
                document.getElementById(
                  "ontology-save-entry",
                ) as HTMLButtonElement,
              );
          }
        }}
        onSubmit={async (e) => {
          e.preventDefault();
          if (busy) return;
          setError("");
          setBusy(true);
          try {
            if (!form.name.trim()) throw new Error(t("Enter a name."));
            if (new TextEncoder().encode(form.name.trim()).length > 256)
              throw new Error(t("The name is too long. Use a shorter name."));
            if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,95}$/.test(form.id))
              throw new Error(
                t("Enter a valid reference ID under Reference ID & aliases."),
              );
            const keepOpen =
              !existing &&
              (e.nativeEvent as SubmitEvent).submitter?.getAttribute(
                "value",
              ) === "another";
            const cleaned = {
              ...form,
              name: form.name.trim(),
              aliases: (form.aliases || [])
                .map((v) => v.trim())
                .filter(Boolean),
            };
            if ("enums" in cleaned)
              cleaned.enums = (cleaned.enums || []).filter(Boolean);
            await onSave(cleaned, keepOpen);
            if (keepOpen) {
              const blank = newOntologyItem(
                kind,
                property?.entity || relation?.from || "",
              );
              initial.current = blank;
              setForm(blank);
              setManualID(false);
              setConstraintsOpen(false);
              setClosing(false);
              nameInput.current?.focus();
            }
          } catch (e) {
            setError(message(e));
            setConflict(e instanceof APIError && e.detail.code === "conflict");
          } finally {
            setBusy(false);
          }
        }}
      >
        <fieldset disabled={busy} className="ontology-form-fields">
          <Field label={t("Name")} required>
            <input
              ref={nameInput}
              autoFocus
              required
              maxLength={256}
              placeholder={
                entity
                  ? t("e.g. Customer")
                  : property
                    ? t("e.g. Email address")
                    : t("e.g. places")
              }
              value={form.name}
              onChange={(e) => rename(e.target.value)}
            />
          </Field>
          <Field label={t("Description")}>
            <textarea
              rows={2}
              value={form.description || ""}
              onChange={(e) => update({ description: e.target.value })}
            />
          </Field>
          {entity && (
            <>
              <Field label={t("Parent entity")}>
                <select
                  value={entity.parent || ""}
                  onChange={(e) => update({ parent: e.target.value })}
                >
                  <option value="">{t("No parent")}</option>
                  {definition.entities
                    .filter(
                      (e) =>
                        !effectiveEntities(definition, e.id).includes(
                          entity.id,
                        ),
                    )
                    .map((e) => (
                      <option key={e.id} value={e.id}>
                        {e.name}
                      </option>
                    ))}
                </select>
              </Field>
              <section className="form-section">
                <h3>{t("Identity property combination")}</h3>
                <p className="help">
                  {t(
                    "Choose required, single-valued properties. This declares identity; it does not verify database uniqueness. Save the entity and add its properties first.",
                  )}
                </p>
                <div
                  className="checkbox-list"
                  role="group"
                  aria-label={t("Identity property combination")}
                >
                  {identityOptions.map((p) => (
                    <label className="checkbox-row" key={p.id}>
                      <input
                        type="checkbox"
                        checked={(entity.identity || []).includes(p.id)}
                        disabled={
                          (!p.required || p.multiple) &&
                          !(entity.identity || []).includes(p.id)
                        }
                        onChange={(e) =>
                          update({
                            identity: e.target.checked
                              ? [...(entity.identity || []), p.id]
                              : entity.identity?.filter((id) => id !== p.id),
                          })
                        }
                      />
                      <span>
                        {p.name}
                        {p.entity !== entity.id && (
                          <small>{t(" (inherited)")}</small>
                        )}
                        {(!p.required || p.multiple) && (
                          <small>
                            {t("Requires a required, single-valued property")}
                          </small>
                        )}
                      </span>
                    </label>
                  ))}
                </div>
                {(entity.identity || [])
                  .filter((id) => !identityOptions.some((p) => p.id === id))
                  .map((id) => (
                    <p key={id} className="help">
                      {t("Missing identity property: ")}
                      {id}{" "}
                      <Button
                        type="button"
                        onClick={() =>
                          update({
                            identity: entity.identity?.filter((v) => v !== id),
                          })
                        }
                      >
                        {t("Remove reference")}
                      </Button>
                    </p>
                  ))}
              </section>
            </>
          )}
          {property && (
            <>
              <div className="field-grid">
                <Field label={t("Owner entity")} required>
                  <select
                    required
                    value={property.entity}
                    onChange={(e) => update({ entity: e.target.value })}
                  >
                    <option value="">{t("Select entity")}</option>
                    {entityOptions}
                  </select>
                </Field>
                <Field label={t("Value type")}>
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
                      <option key={v} value={v}>
                        {t(v)}
                      </option>
                    ))}
                  </select>
                </Field>
              </div>
              <fieldset className="ontology-property-rules">
                <legend>{t("Property rules")}</legend>
                <div className="ontology-property-options">
                  {(["required", "multiple", "unique"] as const).map((key) => (
                    <label key={key} className="ontology-property-option">
                      <input
                        type="checkbox"
                        checked={!!property[key]}
                        onChange={(e) => update({ [key]: e.target.checked })}
                      />
                      <span>
                        {key === "multiple"
                          ? t("Multiple values")
                          : key === "unique"
                            ? t("Declared unique")
                            : t("Required")}
                      </span>
                    </label>
                  ))}
                </div>
              </fieldset>
              <Field label={t("Unit")}>
                <input
                  value={property.unit || ""}
                  onChange={(e) => update({ unit: e.target.value })}
                />
              </Field>
              <details
                className="ontology-advanced"
                open={constraintsOpen}
                onToggle={(e) => setConstraintsOpen(e.currentTarget.open)}
              >
                <summary>
                  {t("Allowed values, ranges & time conventions")}
                </summary>
                <Field
                  label={t("Enumeration")}
                  hint={t("One logical value per line.")}
                >
                  <textarea
                    rows={3}
                    value={(property.enums || []).join("\n")}
                    onChange={(e) =>
                      update({ enums: e.target.value.split("\n") })
                    }
                  />
                </Field>
                <div className="field-grid">
                  <Field
                    label={t("Minimum")}
                    hint={t("Exact decimal or integer text.")}
                  >
                    <input
                      value={property.minimum || ""}
                      onChange={(e) => update({ minimum: e.target.value })}
                    />
                  </Field>
                  <Field label={t("Maximum")}>
                    <input
                      value={property.maximum || ""}
                      onChange={(e) => update({ maximum: e.target.value })}
                    />
                  </Field>
                </div>
                <Field label={t("Time convention")}>
                  <textarea
                    rows={2}
                    value={property.time_definition || ""}
                    onChange={(e) =>
                      update({ time_definition: e.target.value })
                    }
                  />
                </Field>
              </details>
            </>
          )}
          {relation && (
            <>
              <div className="field-grid">
                <Field label={t("From entity")} required>
                  <select
                    required
                    value={relation.from}
                    onChange={(e) => update({ from: e.target.value })}
                  >
                    <option value="">{t("Select entity")}</option>
                    {entityOptions}
                  </select>
                </Field>
                <Field label={t("To entity")} required>
                  <select
                    required
                    value={relation.to}
                    onChange={(e) => update({ to: e.target.value })}
                  >
                    <option value="">{t("Select entity")}</option>
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
                {t("Directed from origin to target")}
              </label>
              <div className="ontology-relationship-sentence">
                {entityName(relation.from)} →{" "}
                {relation.name || t("relationship")} → {entityName(relation.to)}
              </div>
              <OntologyCardinality
                label={t("{value1} per {value2} (origin)", {
                  value1: entityName(relation.from),
                  value2: entityName(relation.to),
                })}
                value={relation.from_cardinality}
                onChange={(from_cardinality) => update({ from_cardinality })}
              />
              <OntologyCardinality
                label={t("{value1} per {value2} (target)", {
                  value1: entityName(relation.to),
                  value2: entityName(relation.from),
                })}
                value={relation.to_cardinality}
                onChange={(to_cardinality) => update({ to_cardinality })}
              />
              <p className="help">
                {t(
                  "These counts describe your business rules; they do not validate database values.",
                )}
              </p>
            </>
          )}
          <details className="ontology-advanced">
            <summary>{t("Reference ID & aliases")}</summary>
            <Field
              label={t("Reference ID")}
              hint={
                existing
                  ? t(
                      "This stable reference is preserved when you rename the definition.",
                    )
                  : t(
                      "Generated from the name. Customize only when you need a specific reference.",
                    )
              }
            >
              <input
                maxLength={96}
                readOnly={existing}
                value={form.id}
                onChange={(e) => {
                  setManualID(true);
                  update({ id: e.target.value });
                }}
              />
            </Field>
            <Field
              label={t("Aliases")}
              hint={t(
                "One alias per line. Business content may use any language.",
              )}
            >
              <textarea
                rows={2}
                value={(form.aliases || []).join("\n")}
                onChange={(e) =>
                  update({ aliases: e.target.value.split("\n") })
                }
              />
            </Field>
          </details>
        </fieldset>
      </form>
    </Drawer>
  );
}
