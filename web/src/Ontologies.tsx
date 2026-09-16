import { t } from "./i18n";
import "./ontology-editor.css";
import { useCallback, useEffect, useState } from "react";
import { ArrowLeft, Download, Plus, RefreshCw, Upload } from "lucide-react";
import { api, message, payload } from "./api";
import { Button, Empty, ErrorNote, Field, Loading } from "./components";
import {
  OntologyEditor,
  newOntologyItem,
  type OntologyItem,
  type OntologyKind,
} from "./OntologyEditor";
import {
  downloadJSON,
  emptyOntology,
  type OntologyDefinition,
  type OntologyState,
  type OntologySummary,
  type OntologyVersion,
} from "./ontology-types";
import { OntologyModel } from "./OntologyModel";
import { OntologyDefinitions } from "./OntologyDefinitions";
import { OntologyDialog } from "./OntologyDialog";
import type { Agent, Source } from "./types";
import { useNavigationGuard } from "./useNavigationGuard";

export function Ontologies({
  id,
  initialQuery = "",
  sources,
  agents,
  navigate,
  notify,
}: {
  id?: string;
  initialQuery?: string;
  sources: Source[];
  agents: Agent[];
  navigate: (path: string) => void;
  notify: (text: string) => void;
}) {
  const [list, setList] = useState<OntologySummary[]>([]);
  const [state, setState] = useState<OntologyState | null>(null);
  const [usageRefreshKey, setUsageRefreshKey] = useState(0);
  const [draft, setDraft] = useState<OntologyDefinition>(emptyOntology);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [tab, setTab] = useState("Model");
  const initial = new URLSearchParams(initialQuery);
  const [selectedEntity, setSelectedEntity] = useState(
    initial.get("entity") || "",
  );
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [editing, setEditing] = useState<{
    kind: OntologyKind;
    item: OntologyItem;
    existing: boolean;
  } | null>(null);
  const [dialog, setDialog] = useState("");
  const [version, setVersion] = useState<OntologyVersion | null>(null);
  const [deleteItem, setDeleteItem] = useState<{
    kind: OntologyKind;
    id: string;
  } | null>(null);
  const accept = useCallback((v: OntologyState) => {
    setState(v);
    setDraft(v.draft);
  }, []);
  const endpoint = `/api/ontologies/${id}`;
  const reload = useCallback(async () => {
    if (id) accept(await api<OntologyState>(`/api/ontologies/${id}`));
    else setList(await api<OntologySummary[]>("/api/ontologies"));
    setUsageRefreshKey((v) => v + 1);
  }, [id, accept]);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setState(null);
    setPage(0);
    setSearch("");
    setTab("Model");
    setSelectedEntity(initial.get("entity") || "");
    const task = id
      ? api<OntologyState>(`/api/ontologies/${id}`, {
          signal: controller.signal,
        }).then(accept)
      : api<OntologySummary[]>("/api/ontologies", {
          signal: controller.signal,
        }).then(setList);
    task
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [id, accept]);
  async function action(fn: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function save(definition: OntologyDefinition) {
    if (!state || state.id !== id)
      throw new Error(t("The active ontology changed. Reload before saving."));
    accept(
      await api<OntologyState>(endpoint, {
        method: "PUT",
        body: payload({ revision: state.revision, definition }),
      }),
    );
    notify(t("Ontology draft saved."));
  }
  const dirty =
    !!state && JSON.stringify(draft) !== JSON.stringify(state.draft);
  useNavigationGuard(dirty, busy && !dialog);
  const tabs = ["Model", "Definitions", "Versions", "Usage", "Settings"];
  function addDefinition(kind: OntologyKind, entity?: string) {
    setEditing({ kind, item: newOntologyItem(kind, entity), existing: false });
  }
  function editDefinition(kind: OntologyKind, item: OntologyItem) {
    setEditing({ kind, item, existing: true });
  }
  function removeDefinition(kind: OntologyKind, id: string) {
    setDeleteItem({ kind, id });
    setDialog("delete-item");
  }
  function selectTab(next: string) {
    if (dirty) {
      setError(
        t("Save or revert ontology settings before switching sections."),
      );
      return;
    }
    setError("");
    setTab(next);
  }
  const matchingOntologies = list.filter((v) =>
    `${v.name} ${v.description || ""}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const dialogError = dialog || editing ? "" : error;
  return (
    <>
      {id && (
        <Button
          onClick={() => {
            if (dirty) {
              setError(t("Save or revert unsaved changes before leaving."));
              return;
            }
            navigate("/ontologies");
          }}
        >
          <ArrowLeft size={16} />
          {t("Ontologies")}
        </Button>
      )}
      <div className="page-header">
        <div>
          <h1>{id ? state?.draft.name || t("Ontology") : t("Ontologies")}</h1>
        </div>
        <div className="button-row">
          <Button disabled={busy || dirty} onClick={() => action(reload)}>
            <RefreshCw size={16} />
            {t("Reload")}
          </Button>
          {!id && (
            <>
              <Button
                onClick={() => {
                  setDialog("import");
                }}
              >
                <Upload size={16} />
                {t("Import JSON")}
              </Button>
              <Button
                primary
                onClick={() => {
                  setDialog("create");
                }}
              >
                <Plus size={16} />
                {t("Create ontology")}
              </Button>
            </>
          )}
          {state && (
            <>
              <Button
                disabled={busy || dirty}
                onClick={() =>
                  action(async () => {
                    await api(endpoint + "/validate", {
                      method: "POST",
                      body: payload({ revision: state.revision }),
                    });
                    notify(
                      t(
                        "Definition is consistent. Database constraints remain declarative.",
                      ),
                    );
                  })
                }
              >
                {t("Validate")}
              </Button>
              <Button
                primary
                disabled={busy || dirty || !state.changed || state.archived}
                onClick={() => setDialog("publish")}
              >
                {t("Publish version")}
              </Button>
            </>
          )}
        </div>
      </div>
      <ErrorNote error={dialogError} />
      {loading ? (
        <Loading />
      ) : !id ? (
        <>
          <Field label={t("Search ontologies")}>
            <input
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(0);
              }}
              placeholder={t("Name or description")}
            />
          </Field>
          {matchingOntologies.length === 0 ? (
            <Empty
              title={
                search
                  ? t("No matching ontologies")
                  : t("No shared ontologies yet")
              }
              description={t(
                "Define business entities once, then map each data source to a pinned version.",
              )}
            />
          ) : (
            <div className="table-scroll">
              <table className="ontologies-table">
                <thead>
                  <tr>
                    <th>{t("Ontology")}</th>
                    <th>{t("Latest version")}</th>
                    <th>{t("Status")}</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {matchingOntologies
                    .slice(page * 20, (page + 1) * 20)
                    .map((v) => (
                      <tr key={v.id}>
                        <td>
                          <strong>{v.name || t("Untitled ontology")}</strong>
                          <small className="block">{v.id}</small>
                        </td>
                        <td data-label={t("Latest version")}>
                          {v.latest_version === "0"
                            ? t("Not published")
                            : v.latest_version}
                        </td>
                        <td data-label={t("Status")}>
                          <span
                            className={`status ${v.archived ? "muted" : "green"}`}
                          >
                            {v.archived ? t("Archived") : t("Active")}
                          </span>
                        </td>
                        <td>
                          <Button
                            onClick={() => navigate(`/ontologies/${v.id}`)}
                          >
                            {t("Open")}
                          </Button>
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
          )}
          {matchingOntologies.length > 20 && (
            <div className="pagination">
              <Button disabled={page === 0} onClick={() => setPage(page - 1)}>
                {t("Previous")}
              </Button>
              <span>{page + 1}</span>
              <Button
                disabled={(page + 1) * 20 >= matchingOntologies.length}
                onClick={() => setPage(page + 1)}
              >
                {t("Next")}
              </Button>
            </div>
          )}
        </>
      ) : (
        state && (
          <>
            <div className="semantic-status">
              <span
                className={`status ${state.changed || dirty ? "amber" : "green"}`}
              >
                {dirty
                  ? t("Unsaved changes")
                  : state.changed
                    ? t("Unpublished changes")
                    : t("Draft matches latest version")}
              </span>
              <span>
                {t("Revision ")}
                {state.revision}
              </span>
              <span>
                {t("Latest version ")}
                {state.latest_version}
              </span>
              {state.archived && (
                <span className="status muted">
                  {t("Archived · existing bindings remain active")}
                </span>
              )}
            </div>
            <div
              className="semantic-tabs"
              role="tablist"
              aria-label={t("Ontology sections")}
            >
              {tabs.map((section, i) => (
                <button
                  key={section}
                  role="tab"
                  aria-selected={tab === section}
                  tabIndex={tab === section ? 0 : -1}
                  onClick={() => {
                    selectTab(section);
                  }}
                  onKeyDown={(e) => {
                    if (!["ArrowLeft", "ArrowRight"].includes(e.key)) return;
                    e.preventDefault();
                    const n =
                      (i + (e.key === "ArrowRight" ? 1 : tabs.length - 1)) %
                      tabs.length;
                    if (dirty) {
                      selectTab(tabs[n]);
                      return;
                    }
                    selectTab(tabs[n]);
                    (
                      e.currentTarget.parentElement?.children[n] as HTMLElement
                    )?.focus();
                  }}
                >
                  {t(section)}
                </button>
              ))}
            </div>
            <div role="tabpanel">
              {tab === "Model" ? (
                <OntologyModel
                  key={state.id}
                  ontologyID={state.id}
                  refreshKey={usageRefreshKey}
                  sources={[...sources].sort(
                    (a, b) =>
                      Number(state.usage.some((u) => u.source_id === b.id)) -
                      Number(state.usage.some((u) => u.source_id === a.id)),
                  )}
                  initialInspect={
                    initial.get("section") === "queries"
                      ? initial.get("entity") || ""
                      : ""
                  }
                  initialSourceID={initial.get("source_id") || ""}
                  agents={agents}
                  navigate={navigate}
                  overlayOpen={!!editing || !!dialog}
                  definition={draft}
                  selected={selectedEntity}
                  onSelect={setSelectedEntity}
                  onAdd={addDefinition}
                  onConnect={(from, to) =>
                    setEditing({
                      kind: "relations",
                      item: { ...newOntologyItem("relations", from), to },
                      existing: false,
                    })
                  }
                  onEdit={editDefinition}
                  onRemove={removeDefinition}
                  disabled={dirty || busy}
                />
              ) : tab === "Settings" ? (
                <div className="semantic-overview">
                  <section>
                    <div className="field-grid">
                      <Field label={t("Ontology name")} required>
                        <input
                          disabled={busy}
                          value={draft.name}
                          maxLength={256}
                          onChange={(e) =>
                            setDraft({ ...draft, name: e.target.value })
                          }
                        />
                      </Field>
                      <Field label={t("Stable ontology ID")}>
                        <input value={state.id} readOnly />
                      </Field>
                    </div>
                    <Field label={t("Description")}>
                      <textarea
                        disabled={busy}
                        rows={2}
                        value={draft.description || ""}
                        onChange={(e) =>
                          setDraft({ ...draft, description: e.target.value })
                        }
                      />
                    </Field>
                    <div className="button-row">
                      <Button
                        primary
                        disabled={!dirty || busy}
                        onClick={() => action(() => save(draft))}
                      >
                        {t("Save draft")}
                      </Button>
                      <Button
                        disabled={!dirty || busy}
                        onClick={() => setDraft(state.draft)}
                      >
                        {t("Revert unsaved changes")}
                      </Button>
                    </div>
                  </section>
                  <section>
                    <h2>{t("Version adoption is explicit")}</h2>
                    <p className="help">
                      {t(
                        "Publishing preserves older versions. Sources remain pinned until an administrator reviews the differences and publishes their mapping. Identity, uniqueness and cardinality are declarations, not verified data quality.",
                      )}
                    </p>
                    <div className="button-row">
                      <Button
                        disabled={dirty || busy}
                        onClick={() => {
                          setDialog("import");
                        }}
                      >
                        <Upload size={15} />
                        {t("Import JSON")}
                      </Button>
                      <Button
                        disabled={dirty || busy}
                        onClick={() =>
                          action(async () =>
                            downloadJSON(
                              await api(endpoint + "/export"),
                              "ontology.json",
                            ),
                          )
                        }
                      >
                        <Download size={15} />
                        {t("Export draft")}
                      </Button>
                      <Button
                        disabled={!state.changed || dirty || busy}
                        onClick={() => setDialog("discard")}
                      >
                        {t("Discard draft")}
                      </Button>
                      <Button
                        disabled={dirty || busy}
                        onClick={() => setDialog("archive")}
                      >
                        {state.archived ? t("Unarchive") : t("Archive")}
                      </Button>
                      <Button
                        className="danger"
                        disabled={dirty || busy || state.usage.length > 0}
                        onClick={() => setDialog("delete")}
                      >
                        {t("Delete ontology")}
                      </Button>
                    </div>
                  </section>
                </div>
              ) : tab === "Definitions" ? (
                <OntologyDefinitions
                  definition={draft}
                  disabled={dirty || busy}
                  onAdd={addDefinition}
                  onEdit={editDefinition}
                  onRemove={removeDefinition}
                />
              ) : tab === "Versions" ? (
                <>
                  <p className="help">
                    {t(
                      "Versions are immutable. Referenced versions cannot be deleted. The latest version remains available as the discard target.",
                    )}
                  </p>
                  {state.versions.map((v) => (
                    <div className="ontology-version-row" key={v}>
                      <strong>
                        {t("Version ")}
                        {v}
                      </strong>
                      <Button
                        onClick={() =>
                          action(async () => {
                            setVersion(
                              await api<OntologyVersion>(
                                `${endpoint}/versions/${v}`,
                              ),
                            );
                            setDialog("version");
                          })
                        }
                      >
                        {t("View definition")}
                      </Button>
                      <Button
                        onClick={() =>
                          action(async () =>
                            downloadJSON(
                              await api(`${endpoint}/export?version=${v}`),
                              `ontology-v${v}.json`,
                            ),
                          )
                        }
                      >
                        {t("Export")}
                      </Button>
                      <Button
                        className="danger"
                        disabled={
                          v === state.latest_version ||
                          state.usage.some((u) => u.version === v)
                        }
                        onClick={() => {
                          setVersion({
                            ontology_id: id!,
                            version: v,
                            published_at: "",
                            definition: emptyOntology(),
                          });
                          setDialog("delete-version");
                        }}
                      >
                        {t("Delete version")}
                      </Button>
                    </div>
                  ))}
                  {state.versions.length > 0 &&
                    state.versions.length % 100 === 0 && (
                      <Button
                        onClick={() =>
                          action(async () => {
                            const older = await api<string[]>(
                              `${endpoint}/versions?before=${state.versions.at(-1)}`,
                            );
                            setState({
                              ...state,
                              versions: [...state.versions, ...older],
                            });
                          })
                        }
                      >
                        {t("Load older versions")}
                      </Button>
                    )}
                </>
              ) : (
                <>
                  {state.usage.length === 0 ? (
                    <Empty
                      title={t("No data source bindings")}
                      description={t(
                        "Open a data source's Semantics page to map this ontology.",
                      )}
                    />
                  ) : (
                    <div className="table-scroll">
                      <table>
                        <thead>
                          <tr>
                            <th>{t("Data source")}</th>
                            <th>{t("Published version")}</th>
                            <th>{t("Draft version")}</th>
                            <th />
                          </tr>
                        </thead>
                        <tbody>
                          {[
                            ...new Set(state.usage.map((u) => u.source_id)),
                          ].map((sourceID) => (
                            <tr key={sourceID}>
                              <td>
                                {sources.find((s) => s.id === sourceID)?.name ||
                                  sourceID}
                              </td>
                              <td>
                                {state.usage.find(
                                  (u) =>
                                    u.source_id === sourceID &&
                                    u.phase === "published",
                                )?.version || t("Not adopted")}
                              </td>
                              <td>
                                {state.usage.find(
                                  (u) =>
                                    u.source_id === sourceID &&
                                    u.phase === "draft",
                                )?.version || t("No binding")}
                              </td>
                              <td>
                                <Button
                                  disabled={dirty}
                                  onClick={() =>
                                    navigate(
                                      `/sources/${sourceID}/semantics?tab=Ontology+mapping`,
                                    )
                                  }
                                >
                                  {t("Open semantics")}
                                </Button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </>
              )}
            </div>
          </>
        )
      )}
      {editing && state && (
        <OntologyEditor
          kind={editing.kind}
          item={editing.item}
          existing={editing.existing}
          definition={draft}
          onClose={() => setEditing(null)}
          onReload={async () => {
            await reload();
            setEditing(null);
          }}
          onSave={async (item, keepOpen) => {
            const entries = draft[editing.kind];
            if (!editing.existing && entries.some((v) => v.id === item.id))
              throw new Error(t("This definition ID already exists."));
            if (
              editing.existing &&
              entries.filter((v) => v.id === item.id).length !== 1
            )
              throw new Error(
                t(
                  "This definition is missing or its ID is duplicated. Reload the ontology before editing.",
                ),
              );
            const next = {
              ...draft,
              [editing.kind]: editing.existing
                ? entries.map((v) => (v.id === item.id ? item : v))
                : [...entries, item],
            };
            await save(next);
            if (editing.kind === "entities") setSelectedEntity(item.id);
            if (!keepOpen) setEditing(null);
          }}
        />
      )}
      {dialog && (
        <OntologyDialog
          key={dialog}
          dialog={dialog}
          id={id}
          state={state}
          draft={draft}
          version={version}
          deleteItem={deleteItem}
          busy={busy}
          error={error}
          setError={setError}
          setDialog={setDialog}
          action={action}
          accept={accept}
          save={save}
          reload={reload}
          navigate={navigate}
          notify={notify}
        />
      )}
    </>
  );
}
