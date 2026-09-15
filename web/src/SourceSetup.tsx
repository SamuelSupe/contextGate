import { t } from "./i18n";
import { useState, type ReactNode } from "react";
import { api, date, message } from "./api";
import { Button, ErrorNote, Loading, Protection, Field } from "./components";
import { SourceEditor } from "./SourceEditor";
import { SourceDetails } from "./SourceDetails";
import { TemplatePreview } from "./SemanticTools";
import {
  connectionReady as isConnected,
  readinessLabel,
  semanticsURL,
  useReadiness,
} from "./readiness";
import type { SemanticEntry, SemanticState } from "./semantic-types";
import type { Source, Agent, Capability } from "./types";
import "./product-workflows.css";
export function SourceSetup({
  source,
  agents,
  catalog,
  reload,
  navigate,
  notify,
}: {
  source: Source;
  agents: Agent[];
  catalog: Capability[];
  reload: () => Promise<void>;
  navigate: (url: string) => void;
  notify: (text: string) => void;
}) {
  const {
    data,
    error: loadError,
    refresh,
  } = useReadiness(source.id, source.revision);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [agentID, setAgentID] = useState(""),
    [dialog, setDialog] = useState("");
  const [template, setTemplate] = useState<SemanticEntry | null>(null);
  async function test() {
    setBusy(true);
    setError("");
    try {
      await api(`/api/sources/${source.id}/test`, {
        method: "POST",
        body: "{}",
      });
      await reload();
      refresh();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function preview(id: string) {
    setBusy(true);
    setError("");
    try {
      const state = await api<SemanticState>(
        `/api/sources/${source.id}/semantics`,
      );
      const entry = state.published.entries.find(
        (e) => e.id === id && e.template,
      );
      if (!entry) throw new Error(t("Template changed. Refresh setup."));
      setTemplate(entry);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  const active = agents.filter((a) => data?.active_agents.includes(a.id));
  const selectedAgent =
    active.find((a) => a.id === agentID)?.id || active[0]?.id || "";
  const connectionReady = isConnected(source);
  const workspaceReady = !!(
    source.enabled &&
    selectedAgent &&
    data?.last_query &&
    source.probe?.connected &&
    (source.query_access_mode !== "templates_only" || data.executable_templates)
  );
  const queryContent = data && (
    <>
      <h2>
        {workspaceReady
          ? t("Queries for your Agent")
          : t("Prepare your first query")}
      </h2>
      <p>
        {source.query_access_mode === "templates_only"
          ? t("Templates only: choose a published, verified template.")
          : t("Run a bounded native query or choose a verified template.")}
      </p>
      <p className="query-identity">
        <strong>
          {t("Preview as")}{" "}
          {active.find((a) => a.id === selectedAgent)?.name ||
            t("— select an Agent above")}
        </strong>
      </p>
      {data.templates.length ? (
        <ul className="workflow-items">
          {data.templates.map((queryTemplate) => (
            <li key={queryTemplate.id}>
              <div>
                <strong>{queryTemplate.name}</strong>
                {queryTemplate.description && (
                  <p className="template-purpose">
                    {queryTemplate.description}
                  </p>
                )}
                <small>
                  {queryTemplate.executable
                    ? t("Executable")
                    : t(queryTemplate.status.replaceAll("_", " "))}{" "}
                  {t("· execution ")}
                  {queryTemplate.execution_version}
                </small>
              </div>
              <Button
                primary={queryTemplate.executable}
                disabled={
                  busy ||
                  !queryTemplate.executable ||
                  !source.enabled ||
                  !selectedAgent
                }
                onClick={() => preview(queryTemplate.id)}
              >
                {t("Preview as Agent")}
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="help">
          {source.query_access_mode === "templates_only"
            ? t("Publish a verified template before this Agent can query.")
            : t(
                "No templates yet. You can begin with a native query and add business context later.",
              )}
        </p>
      )}
      <div className="button-row">
        {source.query_access_mode !== "templates_only" && (
          <Button
            primary={!data.executable_templates}
            disabled={busy || !source.enabled || !selectedAgent}
            onClick={() => setDialog("native")}
          >
            {t("Preview native query as Agent")}
          </Button>
        )}
        <Button
          onClick={() => navigate(semanticsURL(source.id, "Query templates"))}
        >
          {t("Manage templates")}
        </Button>
      </div>
      <p className="help">
        {t(
          "Previews enforce current permissions and are labeled separately in audit. Complete a query in your actual MCP client to verify the connection from that client.",
        )}
      </p>
    </>
  );
  return (
    <>
      <Button onClick={() => navigate("/sources")}>
        {t("← Data sources")}
      </Button>
      <div className="page-header">
        <div>
          <h1>
            {workspaceReady ? t("Query workspace") : t("Agent setup")}{" "}
            <span className="semantic-source">/ {source.name}</span>
          </h1>
          <p>
            {workspaceReady
              ? t(
                  "Run a query with your Agent, explore templates, or review a business question.",
                )
              : t(
                  "Connect, prepare a safe query, and confirm a real client result.",
                )}
          </p>
        </div>
        <Button
          disabled={busy}
          onClick={async () => {
            await reload().catch((e) => setError(message(e)));
            refresh();
          }}
        >
          {workspaceReady ? t("Refresh workspace") : t("Refresh setup")}
        </Button>
      </div>
      <ErrorNote error={error || loadError} />
      {!data ? (
        loadError ? (
          <Button onClick={refresh}>{t("Retry")}</Button>
        ) : (
          <Loading />
        )
      ) : (
        <>
          <div className="workflow-summary">
            <strong>{readinessLabel(source, data)}</strong>
            <span>
              {data.executable_templates}
              {t(" executable templates · ")}
              {active.length} {t("active Agents · source publication ")}
              {data.published_version}
            </span>
            <details>
              <summary>{t("Snapshot and evidence")}</summary>
              <p className="help">
                {t("Configuration snapshot ")}
                {date(data.checked_at)}
                {t(". Connection and client evidence are historical checks.")}
              </p>
            </details>
          </div>
          <section
            className="workflow-identity"
            aria-label={t("Selected Agent context")}
          >
            <Field label={t("Agent for preview and connection")}>
              <select
                value={selectedAgent}
                onChange={(e) => setAgentID(e.target.value)}
              >
                {!active.length && (
                  <option value="">{t("No active authorized Agent")}</option>
                )}
                {active.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </Field>
            <div className="button-row">
              <Button
                disabled={!selectedAgent || busy}
                primary={!data.last_query}
                onClick={async () => {
                  setBusy(true);
                  try {
                    await reload();
                    navigate(
                      `/agents?connect=${encodeURIComponent(selectedAgent)}`,
                    );
                  } catch (e) {
                    setError(message(e));
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {t("Connect selected Agent")}
              </Button>
              {!selectedAgent && (
                <Button
                  primary
                  onClick={() =>
                    navigate(
                      `/agents?create=1&source_id=${encodeURIComponent(source.id)}`,
                    )
                  }
                >
                  {t("Create Agent for this source")}
                </Button>
              )}
              <Button
                onClick={() => navigate(`/sources/${source.id}/evaluation`)}
              >
                {t("Evaluate a business question")}
              </Button>
            </div>
          </section>
          {workspaceReady && (
            <section
              className="workspace-queries"
              aria-label={t("Queries for your Agent")}
            >
              {queryContent}
            </section>
          )}
          <ol className="setup-steps">
            <SetupStep
              key={`connection-${connectionReady}`}
              title={t("Connection and protection")}
              complete={connectionReady}
              detail={
                source.probe?.connected
                  ? t("Connected · last checked {time}", {
                      time: date(source.probe.checked_at),
                    })
                  : t("Test the connection to begin")
              }
            >
              <Protection probe={source.probe} detail />
              <p>
                {source.enabled
                  ? t("Source enabled.")
                  : t("Source disabled. Enable it in the connection settings.")}
              </p>
              <div className="button-row">
                <Button primary={!connectionReady} busy={busy} onClick={test}>
                  {t("Test connection")}
                </Button>
                <Button disabled={busy} onClick={() => setDialog("edit")}>
                  {t("Edit connection")}
                </Button>
              </div>
            </SetupStep>
            <SetupStep
              key={`grants-${!!selectedAgent}`}
              title={t("Agent access")}
              complete={!!selectedAgent}
              detail={
                selectedAgent
                  ? t("{count} active authorized Agents", {
                      count: active.length,
                    })
                  : t("Create an Agent and grant this source")
              }
            >
              <p>
                {t(
                  "Each Agent can access only its granted sources. Database accounts and views control the visible data.",
                )}
              </p>
              <div className="button-row">
                <Button
                  onClick={() =>
                    navigate(
                      `/agents?create=1&source_id=${encodeURIComponent(source.id)}`,
                    )
                  }
                >
                  {t("Create Agent for this source")}
                </Button>
                <Button onClick={() => navigate("/agents")}>
                  {t("Manage grants")}
                </Button>
              </div>
            </SetupStep>
            {!workspaceReady && <li>{queryContent}</li>}
            <SetupStep
              key={`client-${!!data.last_query}`}
              title={t("Real client activity")}
              complete={!!data.last_query}
              detail={
                data.last_query
                  ? t("Latest successful query: {time}", {
                      time: date(data.last_query),
                    })
                  : t("Waiting for the first successful client query")
              }
            >
              <p>
                {data.last_query
                  ? t(
                      "This is historical non-preview query evidence. It does not prove current connectivity or correctness of a business answer.",
                    )
                  : t(
                      "Open the connection guide above, configure your MCP client and complete a read-only query. Discovery alone does not complete this step.",
                    )}
              </p>
              <Button onClick={refresh}>{t("Check client activity")}</Button>
            </SetupStep>
          </ol>
          {data.ontology && (
            <div className="workflow-summary">
              <strong>
                {t("Business ontology · version ")}
                {data.ontology.version}
              </strong>
              <span>
                {data.ontology.entities.length}
                {t(" mapped entities ·")} {data.ontology.properties.length}
                {t(" mapped properties")}
              </span>
              <Button
                onClick={() =>
                  navigate(`/ontologies/${data.ontology!.ontology_id}`)
                }
              >
                {t("Explore business concepts")}
              </Button>
            </div>
          )}
        </>
      )}
      {dialog === "edit" && (
        <SourceEditor
          source={source}
          catalog={catalog}
          onClose={() => setDialog("")}
          onSaved={async (notice, close = true) => {
            await reload();
            refresh();
            if (close) setDialog("");
            if (notice) notify(notice);
          }}
        />
      )}
      {dialog === "native" && (
        <SourceDetails
          source={source}
          catalog={catalog}
          agents={agents}
          initialAgentID={selectedAgent}
          onClose={() => setDialog("")}
        />
      )}
      {template && (
        <TemplatePreview
          source={source}
          agents={agents}
          entry={template}
          initialAgentID={selectedAgent}
          onClose={() => setTemplate(null)}
        />
      )}
    </>
  );
}

function SetupStep({
  title,
  detail,
  complete,
  children,
}: {
  title: string;
  detail: string;
  complete: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(!complete);
  return (
    <li className="setup-step">
      <details open={open} onToggle={(e) => setOpen(e.currentTarget.open)}>
        <summary>
          <h2>{title}</h2>
          <span>
            {complete ? t("Recorded") : t("Action needed")} · {detail}
          </span>
        </summary>
        <div className="setup-step-content">{children}</div>
      </details>
    </li>
  );
}
