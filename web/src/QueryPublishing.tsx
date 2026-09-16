import { useEffect, useRef, useState } from "react";
import { ArrowLeft, CheckCircle2 } from "lucide-react";
import { api, date, message, payload } from "./api";
import {
  Button,
  Drawer,
  Empty,
  ErrorNote,
  Field,
  Loading,
  Protection,
} from "./components";
import { SourceEditor } from "./SourceEditor";
import { AgentEditor } from "./AgentEditor";
import { AgentConnection } from "./AgentConnection";
import { QueryClientSetup } from "./QueryClientSetup";
import { QueryDraftEditor } from "./QueryDraftEditor";
import { SemanticReview } from "./SemanticReview";
import { TemplatePreview } from "./SemanticTools";
import { httpTemplate } from "./http-template";
import {
  nativeTemplateQuery,
  queryToolURL,
  rememberJourney,
  sameQueryEntry,
  queryNextStep,
  queryStepLabels,
  catalogReturnURL,
} from "./query-publishing";
import { semanticsURL, useReadiness } from "./readiness";
import { useNavigationGuard } from "./useNavigationGuard";
import { t } from "./i18n";
import type { Agent, Capability, Settings, Source } from "./types";
import type { SemanticEntry, SemanticState } from "./semantic-types";
import "./query-publishing.css";
import { linkQueryConcept } from "./query-concepts";
import { BusinessConcepts } from "./BusinessConcepts";

