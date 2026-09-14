import { t } from "./i18n.ts";
// Validate syntax without serializing the parsed values: JavaScript numbers
// cannot represent every database integer or decimal parameter exactly.
export function queryPayload(
  text: string,
  sourceID: string,
  operation: string,
  agentID = "",
  cursor = "",
) {
  const value = JSON.parse(text);
  if (!value || Array.isArray(value) || typeof value !== "object")
    throw new Error(t("Query parameters must be a JSON object."));
  return `{"source_id":${JSON.stringify(sourceID)},"operation":${JSON.stringify(operation)},"agent_id":${JSON.stringify(agentID)},"cursor":${JSON.stringify(cursor)},"query":${text}}`;
}
