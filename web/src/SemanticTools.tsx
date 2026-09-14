import { t } from "./i18n";
import { useEffect, useRef, useState } from "react";
import { api, message } from "./api";
import { Button, Drawer, Empty, ErrorNote, Field, Loading } from "./components";
import type { Agent, QueryResult, Source } from "./types";
import { TemplateParameters } from "./TemplateParameters";
import { validateParameters } from "./template-parameters";
import { ResultTable } from "./ResultTable";
import {
  templatePayload,
  type ObjectReference,
  type SemanticEntry,
} from "./semantic-types";

export function StructureImport({
  source,
  onClose,
  onImport,
}: {
  source: Source;
  onClose: () => void;
  onImport: (objects: ObjectReference[]) => Promise<void>;
}) {
  const [namespace, setNamespace] = useState("");
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [namespacesReady, setNamespacesReady] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [objects, setObjects] = useState<
    { name: string; type: string; namespace?: string }[]
  >([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [cursor, setCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const endpoint = `/api/sources/${source.id}/objects`;
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setNamespacesReady(false);
    api<QueryResult>(`${endpoint}?operation=namespaces`, {
      signal: controller.signal,
    })
      .then((r) => {
        const names = (r.data as { name: string }[]).map((v) => v.name);
        setNamespaces(names);
        setNamespace(
          names.includes("public")
            ? "public"
            : names.find(
                (n) =>
                  ![
                    "information_schema",
                    "pg_catalog",
                    "mysql",
                    "performance_schema",
                    "sys",
                  ].includes(n),
              ) ||
                names[0] ||
                "",
        );
        setNamespacesReady(true);
      })
      .catch((e) => {
        if (!controller.signal.aborted) {
          setError(message(e));
          setLoading(false);
        }
      });
    return () => controller.abort();
  }, [endpoint, refresh]);
  useEffect(() => {
    if (!namespacesReady) return;
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setSelected([]);
    api<QueryResult>(`${endpoint}?namespace=${encodeURIComponent(namespace)}`, {
      signal: controller.signal,
    })
      .then((r) => {
        setObjects(r.data as typeof objects);
        setCursor(r.next_cursor || "");
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [namespace, endpoint, namespacesReady]);
  return (
    <Drawer
      title={t("Import structure")}
      subtitle={t(
        "Import object names and metadata as a draft skeleton. Existing business descriptions are preserved.",
      )}
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={busy} onClick={onClose}>
            {t("Cancel")}
          </Button>
          <Button
            primary
            busy={busy}
            disabled={!selected.length || loading || !!error}
            onClick={async () => {
              setBusy(true);
              setError("");
              try {
                await onImport(
                  selected.map((object) => ({ namespace, object })),
                );
              } catch (e) {
                setError(message(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            {t("Import ")}
            {selected.length}
            {t(" selected")}
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      {error && (
        <Button
          disabled={busy || loading}
          onClick={() => setRefresh((v) => v + 1)}
        >
          {t("Retry structure discovery")}
        </Button>
      )}
      <Field label={t("Namespace")}>
        <select
          disabled={busy || loading}
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
        >
          {namespaces.length ? (
            namespaces.map((n) => <option key={n}>{n}</option>)
          ) : (
            <option value="">{t("Default namespace")}</option>
          )}
        </select>
      </Field>
      <p className="help">
        {t(
          "No business samples are read to infer meanings. Schemaless stores may provide object names only. Select up to 20 objects.",
        )}
      </p>
      {loading ? (
        <Loading />
      ) : error ? null : objects.length ? (
        <div
          className="checkbox-list semantic-object-list"
          role="group"
          aria-label={t("Import structure")}
        >
          {objects.map((o) => (
            <label className="checkbox-row" key={o.name}>
              <input
                type="checkbox"
                checked={selected.includes(o.name)}
                disabled={
                  busy || (!selected.includes(o.name) && selected.length >= 20)
                }
                onChange={(e) =>
                  setSelected((v) =>
                    e.target.checked
                      ? [...v, o.name]
                      : v.filter((n) => n !== o.name),
                  )
                }
              />
              <span>
                {o.name}
                <small className="block">{o.type}</small>
              </span>
            </label>
          ))}
        </div>
      ) : (
        <Empty
          title={t("No objects available")}
          description={t(
            "Check the namespace and database account's metadata permissions.",
          )}
        />
      )}
      {cursor && (
        <Button
          disabled={loading || busy}
          onClick={async () => {
            setLoading(true);
            setError("");
            try {
              const r = await api<QueryResult>(
                `${endpoint}?namespace=${encodeURIComponent(namespace)}&cursor=${encodeURIComponent(cursor)}`,
              );
              setObjects((v) => [...v, ...(r.data as typeof objects)]);
              setCursor(r.next_cursor || "");
            } catch (e) {
              setError(message(e));
            } finally {
              setLoading(false);
            }
          }}
        >
          {t("Load more objects")}
        </Button>
      )}
    </Drawer>
  );
}

export function TemplatePreview({
  source,
  entry,
  agents,
  onClose,
  initialAgentID = "",
}: {
  source: Source;
  entry: SemanticEntry;
  agents: Agent[];
  onClose: () => void;
  initialAgentID?: string;
}) {
  const [params, setParams] = useState(entry.template!.example_json);
  const [agent, setAgent] = useState(initialAgentID);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [resultView, setResultView] = useState(
    ["query_sql", "query_cql"].includes(entry.template!.tool)
      ? "table"
      : "json",
  );
  const abort = useRef<AbortController | null>(null);
  const parameters = entry.template!.parameters || [];
  const parameterIssue = validateParameters(parameters, params);
  const identity = agents.find((a) => a.id === agent);
  const identityIssue =
    agent && (!identity || !identity.sources.includes(source.id))
      ? t(
          "This Agent has no access to this data source. Choose an authorized Agent or update its grants in Agents.",
        )
      : identity &&
          (!identity.enabled ||
            identity.revoked_at ||
            new Date(identity.expires_at) <= new Date())
        ? t(
            "This Agent is paused, expired or revoked. Choose an active Agent to preview its access.",
          )
        : "";
  useEffect(() => () => abort.current?.abort(), []);
  async function run(cursor = "") {
    if (parameterIssue || identityIssue) {
      setError(parameterIssue || identityIssue);
      return;
    }
    const controller = new AbortController();
    abort.current = controller;
    setBusy(true);
    setError("");
    setResult(null);
    try {
      setResult(
        await api<QueryResult>(`/api/sources/${source.id}/semantics/execute`, {
          method: "POST",
          body: templatePayload(source.id, entry, params, agent, cursor),
          signal: controller.signal,
        }),
      );
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Drawer
      wide
      title={t("Preview published template")}
      subtitle={t("{name} · Execution version {execution_version}", {
        name: entry.name,
        execution_version: entry.template!.execution_version,
      })}
      onClose={() => {
        abort.current?.abort();
        onClose();
      }}
      footer={
        <>
          {busy ? (
            <Button onClick={() => abort.current?.abort()}>
              {t("Cancel query")}
            </Button>
          ) : (
            <Button
              primary
              disabled={!!parameterIssue || !!identityIssue}
              onClick={() => run()}
            >
              {t("Run template")}
            </Button>
          )}
          {result?.next_cursor && (
            <Button disabled={busy} onClick={() => run(result.next_cursor)}>
              {t("Next result page")}
            </Button>
          )}
        </>
      }
    >
      <ErrorNote error={error} />
      <Field
        label={t("Preview identity")}
        hint={t(
          "Agent previews enforce current grants and the data source's query access mode.",
        )}
      >
        <select
          disabled={busy}
          value={agent}
          onChange={(e) => {
            setAgent(e.target.value);
            setResult(null);
            setError("");
          }}
        >
          <option value="">{t("Administrator")}</option>
          {agents.map((a) => (
            <option value={a.id} key={a.id}>
              {a.name}
            </option>
          ))}
        </select>
      </Field>
      {identityIssue && (
        <div className="notice warning" role="status">
          {identityIssue}
        </div>
      )}
      <TemplateParameters
        parameters={parameters}
        example={entry.template!.example_json}
        value={params}
        disabled={busy}
        onChange={(value) => {
          setParams(value);
          setResult(null);
          setError("");
        }}
      />
      {parameterIssue && (
        <p className="field-error" role="status">
          {parameterIssue}
        </p>
      )}
      {result && (
        <>
          <p className="help">
            {result.row_count}
            {t(" rows · ")}
            {result.elapsed_ms}
            {t(" ms ·")}{" "}
            {result.truncated ? t("Truncated") : t("Complete page")}
            {t(" · Request")} {result.request_id}
          </p>
          {result.ontology_context && (
            <details className="ontology-result-context">
              <summary>{t("Ontology and execution details")}</summary>
              <strong>
                {t("Ontology ")}
                {result.ontology_context.ontology_id}
                {t(" · version")} {result.ontology_context.version}
              </strong>
              <p className="help">
                {t("Source publication ")}
                {result.semantic_version}
                {t(" · template execution ")}
                {result.template_version}
              </p>
              <ul>
                {result.ontology_context.concept_refs.map((ref) => (
                  <li key={ref}>
                    <code>{ref}</code>
                  </li>
                ))}
              </ul>
            </details>
          )}
          <div className="button-row">
            <Button
              aria-pressed={resultView === "table"}
              onClick={() => setResultView("table")}
            >
              {t("Table")}
            </Button>
            <Button
              aria-pressed={resultView === "json"}
              onClick={() => setResultView("json")}
            >
              JSON
            </Button>
          </div>
          {!result.row_count ? (
            <Empty
              title={t("No matching results")}
              description={t(
                "The query succeeded. Try different parameter values.",
              )}
            />
          ) : resultView === "table" ? (
            <ResultTable result={result} />
          ) : (
            <pre className="semantic-result">
              {JSON.stringify(result.data, null, 2)}
            </pre>
          )}
        </>
      )}
    </Drawer>
  );
}