export function QueryPublishing({
  sources,
  agents,
  catalog,
  settings,
  administratorID,
  initialQuery,
  seed,
  navigate,
  reload,
  notify,
  clearSeed,
}: {
  sources: Source[];
  agents: Agent[];
  catalog: Capability[];
  settings: Settings | null;
  administratorID: string;
  initialQuery: string;
  seed?: { sourceID: string; query: string };
  navigate: (url: string) => void;
  reload: () => Promise<void>;
  notify: (text: string) => void;
  clearSeed: () => void;
}) {
  const route = new URLSearchParams(initialQuery);
  const sourceID = route.get("source_id") || "",
    entryID = route.get("template_id") || "";
  const source = sources.find((s) => s.id === sourceID);
  const conceptContext =
    route.get("ontology_id") && route.get("concept_ref")
      ? {
          ontology_id: route.get("ontology_id")!,
          concept_ref: route.get("concept_ref")!,
        }
      : undefined;
  const fromSemantics = route.get("from") === "semantics";
  const catalogReturn = catalogReturnURL(route.get("return_to"));
  const withCatalogReturn = (url: string) =>
    catalogReturn
      ? `${url}&${new URLSearchParams({ return_to: catalogReturn })}`
      : url;
  const queryURL = (id = "", agent = agentID) =>
    withCatalogReturn(
      queryToolURL(sourceID, id, agent, conceptContext) +
        (fromSemantics ? "&from=semantics" : ""),
    );
  const conceptReturn = conceptContext?.concept_ref.startsWith(
    "ontology:entity_type:",
  )
    ? `/ontologies/${encodeURIComponent(conceptContext.ontology_id)}?${new URLSearchParams({ entity: conceptContext.concept_ref.slice("ontology:entity_type:".length), section: "queries", source_id: sourceID })}`
    : "";
  const title = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    title.current?.focus();
    title.current?.scrollIntoView({ block: "start" });
  }, [entryID]);
  const [state, setState] = useState<SemanticState | null>(null);
  const selectedRouteAgent = route.get("agent_id") || "";
  const [agentID, setAgentID] = useState(selectedRouteAgent);
  useEffect(() => setAgentID(selectedRouteAgent), [selectedRouteAgent]);
  const [editing, setEditing] = useState<SemanticEntry | null>(null);
  const [editStep, setEditStep] = useState(1);
  const [editSection, setEditSection] = useState("query");
  const [showClient, setShowClient] = useState(false);
  const clientHeading = useRef<HTMLHeadingElement>(null);
  const [dialog, setDialog] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  useNavigationGuard(false, !!busy);
  const [loadKey, setLoadKey] = useState(0);
  const [connection, setConnection] = useState<{
    agent: Agent;
    token?: string;
  } | null>(null);
  const endpoint = `/api/sources/${encodeURIComponent(sourceID)}/semantics`;
  const entry = state?.draft.entries.find(
    (e) => e.id === entryID && e.template,
  );
  const published = state?.published.entries.find(
    (e) => e.id === entryID && e.template,
  );
  const ready = useReadiness(
    state ? sourceID : "",
    `${source?.revision}:${state?.revision}:${state?.published_version}`,
    entry || published ? entryID : "",
    agentID,
  );
  const validation = state?.validation.find((v) => v.id === entryID);
  const execution = ready.data?.templates.find((v) => v.id === entryID);
  const activeAgents = agents.filter((a) =>
    ready.data?.active_agents.includes(a.id),
  );
  const selectedAgent = activeAgents.find((a) => a.id === agentID);
  const currentPublication = !!published && sameQueryEntry(entry, published);
  const activity = ready.data?.template_activity;
  const confirmed =
    !!selectedAgent &&
    !!execution?.executable &&
    !!activity?.successful_calls &&
    activity.agent_id === agentID &&
    activity.template_id === entryID &&
    activity.execution_version === published?.template?.execution_version;
  const nextStep = state
    ? queryNextStep(state, ready.data, entryID, agentID)
    : "loading";
  useEffect(() => {
    if (!agentID && activeAgents.length === 1) setAgentID(activeAgents[0].id);
  }, [agentID, activeAgents.map((a) => a.id).join(",")]);
  useEffect(() => {
    if (source && (entry || published)) remember();
  }, [sourceID, entry?.id, published?.id, agentID]);
  function openClient() {
    setShowClient(true);
    requestAnimationFrame(() => {
      clientHeading.current?.focus();
      clientHeading.current?.scrollIntoView({
        block: "start",
        behavior: "smooth",
      });
    });
  }
  const stage = !source
    ? 0
    : editing
      ? editStep
      : !entry && !published
        ? 1
        : !currentPublication || !execution?.executable
          ? 3
          : 4;
  useEffect(() => {
    if (!source) return;
    const abort = new AbortController();
    setError("");
    api<SemanticState>(endpoint, { signal: abort.signal })
      .then(setState)
      .catch((e) => {
        if (!abort.signal.aborted) setError(message(e));
      });
    return () => abort.abort();
  }, [sourceID, endpoint, loadKey]);
  function remember(id = entryID, selected = agentID) {
    rememberJourney(administratorID, {
      source_id: sourceID,
      template_id: id,
      agent_id: selected,
    });
  }
  function edit(value?: SemanticEntry, section = "query") {
    setError("");
    setEditStep(1);
    setEditSection(section);
    if (value) {
      setEditing(structuredClone(value));
      return;
    }
    if (!source) return;
    try {
      const native =
        seed?.sourceID === sourceID
          ? seed.query
          : JSON.stringify(source.capability?.example || {});
      const created: SemanticEntry = {
        id: `template_${crypto.randomUUID()}`,
        kind: "template",
        name: "",
        template: {
          enabled: true,
          tool: source.capability?.tool || "query_sql",
          query_json: nativeTemplateQuery(native),
          parameters: [],
          example_json: "{}",
          ...(source.http_api?.operations[0] && seed?.sourceID !== sourceID
            ? httpTemplate(source.http_api.operations[0])
            : {}),
        },
      };
      setEditing(
        conceptContext && state
          ? linkQueryConcept(
              state.draft,
              created,
              conceptContext.ontology_id,
              conceptContext.concept_ref,
            )
          : created,
      );
    } catch (e) {
      setError(message(e));
    }
  }
  const initialEdit = useRef(false);
  useEffect(() => {
    if (!state || initialEdit.current || route.get("edit") !== "1") return;
    initialEdit.current = true;
    if (entryID && !entry) return;
    edit(entry);
  }, [state, entryID]);
  async function refresh() {
    setError("");
    setBusy("refresh");
    try {
      await reload();
      if (source) setState(await api<SemanticState>(endpoint));
      ready.refresh();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy("");
    }
  }
  async function trial() {
    setBusy("trial");
    setError("");
    try {
      await api(endpoint + "/trial", {
        method: "POST",
        body: payload({ revision: state!.revision, template_id: entryID }),
      });
      notify(t("Checks passed."));
    } catch (e) {
      setError(message(e));
    } finally {
      try {
        setState(await api<SemanticState>(endpoint));
        await reload();
        ready.refresh();
      } catch (e) {
        setState(null);
        setError(message(e));
      }
      setBusy("");
    }
  }
  return (
    <>
      <Button
        onClick={() =>
          navigate(
            conceptReturn ||
              (fromSemantics
                ? semanticsURL(sourceID, "Query templates")
                : catalogReturn || "/business"),
          )
        }
      >
        <ArrowLeft size={15} />
        {t(
          conceptReturn
            ? "Back to concept"
            : fromSemantics
              ? "Back to source queries"
              : catalogReturn
                ? "Back to search results"
                : "Query tools",
        )}
      </Button>
      <div className="page-header">
        <div>
          <h1 ref={title} tabIndex={-1}>
            {entry?.name || published?.name || t("Publish a query")}
          </h1>
        </div>
        {source && !editing && (
          <Button disabled={!!busy} onClick={refresh}>
            {t("Refresh evidence")}
          </Button>
        )}
      </div>
      {!entry && !published && (
        <ol
          className="query-progress"
          aria-label={t("Query publication steps")}
        >
          {[
            "Choose source",
            "Provide query",
            "Purpose and inputs",
            "Verify and publish",
            "Confirm client call",
          ].map((label, i) => (
            <li
              key={label}
              aria-current={stage === i ? "step" : undefined}
              className={stage === i ? "current" : ""}
            >
              <span>
                {i === 4 && confirmed ? <CheckCircle2 size={18} /> : i + 1}
              </span>
              {t(label)}
            </li>
          ))}
        </ol>
      )}
      <ErrorNote error={error || ready.error} />
      {!source ? (
        <section className="business-panel">
          <h2>{t("Choose a data source")}</h2>
          {sourceID && (
            <p className="notice warning">
              {t(
                "The saved data source is no longer available. Choose another source to continue.",
              )}
            </p>
          )}
          <Field label={t("Data source")}>
            <select
              value=""
              onChange={(e) => {
                clearSeed();
                navigate(withCatalogReturn(queryToolURL(e.target.value)));
              }}
            >
              <option value="">{t("Select a data source")}</option>
              {sources.map((s) => (
                <option value={s.id} key={s.id}>
                  {s.name} · {s.kind}
                  {!s.enabled ? ` · ${t("Disabled")}` : ""}
                </option>
              ))}
            </select>
          </Field>
          <Button primary={!sources.length} onClick={() => setDialog("source")}>
            {t("Connect a new data source")}
          </Button>
        </section>
      ) : (
        <>
          <section className="query-source-summary">
            <div>
              <strong>{source.name}</strong>
              <span className="status muted">
                {source.query_access_mode === "templates_only"
                  ? t("Templates only")
                  : t("Native queries and templates")}
              </span>
            </div>
            {!editing && (
              <div className="button-row">
                <Button onClick={() => setDialog("source")}>
                  {t("Connection settings")}
                </Button>
                <Button
                  onClick={() => {
                    clearSeed();
                    navigate(withCatalogReturn(queryToolURL()));
                  }}
                >
                  {t("Choose another source")}
                </Button>
              </div>
            )}
          </section>
          <details
            className="query-protection"
            open={!source.probe?.connected || !source.enabled}
          >
            <summary>
              {source.probe?.connected
                ? t("Connection and protection")
                : t("Check connection before trial")}
            </summary>
            <Protection probe={source.probe} detail />
            <Button
              busy={busy === "test"}
              disabled={!!busy || !!editing}
              onClick={async () => {
                setBusy("test");
                setError("");
                try {
                  await api(`/api/sources/${source.id}/test`, {
                    method: "POST",
                    body: "{}",
                  });
                  await reload();
                  ready.refresh();
                } catch (e) {
                  setError(message(e));
                } finally {
                  setBusy("");
                }
              }}
            >
              {t("Test connection")}
            </Button>
            {!source.enabled && (
              <p className="notice warning">
                {t(
                  "Source disabled. Enable it in connection settings before querying.",
                )}
              </p>
            )}
          </details>
          {!state ? (
            error ? (
              <Button onClick={() => setLoadKey((n) => n + 1)}>
                {t("Reload saved draft")}
              </Button>
            ) : (
              <Loading />
            )
          ) : editing ? (
            <QueryDraftEditor
              key={editing.id}
              source={source}
              entry={editing}
              revision={state.revision}
              ontology={state.draft.ontology}
              creating={!state.draft.entries.some((e) => e.id === editing.id)}
              initialSection={editSection}
              onStep={setEditStep}
              onCancel={() => setEditing(null)}
              onSaved={(next, id, finish) => {
                setState(next);
                remember(id);
                clearSeed();
                if (finish || entryID !== id) {
                  setEditing(null);
                  navigate(queryURL(id));
                }
                notify(t("Draft saved."));
              }}
            />
          ) : !entry && !published ? (
            <section className="business-panel">
              {entryID && (
                <p className="notice warning">
                  {t(
                    "This saved query is no longer in the draft or publication. Select another saved query or create a new one.",
                  )}
                </p>
              )}
              <Empty
                title={t("Start with a query you already use")}
                description={t("Add a read query or continue a saved draft.")}
                action={
                  <Button primary onClick={() => edit()}>
                    {seed?.sourceID === sourceID
                      ? t("Use copied preview query")
                      : t("Provide a query")}
                  </Button>
                }
              />
              {state.draft.entries.some((e) => e.template) && (
                <Field label={t("Continue a saved query")}>
                  <select
                    value=""
                    onChange={(e) => {
                      remember(e.target.value);
                      navigate(queryURL(e.target.value));
                    }}
                  >
                    <option value="">{t("Choose a saved draft")}</option>
                    {state.draft.entries
                      .filter((e) => e.template)
                      .map((e) => (
                        <option key={e.id} value={e.id}>
                          {e.name || e.id}
                        </option>
                      ))}
                  </select>
                </Field>
              )}
            </section>
          ) : (
            <>
              <section
                className="query-next-action"
                aria-label={t("Query status")}
              >
                <div>
                  <strong>
                    {!source.enabled
                      ? t("Source disabled")
                      : nextStep === "trial"
                        ? t("Checks needed")
                        : nextStep === "publish"
                          ? t("Unpublished changes")
                          : nextStep === "loading"
                            ? t("Loading…")
                            : nextStep === "disabled"
                              ? t("Query disabled")
                              : t("Published")}
                  </strong>
                  <span className="help">
                    {nextStep === "done"
                      ? t("Client call confirmed")
                      : nextStep === "access" || nextStep === "client"
                        ? t("Ready for client setup")
                        : currentPublication
                          ? t("Draft matches publication")
                          : t("Draft changes are not yet visible to clients.")}
                  </span>
                </div>
                <Button
                  primary
                  busy={busy === "trial"}
                  disabled={!!busy || !!ready.error || nextStep === "loading"}
                  onClick={() => {
                    if (!source.enabled) setDialog("source");
                    else if (nextStep === "trial") void trial();
                    else if (nextStep === "publish") setDialog("review");
                    else if (nextStep === "disabled") edit(entry, "purpose");
                    else if (nextStep === "done") setDialog("preview");
                    else openClient();
                  }}
                >
                  {t(
                    !source.enabled
                      ? "Connection settings"
                      : nextStep === "disabled"
                        ? "Edit query"
                        : nextStep === "done"
                          ? "Preview as Agent"
                          : nextStep === "client" || nextStep === "access"
                            ? "Set up client"
                            : queryStepLabels[nextStep],
                  )}
                </Button>
              </section>
              {state.changed && currentPublication && (
                <p className="help">
                  <button
                    className="text-button"
                    disabled={!!busy}
                    onClick={() => setDialog("review")}
                  >
                    {t("Review other source changes")}
                  </button>
                </p>
              )}
              <BusinessConcepts
                binding={state.draft.ontology}
                selected={(entry || published)?.template?.concept_refs || []}
              />
              <div className="query-workspace">
                <section className="business-panel">
                  <div className="section-heading">
                    <h2>{t("Query details")}</h2>
                    <div className="button-row">
                      <Button onClick={() => setDialog("evidence")}>
                        {t("Validation and versions")}
                      </Button>
                      <Button
                        disabled={!!busy || !entry}
                        onClick={() => edit(entry, "purpose")}
                      >
                        {t("Edit draft")}
                      </Button>
                    </div>
                  </div>
                  {!entry && (
                    <p className="help">
                      {t("Published version · removed from draft")}
                    </p>
                  )}
                  <p>
                    {(entry || published)?.description ||
                      t(
                        "Add a purpose so Agents know when to call this query.",
                      )}
                  </p>
                  <h3>{t("Inputs")}</h3>
                  <ul>
                    {((entry || published)?.template?.parameters || []).map(
                      (p) => (
                        <li key={p.name}>
                          <strong>{p.name}</strong> · {t(p.type)} ·{" "}
                          {p.required ? t("Required") : t("Optional")}
                          <p className="help">{p.description}</p>
                        </li>
                      ),
                    )}
                  </ul>
                  {!(entry || published)?.template?.parameters.length && (
                    <p className="help">
                      {t("This template needs no parameters.")}
                    </p>
                  )}
                  <h3>{t("Returns")}</h3>
                  <p>
                    {(entry || published)?.template?.result_description ||
                      t("Add a result description before sharing this query.")}
                  </p>
                  <details className="advanced">
                    <summary>{t("Business definitions and recovery")}</summary>
                    <div className="button-row">
                      <Button
                        onClick={() =>
                          navigate(semanticsURL(sourceID, "Query templates"))
                        }
                      >
                        {t("Semantics and publication history")}
                      </Button>
                      <Button
                        onClick={() =>
                          navigate(`/sources/${sourceID}/evaluation`)
                        }
                      >
                        {t("Review a business question")}
                      </Button>
                    </div>
                    <p className="help">
                      {t(
                        "Business reviews currently belong to source evaluation runs. They are not automatically attributed to this template.",
                      )}
                    </p>
                  </details>
                </section>
              </div>
              {dialog === "evidence" && (
                <Drawer
                  title={t("Validation and versions")}
                  onClose={() => setDialog("")}
                  footer={
                    <Button onClick={() => setDialog("")}>{t("Close")}</Button>
                  }
                >
                  <p className="help">
                    {t("Source publication {version}", {
                      version: state.published_version,
                    })}{" "}
                    ·{" "}
                    {t("Execution {version}", {
                      version: published?.template?.execution_version || "—",
                    })}
                  </p>
                  <p>
                    {validation?.valid
                      ? t("Saved checks passed.")
                      : t("Run checks before publishing.")}
                  </p>
                  {validation?.checked_at && (
                    <small>
                      {t("Checked ")}
                      {date(validation.checked_at)}
                    </small>
                  )}
                  {validation?.report && (
                    <ul className="query-trial-results">
                      {validation.report.cases.map((c, i) => (
                        <li key={i}>
                          <strong>{c.name}</strong>
                          <span>
                            {c.passed ? t("Passed") : t("Failed")} · {c.rows}{" "}
                            {t("rows")} · {c.elapsed_ms} ms
                          </span>
                          {c.error_code && <code>{c.error_code}</code>}
                        </li>
                      ))}
                    </ul>
                  )}
                  <div className="button-row">
                    <Button
                      primary={nextStep === "trial"}
                      disabled={!!busy || !entry || !source.enabled}
                      busy={busy === "trial"}
                      onClick={() => {
                        setDialog("");
                        void trial();
                      }}
                    >
                      {t("Run checks")}
                    </Button>
                    {state.changed && (
                      <Button
                        primary={nextStep === "publish"}
                        disabled={!!busy}
                        onClick={() => setDialog("review")}
                      >
                        {t("Review and publish")}
                      </Button>
                    )}
                  </div>
                  <p className="help">
                    {t("Execution does not imply acceptance.")}
                  </p>
                </Drawer>
              )}
              <details
                className="query-client-details business-panel"
                open={showClient}
                onToggle={(event) => setShowClient(event.currentTarget.open)}
              >
                <summary>
                  <h2 ref={clientHeading} tabIndex={-1}>
                    {t("Agent client")}
                  </h2>
                  <span className="help">
                    {confirmed
                      ? t("Client call confirmed")
                      : t("Set up client")}
                  </span>
                </summary>
                <QueryClientSetup
                  key={`${sourceID}:${entryID}:${agentID}:${published?.template?.execution_version}:${source?.revision}`}
                  sourceID={sourceID}
                  entryID={entryID}
                  agentID={agentID}
                  activeAgents={activeAgents}
                  selectedAgent={selectedAgent}
                  published={published}
                  executable={!!execution?.executable}
                  confirmed={confirmed}
                  readiness={ready.data}
                  busy={!!busy}
                  active={
                    showClient && !busy && !editing && !dialog && !connection
                  }
                  selectAgent={(id) => {
                    setAgentID(id);
                    remember(entryID, id);
                    navigate(queryURL(entryID, id));
                  }}
                  connect={(agent) => setConnection({ agent })}
                  createAgent={() => setDialog("agent")}
                  preview={() => setDialog("preview")}
                  refresh={ready.refresh}
                  reload={refresh}
                />
              </details>
            </>
          )}
        </>
      )}
      {dialog === "source" && (
        <SourceEditor
          source={source || null}
          catalog={catalog}
          initialQueryAccessMode="templates_only"
          onClose={() => setDialog("")}
          onSaved={async (notice, close, id) => {
            await reload();
            if (notice) notify(notice);
            if (close !== false) {
              setDialog("");
              if (id && id !== sourceID)
                navigate(withCatalogReturn(queryToolURL(id)));
            }
            ready.refresh();
          }}
        />
      )}
      {dialog === "review" && state && (
        <SemanticReview
          endpoint={endpoint}
          state={state}
          focusedTemplateID={entryID}
          onState={setState}
          onClose={() => setDialog("")}
          onRepair={(tab, id) => {
            setDialog("");
            if (id === entryID && entry) edit(entry);
            else
              navigate(
                semanticsURL(sourceID, tab) +
                  (id ? `&focus=${encodeURIComponent(id)}` : ""),
              );
          }}
          onPublished={() => {
            setDialog("");
            ready.refresh();
            notify(t("Changes published."));
          }}
        />
      )}
      {dialog === "agent" && source && (
        <AgentEditor
          agent={null}
          sources={sources}
          initialSources={[sourceID]}
          reload={reload}
          onClose={() => setDialog("")}
          saved={async (result) => {
            setDialog("");
            setConnection(result);
            setAgentID(result.agent.id);
            remember(entryID, result.agent.id);
            navigate(queryURL(entryID, result.agent.id));
            await reload().catch((e) => setError(message(e)));
            ready.refresh();
          }}
        />
      )}
      {connection && (
        <AgentConnection
          agent={connection.agent}
          sources={sources}
          navigate={navigate}
          endpoint={settings?.mcp_url || ""}
          token={connection.token || ""}
          error={error}
          refresh={refresh}
          onClose={() => setConnection(null)}
        />
      )}
      {dialog === "preview" && source && published && (
        <TemplatePreview
          source={source}
          agents={agents}
          entry={published}
          initialAgentID={agentID}
          onClose={() => setDialog("")}
        />
      )}
    </>
  );
}
