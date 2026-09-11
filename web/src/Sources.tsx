import { SourceEditor } from "./SourceEditor";
import { SourceDetails } from "./SourceDetails";
import { useCallback, useMemo, useState } from "react";
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
}: {
  navigate: (path: string) => void;
  sources: Source[];
  agents: Agent[];
  catalog: Capability[];
  reload: () => Promise<void>;
  notify: (s: string) => void;
}) {
  const [search, setSearch] = useState("");
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
  async function test(s: Source) {
    setBusy(s.id);
    setError("");
    try {
      await api<Probe>(`/api/sources/${s.id}/test`, {
        method: "POST",
        body: "{}",
      });
      await reload();
      notify(`${s.name} check complete`);
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
          <h1>Data sources</h1>
          <p>Manage database connections and read-only access</p>
        </div>
        <Button primary onClick={() => setEditing(null)}>
          <Plus size={17} />
          Add data source
        </Button>
      </div>
      <div className="filters">
        <div className="search-input">
          <Search size={16} />
          <input
            aria-label="Search data sources"
            placeholder="Search data sources"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(0);
            }}
          />
        </div>
        <select
          aria-label="Filter by database type"
          value={kind}
          onChange={(e) => {
            setKind(e.target.value);
            setPage(0);
          }}
        >
          <option value="">All types</option>
          {catalog.map((c) => (
            <option key={c.kind} value={c.kind}>
              {c.name}
            </option>
          ))}
        </select>
        <button
          className="icon-button"
          aria-label="Refresh data sources"
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
              ? "No matching data sources"
              : "Add your first data source"
          }
          description={
            sources.length
              ? "Try another name or database type."
              : "Connect a database, check read-only protection, then grant access to an Agent."
          }
          action={
            !sources.length ? (
              <Button primary onClick={() => setEditing(null)}>
                <Plus size={16} />
                Add data source
              </Button>
            ) : undefined
          }
        />
      ) : (
        <>
          <div className="table-scroll">
            <table className="source-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Type</th>
                  <th>Read-only protection</th>
                  <th>Status</th>
                  <th>Actions</th>
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
                      <td>
                        <button
                          className="text-button name-link"
                          onClick={() => setDetail(s)}
                        >
                          {s.name}
                        </button>
                      </td>
                      <td>
                        {catalog.find((c) => c.kind === s.kind)?.name || s.kind}
                        {s.kind === "influxdb" ? (
                          <small className="inline-version">
                            {" "}
                            {s.version}.x
                          </small>
                        ) : null}
                      </td>
                      <td>
                        <Protection probe={s.probe} />
                      </td>
                      <td>
                        <span
                          className={`status ${s.enabled && s.probe?.connected ? "green" : "muted"}`}
                        >
                          <i className="dot" />
                          {!s.enabled
                            ? "Disabled"
                            : s.probe?.connected
                              ? "Connected at last check"
                              : s.probe
                                ? "Connection failed"
                                : "Not checked"}
                        </span>
                        <small className="block">
                          {s.probe
                            ? `Checked ${date(s.probe.checked_at)}`
                            : "No connection check yet"}
                        </small>
                        {s.probe?.error ? (
                          <small className="block amber">
                            {s.probe.error.message}
                          </small>
                        ) : null}
                      </td>
                      <td>
                        <div className="row-actions">
                          <button
                            className="text-button"
                            onClick={() => setEditing(s)}
                          >
                            Edit
                          </button>
                          <button
                            className="text-button"
                            disabled={busy === s.id}
                            onClick={() => test(s)}
                          >
                            {busy === s.id ? "Checking…" : "Test"}
                          </button>
                          <button
                            className="text-button"
                            onClick={() =>
                              navigate(`/sources/${s.id}/semantics`)
                            }
                          >
                            Semantics
                          </button>
                          <div className="menu-wrap">
                            <button
                              className="icon-button"
                              aria-label={`${s.name} More actions`}
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
                                  Explore data
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
                                    ? "Disable data source"
                                    : "Enable data source"}
                                </button>
                                <button
                                  className="danger"
                                  onClick={() => {
                                    setMenu("");
                                    setDeleting(s);
                                  }}
                                >
                                  <Trash2 size={15} />
                                  Delete configuration
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
            <span>{rows.length} data sources</span>
            <div>
              <select
                aria-label="Rows per page"
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value));
                  setPage(0);
                }}
              >
                {[20, 50, 100].map((n) => (
                  <option key={n} value={n}>
                    {n} per page
                  </option>
                ))}
              </select>
              <button
                className="icon-button"
                aria-label="Previous page"
                disabled={current === 0}
                onClick={() => setPage(current - 1)}
              >
                <ChevronLeft size={17} />
              </button>
              <span className="page-number">{current + 1}</span>
              <button
                className="icon-button"
                aria-label="Next page"
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
          agents={agents}
          source={detail}
          catalog={catalog}
          onClose={closeDetail}
        />
      ) : null}
      {deleting ? (
        <Drawer
          title="Delete data source configuration"
          onClose={() => setDeleting(null)}
          footer={
            <>
              <Button onClick={() => setDeleting(null)}>Cancel</Button>
              <Button
                className="danger"
                onClick={async () => {
                  try {
                    await api(`/api/sources/${deleting.id}`, {
                      method: "DELETE",
                    });
                    setDeleting(null);
                    await reload();
                    notify("Data source configuration deleted");
                  } catch (e) {
                    setError(message(e));
                  }
                }}
              >
                Delete configuration
              </Button>
            </>
          }
        >
          <p>
            Delete the connection configuration for{" "}
            <strong>{deleting.name}</strong> and cancel related Agent queries.
            Its grants will be removed from all Agents. The database and its
            data will be preserved.
          </p>
        </Drawer>
      ) : null}
    </>
  );
}
