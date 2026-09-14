import assert from "node:assert/strict";
import test from "node:test";
import {
  arrangeEntities,
  edgeRoute,
  entityPositions,
  fitGraph,
  NODE_HEIGHT,
  NODE_WIDTH,
} from "../src/ontology-graph-layout.ts";

test("adding or removing definitions preserves the remaining manually arranged nodes", () => {
  const saved = {
    customer: { x: -320, y: 250 },
    removed: { x: 9000, y: 9000 },
  };
  const result = entityPositions(
    [
      { id: "customer", name: "Renamed customer" },
      { id: "order", name: "Order" },
    ],
    saved,
  );
  assert.deepEqual(result.customer, saved.customer);
  assert.ok(result.order.y > result.customer.y + NODE_HEIGHT);
  assert.deepEqual(Object.keys(result).sort(), ["customer", "order"]);
  assert.deepEqual(
    entityPositions(
      [
        { id: "customer", name: "Customer" },
        { id: "order", name: "Order" },
      ],
      result,
    ),
    result,
  );
});

test("fit includes self-relation labels and parallel curves on a narrow canvas", () => {
  const positions = { customer: { x: -100, y: 40 }, order: { x: 360, y: 40 } };
  const loop = edgeRoute(positions.customer, positions.customer, 0, 1, true);
  const forward = edgeRoute(positions.customer, positions.order, 0, 2, false);
  const reverse = edgeRoute(positions.order, positions.customer, 1, 2, false);
  assert.ok(loop.labelPoint.y < positions.customer.y);
  assert.ok(Math.abs(forward.labelPoint.y - reverse.labelPoint.y) >= 38);
  const labels = [loop, forward, reverse].map((edge) => edge.labelPoint);
  const view = fitGraph(positions, 320, 450, labels);
  const corners = Object.values(positions).flatMap((p) => [
    p,
    { x: p.x + NODE_WIDTH, y: p.y + NODE_HEIGHT },
  ]);
  corners.push(
    ...labels.flatMap((p) => [
      { x: p.x - 80, y: p.y - 20 },
      { x: p.x + 80, y: p.y + 20 },
    ]),
  );
  for (const p of corners) {
    assert.ok(
      p.x * view.scale + view.x >= 0 && p.x * view.scale + view.x <= 320,
    );
    assert.ok(
      p.y * view.scale + view.y >= 0 && p.y * view.scale + view.y <= 450,
    );
  }
});

test("fit keeps a maximum-size ontology reachable on a narrow canvas", () => {
  const positions = arrangeEntities(
    Array.from({ length: 500 }, (_, i) => ({
      id: `entity-${i}`,
      name: `Entity ${i}`,
    })),
  );
  const view = fitGraph(positions, 320, 450);
  for (const p of Object.values(positions)) {
    assert.ok(p.x * view.scale + view.x >= 0);
    assert.ok((p.x + NODE_WIDTH) * view.scale + view.x <= 320);
    assert.ok(p.y * view.scale + view.y >= 0);
    assert.ok((p.y + NODE_HEIGHT) * view.scale + view.y <= 450);
  }
});
