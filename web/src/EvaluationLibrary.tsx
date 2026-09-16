import type { EvaluationKind } from "./evaluation-types";
import { t } from "./i18n";
import { HelpTip } from "./HelpTip";
import { useEffect, useState } from "react";
import { api, date, message, payload } from "./api";
import { Button, Empty, ErrorNote, Field, Loading } from "./components";
import { downloadJSON } from "./ontology-types";
import {
  matchingConditions,
  type Evaluation,
  type SavedQuestion,
} from "./evaluation-types";

export interface EvaluationLibraryState {
  filters: { search: string; agent: string; from: string; until: string };
  applied: string;
  cursor: string;
  pages: string[];
}
export const emptyLibraryState: EvaluationLibraryState = {
  filters: { search: "", agent: "", from: "", until: "" },
  applied: "",
  cursor: "",
  pages: [],
};

export function EvaluationLibrary({
  sourceID,
  questions,
  busy,
  onBusy,
  onUse,
  onOpen,
  state,
  onState,
}: {
  sourceID: string;
  questions: boolean;
  busy: boolean;
  onBusy: (value: boolean) => void;
  onUse: (value: SavedQuestion) => void;
  onOpen: (value: Evaluation) => void;
  state: EvaluationLibraryState;
  onState: (patch: Partial<EvaluationLibraryState>) => void;
}) {
  const [items, setItems] = useState<(SavedQuestion | Evaluation)[]>([]);
  const { cursor, pages, applied } = state;
  const [next, setNext] = useState("");
  const [refresh, setRefresh] = useState(0);
  const [loading, setLoading] = useState(true),
    [error, setError] = useState("");
  const [deleting, setDeleting] = useState<SavedQuestion | Evaluation | null>(
    null,
  );
  const [filters, setFilters] = useState(state.filters);
  const [summary, setSummary] = useState<{
    total: number;
    single_checks: number;
    completed_singles: number;
    reviewed_singles: number;
    single_correct: number;
    completed_pairs: number;
    reviewed_pairs: number;
    changed_pairs: number;
    baseline_correct: number;
    guided_correct: number;
  } | null>(null);
  const [agentOptions, setAgentOptions] = useState<
    { id: string; name: string }[]
  >([]);
  const endpoint = `/api/sources/${sourceID}/evaluation/${questions ? "questions" : "history"}`;
  useEffect(() => {
    const abort = new AbortController();
    setLoading(true);
    setError("");
    api<{
      items: (SavedQuestion | Evaluation)[];
      next_cursor: string;
      summary: NonNullable<typeof summary>;
      agents: typeof agentOptions;
    }>(`${endpoint}?${new URLSearchParams({ cursor })}&${applied}`, {
      signal: abort.signal,
    })
      .then((v) => {
        setItems(v.items);
        setNext(v.next_cursor);
        setSummary(v.summary);
        setAgentOptions(v.agents);
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!abort.signal.aborted) setLoading(false);
      });
    return () => abort.abort();
  }, [endpoint, cursor, refresh, applied]);
  function applyFilters(reset = false) {
    const query = new URLSearchParams();
    if (!reset) {
      if (filters.search.trim()) query.set("search", filters.search.trim());
      if (!questions && filters.agent) query.set("agent", filters.agent);
      for (const key of ["from", "until"] as const) {
        if (!questions && filters[key]) {
          const date = new Date(filters[key] + "T00:00:00");
          if (key === "until") date.setDate(date.getDate() + 1);
          query.set(key, date.toISOString());
        }
      }
    } else setFilters({ search: "", agent: "", from: "", until: "" });
    onState({
      applied: query.toString(),
      cursor: "",
      pages: [],
      filters: reset ? emptyLibraryState.filters : filters,
    });
    setDeleting(null);
    setRefresh((v) => v + 1);
  }
  async function remove() {
    if (!deleting) return;
    onBusy(true);
    setError("");
    try {
      await api(`${endpoint}/${deleting.id}`, {
        method: "DELETE",
        body: payload({ revision: deleting.revision }),
      });
      setDeleting(null);
      setRefresh((v) => v + 1);
    } catch (e) {
      setError(message(e));
    } finally {
      onBusy(false);
    }
  }
  return (
    <section className="evaluation-library">
      <div className="page-header">
        <div>
          <h2>{questions ? t("Saved questions") : t("Evaluation history")}</h2>
          <p>
            {questions
              ? t(
                  "Reuse questions and criteria. Editing a question preserves past evaluation snapshots.",
                )
              : t(
                  "Find past runs, resume captures and compare manual reviews.",
                )}
          </p>
        </div>
        <Button
          disabled={busy || loading}
          onClick={() => setRefresh((v) => v + 1)}
        >
          {t("Refresh list")}
        </Button>
      </div>
      <form
        className="history-filters"
        onSubmit={(e) => {
          e.preventDefault();
          applyFilters();
        }}
      >
        <Field label={t("Find a question")}>
          <input
            type="search"
            maxLength={120}
            disabled={busy}
            placeholder={t("Question name or text")}
            value={filters.search}
            onChange={(e) => setFilters({ ...filters, search: e.target.value })}
          />
        </Field>
        {!questions && (
          <>
            <Field label={t("Agent")}>
              <select
                disabled={busy}
                value={filters.agent}
                onChange={(e) =>
                  setFilters({ ...filters, agent: e.target.value })
                }
              >
                <option value="">{t("All Agents")}</option>
                {agentOptions.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={t("Created from")}>
              <input
                type="date"
                disabled={busy}
                max={filters.until || undefined}
                value={filters.from}
                onChange={(e) =>
                  setFilters({ ...filters, from: e.target.value })
                }
              />
            </Field>
            <Field label={t("Through")}>
              <input
                type="date"
                disabled={busy}
                min={filters.from || undefined}
                value={filters.until}
                onChange={(e) =>
                  setFilters({ ...filters, until: e.target.value })
                }
              />
            </Field>
          </>
        )}
        <div className="button-row">
          <Button type="submit" primary disabled={busy || loading}>
            {t("Apply filters")}
          </Button>
          {(applied || Object.values(filters).some(Boolean)) && (
            <Button
              disabled={busy || loading}
              onClick={() => applyFilters(true)}
            >
              {t("Clear filters")}
            </Button>
          )}
        </div>
      </form>
      <ErrorNote error={error} />
      {!loading && !error && summary && (
        <>
          <div className="history-summary" role="status">
            <span>
              {questions
                ? t("{count} matching questions", { count: summary.total })
                : t("{count} matching evaluations", { count: summary.total })}
            </span>
            {!questions && summary.single_checks > 0 && (
              <>
                <span>
                  {t("{count} completed single checks", {
                    count: summary.completed_singles,
                  })}
                </span>
                <span>
                  {t(
                    "{count} reviewed single checks with unchanged configuration",
                    { count: summary.reviewed_singles },
                  )}
                </span>
              </>
            )}
            {!questions && summary.total > summary.single_checks && (
              <>
                <span>
                  <strong>{summary.completed_pairs}</strong>
                  {t(" completed pairs")}
                </span>
                <span>
                  <strong>{summary.reviewed_pairs}</strong>
                  {t(" reviewed pairs with unchanged configuration")}
                </span>
              </>
            )}
          </div>
          {!questions && summary.single_checks > 0 && (
            <p className="help">
              {t(
                "Single answer checks: {correct} of {reviewed} reviewed answers marked correct. Includes completed captures with query calls and unchanged configuration; all matching pages are counted.",
                {
                  correct: summary.single_correct,
                  reviewed: summary.reviewed_singles,
                },
              )}
            </p>
          )}
          {!questions && summary.total > summary.single_checks && (
            <div className="evaluation-context">
              <div className="label-with-help">
                <h3>{t("Review summary across all matching history")}</h3>
                <HelpTip
                  title={t("Review summary across all matching history")}
                >
                  <p>
                    {t(
                      "Only completed pairs with queries in both runs, two reviews and unchanged source / Agent configuration are included.",
                    )}
                  </p>
                  <p>
                    {summary.changed_pairs}
                    {t(
                      " completed pairs had configuration changes and are excluded. Counts cover every matching page. Different questions, models and data may still affect the comparison; these counts do not measure model improvement automatically.",
                    )}
                  </p>
                </HelpTip>
              </div>
              <p>
                {t("Correct, based on manual reviews: baseline")}{" "}
                <strong>
                  {summary.baseline_correct} / {summary.reviewed_pairs}
                </strong>{" "}
                {t("· guided")}{" "}
                <strong>
                  {summary.guided_correct} / {summary.reviewed_pairs}
                </strong>
                .
              </p>
            </div>
          )}
        </>
      )}
      {deleting && (
        <div className="notice warning">
          <p>
            {t("Delete “")}
            {deleting.name}”?{" "}
            {questions
              ? t("Existing history keeps its question snapshot.")
              : t("Export first to keep this evaluation.")}
          </p>
          <div className="button-row">
            <Button disabled={busy} onClick={() => setDeleting(null)}>
              {t("Keep record")}
            </Button>
            <Button busy={busy} onClick={remove}>
              {t("Delete record")}
            </Button>
          </div>
        </div>
      )}
      {loading ? (
        <Loading />
      ) : error ? null : !items.length ? (
        <Empty
          title={
            applied
              ? t("No matches")
              : questions
                ? t("No saved questions")
                : t("No evaluations on this page")
          }
          description={
            applied
              ? t("Try a broader question search or clear the filters.")
              : questions
                ? t(
                    "Open New evaluation, enter a business question, and save it for reuse.",
                  )
                : t("Start a capture to create a durable evaluation record.")
          }
          action={
            applied ? (
              <Button onClick={() => applyFilters(true)}>
                {t("Clear filters")}
              </Button>
            ) : undefined
          }
        />
      ) : (
        <ul className="workflow-items">
          {items.map((item) => {
            const evaluation = !questions ? (item as Evaluation) : null;
            const active =
              evaluation &&
              Object.values(evaluation.runs).some(
                (r) => r.state === "capturing",
              );
            return (
              <li key={item.id}>
                <div>
                  <strong>{item.name}</strong>
                  <small>
                    {date(item.created_at)} ·{" "}
                    {questions
                      ? t("Question revision {revision}", {
                          revision: item.revision,
                        })
                      : `${evaluation!.agent_name} · ${evaluation!.client || t("Client not recorded")}`}
                  </small>
                  <p className="help">
                    {item.question.length > 180
                      ? item.question.slice(0, 180) + "…"
                      : item.question}
                  </p>
                  {evaluation && (
                    <>
                      <small>
                        {evaluation.case_id
                          ? t("Saved question revision {case_revision}", {
                              case_revision: evaluation.case_revision,
                            })
                          : t("Ad-hoc question")}
                      </small>
                      {(
                        (evaluation.mode === "single"
                          ? ["guided"]
                          : ["baseline", "guided"]) as EvaluationKind[]
                      ).map((kind) => {
                        const r = evaluation.runs[kind];
                        return (
                          <small key={kind}>
                            {kind === "baseline" ? t("Baseline") : t("Guided")}:{" "}
                            {r
                              ? t(
                                  "{state} · {verdict}{value3} · source publication {published_version}",
                                  {
                                    state: t(r.state),
                                    verdict: t(r.verdict),
                                    value3: r.stats
                                      ? t(
                                          " · {queries} queries / {errors} errors",
                                          {
                                            queries: r.stats.queries,
                                            errors: r.stats.errors,
                                          },
                                        )
                                      : "",
                                    published_version:
                                      r.configuration.published_version,
                                  },
                                )
                              : t("Not started")}
                          </small>
                        );
                      })}
                      {evaluation.runs.baseline &&
                        evaluation.runs.guided &&
                        !matchingConditions(evaluation) && (
                          <small>
                            {t("Conditions changed; compare with care.")}
                          </small>
                        )}
                    </>
                  )}
                </div>
                <div className="button-row">
                  <Button
                    primary
                    disabled={busy}
                    onClick={() =>
                      questions
                        ? onUse(item as SavedQuestion)
                        : onOpen(item as Evaluation)
                    }
                  >
                    {questions
                      ? t("Use question")
                      : active
                        ? t("Resume capture")
                        : t("Open evaluation")}
                  </Button>
                  <Button
                    disabled={busy}
                    onClick={() =>
                      downloadJSON(
                        { format_version: questions ? 1 : 2, ...item },
                        questions
                          ? "evaluation-question.json"
                          : "query-evaluation.json",
                      )
                    }
                  >
                    {t("Export JSON")}
                  </Button>
                  <Button
                    disabled={busy || !!active}
                    onClick={() => setDeleting(item)}
                  >
                    {t("Delete")}
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
      <div className="pagination">
        <Button
          disabled={loading || busy || !pages.length}
          onClick={() => {
            onState({
              cursor: pages[pages.length - 1],
              pages: pages.slice(0, -1),
            });
          }}
        >
          {t("Previous")}
        </Button>
        <span>
          {t("Page ")}
          {pages.length + 1}
        </span>
        <Button
          disabled={loading || busy || !next}
          onClick={() => {
            onState({ pages: [...pages, cursor], cursor: next });
          }}
        >
          {t("Next")}
        </Button>
      </div>
    </section>
  );
}
