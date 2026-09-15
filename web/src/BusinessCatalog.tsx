import { useEffect, useRef, useState } from "react";
import { Search, ArrowRight } from "lucide-react";
import { api, message } from "./api";
import { Button, Drawer, Empty, ErrorNote, Field, Loading } from "./components";
import { t } from "./i18n";
import { semanticsURL } from "./readiness";
import { TemplatePreview } from "./SemanticTools";
import { SemanticDefinition } from "./SemanticDefinition";
import { semanticKindLabel, type SemanticEntry } from "./semantic-types";
import type { Agent, Source } from "./types";
import "./business-workflows.css";

interface CatalogItem {
  id: string;
  kind: string;
  name: string;
  description: string;
  source_id: string;
  source_name: string;
  source_kind: string;
  published_version: string;
  unit?: string;
  grain?: string;
  time_definition?: string;
  ontology_id?: string;
  ontology_version?: string;
  entity_name?: string;
  templates: {
    id: string;
    name: string;
    execution_version: string;
    executable: boolean;
    status: string;
  }[];
  templates_limited: boolean;
}
interface CatalogPage {
  entries: CatalogItem[];
  total: number;
  revision: string;
  sources_limited: boolean;
  sources_scanned: number;
}
interface EntryDetail {
  entry: SemanticEntry;
  published_version: string;
  executable?: boolean;
  context_entries?: SemanticEntry[];
  context_limited?: boolean;
}
const kinds = [
  "template",
  "metric",
  "term",
  "entity_type",
  "property",
  "relation_type",
  "object",
  "field",
  "relationship",
];

