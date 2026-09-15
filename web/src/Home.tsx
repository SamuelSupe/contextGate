import { useEffect, useState } from "react";
import { ArrowRight, Search, CheckCircle2 } from "lucide-react";
import { api, date, message } from "./api";
import { Button, Empty, ErrorNote, Field, Loading } from "./components";
import { t } from "./i18n";
import { SourceEditor } from "./SourceEditor";
import {
  connectionReady,
  readinessLabel,
  semanticsURL,
  useReadiness,
} from "./readiness";
import type { Agent, Capability, Source } from "./types";
import "./business-workflows.css";

type HealthSummary = {
  sources: {
    id: string;
    name: string;
    status: string;
    invalid_templates: unknown[];
  }[];
  expiring_agents: Agent[];
  pending_changes: number;
  audit_export: { state: string; last_error?: string };
};

export function Home({
  sources,
  catalog,
  navigate,
  reload,
  notify,
}: {
  sources: Source[];
  catalog: Capability[];
  navigate: (url: string) => void;
  reload: () => Promise<void>;
  notify: (text: string) => void;
}) {
  const [sourceID, setSourceID] = useState("");
  const [keyword, setKeyword] = useState("");
  const [adding, setAdding] = useState(false);
  const [health, setHealth] = useState<HealthSummary | null>(null);
  const [error, setError] = useState("");
  const [refresh, setRefresh] = useState(0);
  const source = sources.find((s) => s.id === sourceID) || sources[0];
  useEffect(() => {
    const abort = new AbortController();
    setError("");
    api<HealthSummary>("/api/health", { signal: abort.signal })
      .then(setHealth)
      .catch((e) => {
        if (!abort.signal.aborted) {
          setHealth(null);
          setError(message(e));
        }
      });
    return () => abort.abort();
  }, [sources, refresh]);
  const attention =
    health?.sources.filter(
      (s) =>
        !["checked", "disabled"].includes(s.status) ||
        s.invalid_templates.length,
    ) || [];
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Home")}</h1>
          <p>{t("Turn business questions into trusted Agent queries.")}</p>
        </div>
        <Button onClick={() => setAdding(true)}>{t("Add data source")}</Button>
      </div>
      {sources.length > 0 && (
        <section className="business-search-banner">
          <h2>{t("What can your Agent answer?")}</h2>
          <p>
            {t(
              "Find published terms, metrics, business concepts and verified queries across your data sources.",
            )}
          </p>
          <form
            className="business-search-form"
            onSubmit={(e) => {
              e.preventDefault();
              navigate(
                `/business?${new URLSearchParams({ keyword: keyword.trim() })}`,
              );
            }}
          >
            <div className="search-input">
              <Search size={18} />
              <input
                aria-label={t("Search business catalog")}
                placeholder={t("Try a keyword: customer, revenue, orders…")}
                maxLength={256}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
            </div>
            <Button primary type="submit">
              {t("Explore business catalog")}
              <ArrowRight size={16} />
            </Button>
          </form>
        </section>
      )}
      <div className="home-columns">
        <section className="business-panel">
          <div className="section-heading">
            <h2>{t("From connection to first answer")}</h2>
          </div>
          {source ? (
            <>
              <Field label={t("Continue with a data source")}>
                <select
                  value={source.id}
                  onChange={(e) => setSourceID(e.target.value)}
                >
                  {sources.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </select>
              </Field>
              <SourceJourney
                key={source.id}
                source={source}
                navigate={navigate}
              />
            </>
          ) : (
            <Empty
              title={t("Start with one business question")}
              description={t(
                "Connect a database or read API, publish one useful query, then try it with your Agent. You can add a shared ontology later.",
              )}
              action={
                <Button primary onClick={() => setAdding(true)}>
                  {t("Connect your first data source")}
                </Button>
              }
            />
          )}
        </section>
        <section className="business-panel">
          <div className="section-heading">
            <h2>{t("Needs attention")}</h2>
            <Button onClick={() => setRefresh((x) => x + 1)}>
              {t("Refresh")}
            </Button>
          </div>
          <ErrorNote error={error} />
          {!health && !error ? (
            <Loading />
          ) : health ? (
            <>
              <ul className="business-attention">
                {attention.slice(0, 3).map((s) => (
                  <li key={s.id}>
                    <div>
                      <strong>{s.name}</strong>
                      <small>
                        {s.invalid_templates.length
                          ? t("Templates need validation")
                          : t("Review connection and structure evidence")}
                      </small>
                    </div>
                    <Button
                      onClick={() =>
                        navigate(
                          s.invalid_templates.length
                            ? semanticsURL(s.id, "Query templates")
                            : `/sources/${s.id}/setup`,
                        )
                      }
                    >
                      {t("Review")}
                    </Button>
                  </li>
                ))}
                {health.expiring_agents.length > 0 && (
                  <li>
                    <div>
                      <strong>{t("Agent credentials need attention")}</strong>
                      <small>
                        {t("Expired or expiring within 7 days: {count}", {
                          count: health.expiring_agents.length,
                        })}
                      </small>
                    </div>
                    <Button
                      onClick={() =>
                        navigate(
                          `/agents?attention=${encodeURIComponent(health.expiring_agents[0].id)}`,
                        )
                      }
                    >
                      {t("Review Agents")}
                    </Button>
                  </li>
                )}
                {(health.pending_changes > 0 ||
                  health.audit_export.last_error) && (
                  <li>
                    <span>
                      {t("Management or audit delivery needs review")}
                    </span>
                    <Button onClick={() => navigate("/health")}>
                      {t("Review")}
                    </Button>
                  </li>
                )}
              </ul>
              {!attention.length &&
                !health.expiring_agents.length &&
                !health.pending_changes &&
                !health.audit_export.last_error && (
                  <p className="help">
                    {t(
                      "No items need attention in the current health page. Open Health to review all sources and evidence coverage.",
                    )}
                  </p>
                )}
              <Button onClick={() => navigate("/health")}>
                {t("View all health checks")}
                <ArrowRight size={15} />
              </Button>
            </>
          ) : null}
        </section>
      </div>
      <section className="business-optional">
        <div>
          <h2>{t("Reuse business meaning when you need it")}</h2>
          <p>
            {t(
              "Shared ontologies connect concepts such as Customer and Order to different data sources. They are optional for your first query.",
            )}
          </p>
        </div>
        <Button onClick={() => navigate("/ontologies")}>
          {t("Explore ontologies")}
        </Button>
      </section>
      {adding && (
        <SourceEditor
          source={null}
          catalog={catalog}
          onClose={() => setAdding(false)}
          onSaved={async (notice, close, id) => {
            await reload();
            if (id) setSourceID(id);
            if (notice) notify(notice);
            if (close !== false) setAdding(false);
          }}
        />
      )}
    </>
  );
}

