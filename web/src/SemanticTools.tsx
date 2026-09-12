import { useEffect, useRef, useState } from "react";
import { api, message } from "./api";
import { Button, Drawer, Empty, ErrorNote, Field, Loading } from "./components";
import type { Agent, QueryResult, Source } from "./types";
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
    api<QueryResult>(`${endpoint}?operation=namespaces`, {
      signal: controller.signal,
    })
      .then((r) => {
        const names = (r.data as { name: string }[]).map((v) => v.name);
        setNamespaces(names);
        setNamespace(names[0] || "");
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      });
    return () => controller.abort();
  }, [endpoint]);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
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
  }, [namespace, endpoint]);
  return (
    <Drawer
      title="Import structure"
      subtitle="Import object names and metadata as a draft skeleton. Existing business descriptions are preserved."
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={busy} onClick={onClose}>
            Cancel
          </Button>
          <Button
            primary
            busy={busy}
            disabled={!selected.length || loading}
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
            Import {selected.length} selected
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      <Field label="Namespace">
        <select
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
        >
          {namespaces.length ? (
            namespaces.map((n) => <option key={n}>{n}</option>)
          ) : (
            <option value="">Default namespace</option>
          )}
        </select>
      </Field>
      <p className="help">
        No business samples are read to infer meanings. Schemaless stores may
        provide object names only. Select up to 20 objects.
      </p>
      {loading ? (
        <Loading />
      ) : objects.length ? (
        <div className="semantic-object-list">
          {objects.map((o) => (
            <label className="checkbox-row" key={o.name}>
              <input
                type="checkbox"
                checked={selected.includes(o.name)}
                disabled={!selected.includes(o.name) && selected.length >= 20}
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
          title="No objects available"
          description="Check the namespace and database account's metadata permissions."
        />
      )}
      {cursor && (
        <Button
          disabled={loading}
          onClick={async () => {
            setLoading(true);
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
          Load more objects
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
}: {
  source: Source;
  entry: SemanticEntry;
  agents: Agent[];
  onClose: () => void;
}) {
  const [params, setParams] = useState(entry.template!.example_json);
  const [agent, setAgent] = useState("");
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const abort = useRef<AbortController | null>(null);
  useEffect(() => () => abort.current?.abort(), []);
  async function run(cursor = "") {
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
      title="Preview published template"
      subtitle={`${entry.name} · Execution version ${entry.template!.execution_version}`}
      onClose={() => {
        abort.current?.abort();
        onClose();
      }}
      footer={
        <>
          {busy ? (
            <Button onClick={() => abort.current?.abort()}>Cancel query</Button>
          ) : (
            <Button primary onClick={() => run()}>
              Run template
            </Button>
          )}
          {result?.next_cursor && (
            <Button disabled={busy} onClick={() => run(result.next_cursor)}>
              Next result page
            </Button>
          )}
        </>
      }
    >
      <ErrorNote error={error} />
      <Field
        label="Preview identity"
        hint="Agent previews enforce current grants and the data source's query access mode."
      >
        <select
          disabled={busy}
          value={agent}
          onChange={(e) => {
            setAgent(e.target.value);
            setResult(null);
          }}
        >
          <option value="">Administrator</option>
          {agents.map((a) => (
            <option value={a.id} key={a.id}>
              {a.name}
            </option>
          ))}
        </select>
      </Field>
      <Field label="Template parameters (JSON)">
        <textarea
          disabled={busy}
          className="query-editor"
          rows={6}
          value={params}
          onChange={(e) => {
            setParams(e.target.value);
            setResult(null);
          }}
        />
      </Field>
      {result && (
        <>
          <p className="help">
            {result.row_count} rows · {result.elapsed_ms} ms ·{" "}
            {result.truncated ? "Truncated" : "Complete page"} · Request{" "}
            {result.request_id}
          </p>
          {result.ontology_context && (
            <div className="ontology-result-context">
              <strong>
                Ontology {result.ontology_context.ontology_id} · version{" "}
                {result.ontology_context.version}
              </strong>
              <p className="help">
                Source publication {result.semantic_version} · template
                execution {result.template_version}
              </p>
              <ul>
                {result.ontology_context.concept_refs.map((ref) => (
                  <li key={ref}>
                    <code>{ref}</code>
                  </li>
                ))}
              </ul>
            </div>
          )}
          <pre className="semantic-result">
            {JSON.stringify(result.data, null, 2)}
          </pre>
        </>
      )}
    </Drawer>
  );
}
