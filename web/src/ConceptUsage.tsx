import { t } from "./i18n";
import { useEffect, useState } from "react";
import { api, APIError, message, payload } from "./api";
import { Button, ErrorNote, Field, Loading } from "./components";
import { useReadiness, semanticsURL } from "./readiness";
import { useOntologyVersion } from "./BusinessConcepts";
import {
  conceptQueries,
  coverageLabel,
  linkQueryConcept,
} from "./query-concepts";
import { queryToolURL, sameQueryEntry } from "./query-publishing";
import { TemplatePreview } from "./SemanticTools";
import { useNavigationGuard } from "./useNavigationGuard";
import type { SemanticEntry, SemanticState } from "./semantic-types";
import type { Source, Agent } from "./types";
import "./product-workflows.css";

export function ConceptUsage({
  ontologyID,
  entityID,
  sources,
  agents,
  navigate,
  initialSourceID = "",
  mappedSourceIDs,
}: {
  ontologyID: string;
  entityID: string;
  sources: Source[];
  agents: Agent[];
  navigate: (url: string) => void;
  initialSourceID?: string;
  mappedSourceIDs: string[];
}) {
  const [selected, setSelected] = useState(initialSourceID);
  const [showOther, setShowOther] = useState(
    !!initialSourceID && !mappedSourceIDs.includes(initialSourceID),
  );
  const mappedSources = sources.filter((s) => mappedSourceIDs.includes(s.id));
  const otherSources = sources.filter((s) => !mappedSourceIDs.includes(s.id));
  const [busy, setBusy] = useState(false);
  const source =
    sources.find((s) => s.id === selected) || mappedSources[0] || sources[0];
  return (
    <section className="concept-usage">
      {!source ? (
        <div className="empty-inline">
          <p>{t("Connect a data source to use this concept.")}</p>
          <Button onClick={() => navigate("/sources")}>
            {t("Data sources")}
          </Button>
        </div>
      ) : (
        <>
          {mappedSources.length === 1 &&
          !showOther &&
          source.id === mappedSources[0].id ? (
            <div className="section-heading">
              <div>
                <small className="help">{t("Data source")}</small>
                <p>
                  <strong>{source.name}</strong>
                </p>
              </div>
              {otherSources.length > 0 && (
                <Button disabled={busy} onClick={() => setShowOther(true)}>
                  {t("Connect another data source")}
                </Button>
              )}
            </div>
          ) : (
            <>
              <Field label={t("Data source")}>
                <select
                  disabled={busy}
                  value={source.id}
                  onChange={(e) => setSelected(e.target.value)}
                >
                  {mappedSources.length > 0 && (
                    <optgroup label={t("Mapped data sources")}>
                      {mappedSources.map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name}
                        </option>
                      ))}
                    </optgroup>
                  )}
                  {(showOther || !mappedSources.length) && (
                    <optgroup label={t("Other data sources")}>
                      {otherSources.map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name}
                        </option>
                      ))}
                    </optgroup>
                  )}
                </select>
              </Field>
              {!showOther &&
                mappedSources.length > 0 &&
                otherSources.length > 0 && (
                  <Button disabled={busy} onClick={() => setShowOther(true)}>
                    {t("Connect another data source")}
                  </Button>
                )}
            </>
          )}
          <SourceConcept
            key={source.id + entityID}
            ontologyID={ontologyID}
            entityID={entityID}
            source={source}
            agents={agents}
            navigate={navigate}
            onBusy={setBusy}
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
  onBusy,
}: {
  ontologyID: string;
  entityID: string;
  source: Source;
  agents: Agent[];
  navigate: (url: string) => void;
  onBusy: (busy: boolean) => void;
}) {
  const [reloadKey, setReloadKey] = useState(0);
  const ready = useReadiness(source.id, `${source.revision}:${reloadKey}`);
  const [state, setState] = useState<SemanticState | null>(null);
  const [error, setError] = useState("");
  const [linking, setLinking] = useState(false);
  const [selectedQuery, setSelectedQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [saved, setSaved] = useState("");
  const [preview, setPreview] = useState<SemanticEntry | null>(null);
  useNavigationGuard(false, busy);
  const endpoint = `/api/sources/${encodeURIComponent(source.id)}/semantics`;
  useEffect(() => {
    const abort = new AbortController();
    setError("");
    setConflict(false);
    setState(null);
    api<SemanticState>(endpoint, { signal: abort.signal })
      .then(setState)
      .catch((e) => {
        if (!abort.signal.aborted) setError(message(e));
      });
    return () => abort.abort();
  }, [endpoint, reloadKey]);
  const publishedBinding =
    state?.published.ontology?.ontology_id === ontologyID
      ? state.published.ontology
      : undefined;
  const draftBinding =
    state?.draft.ontology?.ontology_id === ontologyID
      ? state.draft.ontology
      : undefined;
  const publishedVersion = useOntologyVersion(publishedBinding);
  const sameVersion =
    !!draftBinding && draftBinding.version === publishedBinding?.version;
  const loadedDraftVersion = useOntologyVersion(
    sameVersion ? undefined : draftBinding,
  );
  const draftVersion = sameVersion ? publishedVersion : loadedDraftVersion;
  const entity = publishedBinding?.entities.find((m) => m.entity === entityID);
  const draftEntity = draftBinding?.entities.find((m) => m.entity === entityID);
  const ref = `ontology:entity_type:${entityID}`;
  const context = { ontology_id: ontologyID, concept_ref: ref };
  const publishedQueries = state
    ? conceptQueries(
        state.published,
        ontologyID,
        entityID,
        publishedVersion.data?.definition,
      )
    : [];
  const draftQueries = state
    ? conceptQueries(
        state.draft,
        ontologyID,
        entityID,
        draftVersion.data?.definition,
      )
    : [];
  const queries = [
    ...publishedQueries,
    ...draftQueries.filter((e) => !publishedQueries.some((p) => p.id === e.id)),
  ];
  const candidates =
    state?.draft.entries.filter(
      (e) => e.template && !e.template.concept_refs?.includes(ref),
    ) || [];
  const activeAgent = agents.find((a) =>
    ready.data?.active_agents.includes(a.id),
  );
  const executable = publishedQueries.filter(
    (entry) =>
      source.enabled &&
      ready.data?.templates.some((r) => r.id === entry.id && r.executable),
  ).length;
  const loadingVersion =
    (!!publishedBinding && !publishedVersion.data && !publishedVersion.error) ||
    (!!draftBinding && !draftVersion.data && !draftVersion.error);
  const loadError =
    error || ready.error || publishedVersion.error || draftVersion.error;
  async function link() {
    const entry = candidates.find((e) => e.id === selectedQuery);
    if (!state || !entry) return;
    setBusy(true);
    onBusy(true);
    setError("");
    try {
      const linked = linkQueryConcept(state.draft, entry, ontologyID, ref);
      const next = await api<SemanticState>(
        `${endpoint}/entries/${encodeURIComponent(entry.id)}`,
        {
          method: "PUT",
          body: payload({ revision: state.revision, entry: linked }),
        },
      );
      setState(next);
      setSaved(entry.id);
      setLinking(false);
      setSelectedQuery("");
      ready.refresh();
    } catch (e) {
      setConflict(e instanceof APIError && e.detail.code === "conflict");
      setError(message(e));
    } finally {
      setBusy(false);
      onBusy(false);
    }
  }
  function refresh() {
    setReloadKey((n) => n + 1);
    publishedVersion.retry();
    loadedDraftVersion.retry();
    setSaved("");
  }
  return (
    <>
      <ErrorNote error={loadError} />
      {loadError && (
        <Button disabled={busy} onClick={refresh}>
          {t("Reload queries")}
        </Button>
      )}
      {(!state || !ready.data || loadingVersion) && !loadError ? (
        <Loading />
      ) : (
        state &&
        ready.data &&
        !loadingVersion &&
        !publishedVersion.error &&
        !draftVersion.error &&
        !ready.error && (
          <>
            <div className="concept-coverage">
              <span className={`status ${executable ? "green" : "muted"}`}>
                {t(
                  coverageLabel({
                    sources: entity ? 1 : 0,
                    linked_templates: publishedQueries.length,
                    templates: executable,
                  }),
                )}
              </span>
              {publishedBinding && (
                <small>
                  {t("Adopted version {version}", {
                    version: publishedBinding.version,
                  })}
                </small>
              )}
              <Button disabled={busy} onClick={refresh}>
                {t("Refresh")}
              </Button>
            </div>
            {!!entity && (
              <p className="help">
                {t("{count} executable queries", { count: executable })}
              </p>
            )}
            {!draftEntity && (
              <div className="notice">
                <span>
                  {t(
                    "Map this concept in the source draft before linking queries.",
                  )}
                </span>
                <Button
                  onClick={() =>
                    navigate(semanticsURL(source.id, "Ontology mapping"))
                  }
                >
                  {t("Set up mapping")}
                </Button>
              </div>
            )}
            {!!draftEntity && !entity && (
              <p className="help">
                {t("Mapping in draft · not visible to Agents yet")}
              </p>
            )}
            {saved && (
              <div className="notice success" role="status">
                <span>{t("Association saved to draft.")}</span>
                <Button
                  onClick={() =>
                    navigate(queryToolURL(source.id, saved, "", context))
                  }
                >
                  {t("Review and publish")}
                </Button>
              </div>
            )}
            <div className="button-row">
              <Button
                primary
                disabled={busy || !draftEntity}
                onClick={() =>
                  navigate(queryToolURL(source.id, "", "", context, true))
                }
              >
                {t("Create query")}
              </Button>
              <Button
                disabled={busy || !draftEntity}
                aria-expanded={linking}
                onClick={() => setLinking(!linking)}
              >
                {t("Link existing query")}
              </Button>
            </div>
            {linking && (
              <section className="concept-link-form">
                <Field label={t("Query to link")}>
                  <select
                    disabled={busy}
                    value={selectedQuery}
                    onChange={(e) => setSelectedQuery(e.target.value)}
                  >
                    <option value="">{t("Select a query")}</option>
                    {candidates.map((entry) => (
                      <option key={entry.id} value={entry.id}>
                        {entry.name}
                      </option>
                    ))}
                  </select>
                </Field>
                {!candidates.length && (
                  <p className="help">
                    {t("No unlinked query drafts in this source.")}
                  </p>
                )}
                <div className="button-row">
                  <Button
                    primary
                    busy={busy}
                    disabled={!selectedQuery || conflict}
                    onClick={link}
                  >
                    {t("Save association")}
                  </Button>
                  <Button disabled={busy} onClick={() => setLinking(false)}>
                    {t("Cancel")}
                  </Button>
                </div>
              </section>
            )}
            <h3>{t("Linked queries")}</h3>
            {queries.length ? (
              <ul className="workflow-items concept-query-list">
                {queries.map((entry) => {
                  const published = publishedQueries.find(
                    (e) => e.id === entry.id,
                  );
                  const draft = state.draft.entries.find(
                    (e) => e.id === entry.id,
                  );
                  const inDraft = draftQueries.some((e) => e.id === entry.id);
                  const evidence = ready.data?.templates.find(
                    (e) => e.id === entry.id,
                  );
                  const available =
                    !!published && source.enabled && evidence?.executable;
                  return (
                    <li key={entry.id}>
                      <div>
                        <strong>{entry.name}</strong>
                        <small>
                          {t(
                            published
                              ? available
                                ? "Executable"
                                : source.enabled
                                  ? evidence?.status.replaceAll("_", " ") ||
                                    "Unavailable"
                                  : "Source disabled"
                              : "Draft association",
                          )}
                        </small>
                        {published &&
                          (!inDraft || !sameQueryEntry(published, draft)) && (
                            <small>{t("Unpublished changes")}</small>
                          )}
                      </div>
                      <div className="button-row">
                        <Button
                          disabled={!available || busy}
                          onClick={() => setPreview(published!)}
                        >
                          {t("Preview")}
                        </Button>
                        {draft && (
                          <Button
                            disabled={busy}
                            onClick={() =>
                              navigate(
                                queryToolURL(
                                  source.id,
                                  entry.id,
                                  "",
                                  context,
                                  true,
                                ),
                              )
                            }
                          >
                            {t("Edit query")}
                          </Button>
                        )}
                      </div>
                    </li>
                  );
                })}
              </ul>
            ) : (
              <p className="help">{t("No queries linked yet")}</p>
            )}
            {(entity || draftEntity) && (
              <details className="concept-mapping-details">
                <summary>{t("Source mapping")}</summary>
                {(entity || draftEntity)!.objects.map((object, i) => (
                  <p key={i}>
                    <code>
                      {[object.namespace, object.object]
                        .filter(Boolean)
                        .join(".")}
                    </code>
                  </p>
                ))}
                <Button
                  disabled={busy}
                  onClick={() =>
                    navigate(semanticsURL(source.id, "Ontology mapping"))
                  }
                >
                  {t("Manage mapping")}
                </Button>
              </details>
            )}
          </>
        )
      )}
      {preview && (
        <TemplatePreview
          source={source}
          entry={preview}
          agents={agents}
          initialAgentID={activeAgent?.id || ""}
          backLabel={t("Back to concept")}
          onClose={() => setPreview(null)}
        />
      )}
    </>
  );
}
