import { useCallback, useEffect, useState } from "react";
import {
  ArrowLeft,
  Download,
  Plus,
  RefreshCw,
  Search,
  Upload,
} from "lucide-react";
import { api, date, message, payload } from "./api";
import { Button, Drawer, Empty, ErrorNote, Field, Loading } from "./components";
import { SemanticVisibility } from "./SemanticVisibility";
import { OntologyMapping } from "./OntologyMapping";
import { conceptRefs } from "./ontology-types";
import { SemanticEditor } from "./SemanticEditor";
import { StructureImport, TemplatePreview } from "./SemanticTools";
import {
  entryKinds,
  type SemanticEntry,
  type SemanticSnapshot,
  type SemanticState,
} from "./semantic-types";
import type { Agent, Source } from "./types";

export function Semantics({
  source,
  agents,
  onBack,
  notify,
}: {
  source: Source;
  agents: Agent[];
  onBack: () => void;
  notify: (s: string) => void;
}) {
  const [state, setState] = useState<SemanticState | null>(null);
  const [tab, setTab] = useState("Overview");
  const [mappingDirty, setMappingDirty] = useState(false);
  const [overview, setOverview] = useState("");
  const [search, setSearch] = useState("");
  const [kind, setKind] = useState("");
  const [page, setPage] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [editing, setEditing] = useState<SemanticEntry | null>(null);
  const [dialog, setDialog] = useState("");
  const [selected, setSelected] = useState<SemanticEntry | null>(null);
  const [importJSON, setImportJSON] = useState("");
  const endpoint = `/api/sources/${source.id}/semantics`;
  const accept = useCallback((v: SemanticState) => {
    setState(v);
    setOverview(v.draft.overview);
  }, []);
  const reload = useCallback(async () => {
    accept(await api<SemanticState>(endpoint));
  }, [endpoint, accept]);
  useEffect(() => {
    const controller = new AbortController();
    api<SemanticState>(endpoint, { signal: controller.signal })
      .then(accept)
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      });
    return () => controller.abort();
  }, [endpoint, accept]);
  async function action(name: string, fn: () => Promise<void>) {
    setBusy(name);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy("");
    }
  }
  if (!state)
    return (
      <>
        <Button onClick={onBack}>
          <ArrowLeft size={16} />
          Data sources
        </Button>
        <ErrorNote error={error} />
        {error ? (
          <Button onClick={() => action("reload", reload)}>
            Retry loading semantics
          </Button>
        ) : (
          <Loading />
        )}
      </>
    );
  const entries = state.draft.entries;
  const rows = entries.filter(
    (en) =>
      (tab === "Query templates"
        ? en.kind === "template"
        : en.kind !== "template") &&
      (!kind || en.kind === kind) &&
      `${en.name} ${(en.aliases || []).join(" ")} ${en.description || ""}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const current = Math.min(page, Math.max(0, Math.ceil(rows.length / 20) - 1));
  const trialRequired = state.validation.filter(
    (v) => !v.valid && v.status !== "disabled",
  ).length;
  const unsavedOverview = overview !== state.draft.overview || mappingDirty;
  const create = () => {
    const template = tab === "Query templates";
    const example = { ...source.capability?.example };
    delete example.source_id;
    setEditing({
      id: `${template ? "template" : "entry"}_${crypto.randomUUID()}`,
      kind: template ? "template" : "term",
      name: "",
      ...(template
        ? {
            template: {
              enabled: true,
              tool: source.capability?.tool || "query_sql",
              query_json: JSON.stringify(example, null, 2),
              parameters: [],
              example_json: "{}",
            },
          }
        : {}),
    });
  };
  return (
    <>
      <button
        className="text-button semantic-back"
        disabled={!!busy || unsavedOverview}
        onClick={onBack}
      >
        <ArrowLeft size={14} />
        Data sources
      </button>
      <div className="page-header">
        <div>
          <h1>
            Semantics <span className="semantic-source">/ {source.name}</span>
          </h1>
          <p>Business context and verified native queries for your Agents</p>
        </div>
        <div className="row-actions">
          <Button
            disabled={!!busy || unsavedOverview}
            onClick={() => action("reload", reload)}
            aria-label="Refresh semantics"
          >
            <RefreshCw size={16} />
          </Button>
          <Button
            disabled={!!busy || unsavedOverview}
            onClick={() =>
              action("validate", async () => {
                await api(endpoint + "/validate", {
                  method: "POST",
                  body: "{}",
                });
                await reload();
                notify("Structure and example parameters are valid.");
              })
            }
          >
            Validate draft
          </Button>
          <Button
            primary
            disabled={!!busy || unsavedOverview}
            onClick={() => setDialog("publish")}
          >
            Publish
          </Button>
        </div>
      </div>
      <div className="button-row">
        <Button
          disabled={!!busy || unsavedOverview}
          onClick={() => setDialog("visibility")}
        >
          Preview Agent visibility
        </Button>
      </div>
      <ErrorNote error={dialog ? "" : error} />
      <div className="semantic-status">
        <span className={`status ${state.changed ? "amber" : "green"}`}>
          {state.changed ? "Unpublished changes" : "Draft matches publication"}
        </span>
        <span>Draft revision {state.revision}</span>
        <span>
          {state.published_version === "0"
            ? "Not published"
            : `Published version ${state.published_version}`}
        </span>
        <span>
          {source.query_access_mode === "templates_only"
            ? "Templates only"
            : "Native queries and templates"}
        </span>
      </div>
      <div
        className="semantic-tabs"
        role="tablist"
        aria-label="Semantic sections"
      >
        {["Overview", "Catalog", "Query templates", "Ontology mapping"].map(
          (t) => (
            <button
              key={t}
              role="tab"
              aria-selected={tab === t}
              tabIndex={tab === t ? 0 : -1}
              onKeyDown={(e) => {
                if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
                  if (unsavedOverview) {
                    setError(
                      "Save or revert unsaved changes before switching sections.",
                    );
                    return;
                  }
                  const tabs = [
                    "Overview",
                    "Catalog",
                    "Query templates",
                    "Ontology mapping",
                  ];
                  const next =
                    tabs[
                      (tabs.indexOf(tab) +
                        (e.key === "ArrowRight" ? 1 : tabs.length - 1)) %
                        tabs.length
                    ];
                  setTab(next);
                  setKind("");
                  setPage(0);
                  (
                    e.currentTarget.parentElement?.querySelectorAll("button")[
                      tabs.indexOf(next)
                    ] as HTMLButtonElement
                  )?.focus();
                }
              }}
              onClick={() => {
                if (unsavedOverview) {
                  setError(
                    "Save or revert unsaved changes before switching sections.",
                  );
                  return;
                }
                setTab(t);
                setKind("");
                setPage(0);
              }}
            >
              {t}
              {(t === "Catalog" || t === "Query templates") && (
                <small>
                  {
                    entries.filter((en) =>
                      t === "Catalog"
                        ? en.kind !== "template"
                        : en.kind === "template",
                    ).length
                  }
                </small>
              )}
            </button>
          ),
        )}
      </div>
      {tab === "Ontology mapping" ? (
        <OntologyMapping
          endpoint={endpoint}
          state={state}
          accept={accept}
          notify={notify}
          onDirty={setMappingDirty}
        />
      ) : tab === "Overview" ? (
        <div className="semantic-overview" role="tabpanel">
          <section>
            <h2>Data source context</h2>
            <p className="help">
              Describe the business domain, source of truth, and conventions.
              Context is descriptive and does not change database permissions.
            </p>
            <Field label="Overview">
              <textarea
                rows={8}
                value={overview}
                onChange={(e) => setOverview(e.target.value)}
              />
            </Field>
            <Button
              primary
              disabled={!!busy || !unsavedOverview}
              onClick={() =>
                action("save", async () => {
                  accept(
                    await api<SemanticState>(endpoint, {
                      method: "PUT",
                      body: payload({
                        revision: state.revision,
                        snapshot: { ...state.draft, overview },
                      }),
                    }),
                  );
                  notify("Overview saved to draft.");
                })
              }
            >
              Save overview draft
            </Button>
            <Button
              disabled={!!busy || !unsavedOverview}
              onClick={() => setOverview(state.draft.overview)}
            >
              Revert unsaved overview
            </Button>
          </section>
          <section>
            <h2>Publication readiness</h2>
            <p>
              {entries.length} entries ·{" "}
              {state.validation.filter((v) => v.valid).length} verified
              templates · {trialRequired} requiring a trial
            </p>
            <p className="help">
              Enabled templates require a successful read-only trial against the
              current connection before publication. Connection, credential, or
              database version changes expire validation. Current source limits
              always apply.
            </p>
            <div className="button-row">
              <Button
                disabled={!!busy || unsavedOverview}
                onClick={() => setDialog("structure")}
              >
                Import structure
              </Button>
              <Button
                disabled={!!busy || unsavedOverview}
                onClick={() => {
                  setImportJSON("");
                  setDialog("import");
                }}
              >
                <Upload size={15} />
                Import JSON
              </Button>
              <Button
                onClick={() =>
                  action("export", async () => {
                    const snapshot = await api<SemanticSnapshot>(
                      endpoint + "/export",
                    );
                    const url = URL.createObjectURL(
                      new Blob([JSON.stringify(snapshot, null, 2)], {
                        type: "application/json",
                      }),
                    );
                    const a = document.createElement("a");
                    a.href = url;
                    a.download = "semantics.json";
                    a.click();
                    setTimeout(() => URL.revokeObjectURL(url), 1000);
                  })
                }
              >
                <Download size={15} />
                Export draft
              </Button>
              <Button
                disabled={!state.changed || !!busy || unsavedOverview}
                className="danger"
                onClick={() => setDialog("discard")}
              >
                Discard draft
              </Button>
            </div>
          </section>
        </div>
      ) : (
        <div role="tabpanel">
          <div className="filters">
            <div className="search-input">
              <Search size={16} />
              <input
                aria-label="Search semantic entries"
                placeholder="Search names, aliases and descriptions"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(0);
                }}
              />
            </div>
            {tab === "Catalog" && (
              <select
                aria-label="Filter entry kind"
                value={kind}
                onChange={(e) => {
                  setKind(e.target.value);
                  setPage(0);
                }}
              >
                <option value="">All catalog entries</option>
                {entryKinds.map((v) => (
                  <option key={v}>{v}</option>
                ))}
              </select>
            )}
            <Button disabled={!!busy} onClick={create}>
              <Plus size={16} />
              {tab === "Catalog" ? "Add entry" : "Add template"}
            </Button>
            {tab === "Catalog" && (
              <Button onClick={() => setDialog("structure")}>
                Import structure
              </Button>
            )}
          </div>
          {!rows.length ? (
            <Empty
              title={
                search || kind
                  ? "No matching entries"
                  : tab === "Catalog"
                    ? "Build your business catalog"
                    : "No query templates yet"
              }
              description={
                tab === "Catalog"
                  ? "Import metadata or add a term, object, field, relationship, or metric."
                  : "Create a native query with a parameter contract, then trial and publish it."
              }
              action={
                <Button onClick={create}>
                  {tab === "Catalog" ? "Add entry" : "Add template"}
                </Button>
              }
            />
          ) : (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>
                      {tab === "Catalog" ? "Kind / reference" : "Validation"}
                    </th>
                    <th>Description</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.slice(current * 20, current * 20 + 20).map((en) => {
                    const v = state.validation.find((v) => v.id === en.id);
                    const published = state.published.entries.find(
                      (p) => p.id === en.id && p.template?.enabled,
                    );
                    return (
                      <tr key={en.id}>
                        <td>
                          <button
                            className="text-button name-link"
                            onClick={() => setEditing(en)}
                          >
                            {en.name || "Untitled entry"}
                          </button>
                          <small className="block">{en.id}</small>
                        </td>
                        <td>
                          {v ? (
                            <>
                              <span
                                className={`status ${v.valid ? "green" : "amber"}`}
                              >
                                {v.status === "trial_required"
                                  ? "Trial required"
                                  : v.status === "expired"
                                    ? "Validation expired"
                                    : v.status === "disabled"
                                      ? "Disabled"
                                      : "Verified"}
                              </span>
                              <small className="block">
                                {v.checked_at
                                  ? date(v.checked_at)
                                  : "No successful trial"}
                              </small>
                            </>
                          ) : (
                            <>
                              {en.kind}
                              <small className="block">
                                {en.reference &&
                                  [
                                    en.reference.namespace,
                                    en.reference.object,
                                    en.reference.field,
                                  ]
                                    .filter(Boolean)
                                    .join(".")}
                              </small>
                            </>
                          )}
                        </td>
                        <td className="semantic-description">
                          {en.description || "No description"}
                        </td>
                        <td>
                          <div className="row-actions">
                            <Button
                              disabled={!!busy}
                              onClick={() => setEditing(en)}
                            >
                              Edit
                            </Button>
                            {en.template && (
                              <Button
                                busy={busy === en.id}
                                disabled={!!busy || !en.template.enabled}
                                onClick={() =>
                                  action(en.id, async () => {
                                    await api(endpoint + "/trial", {
                                      method: "POST",
                                      body: payload({
                                        revision: state.revision,
                                        template_id: en.id,
                                      }),
                                    });
                                    await reload();
                                    notify(
                                      "Read-only trial passed. No results were saved.",
                                    );
                                  })
                                }
                              >
                                Trial
                              </Button>
                            )}
                            {published && (
                              <Button
                                onClick={() => {
                                  setSelected(published);
                                  setDialog("preview");
                                }}
                              >
                                Preview published
                              </Button>
                            )}
                            <button
                              className="text-button danger"
                              disabled={!!busy}
                              onClick={() => {
                                setSelected(en);
                                setDialog("delete");
                              }}
                            >
                              Delete
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
          <div className="pagination">
            <span>{rows.length} entries</span>
            <div>
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
          </div>
        </div>
      )}
      {editing && (
        <SemanticEditor
          entry={editing}
          entries={entries}
          concepts={conceptRefs(state.draft.ontology)}
          onClose={() => setEditing(null)}
          onSave={async (entry) => {
            accept(
              await api<SemanticState>(
                `${endpoint}/entries/${encodeURIComponent(entry.id)}`,
                {
                  method: "PUT",
                  body: payload({ revision: state.revision, entry }),
                },
              ),
            );
            setEditing(null);
            notify("Entry saved to draft.");
          }}
        />
      )}
      {dialog === "structure" && (
        <StructureImport
          source={source}
          onClose={() => setDialog("")}
          onImport={async (objects) => {
            accept(
              await api<SemanticState>(endpoint + "/import-structure", {
                method: "POST",
                body: payload({ revision: state.revision, objects }),
              }),
            );
            setDialog("");
            notify(
              "Structure imported without replacing existing descriptions.",
            );
          }}
        />
      )}
      {dialog === "visibility" && (
        <SemanticVisibility
          endpoint={endpoint}
          agents={agents}
          onClose={() => setDialog("")}
        />
      )}
      {dialog === "preview" && selected && (
        <TemplatePreview
          source={source}
          entry={selected}
          agents={agents}
          onClose={() => setDialog("")}
        />
      )}
      {["publish", "discard", "delete", "import"].includes(dialog) && (
        <Drawer
          title={
            dialog === "publish"
              ? "Publish semantic catalog"
              : dialog === "discard"
                ? "Discard unpublished changes"
                : dialog === "delete"
                  ? "Delete draft entry"
                  : "Import semantic JSON"
          }
          onClose={() => {
            if (!busy) setDialog("");
          }}
          footer={
            <>
              <Button disabled={!!busy} onClick={() => setDialog("")}>
                Cancel
              </Button>
              <Button
                primary
                busy={!!busy}
                onClick={() =>
                  action(dialog, async () => {
                    let suffix = dialog;
                    let method = "POST";
                    let body: unknown = { revision: state.revision };
                    if (dialog === "delete") {
                      suffix = `entries/${encodeURIComponent(selected!.id)}`;
                      method = "DELETE";
                    }
                    if (dialog === "import") {
                      body = {
                        revision: state.revision,
                        snapshot: JSON.parse(importJSON),
                      };
                    }
                    accept(
                      await api<SemanticState>(`${endpoint}/${suffix}`, {
                        method,
                        body: payload(body),
                      }),
                    );
                    setDialog("");
                    notify(
                      dialog === "publish"
                        ? "Semantic catalog published."
                        : "Draft updated.",
                    );
                  })
                }
              >
                {dialog === "publish"
                  ? "Confirm publication"
                  : dialog === "discard"
                    ? "Discard draft"
                    : dialog === "delete"
                      ? "Delete from draft"
                      : "Replace draft with import"}
              </Button>
            </>
          }
        >
          <ErrorNote error={error} />
          {dialog === "publish" ? (
            <>
              <p>
                Publish {entries.length} entries as version{" "}
                {String(BigInt(state.published_version) + 1n)}. Agents will see
                this snapshot immediately.
              </p>
              <p>
                {trialRequired
                  ? `${trialRequired} enabled templates still require successful trials. Publication will be blocked until they pass.`
                  : "All enabled templates have current trial evidence. Publication will also validate the complete catalog."}
              </p>
              {state.draft.ontology && (
                <p>
                  Adopt ontology <code>{state.draft.ontology.ontology_id}</code>{" "}
                  version {state.draft.ontology.version} with{" "}
                  {state.draft.ontology.entities.length} mapped entities.{" "}
                  {state.mapping_validation?.status === "checked"
                    ? `${state.mapping_validation.checks?.filter((c) => c.status === "unverified").length || 0} fields remain administrator-declared and unverified.`
                    : "Structure check is required before publication."}
                </p>
              )}
              <p className="help">
                Changed or removed executable templates cancel affected queries.
                Description-only changes preserve running queries.
              </p>
            </>
          ) : dialog === "discard" ? (
            <p>
              Replace the saved draft with the published snapshot. Unpublished
              edits will be lost.
            </p>
          ) : dialog === "delete" ? (
            <p>
              Remove {selected?.name} from the draft. It remains visible to
              Agents until publication. Remove metric links to this template
              before publishing.
            </p>
          ) : (
            <>
              <p>
                Import versioned semantic and template configuration. This
                replaces the draft; credentials and validation evidence are
                never imported.
              </p>
              <Field label="JSON file">
                <input
                  type="file"
                  accept=".json,application/json"
                  onChange={async (e) => {
                    const file = e.target.files?.[0];
                    if (file) {
                      if (file.size > 768 * 1024) {
                        setError("File exceeds 768 KiB.");
                        return;
                      }
                      setImportJSON(await file.text());
                    }
                  }}
                />
              </Field>
              <Field label="Semantic JSON">
                <textarea
                  className="query-editor"
                  rows={16}
                  value={importJSON}
                  onChange={(e) => setImportJSON(e.target.value)}
                />
              </Field>
            </>
          )}
        </Drawer>
      )}
    </>
  );
}
