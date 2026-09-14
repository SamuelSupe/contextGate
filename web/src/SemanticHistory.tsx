import { useEffect, useState } from "react";
import { api, date, message, payload } from "./api";
import { Button, Drawer, Empty, ErrorNote, Loading } from "./components";
import { t } from "./i18n";
import type { SemanticSnapshot, SemanticState } from "./semantic-types";

type Version = {
  version: string;
  published_at?: string;
  entry_count: number;
  snapshot?: SemanticSnapshot;
};
type Detail = {
  version: Version;
  revision: string;
  changes: { id: string; name: string; change: string; fields: string[] }[];
};

export function SemanticHistory({
  endpoint,
  state,
  onClose,
  accept,
}: {
  endpoint: string;
  state: SemanticState;
  onClose: () => void;
  accept: (state: SemanticState) => void;
}) {
  const [versions, setVersions] = useState<Version[]>([]);
  const [pages, setPages] = useState<string[]>([""]);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const before = pages[pages.length - 1];
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    api<Version[]>(`${endpoint}/versions?before=${before}`, {
      signal: controller.signal,
    })
      .then(setVersions)
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [endpoint, before]);
  async function inspect(version: string) {
    setBusy(true);
    setError("");
    setConfirm(false);
    setDeleting(false);
    setDetail(null);
    try {
      const next = await api<Detail>(`${endpoint}/versions/${version}`);
      if (next.revision !== state.revision)
        throw new Error(
          t(
            "The draft changed in another tab. Close history and refresh semantics before comparing or restoring.",
          ),
        );
      setDetail(next);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Drawer
      title={t("Publication history")}
      subtitle={t(
        "Compare a saved version with the current draft, then restore it for review.",
      )}
      onClose={() => {
        if (!busy) onClose();
      }}
      wide
    >
      <ErrorNote error={error} />
      <p className="help">
        {t(
          "Earlier releases retained only the current publication. History starts with the first retained snapshot. Restoring never changes the live publication.",
        )}
      </p>
      {loading ? (
        <Loading />
      ) : !versions.length ? (
        <Empty
          title={t("No published versions")}
          description={t("Publish a verified draft to start its history.")}
        />
      ) : (
        <>
          <div className="history-versions">
            {versions.map((v) => (
              <Button
                key={v.version}
                disabled={busy}
                aria-pressed={detail?.version.version === v.version}
                onClick={() => inspect(v.version)}
              >
                {t("Version {version}", { version: v.version })} ·{" "}
                {v.published_at
                  ? date(v.published_at)
                  : t("Retained during upgrade")}{" "}
                · {t("{count} entries", { count: v.entry_count })}
              </Button>
            ))}
          </div>
          <div className="button-row pagination">
            <Button
              disabled={busy || pages.length < 2}
              onClick={() => setPages((p) => p.slice(0, -1))}
            >
              {t("Newer records")}
            </Button>
            <Button
              disabled={busy || versions.length < 25}
              onClick={() =>
                setPages((p) => [...p, versions[versions.length - 1].version])
              }
            >
              {t("Older records")}
            </Button>
          </div>
        </>
      )}
      {busy && !detail ? <Loading /> : null}
      {detail && (
        <section>
          <h2>
            {t("Version {version} compared with the draft", {
              version: detail.version.version,
            })}
          </h2>
          {!detail.changes.length ? (
            <p>{t("This version matches the current draft.")}</p>
          ) : (
            detail.changes.map((change) => (
              <details className="history-change" key={change.id}>
                <summary>
                  {change.name} · {t(change.change)}
                  {change.fields.length ? ` · ${change.fields.join(", ")}` : ""}
                </summary>
                <div className="history-diff">
                  <div>
                    <h3>{t("Saved version")}</h3>
                    <pre>
                      {JSON.stringify(
                        change.id === "overview"
                          ? {
                              overview: detail.version.snapshot?.overview,
                              ontology: detail.version.snapshot?.ontology,
                            }
                          : (detail.version.snapshot?.entries.find(
                              (e) => e.id === change.id,
                            ) ?? null),
                        null,
                        2,
                      )}
                    </pre>
                  </div>
                  <div>
                    <h3>{t("Current draft")}</h3>
                    <pre>
                      {JSON.stringify(
                        change.id === "overview"
                          ? {
                              overview: state.draft.overview,
                              ontology: state.draft.ontology,
                            }
                          : (state.draft.entries.find(
                              (e) => e.id === change.id,
                            ) ?? null),
                        null,
                        2,
                      )}
                    </pre>
                  </div>
                </div>
              </details>
            ))
          )}
          <div className="notice warning">
            {t(
              "Restoring replaces the saved draft. Enabled templates must pass a new trial, and ontology mappings must be checked again, before you can publish.",
            )}
          </div>
          <label className="check">
            <input
              type="checkbox"
              checked={confirm}
              disabled={busy}
              onChange={(e) => setConfirm(e.target.checked)}
            />
            {t("Replace the draft with this version for review")}
          </label>
          <Button
            primary
            busy={busy}
            disabled={!confirm}
            onClick={async () => {
              setBusy(true);
              setError("");
              try {
                const next = await api<SemanticState>(endpoint + "/restore", {
                  method: "POST",
                  body: payload({
                    revision: detail.revision,
                    version: detail.version.version,
                  }),
                });
                accept(next);
                onClose();
              } catch (e) {
                setError(message(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            {t("Restore as draft")}
          </Button>
          {detail.version.version !== state.published_version && (
            <div className="baseline-actions">
              {deleting ? (
                <>
                  <p className="amber">
                    {t(
                      "Delete this historical snapshot permanently? Its query definitions can no longer be restored. The current draft and publication are kept.",
                    )}
                  </p>
                  <div className="button-row">
                    <Button
                      className="danger"
                      disabled={busy}
                      onClick={async () => {
                        setBusy(true);
                        setError("");
                        try {
                          await api(
                            `${endpoint}/versions/${detail.version.version}`,
                            {
                              method: "DELETE",
                              body: payload({ revision: detail.revision }),
                            },
                          );
                          setVersions((items) =>
                            items.filter(
                              (v) => v.version !== detail.version.version,
                            ),
                          );
                          setDetail(null);
                          setDeleting(false);
                        } catch (e) {
                          setError(message(e));
                        } finally {
                          setBusy(false);
                        }
                      }}
                    >
                      {t("Delete historical version")}
                    </Button>
                    <Button disabled={busy} onClick={() => setDeleting(false)}>
                      {t("Cancel")}
                    </Button>
                  </div>
                </>
              ) : (
                <Button
                  className="danger"
                  disabled={busy}
                  onClick={() => setDeleting(true)}
                >
                  {t("Delete historical version")}
                </Button>
              )}
            </div>
          )}
        </section>
      )}
    </Drawer>
  );
}
