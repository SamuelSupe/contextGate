import { lazy, useState } from "react";
import { t, useLocale, setLocale, validLocale } from "./i18n";
import { Brand } from "./Brand";
import { api, message, payload, setCSRF } from "./api";
import { Button, CopyButton, ErrorNote, Field, Loading } from "./components";
import { canNavigate, useNavigationGuard } from "./useNavigationGuard";
import { PageBoundary } from "./PageBoundary";
import type { Administrator, Settings } from "./types";

const Administrators = lazy(() =>
  import("./Administrators").then((module) => ({ default: module.Administrators })),
);
const Diagnostics = lazy(() =>
  import("./Diagnostics").then((module) => ({ default: module.Diagnostics })),
);
const AuditExport = lazy(() =>
  import("./AuditExport").then((module) => ({ default: module.AuditExport })),
);
const ConfigurationMCP = lazy(() =>
  import("./ConfigurationMCP").then((module) => ({ default: module.ConfigurationMCP })),
);

export function SettingsPage({
  administrator,
  settings,
  notify,
}: {
  administrator: Administrator;
  settings: Settings | null;
  notify: (s: string) => void;
}) {
  const locale = useLocale();
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const superAdmin = administrator.role === "super_admin";
  const [section, setSection] = useState(() =>
    location.hash === "#configuration-mcp" ? "configuration" : "language",
  );
  const [visited, setVisited] = useState([section]);
  useNavigationGuard(!!current || !!password, busy);

  function selectSection(id: string) {
    if (id === section) return;
    if (!canNavigate()) {
      notify(t("Changes kept. Finish or cancel the current edit before leaving."));
      return;
    }
    setVisited((current) =>
      current.includes(id) ? current : [...current, id],
    );
    setSection(id);
  }
  if (!settings) return <Loading />;
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Settings")}</h1>
        </div>
      </div>
      <div className="settings-layout">
        <nav
          className="settings-navigation"
          aria-label={t("Settings sections")}
        >
          {[
            ["language", "Language"],
            ["service", "Service information"],
            ["configuration", "My configuration MCP"],
            ...(superAdmin ? [["administrators", "Administrators"]] : []),
            ["diagnostics", "Diagnostics"],
            ["audit", "Audit export"],
            ["security", "Administrator security"],
          ]
            .filter(
              ([id]) => superAdmin || !["diagnostics", "audit"].includes(id),
            )
            .map(([id, label]) => (
              <Button
                key={id}
                aria-pressed={section === id}
                aria-controls={"settings-" + id}
                onClick={() => selectSection(id)}
              >
                {t(label)}
              </Button>
            ))}
        </nav>
        <div className="settings-body">
          <section id="settings-language" hidden={section !== "language"}>
            <h2>{t("Language")}</h2>
            <Field
              label={t("Display language")}
              hint={t("Saved in this browser.")}
            >
              <select
                value={locale}
                onChange={(event) => setLocale(validLocale(event.target.value))}
              >
                <option value="en" lang="en">
                  {t("English")}
                </option>
                <option value="zh-CN" lang="zh-CN">
                  简体中文
                </option>
              </select>
            </Field>
          </section>
          <section id="settings-service" hidden={section !== "service"}>
            <h2>{t("Service information")}</h2>
            <div className="service-brand">
              <Brand tagline />
            </div>
            <dl className="settings-list">
              <div>
                <dt>{t("Service version")}</dt>
                <dd>
                  {settings.version}
                  <small className="block mono break-all">
                    {settings.commit}
                  </small>
                </dd>
              </div>
              <div>
                <dt>{t("MCP endpoint")}</dt>
                <dd>
                  <code>{settings.mcp_url}</code>
                  <CopyButton
                    text={settings.mcp_url}
                    onCopied={() => notify(t("MCP endpoint copied"))}
                  />
                </dd>
              </div>
              <div>
                <dt>{t("Database file directory")}</dt>
                <dd>
                  <code>{settings.database_directory}</code>
                </dd>
              </div>
              <div>
                <dt>{t("Audit retention")}</dt>
                <dd>
                  {settings.audit_retention_days}
                  {t(" days")}
                </dd>
              </div>
              <div>
                <dt>{t("Concurrency limits")}</dt>
                <dd>
                  {t("Global ")}
                  {settings.global_concurrency}
                  {t(" · Per Agent")} {settings.agent_concurrency}
                </dd>
              </div>
              <div>
                <dt>{t("OAuth lifetime")}</dt>
                <dd>
                  {t("Access token ")}
                  {settings.oauth_access_token_minutes}
                  {t(" minutes · Refresh grant ")}
                  {settings.oauth_refresh_token_days}
                  {t(" days")}
                </dd>
              </div>
            </dl>
            <p className="help">
              {t(
                "Set the public URL and file directory using startup options, then restart the service.",
              )}
            </p>
            <p className="help">
              <a
                href="https://github.com/SamuelSupe/contextGate/blob/main/docs/operations.md"
                target="_blank"
                rel="noreferrer"
              >
                {t("Deployment and recovery guide")}
              </a>
            </p>
          </section>
          <div id="settings-configuration" hidden={section !== "configuration"}>
            {visited.includes("configuration") && (
              <PageBoundary>
                <ConfigurationMCP notify={notify} />
              </PageBoundary>
            )}
          </div>
          {superAdmin && (
            <div
              id="settings-administrators"
              hidden={section !== "administrators"}
            >
              {visited.includes("administrators") && (
                <PageBoundary>
                  <Administrators current={administrator} notify={notify} />
                </PageBoundary>
              )}
            </div>
          )}
          {superAdmin && (
            <div id="settings-diagnostics" hidden={section !== "diagnostics"}>
              {visited.includes("diagnostics") && (
                <PageBoundary>
                  <Diagnostics />
                </PageBoundary>
              )}
            </div>
          )}
          {superAdmin && (
            <div id="settings-audit" hidden={section !== "audit"}>
              {visited.includes("audit") && (
                <PageBoundary>
                  <AuditExport active={section === "audit"} notify={notify} />
                </PageBoundary>
              )}
            </div>
          )}
          <section id="settings-security" hidden={section !== "security"}>
            <h2>{t("Change administrator password")}</h2>
            <p className="help">
              {t(
                "Changing your password signs out your other sessions. Your configuration token and query Agent grants remain valid.",
              )}
            </p>
            <ErrorNote error={error} />
            <form
              className="password-form"
              onSubmit={async (e) => {
                e.preventDefault();
                setBusy(true);
                setError("");
                try {
                  const s = await api<{ csrf: string }>("/api/password", {
                    method: "POST",
                    body: payload({ current_password: current, password }),
                  });
                  setCSRF(s.csrf);
                  setPassword("");
                  setCurrent("");
                  notify(t("Administrator password updated"));
                } catch (e) {
                  setError(message(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              <input
                type="text"
                name="username"
                autoComplete="username"
                value={administrator.username}
                readOnly
                className="sr-only"
                tabIndex={-1}
                aria-hidden="true"
              />
              <Field label={t("Current password")} required>
                <input
                  type="password"
                  disabled={busy}
                  autoComplete="current-password"
                  required
                  value={current}
                  onChange={(e) => setCurrent(e.target.value)}
                />
              </Field>
              <Field
                label={t("New password")}
                required
                hint={t("At least 12 characters")}
              >
                <input
                  type="password"
                  autoComplete="new-password"
                  disabled={busy}
                  required
                  minLength={12}
                  maxLength={256}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </Field>
              <Button primary busy={busy} type="submit">
                {t("Update password")}
              </Button>
            </form>
            <details className="advanced">
              <summary>{t("Administrator recovery")}</summary>
              <p>
                {t(
                  "Use the same MCPDBHUB_DATABASE_URL and master key as the service. Stop the service, provide a new password through standard input, then restart:",
                )}
              </p>
              <pre>
                contextgate reset-password --username USERNAME --data-dir DIR
                --password-stdin
              </pre>
              <p className="help">
                {t(
                  "Keep the existing master key. Recovery signs out the selected account and revokes its configuration token. Other accounts and query Agent grants remain valid.",
                )}
              </p>
            </details>
          </section>
        </div>
      </div>
    </>
  );
}
