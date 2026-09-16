import { httpTemplate } from "../src/http-template.ts";
import assert from "node:assert/strict";
import test from "node:test";
import {
  conceptQueries,
  linkQueryConcept,
  mappedConcepts,
} from "../src/query-concepts.ts";
import { emptyOntology } from "../src/ontology-types.ts";
import type { SemanticSnapshot } from "../src/semantic-types.ts";
import type { Readiness } from "../src/readiness.ts";
import { waitForClientCall } from "../src/client-call.ts";
import {
  parameterChoices,
  parameterError,
  parameterJSON,
  parameterValues,
  validateParameters,
} from "../src/template-parameters.ts";

test("client-call waiting matches the selected Agent and current execution, and stops on access changes", async () => {
  const identity = { source: "s", template: "q", version: "2", agent: "a" };
  const ready = {
    source_id: "s",
    active_agents: ["a"],
    templates: [{ id: "q", execution_version: "2", executable: true }],
    template_activity: {
      template_id: "q",
      execution_version: "2",
      agent_id: "a",
      successful_calls: 1,
    },
  } as Readiness;
  let calls = 0;
  const result = await waitForClientCall(
    async () => {
      calls++;
      return calls === 1
        ? {
            ...ready,
            template_activity: {
              ...ready.template_activity!,
              agent_id: "unrelated",
            },
          }
        : ready;
    },
    identity,
    new AbortController().signal,
    { interval: 1, timeout: 1000 },
  );
  assert.equal(result, "confirmed");
  assert.equal(calls, 2);
  for (const changed of [
    { ...ready, active_agents: [] },
    {
      ...ready,
      templates: [{ id: "q", execution_version: "3", executable: true }],
    },
  ]) {
    let checks = 0;
    assert.equal(
      await waitForClientCall(
        async () => {
          checks++;
          return changed as Readiness;
        },
        identity,
        new AbortController().signal,
      ),
      "changed",
    );
    assert.equal(checks, 1);
  }
});

test("client-call timeout and navigation cancellation abort the in-flight request without retrying", async () => {
  const identity = { source: "s", template: "q", version: "2", agent: "a" };
  for (const cancelled of [false, true]) {
    const abort = new AbortController();
    let calls = 0;
    let requestAborted = false;
    const waiting = waitForClientCall(
      (signal) =>
        new Promise((_, reject) => {
          calls++;
          signal.addEventListener(
            "abort",
            () => {
              requestAborted = true;
              reject(signal.reason);
            },
            { once: true },
          );
          if (cancelled) queueMicrotask(() => abort.abort());
        }),
      identity,
      abort.signal,
      { interval: 1, timeout: 15 },
    );
    if (cancelled) await assert.rejects(waiting, { name: "AbortError" });
    else assert.equal(await waiting, "timeout");
    assert.equal(calls, 1);
    assert.equal(requestAborted, true);
  }
});

const integer = {
  name: "id",
  type: "integer",
  required: true,
  pointers: ["/params/0"],
  minimum: "9007199254740992",
  maximum: "9007199254740994",
  enum_json: "[9007199254740993]",
};

test("concept links preserve query values and remain isolated to the mapped source snapshot", () => {
  const snapshot: SemanticSnapshot = {
    format_version: 2,
    overview: "",
    entries: [],
    ontology: {
      ontology_id: "commerce",
      version: "1",
      entities: [
        {
          entity: "customer",
          objects: [{ namespace: "public", object: "customers" }],
        },
      ],
      properties: [],
      relations: [],
    },
  };
  const entry = {
    id: "orders",
    kind: "template",
    name: "Orders",
    template: {
      enabled: true,
      tool: "query_sql",
      query_json: '{"query":"SELECT $1","params":[9007199254740993]}',
      parameters: [],
      example_json: "{}",
    },
  };
  snapshot.entries = [entry];
  const ref = "ontology:entity_type:customer";
  const linked = linkQueryConcept(snapshot, entry, "commerce", ref);
  assert.equal(linked.template?.query_json, entry.template.query_json);
  assert.deepEqual(linked.template?.concept_refs, [ref]);
  assert.deepEqual(linkQueryConcept(snapshot, linked, "commerce", ref), linked);
  assert.throws(() =>
    linkQueryConcept(snapshot, entry, "another-ontology", ref),
  );
  assert.throws(() =>
    linkQueryConcept(
      snapshot,
      entry,
      "commerce",
      "ontology:entity_type:private",
    ),
  );
  assert.equal(conceptQueries(snapshot, "commerce", "customer").length, 0);
  const draft = { ...snapshot, entries: [linked] };
  assert.equal(conceptQueries(draft, "commerce", "customer").length, 1);
  assert.equal(conceptQueries(draft, "another-ontology", "customer").length, 0);
  assert.equal(conceptQueries(snapshot, "commerce", "customer").length, 0);
});

