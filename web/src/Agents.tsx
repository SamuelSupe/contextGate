import { useCallback, useState } from "react";
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
}: {
  agents: Agent[];
  sources: Source[];
  settings: Settings | null;
  reload: () => Promise<void>;
  notify: (s: string) => void;
}) {
  const [editing, setEditing] = useState<Agent | null | undefined>();
  const [oauth, setOAuth] = useState(false);
  const [connection, setConnection] = useState<Agent | null>(null);
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
      setError(`Refresh failed. ${message(e)}`);
    }
  }
  async function showCredential(result: { agent: Agent; token?: string }) {
    setEditing(undefined);
    if (result.token) {
      setToken(result.token);
      setConnection(result.agent);
    } else notify("Agent grants updated");
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
        a.enabled ? "Agent paused. Its token is retained." : "Agent resumed.",
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
          <h1>Agents</h1>
          <p>Credentials, data source grants and client connections</p>
        </div>
        <div className="button-row">
          <Button disabled={busy} onClick={() => void refresh()}>
            <RefreshCw size={15} />
            Refresh
          </Button>
          <Button onClick={() => setOAuth(true)}>OAuth clients</Button>
          <Button
            primary
            onClick={() => {
              setError("");
              setEditing(null);
            }}
          >
            <Plus size={17} />
            Create Agent
          </Button>
        </div>
      </div>
      <div className="connection-strip">
        <span>MCP endpoint</span>
        <code>{settings?.mcp_url}</code>
        <CopyButton text={settings?.mcp_url || ""} />
      </div>
      <ErrorNote error={error} />
      {!agents.length ? (
        <Empty
          title="No Agents yet"
          description="Create a token and grant data source access, or connect an OAuth client."
          action={
            <Button primary onClick={() => setEditing(null)}>
              Create Agent
            </Button>
          }
        />
      ) : (
        <div className="table-scroll">
          <table className="agents-table">
            <thead>
              <tr>
                <th>Agent</th>
                <th>Data source grants</th>
                <th>Credential</th>
                <th>Client activity · 30 days</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {agents.map((a) => {
                const expired = new Date(a.expires_at) <= new Date();
                const grants = a.sources || [];
                const usable = grants.filter((id) =>
                  sources.some((s) => s.id === id && s.enabled),
                ).length;
                return (
                  <tr key={a.id}>
                    <td>
                      <strong>{a.name}</strong>
                      <small className="block">
                        {a.auth_type === "oauth" ? "OAuth" : "Token"}
                      </small>
                    </td>
                    <td>
                      {grants
                        .map(
                          (id) =>
                            sources.find((s) => s.id === id)?.name ||
                            "Deleted data source",
                        )
                        .join(", ") || "No grants"}
                      <small className={usable ? "block" : "block amber"}>
                        {usable} enabled data sources
                      </small>
                    </td>
                    <td>
                      <span
                        className={`status ${a.revoked_at || !a.enabled || expired ? "muted" : "green"}`}
                      >
                        {a.revoked_at
                          ? "Permanently revoked"
                          : !a.enabled
                            ? "Paused"
                            : expired
                              ? "Expired"
                              : "Active"}
                      </span>
                      <small className="block">
                        Expires {date(a.expires_at)}
                      </small>
                    </td>
                    <td>
                      {a.activity?.last_call ? (
                        <>
                          <span
                            className={`status ${a.activity.error_code ? "amber" : "green"}`}
                          >
                            {a.activity.error_code
                              ? "Last call failed"
                              : "Last call succeeded"}
                          </span>
                          <small className="block">
                            {date(a.activity.last_call)}
                          </small>
                          <small className="block">
                            Last success: {date(a.activity.last_success)}
                          </small>
                        </>
                      ) : (
                        <span className="muted">No recorded calls</span>
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
                          Connect
                        </button>
                        <button
                          className="text-button"
                          onClick={() => setEditing(a)}
                        >
                          Edit grants
                        </button>
                        {!a.revoked_at ? (
                          <button
                            className="text-button"
                            disabled={busy || expired}
                            onClick={() => void pause(a)}
                          >
                            {a.enabled ? "Pause" : "Resume"}
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
                            {a.revoked_at ? "Issue new token" : "Rotate token"}
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
                            Revoke
                          </button>
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
          reload={reload}
          agent={editing}
          sources={sources}
          onClose={close}
          saved={showCredential}
        />
      ) : null}
      {connection ? (
        <AgentConnection
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
              ? "Permanently revoke credential"
              : "Issue a replacement token"
          }
          onClose={() => {
            if (!busy) setConfirm(null);
          }}
          footer={
            <>
              <Button disabled={busy} onClick={() => setConfirm(null)}>
                Cancel
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
                      notify("Credential permanently revoked.");
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
                  ? "Revoke permanently"
                  : "Generate replacement token"}
              </Button>
            </>
          }
        >
          <ErrorNote error={error} />
          <p>
            {confirm.action === "revoke"
              ? "The current credential will stop working permanently. Running queries will be cancelled. Pausing is available if you only need to suspend access temporarily."
              : "The previous token will stop working immediately and running queries will be cancelled. Save the replacement token and update your client. Agent identity, grants and audit history are retained."}
          </p>
          <strong>{confirm.agent.name}</strong>
        </Drawer>
      ) : null}
    </>
  );
}
