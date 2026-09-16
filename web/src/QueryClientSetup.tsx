import { useEffect, useRef, useState } from "react";
import { Button, CopyButton, ErrorNote, Field } from "./components";
import { api, date, message } from "./api";
import { waitForClientCall } from "./client-call";
import { t } from "./i18n";
import { HelpTip } from "./HelpTip";
import type { Readiness } from "./readiness";
import type { Agent } from "./types";
import type { SemanticEntry } from "./semantic-types";

export function QueryClientSetup({
  sourceID,
  entryID,
  agentID,
  activeAgents,
  selectedAgent,
  published,
  executable,
  confirmed,
  readiness,
  busy,
  active,
  selectAgent,
  connect,
  createAgent,
  preview,
  refresh,
  reload,
}: {
  sourceID: string;
  entryID: string;
  agentID: string;
  activeAgents: Agent[];
  selectedAgent?: Agent;
  published?: SemanticEntry;
  executable: boolean;
  confirmed: boolean;
  readiness: Readiness | null;
  busy: boolean;
  active: boolean;
  selectAgent: (id: string) => void;
  connect: (agent: Agent) => void;
  createAgent: () => void;
  preview: () => void;
  refresh: () => void;
  reload: () => void;
}) {
  const [checking, setChecking] = useState("idle");
  const [error, setError] = useState("");
  const refreshRef = useRef(refresh);
  refreshRef.current = refresh;
  const reloadRef = useRef(reload);
  reloadRef.current = reload;
  const version = published?.template?.execution_version || "";
  const hasAgent = !!selectedAgent;
  useEffect(() => {
    if (checking !== "waiting") return;
    if (!active || !hasAgent || !executable || document.hidden) {
      setChecking("stopped");
      return;
    }
    const abort = new AbortController();
    const hidden = () => {
      if (document.hidden) {
        abort.abort();
        setChecking("stopped");
      }
    };
    document.addEventListener("visibilitychange", hidden);
    const query = new URLSearchParams({
      template_id: entryID,
      agent_id: agentID,
    });
    void waitForClientCall(
      (signal) =>
        api<Readiness>(
          `/api/sources/${encodeURIComponent(sourceID)}/readiness?${query}`,
          { signal },
        ),
      { source: sourceID, template: entryID, version, agent: agentID },
      abort.signal,
    )
      .then((result) => {
        if (abort.signal.aborted) return;
        setChecking(result === "confirmed" ? "idle" : result);
        if (result === "confirmed") refreshRef.current();
        else if (result === "changed") reloadRef.current();
      })
      .catch((e) => {
        if (!abort.signal.aborted) {
          setError(message(e));
          setChecking("idle");
        }
      });
    return () => {
      abort.abort();
      document.removeEventListener("visibilitychange", hidden);
    };
  }, [
    checking,
    active,
    hasAgent,
    executable,
    sourceID,
    entryID,
    agentID,
    version,
  ]);
  const activity = readiness?.template_activity;
  // Preserve raw example JSON, including integers and Decimal values.
  const exampleCall = published
    ? `{\n  "name": "execute_query_template",\n  "arguments": {\n    "source_id": ${JSON.stringify(sourceID)},\n    "template_id": ${JSON.stringify(entryID)},\n    "execution_version": ${JSON.stringify(published.template!.execution_version)},\n    "parameters": ${published.template!.example_json}\n  }\n}`
    : "";
  return (
    <section className="query-client-step">
      <h3>{t("1. Connect an Agent")}</h3>
      <Field label={t("Agent")}>
        <select
          disabled={busy}
          value={agentID}
          onChange={(e) => {
            selectAgent(e.target.value);
          }}
        >
          <option value="">{t("Choose an Agent")}</option>
          {activeAgents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </select>
      </Field>
      {agentID && !selectedAgent && (
        <p className="notice warning">
          {t(
            "The selected Agent is no longer active or authorized for this source. Update its grants or choose another Agent.",
          )}
        </p>
      )}
      <div className="button-row">
        <Button
          disabled={busy || !selectedAgent}
          onClick={() => selectedAgent && connect(selectedAgent)}
        >
          {t("Get connection configuration")}
        </Button>
        {!activeAgents.length && (
          <Button disabled={busy} onClick={createAgent}>
            {t("Create Agent for this source")}
          </Button>
        )}
      </div>
      {activeAgents.length > 0 && (
        <details className="advanced">
          <summary>{t("More options")}</summary>
          <Button disabled={busy} onClick={createAgent}>
            {t("Create Agent for this source")}
          </Button>
        </details>
      )}
      {published && (
        <div className="query-client-action">
          <h3>{t("2. Call the query from your client")}</h3>
          <div className="button-row">
            <CopyButton text={exampleCall} label={t("Copy tool call")} />
            <Button
              disabled={busy || !selectedAgent || !executable}
              onClick={preview}
            >
              {t("Preview as Agent")}
            </Button>
          </div>
          <details className="advanced">
            <summary>{t("View tool call")}</summary>
            <pre>{exampleCall}</pre>
          </details>
        </div>
      )}
      <div className="query-client-action">
        <h3>{t("3. Confirm the client call")}</h3>
        <div className="button-row">
          {checking === "waiting" ? (
            <Button onClick={() => setChecking("stopped")}>
              {t("Stop waiting")}
            </Button>
          ) : (
            <Button
              primary={!confirmed}
              disabled={busy || !selectedAgent || !executable || confirmed}
              onClick={() => {
                setError("");
                setChecking("waiting");
              }}
            >
              {t(confirmed ? "Client call confirmed" : "Wait for client call")}
            </Button>
          )}
          <Button
            disabled={busy || !selectedAgent || checking === "waiting"}
            onClick={refresh}
          >
            {t("Check now")}
          </Button>
        </div>
        <p className="help" role="status">
          {checking === "waiting"
            ? t("Waiting for a matching call · up to 2 minutes")
            : checking === "timeout"
              ? t(
                  "No matching call within 2 minutes. Check your client and try again.",
                )
              : checking === "stopped"
                ? t("Waiting paused. Start again when ready.")
                : checking === "changed"
                  ? t(
                      "Query or access changed. Review the current setup before trying again.",
                    )
                  : ""}
        </p>
        <ErrorNote error={error} />
      </div>
      {selectedAgent && (
        <div className={`notice ${confirmed ? "success" : "neutral"}`}>
          <div>
            <div className="label-with-help">
              <strong>
                {confirmed
                  ? t("Client call observed for this template version")
                  : t("No matching successful client call confirmed")}
              </strong>
              <HelpTip title={t("Client call evidence")}>
                <p>
                  {t(
                    "Only successful calls by the selected Agent to this query's current execution version are counted. Previews are excluded.",
                  )}
                </p>
                <p>
                  {t(
                    "Client activity does not confirm business answer correctness.",
                  )}
                </p>
                {readiness && (
                  <p>
                    {t("Evidence window: {from} to {until}", {
                      from: date(readiness.activity_since),
                      until: date(readiness.checked_at),
                    })}
                  </p>
                )}
              </HelpTip>
            </div>
            {agentID &&
              activity?.agent_id === agentID &&
              activity.recent_calls.map((call) => (
                <p key={call.request_id || call.at}>
                  {date(call.at)} · <code>{call.request_id}</code>
                </p>
              ))}
          </div>
        </div>
      )}
    </section>
  );
}
