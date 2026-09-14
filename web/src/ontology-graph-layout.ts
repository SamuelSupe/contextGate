import type { EntityType } from "./ontology-types";

export interface Point {
  x: number;
  y: number;
}
export interface GraphView extends Point {
  scale: number;
}
export type Positions = Record<string, Point>;
export const NODE_WIDTH = 248;
export const NODE_HEIGHT = 230;
export const MIN_ZOOM = 0.001;
export const MAX_ZOOM = 2;

export function arrangeEntities(entities: EntityType[]): Positions {
  const columns = Math.max(1, Math.ceil(Math.sqrt(entities.length)));
  return Object.fromEntries(
    entities.map((entity, index) => [
      entity.id,
      {
        x: (index % columns) * (NODE_WIDTH + 190),
        y: Math.floor(index / columns) * (NODE_HEIGHT + 140),
      },
    ]),
  );
}

export function readPositions(key: string): Positions {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(key) || "{}");
    if (!value || typeof value !== "object" || Array.isArray(value)) return {};
    return Object.fromEntries(
      Object.entries(value)
        .slice(0, 500)
        .filter(
          ([, p]) =>
            p &&
            typeof p === "object" &&
            Number.isFinite(p.x) &&
            Number.isFinite(p.y) &&
            Math.abs(p.x) <= 100000 &&
            Math.abs(p.y) <= 100000,
        ),
    );
  } catch {
    return {};
  }
}

export function entityPositions(
  entities: EntityType[],
  saved: Positions,
): Positions {
  const known = entities.filter((e) => Object.hasOwn(saved, e.id));
  if (!known.length) return arrangeEntities(entities);
  const missing = entities.filter((e) => !Object.hasOwn(saved, e.id));
  const extra = arrangeEntities(missing);
  const bottom =
    Math.max(...known.map((e) => saved[e.id].y)) + NODE_HEIGHT + 140;
  return Object.fromEntries(
    entities.map((e) => [
      e.id,
      Object.hasOwn(saved, e.id)
        ? saved[e.id]
        : { x: extra[e.id].x, y: extra[e.id].y + bottom },
    ]),
  );
}

export function fitGraph(
  positions: Positions,
  width: number,
  height: number,
  labels: Point[] = [],
): GraphView {
  const points = Object.values(positions).flatMap((p) => [
    p,
    { x: p.x + NODE_WIDTH, y: p.y + NODE_HEIGHT },
  ]);
  if (!points.length) return { x: width / 2, y: height / 2, scale: 1 };
  points.push(
    ...labels.flatMap((p) => [
      { x: p.x - 80, y: p.y - 20 },
      { x: p.x + 80, y: p.y + 20 },
    ]),
  );
  const left = Math.min(...points.map((p) => p.x)) - 40;
  const top = Math.min(...points.map((p) => p.y)) - 40;
  const right = Math.max(...points.map((p) => p.x)) + 40;
  const bottom = Math.max(...points.map((p) => p.y)) + 40;
  const scale = Math.max(
    MIN_ZOOM,
    Math.min(1, width / (right - left), height / (bottom - top)),
  );
  return {
    x: (width - (right + left) * scale) / 2,
    y: (height - (bottom + top) * scale) / 2,
    scale,
  };
}

// Curve both the path and its label together, including parallel and self relations.
export function edgeRoute(
  from: Point,
  to: Point,
  index: number,
  total: number,
  self: boolean,
) {
  const lane = (index - (total - 1) / 2) * 62;
  let start: Point, end: Point, c1: Point, c2: Point;
  if (self) {
    start = { x: from.x + 48, y: from.y };
    end = { x: from.x + NODE_WIDTH - 48, y: from.y };
    const reach = 135 + index * 80;
    c1 = { x: start.x - 70, y: start.y - reach };
    c2 = { x: end.x + 70, y: end.y - reach };
  } else {
    const dx = to.x - from.x,
      dy = to.y - from.y;
    if (Math.abs(dx) > Math.abs(dy)) {
      const direction = dx >= 0 ? 1 : -1;
      start = {
        x: from.x + (direction > 0 ? NODE_WIDTH : 0),
        y: from.y + NODE_HEIGHT / 2,
      };
      end = {
        x: to.x + (direction > 0 ? 0 : NODE_WIDTH),
        y: to.y + NODE_HEIGHT / 2,
      };
      const bend = Math.max(60, Math.abs(end.x - start.x) / 2);
      c1 = { x: start.x + direction * bend, y: start.y + lane };
      c2 = { x: end.x - direction * bend, y: end.y + lane };
    } else {
      const direction = dy >= 0 ? 1 : -1;
      start = {
        x: from.x + NODE_WIDTH / 2,
        y: from.y + (direction > 0 ? NODE_HEIGHT : 0),
      };
      end = {
        x: to.x + NODE_WIDTH / 2,
        y: to.y + (direction > 0 ? 0 : NODE_HEIGHT),
      };
      const bend = Math.max(60, Math.abs(end.y - start.y) / 2);
      c1 = { x: start.x + lane, y: start.y + direction * bend };
      c2 = { x: end.x + lane, y: end.y - direction * bend };
    }
  }
  return {
    path: `M ${start.x} ${start.y} C ${c1.x} ${c1.y}, ${c2.x} ${c2.y}, ${end.x} ${end.y}`,
    labelPoint: {
      x: (start.x + 3 * c1.x + 3 * c2.x + end.x) / 8,
      y: (start.y + 3 * c1.y + 3 * c2.y + end.y) / 8,
    },
  };
}

export function arrowDelta(key: string, step: number): Point | null {
  switch (key) {
    case "ArrowLeft":
      return { x: -step, y: 0 };
    case "ArrowRight":
      return { x: step, y: 0 };
    case "ArrowUp":
      return { x: 0, y: -step };
    case "ArrowDown":
      return { x: 0, y: step };
    default:
      return null;
  }
}
