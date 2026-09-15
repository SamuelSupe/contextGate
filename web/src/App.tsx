import { t, useLocale, setLocale, validLocale } from "./i18n";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Database,
  Home as HomeIcon,
  BookOpen,
  Network,
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
import { Ontologies } from "./Ontologies";
import { canNavigate } from "./useNavigationGuard";
import { Semantics } from "./Semantics";
import { SourceSetup } from "./SourceSetup";
import { QueryEvaluation } from "./QueryEvaluation";
import { Home } from "./Home";
import { BusinessCatalog } from "./BusinessCatalog";
import { HealthPage } from "./Health";
import { Sources } from "./Sources";
import { Brand } from "./Brand";
import { Agents } from "./Agents";
import { AuditPage } from "./Audit";
import { CatalogPage, SettingsPage, ConsentPage } from "./Pages";
import type { Agent, Capability, Session, Settings, Source } from "./types";

const nav = [
  ["/", "Home", HomeIcon],
  ["/business", "Business catalog", BookOpen],
  ["/sources", "Data sources", Database],
  ["/ontologies", "Ontologies", Network],
  ["/agents", "Agents", Users],
  ["/health", "Health", CheckCircle2],
  ["/audit", "Audit log", ClipboardList],
  ["/catalog", "Data source types", ListChecks],
  ["/settings", "Settings", SettingsIcon],
] as const;
export function App() {
  const locale = useLocale();
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  const [session, setSession] = useState<Session | null>(null);
  const [path, setPath] = useState(location.pathname);
  const [routeQuery, setRouteQuery] = useState(location.search);
  const [sources, setSources] = useState<Source[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [catalog, setCatalog] = useState<Capability[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState("");
  const [mobile, setMobile] = useState(false);
  const [narrow, setNarrow] = useState(
    () => matchMedia("(max-width: 760px)").matches,
  );
  const mainRef = useRef<HTMLElement>(null);
  const menuRef = useRef<HTMLButtonElement>(null);
  const sidebarRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const media = matchMedia("(max-width: 760px)");
    const change = () => {
      const matches = matchMedia("(max-width: 760px)").matches;
      setNarrow(matches);
      if (!matches) setMobile(false);
    };
    media.addEventListener("change", change);
    const observer = new ResizeObserver(change);
    observer.observe(document.documentElement);
    return () => {
      media.removeEventListener("change", change);
      observer.disconnect();
    };
  }, []);
  useEffect(() => {
    const target =
      mainRef.current?.querySelector<HTMLElement>("[data-route-focus]");
    if (target) {
      target.scrollIntoView({ block: "center" });
      target.focus({ preventScroll: true });
    } else {
      window.scrollTo(0, 0);
      mainRef.current?.focus({ preventScroll: true });
    }
  }, [path]);
  useEffect(() => {
    if (!mobile) return;
    sidebarRef.current?.querySelector<HTMLButtonElement>("button")?.focus();
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setMobile(false);
        menuRef.current?.focus();
      }
      if (e.key === "Tab") {
        const items =
          sidebarRef.current?.querySelectorAll<HTMLElement>("button,a[href]");
        if (!items?.length) return;
        const first = items[0],
          last = items[items.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [mobile]);
  const notify = useCallback((text: string) => setToast(text), []);
  const historyIndex = useRef(0);
  const currentURL = useRef(
    location.pathname + location.search + location.hash,
  );
  const restoringHistory = useRef(false);
  const navigate = useCallback((next: string) => {
    if (next === currentURL.current) {
      setMobile(false);
      return;
    }
    if (!canNavigate()) {
      setMobile(false);
      setToast(
        t("Changes kept. Finish or cancel the current edit before leaving."),
      );
      return;
    }
    historyIndex.current++;
    history.pushState({ mcpdbhubIndex: historyIndex.current }, "", next);
    currentURL.current = location.pathname + location.search + location.hash;
    setPath(location.pathname);
    setRouteQuery(location.search);
    setMobile(false);
  }, []);
  useEffect(() => {
    history.replaceState({ ...history.state, mcpdbhubIndex: 0 }, "");
    const change = (event: PopStateEvent) => {
      if (restoringHistory.current) {
        restoringHistory.current = false;
        return;
      }
      const nextIndex = event.state?.mcpdbhubIndex;
      if (!canNavigate()) {
        if (
          Number.isSafeInteger(nextIndex) &&
          nextIndex !== historyIndex.current
        ) {
          restoringHistory.current = true;
          history.go(historyIndex.current - nextIndex);
        } else {
          history.pushState(
            { mcpdbhubIndex: historyIndex.current },
            "",
            currentURL.current,
          );
        }
        setToast(
          t("Changes kept. Finish or cancel the current edit before leaving."),
        );
        return;
      }
      historyIndex.current = Number.isSafeInteger(nextIndex) ? nextIndex : 0;
      currentURL.current = location.pathname + location.search + location.hash;
      setPath(location.pathname);
      setRouteQuery(location.search);
      setMobile(false);
    };
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
    setLoading(true);
    setCSRF(csrf);
    setSession({ initialized: true, authenticated: true, csrf });
    const next = new URLSearchParams(location.search).get("next");
    if (
      next?.startsWith("/") &&
      new URL(next, location.origin).origin === location.origin
    )
      location.assign(next);
    else navigate("/");
  };
  if (!session)
    return (
      <div className="boot">
        <h1>
          <Brand />
        </h1>
        {error ? (
          <>
            <ErrorNote error={error} />
            <Button onClick={() => location.reload()}>{t("Reconnect")}</Button>
          </>
        ) : (
          <Loading />
        )}
      </div>
    );
  if (!session.authenticated)
    return <Login initialized={session.initialized} onLogin={loggedIn} />;
  if (path === "/oauth/consent") return <ConsentPage notify={notify} />;
  const workflowRoute = path.match(/^\/sources\/([^/]+)\/(setup|evaluation)$/);
  const workflowSource = sources.find((s) => s.id === workflowRoute?.[1]);
  const semanticID = path.match(/^\/sources\/([^/]+)\/semantics$/)?.[1];
  const semanticSource = sources.find((s) => s.id === semanticID);
  const ontologyID = path.match(/^\/ontologies\/([^/]+)$/)?.[1];
  const active = ontologyID
    ? "/ontologies"
    : workflowRoute || semanticID
      ? "/sources"
      : nav.find((n) => n[0] === path)?.[0];
  return (
    <div className="app">
      <button
        ref={menuRef}
        className="mobile-menu icon-button"
        aria-label={t("Open navigation")}
        aria-expanded={mobile}
        aria-controls="main-navigation"
        onClick={() => setMobile(true)}
      >
        <Menu />
      </button>
      {mobile ? (
        <div
          className="mobile-scrim"
          onClick={() => {
            setMobile(false);
            menuRef.current?.focus();
          }}
        />
      ) : null}
      <aside
        ref={sidebarRef}
        id="main-navigation"
        inert={narrow && !mobile}
        className={`sidebar ${mobile ? "open" : ""}`}
      >
        <div className="brand">
          <Brand />
          <button
            className="icon-button mobile-close"
            onClick={() => {
              setMobile(false);
              menuRef.current?.focus();
            }}
            aria-label={t("Close navigation")}
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
              aria-current={active === url ? "page" : undefined}
              onClick={(e) => {
                e.preventDefault();
                navigate(url);
              }}
            >
              <Icon size={19} strokeWidth={1.6} />
              {t(label)}
            </a>
          ))}
        </nav>
        <div className="account">
          <span className="avatar">{t("A")}</span>
          <span>{t("administrator")}</span>
          <button
            title={t("Sign out")}
            aria-label={t("Sign out")}
            className="icon-button"
            onClick={async () => {
              if (!canNavigate()) return;
              try {
                await api("/api/logout", { method: "POST" });
                setCSRF("");
                setSession({ ...session, authenticated: false });
              } catch (e) {
                setError(message(e));
              }
            }}
          >
            <LogOut size={16} />
          </button>
        </div>
      </aside>
      <main ref={mainRef} tabIndex={-1} inert={narrow && mobile}>
        <ErrorNote error={error} />
        {error && (
          <Button
            disabled={loading}
            onClick={async () => {
              setLoading(true);
              setError("");
              try {
                await reload();
              } catch (e) {
                setError(message(e));
              } finally {
                setLoading(false);
              }
            }}
          >
            {t("Retry loading data")}
          </Button>
        )}
        {loading ? (
          <Loading />
        ) : error && !settings ? null : !active ? (
          <div className="empty">
            <h1>{t("Page not found")}</h1>
            <p>
              {t(
                "Choose a section from the navigation or return to your data sources.",
              )}
            </p>
            <Button onClick={() => navigate("/sources")}>
              {t("Data sources")}
            </Button>
          </div>
        ) : workflowRoute ? (
          workflowSource ? (
            workflowRoute[2] === "setup" ? (
              <SourceSetup
                key={workflowSource.id}
                source={workflowSource}
                agents={agents}
                catalog={catalog}
                navigate={navigate}
                reload={reload}
                notify={notify}
              />
            ) : (
              <QueryEvaluation
                key={workflowSource.id}
                source={workflowSource}
                agents={agents}
                navigate={navigate}
                notify={notify}
              />
            )
          ) : (
            <div>
              <ErrorNote error={t("Data source not found.")} />
              <Button onClick={() => navigate("/sources")}>
                {t("Data sources")}
              </Button>
            </div>
          )
        ) : semanticID ? (
          semanticSource ? (
            <Semantics
              key={semanticID + routeQuery}
              initialQuery={routeQuery}
              source={semanticSource}
              agents={agents}
              notify={notify}
              onBack={() => navigate("/sources")}
              onWorkspace={() => navigate(`/sources/${semanticID}/setup`)}
            />
          ) : (
            <div>
              <ErrorNote error={t("Data source not found.")} />
              <Button onClick={() => navigate("/sources")}>
                {t("Data sources")}
              </Button>
            </div>
          )
        ) : active === "/" ? (
          <Home
            sources={sources}
            catalog={catalog}
            navigate={navigate}
            reload={reload}
            notify={notify}
          />
        ) : active === "/business" ? (
          <BusinessCatalog
            key={routeQuery}
            initialQuery={routeQuery}
            sources={sources}
            agents={agents}
            navigate={navigate}
          />
        ) : active === "/sources" ? (
          <Sources
            navigate={navigate}
            agents={agents}
            sources={sources}
            catalog={catalog}
            reload={reload}
            notify={notify}
          />
        ) : active === "/ontologies" ? (
          <Ontologies
            key={ontologyID || "ontology-list"}
            id={ontologyID}
            sources={sources}
            agents={agents}
            navigate={navigate}
            notify={notify}
          />
        ) : active === "/agents" ? (
          <Agents
            key={routeQuery}
            initialQuery={routeQuery}
            navigate={navigate}
            agents={agents}
            sources={sources}
            settings={settings}
            reload={reload}
            notify={notify}
          />
        ) : active === "/health" ? (
          <HealthPage navigate={navigate} reloadSources={reload} />
        ) : active === "/audit" ? (
          <AuditPage sources={sources} agents={agents} navigate={navigate} />
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
  const locale = useLocale();
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <div className="auth">
      <div className="auth-language">
        <label>
          {t("Display language")}
          <select
            value={locale}
            onChange={(e) => setLocale(validLocale(e.target.value))}
          >
            <option value="en" lang="en">
              English
            </option>
            <option value="zh-CN" lang="zh-CN">
              简体中文
            </option>
          </select>
        </label>
      </div>
      <div className="auth-brand">
        <Brand tagline />
      </div>
      <p className="auth-tagline">
        {t("Understand your business. Query data safely.")}
      </p>
      <section className="auth-panel">
        <h1>
          {initialized ? t("Administrator sign in") : t("Set up administrator")}
        </h1>
        <p>
          {initialized
            ? t("Manage business context, verified queries and Agent access.")
            : t(
                "Enter the one-time setup code from the server log to create your administrator password.",
              )}
        </p>
        {!initialized && (
          <details className="recovery-help">
            <summary>{t("Where do I find the setup code?")}</summary>
            <p>
              {t(
                "Docker Compose: run this command in the deployment directory, then copy the value after First-time setup token:",
              )}
            </p>
            <pre>docker compose logs hub</pre>
            <p>
              {t(
                "Local binary: look in the terminal where you started contextgate serve. The latest startup code is valid until administrator setup completes. Restart the service to generate a new code if needed.",
              )}
            </p>
          </details>
        )}
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
            <Field label={t("One-time setup code")} required>
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
                ? t("Administrator password")
                : t("Create administrator password")
            }
            required
            hint={
              !initialized
                ? t(
                    "At least 12 characters. Passwords are never stored in plain text.",
                  )
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
            {initialized ? t("Sign in") : t("Create administrator")}
          </Button>
        </form>
        {initialized ? (
          <details className="recovery-help">
            <summary>{t("Forgot your password?")}</summary>
            <p>
              {t("On the server, stop the service and run")}{" "}
              <code>
                contextgate reset-password --data-dir DIR --password-stdin
              </code>{" "}
              {t(
                "with the same MCPDBHUB_DATABASE_URL, key directory and master key as the service. Supply the new password through standard input, then restart the service.",
              )}
            </p>
            <p>
              {t(
                "All administrator sessions are signed out. Data sources, Agent credentials and audit history are retained.",
              )}
            </p>
          </details>
        ) : null}
      </section>
      <p className="auth-foot">
        {t("Read-only queries · Individual grants · Auditable access")}
      </p>
    </div>
  );
}
