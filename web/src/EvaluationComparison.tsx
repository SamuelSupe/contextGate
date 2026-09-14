import { t } from "./i18n";
import { date } from "./api";
import {
  matchingConditions,
  type Evaluation,
  type EvaluationCapture,
} from "./evaluation-types";

const verdicts: Record<string, string> = {
  correct: "Correct",
  partial: "Partially correct",
  incorrect: "Incorrect",
  unrated: "Not reviewed",
};
export function EvaluationComparison({ value }: { value: Evaluation }) {
  const { baseline, guided } = value.runs;
  const complete =
    baseline?.state === "completed" && guided?.state === "completed";
  const metrics: {
    label: string;
    read: (run: EvaluationCapture) => string | number | undefined;
  }[] = [
    {
      label: "Answer · manual review",
      read: (r) =>
        r.state === "completed"
          ? t(verdicts[r.verdict] || "Not reviewed")
          : t("Pending"),
    },
    { label: "Query calls", read: (r) => r.stats?.queries },
    { label: "Successful queries", read: (r) => r.stats?.successful_queries },
    { label: "Errors · all calls", read: (r) => r.stats?.errors },
    { label: "Template calls", read: (r) => r.stats?.template_calls },
    {
      label: "ContextGate call duration · total",
      read: (r) => (r.stats ? `${r.stats.elapsed_ms} ms` : undefined),
    },
  ];
  return (
    <section className="evaluation-comparison">
      <h2>{value.name}</h2>
      <p className="help">
        {value.agent_name} · {date(value.created_at)} ·{" "}
        {complete ? t("Both captures completed") : t("Evaluation in progress")}
      </p>
      <div className="table-scroll">
        <table>
          <caption className="sr-only">
            {t("Baseline and guided results")}
          </caption>
          <thead>
            <tr>
              <th scope="col">{t("Measure")}</th>
              <th scope="col">{t("Baseline")}</th>
              <th scope="col">{t("With semantic guidance")}</th>
            </tr>
          </thead>
          <tbody>
            {metrics.map((metric) => (
              <tr key={t(metric.label)}>
                <th scope="row">{t(metric.label)}</th>
                {[baseline, guided].map((run, i) => (
                  <td key={i}>
                    {run ? (metric.read(run) ?? "—") : t("Not started")}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {complete && !matchingConditions(value) ? (
        <div className="notice warning">
          {t(
            "Source configuration or Agent grants changed. Repeat under stable conditions before comparing results.",
          )}
        </div>
      ) : (
        <p className="help">
          {complete
            ? t(
                "Call durations exclude model and network time. Answer correctness is your manual assessment.",
              )
            : t(
                "Complete both captures, then review the answers against your acceptance criteria.",
              )}
        </p>
      )}
      <details className="evaluation-context">
        <summary>{t("Question, expected answer and test conditions")}</summary>
        <dl className="evaluation-snapshot">
          <dt>{t("Business question")}</dt>
          <dd>{value.question}</dd>
          <dt>{t("Expected answer / acceptance criteria")}</dt>
          <dd>{value.criteria}</dd>
          <dt>{t("Client, model and settings")}</dt>
          <dd>{value.client || t("Not recorded")}</dd>
          <dt>{t("Question snapshot")}</dt>
          <dd>
            {value.case_id
              ? t("Saved question · revision {case_revision}", {
                  case_revision: value.case_revision,
                })
              : t("Ad-hoc question")}
          </dd>
        </dl>
        <p className="help">
          {t(
            "Inputs are fixed for this evaluation. Repeat it to use different inputs. Keep a dedicated Agent, stable data and model settings, and fresh client conversations.",
          )}
        </p>
      </details>
    </section>
  );
}
