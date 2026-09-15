import type { HTTPOperation } from "./types";
import type { SemanticEntry } from "./semantic-types";
import { parameterValues, parameterJSON } from "./template-parameters.ts";

export function httpTemplate(
  op: HTTPOperation,
): NonNullable<SemanticEntry["template"]> {
  const examples = parameterValues(op.example_json || "{}");
  // An absent optional API parameter may change server behavior if sent as an
  // empty value. Keep it absent until the administrator supplies a fixed value.
  const parameters = (op.parameters || []).filter(
    (p) => p.required || examples[p.name] !== undefined || !!p.default_json,
  );
  const fixed: Record<string, string> = Object.create(null);
  for (const p of parameters) {
    fixed[p.name] =
      examples[p.name] ??
      p.default_json ??
      (p.type === "string" ? '""' : p.type === "boolean" ? "false" : "0");
  }
  return {
    enabled: true,
    tool: "query_http_api",
    query_json: parameterJSON({
      operation: JSON.stringify(op.id),
      named_params: parameterJSON(fixed),
    }),
    parameters: parameters.map((p) => ({
      name: p.name,
      type: p.type,
      required: p.required,
      default_json: p.default_json,
      enum_json: p.enum_json,
      minimum: p.minimum,
      maximum: p.maximum,
      pointers: [
        "/named_params/" + p.name.replaceAll("~", "~0").replaceAll("/", "~1"),
      ],
    })),
    example_json: parameterJSON(
      Object.fromEntries(
        Object.entries(examples).filter(([key]) =>
          parameters.some((p) => p.name === key),
        ),
      ),
    ),
    result_description: (op.columns || [])
      .map((c) => c.name + ": " + c.type)
      .join("\n"),
  };
}
