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
  activity_since: string;
  client_queries: number;
  template_activity?: {
    template_id: string;
    execution_version: string;
    agent_id: string;
    successful_calls: number;
    recent_calls: { agent_id: string; request_id: string; at: string }[];
  };
  ontology?: OntologyBinding;
  draft_ontology?: OntologyBinding;
  templates: {
    id: string;
    name: string;
    description?: string;
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
  return ready.last_query
    ? t("Client query recorded")
    : t("Ready for a client query");
}
export function connectionReady(source: Source) {
  return !!(source.enabled && source.probe?.connected);
}
export function useReadiness(
  sourceID: string,
  revision = "",
  templateID = "",
  agentID = "",
) {
  const key = JSON.stringify([sourceID, revision, templateID, agentID]);
  const [snapshot, setSnapshot] = useState<{
    key: string;
    data: Readiness;
  } | null>(null);
  const [error, setError] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    const abort = new AbortController();
    setError("");
    if (!sourceID) return () => abort.abort();
    const query = new URLSearchParams({
      template_id: templateID,
      agent_id: agentID,
    });
    api<Readiness>(
      `/api/sources/${encodeURIComponent(sourceID)}/readiness?${query}`,
      {
        signal: abort.signal,
      },
    )
      .then((data) => {
        if (!abort.signal.aborted) setSnapshot({ key, data });
      })
      .catch((e) => {
        if (!abort.signal.aborted) {
          setSnapshot(null);
          setError(message(e));
        }
      });
    return () => abort.abort();
  }, [sourceID, key, templateID, agentID, refreshKey]);
  return {
    data: snapshot?.key === key ? snapshot.data : null,
    error,
    refresh: () => setRefreshKey((v) => v + 1),
  };
}
export const semanticsURL = (
  id: string,
  tab = "Overview",
  template = "",
  agent = "",
) =>
  `/sources/${encodeURIComponent(id)}/semantics?${new URLSearchParams({ tab, template, agent_id: agent })}`;
