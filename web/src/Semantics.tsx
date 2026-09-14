import { SemanticHistory } from "./SemanticHistory";
import { t } from "./i18n";
import { useCallback, useEffect, useRef, useState } from "react";
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

const regressionLabels: Record<string, string> = {
  row_count_mismatch: "Row count outside expected range",
  column_mismatch: "Expected column or type not found",
  value_missing: "Expected value path not found",
  value_mismatch: "Value differs from expectation",
  incomplete_result: "Result is incomplete; narrow the test query",
  unreadable_result: "Could not read result",
  invalid_expectation: "Invalid expected value",
};

export function Semantics({
  source,
  agents,
  onBack,
  onWorkspace,
  notify,
  initialQuery = "",
}: {
  source: Source;
  agents: Agent[];
  onBack: () => void;
  onWorkspace: () => void;
  notify: (s: string) => void;
  initialQuery?: string;
}) {
  const [state, setState] = useState<SemanticState | null>(null);
  const initial = new URLSearchParams(initialQuery);
  const [tab, setTab] = useState(
    ["Overview", "Catalog", "Query templates", "Ontology mapping"].includes(
      initial.get("tab") || "",
    )
      ? initial.get("tab")!
      : "Overview",
  );
  const initialPreviewOpened = useRef(false);
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
  useEffect(() => {
    if (!state || initialPreviewOpened.current || !initial.get("template"))
      return;
    initialPreviewOpened.current = true;
    const entry = state.published.entries.find(
      (e) => e.id === initial.get("template") && e.template,
    );
    if (entry) {
      setSelected(entry);
      setDialog("preview");
    } else
      setError(t("Published template changed. Select a current template."));
  }, [state, initialQuery]);
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
          {t("Data sources")}
        </Button>
        <ErrorNote error={error} />
        {error ? (
          <Button onClick={() => action("reload", reload)}>
            {t("Retry loading semantics")}
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
      <div className="button-row semantic-navigation">
        <button
          className="text-button semantic-back"
          disabled={!!busy || unsavedOverview}
          onClick={onBack}
        >
          <ArrowLeft size={14} />
          {t("Data sources")}
        </button>
        <Button disabled={!!busy || unsavedOverview} onClick={onWorkspace}>
          {t("Query workspace")}
        </Button>
      </div>
      <div className="page-header">
        <div>
          <h1>
            {t("Semantics ")}
            <span className="semantic-source">/ {source.name}</span>
          </h1>
          <p>
            {t("Business context and verified native queries for your Agents")}
          </p>
        </div>
        <div className="button-row">
          <Button
            disabled={!!busy || unsavedOverview}
            onClick={() => setDialog("history")}
          >
            {t("Publication history")}
          </Button>
          <Button
            disabled={!!busy || unsavedOverview}
            onClick={() => action("reload", reload)}
            aria-label={t("Refresh semantics")}
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
                notify(t("Structure and example parameters are valid."));
              })
            }
          >
            {t("Validate draft")}
          </Button>
          <Button
            primary
            disabled={!!busy || unsavedOverview}
            onClick={() => setDialog("publish")}
          >
            {t("Publish")}
          </Button>
        </div>
      </div>
      <ErrorNote error={dialog ? "" : error} />
      <div className="semantic-publication-bar">
        <div className="semantic-status">
          <span className={`status ${state.changed ? "amber" : "green"}`}>
            {state.changed
              ? t("Unpublished changes")
              : t("Draft matches publication")}
          </span>
          <span>
            {t("Draft revision ")}
            {state.revision}
          </span>
          <span>
            {state.published_version === "0"
              ? t("Not published")
              : t("Published version {published_version}", {
                  published_version: state.published_version,
                })}
          </span>
          <span>
            {source.query_access_mode === "templates_only"
              ? t("Templates only")
              : t("Native queries and templates")}
          </span>
        </div>
        <Button
          disabled={!!busy || unsavedOverview}
          onClick={() => setDialog("visibility")}
        >
          {t("Preview Agent visibility")}
        </Button>
      </div>
      <div
        className="semantic-tabs"
        role="tablist"
        aria-label={t("Semantic sections")}
      >
        {["Overview", "Catalog", "Query templates", "Ontology mapping"].map(
          (section) => (
            <button
              key={section}
              role="tab"
              aria-selected={tab === section}
              tabIndex={tab === section ? 0 : -1}
              onKeyDown={(e) => {
                if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
                  if (unsavedOverview) {
                    setError(
                      t(
                        "Save or revert unsaved changes before switching sections.",
                      ),
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
                    t(
                      "Save or revert unsaved changes before switching sections.",
                    ),
                  );
                  return;
                }
                setTab(section);
                setKind("");
                setPage(0);
              }}
            >
              {t(section)}
              {(section === "Catalog" || section === "Query templates") && (
                <small>
                  {
                    entries.filter((en) =>
                      section === "Catalog"
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
            <h2>{t("Data source context")}</h2>
            <p className="help">
              {t(
                "Describe the business domain, source of truth, and conventions. Context is descriptive and does not change database permissions.",
              )}
            </p>
            <Field label={t("Overview")}>
              <textarea
                rows={8}
                value={overview}
                onChange={(e) => setOverview(e.target.value)}
              />
            </Field>
            <div className="button-row">
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
                    notify(t("Overview saved to draft."));
                  })
                }
              >
                {t("Save overview draft")}
              </Button>
              <Button
                disabled={!!busy || !unsavedOverview}
                onClick={() => setOverview(state.draft.overview)}
              >
                {t("Revert unsaved overview")}
              </Button>
            </div>
          </section>
          <section>
            <h2>{t("Publication readiness")}</h2>
            <p>
              {entries.length}
              {t(" entries ·")} {state.validation.filter((v) => v.valid).length}
              {t(" verified templates · ")}
              {trialRequired}
              {t(" requiring a trial")}
            </p>
            <p className="help">
              {t(
                "Enabled templates require a successful read-only trial against the current connection before publication. Connection, credential, or database version changes expire validation. Current source limits always apply.",
              )}
            </p>
            <div className="button-row">
              <Button
                busy={busy === "trial-all"}
                disabled={
                  !!busy ||
                  unsavedOverview ||
                  !state.validation.some((v) => v.status !== "disabled")
                }
                onClick={() =>
                  action("trial-all", async () => {
                    const result = await api<{ remaining: number }>(
                      endpoint + "/trial-all",
                      {
                        method: "POST",
                        body: payload({ revision: state.revision }),
                      },
                    );
                    await reload();
                    notify(
                      result.remaining
                        ? t(
                            "Some templates were not run before the time limit. Run their trials individually.",
                          )
                        : t(
                            "Trials finished. Review each template result before publishing.",
                          ),
                    );
                  })
                }
              >
                {t("Run all template checks")}
              </Button>
              <Button
                disabled={!!busy || unsavedOverview}
                onClick={() => setDialog("structure")}
              >
                {t("Import structure")}
              </Button>
              <Button
                disabled={!!busy || unsavedOverview}
                onClick={() => {
                  setImportJSON("");
                  setDialog("import");
                }}
              >
                <Upload size={15} />
                {t("Import JSON")}
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
                {t("Export draft")}
              </Button>
              <Button
                disabled={!state.changed || !!busy || unsavedOverview}
                className="danger"
                onClick={() => setDialog("discard")}
              >
                {t("Discard draft")}
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
                aria-label={t("Search semantic entries")}
                placeholder={t("Search names, aliases and descriptions")}
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(0);
                }}
              />
            </div>
            {tab === "Catalog" && (
              <select
                aria-label={t("Filter entry kind")}
                value={kind}
                onChange={(e) => {
                  setKind(e.target.value);
                  setPage(0);
                }}
              >
                <option value="">{t("All catalog entries")}</option>
                {entryKinds.map((v) => (
                  <option key={v} value={v}>
                    {t(v)}
                  </option>
                ))}
              </select>
            )}
            <Button disabled={!!busy} onClick={create}>
              <Plus size={16} />
              {tab === "Catalog" ? t("Add entry") : t("Add template")}
            </Button>
            {tab === "Catalog" && (
              <Button onClick={() => setDialog("structure")}>
                {t("Import structure")}
              </Button>
            )}
          </div>
          {!rows.length ? (
            <Empty
              title={
                search || kind
                  ? t("No matching entries")
                  : tab === "Catalog"
                    ? t("Build your business catalog")
                    : t("No query templates yet")
              }
              description={
                tab === "Catalog"
                  ? t(
                      "Import metadata or add a term, object, field, relationship, or metric.",
                    )
                  : t(
                      "Create a native query with a parameter contract, then trial and publish it.",
                    )
              }
              action={
                <Button onClick={create}>
                  {tab === "Catalog" ? t("Add entry") : t("Add template")}
                </Button>
              }
            />
          ) : (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>{t("Name")}</th>
                    <th>
                      {tab === "Catalog"
                        ? t("Kind / reference")
                        : t("Validation")}
                    </th>
                    <th>{t("Description")}</th>
                    <th>{t("Actions")}</th>
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
                            {en.name || t("Untitled entry")}
                          </button>
                          <small className="block">{en.id}</small>
                        </td>
                        <td>
                          {v ? (
                            <>
                              <span
                                className={`status ${v.valid ? "green" : "amber"}`}
                              >
                                {v.status === "regression_failed"
                                  ? t("Regression checks failed")
                                  : v.status === "trial_required"
                                    ? t("Trial required")
                                    : v.status === "expired"
                                      ? t("Validation expired")
                                      : v.status === "disabled"
                                        ? t("Disabled")
                                        : t("Verified")}
                              </span>
                              <small className="block">
                                {v.checked_at
                                  ? date(v.checked_at)
                                  : t("No successful trial")}
                              </small>
                              {v?.report && (
                                <details
                                  className="trial-report"
                                  open={!v.report.passed}
                                >
                                  <summary>{t("Trial report")}</summary>
                                  {v.report.cases.map((c) => (
                                    <p key={c.name}>
                                      {c.name} ·{" "}
                                      {c.passed
                                        ? t("Passed")
                                        : t(
                                            regressionLabels[
                                              c.error_code || ""
                                            ] ||
                                              c.error_code ||
                                              "Failed",
                                          )}{" "}
                                      · {t("{count} rows", { count: c.rows })}
                                    </p>
                                  ))}
                                </details>
                              )}
                            </>
                          ) : (
                            <>
                              {t(en.kind)}
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
                          {en.description || t("No description")}
                        </td>
                        <td>
                          <div className="row-actions">
                            <Button
                              disabled={!!busy}
                              onClick={() => setEditing(en)}
                            >
                              {t("Edit")}
                            </Button>
                            {en.template && (
                              <Button
                                busy={busy === en.id}
                                disabled={!!busy || !en.template.enabled}
                                onClick={() =>
                                  action(en.id, async () => {
                                    try {
                                      await api(endpoint + "/trial", {
                                        method: "POST",
                                        body: payload({
                                          revision: state.revision,
                                          template_id: en.id,
                                        }),
                                      });
                                    } finally {
                                      await reload();
                                    }
                                    notify(
                                      t(
                                        "Read-only trial passed. No results were saved.",
                                      ),
                                    );
                                  })
                                }
                              >
                                {t("Trial")}
                              </Button>
                            )}
                            {published && (
                              <Button
                                onClick={() => {
                                  setSelected(published);
                                  setDialog("preview");
                                }}
                              >
                                {t("Preview published")}
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
                              {t("Delete")}
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
            <span>
              {rows.length}
              {t(" entries")}
            </span>
            <div>
              <Button
                disabled={current === 0}
                onClick={() => setPage(current - 1)}
              >
                {t("Previous")}
              </Button>
              <span>{current + 1}</span>
              <Button
                disabled={(current + 1) * 20 >= rows.length}
                onClick={() => setPage(current + 1)}
              >
                {t("Next")}
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
            notify(t("Entry saved to draft."));
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
              t("Structure imported without replacing existing descriptions."),
            );
          }}
        />
      )}
      {dialog === "history" && (
        <SemanticHistory
          endpoint={endpoint}
          state={state}
          accept={accept}
          onClose={() => setDialog("")}
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
          initialAgentID={initial.get("agent_id") || ""}
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
              ? t("Publish semantic catalog")
              : dialog === "discard"
                ? t("Discard unpublished changes")
                : dialog === "delete"
                  ? t("Delete draft entry")
                  : t("Import semantic JSON")
          }
          onClose={() => {
            if (!busy) setDialog("");
          }}
          footer={
            <>
              <Button disabled={!!busy} onClick={() => setDialog("")}>
                {t("Cancel")}
              </Button>
              <Button
                primary
                busy={!!busy}
                disabled={
                  dialog === "publish" &&
                  (trialRequired > 0 ||
                    (!!state.draft.ontology &&
                      state.mapping_validation?.status !== "checked"))
                }
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
                        ? t("Semantic catalog published.")
                        : t("Draft updated."),
                    );
                  })
                }
              >
                {dialog === "publish"
                  ? t("Confirm publication")
                  : dialog === "discard"
                    ? t("Discard draft")
                    : dialog === "delete"
                      ? t("Delete from draft")
                      : t("Replace draft with import")}
              </Button>
            </>
          }
        >
          <ErrorNote error={error} />
          {dialog === "publish" ? (
            <>
              <p>
                {t("Publish ")}
                {entries.length}
                {t(" entries as version")}{" "}
                {String(BigInt(state.published_version) + 1n)}
                {t(". Agents will see this snapshot immediately.")}
              </p>
              <p>
                {trialRequired
                  ? t(
                      "{trialRequired} enabled templates still require successful trials. Publication will be blocked until they pass.",
                      { trialRequired: trialRequired },
                    )
                  : t(
                      "All enabled templates have current trial evidence. Publication will also validate the complete catalog.",
                    )}
              </p>
              {trialRequired > 0 && (
                <Button
                  onClick={() => {
                    setDialog("");
                    setTab("Query templates");
                  }}
                >
                  {t("Review templates")}
                </Button>
              )}
              {state.draft.ontology && (
                <p>
                  {t("Adopt ontology ")}
                  <code>{state.draft.ontology.ontology_id}</code>{" "}
                  {t("version ")}
                  {state.draft.ontology.version}
                  {t(" with")} {state.draft.ontology.entities.length}
                  {t(" mapped entities.")}{" "}
                  {state.mapping_validation?.status === "checked"
                    ? t(
                        "{value1} fields remain administrator-declared and unverified.",
                        {
                          value1:
                            state.mapping_validation.checks?.filter(
                              (c) => c.status === "unverified",
                            ).length || 0,
                        },
                      )
                    : t("Structure check is required before publication.")}
                </p>
              )}
              <p className="help">
                {t(
                  "Changed or removed executable templates cancel affected queries. Description-only changes preserve running queries.",
                )}
              </p>
            </>
          ) : dialog === "discard" ? (
            <p>
              {t(
                "Replace the saved draft with the published snapshot. Unpublished edits will be lost.",
              )}
            </p>
          ) : dialog === "delete" ? (
            <p>
              {t("Remove ")}
              {selected?.name}
              {t(
                " from the draft. It remains visible to Agents until publication. Remove metric links to this template before publishing.",
              )}
            </p>
          ) : (
            <>
              <p>
                {t(
                  "Import versioned semantic and template configuration. This replaces the draft; credentials and validation evidence are never imported.",
                )}
              </p>
              <Field label={t("JSON file")}>
                <input
                  type="file"
                  accept=".json,application/json"
                  onChange={async (e) => {
                    const file = e.target.files?.[0];
                    if (file) {
                      if (file.size > 768 * 1024) {
                        setError(t("File exceeds 768 KiB."));
                        return;
                      }
                      setImportJSON(await file.text());
                    }
                  }}
                />
              </Field>
              <Field label={t("Semantic JSON")}>
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
