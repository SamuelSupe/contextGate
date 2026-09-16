import { useEffect, useState } from "react";
import { api, message } from "./api";
import { Button, ErrorNote, Field, Loading } from "./components";
import { t } from "./i18n";
import { mappedConcepts } from "./query-concepts";
import type { OntologyBinding, OntologyVersion } from "./ontology-types";

export function useOntologyVersion(binding?: OntologyBinding | null) {
  const id = binding?.ontology_id,
    version = binding?.version;
  const [data, setData] = useState<OntologyVersion | null>(null);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setData(null);
    setError("");
    if (id && version) {
      api<OntologyVersion>(
        `/api/ontologies/${encodeURIComponent(id)}/versions/${encodeURIComponent(version)}`,
        { signal: controller.signal },
      )
        .then(setData)
        .catch((e) => {
          if (!controller.signal.aborted) setError(message(e));
        });
    }
    return () => controller.abort();
  }, [id, version, attempt]);
  return {
    data:
      data && data.ontology_id === id && data.version === version ? data : null,
    error,
    retry: () => setAttempt((n) => n + 1),
  };
}

export function BusinessConcepts({
  binding,
  selected,
  onChange,
}: {
  binding?: OntologyBinding | null;
  selected: string[];
  onChange?: (refs: string[]) => void;
}) {
  const { data, error, retry } = useOntologyVersion(binding);
  const [search, setSearch] = useState("");
  const [inspected, setInspected] = useState("");
  const options =
    binding && data ? mappedConcepts(binding, data.definition) : [];
  const active = options.find((c) => c.ref === inspected);
  const matches = options.filter((c) =>
    `${c.name} ${c.ref} ${c.description}`
      .toLowerCase()
      .includes(search.trim().toLowerCase()),
  );
  if (!onChange && !selected.length) return null;
  return (
    <section className="query-concepts">
      <div className="query-concepts-heading">
        <h3>{t("Business concepts")}</h3>
        {binding && (
          <span className="help">
            {data?.definition.name || binding.ontology_id} · v{binding.version}
          </span>
        )}
      </div>
      <ErrorNote error={error} />
      {error ? (
        <Button onClick={retry}>{t("Retry")}</Button>
      ) : binding && !data ? (
        <Loading />
      ) : null}
      {selected.length > 0 && (
        <div className="concept-chips" aria-label={t("Linked concepts")}>
          {selected.map((ref) => {
            const concept = options.find((c) => c.ref === ref);
            return (
              <div className="concept-chip" key={ref}>
                <button
                  type="button"
                  aria-pressed={inspected === ref}
                  onClick={() => setInspected(inspected === ref ? "" : ref)}
                >
                  {concept?.name || ref}
                </button>
                {onChange && (
                  <button
                    type="button"
                    aria-label={t("Remove {name}", {
                      name: concept?.name || ref,
                    })}
                    onClick={() => onChange(selected.filter((v) => v !== ref))}
                  >
                    ×
                  </button>
                )}
                {!concept && (!binding || data) && (
                  <small>{t("Unmapped")}</small>
                )}
              </div>
            );
          })}
        </div>
      )}
      {active && (
        <div className="concept-definition">
          <strong>{active.name}</strong>
          <span className="status muted">{t(active.kind)}</span>
          {active.description && <p>{active.description}</p>}
          <small>
            <code>{active.ref}</code>
          </small>
        </div>
      )}
      {!selected.length && <p className="help">{t("No concepts linked")}</p>}
      {onChange &&
        (options.length ? (
          <details className="concept-picker">
            <summary>{t("Choose concepts")}</summary>
            <Field label={t("Find a concept")}>
              <input
                type="search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </Field>
            <div
              className="concept-options"
              role="group"
              aria-label={t("Available concepts")}
            >
              {matches.map((c) => (
                <label className="checkbox-row" key={c.ref}>
                  <input
                    type="checkbox"
                    checked={selected.includes(c.ref)}
                    onChange={(e) =>
                      onChange(
                        e.target.checked
                          ? [...selected, c.ref]
                          : selected.filter((ref) => ref !== c.ref),
                      )
                    }
                  />
                  <span>
                    {c.name}
                    <small>{t(c.kind)}</small>
                  </span>
                </label>
              ))}
              {!matches.length && (
                <p className="help">{t("No matching concepts")}</p>
              )}
            </div>
          </details>
        ) : !error && (!binding || data) ? (
          <p className="help">
            {t("Map concepts in this data source to link them to a query.")}
          </p>
        ) : null)}
    </section>
  );
}
