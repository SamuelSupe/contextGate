import { t, useLocale } from "./i18n";
import { lazy, useCallback, useEffect, useRef, useState } from "react";
import {
  Database,
  Network,
  Home as HomeIcon,
  BookOpen,
  Users,
  Settings as SettingsIcon,
  LogOut,
  Menu,
  X,
  CheckCircle2,
} from "lucide-react";
import { api, message, setCSRF } from "./api";
import { Button, ErrorNote, Loading } from "./components";
import { canNavigate } from "./useNavigationGuard";
import { queryToolURL } from "./query-publishing";
import { administratorRole } from "./display";
import { Login } from "./Login";
import { PageBoundary } from "./PageBoundary";
import { Brand } from "./Brand";
import type { Agent, Capability, Session, Settings, Source } from "./types";

const Ontologies = lazy(() =>
  import("./Ontologies").then((module) => ({ default: module.Ontologies })),
);
const Semantics = lazy(() =>
  import("./Semantics").then((module) => ({ default: module.Semantics })),
);
const QueryPublishing = lazy(() =>
  import("./QueryPublishing").then((module) => ({ default: module.QueryPublishing })),
);
const SourceSetup = lazy(() =>
  import("./SourceSetup").then((module) => ({ default: module.SourceSetup })),
);
const QueryEvaluation = lazy(() =>
  import("./QueryEvaluation").then((module) => ({ default: module.QueryEvaluation })),
);
const Home = lazy(() =>
  import("./Home").then((module) => ({ default: module.Home })),
);
const BusinessCatalog = lazy(() =>
  import("./BusinessCatalog").then((module) => ({ default: module.BusinessCatalog })),
);
const HealthPage = lazy(() =>
  import("./Health").then((module) => ({ default: module.HealthPage })),
);
const Sources = lazy(() =>
  import("./Sources").then((module) => ({ default: module.Sources })),
);
const Agents = lazy(() =>
  import("./Agents").then((module) => ({ default: module.Agents })),
);
const AuditPage = lazy(() =>
  import("./Audit").then((module) => ({ default: module.AuditPage })),
);
const CatalogPage = lazy(() =>
  import("./CatalogPage").then((module) => ({ default: module.CatalogPage })),
);
const SettingsPage = lazy(() =>
  import("./SettingsPage").then((module) => ({ default: module.SettingsPage })),
);
const ConsentPage = lazy(() =>
  import("./ConsentPage").then((module) => ({ default: module.ConsentPage })),
);
const RequiredPasswordChange = lazy(() =>
  import("./Administrators").then((module) => ({ default: module.RequiredPasswordChange })),
);

