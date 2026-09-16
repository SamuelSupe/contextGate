import { t } from "./i18n";
import { ArrowLeftRight, ArrowRight, KeyRound, Plus } from "lucide-react";
import { Button } from "./components";
import {
  effectiveEntities,
  type EntityType,
  type OntologyDefinition,
} from "./ontology-types";
import type { OntologyItem, OntologyKind } from "./OntologyEditor";
import { cardinalityLabel } from "./OntologyCardinality";

export function OntologyEntityDetails({
  definition,
  entity,
  onSelect,
  onAdd,
  onEdit,
  onRemove,
  disabled,
}: {
  definition: OntologyDefinition;
  entity: EntityType;
  onSelect: (id: string) => void;
  onAdd: (kind: OntologyKind, entity?: string) => void;
  onEdit: (kind: OntologyKind, item: OntologyItem) => void;
  onRemove: (kind: OntologyKind, id: string) => void;
  disabled: boolean;
}) {
  const chain = entity ? effectiveEntities(definition, entity.id) : [];
  const properties = definition.properties.filter((p) =>
    chain.includes(p.entity),
  );
  const relations = definition.relations.filter(
    (r) => r.from === entity?.id || r.to === entity?.id,
  );
  const name = (id: string) =>
    definition.entities.find((e) => e.id === id)?.name || id;
  return (
    <section
      className="ontology-entity-detail"
      aria-label={t("Selected entity")}
    >
      <div className="ontology-entity-header">
        <div>
          <span className="ontology-eyebrow">{t("ENTITY TYPE")}</span>
          <h2>{entity.name}</h2>
          <code>{entity.id}</code>
        </div>
        <div className="button-row">
          <Button
            disabled={disabled}
            onClick={() => onEdit("entities", entity)}
          >
            {t("Edit entity")}
          </Button>
          <Button
            disabled={disabled}
            className="danger"
            onClick={() => onRemove("entities", entity.id)}
          >
            {t("Remove entity")}
          </Button>
        </div>
      </div>
      <p className="ontology-description">
        {entity.description ||
          t(
            "Add a description to explain what this entity means to your business.",
          )}
      </p>
      {chain.length > 1 && (
        <div className="ontology-inheritance">
          <span>{t("Inherits from")}</span>
          {chain.slice(1).map((id) => (
            <Button key={id} onClick={() => onSelect(id)}>
              {name(id)}
            </Button>
          ))}
        </div>
      )}
      <section
        className="ontology-detail-section"
        aria-label={t("Entity properties")}
      >
        <div className="ontology-section-heading">
          <div>
            <h3>
              {t("Properties ")}
              <small>{properties.length}</small>
            </h3>
            <p>
              {t("What do you know about each {name}?", { name: entity.name })}
            </p>
          </div>
          <Button
            primary
            disabled={disabled}
            onClick={() => onAdd("properties", entity.id)}
          >
            <Plus size={15} />
            {t(" Add property")}
          </Button>
        </div>
        {properties.length ? (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>{t("Property")}</th>
                  <th>{t("Type")}</th>
                  <th>{t("Rules")}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {properties.map((p) => (
                  <tr key={p.id}>
                    <td>
                      <button
                        className="ontology-text-button"
                        disabled={disabled}
                        onClick={() =>
                          p.entity === entity.id
                            ? onEdit("properties", p)
                            : onSelect(p.entity)
                        }
                      >
                        {p.name}
                      </button>
                      <small className="block">
                        {p.entity !== entity.id
                          ? t("Inherited from {value1}", {
                              value1: name(p.entity),
                            })
                          : p.description || p.id}
                      </small>
                    </td>
                    <td>
                      <span className="ontology-value-type">
                        {t(p.type)}
                        {p.multiple ? "[]" : ""}
                      </span>
                      {p.unit && <small className="block">{p.unit}</small>}
                    </td>
                    <td>
                      <div className="ontology-rule-list">
                        {entity.identity?.includes(p.id) && (
                          <span className="status green">
                            <KeyRound size={12} />
                            {t(" Identity")}
                          </span>
                        )}
                        <span className="status muted">
                          {p.required ? t("Required") : t("Optional")}
                        </span>
                        {p.unique && (
                          <span className="status muted">{t("Unique")}</span>
                        )}
                      </div>
                    </td>
                    <td>
                      {p.entity === entity.id ? (
                        <div className="button-row">
                          <Button
                            disabled={disabled}
                            onClick={() => onEdit("properties", p)}
                          >
                            {t("Edit")}
                          </Button>
                          <Button
                            disabled={disabled}
                            className="danger"
                            onClick={() => onRemove("properties", p.id)}
                          >
                            {t("Remove")}
                          </Button>
                        </div>
                      ) : (
                        <Button onClick={() => onSelect(p.entity)}>
                          {t("View owner")}
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="ontology-inline-empty">
            <p>
              {t(
                "No properties yet. Add an identifier, name or business value.",
              )}
            </p>
            <Button
              disabled={disabled}
              onClick={() => onAdd("properties", entity.id)}
            >
              {t("Add the first property")}
            </Button>
          </div>
        )}
      </section>
      <section
        className="ontology-detail-section"
        aria-label={t("Entity relationships")}
      >
        <div className="ontology-section-heading">
          <div>
            <h3>
              {t("Relationship map ")}
              <small>{relations.length}</small>
            </h3>
            <p>
              {t("Click an entity to navigate, or a relationship to edit it.")}
            </p>
          </div>
          <Button
            disabled={disabled}
            onClick={() => onAdd("relations", entity.id)}
          >
            <Plus size={15} />
            {t(" Add relationship")}
          </Button>
        </div>
        {relations.length ? (
          <div className="ontology-relations">
            {relations.map((r) => (
              <article key={r.id} className="ontology-relation-card">
                <div className="ontology-relation-path">
                  <Button
                    className={
                      r.from === entity.id ? "ontology-current-node" : ""
                    }
                    onClick={() => onSelect(r.from)}
                  >
                    {name(r.from)}
                  </Button>
                  <button
                    className="ontology-relation-edge"
                    disabled={disabled}
                    onClick={() => onEdit("relations", r)}
                    aria-label={t("Edit relationship {name}", { name: r.name })}
                  >
                    <span>{r.name}</span>
                    {r.directed ? (
                      <ArrowRight size={24} />
                    ) : (
                      <ArrowLeftRight size={24} />
                    )}
                  </button>
                  <Button
                    className={
                      r.to === entity.id ? "ontology-current-node" : ""
                    }
                    onClick={() => onSelect(r.to)}
                  >
                    {name(r.to)}
                  </Button>
                </div>
                {r.description && <p>{r.description}</p>}
                <div className="ontology-relation-rules">
                  <span>
                    {t("Each ")}
                    {name(r.to)} → {cardinalityLabel(r.from_cardinality)}{" "}
                    {name(r.from)}
                  </span>
                  <span>
                    {t("Each ")}
                    {name(r.from)} → {cardinalityLabel(r.to_cardinality)}{" "}
                    {name(r.to)}
                  </span>
                </div>
                <div className="button-row">
                  <Button
                    disabled={disabled}
                    onClick={() => onEdit("relations", r)}
                  >
                    {t("Edit relationship")}
                  </Button>
                  <Button
                    disabled={disabled}
                    className="danger"
                    onClick={() => onRemove("relations", r.id)}
                  >
                    {t("Remove")}
                  </Button>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <div className="ontology-inline-empty">
            <p>
              {t("No relationships yet. Describe how ")}
              {entity.name}
              {t(" connects to another entity.")}
            </p>
          </div>
        )}
      </section>
      <p className="help">
        {t(
          "Edits are saved to the draft. Publish a version when ready; data sources keep their adopted version.",
        )}
      </p>
    </section>
  );
}
