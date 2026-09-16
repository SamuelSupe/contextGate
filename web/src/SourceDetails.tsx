import { httpOperationQuery } from "./HTTPAPIEditor";
import { t } from "./i18n";
import { ResultTable } from "./ResultTable";
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
  initialAgentID = "",
  onCreateQuery,
}: {
  source: Source;
  catalog: Capability[];
  agents: Agent[];
  onClose: () => void;
  initialAgentID?: string;
  onCreateQuery?: (source: Source, query: string) => void;
}) {
  const cap = source.capability || catalog.find((c) => c.kind === source.kind);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [namespace, setNamespace] = useState("");
  const [object, setObject] = useState("");
  const [agentID, setAgentID] = useState(initialAgentID);
  const [view, setView] = useState("table");
  const [query, setQuery] = useState(() =>
    source.http_api?.operations[0]
      ? httpOperationQuery(source.http_api.operations[0])
      : JSON.stringify(cap?.example || {}, null, 2),
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
  let selectedOperation = "";
  if (source.http_api) {
    try {
      selectedOperation = JSON.parse(query).operation || "";
    } catch {
      /* Keep editing incomplete JSON. */
    }
  }
  function changeQuery(text: string) {
    setQuery(text);
    setResult(null);
  }

  return (
    <Drawer
      title={source.name}
      subtitle={t("Explore source structure and preview read-only queries")}
      onClose={() => {
        controller.current?.abort();
        onClose();
      }}
      wide
    >
      <Protection probe={source.probe} detail />
      <Field
        label={t("Preview authorization as")}
        hint={t(
          "Agent previews enforce the same data source grants. They are marked as previews in the audit log and do not count as a successful client connection.",
        )}
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
          <option value="">{t("Administrator")}</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
              {a.revoked_at
                ? t(" (revoked)")
                : !a.enabled
                  ? t(" (paused)")
                  : ""}
            </option>
          ))}
        </select>
      </Field>
      <div className="field-grid">
        <Field label={t("Namespace")}>
          <input
            disabled={busy}
            value={namespace}
            onChange={(e) => {
              setNamespace(e.target.value);
              setResult(null);
            }}
          />
        </Field>
        <Field label={t("Object name")}>
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
          {t("Namespaces")}
        </Button>
        <Button disabled={busy} onClick={() => discover("objects")}>
          <Table2 size={15} />
          {t("Objects")}
        </Button>
        <Button disabled={busy || !object} onClick={() => discover("describe")}>
          {t("Describe object")}
        </Button>
      </div>
      <section className="form-section">
        <h3>
          {t("Query preview ")}
          <code>{cap?.tool}</code>
        </h3>
        {source.http_api && (
          <Field label={t("API operation")}>
            <select
              disabled={busy}
              value={selectedOperation}
              onChange={(e) => {
                const op = source.http_api?.operations.find(
                  (op) => op.id === e.target.value,
                );
                if (op) changeQuery(httpOperationQuery(op));
              }}
            >
              <option value="" disabled>
                {t("Choose an operation to load its example")}
              </option>
              {source.http_api.operations.map((op) => (
                <option key={op.id} value={op.id}>
                  {op.name} · {op.method}
                </option>
              ))}
            </select>
          </Field>
        )}
        <textarea
          disabled={busy}
          className="query-editor"
          aria-label={t("Query parameters JSON")}
          spellCheck={false}
          rows={9}
          value={query}
          onChange={(e) => changeQuery(e.target.value)}
        />
        <p className="help">
          {t(
            "Enter a JSON object using native query parameters. Integer and decimal literals are sent unchanged.",
          )}
        </p>
        <div className="button-row">
          <Button
            primary
            busy={busy}
            disabled={!source.enabled}
            onClick={() => run()}
          >
            <Play size={14} />
            {t("Run read-only query")}
          </Button>
          {onCreateQuery && (
            <Button
              disabled={busy}
              onClick={() => {
                onClose();
                onCreateQuery(source, query);
              }}
            >
              {t("Create query tool from this query")}
            </Button>
          )}
          {busy ? (
            <Button onClick={() => controller.current?.abort()}>
              {t("Cancel query")}
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
              {result.row_count}
              {t(" rows · ")}
              {result.elapsed_ms}
              {t(" ms")}{" "}
              {result.truncated ? (
                <span className="amber">{t("· Truncated")}</span>
              ) : null}
            </h3>
            <Field label={t("Result view")}>
              <select value={view} onChange={(e) => setView(e.target.value)}>
                <option value="table">{t("Table")}</option>
                <option value="json">JSON</option>
              </select>
            </Field>
          </div>
          {view === "json" ? (
            <pre>{JSON.stringify(result, null, 2)}</pre>
          ) : result.data.length === 0 ? (
            <p className="help">{t("No rows returned.")}</p>
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
              {t("Next page")}
            </Button>
          ) : result.truncated ? (
            <p className="help">
              {t(
                "Refine your query or use explicit query pagination to retrieve more data.",
              )}
            </p>
          ) : null}
          {result.request_id ? (
            <p className="help">
              {t("Request ID: ")}
              <code>{result.request_id}</code>
            </p>
          ) : null}
        </section>
      ) : null}
    </Drawer>
  );
}
