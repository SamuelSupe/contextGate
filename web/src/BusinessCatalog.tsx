import { useEffect, useRef, useState } from "react";
import { Search, ArrowRight } from "lucide-react";
import { api, APIError, message } from "./api";
import { Button, Drawer, Empty, ErrorNote, Field, Loading } from "./components";
import { t } from "./i18n";
import { queryToolURL } from "./query-publishing";
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
  draft_change?: string;
  query_status?: string;
  matched_definitions?: { id: string; kind: string; name: string }[];
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

function catalogError(error: unknown) {
  return error instanceof APIError && error.detail.code === "conflict"
    ? "Catalog or access changed; restart the search"
    : message(error);
}

function QueryRow({
  item,
  busy,
  open,
  preview,
}: {
  item: CatalogItem;
  busy: boolean;
  open: () => void;
  preview: () => void;
}) {
  const executable = item.templates[0]?.executable;
  const draftLabels: Record<string, string> = {
    added: "New draft",
    changed: "Unpublished changes",
    removed: "Removal pending",
  };
  const status = item.draft_change
    ? draftLabels[item.draft_change]
    : executable
      ? "Ready to run"
      : item.query_status === "source_disabled"
        ? "Source disabled"
        : item.query_status === "disabled"
          ? "Query disabled"
          : item.query_status === "publish_required"
            ? "Ready to republish"
            : "Needs validation";
  return (
    <article
      className="business-result catalog-query"
      data-result-key={`${item.source_id}:${item.id}`}
    >
      <div className="catalog-result-heading">
        <div className="catalog-query-title">
          <h2>
            <button
              className="text-button name-link"
              disabled={busy}
              onClick={open}
            >
              {item.name}
            </button>
          </h2>
          <span
            className={`status ${item.draft_change || !executable ? "amber" : "green"}`}
          >
            {t(status)}
          </span>
        </div>
        <Button disabled={busy} onClick={executable ? preview : open}>
          {t(
            executable
              ? item.draft_change
                ? "Preview published query"
                : "Preview query"
              : "Open query",
          )}
        </Button>
      </div>
      <p className="catalog-source">
        <span className="catalog-source-badge">{item.source_name}</span>
        {item.description && (
          <span className="catalog-query-purpose">{item.description}</span>
        )}
      </p>
      {!!item.matched_definitions?.length && (
        <div
          className="catalog-matches"
          aria-label={t("Related business definitions")}
        >
          {item.matched_definitions.map((match) => (
            <span key={match.id}>{match.name}</span>
          ))}
        </div>
      )}
    </article>
  );
}

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
  const parsedOffset = Number(initial.get("offset") || 0);
  const offset =
    Number.isSafeInteger(parsedOffset) &&
    parsedOffset >= 0 &&
    parsedOffset <= 1000000
      ? parsedOffset
      : 0;
  const revision = initial.get("revision") || "";
  const management = ["drafts", "attention"].includes(filters.view);
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
  const restored = useRef(false);
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
          setError(catalogError(e));
          setData(null);
        }
      })
      .finally(() => {
        if (!abort.signal.aborted) setLoading(false);
      });
    return () => abort.abort();
  }, [initialQuery, offset, revision, refresh]);
  useEffect(() => {
    if (loading || !data || restored.current) return;
    restored.current = true;
    const scroll = Number(initial.get("scroll_y") || 0);
    const focus = initial.get("focus");
    const frame = requestAnimationFrame(() => {
      if (focus) {
        const item = Array.from(
          document.querySelectorAll<HTMLElement>("[data-result-key]"),
        ).find((el) => el.dataset.resultKey === focus);
        item
          ?.querySelector<HTMLButtonElement>(".name-link")
          ?.focus({ preventScroll: true });
      }
      window.scrollTo(
        0,
        Number.isFinite(scroll) && scroll >= 0 ? Math.min(scroll, 10000000) : 0,
      );
    });
    return () => cancelAnimationFrame(frame);
  }, [loading, data]);
  const selectedAgent = agents.find((a) => a.id === filters.agent_id);
  const visibleSources = sources.filter(
    (s) =>
      (management || s.enabled) &&
      (!filters.agent_id || selectedAgent?.sources.includes(s.id)),
  );
  function search(nextFilters = filters) {
    const next = `/business?${new URLSearchParams(nextFilters)}`;
    setRefresh((x) => x + 1);
    navigate(next);
  }
  function pageURL(nextOffset: number, scroll = 0, focus = "") {
    const query = new URLSearchParams(initialQuery);
    query.set("offset", String(nextOffset));
    if (data) query.set("revision", data.revision);
    query.set("scroll_y", String(Math.round(scroll)));
    if (focus) query.set("focus", focus);
    else query.delete("focus");
    return `/business?${query}`;
  }
  function openWorkspace(item?: CatalogItem) {
    const back = pageURL(
      offset,
      window.scrollY,
      item ? `${item.source_id}:${item.id}` : "",
    );
    // Keep browser Back and the explicit return link on the same search snapshot.
    history.replaceState(history.state, "", back);
    navigate(
      `${queryToolURL(item?.source_id || filters.source_id, item?.id || "", initial.get("agent_id") || "")}&${new URLSearchParams({ return_to: back })}`,
    );
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
      if (!abort.signal.aborted) setError(catalogError(e));
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
                    ? t("Execution evidence valid")
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
          <h1>{t("Query tools")}</h1>
        </div>
        <Button primary onClick={() => openWorkspace()}>
          {t("Publish a query")}
        </Button>
      </div>
      <div
        className="button-row catalog-view"
        role="group"
        aria-label={t("Catalog view")}
      >
        {[
          "queries",
          ...(!filters.agent_id ? ["drafts", "attention"] : []),
          "all",
        ].map((view) => (
          <Button
            key={view}
            primary={filters.view === view}
            aria-pressed={filters.view === view}
            onClick={() => search({ ...filters, view, kind: "" })}
          >
            {t(
              (
                {
                  queries: "Available queries",
                  drafts: "Drafts",
                  attention: "Needs attention",
                  all: "All definitions",
                } as Record<string, string>
              )[view],
            )}
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
              search({
                ...filters,
                agent_id: e.target.value,
                source_id: "",
                view: e.target.value && management ? "queries" : filters.view,
              })
            }
          >
            <option value="">{t("Administrator")}</option>
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
              search();
            }}
          >
            {t("Refresh catalog")}
          </Button>
        </div>
        <details
          className="catalog-advanced"
          open={!!(initial.get("kind") || initial.get("source_id"))}
        >
          <summary>
            {t(
              filters.view !== "all"
                ? "Filter by data source"
                : "Filter by type and source",
            )}
          </summary>
          <div className="field-grid">
            {filters.view === "all" && (
              <Field label={t("Content type")}>
                <select
                  value={filters.kind}
                  onChange={(e) =>
                    setFilters({ ...filters, kind: e.target.value })
                  }
                >
                  <option value="">{t("All types")}</option>
                  {kinds.map((k) => (
                    <option key={k} value={k}>
                      {t(semanticKindLabel(k))}
                    </option>
                  ))}
                </select>
              </Field>
            )}
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

      <ErrorNote error={error} />
      {error && (
        <Button
          onClick={() => {
            search();
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
              title={t(
                management
                  ? filters.view === "drafts"
                    ? "No query drafts"
                    : "No queries need attention"
                  : "No published matches",
              )}
              description={
                filters.keyword || filters.source_id || filters.kind
                  ? t("Try another keyword or data source.")
                  : undefined
              }
              action={
                filters.view === "attention" ? undefined : (
                  <Button
                    onClick={() =>
                      filters.view === "all"
                        ? navigate("/sources")
                        : openWorkspace()
                    }
                  >
                    {t(
                      filters.view === "all"
                        ? "Manage data sources"
                        : "New query",
                    )}
                  </Button>
                )
              }
            />
          ) : (
            <div className="business-results">
              {data.entries.map((item) =>
                item.kind === "template" ? (
                  <QueryRow
                    key={`${item.source_id}:${item.id}`}
                    item={item}
                    busy={busy}
                    open={() => openWorkspace(item)}
                    preview={() => open(item, item.id)}
                  />
                ) : (
                  <article
                    className="business-result"
                    key={`${item.source_id}:${item.id}`}
                    data-result-key={`${item.source_id}:${item.id}`}
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
                          <span className="catalog-source-badge">
                            {item.source_name}
                          </span>{" "}
                          {item.source_kind} ·{" "}
                          {t("Publication {version}", {
                            version: item.published_version,
                          })}
                        </p>
                      </div>
                      <Button disabled={busy} onClick={() => open(item)}>
                        {t("View definition")}
                      </Button>
                    </div>
                    {item.description && (
                      <p className="catalog-description">{item.description}</p>
                    )}
                    {!!item.matched_definitions?.length && (
                      <div
                        className="catalog-matches"
                        aria-label={t("Related business definitions")}
                      >
                        {item.matched_definitions.map((match) => (
                          <span key={match.id}>{match.name}</span>
                        ))}
                      </div>
                    )}
                    {(item.unit || item.grain || item.time_definition) && (
                      <details className="catalog-context">
                        <summary>{t("Business context")}</summary>
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
                      </details>
                    )}
                    {templateList(item)}
                  </article>
                ),
              )}
            </div>
          )}
          {data.total > 0 && (
            <div className="catalog-pagination">
              <span>
                {t("{count} matching entries", { count: data.total })}
              </span>
              {data.total > 20 && (
                <>
                  <Button
                    disabled={offset === 0}
                    onClick={() => {
                      navigate(pageURL(Math.max(0, offset - 20)));
                    }}
                  >
                    {t("Previous")}
                  </Button>
                  <span>{Math.floor(offset / 20) + 1}</span>
                  <Button
                    disabled={offset + 20 >= data.total}
                    onClick={() => {
                      navigate(pageURL(offset + 20));
                    }}
                  >
                    {t("Next")}
                  </Button>
                </>
              )}
            </div>
          )}
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
          {detail.data.entry.template &&
            !!detail.data.context_entries?.length && (
              <section className="query-concepts">
                <h3>{t("Business concepts")}</h3>
                {detail.data.context_entries.map((concept) => (
                  <details className="concept-picker" key={concept.id}>
                    <summary>
                      {concept.name} · {t(semanticKindLabel(concept.kind))}
                    </summary>
                    {concept.description && <p>{concept.description}</p>}
                    <SemanticDefinition
                      entry={concept}
                      context={detail.data.context_entries}
                    />
                  </details>
                ))}
              </section>
            )}
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
                detail.item.kind === "template"
                  ? openWorkspace(detail.item)
                  : navigate(`/sources/${detail.item.source_id}/setup`)
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
