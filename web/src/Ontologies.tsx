import { useCallback, useEffect, useState } from "react";
import { ArrowLeft, Download, Plus, RefreshCw, Upload } from "lucide-react";
import { api, message, payload } from "./api";
import { Button, Empty, ErrorNote, Field, Loading } from "./components";
import {
  OntologyEditor,
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
import { OntologyDialog } from "./OntologyDialog";
import type { Source } from "./types";

export function Ontologies({
  id,
  sources,
  navigate,
  notify,
}: {
  id?: string;
  sources: Source[];
  navigate: (path: string) => void;
  notify: (text: string) => void;
}) {
  const [list, setList] = useState<OntologySummary[]>([]);
  const [state, setState] = useState<OntologyState | null>(null);
  const [draft, setDraft] = useState<OntologyDefinition>(emptyOntology);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [tab, setTab] = useState("Entities");
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
  }, [id, accept]);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setState(null);
    setPage(0);
    setSearch("");
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
    if (!state) return;
    accept(
      await api<OntologyState>(endpoint, {
        method: "PUT",
        body: payload({ revision: state.revision, definition }),
      }),
    );
    notify("Ontology draft saved.");
  }
  const dirty =
    !!state && JSON.stringify(draft) !== JSON.stringify(state.draft);
  const tabs = ["Entities", "Properties", "Relations", "Versions", "Usage"];
  const kind = tab.toLowerCase() as OntologyKind;
  const rows: OntologyItem[] = ["entities", "properties", "relations"].includes(
    kind,
  )
    ? draft[kind].filter((v) =>
        `${v.name} ${(v.aliases || []).join(" ")} ${v.description || ""}`
          .toLowerCase()
          .includes(search.toLowerCase()),
      )
    : [];
  const current = Math.min(page, Math.max(0, Math.ceil(rows.length / 20) - 1));
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
              setError("Save or revert unsaved changes before leaving.");
              return;
            }
            navigate("/ontologies");
          }}
        >
          <ArrowLeft size={16} />
          Ontologies
        </Button>
      )}
      <div className="page-header">
        <div>
          <h1>{id ? state?.draft.name || "Ontology" : "Ontologies"}</h1>
          <p>
            Shared business definitions, independently mapped and authorized per
            data source.
          </p>
        </div>
        <div className="button-row">
          <Button disabled={busy || dirty} onClick={() => action(reload)}>
            <RefreshCw size={16} />
            Reload
          </Button>
          {!id && (
            <>
              <Button
                onClick={() => {
                  setDialog("import");
                }}
              >
                <Upload size={16} />
                Import JSON
              </Button>
              <Button
                primary
                onClick={() => {
                  setDialog("create");
                }}
              >
                <Plus size={16} />
                Create ontology
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
                      "Definition is consistent. Database constraints remain declarative.",
                    );
                  })
                }
              >
                Validate
              </Button>
              <Button
                primary
                disabled={busy || dirty || !state.changed || state.archived}
                onClick={() => setDialog("publish")}
              >
                Publish version
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
          <Field label="Search ontologies">
            <input
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(0);
              }}
              placeholder="Name or description"
            />
          </Field>
          {matchingOntologies.length === 0 ? (
            <Empty
              title={
                search ? "No matching ontologies" : "No shared ontologies yet"
              }
              description="Define business entities once, then map each data source to a pinned version."
            />
          ) : (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>Ontology</th>
                    <th>Latest version</th>
                    <th>Status</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {list
                    .filter((v) =>
                      `${v.name} ${v.description}`
                        .toLowerCase()
                        .includes(search.toLowerCase()),
                    )
                    .slice(page * 20, (page + 1) * 20)
                    .map((v) => (
                      <tr key={v.id}>
                        <td>
                          <strong>{v.name || "Untitled ontology"}</strong>
                          <small className="block">{v.id}</small>
                        </td>
                        <td>
                          {v.latest_version === "0"
                            ? "Not published"
                            : v.latest_version}
                        </td>
                        <td>
                          <span
                            className={`status ${v.archived ? "muted" : "green"}`}
                          >
                            {v.archived ? "Archived" : "Active"}
                          </span>
                        </td>
                        <td>
                          <Button
                            onClick={() => navigate(`/ontologies/${v.id}`)}
                          >
                            Open
                          </Button>
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
          )}
          {list.length > 20 && (
            <div className="pagination">
              <Button disabled={page === 0} onClick={() => setPage(page - 1)}>
                Previous
              </Button>
              <span>{page + 1}</span>
              <Button
                disabled={
                  (page + 1) * 20 >=
                  list.filter((v) =>
                    `${v.name} ${v.description}`
                      .toLowerCase()
                      .includes(search.toLowerCase()),
                  ).length
                }
                onClick={() => setPage(page + 1)}
              >
                Next
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
                  ? "Unsaved changes"
                  : state.changed
                    ? "Unpublished changes"
                    : "Draft matches latest version"}
              </span>
              <span>Revision {state.revision}</span>
              <span>Latest version {state.latest_version}</span>
              {state.archived && (
                <span className="status muted">
                  Archived · existing bindings remain active
                </span>
              )}
            </div>
            <div className="semantic-overview">
              <section>
                <div className="field-grid">
                  <Field label="Ontology name" required>
                    <input
                      value={draft.name}
                      maxLength={256}
                      onChange={(e) =>
                        setDraft({ ...draft, name: e.target.value })
                      }
                    />
                  </Field>
                  <Field label="Stable ontology ID">
                    <input value={state.id} readOnly />
                  </Field>
                </div>
                <Field label="Description">
                  <textarea
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
                    Save draft
                  </Button>
                  <Button
                    disabled={!dirty || busy}
                    onClick={() => setDraft(state.draft)}
                  >
                    Revert unsaved changes
                  </Button>
                </div>
              </section>
              <section>
                <h2>Version adoption is explicit</h2>
                <p className="help">
                  Publishing preserves older versions. Sources remain pinned
                  until an administrator reviews the differences and publishes
                  their mapping. Identity, uniqueness and cardinality are
                  declarations, not verified data quality.
                </p>
                <div className="button-row">
                  <Button
                    disabled={dirty || busy}
                    onClick={() => {
                      setDialog("import");
                    }}
                  >
                    <Upload size={15} />
                    Import JSON
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
                    Export draft
                  </Button>
                  <Button
                    disabled={!state.changed || dirty || busy}
                    onClick={() => setDialog("discard")}
                  >
                    Discard draft
                  </Button>
                  <Button
                    disabled={dirty || busy}
                    onClick={() => setDialog("archive")}
                  >
                    {state.archived ? "Unarchive" : "Archive"}
                  </Button>
                  <Button
                    className="danger"
                    disabled={dirty || busy || state.usage.length > 0}
                    onClick={() => setDialog("delete")}
                  >
                    Delete ontology
                  </Button>
                </div>
              </section>
            </div>
            <div
              className="semantic-tabs"
              role="tablist"
              aria-label="Ontology sections"
            >
              {tabs.map((t, i) => (
                <button
                  key={t}
                  role="tab"
                  aria-selected={tab === t}
                  tabIndex={tab === t ? 0 : -1}
                  onClick={() => {
                    setTab(t);
                    setPage(0);
                  }}
                  onKeyDown={(e) => {
                    if (!["ArrowLeft", "ArrowRight"].includes(e.key)) return;
                    e.preventDefault();
                    const n =
                      (i + (e.key === "ArrowRight" ? 1 : tabs.length - 1)) %
                      tabs.length;
                    setTab(tabs[n]);
                    setPage(0);
                    (
                      e.currentTarget.parentElement?.children[n] as HTMLElement
                    )?.focus();
                  }}
                >
                  {t}
                </button>
              ))}
            </div>
            <div role="tabpanel">
              {["Entities", "Properties", "Relations"].includes(tab) ? (
                <>
                  <div className="filters">
                    <input
                      aria-label="Search ontology definitions"
                      placeholder="Search names, aliases and descriptions"
                      value={search}
                      onChange={(e) => {
                        setSearch(e.target.value);
                        setPage(0);
                      }}
                    />
                    <Button
                      primary
                      disabled={dirty || busy}
                      onClick={() =>
                        setEditing({
                          kind,
                          existing: false,
                          item:
                            kind === "entities"
                              ? { id: "", name: "" }
                              : kind === "properties"
                                ? {
                                    id: "",
                                    name: "",
                                    entity: "",
                                    type: "string",
                                    required: false,
                                    multiple: false,
                                  }
                                : {
                                    id: "",
                                    name: "",
                                    from: "",
                                    to: "",
                                    directed: true,
                                    from_cardinality: { min: 0, max: null },
                                    to_cardinality: { min: 0, max: null },
                                  },
                        })
                      }
                    >
                      <Plus size={16} />
                      Add{" "}
                      {kind === "entities"
                        ? "entity"
                        : kind === "properties"
                          ? "property"
                          : "relation"}
                    </Button>
                  </div>
                  {rows.length === 0 ? (
                    <Empty
                      title={
                        search
                          ? "No matching definitions"
                          : `No ${kind} defined`
                      }
                      description="Save definitions to the draft, validate, then publish an immutable version."
                    />
                  ) : (
                    <div className="table-scroll">
                      <table>
                        <thead>
                          <tr>
                            <th>Name / ID</th>
                            <th>Definition</th>
                            <th>Description</th>
                            <th />
                          </tr>
                        </thead>
                        <tbody>
                          {rows
                            .slice(current * 20, (current + 1) * 20)
                            .map((v) => (
                              <tr key={v.id}>
                                <td>
                                  <strong>{v.name}</strong>
                                  <small className="block">{v.id}</small>
                                </td>
                                <td>
                                  {"entity" in v
                                    ? `${v.entity} · ${v.type}${v.multiple ? "[]" : ""}`
                                    : "from" in v
                                      ? `${v.from} ${v.directed ? "→" : "↔"} ${v.to}`
                                      : v.parent
                                        ? `Inherits ${v.parent}`
                                        : "Root entity"}
                                </td>
                                <td>{v.description || "—"}</td>
                                <td>
                                  <div className="button-row">
                                    <Button
                                      disabled={dirty || busy}
                                      onClick={() =>
                                        setEditing({
                                          kind,
                                          item: v,
                                          existing: true,
                                        })
                                      }
                                    >
                                      Edit
                                    </Button>
                                    <Button
                                      disabled={dirty || busy}
                                      className="danger"
                                      onClick={() => {
                                        setDeleteItem({ kind, id: v.id });
                                        setDialog("delete-item");
                                      }}
                                    >
                                      Remove
                                    </Button>
                                  </div>
                                </td>
                              </tr>
                            ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                  {rows.length > 20 && (
                    <div className="pagination">
                      <Button
                        disabled={current === 0}
                        onClick={() => setPage(current - 1)}
                      >
                        Previous
                      </Button>
                      <span>{current + 1}</span>
                      <Button
                        disabled={(current + 1) * 20 >= rows.length}
                        onClick={() => setPage(current + 1)}
                      >
                        Next
                      </Button>
                    </div>
                  )}
                </>
              ) : tab === "Versions" ? (
                <>
                  <p className="help">
                    Versions are immutable. Referenced versions cannot be
                    deleted. The latest version remains available as the discard
                    target.
                  </p>
                  {state.versions.map((v) => (
                    <div className="ontology-version-row" key={v}>
                      <strong>Version {v}</strong>
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
                        View definition
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
                        Export
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
                        Delete version
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
                        Load older versions
                      </Button>
                    )}
                </>
              ) : (
                <>
                  {state.usage.length === 0 ? (
                    <Empty
                      title="No data source bindings"
                      description="Open a data source's Semantics page to map this ontology."
                    />
                  ) : (
                    <div className="table-scroll">
                      <table>
                        <thead>
                          <tr>
                            <th>Data source</th>
                            <th>Phase</th>
                            <th>Pinned version</th>
                            <th />
                          </tr>
                        </thead>
                        <tbody>
                          {state.usage.map((u) => (
                            <tr key={u.source_id + u.phase}>
                              <td>
                                {sources.find((s) => s.id === u.source_id)
                                  ?.name || u.source_id}
                              </td>
                              <td>{u.phase}</td>
                              <td>{u.version}</td>
                              <td>
                                <Button
                                  disabled={dirty}
                                  onClick={() =>
                                    navigate(
                                      `/sources/${u.source_id}/semantics`,
                                    )
                                  }
                                >
                                  Open semantics
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
          onSave={async (item) => {
            const entries = draft[editing.kind];
            if (!editing.existing && entries.some((v) => v.id === item.id))
              throw new Error("This definition ID already exists.");
            const next = {
              ...draft,
              [editing.kind]: editing.existing
                ? entries.map((v) => (v.id === item.id ? item : v))
                : [...entries, item],
            };
            await save(next);
            setEditing(null);
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
