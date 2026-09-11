export interface ObjectReference {
  namespace: string;
  object: string;
  field?: string;
}
export interface SemanticParameter {
  name: string;
  description?: string;
  type: string;
  required: boolean;
  pointers: string[];
  default_json?: string;
  enum_json?: string;
  minimum?: string;
  maximum?: string;
}
export interface QueryTemplate {
  enabled: boolean;
  tool: string;
  query_json: string;
  parameters: SemanticParameter[];
  example_json: string;
  result_description?: string;
  execution_version?: string;
}
export interface SemanticEntry {
  id: string;
  kind: string;
  name: string;
  aliases?: string[];
  description?: string;
  reference?: ObjectReference;
  data_type?: string;
  unit?: string;
  enums?: Record<string, string>;
  time_definition?: string;
  grain?: string;
  caveats?: string;
  related?: ObjectReference[];
  template_id?: string;
  template?: QueryTemplate;
}
export interface SemanticSnapshot {
  format_version: number;
  overview: string;
  entries: SemanticEntry[];
}
export interface SemanticState {
  revision: string;
  published_version: string;
  draft: SemanticSnapshot;
  published: SemanticSnapshot;
  changed: boolean;
  validation: {
    id: string;
    valid: boolean;
    status: string;
    checked_at?: string;
    server_version?: string;
  }[];
}
export const entryKinds = ["term", "object", "field", "relationship", "metric"];
export function templatePayload(
  source: string,
  template: SemanticEntry,
  parameters: string,
  agent: string,
  cursor = "",
) {
  const parsed: unknown = JSON.parse(parameters);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
    throw new Error("Parameters must be a JSON object.");
  return `{"source_id":${JSON.stringify(source)},"template_id":${JSON.stringify(template.id)},"execution_version":${JSON.stringify(template.template?.execution_version)},"agent_id":${JSON.stringify(agent)},"cursor":${JSON.stringify(cursor)},"parameters":${parameters}}`;
}