test("concept choices use effective inherited properties and relationship queries are deduplicated", () => {
  const definition = emptyOntology();
  definition.entities = [
    { id: "record", name: "Record" },
    { id: "customer", name: "Customer", parent: "record" },
    { id: "order", name: "Order" },
  ];
  definition.properties = [
    {
      id: "id",
      name: "Identifier",
      entity: "record",
      type: "integer",
      required: true,
      multiple: false,
    },
    {
      id: "secret",
      name: "Private",
      entity: "customer",
      type: "string",
      required: false,
      multiple: false,
    },
  ];
  definition.relations = [
    {
      id: "places",
      name: "places",
      from: "customer",
      to: "order",
      directed: true,
      from_cardinality: { min: 0, max: null },
      to_cardinality: { min: 1, max: 1 },
    },
  ];
  const snapshot: SemanticSnapshot = {
    format_version: 2,
    overview: "",
    ontology: {
      ontology_id: "commerce",
      version: "1",
      entities: [
        { entity: "customer", objects: [] },
        { entity: "order", objects: [] },
      ],
      properties: [
        { entity: "customer", property: "id", template_id: "orders" },
      ],
      relations: [{ relation: "places", template_id: "orders" }],
    },
    entries: [
      {
        id: "orders",
        kind: "template",
        name: "Orders",
        template: {
          enabled: true,
          tool: "query_sql",
          query_json: "{}",
          parameters: [],
          example_json: "{}",
          concept_refs: ["ontology:relation_type:places"],
        },
      },
    ],
  };
  const options = mappedConcepts(snapshot.ontology!, definition);
  assert.equal(
    options.find((c) => c.ref === "ontology:property:customer:id")?.name,
    "Customer · Identifier",
  );
  assert.ok(!options.some((c) => c.ref.includes("secret")));
  assert.equal(
    conceptQueries(snapshot, "commerce", "customer", definition).length,
    1,
  );
  assert.equal(
    conceptQueries(snapshot, "commerce", "order", definition).length,
    1,
  );
});

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

test("native preview conversion preserves lossless values and derives only existing parameter slots", async () => {
  const { nativeTemplateQuery, suggestedParameters } = await import(
    "../src/query-publishing.ts"
  );
  const input =
    '{"source_id":"sensitive-connection-reference","agent_id":"preview","cursor":"old","max_rows":5,"tool":"query_sql","query":"SELECT $1, $2, $3","params":[9007199254740993,0.123456789012345678901,{"a/b~c":"x"}]}';
  const query = nativeTemplateQuery(input);
  assert.deepEqual(Object.keys(parameterValues(query)), ["query", "params"]);
  const suggestion = suggestedParameters(query, [
    "/params/0",
    "/params/1",
    "/params/2/a~1b~0c",
  ]);
  assert.equal(suggestion.examples.parameter_1, "9007199254740993");
  assert.equal(suggestion.examples.parameter_2, "0.123456789012345678901");
  assert.equal(suggestion.examples.a_b_c, '"x"');
  assert.equal(suggestion.parameters[0].type, "integer");
  assert.equal(suggestion.parameters[1].type, "number");
  assert.equal(
    suggestedParameters(query, ["/params/0"], suggestion.parameters).parameters
      .length,
    3,
  );
  assert.throws(() => suggestedParameters(query, ["/params/9"]));
  assert.throws(() =>
    nativeTemplateQuery('{"query":"SELECT 1","query":"SELECT 2"}'),
  );
});

test("resuming query setup stores only account-scoped references and tolerates unavailable storage", async () => {
  const { rememberJourney, savedJourney, savedJourneys } = await import(
    "../src/query-publishing.ts"
  );
  const values = new Map<string, string>();
  const original = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => values.get(key),
      setItem: (key: string, value: string) => values.set(key, value),
    },
  });
  try {
    const input = {
      source_id: "s",
      template_id: "t",
      agent_id: "a",
      query: "SECRET",
      token: "SECRET",
    };
    rememberJourney("one", input);
    assert.deepEqual(savedJourney("one"), {
      source_id: "s",
      template_id: "t",
      agent_id: "a",
    });
    assert.equal(savedJourney("two"), null);
    assert.ok(![...values.values()].join().includes("SECRET"));
    values.set("contextgate.query-journey.legacy", JSON.stringify(input));
    assert.deepEqual(savedJourneys("legacy"), [savedJourney("one")]);
    for (let n = 0; n < 7; n++)
      rememberJourney("one", { ...input, template_id: `t${n}` });
    rememberJourney("one", {
      ...input,
      template_id: "t4",
      agent_id: "new-agent",
    });
    const history = savedJourneys("one");
    assert.deepEqual(
      history.map((j) => j.template_id),
      ["t4", "t6", "t5", "t3", "t2"],
    );
    assert.equal(history[0].agent_id, "new-agent");
    assert.ok(
      ![...values.values()]
        .filter((v) => v !== JSON.stringify(input))
        .join()
        .includes("SECRET"),
    );
    rememberJourney("one", { ...input, source_id: "", template_id: "" });
    assert.deepEqual(savedJourneys("one"), history);
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      get() {
        throw new Error("disabled");
      },
    });
    assert.equal(savedJourney("one"), null);
    assert.doesNotThrow(() => rememberJourney("one", input));
  } finally {
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else Reflect.deleteProperty(globalThis, "localStorage");
  }
});