function SourceJourney({
  source,
  navigate,
}: {
  source: Source;
  navigate: (url: string) => void;
}) {
  const { data, error, refresh } = useReadiness(source.id, source.revision);
  if (error)
    return (
      <>
        <ErrorNote error={error} />
        <Button onClick={refresh}>{t("Retry")}</Button>
      </>
    );
  if (!data) return <Loading />;
  const steps = [
    {
      name: "Connect data source",
      done: connectionReady(source),
      url: `/sources/${source.id}/setup`,
      hint: "Review the connection and its protection evidence.",
    },
    {
      name:
        source.query_access_mode === "templates_only"
          ? "Publish one useful query"
          : "Choose a query",
      done:
        data.executable_templates > 0 ||
        source.query_access_mode !== "templates_only",
      url:
        source.query_access_mode === "templates_only"
          ? semanticsURL(source.id, "Query templates")
          : `/sources/${source.id}/setup`,
      hint:
        source.query_access_mode === "templates_only"
          ? "Write a native template, trial it and publish. Ontology mapping is optional."
          : "Native queries are available. Verified templates are optional.",
    },
    {
      name: "Grant Agent access",
      done: data.active_agents.length > 0,
      url: data.active_agents.length
        ? `/sources/${source.id}/setup`
        : `/agents?create=1&source_id=${encodeURIComponent(source.id)}`,
      hint: "Choose data source access and follow your client's setup guide.",
    },
    {
      name: "Confirm a real client query",
      done: !!data.last_query,
      url: `/sources/${source.id}/setup`,
      hint: "Ask in your Agent client. Administrator previews do not complete this step.",
    },
  ];
  const next = steps.findIndex((step) => !step.done);
  return (
    <>
      <p className="help">{readinessLabel(source, data)}</p>
      <ol className="journey-steps">
        {steps.map((step, index) => (
          <li key={step.name} className={index === next ? "current" : ""}>
            <span
              className="journey-marker"
              aria-label={
                step.done
                  ? t("Complete")
                  : t("Step {number}", { number: index + 1 })
              }
            >
              {step.done ? <CheckCircle2 size={19} /> : index + 1}
            </span>
            <div>
              <strong>{t(step.name)}</strong>
              <p>{t(step.hint)}</p>
              {index === 3 && data.last_query && (
                <small>
                  {t("Last successful client query: {time}", {
                    time: date(data.last_query),
                  })}
                </small>
              )}
            </div>
            <Button primary={index === next} onClick={() => navigate(step.url)}>
              {index === next ? t("Continue") : t("Open")}
            </Button>
          </li>
        ))}
      </ol>
      {next === -1 && (
        <div className="button-row">
          <Button
            primary
            onClick={() =>
              navigate(`/business?source_id=${encodeURIComponent(source.id)}`)
            }
          >
            {t("Explore available queries")}
          </Button>
          <Button onClick={() => navigate(`/sources/${source.id}/evaluation`)}>
            {t("Evaluate a business question")}
          </Button>
        </div>
      )}
    </>
  );
}