const nav = [
  ["/", "Home", HomeIcon],
  ["/business", "Query tools", BookOpen],
  ["/sources", "Data sources", Database],
  ["/ontologies", "Ontologies", Network],
  ["/agents", "Agents", Users],
  ["/health", "Operations", CheckCircle2],
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
  const [querySeed, setQuerySeed] = useState<{
    sourceID: string;
    query: string;
  }>();
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
  function startQuery(source: Source, query: string) {
    setQuerySeed({ sourceID: source.id, query });
    navigate(queryToolURL(source.id));
  }
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
    const expired = () => {
      setQuerySeed(undefined);
      setCSRF("");
      setSession({ initialized: true, authenticated: false, csrf: "" });
      setSources([]);
      setAgents([]);
      setError("");
      setMobile(false);
    };
    window.addEventListener("contextgate:session-expired", expired);
    window.addEventListener("popstate", change);
    api<Session>("/api/session")
      .then((s) => {
        setCSRF(s.csrf);
        setSession(s);
      })
      .catch((e) => setError(message(e)));
    return () => {
      window.removeEventListener("popstate", change);
      window.removeEventListener("contextgate:session-expired", expired);
    };
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
    if (!session?.authenticated || session.administrator?.must_change_password)
      return;
    setLoading(true);
    reload()
      .catch((e) => setError(message(e)))
      .finally(() => setLoading(false));
  }, [
    session?.authenticated,
    session?.administrator?.must_change_password,
    reload,
  ]);
  const loggedIn = (result: Session) => {
    const csrf = result.csrf;
    setLoading(true);
    setCSRF(csrf);
    setSession({ ...result, initialized: true, authenticated: true });
    if (result.administrator?.must_change_password) return;
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
  if (session.administrator?.must_change_password)
    return (
      <PageBoundary>
        <RequiredPasswordChange
          administrator={session.administrator}
          onComplete={loggedIn}
        />
      </PageBoundary>
    );
  if (path === "/oauth/consent")
    return (
      <PageBoundary>
        <ConsentPage notify={notify} />
      </PageBoundary>
    );
  const workflowRoute = path.match(/^\/sources\/([^/]+)\/(setup|evaluation)$/);
  const workflowSource = sources.find((s) => s.id === workflowRoute?.[1]);
  const semanticID = path.match(/^\/sources\/([^/]+)\/semantics$/)?.[1];
  const semanticSource = sources.find((s) => s.id === semanticID);
  const ontologyID = path.match(/^\/ontologies\/([^/]+)$/)?.[1];
  const publishing = path === "/queries/publish";
  const active = publishing
    ? "/business"
    : ontologyID
      ? "/ontologies"
      : workflowRoute || semanticID
        ? "/sources"
        : [...nav.map((n) => n[0]), "/audit", "/catalog"].find(
            (url) => url === path,
          );
  const navActive =
    active === "/audit"
      ? "/health"
      : active === "/catalog"
        ? "/settings"
        : active;
  const secondary =
    navActive === "/health"
      ? [
          ["/health", "Health"],
          ["/audit", "Audit log"],
        ]
      : navActive === "/settings"
        ? [
            ["/settings", "Settings"],
            ["/catalog", "Data source types"],
          ]
        : [];
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
              className={navActive === url ? "active" : ""}
              aria-current={navActive === url ? "page" : undefined}
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
          <span>
            {session.administrator?.display_name}
            <small className="block help">
              {administratorRole(session.administrator?.role || "admin")}
            </small>
          </span>
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
                setQuerySeed(undefined);
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
        {!!secondary.length && (
          <nav
            className="workspace-subnav"
            aria-label={t("Section navigation")}
          >
            {secondary.map(([url, label]) => (
              <a
                key={url}
                href={url}
                aria-current={active === url ? "page" : undefined}
                onClick={(event) => {
                  event.preventDefault();
                  navigate(url);
                }}
              >
                {t(label)}
              </a>
            ))}
          </nav>
        )}
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
        <PageBoundary key={path}>
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
          ) : publishing ? (
            <QueryPublishing
              key={`${new URLSearchParams(routeQuery).get("source_id")}:${new URLSearchParams(routeQuery).get("template_id")}`}
              sources={sources}
              agents={agents}
              catalog={catalog}
              settings={settings}
              administratorID={session.administrator!.id}
              initialQuery={routeQuery}
              seed={querySeed}
              clearSeed={() => setQuerySeed(undefined)}
              navigate={navigate}
              reload={reload}
              notify={notify}
            />
          ) : workflowRoute ? (
            workflowSource ? (
              workflowRoute[2] === "setup" ? (
                <SourceSetup
                  onCreateQuery={startQuery}
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
                onQuery={(id = "") =>
                  navigate(
                    queryToolURL(semanticID, id, "", undefined, true) +
                      "&from=semantics",
                  )
                }
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
              administratorID={session.administrator!.id}
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
              onCreateQuery={startQuery}
              navigate={navigate}
              agents={agents}
              sources={sources}
              catalog={catalog}
              reload={reload}
              notify={notify}
            />
          ) : active === "/ontologies" ? (
            <Ontologies
              key={`${ontologyID || "ontology-list"}:${routeQuery}`}
              id={ontologyID}
              initialQuery={routeQuery}
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
            <HealthPage
              superAdmin={session.administrator?.role === "super_admin"}
              navigate={navigate}
              reloadSources={reload}
            />
          ) : active === "/audit" ? (
            <AuditPage
              superAdmin={session.administrator?.role === "super_admin"}
              sources={sources}
              agents={agents}
              navigate={navigate}
            />
          ) : active === "/catalog" ? (
            <CatalogPage catalog={catalog} />
          ) : (
            <SettingsPage
              administrator={session.administrator!}
              settings={settings}
              notify={notify}
            />
          )}
        </PageBoundary>
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
