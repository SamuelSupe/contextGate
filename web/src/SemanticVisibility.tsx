import { useState } from "react";
import { api, message, payload } from "./api";
import { Button, Drawer, Empty, ErrorNote, Field } from "./components";
import type { Agent } from "./types";

interface SearchPage {
  entries: { id: string; name: string; kind: string; description: string }[];
  next_cursor: string;
  published_version: string;
  ontology?: { id: string; version: string };
}

export function SemanticVisibility({
  endpoint,
  agents,
  onClose,
}: {
  endpoint: string;
  agents: Agent[];
  onClose: () => void;
}) {
  const [agent, setAgent] = useState("");
  const [kind, setKind] = useState("");
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState<SearchPage | null>(null);
  const [detail, setDetail] = useState<unknown>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function search(cursor = "", entry_id = "") {
    setBusy(true);
    setError("");
    setDetail(null);
    try {
      const out = await api<SearchPage>(endpoint + "/preview", {
        method: "POST",
        body: payload({
          agent_id: agent,
          kind,
          keyword,
          cursor,
          entry_id,
          limit: 20,
        }),
      });
      if (entry_id) setDetail(out);
      else setPage(out);
    } catch (e) {
      setError(message(e));
      setPage(null);
    } finally {
      setBusy(false);
    }
  }
  function reset() {
    setPage(null);
    setDetail(null);
  }
  return (
    <Drawer
      wide
      title="Preview Agent visibility"
      subtitle="Uses the selected Agent's current data source grant and the published snapshot."
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <Button disabled={busy} onClick={onClose}>
          Close
        </Button>
      }
    >
      <ErrorNote error={error} />
      <form
        onSubmit={(e) => {
          e.preventDefault();
          search();
        }}
      >
        <Field label="Agent identity" required>
          <select
            required
            value={agent}
            onChange={(e) => {
              setAgent(e.target.value);
              reset();
            }}
          >
            <option value="">Select Agent</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
                {!a.enabled ? " (disabled)" : ""}
              </option>
            ))}
          </select>
        </Field>
        <div className="field-grid">
          <Field label="Concept kind">
            <select
              value={kind}
              onChange={(e) => {
                setKind(e.target.value);
                reset();
              }}
            >
              <option value="">All visible entries</option>
              {[
                "entity_type",
                "property",
                "relation_type",
                "template",
                "term",
                "object",
                "field",
                "metric",
                "relationship",
                "overview",
              ].map((v) => (
                <option key={v}>{v}</option>
              ))}
            </select>
          </Field>
          <Field label="Keyword">
            <input
              value={keyword}
              onChange={(e) => {
                setKeyword(e.target.value);
                reset();
              }}
            />
          </Field>
        </div>
        <Button primary type="submit" busy={busy} disabled={!agent}>
          Search published semantics
        </Button>
      </form>
      {page && (
        <>
          <p className="help">
            Source publication {page.published_version}
            {page.ontology
              ? ` · Ontology ${page.ontology.id} version ${page.ontology.version}`
              : ""}
          </p>
          {page.entries.length === 0 ? (
            <Empty
              title="No visible entries"
              description="Publish a mapping or adjust the search. Drafts and unmapped concepts are hidden."
            />
          ) : (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>Name / ID</th>
                    <th>Kind</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {page.entries.map((en) => (
                    <tr key={en.id}>
                      <td>
                        {en.name}
                        <small className="block">{en.id}</small>
                      </td>
                      <td>{en.kind}</td>
                      <td>
                        <Button
                          disabled={busy}
                          onClick={() => search("", en.id)}
                        >
                          Read definition
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {page.next_cursor && (
            <Button disabled={busy} onClick={() => search(page.next_cursor)}>
              Next page
            </Button>
          )}
        </>
      )}
      {detail !== null && (
        <section>
          <h3>Visible definition and source mapping</h3>
          <pre className="query-output">{JSON.stringify(detail, null, 2)}</pre>
        </section>
      )}
    </Drawer>
  );
}
