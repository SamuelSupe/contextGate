import { useEffect, useState } from "react";
import { ArrowRight, Search } from "lucide-react";
import { api, date, message } from "./api";
import { Button, Empty, ErrorNote, Field, Loading } from "./components";
import { t } from "./i18n";
import { HelpTip } from "./HelpTip";
import { healthStatusLabels } from "./Health";
import { useNavigationGuard } from "./useNavigationGuard";
import {
  queryToolURL,
  savedJourneys,
  queryNextStep,
  queryStepLabels,
  type QueryJourney,
} from "./query-publishing";
import { SourceEditor } from "./SourceEditor";
import { RecentQueries } from "./RecentQueries";
import { useReadiness } from "./readiness";
import type { Agent, Capability, Source } from "./types";
import type { SemanticState } from "./semantic-types";
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
  administratorID,
}: {
  administratorID: string;
  sources: Source[];
  catalog: Capability[];
  navigate: (url: string) => void;
  reload: () => Promise<void>;
  notify: (text: string) => void;
}) {
  const journeys = savedJourneys(administratorID).filter((j) =>
    sources.some((s) => s.id === j.source_id),
  );
  const lastJourney = journeys[0];
  const [sourceID, setSourceID] = useState(lastJourney?.source_id || "");
  const [keyword, setKeyword] = useState("");
  const [adding, setAdding] = useState(false);
  const [health, setHealth] = useState<HealthSummary | null>(null);
  const [error, setError] = useState("");
  const [refresh, setRefresh] = useState(0);
  const [resumePending, setResumePending] = useState(false);
  const [checking, setChecking] = useState("");
  useNavigationGuard(false, !!checking);
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
  async function checkSource(id: string) {
    setChecking(id);
    setError("");
    try {
      await api(`/api/health/${id}/check`, { method: "POST", body: "{}" });
      await reload();
      setRefresh((n) => n + 1);
    } catch (e) {
      setError(message(e));
    } finally {
      setChecking("");
    }
  }
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Home")}</h1>
        </div>
        {sources.length > 0 && (
          <Button onClick={() => navigate(queryToolURL())}>
            {t("New query")}
          </Button>
        )}
      </div>
      {lastJourney && sources.some((s) => s.id === lastJourney.source_id) && (
        <ResumeQuery
          key={lastJourney.source_id + lastJourney.template_id}
          journey={lastJourney}
          source={sources.find((s) => s.id === lastJourney.source_id)!}
          navigate={navigate}
          onPending={setResumePending}
        />
      )}
      <RecentQueries
        journeys={journeys.slice(1)}
        sources={sources}
        navigate={navigate}
      />
      {sources.length > 0 && (
        <section className="business-search-banner">
          <h2>{t("Find a query")}</h2>

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
            <Button primary={!resumePending} type="submit">
              {t("Find query tools")}
              <ArrowRight size={16} />
            </Button>
          </form>
        </section>
      )}
      <div className="home-columns">
        <section className="business-panel">
          <div className="section-heading">
            <h2>{t("Source overview")}</h2>
          </div>
          {source ? (
            <>
              <Field label={t("Data source")}>
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
              description={t("Connect a source and publish your first query.")}
              action={
                <Button primary onClick={() => navigate(queryToolURL())}>
                  {t("Publish your first query")}
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
                          ? t("{count} queries need validation", {
                              count: s.invalid_templates.length,
                            })
                          : t(
                              healthStatusLabels[s.status] ||
                                "Review connection and structure evidence",
                            )}
                      </small>
                    </div>
                    <Button
                      busy={checking === s.id}
                      disabled={!!checking}
                      onClick={() => {
                        if (s.invalid_templates.length)
                          navigate(
                            `/business?view=attention&source_id=${encodeURIComponent(s.id)}`,
                          );
                        else if (
                          ["not_checked", "overdue", "stale"].includes(s.status)
                        )
                          void checkSource(s.id);
                        else
                          navigate(
                            s.status === "connection_failed"
                              ? `/sources/${s.id}/setup`
                              : "/health",
                          );
                      }}
                    >
                      {t(
                        s.invalid_templates.length
                          ? "Review queries"
                          : ["not_checked", "overdue", "stale"].includes(
                                s.status,
                              )
                            ? "Check now"
                            : s.status === "connection_failed"
                              ? "Review connection"
                              : "Review structure",
                      )}
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
                  <p className="help">{t("No items need attention.")}</p>
                )}
              <Button onClick={() => navigate("/health")}>
                {t("View all health checks")}
                <ArrowRight size={15} />
              </Button>
            </>
          ) : null}
        </section>
      </div>
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
  return (
    <>
      <div className="query-status-grid home-source-metrics">
        <div>
          <div className="label-with-help metric-label">
            <small>{t("Publication")}</small>
          </div>
          <strong>
            {t("{count} published queries", { count: data.templates.length })}
          </strong>
          <span>
            {t("{count} currently executable", {
              count: data.executable_templates,
            })}
          </span>
        </div>
        <div>
          <div className="label-with-help metric-label">
            <small>{t("Client activity")}</small>
            <HelpTip title={t("Client activity")}>
              <p>
                {t(
                  "Successful Agent calls on this source in the last 30 days, including native queries and query tools. Previews are excluded.",
                )}
              </p>
              <p>
                {t(
                  "Client activity does not confirm business answer correctness.",
                )}
              </p>
              <p>
                {t("Evidence window: {from} to {until}", {
                  from: date(data.activity_since),
                  until: date(data.checked_at),
                })}
              </p>
            </HelpTip>
          </div>
          <strong>
            {t("{count} successful calls in 30 days", {
              count: data.client_queries,
            })}
          </strong>
          <span>
            {data.last_query
              ? t("Last call: {time}", { time: date(data.last_query) })
              : t("No client query observed in this window")}
          </span>
        </div>
      </div>

      <div className="button-row">
        <Button
          onClick={() =>
            navigate(
              `/business?${new URLSearchParams({ view: "queries", source_id: source.id })}`,
            )
          }
        >
          {t("Open queries")}
        </Button>
        <Button onClick={() => navigate(`/sources/${source.id}/evaluation`)}>
          {t("Review business questions")}
        </Button>
        <Button onClick={() => navigate(`/sources/${source.id}/setup`)}>
          {t("Connection and access")}
        </Button>
      </div>
    </>
  );
}

