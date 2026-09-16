import type { ConceptCoverage } from "./query-concepts";
import { t } from "./i18n";
import { HelpTip } from "./HelpTip";
import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type PointerEvent,
} from "react";
import {
  Link2,
  Maximize,
  Plus,
  RotateCcw,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { Button, Empty } from "./components";
import { type OntologyDefinition } from "./ontology-types";
import type { OntologyItem, OntologyKind } from "./OntologyEditor";
import {
  arrangeEntities,
  arrowDelta,
  edgeRoute,
  entityPositions,
  fitGraph,
  MAX_ZOOM,
  MIN_ZOOM,
  NODE_HEIGHT,
  NODE_WIDTH,
  readPositions,
  type GraphView,
  type Point,
  type Positions,
} from "./ontology-graph-layout";
import "./ontology-graph.css";
import { OntologyGraphNodes } from "./OntologyGraphNodes";

interface Gesture {
  kind: "node" | "pan" | "connect";
  id: string;
  pointer: number;
  start: Point;
  origin: Point;
  moved: boolean;
}

export function OntologyGraph({
  ontologyID,
  definition,
  selected,
  onSelect,
  onInspect,
  usage,
  onAdd,
  onEdit,
  onConnect,
  disabled,
}: {
  ontologyID: string;
  definition: OntologyDefinition;
  selected: string;
  onSelect: (id: string) => void;
  onInspect: (id: string, queries?: boolean) => void;
  usage: Record<string, ConceptCoverage> | null;
  onAdd: (kind: OntologyKind, entity?: string) => void;
  onEdit: (kind: OntologyKind, item: OntologyItem) => void;
  onConnect: (from: string, to: string) => void;
  disabled: boolean;
}) {
  const storageKey = `mcpdbhub:ontology-layout:v1:${ontologyID}`;
  const [saved, setSaved] = useState(() => readPositions(storageKey));
  const positions = useMemo(
    () => entityPositions(definition.entities, saved),
    [definition.entities, saved],
  );
  const [view, setView] = useState<GraphView>({ x: 0, y: 0, scale: 1 });
  const [connecting, setConnecting] = useState("");
  const [pointer, setPointer] = useState<Point | null>(null);
  useEffect(() => {
    if (
      disabled ||
      (connecting && !definition.entities.some((e) => e.id === connecting))
    )
      cancelConnection();
  }, [disabled, connecting, definition.entities]);
  const [storageError, setStorageError] = useState(false);
  const canvas = useRef<HTMLDivElement>(null);
  const gesture = useRef<Gesture | null>(null);
  const initialFit = useRef(false);
  const livePositions = useRef(positions);
  livePositions.current = positions;
  const arrowID = useId();
  const name = (id: string) =>
    definition.entities.find((e) => e.id === id)?.name || id;
  const edges = useMemo(() => {
    const all = [
      ...definition.relations.map((r) => ({
        key: `relation:${r.id}`,
        from: r.from,
        to: r.to,
        label: r.name,
        directed: r.directed,
        item: r,
        kind: "relations" as const,
      })),
      ...definition.entities
        .filter((e) => e.parent)
        .map((e) => ({
          key: `parent:${e.id}`,
          from: e.id,
          to: e.parent!,
          label: "inherits",
          directed: true,
          item: e,
          kind: "entities" as const,
        })),
    ].filter(
      (edge) =>
        Object.hasOwn(positions, edge.from) &&
        Object.hasOwn(positions, edge.to),
    );
    const groups = new Map<string, typeof all>();
    for (const edge of all) {
      const key = JSON.stringify([edge.from, edge.to].sort());
      const group = groups.get(key) || [];
      group.push(edge);
      groups.set(key, group);
    }
    return [...groups.values()].flatMap((group) =>
      group.map((edge, index) => ({
        ...edge,
        index,
        total: group.length,
        ...edgeRoute(
          positions[edge.from],
          positions[edge.to],
          index,
          group.length,
          edge.from === edge.to,
        ),
      })),
    );
  }, [definition.entities, definition.relations, positions]);
  const liveEdges = useRef(edges);
  liveEdges.current = edges;

  function edgeLabels(next: Positions) {
    return liveEdges.current
      .filter((e) => Object.hasOwn(next, e.from) && Object.hasOwn(next, e.to))
      .map(
        (e) =>
          edgeRoute(next[e.from], next[e.to], e.index, e.total, e.from === e.to)
            .labelPoint,
      );
  }

  function persist(next: Positions) {
    setSaved(next);
    livePositions.current = next;
    try {
      localStorage.setItem(storageKey, JSON.stringify(next));
      setStorageError(false);
    } catch {
      setStorageError(true);
    }
  }

  function fit(next = positions) {
    if (canvas.current)
      setView(
        fitGraph(
          next,
          canvas.current.clientWidth,
          canvas.current.clientHeight,
          edgeLabels(next),
        ),
      );
  }

  useEffect(() => {
    const element = canvas.current;
    if (!element) return;
    const observer = new ResizeObserver(() => {
      if (element.clientWidth && element.clientHeight) {
        setView(
          fitGraph(
            livePositions.current,
            element.clientWidth,
            element.clientHeight,
            edgeLabels(livePositions.current),
          ),
        );
        initialFit.current = true;
      }
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [definition.entities.length > 0]);

  useEffect(() => {
    if (
      !initialFit.current ||
      !selected ||
      !canvas.current ||
      !positions[selected]
    )
      return;
    const p = positions[selected];
    const { clientWidth: width, clientHeight: height } = canvas.current;
    setView((v) => {
      const left = v.x + p.x * v.scale,
        top = v.y + p.y * v.scale;
      if (
        left >= 16 &&
        top >= 16 &&
        left + NODE_WIDTH * v.scale <= width - 16 &&
        top + NODE_HEIGHT * v.scale <= height - 60
      )
        return v;
      return {
        ...v,
        x: width / 2 - (p.x + NODE_WIDTH / 2) * v.scale,
        y: height / 2 - (p.y + NODE_HEIGHT / 2) * v.scale,
      };
    });
  }, [selected]);

  function zoom(factor: number) {
    if (!canvas.current) return;
    const cx = canvas.current.clientWidth / 2,
      cy = canvas.current.clientHeight / 2;
    setView((v) => {
      const scale = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, v.scale * factor));
      return {
        scale,
        x: cx - ((cx - v.x) * scale) / v.scale,
        y: cy - ((cy - v.y) * scale) / v.scale,
      };
    });
  }

  function worldPoint(e: PointerEvent): Point {
    const rect = canvas.current!.getBoundingClientRect();
    return {
      x: (e.clientX - rect.left - view.x) / view.scale,
      y: (e.clientY - rect.top - view.y) / view.scale,
    };
  }

  function begin(e: PointerEvent, kind: Gesture["kind"], id = "") {
    if (e.button !== 0 || gesture.current || (kind === "connect" && disabled))
      return;
    e.preventDefault();
    e.stopPropagation();
    if (e.currentTarget instanceof HTMLElement) e.currentTarget.focus();
    gesture.current = {
      kind,
      id,
      pointer: e.pointerId,
      start: { x: e.clientX, y: e.clientY },
      origin: kind === "node" ? positions[id] : view,
      moved: false,
    };
    canvas.current!.setPointerCapture(e.pointerId);
  }

  function move(e: PointerEvent) {
    const current = gesture.current;
    if (connecting) setPointer(worldPoint(e));
    if (!current || current.pointer !== e.pointerId) return;
    const dx = e.clientX - current.start.x,
      dy = e.clientY - current.start.y;
    current.moved ||= Math.hypot(dx, dy) > 4;
    if (!current.moved) return;
    if (current.kind === "pan")
      setView((v) => ({
        ...v,
        x: current.origin.x + dx,
        y: current.origin.y + dy,
      }));
    if (current.kind === "node") {
      const next = {
        ...livePositions.current,
        [current.id]: boundedPoint({
          x: current.origin.x + dx / view.scale,
          y: current.origin.y + dy / view.scale,
        }),
      };
      livePositions.current = next;
      setSaved(next);
    }
    if (current.kind === "connect") {
      setConnecting(current.id);
      setPointer(worldPoint(e));
    }
  }

  function cancelConnection() {
    setConnecting("");
    setPointer(null);
  }
  function edit(kind: OntologyKind, item: OntologyItem) {
    cancelConnection();
    onEdit(kind, item);
  }
  function add(kind: OntologyKind, id?: string) {
    cancelConnection();
    onAdd(kind, id);
  }

  function connectTo(id: string, from = connecting) {
    if (disabled) return;
    if (
      !Object.hasOwn(positions, id) ||
      (from && !Object.hasOwn(positions, from))
    ) {
      cancelConnection();
      return;
    }
    if (from) {
      cancelConnection();
      onConnect(from, id);
    } else {
      setConnecting(id);
      setPointer(null);
    }
  }

  function end(e: PointerEvent) {
    const current = gesture.current;
    if (!current || current.pointer !== e.pointerId) return;
    gesture.current = null;
    if (canvas.current?.hasPointerCapture(e.pointerId))
      canvas.current.releasePointerCapture(e.pointerId);
    if (current.kind === "node") {
      if (current.moved) persist(livePositions.current);
      else if (connecting) connectTo(current.id);
      else onInspect(current.id);
    }
    if (current.kind === "connect") {
      if (!current.moved) connectTo(current.id);
      else {
        const target = document
          .elementFromPoint(e.clientX, e.clientY)
          ?.closest<HTMLElement>("[data-graph-entity]");
        cancelConnection();
        if (target && canvas.current?.contains(target) && !disabled)
          onConnect(current.id, target.dataset.graphEntity!);
      }
    }
  }

  function cancelGesture() {
    const current = gesture.current;
    gesture.current = null;
    if (current?.kind === "node")
      setSaved({ ...livePositions.current, [current.id]: current.origin });
    if (current?.kind === "pan")
      setView((v) => ({ ...v, x: current.origin.x, y: current.origin.y }));
    if (current && canvas.current?.hasPointerCapture(current.pointer))
      canvas.current.releasePointerCapture(current.pointer);
    cancelConnection();
  }

  return (
    <section
      className="ontology-graph"
      aria-label={t("Graphical ontology editor")}
      onKeyDown={(e) => {
        if (e.key === "Escape" && connecting) {
          e.preventDefault();
          cancelGesture();
        }
      }}
    >
      <div className="ontology-graph-toolbar">
        <div className="button-row">
          <Button primary disabled={disabled} onClick={() => add("entities")}>
            <Plus size={15} />
            {t(" Add entity")}
          </Button>
          <Button
            disabled={disabled || !definition.entities.length}
            onClick={() => add("relations", selected)}
          >
            <Link2 size={15} />
            {t(" Add relationship")}
          </Button>
        </div>
        <div className="button-row">
          <select
            aria-label={t("Find entity on graph")}
            value={selected}
            onChange={(e) => {
              const id = e.target.value;
              onSelect(id);
              if (positions[id] && canvas.current)
                setView((v) => ({
                  ...v,
                  scale: Math.max(0.75, v.scale),
                  x:
                    canvas.current!.clientWidth / 2 -
                    (positions[id].x + NODE_WIDTH / 2) *
                      Math.max(0.75, v.scale),
                  y:
                    canvas.current!.clientHeight / 2 -
                    (positions[id].y + NODE_HEIGHT / 2) *
                      Math.max(0.75, v.scale),
                }));
            }}
          >
            <option value="" disabled>
              {t("Find an entity…")}
            </option>
            {definition.entities.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>
          <Button
            disabled={!definition.entities.length}
            onClick={() => {
              const next = arrangeEntities(definition.entities);
              persist(next);
              fit(next);
            }}
            title={t("Arrange nodes in a grid")}
          >
            <RotateCcw size={15} />
            {t(" Auto layout")}
          </Button>
          <HelpTip title={t("Canvas controls and keyboard shortcuts")}>
            <ul id="ontology-graph-help">
              <li>
                {t("Drag card headers to arrange; drag the background to pan.")}
              </li>
              <li>
                {t(
                  "Connect entities using their + points. Open entity details to edit properties, or click a relationship to edit it.",
                )}
              </li>
              <li>
                {t(
                  "Use arrow keys to move a focused card or pan the canvas; + / − to zoom, 0 to fit, Esc to cancel.",
                )}
              </li>
              <li>
                {t(
                  "Use Details on a card to open its properties and relationships.",
                )}
              </li>
            </ul>
            <p>
              {t(
                "Layout is saved in this browser only. Definition edits save to the draft and require publication.",
              )}
            </p>
          </HelpTip>
        </div>
      </div>
      {definition.entities.length ? (
        <>
          <div
            className={`ontology-graph-canvas ${connecting ? "connecting" : ""}`}
            ref={canvas}
            tabIndex={0}
            aria-label={t("Ontology canvas")}
            aria-describedby="ontology-graph-help"
            onPointerDown={(e) => {
              if (e.target === e.currentTarget) begin(e, "pan");
            }}
            onPointerMove={move}
            onPointerUp={end}
            onPointerCancel={cancelGesture}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                e.preventDefault();
                cancelGesture();
              }
              if (e.target !== e.currentTarget) return;
              const delta = arrowDelta(e.key, 40);
              if (delta) {
                e.preventDefault();
                setView((v) => ({ ...v, x: v.x + delta.x, y: v.y + delta.y }));
              }
              if (e.key === "+" || e.key === "=") {
                e.preventDefault();
                zoom(1.2);
              }
              if (e.key === "-") {
                e.preventDefault();
                zoom(1 / 1.2);
              }
              if (e.key === "0") {
                e.preventDefault();
                fit();
              }
            }}
          >
            <div
              className="ontology-graph-world"
              style={{
                transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`,
              }}
            >
              <svg
                className="ontology-graph-edges"
                role="group"
                aria-label={t("Relationships and inheritance")}
              >
                <defs>
                  <marker
                    id={arrowID}
                    viewBox="0 0 10 10"
                    refX="9"
                    refY="5"
                    markerWidth="7"
                    markerHeight="7"
                    orient="auto-start-reverse"
                  >
                    <path d="M 0 0 L 10 5 L 0 10 z" fill="context-stroke" />
                  </marker>
                </defs>
                {edges.map((edge) => (
                  <g
                    key={edge.key}
                    className={`ontology-graph-edge ${edge.kind === "entities" ? "inheritance" : ""}`}
                  >
                    <path
                      d={edge.path}
                      className="ontology-graph-edge-line"
                      markerEnd={edge.directed ? `url(#${arrowID})` : undefined}
                    />
                    <path
                      d={edge.path}
                      className="ontology-graph-edge-hit"
                      onClick={() => {
                        if (!disabled) edit(edge.kind, edge.item);
                      }}
                    />
                    <foreignObject
                      x={edge.labelPoint.x - 78}
                      y={edge.labelPoint.y - 19}
                      width={156}
                      height={38}
                    >
                      <button
                        disabled={disabled}
                        className="ontology-graph-edge-label"
                        title={`${name(edge.from)} ${edge.kind === "relations" ? edge.label : t("inherits")} ${name(edge.to)}`}
                        aria-label={
                          edge.kind === "relations"
                            ? t(
                                "Edit relationship {label} from {value2} to {value3}",
                                {
                                  label: edge.label,
                                  value2: name(edge.from),
                                  value3: name(edge.to),
                                },
                              )
                            : t("Edit inheritance of {value1}", {
                                value1: name(edge.from),
                              })
                        }
                        onClick={() => edit(edge.kind, edge.item)}
                      >
                        {edge.kind === "relations" ? edge.label : t("inherits")}
                      </button>
                    </foreignObject>
                  </g>
                ))}
                {connecting && positions[connecting] && pointer && (
                  <path
                    className="ontology-graph-connection-preview"
                    d={`M ${positions[connecting].x + NODE_WIDTH} ${positions[connecting].y + 35} L ${pointer.x} ${pointer.y}`}
                  />
                )}
              </svg>
              <OntologyGraphNodes
                usage={usage}
                definition={definition}
                positions={positions}
                selected={selected}
                connecting={connecting}
                onBegin={begin}
                onSelect={onSelect}
                onInspect={(id, queries) => {
                  cancelConnection();
                  onInspect(id, queries);
                }}
                onConnect={connectTo}
                disabled={disabled}
                onMove={(id, delta) =>
                  persist({
                    ...positions,
                    [id]: boundedPoint({
                      x: positions[id].x + delta.x,
                      y: positions[id].y + delta.y,
                    }),
                  })
                }
              />
            </div>
            <div
              className="ontology-graph-zoom"
              aria-label={t("Canvas zoom controls")}
            >
              <Button
                aria-label={t("Zoom out")}
                disabled={view.scale <= MIN_ZOOM}
                onClick={() => zoom(1 / 1.2)}
              >
                <ZoomOut size={16} />
              </Button>
              <output aria-label={t("Zoom level")}>
                {Math.round(view.scale * 1000) / 10}%
              </output>
              <Button
                aria-label={t("Zoom in")}
                disabled={view.scale >= MAX_ZOOM}
                onClick={() => zoom(1.2)}
              >
                <ZoomIn size={16} />
              </Button>
              <Button onClick={() => fit()}>
                <Maximize size={15} />
                {t(" Fit")}
              </Button>
            </div>
          </div>
          <div className="ontology-graph-status">
            <p role="status">
              {connecting
                ? t(
                    "Connecting from {value1}. Choose a target entity or connection point.",
                    { value1: name(connecting) },
                  )
                : t(
                    "{count} entities · {count2} relationships · Dashed lines show inheritance",
                    {
                      count: definition.entities.length,
                      count2: definition.relations.length,
                    },
                  )}
            </p>
            {connecting && (
              <Button onClick={cancelConnection}>
                {t("Cancel connection")}
              </Button>
            )}
          </div>
          {storageError && (
            <p className="notice warning">
              {t(
                "Layout could not be saved in this browser. Editing still works; positions may reset on reload.",
              )}
            </p>
          )}
        </>
      ) : (
        <Empty
          title={t("Draw your business model")}
          description={t(
            "Add your first entity, then connect it to another entity to define a relationship.",
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
    </section>
  );
}

function boundedPoint(p: Point): Point {
  return {
    x: Math.max(-100000, Math.min(100000, p.x)),
    y: Math.max(-100000, Math.min(100000, p.y)),
  };
}
