import { t } from "./i18n";
import { useState } from "react";
import { date } from "./api";
import { Button, CopyButton, Drawer, ErrorNote, Field } from "./components";
import { semanticsURL } from "./readiness";
import type { Agent, Source } from "./types";
import { ClientSetup } from "./ClientSetup";

export function AgentConnection({
  agent,
  sources,
  navigate,
  endpoint,
  token,
  error,
  refresh,
  onClose,
}: {
  agent: Agent;
  sources: Source[];
  navigate: (url: string) => void;
  endpoint: string;
  token: string;
  error: string;
  refresh: () => Promise<void>;
  onClose: () => void;
}) {
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const credential = token || "YOUR_AGENT_TOKEN";
  const stdio = JSON.stringify(
    {
      mcpServers: {
        contextgate: {
          command: "/absolute/path/to/contextgate",
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
          t(
            "This token cannot be viewed again. Close without confirming it was saved?",
          ),
        )
      )
        return;
    }
    onClose();
  }
  return (
    <Drawer
      title={
        token
          ? t("Save your Agent token")
          : t("Connect {name}", { name: agent.name })
      }
      subtitle={t(
        "Configure your client, run a query, then check the recorded result",
      )}
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
          {token ? t("I saved the token — close") : t("Close")}
        </Button>
      }
    >
      <ErrorNote error={error} />
      {token ? (
        <>
          <div className="notice warning">
            {t(
              "This token is shown only once. Save it before closing. It remains visible if a background refresh fails.",
            )}
          </div>
          <Field label={t("Access token")}>
            <textarea readOnly rows={2} value={token} />
          </Field>
          <CopyButton text={token} />
          <label className="check-row">
            <input
              type="checkbox"
              checked={saved}
              onChange={(e) => setSaved(e.target.checked)}
            />
            {t("I have saved this token securely")}
          </label>
        </>
      ) : null}
      <ol className="connection-steps">
        <li>
          <strong>{t("Configure your MCP client")}</strong>
          {agent.auth_type === "oauth" ? (
            <p>
              {t(
                "Enter the MCP endpoint in your OAuth-enabled client. Complete administrator consent and select the required data sources.",
              )}
            </p>
          ) : (
            <p>
              {t(
                "Use your saved Agent token. If you lost it, choose Rotate token in Agents.",
              )}
            </p>
          )}
          <div className="connection-strip">
            <code>{endpoint}</code>
            <CopyButton text={endpoint} />
          </div>
          <ClientSetup
            endpoint={endpoint}
            oauth={agent.auth_type === "oauth"}
          />
          {agent.auth_type !== "oauth" ? (
            <>
              <details>
                <summary>{t("Advanced: stdio bridge")}</summary>
                <p className="help">
                  {t(
                    "Install the ContextGate executable on the client machine and replace the absolute path.",
                  )}
                </p>
                <pre>{stdio}</pre>
                <CopyButton text={stdio} />
              </details>
            </>
          ) : null}
        </li>
        <li>
          <strong>{t("Run a read-only query from the client")}</strong>
          <p>
            {t("Call ")}
            <code>list_data_sources</code>
            {t(
              ", select an authorized source, then use its advertised query tool and example. Administrator previews do not verify client connectivity.",
            )}
          </p>
        </li>
        <li>
          <strong>{t("Check the recorded client activity")}</strong>
          <p>
            {agent.activity?.last_call
              ? t("Last call: {value1}. {value2}", {
                  value1: date(agent.activity.last_call),
                  value2: agent.activity.error_code
                    ? `Failed (${agent.activity.error_code}). Open Audit log for details.`
                    : t("Succeeded."),
                })
              : t(
                  "Waiting for a query or structure discovery call from this client. Activity is retained for 30 days.",
                )}
          </p>
          <p className="help">
            {t("Last successful call: ")}
            {date(agent.activity?.last_success)}
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
            {t("Refresh client activity")}
          </Button>
        </li>
      </ol>
      {agent.activity?.error_code && (
        <div className="notice warning">
          <div>
            <strong>{t("Recommended next step")}</strong>
            <p>
              {agent.activity.error_code === "templates_only"
                ? t(
                    "This source accepts templates only. Discover the published catalog with search_semantics and run execute_query_template using its current execution version.",
                  )
                : ["template_changed", "template_unverified"].includes(
                      agent.activity.error_code,
                    )
                  ? t(
                      "Refresh the semantic catalog. An administrator may need to trial and publish the template again.",
                    )
                  : ["forbidden", "unauthorized"].includes(
                        agent.activity.error_code,
                      )
                    ? t(
                        "Check this Agent’s grants, expiration and credential state.",
                      )
                    : ["timeout", "result_too_large"].includes(
                          agent.activity.error_code,
                        )
                      ? t(
                          "Narrow the query, reduce result size or use the supported pagination before retrying.",
                        )
                      : t(
                          "Open Agent setup for the affected source to review connection, query mode, templates and grants.",
                        )}
            </p>
          </div>
        </div>
      )}
      <div className="button-row">
        {sources
          .filter((s) => agent.sources.includes(s.id))
          .map((s) => (
            <Button
              key={s.id}
              disabled={!!token && !saved}
              onClick={() =>
                navigate(
                  agent.activity?.error_code === "templates_only"
                    ? semanticsURL(s.id, "Query templates")
                    : `/sources/${s.id}/setup`,
                )
              }
            >
              {agent.activity?.error_code === "templates_only"
                ? t("Templates")
                : t("Agent setup")}{" "}
              · {s.name}
            </Button>
          ))}
      </div>
      {agent.revoked_at || !agent.enabled ? (
        <div className="notice warning">
          {agent.revoked_at
            ? t(
                "This credential is permanently revoked. Issue a new token or authorize OAuth again.",
              )
            : t(
                "This Agent is paused. Resume it before testing client access.",
              )}
        </div>
      ) : null}
    </Drawer>
  );
}
