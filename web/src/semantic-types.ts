import type {
  OntologyBinding,
  EntityType,
  Property,
  RelationType,
  EntityMapping,
  PropertyMapping,
  RelationMapping,
} from "./ontology-types";
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
export interface RegressionCase {
  name: string;
  parameters_json: string;
  min_rows?: number;
  max_rows?: number;
  columns?: { name: string; type: string }[];
  values?: { pointer: string; expected_json: string }[];
}
export interface QueryTemplate {
  tests?: RegressionCase[];
  concept_refs?: string[];
  enabled: boolean;
  tool: string;
  query_json: string;
  parameters: SemanticParameter[];
  example_json: string;
  result_description?: string;
  execution_version?: string;
}
export interface SemanticEntry {
  definition?: EntityType | Property | RelationType;
  mapping?: (EntityMapping | PropertyMapping | RelationMapping) & {
    verification_status?: string;
    checked_at?: string;
  };
  ancestors?: EntityType[];
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
  ontology?: OntologyBinding | null;
  format_version: number;
  overview: string;
  entries: SemanticEntry[];
}
export interface SemanticState {
  mapping_validation?: {
    status: string;
    checked_at?: string;
    checks?: { reference: ObjectReference; status: string }[];
  };
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
    report?: {
      passed: boolean;
      cases: {
        name: string;
        passed: boolean;
        error_code?: string;
        rows: number;
        elapsed_ms: number;
      }[];
    };
  }[];
}
export const entryKinds = ["term", "object", "field", "relationship", "metric"];
export function semanticKindLabel(kind: string): string {
  return (
    (
      {
        term: "Business term",
        object: "Data object",
        field: "Field",
        relationship: "Relationship",
        metric: "Metric",
        template: "Query template",
        entity_type: "Entity type",
        property: "Property",
        relation_type: "Relation type",
        overview: "Overview",
      } as Record<string, string>
    )[kind] || kind
  );
}
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
