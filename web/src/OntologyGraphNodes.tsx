import { coverageLabel, type ConceptCoverage } from "./query-concepts";
import { t } from "./i18n";
import { useMemo, type PointerEvent } from "react";
import { Boxes, GripVertical, Plus } from "lucide-react";
import { effectiveEntities, type OntologyDefinition } from "./ontology-types";
import {
  arrowDelta,
  NODE_HEIGHT,
  NODE_WIDTH,
  type Point,
  type Positions,
} from "./ontology-graph-layout";

export function OntologyGraphNodes({
  definition,
  positions,
  selected,
  connecting,
  onBegin,
  onSelect,
  onInspect,
  usage,
  onConnect,
  onMove,
  disabled,
}: {
  definition: OntologyDefinition;
  positions: Positions;
  selected: string;
  connecting: string;
  onBegin: (event: PointerEvent, kind: "node" | "connect", id: string) => void;
  onSelect: (id: string) => void;
  onInspect: (id: string, queries?: boolean) => void;
  usage: Record<string, ConceptCoverage> | null;
  onConnect: (id: string) => void;
  onMove: (id: string, delta: Point) => void;
  disabled: boolean;
}) {
  const entries = useMemo(
    () =>
      definition.entities.map((entity) => {
        const own = definition.properties.filter((p) => p.entity === entity.id);
        const inherited = effectiveEntities(definition, entity.id).slice(1);
        const inheritedCount = definition.properties.filter((p) =>
          inherited.includes(p.entity),
        ).length;
        return { entity, own, inheritedCount };
      }),
    [definition],
  );
  return (
    <>
      {entries.map(({ entity, own, inheritedCount }) => {
        return (
          <article
            key={entity.id}
            data-graph-entity={entity.id}
            aria-label={t("Entity {name}", { name: entity.name })}
            className={`ontology-graph-node ${selected === entity.id ? "selected" : ""} ${connecting === entity.id ? "connection-origin" : ""}`}
            style={{
              left: positions[entity.id].x,
              top: positions[entity.id].y,
              width: NODE_WIDTH,
              height: NODE_HEIGHT,
            }}
          >
            <button
              className="ontology-graph-node-header"
              aria-label={t("Open {name} details or drag to move", {
                name: entity.name,
              })}
              aria-pressed={selected === entity.id}
              title={t(
                "Click or press Enter for details. Drag or use arrow keys to move.",
              )}
              onPointerDown={(e) => onBegin(e, "node", entity.id)}
              onFocus={() => onSelect(entity.id)}
              onClick={(e) => {
                if (e.detail === 0) {
                  if (connecting) onConnect(entity.id);
                  else onInspect(entity.id);
                }
              }}
              onKeyDown={(e) => {
                const delta = arrowDelta(e.key, e.shiftKey ? 80 : 20);
                if (delta) {
                  e.preventDefault();
                  e.stopPropagation();
                  onMove(entity.id, delta);
                }
              }}
            >
              <Boxes size={17} />
              <span>
                <strong>{entity.name}</strong>
                <small>{entity.id}</small>
              </span>
              <GripVertical size={16} />
            </button>
            <button
              disabled={disabled}
              className="ontology-graph-port"
              aria-label={
                connecting
                  ? t("Connect to {name}", { name: entity.name })
                  : t("Start relationship from {name}", { name: entity.name })
              }
              title={t(
                "Drag to another entity, or click two connection points",
              )}
              onPointerDown={(e) => onBegin(e, "connect", entity.id)}
              onClick={(e) => {
                if (e.detail === 0) onConnect(entity.id);
              }}
            >
              <Plus size={13} />
            </button>
            <div className="ontology-graph-node-properties">
              {own.slice(0, 3).map((p) => (
                <div key={p.id} title={p.name}>
                  <span>
                    {p.name}
                    {p.required ? " *" : ""}
                  </span>
                  <small>
                    {p.type}
                    {p.multiple ? "[]" : ""}
                  </small>
                </div>
              ))}
              {!own.length && <p>{t("No properties yet")}</p>}
              <small className="ontology-graph-property-count">
                {own.length > 3
                  ? t("+{value1} more · ", { value1: own.length - 3 })
                  : ""}
                {inheritedCount
                  ? t("{inheritedCount} inherited · ", {
                      inheritedCount: inheritedCount,
                    })
                  : ""}
                {own.length}
                {t(" properties")}
              </small>
            </div>
            <button
              className="ontology-graph-query-usage"
              aria-label={t("View queries for {name}", { name: entity.name })}
              onClick={() => onInspect(entity.id, true)}
            >
              {usage
                ? t(coverageLabel(usage[entity.id]))
                : t("View query usage")}{" "}
              <strong>{t("Queries →")}</strong>
            </button>
            <div className="ontology-graph-node-actions">
              <button
                aria-label={t("View all details for {name}", {
                  name: entity.name,
                })}
                onClick={() => onInspect(entity.id)}
              >
                {t("Details")}
              </button>
            </div>
          </article>
        );
      })}
    </>
  );
}
