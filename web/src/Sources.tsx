import { t } from "./i18n";
import { SourceEditor } from "./SourceEditor";
import { type Readiness, readinessLabel } from "./readiness";
import "./product-workflows.css";
import { SourceDetails } from "./SourceDetails";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Plus,
  Search,
  ChevronLeft,
  ChevronRight,
  Ellipsis,
  RefreshCw,
  Trash2,
  Power,
  Table2,
} from "lucide-react";
import { api, date, message, payload } from "./api";
import { Button, Drawer, Empty, ErrorNote, Protection } from "./components";
import type { Agent, Capability, Probe, Source } from "./types";

export function Sources({
  navigate,
  sources,
  agents,
  catalog,
  reload,
  notify,
  onCreateQuery,
}: {
  navigate: (path: string) => void;
  sources: Source[];
  onCreateQuery: (source: Source, query: string) => void;
  agents: Agent[];
  catalog: Capability[];
  reload: () => Promise<void>;
  notify: (s: string) => void;
}) {
  const [search, setSearch] = useState("");
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [kind, setKind] = useState("");
  const [editing, setEditing] = useState<Source | null | undefined>();
  const [detail, setDetail] = useState<Source | null>(null);
  const [menu, setMenu] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(20);
  const [deleting, setDeleting] = useState<Source | null>(null);
  const closeEditor = useCallback(() => setEditing(undefined), []);
  const closeDetail = useCallback(() => setDetail(null), []);
  const rows = useMemo(
    () =>
      sources
        .filter(
          (s) =>
            (!kind || s.kind === kind) &&
            `${s.name} ${s.kind}`.toLowerCase().includes(search.toLowerCase()),
        )
        .sort((a, b) => a.name.localeCompare(b.name)),
    [sources, kind, search],
  );
  const current = Math.min(
    page,
    Math.max(0, Math.ceil(rows.length / pageSize) - 1),
  );
  const [readiness, setReadiness] = useState<Record<string, Readiness>>({});
  const visibleSources = useMemo(
    () => rows.slice(current * pageSize, (current + 1) * pageSize),
    [rows, current, pageSize],
  );
  useEffect(() => {
    const abort = new AbortController();
    setReadiness({});
    void (async () => {
      for (
        let i = 0;
        i < visibleSources.length && !abort.signal.aborted;
        i += 4
      ) {
        const batch = await Promise.all(
          visibleSources.slice(i, i + 4).map(async (source) => {
            try {
              return await api<Readiness>(
                `/api/sources/${source.id}/readiness?summary=1`,
                { signal: abort.signal },
              );
            } catch {
              return null;
            }
          }),
        );
        if (!abort.signal.aborted)
          setReadiness((prior) => ({
            ...prior,
            ...Object.fromEntries(
              batch.filter((v) => v !== null).map((v) => [v.source_id, v]),
            ),
          }));
      }
    })();
    return () => abort.abort();
  }, [visibleSources, agents]);
  async function test(s: Source) {
    setBusy(s.id);
    setError("");
    try {
      await api<Probe>(`/api/sources/${s.id}/test`, {
        method: "POST",
        body: "{}",
      });
      await reload();
      notify(t("{name} check complete", { name: s.name }));
    } catch (e) {
      setError(message(e));
      await reload().catch((e) => setError(message(e)));
    } finally {
      setBusy("");
    }
  }
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Data sources")}</h1>
        </div>
        <Button primary onClick={() => setEditing(null)}>
          <Plus size={17} />
          {t("Add data source")}
        </Button>
      </div>
      <div className="filters">
        <div className="search-input">
          <Search size={16} />
          <input
            aria-label={t("Search data sources")}
            placeholder={t("Search data sources")}
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(0);
            }}
          />
        </div>
        <select
          aria-label={t("Filter by source type")}
          value={kind}
          onChange={(e) => {
            setKind(e.target.value);
            setPage(0);
          }}
        >
          <option value="">{t("All types")}</option>
          {catalog.map((c) => (
            <option key={c.kind} value={c.kind}>
              {c.name}
            </option>
          ))}
        </select>
        <button
          className="icon-button"
          aria-label={t("Refresh data sources")}
          onClick={() => reload().catch((e) => setError(message(e)))}
        >
          <RefreshCw size={16} />
        </button>
      </div>
      <ErrorNote error={error} />
      {rows.length === 0 ? (
        <Empty
          title={
            sources.length
              ? t("No matching data sources")
              : t("Add your first data source")
          }
          description={
            sources.length
              ? t("Try another name or source type.")
              : t(
                  "Connect a database or API, review its read-only protection, then grant access to an Agent.",
                )
          }
          action={
            !sources.length ? (
              <Button primary onClick={() => setEditing(null)}>
                <Plus size={16} />
                {t("Add data source")}
              </Button>
            ) : (
              <Button
                onClick={() => {
                  setSearch("");
                  setKind("");
                  setPage(0);
                }}
              >
                {t("Clear filters")}
              </Button>
            )
          }
        />
      ) : (
        <>
          <div className="table-scroll">
            <table className="source-table responsive-table">
              <thead>
                <tr>
                  <th>{t("Name")}</th>
                  <th>{t("Type")}</th>
                  <th>{t("Read-only protection")}</th>
                  <th>{t("Status")}</th>
                  <th>{t("Actions")}</th>
                </tr>
              </thead>
              <tbody>
                {rows
                  .slice(current * pageSize, (current + 1) * pageSize)
                  .map((s) => (
                    <tr
                      key={s.id}
                      className={editing?.id === s.id ? "selected" : ""}
                    >
                      <td data-label={t("Name")}>
                        <button
                          className="text-button name-link"
                          onClick={() => navigate(`/sources/${s.id}/setup`)}
                        >
                          {s.name}
                        </button>
                        <span className="workflow-readiness">
                          {readiness[s.id]
                            ? readinessLabel(s, readiness[s.id])
                            : t("Open to review Agent setup")}
                        </span>
                        {readiness[s.id] && (
                          <small className="block">
                            {readiness[s.id].executable_templates}
                            {t(" templates ·")}{" "}
                            {readiness[s.id].active_agents.length}
                            {t(" active Agents")}
                          </small>
                        )}
                      </td>
                      <td data-label={t("Type")}>
                        {catalog.find((c) => c.kind === s.kind)?.name || s.kind}
                        {s.kind === "influxdb" ? (
                          <small className="inline-version">
                            {" "}
                            {s.version}
                            {t(".x")}
                          </small>
                        ) : null}
                      </td>
                      <td data-label={t("Read-only protection")}>
                        <Protection probe={s.probe} />
                      </td>
                      <td data-label={t("Status")}>
                        <span
                          className={`status ${s.enabled && s.probe?.connected ? "green" : "muted"}`}
                        >
                          <i className="dot" />
                          {!s.enabled
                            ? t("Disabled")
                            : s.probe?.connected
                              ? t("Connected at last check")
                              : s.probe
                                ? t("Connection failed")
                                : t("Connection not checked")}
                        </span>
                        <small className="block">
                          {s.probe
                            ? t("Checked {value1}", {
                                value1: date(s.probe.checked_at),
                              })
                            : t("No connection check yet")}
                        </small>
                        {s.probe?.error ? (
                          <small className="block amber">
                            {s.probe.error.message}
                          </small>
                        ) : null}
                      </td>
                      <td data-label={t("Actions")}>
                        <div className="row-actions">
                          <button
                            className="text-button"
                            onClick={() => setEditing(s)}
                          >
                            {t("Edit")}
                          </button>
                          <button
                            className="text-button"
                            disabled={busy === s.id}
                            onClick={() => test(s)}
                          >
                            {busy === s.id ? t("Checking…") : t("Test")}
                          </button>
                          <button
                            className="text-button"
                            onClick={() =>
                              navigate(`/sources/${s.id}/semantics`)
                            }
                          >
                            {t("Semantics")}
                          </button>
                          <div className="menu-wrap">
                            <button
                              className="icon-button"
                              aria-label={t("{name} More actions", {
                                name: s.name,
                              })}
                              aria-expanded={menu === s.id}
                              onClick={() => setMenu(menu === s.id ? "" : s.id)}
                            >
                              <Ellipsis size={19} />
                            </button>
                            {menu === s.id ? (
                              <div className="menu">
                                <button
                                  onClick={() => {
                                    setMenu("");
                                    setDetail(s);
                                  }}
                                >
                                  <Table2 size={15} />
                                  {t("Structure & native preview")}
                                </button>
                                <button
                                  onClick={async () => {
                                    setMenu("");
                                    try {
                                      await api(`/api/sources/${s.id}`, {
                                        method: "PUT",
                                        body: payload({
                                          ...s,
                                          enabled: !s.enabled,
                                        }),
                                      });
                                      await reload();
                                    } catch (e) {
                                      setError(message(e));
                                    }
                                  }}
                                >
                                  <Power size={15} />
                                  {s.enabled
                                    ? t("Disable data source")
                                    : t("Enable data source")}
                                </button>
                                <button
                                  className="danger"
                                  onClick={() => {
                                    setMenu("");
                                    setDeleting(s);
                                  }}
                                >
                                  <Trash2 size={15} />
                                  {t("Delete configuration")}
                                </button>
                              </div>
                            ) : null}
                          </div>
                        </div>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
          <div className="pagination">
            <span>
              {rows.length}
              {t(" data sources")}
            </span>
            <div>
              <select
                aria-label={t("Rows per page")}
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value));
                  setPage(0);
                }}
              >
                {[20, 50, 100].map((n) => (
                  <option key={n} value={n}>
                    {n}
                    {t(" per page")}
                  </option>
                ))}
              </select>
              <button
                className="icon-button"
                aria-label={t("Previous page")}
                disabled={current === 0}
                onClick={() => setPage(current - 1)}
              >
                <ChevronLeft size={17} />
              </button>
              <span className="page-number">{current + 1}</span>
              <button
                className="icon-button"
                aria-label={t("Next page")}
                disabled={(current + 1) * pageSize >= rows.length}
                onClick={() => setPage(current + 1)}
              >
                <ChevronRight size={17} />
              </button>
            </div>
          </div>
        </>
      )}
      {editing !== undefined ? (
        <SourceEditor
          source={editing}
          catalog={catalog}
          onClose={closeEditor}
          onSaved={async (notice, close = true) => {
            await reload();
            if (close) closeEditor();
            if (notice) notify(notice);
          }}
        />
      ) : null}
      {detail ? (
        <SourceDetails
          onCreateQuery={onCreateQuery}
          agents={agents}
          source={detail}
          catalog={catalog}
          onClose={closeDetail}
        />
      ) : null}
      {deleting ? (
        <Drawer
          title={t("Delete data source configuration")}
          onClose={() => {
            if (!deleteBusy) {
              setDeleting(null);
              setDeleteError("");
            }
          }}
          footer={
            <>
              <Button
                disabled={deleteBusy}
                onClick={() => {
                  setDeleting(null);
                  setDeleteError("");
                }}
              >
                {t("Cancel")}
              </Button>
              <Button
                className="danger"
                busy={deleteBusy}
                onClick={async () => {
                  setDeleteBusy(true);
                  setDeleteError("");
                  try {
                    await api(`/api/sources/${deleting.id}`, {
                      method: "DELETE",
                    });
                    setDeleting(null);
                    await reload();
                    notify(t("Data source configuration deleted"));
                  } catch (e) {
                    setDeleteError(message(e));
                  } finally {
                    setDeleteBusy(false);
                  }
                }}
              >
                {t("Delete configuration")}
              </Button>
            </>
          }
        >
          <ErrorNote error={deleteError} />
          <p>
            {t("Delete the connection configuration for")}{" "}
            <strong>{deleting.name}</strong>
            {t(
              " and cancel related Agent queries. Its grants will be removed from all Agents. The database and its data will be preserved.",
            )}
          </p>
          <p className="help">
            {t(
              "This also permanently removes this source's semantic catalog, templates, ontology mapping, saved questions and evaluation history. Export anything you need to keep before deleting.",
            )}
          </p>
        </Drawer>
      ) : null}
    </>
  );
}