function ResumeQuery({
  journey,
  source,
  navigate,
  onPending,
}: {
  journey: QueryJourney;
  source: Source;
  navigate: (url: string) => void;
  onPending: (pending: boolean) => void;
}) {
  const [state, setState] = useState<SemanticState | null>(null);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const entry =
    state?.draft.entries.find(
      (e) => e.id === journey.template_id && e.template,
    ) ||
    state?.published.entries.find(
      (e) => e.id === journey.template_id && e.template,
    );
  const ready = useReadiness(
    entry ? source.id : "",
    `${source.revision}:${state?.revision}:${retry}`,
    journey.template_id,
    journey.agent_id,
  );
  useEffect(() => {
    const abort = new AbortController();
    setError("");
    api<SemanticState>(
      `/api/sources/${encodeURIComponent(source.id)}/semantics`,
      { signal: abort.signal },
    )
      .then(setState)
      .catch((e) => {
        if (!abort.signal.aborted) setError(message(e));
      });
    return () => abort.abort();
  }, [source.id, retry]);
  const next = state
    ? queryNextStep(state, ready.data, journey.template_id, journey.agent_id)
    : "loading";
  const missing = !!state && !!journey.template_id && !entry;
  const pending =
    next !== "done" && next !== "loading" && !error && !ready.error;
  useEffect(() => {
    onPending(pending);
    return () => onPending(false);
  }, [pending, onPending]);
  return (
    <section className="home-resume">
      <div className="section-heading">
        <div>
          <h2>{t(next === "done" ? "Recent query" : "Continue last setup")}</h2>
          <strong>{entry?.name || source.name}</strong>
          <span className="help">
            {entry ? `${source.name} · ` : ""}
            {missing
              ? t("Query no longer available")
              : error || ready.error
                ? t("Status unavailable")
                : !source.enabled
                  ? t("Source disabled")
                  : t(queryStepLabels[next])}
          </span>
        </div>
        {(error || ready.error) && !missing ? (
          <Button
            primary={pending}
            onClick={() => {
              setRetry((v) => v + 1);
              ready.refresh();
            }}
          >
            {t("Retry")}
          </Button>
        ) : (
          <Button
            primary={pending}
            disabled={next === "loading"}
            onClick={() =>
              navigate(
                queryToolURL(
                  source.id,
                  missing ? "" : journey.template_id,
                  journey.agent_id,
                ),
              )
            }
          >
            {t(
              next === "done"
                ? "Open query"
                : missing
                  ? "Choose another query"
                  : "Continue",
            )}
          </Button>
        )}
      </div>
    </section>
  );
}
