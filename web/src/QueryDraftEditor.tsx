import { useEffect, useRef, useState } from "react";
import { api, APIError, message, payload } from "./api";
import { Button, ErrorNote, Field } from "./components";
import { NativeQueryEditor } from "./NativeQueryEditor";
import { ParameterFields } from "./SemanticEditor";
import { TemplateParameters } from "./TemplateParameters";
import { RegressionEditor } from "./RegressionEditor";
import { httpTemplate } from "./http-template";
import { nativeTemplateQuery, suggestedParameters } from "./query-publishing";
import {
  parameterJSON,
  parameterValues,
  validateParameters,
} from "./template-parameters";
import { useNavigationGuard } from "./useNavigationGuard";
import { t } from "./i18n";
import type { Source } from "./types";
import type { SemanticEntry, SemanticState } from "./semantic-types";
import type { OntologyBinding } from "./ontology-types";
import { BusinessConcepts } from "./BusinessConcepts";

export function QueryDraftEditor({
  source,
  entry,
  revision,
  ontology,
  onSaved,
  onCancel,
  onStep,
  creating = false,
  initialSection = "query",
}: {
  creating?: boolean;
  initialSection?: string;
  source: Source;
  entry: SemanticEntry;
  revision: string;
  ontology?: OntologyBinding | null;
  onSaved: (state: SemanticState, entryID: string, finish: boolean) => void;
  onCancel: () => void;
  onStep: (step: number) => void;
}) {
  const [form, setForm] = useState(() => structuredClone(entry));
  const [baseline, setBaseline] = useState(() => JSON.stringify(entry));
  const [expected, setExpected] = useState(revision);
  const [binding, setBinding] = useState(ontology);
  const [step, setStep] = useState(
    initialSection === "purpose" ? 2 : initialSection === "parameters" ? 3 : 1,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [latest, setLatest] = useState<SemanticState | null>(null);
  const [bindings, setBindings] = useState<{
    query: string;
    slots: string[];
  } | null>(null);
  const clearGuard = useNavigationGuard(
    JSON.stringify(form) !== baseline,
    busy,
  );
  const errorArea = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (error) errorArea.current?.scrollIntoView({ block: "center" });
  }, [error]);
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    heading.current?.focus({ preventScroll: true });
    heading.current?.scrollIntoView({ block: "nearest" });
  }, [step]);
  const template = form.template!;
  let examples: Record<string, string> = {},
    exampleError = "";
  try {
    examples = parameterValues(template.example_json);
    if (
      Object.keys(examples).some(
        (name) => !template.parameters.some((p) => p.name === name),
      )
    )
      exampleError = t("Remove undeclared parameters from the example JSON.");
  } catch {
    exampleError = t("Fix the example JSON before editing values.");
  }

  let operationID = "";
  if (source.http_api) {
    try {
      const operation = JSON.parse(template.query_json).operation;
      if (typeof operation === "string") operationID = operation;
    } catch {
      /* The editor keeps invalid JSON available for repair. */
    }
  }
  const endpoint = `/api/sources/${encodeURIComponent(source.id)}/semantics`;
  const edit = (patch: Partial<typeof template>) =>
    setForm((f) => ({ ...f, template: { ...f.template!, ...patch } }));
  function move(next: number) {
    setStep(next);
    onStep(Math.min(next, 2));
    setError("");
  }
  function changeParameter(
    index: number,
    value?: (typeof template.parameters)[number],
  ) {
    try {
      const previous = template.parameters[index];
      const examples = parameterValues(template.example_json);
      if (
        value &&
        value.name !== previous.name &&
        Object.hasOwn(examples, previous.name)
      ) {
        if (Object.hasOwn(examples, value.name))
          throw new Error(t("Parameter names must be unique."));
        examples[value.name] = examples[previous.name];
      }
      if (!value || value.name !== previous.name) {
        delete examples[previous.name];
      }
      edit({
        parameters: value
          ? template.parameters.map((p, i) => (i === index ? value : p))
          : template.parameters.filter((_, i) => i !== index),
        example_json: parameterJSON(examples),
      });
      setError("");
    } catch (e) {
      setError(message(e));
    }
  }
  async function findParameters() {
    setBusy(true);
    setError("");
    try {
      const query = nativeTemplateQuery(template.query_json);
      const result = await api<{ slots: string[] }>(endpoint + "/bindings", {
        method: "POST",
        body: payload({ query_json: query }),
      });
      const suggested = suggestedParameters(
        query,
        result.slots,
        template.parameters,
      );
      edit({
        query_json: query,
        parameters: suggested.parameters,
        example_json: parameterJSON({
          ...parameterValues(template.example_json),
          ...suggested.examples,
        }),
      });
      setBindings({ query, slots: result.slots });
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function save(finish: boolean) {
    setBusy(true);
    setError("");
    try {
      if (finish && creating) {
        const invalid = validateParameters(
          template.parameters,
          template.example_json,
        );
        if (invalid) throw new Error(invalid);
      }
      const value = {
        ...form,
        name: form.name.trim() || t("Untitled query"),
        template: {
          ...template,
          query_json: nativeTemplateQuery(template.query_json),
        },
      };
      const state = await api<SemanticState>(
        `${endpoint}/entries/${encodeURIComponent(form.id)}`,
        { method: "PUT", body: payload({ revision: expected, entry: value }) },
      );
      setForm(value);
      setBaseline(JSON.stringify(value));
      setExpected(state.revision);
      setBinding(state.draft.ontology);
      setConflict(false);
      setLatest(null);
      clearGuard();
      onSaved(state, value.id, finish);
    } catch (e) {
      setConflict(e instanceof APIError && e.detail.code === "conflict");
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="query-authoring business-panel">
      <div ref={errorArea}>
        <ErrorNote error={error} />
      </div>
      {conflict && (
        <div className="notice warning">
          <div>
            <p>
              {t(
                "Your edits are kept. Review the current server entry before merging this entry into the latest draft.",
              )}
            </p>
            <Button
              busy={busy}
              onClick={async () => {
                setBusy(true);
                try {
                  setLatest(await api<SemanticState>(endpoint));
                } catch (e) {
                  setError(message(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              {t("Review current server entry")}
            </Button>
            {latest && (
              <>
                <Field label={t("Current server entry")}>
                  <textarea
                    readOnly
                    rows={6}
                    value={JSON.stringify(
                      latest.draft.entries.find((e) => e.id === form.id) ||
                        null,
                      null,
                      2,
                    )}
                  />
                </Field>
                <Button
                  onClick={() => {
                    setExpected(latest.revision);
                    setBinding(latest.draft.ontology);
                    setConflict(false);
                    setLatest(null);
                    setError("");
                  }}
                >
                  {t("Keep my entry edits and use this revision")}
                </Button>
                <p className="help">
                  {t(
                    "Only this entry will be replaced. Other draft entries are preserved; the next save still checks for conflicts.",
                  )}
                </p>
              </>
            )}
          </div>
        </div>
      )}
      {!creating && (
        <div
          className="query-editor-tabs"
          role="group"
          aria-label={t("Query sections")}
        >
          {[
            [1, "Query"],
            [3, "Parameters"],
            [2, "Business context"],
          ].map(([value, label]) => (
            <Button
              key={value}
              disabled={busy}
              aria-pressed={step === value}
              onClick={() => move(Number(value))}
            >
              {t(String(label))}
            </Button>
          ))}
        </div>
      )}
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (creating && step < 3) {
            try {
              nativeTemplateQuery(template.query_json);
              move(step + 1);
            } catch (e) {
              setError(message(e));
            }
          } else void save(true);
        }}
      >
        <fieldset disabled={busy} className="query-fieldset">
          {step === 1 ? (
            <>
              <h2 ref={heading} tabIndex={-1}>
                {t(creating ? "Provide an existing query" : "Query")}
              </h2>
              {source.http_api && (
                <Field label={t("API operation")}>
                  <select
                    value={operationID}
                    onChange={(e) => {
                      const op = source.http_api?.operations.find(
                        (o) => o.id === e.target.value,
                      );
                      if (op) {
                        setForm((f) => ({
                          ...f,
                          name: f.name || op.name,
                          description: f.description || op.description,
                          template: {
                            ...httpTemplate(op),
                            concept_refs: f.template?.concept_refs,
                          },
                        }));
                        setBindings(null);
                      }
                    }}
                  >
                    <option value="">{t("Choose an operation")}</option>
                    {source.http_api.operations.map((op) => (
                      <option key={op.id} value={op.id}>
                        {op.name}
                      </option>
                    ))}
                  </select>
                </Field>
              )}
              <NativeQueryEditor
                tool={template.tool}
                value={template.query_json}
                onChange={(query_json) => edit({ query_json })}
              />
            </>
          ) : step === 2 ? (
            <>
              <h2 ref={heading} tabIndex={-1}>
                {t("Business context")}
              </h2>
              <BusinessConcepts
                binding={binding}
                selected={template.concept_refs || []}
                onChange={(concept_refs) => edit({ concept_refs })}
              />
              <Field label={t("Query name")} required>
                <input
                  required
                  maxLength={256}
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </Field>
              <Field label={t("Purpose")} required>
                <textarea
                  required
                  rows={3}
                  value={form.description || ""}
                  onChange={(e) =>
                    setForm({ ...form, description: e.target.value })
                  }
                />
              </Field>
              <Field label={t("Result description")} required>
                <textarea
                  required
                  rows={3}
                  value={template.result_description || ""}
                  onChange={(e) => edit({ result_description: e.target.value })}
                />
              </Field>
              <details className="advanced">
                <summary>{t("More options")}</summary>
                <Field label={t("Aliases")}>
                  <textarea
                    rows={2}
                    value={(form.aliases || []).join("\n")}
                    onChange={(e) =>
                      setForm({ ...form, aliases: e.target.value.split("\n") })
                    }
                  />
                </Field>
                <Field label={t("Unit")}>
                  <input
                    value={form.unit || ""}
                    onChange={(e) => setForm({ ...form, unit: e.target.value })}
                  />
                </Field>
                <Field label={t("Time definition")}>
                  <textarea
                    rows={2}
                    value={form.time_definition || ""}
                    onChange={(e) =>
                      setForm({ ...form, time_definition: e.target.value })
                    }
                  />
                </Field>
                <Field label={t("Caveats")}>
                  <textarea
                    rows={2}
                    value={form.caveats || ""}
                    onChange={(e) =>
                      setForm({ ...form, caveats: e.target.value })
                    }
                  />
                </Field>
                <label className="checkbox-row">
                  <input
                    type="checkbox"
                    checked={template.enabled}
                    onChange={(event) =>
                      edit({ enabled: event.target.checked })
                    }
                  />
                  {t("Enable this query when published")}
                </label>
              </details>
            </>
          ) : (
            <>
              <div className="section-heading">
                <h2 ref={heading} tabIndex={-1}>
                  {t("Parameters")}
                </h2>
                <Button busy={busy} onClick={findParameters}>
                  {t("Find existing parameter slots")}
                </Button>
              </div>
              {bindings && !bindings.slots.length && (
                <p className="notice neutral">
                  {t(
                    "No supported slots found. Add native placeholders and fixed parameter values in the query, or keep it as a query without inputs.",
                  )}
                </p>
              )}
              {template.parameters.map((parameter, index) => (
                <ParameterFields
                  key={index}
                  value={parameter}
                  slots={
                    bindings?.query === template.query_json
                      ? bindings.slots
                      : []
                  }
                  onChange={(value) => changeParameter(index, value)}
                  remove={() => changeParameter(index)}
                >
                  {!exampleError && (
                    <TemplateParameters
                      compact
                      parameters={[parameter]}
                      value={parameterJSON(
                        Object.hasOwn(examples, parameter.name)
                          ? { [parameter.name]: examples[parameter.name] }
                          : {},
                      )}
                      example={"{}"}
                      disabled={busy}
                      onChange={(value) => {
                        const next = { ...examples };
                        delete next[parameter.name];
                        edit({
                          example_json: parameterJSON({
                            ...next,
                            ...parameterValues(value),
                          }),
                        });
                      }}
                    />
                  )}
                </ParameterFields>
              ))}

              {!template.parameters.length && (
                <p className="help">
                  {t("This template needs no parameters.")}
                </p>
              )}
              <Button
                onClick={() =>
                  edit({
                    parameters: [
                      ...template.parameters,
                      {
                        name: `parameter_${template.parameters.length + 1}`,
                        type: "string",
                        required: true,
                        pointers: [],
                      },
                    ],
                  })
                }
              >
                {t("Add parameter")}
              </Button>
              <details className="advanced" open={!!exampleError}>
                <summary>{t("Example parameters (JSON)")}</summary>
                <Field
                  label={t("Example parameters (JSON)")}
                  hint={exampleError}
                >
                  <textarea
                    rows={5}
                    value={template.example_json}
                    onChange={(e) => edit({ example_json: e.target.value })}
                  />
                </Field>
              </details>
              <details className="advanced">
                <summary>{t("Regression checks")}</summary>
                <RegressionEditor
                  tests={template.tests || []}
                  update={(tests) => edit({ tests })}
                />
              </details>
            </>
          )}
          <div className="query-step-actions">
            <Button
              onClick={() => {
                if (
                  JSON.stringify(form) !== baseline &&
                  !window.confirm(
                    t("Discard unsaved query edits? The saved draft is kept."),
                  )
                )
                  return;
                clearGuard();
                onCancel();
              }}
            >
              {t("Cancel editing")}
            </Button>
            {creating && step > 1 && (
              <Button onClick={() => move(step - 1)}>{t("Back")}</Button>
            )}
            {creating && (
              <Button disabled={conflict} onClick={() => save(false)}>
                {t("Save draft for later")}
              </Button>
            )}
            <Button primary type="submit" disabled={conflict}>
              {creating && step < 3 ? t("Continue") : t("Save draft")}
            </Button>
          </div>
        </fieldset>
      </form>
    </section>
  );
}
