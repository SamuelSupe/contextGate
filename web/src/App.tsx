import { useCallback, useEffect, useState } from "react";
import {
  Database,
  Users,
  ClipboardList,
  ListChecks,
  Settings as SettingsIcon,
  LogOut,
  Menu,
  X,
  CheckCircle2,
} from "lucide-react";
import { api, message, payload, setCSRF } from "./api";
import { Button, ErrorNote, Field, Loading } from "./components";
import { Sources } from "./Sources";
import { Agents } from "./Agents";
import { AuditPage } from "./Audit";
import { CatalogPage, SettingsPage, ConsentPage } from "./Pages";
import type { Agent, Capability, Session, Settings, Source } from "./types";

const nav = [
  ["/sources", "Data sources", Database],
  ["/agents", "Agents", Users],
  ["/audit", "Audit log", ClipboardList],
  ["/catalog", "Supported databases", ListChecks],
  ["/settings", "Settings", SettingsIcon],
] as const;
export function App() {
  const [session, setSession] = useState<Session | null>(null);
  const [path, setPath] = useState(location.pathname);
  const [sources, setSources] = useState<Source[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [catalog, setCatalog] = useState<Capability[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [toast, setToast] = useState("");
  const [mobile, setMobile] = useState(false);
  const notify = useCallback((text: string) => setToast(text), []);
  const navigate = useCallback((next: string) => {
    history.pushState({}, "", next);
    setPath(location.pathname);
    setMobile(false);
  }, []);
  useEffect(() => {
    const change = () => setPath(location.pathname);
    window.addEventListener("popstate", change);
    api<Session>("/api/session")
      .then((s) => {
        setCSRF(s.csrf);
        setSession(s);
      })
      .catch((e) => setError(message(e)));
    return () => window.removeEventListener("popstate", change);
  }, []);
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 4000);
    return () => clearTimeout(timer);
  }, [toast]);
  const reload = useCallback(async () => {
    const [s, a, c, t] = await Promise.all([
      api<Source[]>("/api/sources"),
      api<Agent[]>("/api/agents"),
      api<Capability[]>("/api/catalog"),
      api<Settings>("/api/settings"),
    ]);
    setSources(s);
    setAgents(a);
    setCatalog(c);
    setSettings(t);
  }, []);
  useEffect(() => {
    if (!session?.authenticated) return;
    setLoading(true);
    reload()
      .catch((e) => setError(message(e)))
      .finally(() => setLoading(false));
  }, [session?.authenticated, reload]);
  const loggedIn = (csrf: string) => {
    setCSRF(csrf);
    setSession({ initialized: true, authenticated: true, csrf });
    const next = new URLSearchParams(location.search).get("next");
    if (
      next?.startsWith("/") &&
      new URL(next, location.origin).origin === location.origin
    )
      location.assign(next);
    else navigate("/sources");
  };
  if (!session)
    return (
      <div className="boot">
        <h1>MCP DB Hub</h1>
        {error ? (
          <>
            <ErrorNote error={error} />
            <Button onClick={() => location.reload()}>Reconnect</Button>
          </>
        ) : (
          <Loading />
        )}
      </div>
    );
  if (!session.authenticated)
    return <Login initialized={session.initialized} onLogin={loggedIn} />;
  if (path === "/oauth/consent") return <ConsentPage notify={notify} />;
  const active = nav.find((n) => n[0] === path)?.[0] || "/sources";
  return (
    <div className="app">
      <button
        className="mobile-menu icon-button"
        aria-label="Open navigation"
        onClick={() => setMobile(true)}
      >
        <Menu />
      </button>
      {mobile ? (
        <div className="mobile-scrim" onClick={() => setMobile(false)} />
      ) : null}
      <aside className={`sidebar ${mobile ? "open" : ""}`}>
        <div className="brand">
          MCP DB Hub
          <button
            className="icon-button mobile-close"
            onClick={() => setMobile(false)}
            aria-label="Close navigation"
          >
            <X size={18} />
          </button>
        </div>
        <nav>
          {nav.map(([url, label, Icon]) => (
            <a
              key={url}
              href={url}
              className={active === url ? "active" : ""}
              onClick={(e) => {
                e.preventDefault();
                navigate(url);
              }}
            >
              <Icon size={19} strokeWidth={1.6} />
              {label}
            </a>
          ))}
        </nav>
        <div className="account">
          <span className="avatar">A</span>
          <span>administrator</span>
          <button
            title="Sign out"
            aria-label="Sign out"
            className="icon-button"
            onClick={async () => {
              await api("/api/logout", { method: "POST" });
              setCSRF("");
              setSession({ ...session, authenticated: false });
            }}
          >
            <LogOut size={16} />
          </button>
        </div>
      </aside>
      <main>
        <ErrorNote error={error} />
        {loading ? (
          <Loading />
        ) : active === "/sources" ? (
          <Sources
            agents={agents}
            sources={sources}
            catalog={catalog}
            reload={reload}
            notify={notify}
          />
        ) : active === "/agents" ? (
          <Agents
            agents={agents}
            sources={sources}
            settings={settings}
            reload={reload}
            notify={notify}
          />
        ) : active === "/audit" ? (
          <AuditPage sources={sources} agents={agents} />
        ) : active === "/catalog" ? (
          <CatalogPage catalog={catalog} />
        ) : (
          <SettingsPage settings={settings} notify={notify} />
        )}
      </main>
      {toast ? (
        <div className="toast" role="status">
          <CheckCircle2 size={17} />
          {toast}
        </div>
      ) : null}
    </div>
  );
}
function Login({
  initialized,
  onLogin,
}: {
  initialized: boolean;
  onLogin: (csrf: string) => void;
}) {
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <div className="auth">
      <div className="auth-brand">
        <Database size={24} /> MCP DB Hub
      </div>
      <section className="auth-panel">
        <h1>
          {initialized ? "Administrator sign in" : "Set up administrator"}
        </h1>
        <p>
          {initialized
            ? "Manage database connections and read-only access for your Agents."
            : "Enter the one-time setup code from the server log to create your administrator password."}
        </p>
        <ErrorNote error={error} />
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setError("");
            try {
              const s = await api<{ csrf: string }>(
                initialized ? "/api/login" : "/api/setup",
                {
                  method: "POST",
                  body: payload(
                    initialized ? { password } : { password, token },
                  ),
                },
              );
              onLogin(s.csrf);
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          {!initialized ? (
            <Field label="One-time setup code" required>
              <input
                autoComplete="off"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
              />
            </Field>
          ) : null}
          <Field
            label={
              initialized
                ? "Administrator password"
                : "Create administrator password"
            }
            required
            hint={
              !initialized
                ? "At least 12 characters. Passwords are never stored in plain text."
                : undefined
            }
          >
            <input
              type="password"
              autoComplete={initialized ? "current-password" : "new-password"}
              minLength={initialized ? 1 : 12}
              maxLength={256}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </Field>
          <Button primary busy={busy} type="submit">
            {initialized ? "Sign in" : "Create administrator"}
          </Button>
        </form>
        {initialized ? (
          <details className="recovery-help">
            <summary>Forgot your password?</summary>
            <p>
              On the server, stop the service and run{" "}
              <code>
                mcpdbhub reset-password --data-dir DIR --password-stdin
              </code>{" "}
              with the existing configuration directory and master key. Supply
              the new password through standard input, then restart the service.
            </p>
            <p>
              All administrator sessions are signed out. Data sources, Agent
              credentials and audit history are retained.
            </p>
          </details>
        ) : null}
      </section>
      <p className="auth-foot">
        Read-only queries · Individual grants · Auditable access
      </p>
    </div>
  );
}
