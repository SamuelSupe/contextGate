import type {
  SemanticEntry,
  SemanticParameter,
  SemanticState,
} from "./semantic-types";
import type { Readiness } from "./readiness";
import {
  parameterAt,
  parameterJSON,
  parameterValues,
} from "./template-parameters.ts";

export interface QueryJourney {
  source_id: string;
  template_id: string;
  agent_id: string;
}

export interface QueryConceptContext {
  ontology_id: string;
  concept_ref: string;
}

export function queryToolURL(
  source = "",
  template = "",
  agent = "",
  context?: QueryConceptContext,
  edit = false,
) {
  const query = new URLSearchParams({
    source_id: source,
    template_id: template,
    agent_id: agent,
  });
  if (context) {
    query.set("ontology_id", context.ontology_id);
    query.set("concept_ref", context.concept_ref);
  }
  if (edit) query.set("edit", "1");
  return `/queries/publish?${query}`;
}

export function catalogReturnURL(value: string | null) {
  if (
    !value ||
    value.length > 8192 ||
    (value !== "/business" && !value.startsWith("/business?"))
  )
    return "";
  const input = new URLSearchParams(value.slice("/business?".length));
  const query = new URLSearchParams();
  for (const key of [
    "view",
    "keyword",
    "kind",
    "source_id",
    "agent_id",
    "offset",
    "revision",
    "scroll_y",
    "focus",
  ]) {
    if (input.has(key)) query.set(key, input.get(key)!);
  }
  return `/business${query.size ? `?${query}` : ""}`;
}

export function savedJourney(administrator: string): QueryJourney | null {
  return savedJourneys(administrator)[0] || null;
}

function journeyReferences(value: unknown): QueryJourney | null {
  if (!value || typeof value !== "object") return null;
  const ref = value as QueryJourney;
  if (
    ![ref.source_id, ref.template_id, ref.agent_id].every(
      (v) => typeof v === "string" && v.length <= 256,
    ) ||
    !ref.source_id ||
    !ref.template_id
  )
    return null;
  return {
    source_id: ref.source_id,
    template_id: ref.template_id,
    agent_id: ref.agent_id,
  };
}

export function savedJourneys(administrator: string): QueryJourney[] {
  try {
    const value = JSON.parse(
      localStorage.getItem(`contextgate.query-journey.${administrator}`) ||
        "null",
    );
    const entries = Array.isArray(value) ? value.slice(0, 5) : [value];
    const refs: QueryJourney[] = [];
    for (const entry of entries) {
      const ref = journeyReferences(entry);
      if (
        ref &&
        !refs.some(
          (v) =>
            v.source_id === ref.source_id && v.template_id === ref.template_id,
        )
      )
        refs.push(ref);
    }
    return refs;
  } catch {
    return [];
  }
}

export function rememberJourney(administrator: string, value: QueryJourney) {
  try {
    const ref = journeyReferences(value);
    if (!administrator || !ref) return;
    // Only stable references belong in browser storage. Drafts and evidence are
    // loaded from the server; native queries and credentials never go here.
    localStorage.setItem(
      `contextgate.query-journey.${administrator}`,
      JSON.stringify(
        [
          ref,
          ...savedJourneys(administrator).filter(
            (v) =>
              v.source_id !== ref.source_id ||
              v.template_id !== ref.template_id,
          ),
        ].slice(0, 5),
      ),
    );
  } catch {
    /* The URL still resumes a saved draft when storage is unavailable. */
  }
}

export function nativeTemplateQuery(text: string) {
  const values = parameterValues(text);
  for (const key of [
    "source_id",
    "agent_id",
    "tool",
    "cursor",
    "max_rows",
    "max_bytes",
    "timeout_seconds",
  ])
    delete values[key];
  return parameterJSON(values);
}

export function suggestedParameters(
  query: string,
  slots: string[],
  existing: SemanticParameter[] = [],
) {
  const parameters = [...existing];
  const examples: Record<string, string> = Object.create(null);
  for (const slot of slots) {
    if (existing.some((p) => p.pointers.includes(slot))) continue;
    const raw = parameterAt(query, slot);
    const value = JSON.parse(raw);
    const type =
      value === null
        ? "null"
        : Array.isArray(value)
          ? "array"
          : typeof value === "number"
            ? /^-?\d+$/.test(raw)
              ? "integer"
              : "number"
            : typeof value;
    let base = slot
      .split("/")
      .at(-1)!
      .replaceAll("~1", "/")
      .replaceAll("~0", "~")
      .replace(/[^A-Za-z0-9_]/g, "_");
    if (!/^[A-Za-z]/.test(base)) base = `parameter_${parameters.length + 1}`;
    let name = base;
    for (let n = 2; parameters.some((p) => p.name === name); n++)
      name = `${base}_${n}`;
    parameters.push({ name, type, required: true, pointers: [slot] });
    examples[name] = raw;
  }
  return { parameters, examples };
}

export function sameQueryEntry(a?: SemanticEntry, b?: SemanticEntry) {
  const comparable = (entry?: SemanticEntry) =>
    entry && {
      ...entry,
      template: entry.template && {
        ...entry.template,
        execution_version: undefined,
      },
    };
  return JSON.stringify(comparable(a)) === JSON.stringify(comparable(b));
}

export function queryNextStep(
  state: SemanticState,
  ready: Readiness | null,
  id: string,
  agentID: string,
) {
  const draft = state.draft.entries.find((e) => e.id === id && e.template);
  const published = state.published.entries.find(
    (e) => e.id === id && e.template,
  );
  if (!draft && !published) return "query";
  if (!ready || ready.published_version !== state.published_version)
    return "loading";
  const valid = state.validation.some((v) => v.id === id && v.valid);
  if (!sameQueryEntry(draft, published)) {
    return draft?.template?.enabled && !valid ? "trial" : "publish";
  }
  if (!published?.template?.enabled) return "disabled";
  if (!ready.templates.some((e) => e.id === id && e.executable))
    return valid ? "publish" : "trial";
  if (!ready.active_agents.length) return "access";
  const activity = ready.template_activity;
  return agentID &&
    ready.active_agents.includes(agentID) &&
    activity?.agent_id === agentID &&
    activity.template_id === id &&
    activity.execution_version === published.template.execution_version &&
    activity.successful_calls > 0
    ? "done"
    : "client";
}

export const queryStepLabels = {
  query: "Provide a query",
  loading: "Loading…",
  trial: "Run checks",
  publish: "Review and publish",
  disabled: "Query disabled",
  access: "Grant Agent access",
  client: "Confirm client call",
  done: "Client call confirmed",
} as const;
