import { capabilityLabel } from "./display";
import { useEffect, useState } from "react";
import { ShieldCheck, CheckCircle2, Search } from "lucide-react";
import { api, message, payload, setCSRF } from "./api";
import { Button, CopyButton, ErrorNote, Field, Loading } from "./components";
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
          <h1>Supported databases</h1>
          <p>Native query capabilities, protection and tested versions</p>
        </div>
      </div>
      <div className="filters">
        <div className="search-input">
          <Search size={16} />
          <input
            placeholder="Search databases"
            aria-label="Search supported databases"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <span className="help">{catalog.length} products</span>
      </div>
      <div className="table-scroll">
        <table className="catalog-table">
          <thead>
            <tr>
              <th>Database</th>
              <th>Agent query tool</th>
              <th>Pagination</th>
              <th>Tested versions</th>
              <th>Protection and limitations</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((c) => (
              <tr key={c.kind}>
                <td className="semibold">{c.name}</td>
                <td>
                  <code>{c.tool}</code>
                  <small className="block">
                    {c.parameters ? "Parameter binding" : "Native parameters"}
                  </small>
                </td>
                <td>{capabilityLabel(c.pagination)}</td>
                <td>
                  {c.verified_versions.length ? (
                    <span className="status green">
                      <CheckCircle2 size={14} />
                      {c.verified_versions.join("、")}
                    </span>
                  ) : (
                    <span className="muted">Not verified</span>
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
      <div className="notice neutral">
        <ShieldCheck size={19} />
        <p>
          Compatible products are tested individually. Engine protection and
          account permissions are separate checks. InfluxDB 3 Core uses query
          API isolation.
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
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  if (!settings) return <Loading />;
  return (
    <>
      <div className="page-header">
        <div>
          <h1>Settings</h1>
          <p>Service connection and administrator security settings</p>
        </div>
      </div>
      <div className="settings-body">
        <section>
          <h2>Service information</h2>
          <dl className="settings-list">
            <div>
              <dt>Service version</dt>
              <dd>{settings.version}</dd>
            </div>
            <div>
              <dt>MCP endpoint</dt>
              <dd>
                <code>{settings.mcp_url}</code>
                <CopyButton
                  text={settings.mcp_url}
                  onCopied={() => notify("MCP endpoint copied")}
                />
              </dd>
            </div>
            <div>
              <dt>Database file directory</dt>
              <dd>
                <code>{settings.database_directory}</code>
              </dd>
            </div>
            <div>
              <dt>Audit retention</dt>
              <dd>{settings.audit_retention_days} days</dd>
            </div>
            <div>
              <dt>Concurrency limits</dt>
              <dd>
                Global {settings.global_concurrency} · Per Agent{" "}
                {settings.agent_concurrency}
              </dd>
            </div>
            <div>
              <dt>OAuth lifetime</dt>
              <dd>
                Access token {settings.oauth_access_token_minutes} minutes ·
                Refresh grant {settings.oauth_refresh_token_days} days
              </dd>
            </div>
          </dl>
          <p className="help">
            Set the public URL and file directory using startup options, then
            restart the service.
          </p>
        </section>
        <section>
          <h2>Change administrator password</h2>
          <p className="help">
            Changing the password signs out other administrator sessions. Agent
            grants remain valid.
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
                notify("Administrator password updated");
              } catch (e) {
                setError(message(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            <Field label="Current password" required>
              <input
                type="password"
                autoComplete="current-password"
                required
                value={current}
                onChange={(e) => setCurrent(e.target.value)}
              />
            </Field>
            <Field label="New password" required hint="At least 12 characters">
              <input
                type="password"
                autoComplete="new-password"
                required
                minLength={12}
                maxLength={256}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </Field>
            <Button primary busy={busy} type="submit">
              Update password
            </Button>
          </form>
        </section>
        <section>
          <h2>Administrator recovery</h2>
          <p>
            Recovery requires local access to the server and its configuration
            directory. Stop the service, provide a new password through standard
            input, then restart:
          </p>
          <pre>mcpdbhub reset-password --data-dir DIR --password-stdin</pre>
          <p className="help">
            Keep the existing master key. Recovery signs out all administrator
            sessions and retains data sources, Agent credentials and audit
            history.
          </p>
        </section>
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
      notify(allow ? "Access granted" : "Access denied");
      location.assign(out.redirect);
    } catch (e) {
      setError(message(e));
      setBusy(false);
    }
  }
  return (
    <div className="auth">
      <div className="auth-brand">
        <ShieldCheck size={24} />
        MCP DB Hub
      </div>
      <section className="auth-panel consent-panel">
        <h1>Authorize read-only database access</h1>
        <ErrorNote error={error} />
        {!info && error ? (
          <a href="/sources">Return to administration</a>
        ) : null}
        {!info && !error ? (
          <Loading />
        ) : info ? (
          <>
            <p>
              <strong>{info.client_name}</strong> requests database access
              through MCP.
            </p>
            <dl className="consent-details">
              <dt>Client ID</dt>
              <dd>{info.client_id}</dd>
              <dt>Return URL</dt>
              <dd>{info.redirect_uri}</dd>
            </dl>
            <h3>Allowed data sources</h3>
            <div className="source-checklist">
              {info.sources.length ? (
                info.sources.map((s) => (
                  <label key={s.id} className="check-row">
                    <input
                      type="checkbox"
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
                  No data sources are enabled. Add and enable a connection in
                  administration first.
                </p>
              )}
            </div>
            <p className="help">
              Only the selected data sources are shared. Revoke access at any
              time in Agents.
            </p>
            <div className="consent-actions">
              <Button disabled={busy} onClick={() => consent(false)}>
                Deny
              </Button>
              <Button
                primary
                busy={busy}
                disabled={!selected.length}
                onClick={() => consent(true)}
              >
                Allow read-only access
              </Button>
            </div>
          </>
        ) : null}
      </section>
    </div>
  );
}
