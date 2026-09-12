import type { ObjectReference } from "./semantic-types";

export interface EntityType {
  id: string;
  name: string;
  aliases?: string[];
  description?: string;
  parent?: string;
  identity?: string[];
}
export interface Property {
  id: string;
  entity: string;
  name: string;
  aliases?: string[];
  description?: string;
  type: string;
  required: boolean;
  multiple: boolean;
  unique?: boolean;
  unit?: string;
  enums?: string[];
  minimum?: string;
  maximum?: string;
  time_definition?: string;
}
export interface RelationType {
  id: string;
  name: string;
  aliases?: string[];
  description?: string;
  from: string;
  to: string;
  directed: boolean;
  from_cardinality: { min: number; max: number | null };
  to_cardinality: { min: number; max: number | null };
}
export interface OntologyDefinition {
  format_version: number;
  name: string;
  description?: string;
  entities: EntityType[];
  properties: Property[];
  relations: RelationType[];
}
export interface OntologySummary {
  id: string;
  name: string;
  description?: string;
  revision: string;
  latest_version: string;
  archived: boolean;
}
export interface OntologyState {
  id: string;
  revision: string;
  latest_version: string;
  archived: boolean;
  draft: OntologyDefinition;
  versions: string[];
  usage: { source_id: string; phase: string; version: string }[];
  changed: boolean;
}
export interface OntologyVersion {
  ontology_id: string;
  version: string;
  published_at: string;
  definition: OntologyDefinition;
}
export interface EntityMapping {
  entity: string;
  objects: ObjectReference[];
  description?: string;
}
export interface PropertyMapping {
  entity: string;
  property: string;
  reference?: ObjectReference;
  template_id?: string;
  declared?: boolean;
  description?: string;
}
export interface RelationMapping {
  relation: string;
  fields?: { from: ObjectReference; to: ObjectReference; declared?: boolean }[];
  template_id?: string;
  description?: string;
}
export interface OntologyBinding {
  ontology_id: string;
  version: string;
  entities: EntityMapping[];
  properties: PropertyMapping[];
  relations: RelationMapping[];
}
export const emptyOntology = (): OntologyDefinition => ({
  format_version: 1,
  name: "",
  entities: [],
  properties: [],
  relations: [],
});
export function conceptRefs(binding?: OntologyBinding | null) {
  if (!binding) return [];
  return [
    ...binding.entities.map((m) => `ontology:entity_type:${m.entity}`),
    ...binding.properties.map(
      (m) => `ontology:property:${m.entity}:${m.property}`,
    ),
    ...binding.relations.map((m) => `ontology:relation_type:${m.relation}`),
  ];
}
export function effectiveEntities(
  definition: OntologyDefinition,
  entity: string,
) {
  const chain: string[] = [];
  while (entity && !chain.includes(entity)) {
    chain.push(entity);
    entity = definition.entities.find((e) => e.id === entity)?.parent || "";
  }
  return chain;
}
export function downloadJSON(value: unknown, filename: string) {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
