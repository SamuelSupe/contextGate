import { t } from "./i18n.ts";
import type { SemanticParameter } from "./semantic-types";

// Keep raw value slices: parsing and serializing numbers would round database
// integers and decimals before the server gets a chance to bind them.
function parts(text: string): string[] {
  JSON.parse(text);
  const inner = text.trim().slice(1, -1);
  const result: string[] = [];
  let start = 0,
    depth = 0,
    quoted = false,
    escaped = false;
  for (let i = 0; i < inner.length; i++) {
    const c = inner[i];
    if (quoted) {
      if (escaped) escaped = false;
      else if (c === "\\") escaped = true;
      else if (c === '"') quoted = false;
    } else if (c === '"') quoted = true;
    else if (c === "{" || c === "[") depth++;
    else if (c === "}" || c === "]") depth--;
    else if (c === "," && depth === 0) {
      result.push(inner.slice(start, i).trim());
      start = i + 1;
    }
  }
  if (inner.trim()) result.push(inner.slice(start).trim());
  return result;
}

export function parameterValues(text: string): Record<string, string> {
  if (!text.trim().startsWith("{"))
    throw new Error(t("Parameters must be a JSON object."));
  const entries = parts(text).map((part) => {
    const key = part.match(/^"(?:\\.|[^"\\])*"\s*:/)![0];
    return [
      JSON.parse(key.slice(0, key.lastIndexOf(":"))),
      part.slice(key.length).trim(),
    ];
  });
  if (new Set(entries.map(([key]) => key)).size !== entries.length)
    throw new Error(t("Each parameter name must appear only once."));
  return Object.assign(Object.create(null), Object.fromEntries(entries));
}

export function parameterJSON(values: Record<string, string>): string {
  return `{\n${Object.entries(values)
    .map(([key, value]) => `  ${JSON.stringify(key)}: ${value}`)
    .join(",\n")}\n}`;
}

export function parameterAt(text: string, pointer: string): string {
  if (!pointer.startsWith("/")) throw new Error("Invalid parameter position.");
  let value = text;
  for (const part of pointer.slice(1).split("/")) {
    const key = part.replaceAll("~1", "/").replaceAll("~0", "~");
    const next = value.trim().startsWith("[")
      ? parts(value)[Number(key)]
      : parameterValues(value)[key];
    if (next === undefined)
      throw new Error("Parameter position no longer exists.");
    value = next;
  }
  return value;
}

export function parameterChoices(parameter: SemanticParameter): string[] {
  return parameter.enum_json ? parts(parameter.enum_json) : [];
}

function decimal(raw: string): [bigint, bigint] {
  const match = raw.match(/^(-?)(0|[1-9]\d*)(?:\.(\d+))?(?:[eE]([+-]?\d+))?$/);
  if (!match || raw.length > 1024 || Math.abs(Number(match[4] || 0)) > 10000)
    throw new Error(
      t(
        "Enter a JSON number (up to 1,024 characters; exponent within ±10,000).",
      ),
    );
  const scale = Number(match[4] || 0) - (match[3]?.length || 0);
  const coefficient = BigInt(match[1] + match[2] + (match[3] || ""));
  return scale >= 0
    ? [coefficient * 10n ** BigInt(scale), 1n]
    : [coefficient, 10n ** BigInt(-scale)];
}

function compare(a: string, b: string): bigint {
  const [an, ad] = decimal(a),
    [bn, bd] = decimal(b);
  return an * bd - bn * ad;
}

function equalJSON(a: string, b: string, nested = false): boolean {
  const av = JSON.parse(a),
    bv = JSON.parse(b);
  if (typeof av !== typeof bv || Array.isArray(av) !== Array.isArray(bv))
    return false;
  if (typeof av === "number") return nested ? a === b : compare(a, b) === 0n;
  if (av === null || bv === null || typeof av !== "object") return av === bv;
  if (Array.isArray(av)) {
    const aa = parts(a),
      bb = parts(b);
    return (
      aa.length === bb.length && aa.every((v, i) => equalJSON(v, bb[i], true))
    );
  }
  const aa = parameterValues(a),
    bb = parameterValues(b);
  return (
    Object.keys(aa).length === Object.keys(bb).length &&
    Object.entries(aa).every(
      ([k, v]) => Object.hasOwn(bb, k) && equalJSON(v, bb[k], true),
    )
  );
}

export function parameterError(p: SemanticParameter, raw?: string): string {
  if (raw === undefined)
    return p.required && !p.default_json
      ? t("This parameter is required.")
      : "";
  try {
    const value = JSON.parse(raw);
    const type =
      value === null ? "null" : Array.isArray(value) ? "array" : typeof value;
    if (type !== (p.type === "integer" ? "number" : p.type))
      return t("Enter a value of type {value1}.", { value1: p.type });
    if (type === "number") {
      const [n, d] = decimal(raw.trim());
      if (p.type === "integer" && n % d !== 0n)
        return t("Enter a whole number.");
      if (p.minimum && compare(raw.trim(), p.minimum) < 0n)
        return t("Minimum: {value1}.", { value1: p.minimum });
      if (p.maximum && compare(raw.trim(), p.maximum) > 0n)
        return t("Maximum: {value1}.", { value1: p.maximum });
    }
    const choices = parameterChoices(p);
    if (choices.length && !choices.some((v) => equalJSON(v, raw.trim())))
      return t("Choose one of the allowed values.");
    return "";
  } catch (e) {
    return e instanceof SyntaxError
      ? t("Enter valid {value1} JSON.", {
          value1: p.type === "integer" ? "number" : p.type,
        })
      : (e as Error).message;
  }
}

export function validateParameters(
  parameters: SemanticParameter[],
  text: string,
): string {
  try {
    const values = parameterValues(text);
    const unknown = Object.keys(values).find(
      (key) => !parameters.some((p) => p.name === key),
    );
    if (unknown)
      return t(
        "Unknown parameter: {value1}. Remove it or check the template.",
        { value1: unknown },
      );
    for (const p of parameters) {
      const error = parameterError(p, values[p.name]);
      if (error) return `${p.name}: ${error}`;
    }
    return "";
  } catch (e) {
    return e instanceof SyntaxError
      ? t("Enter a valid JSON object.")
      : (e as Error).message;
  }
}
