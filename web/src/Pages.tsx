import { t, useLocale, setLocale, validLocale } from "./i18n";
import { Diagnostics } from "./Diagnostics";
import { Brand } from "./Brand";
import { AuditExport } from "./AuditExport";
import { ConfigurationMCP } from "./ConfigurationMCP";
import { capabilityLabel } from "./display";
import { useEffect, useState } from "react";
import { ShieldCheck, CheckCircle2, Search } from "lucide-react";
import { api, message, payload, setCSRF } from "./api";
import {
  Button,
  CopyButton,
  Empty,
  ErrorNote,
  Field,
  Loading,
} from "./components";
import type { Capability, Settings, Source } from "./types";
export function CatalogPage({ catalog }: { catalog: Capability[] }) {
  const [search, setSearch] = useState("");
  const shown = catalog.filter((c) =>
    (c.name + " " + c.kind).toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Data source types")}</h1>
          <p>
            {t("Native query capabilities, protection and tested versions")}
          </p>
        </div>
      </div>
      <div className="filters">
        <div className="search-input">
          <Search size={16} />
          <input
            placeholder={t("Search data source types")}
            aria-label={t("Search data source types")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <span className="help">
          {shown.length}
          {t(" of ")}
          {catalog.length}
          {t(" connectors")}
        </span>
      </div>
      {!shown.length ? (
        <Empty
          title={t("No matching connectors")}
          description={t("Try another name or clear the search.")}
          action={
            <Button onClick={() => setSearch("")}>{t("Clear search")}</Button>
          }
        />
      ) : (
        <div className="table-scroll">
          <table className="catalog-table">
            <thead>
              <tr>
                <th>{t("Database")}</th>
                <th>{t("Agent query tool")}</th>
                <th>{t("Pagination")}</th>
                <th>{t("Tested versions")}</th>
                <th>{t("Protection and limitations")}</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((c) => (
                <tr key={c.kind}>
                  <td className="semibold">{c.name}</td>
                  <td>
                    <code>{c.tool}</code>
                    <small className="block">
                      {c.parameters
                        ? t("Parameter binding")
                        : t("Native parameters")}
                    </small>
                  </td>
                  <td>{capabilityLabel(c.pagination)}</td>
                  <td>
                    {c.verified_versions.length ? (
                      <span className="status green">
                        <CheckCircle2 size={14} />
                        {c.verified_versions.join(", ")}
                      </span>
                    ) : (
                      <span className="muted">{t("Not verified")}</span>
                    )}
                  </td>
                  <td>
                    <span>{capabilityLabel(c.protection)}</span>
                    {c.limitations.map((l) => (
                      <small className="block" key={l}>
                        {capabilityLabel(l)}
                      </small>
                    ))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="notice neutral">
        <ShieldCheck size={19} />
        <p>
          {t(
            "Compatible products are tested individually. Engine protection and account permissions are separate checks. InfluxDB 3 Core uses query API isolation.",
          )}
        </p>
      </div>
    </>
  );
}
export function SettingsPage({
  settings,
  notify,
}: {
  settings: Settings | null;
  notify: (s: string) => void;
}) {
  const locale = useLocale();
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  if (!settings) return <Loading />;
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Settings")}</h1>
          <p>{t("Service connection and administrator security settings")}</p>
        </div>
      </div>
      <nav className="settings-navigation" aria-label={t("Settings sections")}>
        {[
          ["language", "Language"],
          ["service", "Service information"],
          ["configuration", "Configuration MCP"],
          ["diagnostics", "Diagnostics"],
          ["audit", "Audit export"],
          ["security", "Administrator security"],
        ].map(([id, label]) => (
          <Button
            key={id}
            onClick={() => {
              const section = document.getElementById("settings-" + id);
              section?.scrollIntoView({ block: "start" });
              section?.focus({ preventScroll: true });
            }}
          >
            {t(label)}
          </Button>
        ))}
      </nav>
      <div className="settings-body">
        <section id="settings-language" tabIndex={-1}>
          <h2>{t("Language")}</h2>
          <Field
            label={t("Display language")}
            hint={t(
              "Applies immediately and is remembered in this browser. Business content and query results keep their original language.",
            )}
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
        <section id="settings-service" tabIndex={-1}>
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
        </section>
        <p className="help">
          <a
            href="https://github.com/SamuelSupe/contextGate/blob/main/docs/operations.md"
            target="_blank"
            rel="noreferrer"
          >
            {t("Deployment and recovery guide")}
          </a>
        </p>
        <div id="settings-configuration" tabIndex={-1}>
          <ConfigurationMCP notify={notify} />
        </div>
        <div id="settings-diagnostics" tabIndex={-1}>
          <Diagnostics />
        </div>
        <div id="settings-audit" tabIndex={-1}>
          <AuditExport notify={notify} />
        </div>
        <section id="settings-security" tabIndex={-1}>
          <h2>{t("Change administrator password")}</h2>
          <p className="help">
            {t(
              "Changing the password signs out other administrator sessions. Agent grants remain valid.",
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
              value="administrator"
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
        </section>
        <details className="advanced">
          <summary>{t("Administrator recovery")}</summary>
          <p>
            {t(
              "Use the same MCPDBHUB_DATABASE_URL and master key as the service. Stop the service, provide a new password through standard input, then restart:",
            )}
          </p>
          <pre>contextgate reset-password --data-dir DIR --password-stdin</pre>
          <p className="help">
            {t(
              "Keep the existing master key. Recovery signs out all administrator sessions and retains data sources, Agent credentials and audit history.",
            )}
          </p>
        </details>
      </div>
    </>
  );
}
interface Consent {
  client_name: string;
  client_id: string;
  redirect_uri: string;
  scope: string;
  sources: Pick<Source, "id" | "name" | "kind">[];
}
export function ConsentPage({ notify }: { notify: (s: string) => void }) {
  const [info, setInfo] = useState<Consent | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const request = new URLSearchParams(location.search).get("request") || "";
  useEffect(() => {
    api<Consent>(`/api/oauth/consent?request=${encodeURIComponent(request)}`)
      .then(setInfo)
      .catch((e) => setError(message(e)));
  }, [request]);
  async function consent(allow: boolean) {
    setBusy(true);
    setError("");
    try {
      const out = await api<{ redirect: string }>("/api/oauth/consent", {
        method: "POST",
        body: payload({ request, allow, sources: selected }),
      });
      notify(allow ? t("Access granted") : t("Access denied"));
      location.assign(out.redirect);
    } catch (e) {
      setError(message(e));
      setBusy(false);
    }
  }
  return (
    <div className="auth">
      <div className="auth-brand">
        <Brand tagline />
      </div>
      <section className="auth-panel consent-panel">
        <h1>{t("Authorize read-only database access")}</h1>
        <ErrorNote error={error} />
        {!info && error ? (
          <a href="/sources">{t("Return to administration")}</a>
        ) : null}
        {!info && !error ? (
          <Loading />
        ) : info ? (
          <>
            <p>
              <strong>{info.client_name}</strong>
              {t(" requests database access through MCP.")}
            </p>
            <dl className="consent-details">
              <dt>Client ID</dt>
              <dd>{info.client_id}</dd>
              <dt>{t("Return URL")}</dt>
              <dd>{info.redirect_uri}</dd>
            </dl>
            <h3>{t("Allowed data sources")}</h3>
            <div className="source-checklist">
              {info.sources.length ? (
                info.sources.map((s) => (
                  <label key={s.id} className="check-row">
                    <input
                      type="checkbox"
                      disabled={busy}
                      checked={selected.includes(s.id)}
                      onChange={(e) =>
                        setSelected(
                          e.target.checked
                            ? [...selected, s.id]
                            : selected.filter((id) => id !== s.id),
                        )
                      }
                    />
                    <span>
                      {s.name}
                      <small>{s.kind}</small>
                    </span>
                  </label>
                ))
              ) : (
                <p className="help">
                  {t(
                    "No data sources are enabled. Add and enable a connection in administration first.",
                  )}
                </p>
              )}
            </div>
            <p className="help">
              {t(
                "Only the selected data sources are shared. Revoke access at any time in Agents.",
              )}
            </p>
            <div className="consent-actions">
              <Button disabled={busy} onClick={() => consent(false)}>
                {t("Deny")}
              </Button>
              <Button
                primary
                busy={busy}
                disabled={!selected.length}
                onClick={() => consent(true)}
              >
                {t("Allow read-only access")}
              </Button>
            </div>
          </>
        ) : null}
      </section>
    </div>
  );
}
