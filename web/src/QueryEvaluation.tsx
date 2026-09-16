import { t } from "./i18n";
import { HelpTip } from "./HelpTip";
import { useEffect, useRef, useState } from "react";
import { api, message, payload } from "./api";
import { Button, ErrorNote, Field } from "./components";
import { downloadJSON } from "./ontology-types";
import { useNavigationGuard } from "./useNavigationGuard";
import { EvaluationLibrary, emptyLibraryState } from "./EvaluationLibrary";
import { EvaluationComparison } from "./EvaluationComparison";
import { EvaluationRun } from "./EvaluationRun";
import {
  type Evaluation,
  type EvaluationForm,
  type EvaluationKind,
  type SavedQuestion,
} from "./evaluation-types";
import type { Agent, Source } from "./types";
import "./product-workflows.css";

const emptyForm: EvaluationForm = {
  name: "",
  question: "",
  criteria: "",
  client: "",
};
const tabs = ["Evaluation", "Saved questions", "History"] as const;
export function QueryEvaluation({
  source,
  agents,
  navigate,
  notify,
}: {
  source: Source;
  agents: Agent[];
  navigate: (url: string) => void;
  notify: (text: string) => void;
}) {
  const [mode, setMode] = useState<"single" | "comparison">("single");
  const [tab, setTab] = useState<(typeof tabs)[number]>("Evaluation");
  const [library, setLibrary] = useState({
    questions: emptyLibraryState,
    history: emptyLibraryState,
  });
  const [form, setForm] = useState<EvaluationForm>(emptyForm);
  const [savedQuestion, setSavedQuestion] = useState<SavedQuestion | null>(
    null,
  );
  const [record, setRecord] = useState<Evaluation | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    window.scrollTo(0, 0);
    heading.current?.focus({ preventScroll: true });
  }, [tab, record?.id]);
  const [agentID, setAgentID] = useState("");
  const [busy, setBusy] = useState(false),
    [dirty, setDirty] = useState(false),
    [reviewDirty, setReviewDirty] = useState(false),
    [error, setError] = useState("");
  const [confirmClear, setConfirmClear] = useState<"new" | "review" | null>(
    null,
  );
  useNavigationGuard(dirty || reviewDirty, busy);
  const candidates = agents.filter(
    (a) =>
      a.enabled &&
      !a.revoked_at &&
      new Date(a.expires_at) > new Date() &&
      a.sources.includes(source.id),
  );
  const agent =
    record?.agent_id ||
    candidates.find((a) => a.id === agentID)?.id ||
    candidates[0]?.id ||
    "";
  const single = record ? record.mode === "single" : mode === "single";
  const runKinds: EvaluationKind[] = single
    ? ["guided"]
    : ["baseline", "guided"];
  const completed =
    !!record &&
    runKinds.every((kind) => record.runs[kind]?.state === "completed");
  const active =
    record && Object.values(record.runs).some((r) => r.state === "capturing");
  const caseChanged =
    !!savedQuestion &&
    (form.question !== savedQuestion.question ||
      form.criteria !== savedQuestion.criteria);
  const valid = !!(
    form.name.trim() &&
    form.question.trim() &&
    form.criteria.trim()
  );
  const base = `/api/sources/${source.id}/evaluation`;

  function change(key: keyof EvaluationForm, value: string) {
    setForm((v) => ({ ...v, [key]: value }));
    setDirty(true);
  }
  function selectTab(next: typeof tab) {
    if (dirty || reviewDirty || busy) {
      setError(t("Save or discard your edits before switching views."));
      return;
    }
    setError("");
    setTab(next);
  }
  function useQuestion(value: SavedQuestion) {
    setSavedQuestion(value);
    setForm(value);
    setRecord(null);
    setDirty(false);
    setReviewDirty(false);
    setError("");
    setTab("Evaluation");
  }
  async function openEvaluation(value: Evaluation) {
    setBusy(true);
    setError("");
    try {
      const current = await api<Evaluation>(`${base}/history/${value.id}`);
      setRecord(current);
      setForm(current);
      setSavedQuestion(null);
      setReviewDirty(false);
      setDirty(false);
      setTab("Evaluation");
      setConfirmClear(null);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function saveQuestion(asNew = false) {
    setBusy(true);
    setError("");
    try {
      const prior = asNew ? null : savedQuestion;
      const body = {
        name: form.name,
        question: form.question,
        criteria: form.criteria,
        client: form.client,
        revision: prior?.revision || "0",
      };
      const value = await api<SavedQuestion>(
        `${base}/questions${prior ? `/${prior.id}` : ""}`,
        { method: prior ? "PUT" : "POST", body: payload(body) },
      );
      setSavedQuestion(value);
      setDirty(false);
      notify(t("Question saved"));
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function capture(
    kind: EvaluationKind,
    action: "start" | "collect" | "abandon",
  ) {
    setBusy(true);
    setError("");
    try {
      const value = record
        ? await api<Evaluation>(`${base}/history/${record.id}/capture`, {
            method: "POST",
            body: payload({ revision: record.revision, kind, action }),
          })
        : await api<Evaluation>(`${base}/history`, {
            method: "POST",
            body: payload({
              name: form.name,
              question: form.question,
              criteria: form.criteria,
              client: form.client,
              agent_id: agent,
              kind,
              mode,
              case_id: savedQuestion?.id || "",
              case_revision: savedQuestion?.revision || "0",
            }),
          });
      setRecord(value);
      setForm(value);
      setAgentID(value.agent_id);
      setDirty(false);
      setReviewDirty(false);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function saveReview() {
    if (!record) return;
    setBusy(true);
    setError("");
    try {
      const reviews = Object.fromEntries(
        Object.entries(record.runs)
          .filter(([, r]) => r.state === "completed")
          .map(([k, r]) => [k, { verdict: r.verdict, notes: r.notes }]),
      );
      const value = await api<Evaluation>(
        `${base}/history/${record.id}/review`,
        {
          method: "PUT",
          body: payload({ revision: record.revision, reviews }),
        },
      );
      setRecord(value);
      setReviewDirty(false);
      notify(t("Review saved"));
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  function reset(repeat = false) {
    if (record && repeat) {
      setMode(record.mode || "comparison");
      setForm({
        name: record.name,
        question: record.question,
        criteria: record.criteria,
        client: record.client,
      });
      setAgentID(record.agent_id);
      setDirty(true);
    } else {
      setForm(emptyForm);
      setDirty(false);
    }
    setRecord(null);
    setSavedQuestion(null);
    setReviewDirty(false);
    setConfirmClear(null);
    setError("");
    setTab("Evaluation");
  }
  const prompt = t(
    "Answer this business question using data source {name} ({id}):\n{question}\nExplain the metric definition and the source used.",
    { name: source.name, id: source.id, question: form.question },
  );
  const guidedPrompt =
    prompt +
    t(
      "\nDiscover the published semantic catalog and mapped concepts, then use an appropriate verified query template when available.",
    );
  return (
    <>
      <Button
        disabled={dirty || reviewDirty || busy}
        onClick={() => navigate(`/sources/${source.id}/setup`)}
      >
        {t("← Query workspace")}
      </Button>
      <div className="page-header">
        <div>
          <div className="label-with-help">
            <h1 ref={heading} tabIndex={-1}>
              {t("Evaluate a business question")}
            </h1>{" "}
            <HelpTip title={t("How measurements and history work")}>
              <p className="help">
                {t(
                  "Avoid concurrent Agent calls. Keep model settings and data stable when comparing answers. ContextGate measures completed calls on this source; review answer correctness manually.",
                )}
              </p>
              <p className="help">
                {t(
                  "Questions and review notes are encrypted on the server. Captures save automatically and survive restarts; save manual reviews explicitly. Metrics exclude previews, query text, parameters and results. Completed measurements remain available after audit retention expires. Finish calls before collecting; capture windows expire after 24 hours. No model correctness, token usage or end-to-end latency is measured automatically.",
                )}
              </p>
            </HelpTip>
          </div>
          <p>{source.name}</p>
        </div>
        <Button
          disabled={busy || !!active}
          onClick={() =>
            dirty || reviewDirty ? setConfirmClear("new") : reset()
          }
        >
          {t("New evaluation")}
        </Button>
      </div>
      <div
        className="semantic-tabs"
        role="tablist"
        aria-label={t("Evaluation views")}
      >
        {tabs.map((section, i) => (
          <button
            key={section}
            role="tab"
            aria-selected={tab === section}
            tabIndex={tab === section ? 0 : -1}
            onClick={() => selectTab(section)}
            onKeyDown={(e) => {
              if (
                !["ArrowLeft", "ArrowRight"].includes(e.key) ||
                dirty ||
                reviewDirty ||
                busy
              )
                return;
              e.preventDefault();
              const n =
                (i + (e.key === "ArrowRight" ? 1 : tabs.length - 1)) %
                tabs.length;
              selectTab(tabs[n]);
              (
                e.currentTarget.parentElement?.children[n] as HTMLButtonElement
              )?.focus();
            }}
          >
            {t(section)}
          </button>
        ))}
      </div>
      <ErrorNote error={error} />
      {confirmClear && (
        <div className="notice warning">
          <p>
            {confirmClear === "review"
              ? t("Discard your unsaved review and reload this evaluation?")
              : t(
                  "Discard unsaved question or review edits? Saved captures and history will remain available.",
                )}
          </p>
          <div className="button-row">
            <Button disabled={busy} onClick={() => setConfirmClear(null)}>
              {t("Keep editing")}
            </Button>
            <Button
              disabled={busy}
              onClick={() =>
                confirmClear === "review" && record
                  ? openEvaluation(record)
                  : reset()
              }
            >
              {t("Discard edits")}
            </Button>
          </div>
        </div>
      )}
      {tab !== "Evaluation" ? (
        <EvaluationLibrary
          key={tab}
          sourceID={source.id}
          questions={tab === "Saved questions"}
          busy={busy}
          onBusy={setBusy}
          onUse={useQuestion}
          onOpen={openEvaluation}
          state={
            tab === "Saved questions" ? library.questions : library.history
          }
          onState={(patch) => {
            const key = tab === "Saved questions" ? "questions" : "history";
            setLibrary((v) => ({ ...v, [key]: { ...v[key], ...patch } }));
          }}
        />
      ) : (
        <>
          {record && (
            <>
              <EvaluationComparison value={record} />
              <div className="button-row">
                <Button
                  disabled={busy || reviewDirty}
                  onClick={() => openEvaluation(record)}
                >
                  {t("Reload saved evaluation")}
                </Button>
                <Button
                  disabled={busy || reviewDirty || !!active}
                  onClick={() => reset(true)}
                >
                  {t("Repeat evaluation")}
                </Button>
              </div>
            </>
          )}
          {!record && (
            <>
              <Field label={t("Evaluation mode")}>
                <select
                  value={mode}
                  disabled={busy}
                  onChange={(e) => setMode(e.target.value as typeof mode)}
                >
                  <option value="single">{t("Single answer check")}</option>
                  <option value="comparison">
                    {t("Compare baseline and semantic guidance")}
                  </option>
                </select>
              </Field>
              <p className="help">
                {t("Use a dedicated Agent and a fresh client conversation.")}
              </p>
              <div className="field-grid">
                <Field label={t("Question name")}>
                  <input
                    disabled={busy}
                    maxLength={120}
                    value={form.name}
                    placeholder={t("e.g. Orders by customer")}
                    onChange={(e) => change("name", e.target.value)}
                  />
                </Field>
                <Field label={t("Evaluation Agent")}>
                  <select
                    disabled={busy}
                    value={agent}
                    onChange={(e) => setAgentID(e.target.value)}
                  >
                    {!agent && (
                      <option value="">
                        {t("No active authorized Agent")}
                      </option>
                    )}
                    {candidates.map((a) => (
                      <option key={a.id} value={a.id}>
                        {a.name}
                      </option>
                    ))}
                  </select>
                </Field>
              </div>
              <Field label={t("Business question")}>
                <textarea
                  disabled={busy}
                  maxLength={4000}
                  rows={3}
                  value={form.question}
                  onChange={(e) => change("question", e.target.value)}
                />
              </Field>
              <Field
                label={t("Expected answer / acceptance criteria")}
                hint={t(
                  "Keep the expected answer separate from the client prompt.",
                )}
              >
                <textarea
                  disabled={busy}
                  maxLength={4000}
                  rows={3}
                  value={form.criteria}
                  onChange={(e) => change("criteria", e.target.value)}
                />
              </Field>
              <Field label={t("Client, model and settings")}>
                <input
                  disabled={busy}
                  maxLength={500}
                  value={form.client}
                  placeholder={t("Client version, model and settings")}
                  onChange={(e) => change("client", e.target.value)}
                />
              </Field>
              {!record && (
                <div className="evaluation-status">
                  <Button
                    disabled={!valid || busy}
                    primary={!!savedQuestion && dirty}
                    onClick={() => saveQuestion()}
                  >
                    {savedQuestion
                      ? t("Save question changes")
                      : t("Save question for reuse")}
                  </Button>
                  {savedQuestion && (
                    <>
                      <span>
                        {t("Question revision ")}
                        {savedQuestion.revision}
                      </span>
                      <Button
                        disabled={!valid || busy}
                        onClick={() => saveQuestion(true)}
                      >
                        {t("Save as new question")}
                      </Button>
                    </>
                  )}
                  {dirty && (
                    <Button
                      disabled={busy}
                      onClick={() => setConfirmClear("new")}
                    >
                      {t("Discard edits")}
                    </Button>
                  )}
                </div>
              )}
            </>
          )}
          {caseChanged && (
            <div className="notice warning">
              {t(
                "Save changes to this question before starting a linked evaluation.",
              )}
            </div>
          )}
          {reviewDirty && (
            <div className="evaluation-status">
              <strong>{t("Unsaved manual review")}</strong>
              <Button primary busy={busy} onClick={saveReview}>
                {t("Save review")}
              </Button>
              <Button disabled={busy} onClick={() => setConfirmClear("review")}>
                {t("Discard edits")}
              </Button>
            </div>
          )}
          <details className="evaluation-details" open={!completed}>
            <summary>
              {completed
                ? t("Edit manual reviews and inspect capture details")
                : t("Capture and review client answers")}
            </summary>
            <p className="help">
              {single
                ? t(
                    "Copy the prompt, start capture, then ask your Agent. Collect the completed calls when the answer is ready.",
                  )
                : t(
                    "Baseline uses your existing workflow. Guided asks for semantic discovery and templates. Both use the same permissions and tools.",
                  )}
            </p>
            <div
              className={single ? "evaluation-runs single" : "evaluation-runs"}
            >
              {runKinds.map((kind) => (
                <EvaluationRun
                  key={kind}
                  kind={kind}
                  single={single}
                  run={record?.runs[kind]}
                  prompt={kind === "baseline" ? prompt : guidedPrompt}
                  busy={busy}
                  canStart={
                    valid &&
                    !caseChanged &&
                    !!agent &&
                    candidates.some((a) => a.id === agent) &&
                    !active &&
                    !reviewDirty
                  }
                  onCapture={(kind, action) => {
                    if (reviewDirty) {
                      setError(
                        t(
                          "Save your review before collecting another capture.",
                        ),
                      );
                      return;
                    }
                    void capture(kind, action);
                  }}
                  onReview={(kind, patch) => {
                    setRecord((v) =>
                      v
                        ? {
                            ...v,
                            runs: {
                              ...v.runs,
                              [kind]: { ...v.runs[kind]!, ...patch },
                            },
                          }
                        : v,
                    );
                    setReviewDirty(true);
                  }}
                />
              ))}
            </div>
          </details>
          <div className="button-row">
            <Button
              disabled={!record || busy || reviewDirty}
              onClick={() =>
                downloadJSON(
                  { format_version: 2, ...record },
                  "query-evaluation.json",
                )
              }
            >
              {t("Export evaluation JSON")}
            </Button>
            {record && !active && !reviewDirty && (
              <Button onClick={() => selectTab("History")}>
                {t("View history")}
              </Button>
            )}
          </div>
        </>
      )}
    </>
  );
}
