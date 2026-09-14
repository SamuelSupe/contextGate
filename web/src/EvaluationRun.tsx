import { t } from "./i18n";
import { date } from "./api";
import { Button, CopyButton, Field } from "./components";
import type { EvaluationCapture, EvaluationKind } from "./evaluation-types";

export function EvaluationRun({
  kind,
  run,
  prompt,
  busy,
  canStart,
  onCapture,
  onReview,
}: {
  kind: EvaluationKind;
  run?: EvaluationCapture;
  prompt: string;
  busy: boolean;
  canStart: boolean;
  onCapture: (
    kind: EvaluationKind,
    action: "start" | "collect" | "abandon",
  ) => void;
  onReview: (
    kind: EvaluationKind,
    patch: { verdict?: string; notes?: string },
  ) => void;
}) {
  return (
    <section className="evaluation-run">
      <h2>
        {kind === "baseline"
          ? t("1. Baseline")
          : t("2. With semantic guidance")}
      </h2>
      <p className="help">
        {run
          ? t("Started {value1} · {state}", {
              value1: date(run.started),
              state: t(run.state),
            })
          : t(
              "Start immediately before asking the question in a fresh client conversation.",
            )}
      </p>
      <div className="button-row">
        <span className="help">
          {kind === "baseline" ? t("Baseline prompt") : t("Guided prompt")}
        </span>
        <CopyButton text={prompt} />
      </div>
      {!run ? (
        <Button
          primary
          disabled={!canStart || busy}
          onClick={() => onCapture(kind, "start")}
        >
          {t("Start ")}
          {t(kind)}
          {t(" capture")}
        </Button>
      ) : run.state === "capturing" ? (
        <>
          <p className="help">
            {t(
              "Capture start is saved. Finish all client calls before collecting. Return through History if you leave this page.",
            )}
          </p>
          <div className="button-row">
            <Button
              primary
              busy={busy}
              onClick={() => onCapture(kind, "collect")}
            >
              {t("Collect completed calls")}
            </Button>
            <Button disabled={busy} onClick={() => onCapture(kind, "abandon")}>
              {t("Abandon capture")}
            </Button>
          </div>
        </>
      ) : run.state === "abandoned" ? (
        <p className="help">
          {t(
            "No metrics collected. Use Repeat evaluation for another attempt.",
          )}
        </p>
      ) : null}
      {run?.stats && (
        <>
          {run.configuration_changed && (
            <div className="notice warning">
              {t(
                "Source configuration or Agent grants changed during this run. Repeat under stable conditions.",
              )}
            </div>
          )}
          <dl>
            <dt>{t("Audited calls / queries")}</dt>
            <dd>
              {run.stats.calls} / {run.stats.queries}
            </dd>
            <dt>{t("Successful queries / errors")}</dt>
            <dd>
              {run.stats.successful_queries} / {run.stats.errors}
            </dd>
            <dt>{t("Template calls")}</dt>
            <dd>{run.stats.template_calls}</dd>
            <dt>{t("Sum of ContextGate call durations")}</dt>
            <dd>
              {run.stats.elapsed_ms}
              {t(" ms")}
            </dd>
          </dl>
          {!run.stats.queries && (
            <div className="notice warning">
              {t(
                "No query calls captured. This does not establish a successful business query.",
              )}
            </div>
          )}
          <p className="help">
            {t("Source publication ")}
            {run.configuration.published_version}
            {t(" · Agent revision ")}
            {run.configuration.agent_revision}
            {run.configuration.ontology_id
              ? t(" · ontology version {ontology_version}", {
                  ontology_version: run.configuration.ontology_version,
                })
              : ""}
          </p>
          <Field label={t("{kind} answer assessment", { kind: t(kind) })}>
            <select
              disabled={busy}
              value={run.verdict}
              onChange={(e) => onReview(kind, { verdict: e.target.value })}
            >
              <option value="unrated">{t("Not reviewed")}</option>
              <option value="correct">{t("Correct")}</option>
              <option value="partial">{t("Partially correct")}</option>
              <option value="incorrect">{t("Incorrect")}</option>
            </select>
          </Field>
          <Field label={t("{kind} review notes", { kind: t(kind) })}>
            <textarea
              disabled={busy}
              rows={3}
              maxLength={4000}
              value={run.notes}
              onChange={(e) => onReview(kind, { notes: e.target.value })}
            />
          </Field>
        </>
      )}
    </section>
  );
}
