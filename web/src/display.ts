import { t } from "./i18n";
const labels: Record<string, string> = {
  "explicit SQL": "Explicit SQL pagination",
  "explicit Cypher": "Explicit Cypher pagination",
  "explicit query": "Explicit query pagination",
  "native cursor (5 minute TTL)": "Native cursor · 5 minutes",
  "native scan": "Native scan cursor",
  "native page state": "Native page state",
  search_after: "search_after cursor",
  read_only_transaction: "Read-only transaction",
  select_privileges: "SELECT-only account",
  select_guard_reader_role: "Restricted SELECT and dedicated reader role",
  "explicit SQL; bounded result partitions":
    "Explicit SQL pagination · bounded partitions",
  "explicit SQL; bounded result chunks":
    "Explicit SQL pagination · bounded chunks",
  "explicit SQL; bounded job result pages":
    "Explicit SQL pagination · bounded job pages",
  read_only_file: "Read-only database file",
  readonly_setting: "Engine read-only setting",
  read_role: "Read role and operation restrictions",
  command_allowlist: "Read command allowlist",
  query_api: "Fixed query APIs",
  declared_read_api: "Administrator-declared read API",
  "configured opaque token": "Configured API token pagination",
  engine_classification: "Engine statement classification",
};
export function capabilityLabel(value: string) {
  return t(labels[value] || value);
}
