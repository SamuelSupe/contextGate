import { httpTemplate } from "../src/http-template.ts";
import assert from "node:assert/strict";
import test from "node:test";
import {
  parameterChoices,
  parameterError,
  parameterJSON,
  parameterValues,
  validateParameters,
} from "../src/template-parameters.ts";

const integer = {
  name: "id",
  type: "integer",
  required: true,
  pointers: ["/params/0"],
  minimum: "9007199254740992",
  maximum: "9007199254740994",
  enum_json: "[9007199254740993]",
};

test("editing and switching parameter views preserves database values without numeric rounding", () => {
  const raw =
    '{"id":9007199254740993,"amount":0.123456789012345678901,"object":{"path":"a,b:c\\\"","values":[1e1000,null]},"__proto__":"safe"}';
  const values = parameterValues(raw);
  values.id = "9007199254740994";
  const roundTrip = parameterValues(parameterJSON(values));
  assert.equal(roundTrip.id, "9007199254740994");
  assert.equal(roundTrip.amount, "0.123456789012345678901");
  assert.equal(roundTrip.object, '{"path":"a,b:c\\\"","values":[1e1000,null]}');
  assert.equal(roundTrip.__proto__, '"safe"');
  assert.equal(parameterValues("{}").constructor, undefined);
  assert.equal(parameterValues("{}").toString, undefined);
  assert.deepEqual(parameterChoices(integer), ["9007199254740993"]);
});

test("parameter forms enforce exact ranges, required values, enum choices and numeric types", () => {
  assert.equal(parameterError(integer, "9007199254740993"), "");
  assert.equal(parameterError(integer, "9007199254740993e0"), "");
  for (const raw of [
    "9007199254740992",
    "9007199254740995",
    "9007199254740993.1",
    '"9007199254740993"',
    "true",
    "1e10001",
    "",
  ])
    assert.notEqual(parameterError(integer, raw), "", raw);
  assert.notEqual(parameterError(integer), "");
  assert.equal(
    parameterError({ ...integer, default_json: "9007199254740993" }),
    "",
  );
  assert.equal(parameterError({ ...integer, required: false }), "");
  assert.equal(
    parameterError(
      {
        ...integer,
        type: "number",
        minimum: "0.123456789012345678901",
        maximum: "0.123456789012345678901",
        enum_json: "",
      },
      "0.123456789012345678901",
    ),
    "",
  );
});

test("advanced JSON cannot silently lose unknown or duplicate parameters", () => {
  assert.notEqual(
    validateParameters([integer], '{"id":9007199254740993,"query":"extra"}'),
    "",
  );
  assert.throws(() => parameterValues('{"id":1,"id":2}'));
  assert.throws(() => parameterValues("[]"));
  assert.throws(() => parameterValues('{"id":}'));
  assert.equal(validateParameters([integer], '{"id":9007199254740993}'), "");
  assert.equal(
    parameterError(
      {
        name: "filter",
        type: "object",
        required: true,
        pointers: ["/params/0"],
        enum_json: '[{"id":9007199254740993,"tags":["a,b"]}]',
      },
      '{"tags":["a,b"],"id":9007199254740993}',
    ),
    "",
  );
});

test("API template drafts inherit contracts and preserve exact example values", () => {
  const template = httpTemplate({
    id: "lookup",
    name: "Lookup",
    method: "GET",
    path: "/customers/{id}",
    read_only: true,
    example_json: '{"id":9007199254740993,"price":0.123456789012345678901}',
    parameters: [
      {
        name: "optional_filter",
        type: "string",
        in: "query",
        target: "filter",
        required: false,
      },
      { ...integer, name: "id", type: "integer", in: "path", target: "id" },
      {
        name: "price",
        type: "number",
        in: "query",
        target: "price",
        required: false,
        default_json: "0.123456789012345678901",
      },
    ],
  });
  const query = parameterValues(template.query_json);
  assert.equal(parameterValues(query.named_params).id, "9007199254740993");
  assert.equal(parameterValues(query.named_params).optional_filter, undefined);
  assert.equal(
    template.parameters.some((p) => p.name === "optional_filter"),
    false,
  );
  assert.equal(
    parameterValues(query.named_params).price,
    "0.123456789012345678901",
  );
  assert.equal(template.parameters[0].pointers[0], "/named_params/id");
  assert.equal(template.parameters[0].maximum, integer.maximum);
  assert.equal(
    validateParameters(template.parameters, template.example_json),
    "",
  );
});