export function BusinessCatalog({
  sources,
  agents,
  initialQuery,
  navigate,
}: {
  sources: Source[];
  agents: Agent[];
  initialQuery: string;
  navigate: (url: string) => void;
}) {
  const initial = new URLSearchParams(initialQuery);
  const [filters, setFilters] = useState({
    view: initial.get("view") || (initial.get("kind") ? "all" : "queries"),
    keyword: initial.get("keyword") || "",
    kind: initial.get("kind") || "",
    source_id: initial.get("source_id") || "",
    agent_id: initial.get("agent_id") || "",
  });
  const [data, setData] = useState<CatalogPage | null>(null);
  const [offset, setOffset] = useState(0);
  const [revision, setRevision] = useState("");
  const [refresh, setRefresh] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [detail, setDetail] = useState<{
    item: CatalogItem;
    data: EntryDetail;
  } | null>(null);
  const [preview, setPreview] = useState<{
    source: Source;
    entry: SemanticEntry;
  } | null>(null);
  const pending = useRef<AbortController | null>(null);
  const returnFocus = useRef<HTMLElement | null>(null);
  useEffect(() => () => pending.current?.abort(), []);
  useEffect(() => {
    const abort = new AbortController();
    setLoading(true);
    setError("");
    const query = new URLSearchParams(initialQuery);
    if (!query.has("view"))
      query.set("view", query.get("kind") ? "all" : "queries");
    query.set("offset", String(offset));
    if (revision) query.set("revision", revision);
    api<CatalogPage>(`/api/business-catalog?${query}`, { signal: abort.signal })
      .then(setData)
      .catch((e) => {
        if (!abort.signal.aborted) {
          setError(message(e));
          setData(null);
        }
      })
      .finally(() => {
        if (!abort.signal.aborted) setLoading(false);
      });
    return () => abort.abort();
  }, [initialQuery, offset, revision, refresh]);
  const selectedAgent = agents.find((a) => a.id === filters.agent_id);
  const visibleSources = sources.filter(
    (s) =>
      s.enabled && (!filters.agent_id || selectedAgent?.sources.includes(s.id)),
  );
  function search(nextFilters = filters) {
    const next = `/business?${new URLSearchParams(nextFilters)}`;
    setOffset(0);
    setRevision("");
    setRefresh((x) => x + 1);
    navigate(next);
  }
  async function open(item: CatalogItem, templateID = "") {
    if (busy) return;
    if (!detail) returnFocus.current = document.activeElement as HTMLElement;
    pending.current?.abort();
    const abort = new AbortController();
    pending.current = abort;
    setBusy(true);
    setError("");
    try {
      const query = new URLSearchParams({
        source_id: item.source_id,
        entry_id: templateID || item.id,
        published_version: item.published_version,
        agent_id: initial.get("agent_id") || "",
      });
      const result = await api<EntryDetail>(`/api/business-catalog?${query}`, {
        signal: abort.signal,
      });
      if (abort.signal.aborted) return;
      if (templateID) {
        const source = sources.find((s) => s.id === item.source_id);
        if (!source || !result.entry.template || !result.executable)
          throw new Error(t("Template changed. Refresh setup."));
        setPreview({ source, entry: result.entry });
      } else setDetail({ item, data: result });
    } catch (e) {
      if (!abort.signal.aborted) setError(message(e));
    } finally {
      if (!abort.signal.aborted) setBusy(false);
    }
  }
  const templateList = (item: CatalogItem) => (
    <>
      {item.templates.length ? (
        <ul className="catalog-templates">
          {item.templates.map((template) => (
            <li key={template.id}>
              <div>
                <strong>{template.name}</strong>
                <small
                  className={
                    template.executable ? "status green" : "status amber"
                  }
                >
                  {template.executable
                    ? t("Verified query")
                    : t("Needs validation")}
                  {" · "}
                  {t("Execution {version}", {
                    version: template.execution_version,
                  })}
                </small>
              </div>
              <Button
                primary={template.executable}
                disabled={busy}
                onClick={() =>
                  template.executable
                    ? open(item, template.id)
                    : navigate(
                        semanticsURL(
                          item.source_id,
                          "Query templates",
                          "",
                          initial.get("agent_id") || "",
                        ),
                      )
                }
              >
                {template.executable
                  ? t("Preview query")
                  : t("Repair template")}
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="help">
          {t("Definition only — no query template linked yet.")}
        </p>
      )}
      {item.templates_limited && (
        <p className="help">
          {t(
            "Showing 20 linked queries. Open the source workspace to see all templates.",
          )}
        </p>
      )}
    </>
  );
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Business catalog")}</h1>
          <p>
            {t(
              "Discover published business meaning and choose a verified query. Each query runs on one data source.",
            )}
          </p>
        </div>
        <Button onClick={() => navigate("/sources")}>
          {t("Manage data sources")}
        </Button>
      </div>
      <div
        className="button-row catalog-view"
        role="group"
        aria-label={t("Catalog view")}
      >
        {(["queries", "all"] as const).map((view) => (
          <Button
            key={view}
            primary={filters.view === view}
            aria-pressed={filters.view === view}
            onClick={() => search({ ...filters, view, kind: "" })}
          >
            {view === "queries" ? t("Available queries") : t("All definitions")}
          </Button>
        ))}
      </div>
      <form
        className="catalog-filters"
        onSubmit={(e) => {
          e.preventDefault();
          search();
        }}
      >
        <Field label={t("Search business catalog")}>
          <div className="search-input">
            <Search size={17} />
            <input
              value={filters.keyword}
              maxLength={256}
              placeholder={t("Name, alias or description")}
              onChange={(e) =>
                setFilters({ ...filters, keyword: e.target.value })
              }
            />
          </div>
        </Field>
        <Field label={t("Visible to")}>
          <select
            value={filters.agent_id}
            onChange={(e) =>
              setFilters({
                ...filters,
                agent_id: e.target.value,
                source_id: "",
              })
            }
          >
            <option value="">{t("Administrator · published content")}</option>
            {agents
              .filter(
                (a) =>
                  a.enabled &&
                  !a.revoked_at &&
                  new Date(a.expires_at) > new Date(),
              )
              .map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
          </select>
        </Field>
        <div className="button-row">
          <Button primary type="submit" disabled={busy}>
            {t("Search")}
          </Button>
          <Button
            disabled={busy}
            onClick={() => {
              setOffset(0);
              setRevision("");
              setRefresh((x) => x + 1);
            }}
          >
            {t("Refresh catalog")}
          </Button>
        </div>
        <details
          className="catalog-advanced"
          open={!!(initial.get("kind") || initial.get("source_id"))}
        >
          <summary>{t("Filter by type and source")}</summary>
          <div className="field-grid">
            <Field label={t("Content type")}>
              <select
                value={filters.kind}
                onChange={(e) =>
                  setFilters({ ...filters, kind: e.target.value })
                }
              >
                <option value="">{t("All types")}</option>
                {(filters.view === "queries"
                  ? ["template", "metric", "entity_type"]
                  : kinds
                ).map((k) => (
                  <option key={k} value={k}>
                    {t(semanticKindLabel(k))}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={t("Data source")}>
              <select
                value={filters.source_id}
                onChange={(e) =>
                  setFilters({ ...filters, source_id: e.target.value })
                }
              >
                <option value="">{t("All accessible sources")}</option>
                {visibleSources.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </Field>
          </div>
        </details>
      </form>
      <p className="help">
        {t(
          "Published content only. Identical names may use different business definitions; compare the source, grain and time window.",
        )}
      </p>
      <ErrorNote error={error} />
      {error && (
        <Button
          onClick={() => {
            setOffset(0);
            setRevision("");
            setRefresh((x) => x + 1);
          }}
        >
          {t("Restart search")}
        </Button>
      )}
      {loading ? (
        <Loading />
      ) : data ? (
        <>
          {data.sources_limited && (
            <p className="notice">
              {t(
                "Search covers the first 100 accessible sources. Select a data source to search outside this scope.",
              )}
            </p>
          )}
          {!data.entries.length ? (
            <Empty
              title={t("No published matches")}
              description={t(
                "Try another keyword or visibility filter. To add content, save and publish a source's semantic catalog.",
              )}
              action={
                <Button onClick={() => navigate("/sources")}>
                  {t("Manage data sources")}
                </Button>
              }
            />
          ) : (
            <div className="business-results">
              {data.entries.map((item) => (
                <article
                  className="business-result"
                  key={`${item.source_id}:${item.id}`}
                >
                  <div className="catalog-result-heading">
                    <div>
                      <span className="catalog-kind">
                        {t(semanticKindLabel(item.kind))}
                        {item.entity_name ? ` · ${item.entity_name}` : ""}
                      </span>
                      <h2>
                        <button
                          className="text-button name-link"
                          disabled={busy}
                          onClick={() => open(item)}
                        >
                          {item.name}
                        </button>
                      </h2>
                      <p className="catalog-source">
                        {item.source_name} · {item.source_kind} ·{" "}
                        {t("Publication {version}", {
                          version: item.published_version,
                        })}
                      </p>
                    </div>
                    {item.kind === "template" &&
                    item.templates[0]?.executable ? (
                      <Button
                        primary
                        disabled={busy}
                        onClick={() => open(item, item.id)}
                      >
                        {t("Preview query")}
                      </Button>
                    ) : (
                      <Button disabled={busy} onClick={() => open(item)}>
                        {t("View definition")}
                      </Button>
                    )}
                  </div>
                  {item.description && (
                    <p className="catalog-description">{item.description}</p>
                  )}
                  {(item.unit || item.grain || item.time_definition) && (
                    <dl className="business-facts">
                      {item.unit && (
                        <div>
                          <dt>{t("Unit")}</dt>
                          <dd>{item.unit}</dd>
                        </div>
                      )}
                      {item.grain && (
                        <div>
                          <dt>{t("Grain")}</dt>
                          <dd>{item.grain}</dd>
                        </div>
                      )}
                      {item.time_definition && (
                        <div>
                          <dt>{t("Time definition")}</dt>
                          <dd>{item.time_definition}</dd>
                        </div>
                      )}
                    </dl>
                  )}
                  {item.kind === "template" && item.templates[0]?.executable ? (
                    <small className="status green">
                      {t("Verified query")} ·{" "}
                      {t("Execution {version}", {
                        version: item.templates[0].execution_version,
                      })}
                    </small>
                  ) : (
                    templateList(item)
                  )}
                </article>
              ))}
            </div>
          )}
          <div className="catalog-pagination">
            <span>{t("{count} matching entries", { count: data.total })}</span>
            <Button
              disabled={offset === 0}
              onClick={() => {
                setRevision(data.revision);
                setOffset(Math.max(0, offset - 20));
                window.scrollTo({ top: 0 });
              }}
            >
              {t("Previous")}
            </Button>
            <span>{Math.floor(offset / 20) + 1}</span>
            <Button
              disabled={offset + 20 >= data.total}
              onClick={() => {
                setRevision(data.revision);
                setOffset(offset + 20);
                window.scrollTo({ top: 0 });
              }}
            >
              {t("Next")}
            </Button>
          </div>
        </>
      ) : null}
      {detail && !preview && (
        <Drawer
          wide
          returnFocus={returnFocus.current}
          title={detail.item.name}
          subtitle={`${detail.item.source_name} · ${t("Publication {version}", { version: detail.data.published_version })}`}
          onClose={() => {
            if (!busy) setDetail(null);
          }}
          footer={
            <Button disabled={busy} onClick={() => setDetail(null)}>
              {t("Close")}
            </Button>
          }
        >
          <ErrorNote error={error} />
          <p className="catalog-kind">
            {t(semanticKindLabel(detail.item.kind))}
          </p>
          <p className="catalog-description">
            {detail.data.entry.description || t("No description yet")}
          </p>
          {detail.data.entry.aliases?.length ? (
            <p>
              {t("Aliases")}: {detail.data.entry.aliases.join(" · ")}
            </p>
          ) : null}
          <SemanticDefinition
            entry={detail.data.entry}
            context={detail.data.context_entries}
            limited={detail.data.context_limited}
          />
          {detail.item.ontology_id && (
            <p className="help">
              {t("Ontology {id} · version {version}", {
                id: detail.item.ontology_id,
                version: detail.item.ontology_version || "",
              })}
            </p>
          )}
          <h3>{t("Available queries")}</h3>
          {templateList(detail.item)}
          <p className="help">
            {t(
              "Verified means the query passed its configured trial. Business definitions and answer correctness still require review.",
            )}
          </p>
          <div className="button-row">
            <Button
              disabled={busy}
              onClick={() =>
                navigate(`/sources/${detail.item.source_id}/setup`)
              }
            >
              {t("Open query workspace")}
              <ArrowRight size={15} />
            </Button>
            <Button
              disabled={busy}
              onClick={() =>
                navigate(
                  semanticsURL(
                    detail.item.source_id,
                    detail.item.id.startsWith("ontology:")
                      ? "Ontology mapping"
                      : detail.item.kind === "template"
                        ? "Query templates"
                        : "Catalog",
                  ),
                )
              }
            >
              {t("Manage definition")}
            </Button>
          </div>
        </Drawer>
      )}
      {preview && (
        <TemplatePreview
          returnFocus={returnFocus.current}
          contextLabel={detail?.item.name}
          backLabel={detail ? t("Back to concept") : undefined}
          source={preview.source}
          entry={preview.entry}
          agents={agents}
          initialAgentID={initial.get("agent_id") || ""}
          onClose={() => setPreview(null)}
        />
      )}
    </>
  );
}
