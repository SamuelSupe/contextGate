import {
  conceptRefs,
  effectiveEntities,
  type OntologyBinding,
  type OntologyDefinition,
} from "./ontology-types.ts";
import type { SemanticEntry, SemanticSnapshot } from "./semantic-types.ts";

export interface BusinessConcept {
  ref: string;
  name: string;
  kind: string;
  description: string;
}

export function mappedConcepts(
  binding: OntologyBinding,
  definition: OntologyDefinition,
): BusinessConcept[] {
  const entityName = (id: string) =>
    definition.entities.find((e) => e.id === id)?.name || id;
  return [
    ...binding.entities.map((m) => {
      const entity = definition.entities.find((e) => e.id === m.entity);
      return {
        ref: `ontology:entity_type:${m.entity}`,
        name: entityName(m.entity),
        kind: "Entity type",
        description: entity?.description || "",
      };
    }),
    ...binding.properties.map((m) => {
      const owners = effectiveEntities(definition, m.entity);
      const property = definition.properties.find(
        (p) => p.id === m.property && owners.includes(p.entity),
      );
      return {
        ref: `ontology:property:${m.entity}:${m.property}`,
        name: `${entityName(m.entity)} · ${property?.name || m.property}`,
        kind: "Property",
        description: property?.description || "",
      };
    }),
    ...binding.relations.map((m) => {
      const relation = definition.relations.find((r) => r.id === m.relation);
      return {
        ref: `ontology:relation_type:${m.relation}`,
        name: relation
          ? `${entityName(relation.from)} → ${relation.name} → ${entityName(relation.to)}`
          : m.relation,
        kind: "Relation type",
        description: relation?.description || "",
      };
    }),
  ];
}

export function conceptQueries(
  snapshot: SemanticSnapshot,
  ontologyID: string,
  entityID: string,
  definition?: OntologyDefinition,
) {
  const binding = snapshot.ontology;
  if (
    !binding ||
    binding.ontology_id !== ontologyID ||
    !binding.entities.some((m) => m.entity === entityID)
  )
    return [];
  const properties = binding.properties.filter((m) => m.entity === entityID);
  const relations = binding.relations.filter((m) =>
    definition?.relations.some(
      (r) => r.id === m.relation && (r.from === entityID || r.to === entityID),
    ),
  );
  const refs = new Set([
    `ontology:entity_type:${entityID}`,
    ...properties.map((m) => `ontology:property:${entityID}:${m.property}`),
    ...relations.map((m) => `ontology:relation_type:${m.relation}`),
  ]);
  const ids = new Set(
    [...properties, ...relations].map((m) => m.template_id).filter(Boolean),
  );
  return snapshot.entries.filter(
    (entry) =>
      entry.template &&
      (ids.has(entry.id) ||
        entry.template.concept_refs?.some((ref) => refs.has(ref))),
  );
}

export function linkQueryConcept(
  snapshot: SemanticSnapshot,
  entry: SemanticEntry,
  ontologyID: string,
  ref: string,
): SemanticEntry {
  if (
    !entry.template ||
    snapshot.ontology?.ontology_id !== ontologyID ||
    !conceptRefs(snapshot.ontology).includes(ref)
  ) {
    throw new Error(
      "The concept is not mapped in this draft. Update the source mapping first.",
    );
  }
  return {
    ...entry,
    template: {
      ...entry.template,
      concept_refs: [...new Set([...(entry.template.concept_refs || []), ref])],
    },
  };
}

export interface ConceptCoverage {
  sources: number;
  source_ids?: string[];
  templates: number;
  linked_templates: number;
}

export function coverageLabel(usage?: ConceptCoverage) {
  if (!usage?.sources) return "Definition only";
  if (!usage.linked_templates) return "Mapped · no queries";
  return usage.templates ? "Queries available" : "Queries unavailable";
}