test("query search return links preserve filters and reading position within the catalog", async () => {
  const { catalogReturnURL } = await import("../src/query-publishing.ts");
  const search = new URLSearchParams({
    view: "queries",
    keyword: "客户 ?",
    source_id: "source-one",
    agent_id: "agent-one",
    offset: "20",
    revision: "published-snapshot",
    scroll_y: "760",
    focus: "source-one:query-two",
  });
  assert.equal(catalogReturnURL(`/business?${search}`), `/business?${search}`);
  assert.equal(catalogReturnURL("/business"), "/business");
  for (const target of [
    "https://example.com/business",
    "//example.com/business",
    "/business-other",
    "/settings",
    null,
  ])
    assert.equal(catalogReturnURL(target), "");
  assert.equal(
    catalogReturnURL(
      "/business?keyword=orders&token=secret&return_to=https://example.com",
    ),
    "/business?keyword=orders",
  );
});

test("query next action follows publication and exact client evidence, including deleted drafts", async () => {
  const { queryNextStep } = await import("../src/query-publishing.ts");
  const entry = {
    id: "orders",
    kind: "template",
    name: "Orders",
    template: {
      enabled: true,
      tool: "query_sql",
      query_json: '{"query":"SELECT $1","params":[1]}',
      parameters: [],
      example_json: "{}",
      execution_version: "2",
    },
  };
  const snapshot = { format_version: 2, overview: "", entries: [entry] };
  const state: import("../src/semantic-types.ts").SemanticState = {
    revision: "4",
    published_version: "2",
    changed: false,
    draft: structuredClone(snapshot),
    published: structuredClone(snapshot),
    validation: [{ id: "orders", valid: true, status: "valid" }],
  };
  const ready: import("../src/readiness.ts").Readiness = {
    source_id: "source",
    published_version: "2",
    draft_revision: "4",
    query_revision: "1",
    checked_at: "",
    activity_since: "",
    executable_templates: 1,
    active_agents: ["agent"],
    client_queries: 1,
    templates: [
      {
        id: "orders",
        name: "Orders",
        execution_version: "2",
        status: "valid",
        executable: true,
      },
    ],
    template_activity: {
      template_id: "orders",
      execution_version: "2",
      agent_id: "agent",
      successful_calls: 1,
      recent_calls: [],
    },
  };
  assert.equal(queryNextStep(state, ready, "orders", "agent"), "done");
  assert.equal(queryNextStep(state, ready, "orders", "other-agent"), "client");
  assert.equal(
    queryNextStep(
      state,
      {
        ...ready,
        template_activity: { ...ready.template_activity!, successful_calls: 0 },
      },
      "orders",
      "agent",
    ),
    "client",
  );
  assert.equal(
    queryNextStep(
      state,
      {
        ...ready,
        template_activity: {
          ...ready.template_activity!,
          execution_version: "1",
        },
      },
      "orders",
      "agent",
    ),
    "client",
  );
  assert.equal(
    queryNextStep(state, { ...ready, active_agents: [] }, "orders", "agent"),
    "access",
  );
  assert.equal(
    queryNextStep(
      state,
      { ...ready, published_version: "1" },
      "orders",
      "agent",
    ),
    "loading",
  );
  state.draft.entries[0].description = "Updated explanation";
  state.changed = true;
  assert.equal(queryNextStep(state, ready, "orders", "agent"), "publish");
  state.validation[0].valid = false;
  assert.equal(queryNextStep(state, ready, "orders", "agent"), "trial");
  state.draft.entries = [];
  assert.equal(queryNextStep(state, ready, "orders", "agent"), "publish");
  state.published.entries = [];
  assert.equal(queryNextStep(state, ready, "orders", "agent"), "query");
});
