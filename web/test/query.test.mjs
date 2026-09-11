import { test } from "node:test";
import assert from "node:assert/strict";
import { queryPayload } from "../src/query.ts";

test("preview and subsequent pages preserve native numeric parameters verbatim", () => {
  const text =
    '{"query":"SELECT ?","params":[9007199254740993,0.1234567890123456789012345],"named_params":{"n":9223372036854775807}}';
  for (const cursor of ["", "identity-bound-cursor"]) {
    const wire = queryPayload(text, "source", "query_sql", "agent", cursor);
    assert.ok(wire.includes(`"query":${text}`));
    const envelope = JSON.parse(wire);
    assert.equal(envelope.source_id, "source");
    assert.equal(envelope.agent_id, "agent");
    assert.equal(envelope.cursor, cursor);
  }
  assert.throws(() => queryPayload("null", "source", "query_sql"));
  assert.throws(() => queryPayload("[]", "source", "query_sql"));
  assert.throws(() => queryPayload('{"query":', "source", "query_sql"));
});

import { templatePayload } from "../src/semantic-types.ts";
test("template preview preserves numeric values on first and subsequent pages", () => {
  const params = '{"id":9007199254740993,"amount":0.1234567890123456789012345}';
  for (const cursor of ["", "template-cursor"]) {
    const wire = templatePayload(
      "source",
      { id: "orders", template: { execution_version: "7" } },
      params,
      "agent",
      cursor,
    );
    assert.ok(wire.includes(`"parameters":${params}`));
    assert.equal(JSON.parse(wire).execution_version, "7");
    assert.equal(JSON.parse(wire).cursor, cursor);
  }
  assert.throws(() => templatePayload("source", { id: "t" }, "[]", "agent"));
});
