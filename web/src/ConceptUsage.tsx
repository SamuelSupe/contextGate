import { t } from "./i18n";
import { useEffect, useState } from "react";
import { api, message } from "./api";
import { Button, ErrorNote, Field, Loading } from "./components";
import { useReadiness, semanticsURL } from "./readiness";
import type { OntologyVersion } from "./ontology-types";
import type { Source, Agent } from "./types";
import "./product-workflows.css";
export function ConceptUsage({
  ontologyID,
  entityID,
  sources,
  agents,
  navigate,
}: {
  ontologyID: string;
  entityID: string;
  sources: Source[];
  agents: Agent[];
  navigate: (url: string) => void;
}) {
  const [selected, setSelected] = useState(sources[0]?.id || "");
  const source = sources.find((s) => s.id === selected) || sources[0];
  return (
    <section className="concept-usage">
      <h3>{t("Where it’s used")}</h3>
      <p className="help">
        {t(
          "Published mappings and executable templates, scoped to the adopted version in each source. Unsaved definition edits do not change Agent visibility.",
        )}
      </p>
      {!source ? (
        <p className="help">
          {t(
            "No published source bindings yet. Open a data source’s Semantics → Ontology mapping to adopt this ontology.",
          )}
        </p>
      ) : (
        <>
          <Field label={t("Inspect adopted data source")}>
            <select
              value={source.id}
              onChange={(e) => setSelected(e.target.value)}
            >
              {sources.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </Field>
          <SourceConcept
            key={source.id + entityID}
            ontologyID={ontologyID}
            entityID={entityID}
            source={source}
            agents={agents}
            navigate={navigate}
          />
        </>
      )}
    </section>
  );
}
function SourceConcept({
  ontologyID,
  entityID,
  source,
  agents,
  navigate,
}: {
  ontologyID: string;
  entityID: string;
  source: Source;
  agents: Agent[];
  navigate: (url: string) => void;
}) {
  const { data, error, refresh } = useReadiness(source.id, source.revision);
  const [version, setVersion] = useState<OntologyVersion | null>(null),
    [loadError, setLoadError] = useState(""),
    [agentID, setAgentID] = useState(""),
    [retry, setRetry] = useState(0);
  const binding =
    data?.ontology?.ontology_id === ontologyID ? data.ontology : null;
  useEffect(() => {
    setVersion(null);
    setLoadError("");
    if (!binding) return;
    const abort = new AbortController();
    api<OntologyVersion>(
      `/api/ontologies/${encodeURIComponent(ontologyID)}/versions/${binding.version}`,
      { signal: abort.signal },
    )
      .then(setVersion)
      .catch((e) => {
        if (!abort.signal.aborted) setLoadError(message(e));
      });
    return () => abort.abort();
  }, [ontologyID, binding?.version, retry]);
  const entity = binding?.entities.find((e) => e.entity === entityID);
  const properties =
    binding?.properties.filter((p) => p.entity === entityID) || [];
  const relationIDs = new Set(
    version?.definition.relations
      .filter((r) => r.from === entityID || r.to === entityID)
      .map((r) => r.id),
  );
  const relations =
    binding?.relations.filter((r) => relationIDs.has(r.relation)) || [];
  const refs = new Set([
    `ontology:entity_type:${entityID}`,
    ...properties.map((p) => `ontology:property:${entityID}:${p.property}`),
    ...relations.map((r) => `ontology:relation_type:${r.relation}`),
  ]);
  const ids = new Set(
    [
      ...properties.map((p) => p.template_id),
      ...relations.map((r) => r.template_id),
    ].filter(Boolean),
  );
  const templates = entity
    ? data?.templates.filter(
        (queryTemplate) =>
          ids.has(queryTemplate.id) ||
          queryTemplate.concept_refs?.some((ref) => refs.has(ref)),
      ) || []
    : [];
  const active = agents.filter((a) => data?.active_agents.includes(a.id));
  const agent = active.find((a) => a.id === agentID)?.id || active[0]?.id || "";
  return (
    <>
      <ErrorNote error={error || loadError} />
      {loadError && (
        <Button onClick={() => setRetry((v) => v + 1)}>
          {t("Retry adopted version")}
        </Button>
      )}
      {!data ? (
        error ? (
          <Button onClick={refresh}>{t("Retry usage")}</Button>
        ) : (
          <Loading />
        )
      ) : !binding ? (
        <p className="help">
          {t("This source no longer has a published binding to this ontology.")}
        </p>
      ) : (
        <div className="concept-source">
          <h4>
            {source.name}
            {t(" · ontology version ")}
            {binding.version}
          </h4>
          <p className="help">
            {t("Source publication ")}
            {data.published_version}
            {data.draft_ontology?.version !== binding.version
              ? t(" · A different binding is selected in the draft")
              : ""}
          </p>
          {!entity ? (
            <p>
              {t(
                "This entity is not mapped in the adopted version. It is not visible to Agents on this source.",
              )}
            </p>
          ) : (
            <>
              <p>
                <strong>{t("Physical objects")}</strong>
              </p>
              <ul>
                {entity.objects.map((o, i) => (
                  <li key={i}>
                    <code>
                      {o.namespace ? `${o.namespace}.` : ""}
                      {o.object}
                    </code>
                  </li>
                ))}
              </ul>
              <p className="help">
                {properties.length}
                {t(" mapped properties · ")}
                {relations.length} {t("mapped relationships")}
              </p>
              {properties.length > 0 && (
                <details>
                  <summary>{t("Property mappings")}</summary>
                  <ul>
                    {properties.map((p) => (
                      <li key={p.property}>
                        <strong>{p.property}</strong> →{" "}
                        {p.reference
                          ? `${p.reference.namespace}.${p.reference.object}.${p.reference.field || ""}`
                          : t("Template: {template_id}", {
                              template_id: p.template_id,
                            })}
                      </li>
                    ))}
                  </ul>
                </details>
              )}
              <Field label={t("Agent for template preview")}>
                <select
                  value={agent}
                  onChange={(e) => setAgentID(e.target.value)}
                >
                  {!active.length && (
                    <option value="">{t("No active authorized Agent")}</option>
                  )}
                  {active.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </select>
              </Field>
              {!version && !loadError ? (
                <Loading />
              ) : templates.length ? (
                <ul className="workflow-items">
                  {templates.map((queryTemplate) => (
                    <li key={queryTemplate.id}>
                      <div>
                        <strong>{queryTemplate.name}</strong>
                        <small>
                          {queryTemplate.executable
                            ? t("Executable")
                            : t(queryTemplate.status.replaceAll("_", " "))}
                        </small>
                      </div>
                      <Button
                        disabled={
                          !queryTemplate.executable || !source.enabled || !agent
                        }
                        onClick={() =>
                          navigate(
                            semanticsURL(
                              source.id,
                              "Query templates",
                              queryTemplate.id,
                              agent,
                            ),
                          )
                        }
                      >
                        {t("Preview as Agent")}
                      </Button>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="help">
                  {t(
                    "No published templates explicitly linked to this concept. Link a template or mapping to make the query path discoverable.",
                  )}
                </p>
              )}
            </>
          )}
          <div className="button-row">
            <Button
              onClick={() =>
                navigate(semanticsURL(source.id, "Ontology mapping"))
              }
            >
              {t("Open mapping")}
            </Button>
            <Button onClick={() => navigate(`/sources/${source.id}/setup`)}>
              {t("Agent setup")}
            </Button>
          </div>
        </div>
      )}
    </>
  );
}
