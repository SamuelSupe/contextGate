import { useEffect, useState } from "react";
import { api, date, message } from "./api";
import {
  Button,
  CopyButton,
  Drawer,
  Empty,
  ErrorNote,
  Field,
  Loading,
} from "./components";
import type { Agent, Audit, Source } from "./types";

const hints: Record<string, string> = {
  database_authentication:
    "Check the database authentication method and replace the stored credential if needed.",
  database_permission:
    "Check the configured account's read permissions in the database.",
  database_tls:
    "Check the CA certificate, server hostname and certificate expiry.",
  database_dns: "Check hostname resolution from the Hub server.",
  database_connection:
    "Check the database host, port, name and server availability.",
  database_query:
    "Check the query dialect, parameters, namespace and object name.",
  timeout:
    "Check connectivity, reduce the query scope or review the configured timeout.",
  cancelled: "The caller cancelled the query or its authorization changed.",
  not_found: "The source is disabled, missing or outside this Agent's grants.",
  query_denied:
    "The operation is outside the supported read-only query boundary.",
};
export function AuditPage({
  sources,
  agents,
}: {
  sources: Source[];
  agents: Agent[];
}) {
  const [rows, setRows] = useState<Audit[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [filters, setFilters] = useState({
    agent_id: "",
    source_id: "",
    status: "",
    request_id: "",
    from: "",
    until: "",
  });
  const [query, setQuery] = useState("");
  const [history, setHistory] = useState<string[]>([""]);
  const [refresh, setRefresh] = useState(0);
  const [detail, setDetail] = useState<Audit | null>(null);
  const before = history[history.length - 1];
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    const params = new URLSearchParams(query);
    if (before) params.set("before", before);
    api<Audit[]>(`/api/audit?${params}`, { signal: controller.signal })
      .then(setRows)
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [query, before, refresh]);
  const agentName = (id: string) =>
    id === "admin"
      ? "Administrator"
      : agents.find((a) => a.id === id)?.name || id;
  const sourceName = (id: string) =>
    sources.find((s) => s.id === id)?.name || id || "Unsaved connection";
  return (
    <>
      <div className="page-header">
        <div>
          <h1>Audit log</h1>
          <p>
            30 days of calls. Query text, parameters and results are never
            stored.
          </p>
        </div>
        <Button busy={loading} onClick={() => setRefresh((n) => n + 1)}>
          Refresh
        </Button>
      </div>
      <form
        className="audit-filters"
        onSubmit={(e) => {
          e.preventDefault();
          const params = new URLSearchParams();
          for (const [k, v] of Object.entries(filters)) {
            if (v)
              params.set(
                k,
                k === "from" || k === "until"
                  ? new Date(v).toISOString()
                  : v.trim(),
              );
          }
          setQuery(params.toString());
          setHistory([""]);
          setRefresh((n) => n + 1);
        }}
      >
        <Field label="Agent">
          <select
            value={filters.agent_id}
            onChange={(e) =>
              setFilters({ ...filters, agent_id: e.target.value })
            }
          >
            <option value="">All callers</option>
            <option value="admin">Administrator</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Data source">
          <select
            value={filters.source_id}
            onChange={(e) =>
              setFilters({ ...filters, source_id: e.target.value })
            }
          >
            <option value="">All sources</option>
            {sources.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Result">
          <select
            value={filters.status}
            onChange={(e) => setFilters({ ...filters, status: e.target.value })}
          >
            <option value="">All results</option>
            <option value="success">Success</option>
            <option value="error">Error</option>
          </select>
        </Field>
        <Field label="From">
          <input
            type="datetime-local"
            value={filters.from}
            onChange={(e) => setFilters({ ...filters, from: e.target.value })}
          />
        </Field>
        <Field label="Until">
          <input
            type="datetime-local"
            value={filters.until}
            min={filters.from || undefined}
            onChange={(e) => setFilters({ ...filters, until: e.target.value })}
          />
        </Field>
        <Field label="Request ID">
          <input
            value={filters.request_id}
            onChange={(e) =>
              setFilters({ ...filters, request_id: e.target.value })
            }
          />
        </Field>
        <div className="button-row">
          <Button primary type="submit">
            Apply filters
          </Button>
          <Button
            type="button"
            onClick={() => {
              setFilters({
                agent_id: "",
                source_id: "",
                status: "",
                request_id: "",
                from: "",
                until: "",
              });
              setQuery("");
              setHistory([""]);
            }}
          >
            Clear
          </Button>
        </div>
      </form>
      <ErrorNote error={error} />
      {loading ? (
        <Loading />
      ) : !rows.length ? (
        <Empty
          title="No matching calls"
          description="Adjust the filters or run a query from your Agent."
        />
      ) : (
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Caller</th>
                <th>Data source</th>
                <th>Operation</th>
                <th>Duration</th>
                <th>Rows</th>
                <th>Result</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((a) => (
                <tr key={a.id}>
                  <td className="nowrap">{date(a.at)}</td>
                  <td>
                    {agentName(a.agent_id)}
                    {a.preview ? (
                      <small className="block">Administrator preview</small>
                    ) : null}
                  </td>
                  <td>{sourceName(a.source_id)}</td>
                  <td>
                    {a.operation}
                    {a.template_id && (
                      <small className="block">
                        {a.template_id} · v{a.template_version || "draft"}
                      </small>
                    )}
                  </td>
                  <td>{a.elapsed_ms} ms</td>
                  <td>{a.rows}</td>
                  <td>
                    <button
                      className={`text-button ${a.error_code ? "amber" : ""}`}
                      onClick={() => setDetail(a)}
                    >
                      {a.error_code || "Success"} · Details
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="button-row pagination">
        <Button
          disabled={loading || history.length < 2}
          onClick={() => setHistory((h) => h.slice(0, -1))}
        >
          Newer records
        </Button>
        <span>Page {history.length}</span>
        <Button
          disabled={loading || rows.length < 100}
          onClick={() =>
            setHistory((h) => [...h, String(rows[rows.length - 1].id)])
          }
        >
          Older records
        </Button>
      </div>
      {detail ? (
        <Drawer
          title="Call details"
          subtitle={
            detail.error_code
              ? "Review the diagnostic and suggested action"
              : "Call completed successfully"
          }
          onClose={() => setDetail(null)}
        >
          <dl className="settings-list">
            <div>
              <dt>Request ID</dt>
              <dd>
                <code>{detail.request_id || "Legacy record"}</code>
                {detail.request_id ? (
                  <CopyButton text={detail.request_id} />
                ) : null}
              </dd>
            </div>
            <div>
              <dt>Caller</dt>
              <dd>
                {agentName(detail.agent_id)}
                {detail.preview ? " · Administrator preview" : ""}
              </dd>
            </div>
            <div>
              <dt>Data source</dt>
              <dd>{sourceName(detail.source_id)}</dd>
            </div>
            <div>
              <dt>Time</dt>
              <dd>{date(detail.at)}</dd>
            </div>
            <div>
              <dt>Operation</dt>
              <dd>{detail.operation}</dd>
              {detail.template_id && (
                <>
                  <dt>Query template</dt>
                  <dd>
                    {detail.template_id} · execution version{" "}
                    {detail.template_version || "draft trial"}
                  </dd>
                </>
              )}
            </div>
            <div>
              <dt>Result</dt>
              <dd>{detail.error_code || "Success"}</dd>
            </div>
            <div>
              <dt>Database code</dt>
              <dd>{detail.native_code || "Not reported"}</dd>
            </div>
            <div>
              <dt>Duration / rows</dt>
              <dd>
                {detail.elapsed_ms} ms / {detail.rows}
              </dd>
            </div>
            <div>
              <dt>Query fingerprint</dt>
              <dd className="mono break-all">
                {detail.fingerprint || "Not applicable"}
              </dd>
            </div>
          </dl>
          {detail.error_code ? (
            <div className="notice warning">
              {hints[detail.error_code] ||
                "Check the database server logs at this time. Use the request ID when reporting the issue."}
            </div>
          ) : null}
          <p className="help">
            Diagnostics contain stable error codes only. Credentials, database
            error text, query text and parameters are not retained.
          </p>
        </Drawer>
      ) : null}
    </>
  );
}
