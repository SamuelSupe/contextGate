import { t } from "./i18n";
import { useEffect, useRef, useState } from "react";
import { api, message } from "./api";
import { Boxes, List, Network, Plus } from "lucide-react";
import { Button, Drawer, Empty, ErrorNote } from "./components";
import { type OntologyDefinition } from "./ontology-types";
import type { OntologyItem, OntologyKind } from "./OntologyEditor";
import { OntologyEntityDetails } from "./OntologyEntityDetails";
import { ConceptUsage } from "./ConceptUsage";
import type { Source, Agent } from "./types";
import { OntologyGraph } from "./OntologyGraph";

export function OntologyModel({
  ontologyID,
  refreshKey,
  sources,
  agents,
  navigate,
  definition,
  selected,
  onSelect,
  onAdd,
  onEdit,
  onRemove,
  onConnect,
  disabled,
  overlayOpen,
}: {
  ontologyID: string;
  refreshKey: number;
  sources: Source[];
  agents: Agent[];
  navigate: (url: string) => void;
  definition: OntologyDefinition;
  selected: string;
  onSelect: (id: string) => void;
  onAdd: (kind: OntologyKind, entity?: string) => void;
  onEdit: (kind: OntologyKind, item: OntologyItem) => void;
  onRemove: (kind: OntologyKind, id: string) => void;
  onConnect: (from: string, to: string) => void;
  disabled: boolean;
  overlayOpen: boolean;
}) {
  const [search, setSearch] = useState("");
  const [view, setView] = useState<"graph" | "list">("graph");
  const [inspected, setInspected] = useState<string | null>(null);
  const [section, setSection] = useState<"definition" | "queries">(
    "definition",
  );
  const [usage, setUsage] = useState<Record<
    string,
    { sources: number; templates: number }
  > | null>(null);
  const [usageError, setUsageError] = useState("");
  const [usageRefresh, setUsageRefresh] = useState(0);
  const sourceKey = sources.map((s) => s.id + s.revision).join(",");
  useEffect(() => {
    const abort = new AbortController();
    setUsage(null);
    setUsageError("");
    api<Record<string, { sources: number; templates: number }>>(
      `/api/ontologies/${ontologyID}/usage-summary`,
      { signal: abort.signal },
    )
      .then(setUsage)
      .catch((e) => {
        if (!abort.signal.aborted) setUsageError(message(e));
      });
    return () => abort.abort();
  }, [ontologyID, sourceKey, refreshKey, usageRefresh]);
  const inspectTrigger = useRef<HTMLElement | null>(null);
  const inspectedEntity = definition.entities.find((e) => e.id === inspected);
  function closeInspector() {
    setInspected(null);
    requestAnimationFrame(() =>
      inspectTrigger.current?.focus({ preventScroll: true }),
    );
  }
  const entity =
    definition.entities.find((e) => e.id === selected) ||
    definition.entities[0];
  const name = (id: string) =>
    definition.entities.find((e) => e.id === id)?.name || id;
  const matches = definition.entities.filter((e) => {
    const fields = definition.properties.filter((p) => p.entity === e.id);
    return [
      e.name,
      e.id,
      e.description,
      ...(e.aliases || []),
      ...fields.flatMap((p) => [p.name, p.id, ...(p.aliases || [])]),
    ]
      .join(" ")
      .toLowerCase()
      .includes(search.toLowerCase());
  });
  return (
    <>
      <div
        className="ontology-model-view"
        role="group"
        aria-label={t("Model editing mode")}
      >
        <Button
          aria-pressed={view === "graph"}
          onClick={() => setView("graph")}
        >
          <Network size={16} />
          {t(" Graph")}
        </Button>
        <Button aria-pressed={view === "list"} onClick={() => setView("list")}>
          <List size={16} />
          {t(" List")}
        </Button>
      </div>
      {view === "graph" && (
        <OntologyGraph
          ontologyID={ontologyID}
          definition={definition}
          selected={entity?.id || ""}
          onSelect={onSelect}
          usage={usage}
          onInspect={(id, queries = false) => {
            inspectTrigger.current =
              document.activeElement as HTMLElement | null;
            onSelect(id);
            setInspected(id);
            setSection(queries ? "queries" : "definition");
          }}
          onAdd={onAdd}
          onEdit={onEdit}
          onConnect={onConnect}
          disabled={disabled}
        />
      )}
      {usageError && (
        <div>
          <ErrorNote error={usageError} />
          <Button onClick={() => setUsageRefresh((v) => v + 1)}>
            {t("Retry query usage")}
          </Button>
        </div>
      )}
      {view === "list" && (
        <div className="ontology-workspace">
          <section
            className="ontology-navigator"
            aria-label={t("Entity navigator")}
          >
            <div className="ontology-section-heading">
              <h2>
                {t("Entities ")}
                <small>{definition.entities.length}</small>
              </h2>
              <Button
                disabled={disabled}
                onClick={() => onAdd("entities")}
                aria-label={t("Add entity")}
              >
                <Plus size={16} />
              </Button>
            </div>
            <input
              aria-label={t("Find entity or property")}
              placeholder={t("Find an entity or property…")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <div className="ontology-entity-list">
              {matches.map((e) => (
                <button
                  key={e.id}
                  className={`ontology-entity-choice ${entity?.id === e.id ? "selected" : ""}`}
                  aria-pressed={entity?.id === e.id}
                  onClick={() => onSelect(e.id)}
                >
                  <Boxes size={17} />
                  <span>
                    <strong>{e.name}</strong>
                    <small>
                      {
                        definition.properties.filter((p) => p.entity === e.id)
                          .length
                      }{" "}
                      {t("properties")}
                      {e.parent
                        ? t(" · inherits {value1}", { value1: name(e.parent) })
                        : ""}
                    </small>
                    <small>
                      {usage
                        ? t(
                            "{value1} sources · {value2} executable templates",
                            {
                              value1: usage[e.id]?.sources || 0,
                              value2: usage[e.id]?.templates || 0,
                            },
                          )
                        : usageError
                          ? t("Usage unavailable")
                          : t("Loading query usage…")}
                    </small>
                  </span>
                </button>
              ))}
              {matches.length === 0 && (
                <p className="help">
                  {search
                    ? t("No matching entities or properties.")
                    : t(
                        "Start with a business concept such as Customer or Order.",
                      )}
                </p>
              )}
            </div>
            <p className="help">
              {t(
                "Select an entity to edit its properties and relationships together.",
              )}
            </p>
          </section>
          {entity ? (
            <div>
              <div
                className="button-row"
                role="group"
                aria-label={t("Entity detail section")}
              >
                <Button
                  aria-pressed={section === "definition"}
                  onClick={() => setSection("definition")}
                >
                  {t("Definition")}
                </Button>
                <Button
                  aria-pressed={section === "queries"}
                  onClick={() => setSection("queries")}
                >
                  {t("Queries and sources")}
                </Button>
              </div>
              {section === "definition" ? (
                <OntologyEntityDetails
                  definition={definition}
                  entity={entity}
                  onSelect={onSelect}
                  onAdd={onAdd}
                  onEdit={onEdit}
                  onRemove={onRemove}
                  disabled={disabled}
                />
              ) : (
                <ConceptUsage
                  key={`${entity.id}-${refreshKey}`}
                  ontologyID={ontologyID}
                  entityID={entity.id}
                  sources={sources}
                  agents={agents}
                  navigate={navigate}
                />
              )}
            </div>
          ) : (
            <Empty
              title={t("Build your business model")}
              description={t(
                "Create an entity, add its properties, then connect it to other entities.",
              )}
              action={
                <Button
                  primary
                  disabled={disabled}
                  onClick={() => onAdd("entities")}
                >
                  {t("Create the first entity")}
                </Button>
              }
            />
          )}
        </div>
      )}
      {view === "graph" && inspectedEntity && !overlayOpen && (
        <Drawer
          wide
          title={`${inspectedEntity.name} · ${t(section === "queries" ? "Queries and sources" : "Entity details")}`}
          subtitle={t(
            "{id} · Published usage stays scoped to each data source's adopted version.",
            { id: inspectedEntity.id },
          )}
          onClose={closeInspector}
          footer={
            <Button onClick={closeInspector}>{t("Back to canvas")}</Button>
          }
        >
          <div
            className="button-row"
            role="group"
            aria-label={t("Entity detail section")}
          >
            <Button
              aria-pressed={section === "definition"}
              onClick={() => setSection("definition")}
            >
              {t("Definition")}
            </Button>
            <Button
              aria-pressed={section === "queries"}
              onClick={() => setSection("queries")}
            >
              {t("Queries and sources")}
            </Button>
          </div>
          {section === "definition" ? (
            <OntologyEntityDetails
              key={inspectedEntity.id}
              definition={definition}
              entity={inspectedEntity}
              onSelect={(id) => {
                onSelect(id);
                setInspected(id);
              }}
              onAdd={onAdd}
              onEdit={onEdit}
              onRemove={onRemove}
              disabled={disabled}
            />
          ) : (
            <ConceptUsage
              key={`${inspectedEntity.id}-${refreshKey}`}
              ontologyID={ontologyID}
              entityID={inspectedEntity.id}
              sources={sources}
              agents={agents}
              navigate={navigate}
            />
          )}
        </Drawer>
      )}
    </>
  );
}
