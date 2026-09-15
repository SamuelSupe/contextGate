import { useEffect, useRef, useState } from "react";
import { Link } from "lucide-react";
import { api, date, message, payload } from "./api";
import {
  Button,
  CopyButton,
  Drawer,
  ErrorNote,
  Field,
  Loading,
} from "./components";
import { t } from "./i18n";
import { useNavigationGuard } from "./useNavigationGuard";

type ConfigurationAgent = {
  revision: string;
  id: string;
  name: string;
  created_at: string;
  expires_at: string;
  revoked_at?: string;
};
type ConfigurationAccess = {
  agents: ConfigurationAgent[];
  endpoint: string;
  publication: "administrator_only";
};

export function ConfigurationMCP({
  notify,
}: {
  notify: (text: string) => void;
}) {
  const [access, setAccess] = useState<ConfigurationAccess | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [panel, setPanel] = useState(false);
  const [name, setName] = useState("");
  const [hours, setHours] = useState("24");
  const [created, setCreated] = useState<{
    agent: ConfigurationAgent;
    token: string;
  } | null>(null);
  const [revoke, setRevoke] = useState<ConfigurationAgent | null>(null);
  const [transport, setTransport] = useState("http");
  const [saved, setSaved] = useState(false);
  const [panelError, setPanelError] = useState("");
  const firstField = useRef<HTMLInputElement>(null);
  useNavigationGuard(!!created && !saved, busy);
  useEffect(() => {
    if (panel) firstField.current?.focus();
  }, [panel, created]);

  async function load() {
    setError("");
    try {
      setAccess(await api<ConfigurationAccess>("/api/configuration-agents"));
    } catch (e) {
      setError(message(e));
    }
  }
  useEffect(() => {
    void load();
    if (window.location.hash === "#configuration-mcp") {
      document
        .getElementById("configuration-mcp")
        ?.scrollIntoView({ block: "start" });
    }
  }, []);

  const currentIdentity = access?.agents[0];
  const activeIdentity =
    !!currentIdentity &&
    !currentIdentity.revoked_at &&
    Date.parse(currentIdentity.expires_at) > Date.now();
  const issueLabel = t(activeIdentity ? "Rotate my token" : "Issue my token");
  const token = created?.token || "YOUR_CONFIGURATION_TOKEN";
  const endpoint = access?.endpoint || "";
  const config = JSON.stringify(
    {
      mcpServers: {
        "contextgate-configuration":
          transport === "http"
            ? { url: endpoint, headers: { Authorization: `Bearer ${token}` } }
            : {
                command: "contextgate",
                args: ["stdio", "--url", endpoint],
                env: { MCPDBHUB_TOKEN: token },
              },
      },
    },
    null,
    2,
  );
  const prompt = t(
    "Use ContextGate Configuration MCP to configure my data source, semantic catalog, query templates and ontology mapping. Start with get_configuration_guide and inspect existing configuration. Ask me for missing read-only credentials and business definitions. Preserve unrelated configuration. Validate the ontology and ask the administrator to publish it before mapping; then check mappings and trial enabled templates. Return a change summary, current revisions and review links for administrator publication.",
  );

  function close() {
    if (busy) return;
    if (created && !saved) {
      setPanelError(
        t("Save the token or configuration, then confirm before closing."),
      );
      return;
    }
    setPanel(false);
    setCreated(null);
    setPanelError("");
  }

  return (
    <section id="configuration-mcp" className="configuration-mcp">
      <div className="configuration-heading">
        <div>
          <h2>{t("My configuration MCP")}</h2>
          <p className="help">
            {t(
              "Let a trusted Agent prepare data sources, semantics and ontologies.",
            )}
          </p>
        </div>
        <Button
          primary
          disabled={!access}
          onClick={() => {
            setName(access?.agents[0]?.name || "");
            setCreated(null);
            setSaved(false);
            setPanelError("");
            setPanel(true);
          }}
        >
          {issueLabel}
        </Button>
      </div>
      <p>
        {t(
          "Configuration Agents can change all data source connections immediately, edit semantic and ontology drafts, and run read-only template trials. Administrators publish drafts and manage query Agent access.",
        )}
      </p>
      <p className="help">
        {t(
          "Your fixed configuration identity is linked to your administrator account. One token is active at a time; rotation invalidates the previous token immediately.",
        )}
      </p>
      <div className="configuration-endpoint">
        <code>{endpoint || "…"}</code>
        {endpoint && <CopyButton text={endpoint} />}
      </div>
      <ErrorNote error={error} />
      {error && <Button onClick={load}>{t("Retry")}</Button>}
      {!access && !error ? (
        <Loading />
      ) : access && !access.agents.length ? (
        <p className="help">
          {t(
            "No configuration Agents yet. Create a short-lived token to get started.",
          )}
        </p>
      ) : null}
      {access && access.agents.length > 0 && (
        <div
          className="configuration-agents"
          aria-label={t("Configuration Agents")}
        >
          {access.agents.map((a) => {
            const active =
              !a.revoked_at && new Date(a.expires_at).getTime() > Date.now();
            return (
              <div className="configuration-agent" key={a.id}>
                <div>
                  <strong>{a.name}</strong>
                  <small
                    className="block"
                    title={t("Configuration identity ID")}
                  >
                    <code>{a.id}</code>
                  </small>
                  <small className="block">
                    {Date.parse(a.expires_at) > 0 ? (
                      <>
                        {t("Expires")} {date(a.expires_at)}
                      </>
                    ) : (
                      t("Token not issued")
                    )}
                  </small>
                </div>
                <span className={`status ${active ? "green" : "muted"}`}>
                  {a.revoked_at
                    ? t("Revoked")
                    : active
                      ? t("Active")
                      : Date.parse(a.expires_at) > 0
                        ? t("Expired")
                        : t("Token not issued")}
                </span>
                {active && (
                  <Button
                    onClick={() => {
                      setRevoke(a);
                      setPanelError("");
                    }}
                  >
                    {t("Revoke")}
                  </Button>
                )}
              </div>
            );
          })}
        </div>
      )}
      <details className="advanced">
        <summary>
          <Link size={14} /> {t("Connection guide and starter prompt")}
        </summary>
        <p>
          {t(
            "The client must support Streamable HTTP or stdio. Use the configured public HTTPS address for remote clients; localhost refers to the client machine.",
          )}
        </p>
        <Field label={t("Transport")}>
          <select
            value={transport}
            onChange={(e) => setTransport(e.target.value)}
          >
            <option value="http">Streamable HTTP</option>
            <option value="stdio">stdio</option>
          </select>
        </Field>
        <pre>{config.replaceAll(token, "YOUR_CONFIGURATION_TOKEN")}</pre>
        <CopyButton
          text={config.replaceAll(token, "YOUR_CONFIGURATION_TOKEN")}
        />
        <p className="help">
          {t(
            "Merge this server into your client's MCP configuration and replace YOUR_CONFIGURATION_TOKEN with the saved token. For stdio, install the ContextGate binary on the client machine.",
          )}
        </p>
        <p>{prompt}</p>
        <CopyButton text={prompt} />
      </details>
      {panel && (
        <Drawer
          wide
          title={created ? t("Connect your configuration Agent") : issueLabel}
          onClose={close}
          subtitle={
            created
              ? t("This token is shown only once. Save it before closing.")
              : t("All data sources · Semantic drafts · Ontology drafts")
          }
          footer={
            <>
              <Button onClick={close} disabled={busy || (!!created && !saved)}>
                {created ? t("Done") : t("Cancel")}
              </Button>
              {!created && (
                <Button
                  primary
                  busy={busy}
                  type="submit"
                  form="configuration-access-form"
                >
                  {issueLabel}
                </Button>
              )}
            </>
          }
        >
          <ErrorNote error={panelError} />
          {created ? (
            <>
              <Field label={t("Configuration token")}>
                <input
                  type="password"
                  ref={firstField}
                  readOnly
                  value={created.token}
                  autoComplete="off"
                />
              </Field>
              <CopyButton
                text={created.token}
                onCopied={() => {
                  setPanelError("");
                  setSaved(true);
                  notify(t("Configuration token copied"));
                }}
              />
              <p className="help">
                {t("Expires")} {date(created.agent.expires_at)}
              </p>
              <Field label={t("Transport")}>
                <select
                  value={transport}
                  onChange={(e) => setTransport(e.target.value)}
                >
                  <option value="http">Streamable HTTP</option>
                  <option value="stdio">stdio</option>
                </select>
              </Field>
              <pre>
                {config.replaceAll(created.token, "YOUR_CONFIGURATION_TOKEN")}
              </pre>
              <CopyButton
                text={config}
                onCopied={() => {
                  setPanelError("");
                  setSaved(true);
                  notify(t("MCP configuration copied"));
                }}
              />
              <p className="help">
                {t(
                  "The copied configuration includes your new token. Keep it private. For stdio, install the ContextGate binary on the client machine.",
                )}
              </p>
              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={saved}
                  onChange={(e) => setSaved(e.target.checked)}
                />
                <span>{t("I have saved the token or configuration.")}</span>
              </label>
              <h3>{t("Starter prompt")}</h3>
              <p>{prompt}</p>
              <CopyButton text={prompt} />
            </>
          ) : (
            <form
              id="configuration-access-form"
              onSubmit={async (e) => {
                e.preventDefault();
                setBusy(true);
                setPanelError("");
                try {
                  const result = await api<{
                    agent: ConfigurationAgent;
                    token: string;
                  }>(
                    access?.agents[0]
                      ? `/api/configuration-agents/${access.agents[0].id}/token`
                      : "/api/configuration-agents",
                    {
                      method: "POST",
                      body: payload({
                        name: name.trim(),
                        revision: access?.agents[0]?.revision,
                        expires_at: new Date(
                          Date.now() + Number(hours) * 3600000,
                        ).toISOString(),
                      }),
                    },
                  );
                  setCreated(result);
                  setSaved(false);
                  await load();
                } catch (e) {
                  setPanelError(message(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              <p className="help">
                {t(
                  "Issuing a token replaces any current token for this identity. Update every client that uses it.",
                )}
              </p>
              <Field label={t("Identity label")} required>
                <input
                  ref={firstField}
                  required
                  maxLength={120}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t("For example, Retail setup assistant")}
                />
              </Field>
              <Field label={t("Token lifetime")}>
                <select
                  value={hours}
                  onChange={(e) => setHours(e.target.value)}
                >
                  <option value="1">{t("1 hour")}</option>
                  <option value="24">{t("24 hours (recommended)")}</option>
                  <option value="168">{t("7 days")}</option>
                  <option value="720">{t("30 days")}</option>
                </select>
              </Field>
              <div className="notice warning">
                <div>
                  <strong>
                    {t("Administrator-level configuration access")}
                  </strong>
                  <p>
                    {t(
                      "This token can change connections and credentials for every data source, including sources used by other Agents. Connection changes take effect immediately and can interrupt queries. Semantic and ontology publication still requires an administrator.",
                    )}
                  </p>
                </div>
              </div>
            </form>
          )}
        </Drawer>
      )}
      {revoke && (
        <Drawer
          wide
          title={t("Revoke configuration access")}
          onClose={() => {
            if (!busy) setRevoke(null);
          }}
          footer={
            <>
              <Button disabled={busy} onClick={() => setRevoke(null)}>
                {t("Cancel")}
              </Button>
              <Button
                primary
                busy={busy}
                onClick={async () => {
                  setBusy(true);
                  setPanelError("");
                  try {
                    await api(
                      `/api/configuration-agents/${encodeURIComponent(revoke.id)}`,
                      { method: "DELETE" },
                    );
                    setRevoke(null);
                    await load();
                    notify(t("Configuration access revoked"));
                  } catch (e) {
                    setPanelError(message(e));
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {t("Revoke")}
              </Button>
            </>
          }
        >
          <ErrorNote error={panelError} />
          <p>
            {t(
              "Revoke access for {name}? Active configuration calls and trials will be cancelled. Saved configuration and drafts remain.",
              { name: revoke.name },
            )}
          </p>
        </Drawer>
      )}
    </section>
  );
}
