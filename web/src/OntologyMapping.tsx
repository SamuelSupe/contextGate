import { t } from "./i18n";
import { useEffect, useState } from "react";
import { api, date, message, payload } from "./api";
import { Button, Drawer, Empty, ErrorNote, Field, Loading } from "./components";
import {
  mappingKey,
  OntologyMappingEditor,
  type MappingItem,
  type MappingKind,
} from "./OntologyMappingEditor";
import {
  type OntologyBinding,
  type OntologyState,
  type OntologySummary,
  type OntologyVersion,
} from "./ontology-types";
import type { SemanticState } from "./semantic-types";

export function OntologyMapping({
  endpoint,
  state,
  accept,
  notify,
  onDirty,
}: {
  endpoint: string;
  state: SemanticState;
  accept: (v: SemanticState) => void;
  notify: (text: string) => void;
  onDirty: (dirty: boolean) => void;
}) {
  const [ontologies, setOntologies] = useState<OntologySummary[]>([]);
  const [binding, setBinding] = useState<OntologyBinding | null>(
    state.draft.ontology || null,
  );
  const [versions, setVersions] = useState<string[]>([]);
  const [version, setVersion] = useState<OntologyVersion | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<{
    kind: MappingKind;
    item: MappingItem;
    index: number;
  } | null>(null);
  const [dialog, setDialog] = useState("");
  const [diff, setDiff] = useState<unknown>(null);
  const [remove, setRemove] = useState<{
    kind: MappingKind;
    index: number;
  } | null>(null);
  const dirty =
    JSON.stringify(binding) !== JSON.stringify(state.draft.ontology || null);
  useEffect(() => {
    onDirty(dirty);
  }, [dirty, onDirty]);
  useEffect(() => {
    setBinding(state.draft.ontology || null);
  }, [state.draft.ontology]);
  useEffect(() => {
    const c = new AbortController();
    api<OntologySummary[]>("/api/ontologies", { signal: c.signal })
      .then(setOntologies)
      .catch((e) => {
        if (!c.signal.aborted) setError(message(e));
      });
    return () => c.abort();
  }, []);
  useEffect(() => {
    const c = new AbortController();
    setVersions([]);
    setVersion(null);
    if (!binding?.ontology_id) return () => c.abort();
    setLoading(true);
    setError("");
    Promise.all([
      api<OntologyState>(`/api/ontologies/${binding.ontology_id}`, {
        signal: c.signal,
      }),
      binding.version !== "0"
        ? api<OntologyVersion>(
            `/api/ontologies/${binding.ontology_id}/versions/${binding.version}`,
            { signal: c.signal },
          )
        : Promise.resolve(null),
    ])
      .then(([st, v]) => {
        setVersions(st.versions);
        setVersion(v);
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!c.signal.aborted) setLoading(false);
      });
    return () => c.abort();
  }, [binding?.ontology_id, binding?.version]);
  async function action(fn: () => Promise<void>) {
    setError("");
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function save(next: OntologyBinding | null) {
    accept(
      await api<SemanticState>(endpoint, {
        method: "PUT",
        body: payload({
          revision: state.revision,
          snapshot: { ...state.draft, ontology: next },
        }),
      }),
    );
    onDirty(false);
    notify(
      t("Ontology mapping saved to draft. Check structure before publishing."),
    );
  }
  const prior = state.published.ontology;
  const selected = ontologies.find((o) => o.id === binding?.ontology_id);
  const validation = state.mapping_validation;
  return (
    <div role="tabpanel" className="ontology-mapping">
      <ErrorNote error={dialog || editing ? "" : error} />
      <section className="ontology-binding-panel">
        <h2>{t("Ontology mapping")}</h2>
        <p className="help">
          {t(
            "Bind one immutable ontology version. Only mapped definitions are visible to Agents authorized for this data source. Publish the mapping together with the catalog and template concept links.",
          )}
        </p>
        <div className="field-grid">
          <Field label={t("Shared ontology")}>
            <select
              value={binding?.ontology_id || ""}
              disabled={busy}
              onChange={(e) => {
                const id = e.target.value;
                setBinding(
                  id
                    ? {
                        ontology_id: id,
                        version: "0",
                        entities: [],
                        properties: [],
                        relations: [],
                      }
                    : null,
                );
              }}
            >
              <option value="">{t("No ontology binding")}</option>
              {ontologies.map((o) => (
                <option
                  key={o.id}
                  value={o.id}
                  disabled={o.archived && o.id !== prior?.ontology_id}
                >
                  {o.name} {o.archived ? t("(archived)") : ""}
                </option>
              ))}
              {binding && !selected && (
                <option value={binding.ontology_id}>
                  {t("Missing ontology: ")}
                  {binding.ontology_id}
                </option>
              )}
            </select>
          </Field>
          <Field label={t("Pinned version")}>
            <select
              value={binding?.version || "0"}
              disabled={!binding || busy || loading}
              onChange={(e) =>
                setBinding(
                  binding ? { ...binding, version: e.target.value } : null,
                )
              }
            >
              <option value="0">{t("Select published version")}</option>
              {versions.map((v) => (
                <option
                  key={v}
                  value={v}
                  disabled={
                    selected?.archived &&
                    (binding?.ontology_id !== prior?.ontology_id ||
                      v !== prior?.version)
                  }
                >
                  {t("Version ")}
                  {v}
                </option>
              ))}
              {binding &&
                binding.version !== "0" &&
                !versions.includes(binding.version) && (
                  <option value={binding.version}>
                    {t("Version ")}
                    {binding.version}
                  </option>
                )}
            </select>
          </Field>
        </div>
        <p className="help">
          {prior
            ? t("Published binding: {ontology_id} · version {version}.", {
                ontology_id: prior.ontology_id,
                version: prior.version,
              })
            : t("No published binding.")}{" "}
          {t(
            "Selecting another ontology starts a new mapping draft. Existing published queries continue until publication.",
          )}
        </p>
        <div className="button-row">
          <Button
            primary
            disabled={!dirty || busy || loading}
            onClick={() => action(() => save(binding))}
          >
            {t("Save binding draft")}
          </Button>
          <Button
            disabled={!dirty || busy}
            onClick={() => setBinding(state.draft.ontology || null)}
          >
            {t("Revert unsaved binding")}
          </Button>
          <Button
            disabled={!binding || busy || dirty || loading}
            onClick={() =>
              action(async () => {
                await api(endpoint + "/check-mapping", {
                  method: "POST",
                  body: payload({ revision: state.revision }),
                });
                accept(await api<SemanticState>(endpoint));
                notify(
                  t(
                    "Mapping structure checked. Review unverified declarations before publication.",
                  ),
                );
              })
            }
          >
            {t("Check structure")}
          </Button>
          <Button
            disabled={
              !binding ||
              !prior ||
              binding.ontology_id !== prior.ontology_id ||
              binding.version === "0" ||
              busy
            }
            onClick={() =>
              action(async () => {
                setDiff(
                  await api(
                    `/api/ontologies/${binding!.ontology_id}/versions/${binding!.version}/diff?from=${prior!.version}`,
                  ),
                );
                setDialog("diff");
              })
            }
          >
            {t("Compare with adopted version")}
          </Button>
        </div>
        {dirty && (
          <p className="help">
            {t(
              "Save or revert the binding before leaving this section or publishing.",
            )}
          </p>
        )}
        {loading && <Loading />}
      </section>
      {!binding ? (
        <Empty
          title={t("No ontology selected")}
          description={t(
            "Existing semantic entries and native templates continue to work without an ontology.",
          )}
        />
      ) : (
        version && (
          <>
            <div className="semantic-status">
              <span
                className={`status ${validation?.status === "checked" && !dirty ? "green" : "amber"}`}
              >
                {dirty
                  ? t("Unsaved binding")
                  : (validation?.status
                      ? t(validation.status.replaceAll("_", " "))
                      : "") || t("Check required")}
              </span>
              <span>
                {t("Metadata checked ")}
                {date(validation?.checked_at)}
              </span>
              <span>{t("Database value constraints are not verified")}</span>
            </div>
            {validation?.checks && !dirty && (
              <details>
                <summary>
                  {t("Structure evidence ·")}{" "}
                  {
                    validation.checks.filter((c) => c.status === "unverified")
                      .length
                  }{" "}
                  {t("unverified fields")}
                </summary>
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th>{t("Physical reference")}</th>
                        <th>{t("Evidence")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {validation.checks.map((c, i) => (
                        <tr key={i}>
                          <td>
                            {c.reference.namespace}.{c.reference.object}
                            {c.reference.field ? "." + c.reference.field : ""}
                          </td>
                          <td>
                            <span
                              className={`status ${c.status === "verified" ? "green" : "amber"}`}
                            >
                              {c.status === "verified"
                                ? t("Discovered in metadata")
                                : t("Administrator declared · unverified")}
                            </span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </details>
            )}
            {(["entities", "properties", "relations"] as const).map((kind) => (
              <section className="ontology-mapping-section" key={kind}>
                <div className="page-header">
                  <h2>
                    {kind === "entities"
                      ? t("Entity mappings")
                      : kind === "properties"
                        ? t("Property mappings")
                        : t("Relation mappings")}
                  </h2>
                  <Button
                    disabled={dirty || busy}
                    onClick={() =>
                      setEditing({
                        kind,
                        index: -1,
                        item:
                          kind === "entities"
                            ? {
                                entity: "",
                                objects: [{ namespace: "", object: "" }],
                              }
                            : kind === "properties"
                              ? {
                                  entity: "",
                                  property: "",
                                  reference: {
                                    namespace: "",
                                    object: "",
                                    field: "",
                                  },
                                }
                              : { relation: "", fields: [] },
                      })
                    }
                  >
                    {t("Add")}{" "}
                    {kind === "entities"
                      ? t("entity")
                      : kind === "properties"
                        ? t("property")
                        : t("relation")}{" "}
                    {t("mapping")}
                  </Button>
                </div>
                {binding[kind].length === 0 ? (
                  <p className="help">
                    {t("No ")}
                    {t(kind)}
                    {t(" mapped. Unmapped definitions are hidden from Agents.")}
                  </p>
                ) : (
                  <div className="table-scroll">
                    <table>
                      <thead>
                        <tr>
                          <th>{t("Concept")}</th>
                          <th>{t("Physical mapping / template")}</th>
                          <th />
                        </tr>
                      </thead>
                      <tbody>
                        {binding[kind].map((m, i) => (
                          <tr key={mappingKey(m)}>
                            <td>{mappingKey(m)}</td>
                            <td>
                              {"objects" in m
                                ? m.objects
                                    .map((o) => `${o.namespace}.${o.object}`)
                                    .join(", ")
                                : "property" in m
                                  ? m.reference
                                    ? `${m.reference.namespace}.${m.reference.object}.${m.reference.field}${m.declared ? t(" · declaration allowed") : ""}`
                                    : t("Template: {value1}", {
                                        value1: m.template_id || "missing",
                                      })
                                  : t("{value1} field pairs{value2}", {
                                      value1: m.fields?.length || 0,
                                      value2: m.template_id
                                        ? ` · Template: ${m.template_id}`
                                        : "",
                                    })}
                            </td>
                            <td>
                              <div className="button-row">
                                <Button
                                  disabled={dirty || busy}
                                  onClick={() =>
                                    setEditing({ kind, item: m, index: i })
                                  }
                                >
                                  {t("Edit")}
                                </Button>
                                <Button
                                  disabled={dirty || busy}
                                  className="danger"
                                  onClick={() => {
                                    setRemove({ kind, index: i });
                                    setDialog("remove");
                                  }}
                                >
                                  {t("Remove")}
                                </Button>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
            ))}
          </>
        )
      )}
      {editing && binding && version && (
        <OntologyMappingEditor
          kind={editing.kind}
          item={editing.item}
          existing={editing.index >= 0}
          definition={version.definition}
          binding={binding}
          entries={state.draft.entries}
          onClose={() => setEditing(null)}
          onSave={async (item) => {
            if (
              binding[editing.kind].some(
                (v, i) =>
                  i !== editing.index && mappingKey(v) === mappingKey(item),
              )
            )
              throw new Error(t("This concept already has a mapping."));
            const rows: MappingItem[] = [...binding[editing.kind]];
            if (editing.index < 0) rows.push(item);
            else rows[editing.index] = item;
            await save({ ...binding, [editing.kind]: rows });
            setEditing(null);
          }}
        />
      )}
      {dialog && (
        <Drawer
          wide={dialog === "diff"}
          title={
            dialog === "diff"
              ? t("Ontology version differences")
              : t("Remove mapping from draft")
          }
          onClose={() => {
            if (!busy) setDialog("");
          }}
          footer={
            <>
              <Button
                disabled={busy}
                onClick={() => {
                  setDialog("");
                  setError("");
                }}
              >
                {dialog === "diff" ? t("Close") : t("Cancel")}
              </Button>
              {dialog === "remove" && (
                <Button
                  primary
                  busy={busy}
                  onClick={() =>
                    action(async () => {
                      if (binding && remove)
                        await save({
                          ...binding,
                          [remove.kind]: binding[remove.kind].filter(
                            (_, i) => i !== remove.index,
                          ),
                        });
                      setDialog("");
                    })
                  }
                >
                  {t("Remove draft mapping")}
                </Button>
              )}
            </>
          }
        >
          <ErrorNote error={error} />
          {dialog === "diff" ? (
            <>
              <p>
                {t(
                  "Review removed or changed definitions and repair mappings before adopting this version.",
                )}
              </p>
              <pre className="query-output">
                {JSON.stringify(diff, null, 2)}
              </pre>
            </>
          ) : (
            <p>
              {t(
                "Related property mappings, relation endpoints and template concept references must be repaired before publication. The published snapshot remains available.",
              )}
            </p>
          )}
        </Drawer>
      )}
    </div>
  );
}
