import { useState } from "react";
import { date } from "./api";
import { Button, CopyButton, Drawer, ErrorNote, Field } from "./components";
import type { Agent } from "./types";

export function AgentConnection({
  agent,
  endpoint,
  token,
  error,
  refresh,
  onClose,
}: {
  agent: Agent;
  endpoint: string;
  token: string;
  error: string;
  refresh: () => Promise<void>;
  onClose: () => void;
}) {
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const credential = token || "YOUR_AGENT_TOKEN";
  const http = JSON.stringify(
    {
      mcpServers: {
        mcpdbhub: {
          url: endpoint,
          headers: { Authorization: `Bearer ${credential}` },
        },
      },
    },
    null,
    2,
  );
  const stdio = JSON.stringify(
    {
      mcpServers: {
        mcpdbhub: {
          command: "/absolute/path/to/mcpdbhub",
          args: ["stdio", "--url", endpoint],
          env: { MCPDBHUB_TOKEN: credential },
        },
      },
    },
    null,
    2,
  );
  function close() {
    if (token && !saved) {
      if (
        !window.confirm(
          "This token cannot be viewed again. Close without confirming it was saved?",
        )
      )
        return;
    }
    onClose();
  }
  return (
    <Drawer
      title={token ? "Save your Agent token" : `Connect ${agent.name}`}
      subtitle="Configure your client, run a query, then check the recorded result"
      wide
      onClose={close}
      footer={
        <Button
          primary
          onClick={() => {
            if (token) setSaved(true);
            onClose();
          }}
        >
          {token ? "I saved the token — close" : "Close"}
        </Button>
      }
    >
      <ErrorNote error={error} />
      {token ? (
        <>
          <div className="notice warning">
            This token is shown only once. Save it before closing. It remains
            visible if a background refresh fails.
          </div>
          <Field label="Access token">
            <textarea readOnly rows={2} value={token} />
          </Field>
          <CopyButton text={token} />
          <label className="check-row">
            <input
              type="checkbox"
              checked={saved}
              onChange={(e) => setSaved(e.target.checked)}
            />
            I have saved this token securely
          </label>
        </>
      ) : null}
      <ol className="connection-steps">
        <li>
          <strong>Configure your MCP client</strong>
          {agent.auth_type === "oauth" ? (
            <p>
              Enter the MCP endpoint in your OAuth-enabled client. Complete
              administrator consent and select the required data sources.
            </p>
          ) : (
            <p>
              Use your saved Agent token. If you lost it, choose Rotate token in
              Agents. The placeholder below is not a working credential.
            </p>
          )}
          <div className="connection-strip">
            <code>{endpoint}</code>
            <CopyButton text={endpoint} />
          </div>
          {agent.auth_type !== "oauth" ? (
            <>
              <details open>
                <summary>HTTP configuration</summary>
                <p className="help">
                  Use these connection values in your client's MCP
                  configuration. The outer JSON format may vary by client.
                </p>
                <pre>{http}</pre>
                <CopyButton text={http} />
              </details>
              <details>
                <summary>stdio bridge configuration</summary>
                <p className="help">
                  Install the Hub executable on the client machine and replace
                  the absolute path.
                </p>
                <pre>{stdio}</pre>
                <CopyButton text={stdio} />
              </details>
            </>
          ) : null}
        </li>
        <li>
          <strong>Run a read-only query from the client</strong>
          <p>
            Call <code>list_data_sources</code>, select an authorized source,
            then use its advertised query tool and example. Administrator
            previews do not verify client connectivity.
          </p>
        </li>
        <li>
          <strong>Check the recorded client activity</strong>
          <p>
            {agent.activity?.last_call
              ? `Last call: ${date(agent.activity.last_call)}. ${agent.activity.error_code ? `Failed (${agent.activity.error_code}). Open Audit log for details.` : "Succeeded."}`
              : "Waiting for a query or structure discovery call from this client. Activity is retained for 30 days."}
          </p>
          <p className="help">
            Last successful call: {date(agent.activity?.last_success)}
          </p>
          <Button
            busy={busy}
            onClick={async () => {
              setBusy(true);
              try {
                await refresh();
              } finally {
                setBusy(false);
              }
            }}
          >
            Refresh client activity
          </Button>
        </li>
      </ol>
      {agent.revoked_at || !agent.enabled ? (
        <div className="notice warning">
          {agent.revoked_at
            ? "This credential is permanently revoked. Issue a new token or authorize OAuth again."
            : "This Agent is paused. Resume it before testing client access."}
        </div>
      ) : null}
    </Drawer>
  );
}
