import { t } from "./i18n";
import { useCallback, useState, useEffect, useRef } from "react";
import { Plus, RefreshCw } from "lucide-react";
import { api, date, message, payload } from "./api";
import { Button, CopyButton, Drawer, Empty, ErrorNote } from "./components";
import { AgentEditor } from "./AgentEditor";
import { OAuthClient } from "./OAuthClient";
import { AgentConnection } from "./AgentConnection";
import type { Agent, Source, Settings } from "./types";

export function Agents({
  agents,
  sources,
  settings,
  reload,
  notify,
  navigate,
  initialQuery = "",
}: {
  agents: Agent[];
  sources: Source[];
  settings: Settings | null;
  reload: () => Promise<void>;
  notify: (s: string) => void;
  navigate: (url: string) => void;
  initialQuery?: string;
}) {
  const entry = new URLSearchParams(initialQuery);
  const attention = entry.get("attention") || "";
  const target = useRef<HTMLTableRowElement>(null);
  useEffect(() => {
    if (attention && target.current) {
      target.current.scrollIntoView({ block: "center" });
      target.current.focus({ preventScroll: true });
    }
  }, [attention]);
  const [editing, setEditing] = useState<Agent | null | undefined>(
    entry.get("create") === "1" ? null : undefined,
  );
  const [oauth, setOAuth] = useState(false);
  const [connection, setConnection] = useState<Agent | null>(
    agents.find((a) => a.id === entry.get("connect")) || null,
  );
  const [token, setToken] = useState("");
  const [confirm, setConfirm] = useState<{
    agent: Agent;
    action: "revoke" | "rotate";
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const close = useCallback(() => setEditing(undefined), []);
  async function refresh() {
    setError("");
    try {
      await reload();
    } catch (e) {
      setError(t("Refresh failed. {value1}", { value1: message(e) }));
    }
  }
  async function showCredential(result: { agent: Agent; token?: string }) {
    setEditing(undefined);
    if (result.token) {
      setToken(result.token);
      setConnection(result.agent);
    } else notify(t("Agent grants updated"));
    await refresh();
  }
  async function pause(a: Agent) {
    setBusy(true);
    setError("");
    try {
      await api(`/api/agents/${a.id}`, {
        method: "PUT",
        body: payload({
          revision: a.revision,
          name: a.name,
          sources: a.sources,
          expires_at: a.expires_at,
          enabled: !a.enabled,
        }),
      });
      await reload();
      notify(
        a.enabled
          ? t("Agent paused. Its token is retained.")
          : t("Agent resumed."),
      );
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Agents")}</h1>
          <p>{t("Credentials, data source grants and client connections")}</p>
        </div>
        <div className="button-row">
          <Button disabled={busy} onClick={() => void refresh()}>
            <RefreshCw size={15} />
            {t("Refresh")}
          </Button>
          <Button onClick={() => setOAuth(true)}>{t("OAuth clients")}</Button>
          <Button
            primary
            onClick={() => {
              setError("");
              setEditing(null);
            }}
          >
            <Plus size={17} />
            {t("Create Agent")}
          </Button>
        </div>
      </div>
      <div className="connection-strip">
        <span>{t("MCP endpoint")}</span>
        <code>{settings?.mcp_url}</code>
        <CopyButton text={settings?.mcp_url || ""} />
      </div>
      <ErrorNote error={error} />
      {!agents.length ? (
        <Empty
          title={t("No Agents yet")}
          description={t(
            "Create a token and grant data source access, or connect an OAuth client.",
          )}
          action={
            <Button primary onClick={() => setEditing(null)}>
              {t("Create Agent")}
            </Button>
          }
        />
      ) : (
        <div className="table-scroll">
          <table className="agents-table">
            <thead>
              <tr>
                <th>{t("Agent")}</th>
                <th>{t("Data source grants")}</th>
                <th>{t("Credential")}</th>
                <th>{t("Client activity · 30 days")}</th>
                <th>{t("Actions")}</th>
              </tr>
            </thead>
            <tbody>
              {agents.map((a) => {
                const expired = new Date(a.expires_at) <= new Date();
                const expiring =
                  !expired &&
                  new Date(a.expires_at).getTime() - Date.now() <= 7 * 86400000;
                const grants = a.sources || [];
                const usable = grants.filter((id) =>
                  sources.some((s) => s.id === id && s.enabled),
                ).length;
                return (
                  <tr
                    key={a.id}
                    ref={a.id === attention ? target : undefined}
                    data-route-focus={a.id === attention ? "" : undefined}
                    tabIndex={a.id === attention ? -1 : undefined}
                    className={a.id === attention ? "attention-row" : ""}
                  >
                    <td>
                      <strong>{a.name}</strong>
                      <small className="block">
                        {a.auth_type === "oauth" ? "OAuth" : t("Token")}
                      </small>
                    </td>
                    <td>
                      {grants
                        .map(
                          (id) =>
                            sources.find((s) => s.id === id)?.name ||
                            t("Deleted data source"),
                        )
                        .join(", ") || t("No grants")}
                      <small className={usable ? "block" : "block amber"}>
                        {usable}
                        {t(" enabled data sources")}
                      </small>
                    </td>
                    <td>
                      <span
                        className={`status ${a.revoked_at || !a.enabled || expired ? "muted" : expiring ? "amber" : "green"}`}
                      >
                        {a.revoked_at
                          ? t("Permanently revoked")
                          : !a.enabled
                            ? t("Paused")
                            : expired
                              ? t("Expired")
                              : expiring
                                ? t("Expiring soon")
                                : t("Active")}
                      </span>
                      <small className="block">
                        {t("Expires ")}
                        {date(a.expires_at)}
                      </small>
                    </td>
                    <td>
                      {a.activity?.last_call ? (
                        <>
                          <span
                            className={`status ${a.activity.error_code ? "amber" : "green"}`}
                          >
                            {a.activity.error_code
                              ? t("Last call failed")
                              : t("Last call succeeded")}
                          </span>
                          <small className="block">
                            {date(a.activity.last_call)}
                          </small>
                          <small className="block">
                            {t("Last success: ")}
                            {date(a.activity.last_success)}
                          </small>
                        </>
                      ) : (
                        <span className="muted">{t("No recorded calls")}</span>
                      )}
                    </td>
                    <td>
                      <div className="row-actions wrap">
                        <button
                          className="text-button"
                          onClick={() => {
                            setError("");
                            setToken("");
                            setConnection(a);
                          }}
                        >
                          {t("Connect")}
                        </button>
                        <button
                          className="text-button"
                          onClick={() => setEditing(a)}
                        >
                          {t("Edit grants")}
                        </button>
                        {!a.revoked_at || a.auth_type === "token" ? (
                          <details
                            className="agent-secondary-actions"
                            open={a.id === attention}
                          >
                            <summary
                              aria-label={t("More actions for {name}", {
                                name: a.name,
                              })}
                            >
                              {t("More")}
                            </summary>
                            <div className="button-row">
                              {!a.revoked_at ? (
                                <button
                                  className="text-button"
                                  disabled={busy || expired}
                                  onClick={() => void pause(a)}
                                >
                                  {a.enabled ? t("Pause") : t("Resume")}
                                </button>
                              ) : null}
                              {a.auth_type === "token" ? (
                                <button
                                  className="text-button"
                                  onClick={() => {
                                    setError("");
                                    setConfirm({ agent: a, action: "rotate" });
                                  }}
                                >
                                  {a.revoked_at
                                    ? t("Issue new token")
                                    : t("Rotate token")}
                                </button>
                              ) : null}
                              {!a.revoked_at ? (
                                <button
                                  className="text-button danger"
                                  onClick={() => {
                                    setError("");
                                    setConfirm({ agent: a, action: "revoke" });
                                  }}
                                >
                                  {t("Revoke")}
                                </button>
                              ) : null}
                            </div>
                          </details>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      {editing !== undefined ? (
        <AgentEditor
          initialSources={sources
            .filter((s) => s.id === entry.get("source_id"))
            .map((s) => s.id)}
          reload={reload}
          agent={editing}
          sources={sources}
          onClose={close}
          saved={showCredential}
        />
      ) : null}
      {connection ? (
        <AgentConnection
          sources={sources}
          navigate={navigate}
          agent={
            token
              ? {
                  ...connection,
                  activity: agents.find((a) => a.id === connection.id)
                    ?.activity,
                }
              : agents.find((a) => a.id === connection.id) || connection
          }
          endpoint={settings?.mcp_url || ""}
          token={token}
          error={error}
          refresh={refresh}
          onClose={() => {
            setConnection(null);
            setToken("");
          }}
        />
      ) : null}
      {oauth ? (
        <OAuthClient
          reloadAgents={reload}
          settings={settings}
          onClose={() => setOAuth(false)}
          notify={notify}
        />
      ) : null}
      {confirm ? (
        <Drawer
          title={
            confirm.action === "revoke"
              ? t("Permanently revoke credential")
              : t("Issue a replacement token")
          }
          onClose={() => {
            if (!busy) setConfirm(null);
          }}
          footer={
            <>
              <Button disabled={busy} onClick={() => setConfirm(null)}>
                {t("Cancel")}
              </Button>
              <Button
                primary
                busy={busy}
                onClick={async () => {
                  setBusy(true);
                  setError("");
                  try {
                    if (confirm.action === "revoke") {
                      await api(`/api/agents/${confirm.agent.id}`, {
                        method: "DELETE",
                      });
                      setConfirm(null);
                      await reload();
                      notify(t("Credential permanently revoked."));
                    } else {
                      const result = await api<{ agent: Agent; token: string }>(
                        `/api/agents/${confirm.agent.id}/token`,
                        { method: "POST" },
                      );
                      setConfirm(null);
                      await showCredential(result);
                    }
                  } catch (e) {
                    setError(message(e));
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {confirm.action === "revoke"
                  ? t("Revoke permanently")
                  : t("Generate replacement token")}
              </Button>
            </>
          }
        >
          <ErrorNote error={error} />
          <p>
            {confirm.action === "revoke"
              ? t(
                  "The current credential will stop working permanently. Running queries will be cancelled. Pausing is available if you only need to suspend access temporarily.",
                )
              : t(
                  "The previous token will stop working immediately and running queries will be cancelled. Save the replacement token and update your client. Agent identity, grants and audit history are retained.",
                )}
          </p>
          <strong>{confirm.agent.name}</strong>
        </Drawer>
      ) : null}
    </>
  );
}
