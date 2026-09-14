import { t } from "./i18n";
import { useState } from "react";
import { CopyButton, Field } from "./components";

const clients = {
  codex: {
    name: "Codex",
    location: "~/.codex/config.toml",
    docs: "https://developers.openai.com/codex/mcp",
    check:
      "Restart Codex, then use /mcp in the CLI to check that contextgate is connected.",
  },
  cursor: {
    name: "Cursor",
    location: "~/.cursor/mcp.json",
    docs: "https://cursor.com/docs/context/mcp",
    check: "Open Cursor Settings → Tools & MCP and enable contextgate.",
  },
  vscode: {
    name: "VS Code",
    location: "MCP: Open User Configuration",
    docs: "https://code.visualstudio.com/docs/agents/reference/mcp-configuration",
    check:
      "Run MCP: List Servers in the Command Palette, select contextgate, and start it. Use Show Output if it fails.",
  },
} as const;

export function ClientSetup({
  endpoint,
  oauth,
}: {
  endpoint: string;
  oauth: boolean;
}) {
  const [client, setClient] = useState<keyof typeof clients>("codex");
  const guide = clients[client];
  const headers = oauth
    ? {}
    : { headers: { Authorization: "Bearer ${env:MCPDBHUB_TOKEN}" } };
  const config =
    client === "codex"
      ? `[mcp_servers.contextgate]\nurl = ${JSON.stringify(endpoint)}${oauth ? "" : '\nbearer_token_env_var = "MCPDBHUB_TOKEN"'}`
      : JSON.stringify(
          client === "vscode"
            ? {
                servers: {
                  contextgate: { type: "http", url: endpoint, ...headers },
                },
              }
            : { mcpServers: { contextgate: { url: endpoint, ...headers } } },
          null,
          2,
        );
  return (
    <div className="client-setup">
      <Field label={t("MCP client")}>
        <select
          value={client}
          onChange={(e) => setClient(e.target.value as keyof typeof clients)}
        >
          {Object.entries(clients).map(([id, v]) => (
            <option key={id} value={id}>
              {v.name}
            </option>
          ))}
        </select>
      </Field>
      <p>
        {client === "vscode"
          ? t("Open the Command Palette and run ")
          : t("Open ")}
        <code>{guide.location}</code>
        {t(". Merge this server into your existing configuration.")}
      </p>
      <pre>{config}</pre>
      <CopyButton text={config} />
      {oauth ? (
        <p>
          {t(
            "Complete the browser sign-in and administrator consent when the client requests authorization. For Codex CLI, run",
          )}{" "}
          <code>codex mcp login contextgate</code>.
        </p>
      ) : (
        <p>
          {t("Set ")}
          <code>MCPDBHUB_TOKEN</code>
          {t(" to your saved Agent token in the environment that launches ")}
          {guide.name}
          {t(
            ", then restart the client. A terminal export only reaches apps started from that terminal.",
          )}
        </p>
      )}
      <p>{t(guide.check)}</p>
      <a href={guide.docs} target="_blank" rel="noreferrer">
        {guide.name}
        {t(" configuration reference ↗")}
      </a>
      <details>
        <summary>{t("Remote clients and troubleshooting")}</summary>
        <p>
          {t(
            "The endpoint must be reachable from the machine running the MCP client. A localhost address refers to that machine. Remote use requires the ContextGate's configured public HTTPS address.",
          )}
        </p>
        <p>
          {t(
            "For token authentication, confirm the client received MCPDBHUB_TOKEN and the Agent is active. For OAuth, reconnect through browser consent; authorization creates its own Agent identity and grants.",
          )}
        </p>
      </details>
    </div>
  );
}
