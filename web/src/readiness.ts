import { t } from "./i18n";
import { useEffect, useState } from "react";
import { api, message } from "./api";
import type { OntologyBinding } from "./ontology-types";
import type { Source } from "./types";
export interface Readiness {
  source_id: string;
  published_version: string;
  draft_revision: string;
  query_revision: string;
  checked_at: string;
  executable_templates: number;
  active_agents: string[];
  last_query?: string;
  ontology?: OntologyBinding;
  draft_ontology?: OntologyBinding;
  templates: {
    id: string;
    name: string;
    execution_version: string;
    status: string;
    executable: boolean;
    concept_refs?: string[];
  }[];
}
export function readinessLabel(source: Source, ready: Readiness) {
  if (!source.enabled) return t("Source disabled");
  if (!source.probe?.connected) return t("Check connection");
  if (
    source.query_access_mode === "templates_only" &&
    !ready.executable_templates
  )
    return t("Publish a verified template");
  if (!ready.active_agents.length) return t("Grant Agent access");
  if (source.probe.permission_status === "unverified")
    return t("Review protection evidence");
  return ready.last_query
    ? t("Client query recorded")
    : t("Ready for a client query");
}
export function useReadiness(sourceID: string, revision = "") {
  const [data, setData] = useState<Readiness | null>(null);
  const [error, setError] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    const abort = new AbortController();
    setData(null);
    setError("");
    api<Readiness>(`/api/sources/${encodeURIComponent(sourceID)}/readiness`, {
      signal: abort.signal,
    })
      .then(setData)
      .catch((e) => {
        if (!abort.signal.aborted) setError(message(e));
      });
    return () => abort.abort();
  }, [sourceID, revision, refreshKey]);
  return { data, error, refresh: () => setRefreshKey((v) => v + 1) };
}
export const semanticsURL = (
  id: string,
  tab = "Overview",
  template = "",
  agent = "",
) =>
  `/sources/${encodeURIComponent(id)}/semantics?${new URLSearchParams({ tab, template, agent_id: agent })}`;
