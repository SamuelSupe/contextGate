export interface SavedQuestion {
  id: string;
  revision: string;
  name: string;
  question: string;
  criteria: string;
  client: string;
  created_at: string;
}
export interface EvaluationStats {
  calls: number;
  queries: number;
  successful_queries: number;
  errors: number;
  elapsed_ms: number;
  template_calls: number;
}
export interface EvaluationCapture {
  state: "capturing" | "completed" | "abandoned";
  started: string;
  until?: string;
  stats?: EvaluationStats;
  configuration: {
    query_revision: string;
    published_version: string;
    agent_revision: string;
    ontology_id?: string;
    ontology_version?: string;
  };
  configuration_changed: boolean;
  verdict: string;
  notes: string;
}
export interface Evaluation {
  id: string;
  revision: string;
  source_id: string;
  case_id?: string;
  case_revision?: string;
  name: string;
  question: string;
  criteria: string;
  client: string;
  agent_id: string;
  agent_name: string;
  created_at: string;
  runs: Partial<Record<"baseline" | "guided", EvaluationCapture>>;
}
export type EvaluationKind = "baseline" | "guided";
export type EvaluationForm = Pick<
  SavedQuestion,
  "name" | "question" | "criteria" | "client"
>;
export function matchingConditions(value: Evaluation) {
  const { baseline, guided } = value.runs;
  return !!(
    baseline &&
    guided &&
    !baseline.configuration_changed &&
    !guided.configuration_changed &&
    JSON.stringify(baseline.configuration) ===
      JSON.stringify(guided.configuration)
  );
}
