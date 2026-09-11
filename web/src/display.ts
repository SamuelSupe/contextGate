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
  read_only_file: "Read-only database file",
  readonly_setting: "Engine read-only setting",
  read_role: "Read role and operation restrictions",
  command_allowlist: "Read command allowlist",
  query_api: "Fixed query APIs",
  engine_classification: "Engine statement classification",
};
export function capabilityLabel(value: string) {
  return labels[value] || value;
}
