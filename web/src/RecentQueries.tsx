import { useEffect, useState } from "react";
import { api } from "./api";
import { Button } from "./components";
import { t } from "./i18n";
import { queryToolURL, type QueryJourney } from "./query-publishing";
import type { SemanticState } from "./semantic-types";
import type { Source } from "./types";

export function RecentQueries({
  journeys,
  sources,
  navigate,
}: {
  journeys: QueryJourney[];
  sources: Source[];
  navigate: (url: string) => void;
}) {
  const [states, setStates] = useState<Record<string, SemanticState | null>>(
    {},
  );
  const [retry, setRetry] = useState(0);
  const sourceIDs = JSON.stringify([
    ...new Set(journeys.map((j) => j.source_id)),
  ]);
  useEffect(() => {
    const abort = new AbortController();
    setStates({});
    void Promise.all(
      (JSON.parse(sourceIDs) as string[]).map(async (id) => {
        try {
          const state = await api<SemanticState>(
            `/api/sources/${encodeURIComponent(id)}/semantics`,
            { signal: abort.signal },
          );
          if (!abort.signal.aborted) setStates((s) => ({ ...s, [id]: state }));
        } catch {
          if (!abort.signal.aborted) setStates((s) => ({ ...s, [id]: null }));
        }
      }),
    );
    return () => abort.abort();
  }, [sourceIDs, retry]);
  if (!journeys.length) return null;
  return (
    <section className="home-recent" aria-label={t("Other recent queries")}>
      <h2>{t("Other recent queries")}</h2>
      <ul>
        {journeys.map((journey) => {
          const source = sources.find((s) => s.id === journey.source_id);
          if (!source) return null;
          const state = states[source.id];
          const entry =
            state?.draft.entries.find(
              (e) => e.id === journey.template_id && e.template,
            ) ||
            state?.published.entries.find(
              (e) => e.id === journey.template_id && e.template,
            );
          return (
            <li key={`${source.id}:${journey.template_id}`}>
              <div>
                <button
                  className="text-button name-link"
                  disabled={!entry}
                  onClick={() =>
                    navigate(
                      queryToolURL(
                        source.id,
                        journey.template_id,
                        journey.agent_id,
                      ),
                    )
                  }
                >
                  {entry?.name ||
                    t(
                      state === undefined
                        ? "Loading…"
                        : state === null
                          ? "Status unavailable"
                          : "Query no longer available",
                    )}
                </button>
                <small>{source.name}</small>
              </div>
              {state === null ? (
                <Button onClick={() => setRetry((v) => v + 1)}>
                  {t("Retry")}
                </Button>
              ) : (
                entry && (
                  <span className="help">
                    {t(
                      !source.enabled
                        ? "Source disabled"
                        : state?.published.entries.some(
                              (e) => e.id === journey.template_id,
                            )
                          ? "Published"
                          : "Draft",
                    )}
                  </span>
                )
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
