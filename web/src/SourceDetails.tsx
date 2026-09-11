import { useEffect, useRef, useState } from "react";
import { Database, Table2, Play } from "lucide-react";
import { api, message } from "./api";
import { queryPayload } from "./query";
import {
  Button,
  Drawer,
  ErrorNote,
  Field,
  Loading,
  Protection,
} from "./components";
import type { Agent, Source, Capability, QueryResult } from "./types";

export function SourceDetails({
  source,
  catalog,
  agents,
  onClose,
}: {
  source: Source;
  catalog: Capability[];
  agents: Agent[];
  onClose: () => void;
}) {
  const cap = source.capability || catalog.find((c) => c.kind === source.kind);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [namespace, setNamespace] = useState("");
  const [object, setObject] = useState("");
  const [agentID, setAgentID] = useState("");
  const [view, setView] = useState("table");
  const [query, setQuery] = useState(() =>
    JSON.stringify(cap?.example || {}, null, 2),
  );
  const [discovery, setDiscovery] = useState("");
  const controller = useRef<AbortController | null>(null);
  const lastQuery = useRef("");
  useEffect(() => () => controller.current?.abort(), []);

  async function request(
    action: (signal: AbortSignal) => Promise<QueryResult>,
  ) {
    controller.current?.abort();
    const current = new AbortController();
    controller.current = current;
    setBusy(true);
    setError("");
    setResult(null);
    try {
      const result = await action(current.signal);
      if (!current.signal.aborted) setResult(result);
    } catch (e) {
      if (controller.current === current) setError(message(e));
    } finally {
      if (controller.current === current) setBusy(false);
    }
  }
  function discover(
    operation: string,
    ns = namespace,
    obj = object,
    cursor = "",
  ) {
    setDiscovery(operation);
    void request((signal) =>
      api<QueryResult>(
        `/api/sources/${source.id}/objects?${new URLSearchParams({ operation, namespace: ns, object: obj, agent_id: agentID, cursor })}`,
        { signal },
      ),
    );
  }
  function run(next = false) {
    const text = next ? lastQuery.current : query;
    const cursor = next ? result?.next_cursor || "" : "";
    setDiscovery("");
    void request((signal) => {
      const body = queryPayload(
        text,
        source.id,
        cap?.tool || "",
        agentID,
        cursor,
      );
      lastQuery.current = text;
      return api<QueryResult>("/api/query", { method: "POST", body, signal });
    });
  }
  function changeQuery(text: string) {
    setQuery(text);
    setResult(null);
  }

  return (
    <Drawer
      title={source.name}
      subtitle="Explore database structure and preview read-only queries"
      onClose={() => {
        controller.current?.abort();
        onClose();
      }}
      wide
    >
      <Protection probe={source.probe} detail />
      <Field
        label="Preview authorization as"
        hint="Agent previews enforce the same data source grants. They are marked as previews in the audit log and do not count as a successful client connection."
      >
        <select
          disabled={busy}
          value={agentID}
          onChange={(e) => {
            setAgentID(e.target.value);
            setResult(null);
            setError("");
          }}
        >
          <option value="">Administrator</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
              {a.revoked_at ? " (revoked)" : !a.enabled ? " (paused)" : ""}
            </option>
          ))}
        </select>
      </Field>
      <div className="field-grid">
        <Field label="Namespace">
          <input
            disabled={busy}
            value={namespace}
            onChange={(e) => {
              setNamespace(e.target.value);
              setResult(null);
            }}
          />
        </Field>
        <Field label="Object name">
          <input
            disabled={busy}
            value={object}
            onChange={(e) => {
              setObject(e.target.value);
              setResult(null);
            }}
          />
        </Field>
      </div>
      <div className="button-row">
        <Button disabled={busy} onClick={() => discover("namespaces")}>
          <Database size={15} />
          Namespaces
        </Button>
        <Button disabled={busy} onClick={() => discover("objects")}>
          <Table2 size={15} />
          Objects
        </Button>
        <Button disabled={busy || !object} onClick={() => discover("describe")}>
          Describe object
        </Button>
      </div>
      <section className="form-section">
        <h3>
          Query preview <code>{cap?.tool}</code>
        </h3>
        <textarea
          disabled={busy}
          className="query-editor"
          aria-label="Query parameters JSON"
          spellCheck={false}
          rows={9}
          value={query}
          onChange={(e) => changeQuery(e.target.value)}
        />
        <p className="help">
          Enter a JSON object using native query parameters. Integer and decimal
          literals are sent unchanged.
        </p>
        <div className="button-row">
          <Button
            primary
            busy={busy}
            disabled={!source.enabled}
            onClick={() => run()}
          >
            <Play size={14} />
            Run read-only query
          </Button>
          {busy ? (
            <Button onClick={() => controller.current?.abort()}>
              Cancel query
            </Button>
          ) : null}
        </div>
      </section>
      <ErrorNote error={error} />
      {busy ? (
        <Loading />
      ) : result ? (
        <section className="results">
          <div className="result-heading">
            <h3>
              {result.row_count} rows · {result.elapsed_ms} ms{" "}
              {result.truncated ? (
                <span className="amber">· Truncated</span>
              ) : null}
            </h3>
            <Field label="Result view">
              <select value={view} onChange={(e) => setView(e.target.value)}>
                <option value="table">Table</option>
                <option value="json">JSON</option>
              </select>
            </Field>
          </div>
          {view === "json" ? (
            <pre>{JSON.stringify(result, null, 2)}</pre>
          ) : result.data.length === 0 ? (
            <p className="help">No rows returned.</p>
          ) : result.format === "metadata" && discovery !== "describe" ? (
            <div className="object-list">
              {result.data.map((item, i) => {
                const o = item as {
                  name: string;
                  namespace?: string;
                  type: string;
                  columns?: unknown;
                  details?: unknown;
                };
                return (
                  <div className="object-row" key={i}>
                    <Button
                      disabled={busy}
                      onClick={() => {
                        if (discovery === "namespaces") {
                          setNamespace(o.name);
                          setObject("");
                          discover("objects", o.name, "");
                        } else {
                          setObject(o.name);
                          const ns = o.namespace || namespace;
                          setNamespace(ns);
                          discover("describe", ns, o.name);
                        }
                      }}
                    >
                      {o.name}
                    </Button>
                    <small>
                      {o.type}
                      {o.namespace ? ` · ${o.namespace}` : ""}
                    </small>
                  </div>
                );
              })}
            </div>
          ) : (
            <ResultTable result={result} />
          )}
          {result.next_cursor ? (
            <Button
              onClick={() =>
                discovery
                  ? discover(discovery, namespace, object, result.next_cursor)
                  : run(true)
              }
            >
              Next page
            </Button>
          ) : result.truncated ? (
            <p className="help">
              Refine your query or use explicit query pagination to retrieve
              more data.
            </p>
          ) : null}
          {result.request_id ? (
            <p className="help">
              Request ID: <code>{result.request_id}</code>
            </p>
          ) : null}
        </section>
      ) : null}
    </Drawer>
  );
}
function cell(value: unknown) {
  return value === null
    ? "null"
    : typeof value === "object"
      ? JSON.stringify(value)
      : String(value ?? "");
}
function ResultTable({ result }: { result: QueryResult }) {
  const first = result.data[0];
  const keys =
    !Array.isArray(first) && typeof first === "object" && first
      ? [
          ...new Set(
            result.data.flatMap((row) =>
              row && typeof row === "object" && !Array.isArray(row)
                ? Object.keys(row)
                : [],
            ),
          ),
        ]
      : [];
  const columns = result.columns?.length
    ? result.columns
    : keys.length
      ? keys.map((name) => ({ name, type: "" }))
      : Array.from(
          { length: Array.isArray(first) ? first.length : 1 },
          (_, i) => ({ name: `Value ${i + 1}`, type: "" }),
        );
  return (
    <div className="table-scroll result-table">
      <table>
        <thead>
          <tr>
            {columns.map((c, i) => (
              <th key={i}>
                {c.name}
                <small className="block">{c.type}</small>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {result.data.map((row, i) => (
            <tr key={i}>
              {columns.map((c, j) => (
                <td key={j}>
                  {cell(
                    Array.isArray(row)
                      ? row[j]
                      : row && typeof row === "object"
                        ? (row as Record<string, unknown>)[c.name]
                        : row,
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
