import { useEffect, useState } from "react";
import { api, message, payload } from "./api";
import { Button, Drawer, ErrorNote, Loading } from "./components";
import { t } from "./i18n";
import { semanticKindLabel, type SemanticState } from "./semantic-types";
import "./business-workflows.css";

interface Impact {
  revision: string;
  published_version: string;
  changes: {
    id: string;
    name: string;
    kind: string;
    change: string;
    effect: string;
    templates: string[];
  }[];
  issues: { id: string; name: string; action: string; status: string }[];
  validation_error: string;
  agents: { id: string; name: string }[];
  interrupted_templates: number;
  can_publish: boolean;
}

export function SemanticReview({
  endpoint,
  state,
  onState,
  onClose,
  onRepair,
  onPublished,
}: {
  endpoint: string;
  state: SemanticState;
  onState: (state: SemanticState) => void;
  onClose: () => void;
  onRepair: (tab: string, id?: string) => void;
  onPublished: () => void;
}) {
  const [impact, setImpact] = useState<Impact | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [loadError, setLoadError] = useState("");
  const [refresh, setRefresh] = useState(0);
  useEffect(() => {
    const abort = new AbortController();
    setLoading(true);
    setLoadError("");
    api<Impact>(`${endpoint}/impact?revision=${state.revision}`, {
      signal: abort.signal,
    })
      .then(setImpact)
      .catch((e) => {
        if (!abort.signal.aborted) {
          setImpact(null);
          setLoadError(message(e));
        }
      })
      .finally(() => {
        if (!abort.signal.aborted) setLoading(false);
      });
    return () => abort.abort();
  }, [endpoint, state, refresh]);
  async function action(name: string, templateID = "") {
    setBusy(name + templateID);
    setError("");
    try {
      await api(`${endpoint}/${name}`, {
        method: "POST",
        body: payload({ revision: state.revision, template_id: templateID }),
      });
      onState(await api<SemanticState>(endpoint));
      if (name === "publish") onPublished();
    } catch (e) {
      setError(message(e));
      if (name !== "publish") {
        try {
          onState(await api<SemanticState>(endpoint));
        } catch {
          setLoadError(
            t("Could not refresh validation. Reload before publishing."),
          );
          setImpact(null);
        }
      }
    } finally {
      setBusy("");
    }
  }
  const hasChanges = !!impact?.changes.length;
  return (
    <Drawer
      wide
      title={t("Review and publish")}
      subtitle={t(
        "See what changes, resolve the remaining checks, then publish one snapshot.",
      )}
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={!!busy} onClick={onClose}>
            {t("Close")}
          </Button>
          <Button
            primary
            busy={busy === "publish"}
            disabled={
              !!busy ||
              loading ||
              !!loadError ||
              !impact?.can_publish ||
              !hasChanges
            }
            onClick={() => action("publish")}
          >
            {t("Publish reviewed changes")}
          </Button>
        </>
      }
    >
      <ErrorNote error={error || loadError} />
      {loadError && (
        <Button
          disabled={!!busy}
          onClick={async () => {
            setBusy("reload");
            try {
              onState(await api<SemanticState>(endpoint));
              setRefresh((v) => v + 1);
            } catch (e) {
              setLoadError(message(e));
            } finally {
              setBusy("");
            }
          }}
        >
          {t("Reload review")}
        </Button>
      )}
      {loading ? (
        <Loading />
      ) : (
        impact && (
          <>
            <div className="review-summary">
              <strong>
                {t("Draft {revision} → publication {version}", {
                  revision: impact.revision,
                  version: String(BigInt(impact.published_version) + 1n),
                })}
              </strong>
              <span
                className={`status ${impact.can_publish ? "green" : "amber"}`}
              >
                {impact.can_publish
                  ? hasChanges
                    ? t("Ready to publish")
                    : t("No changes to publish")
                  : t("Resolve checks before publishing")}
              </span>
            </div>
            <section className="review-section">
              <h3>{t("1. Resolve required checks")}</h3>
              <ErrorNote error={impact.validation_error} />
              {!impact.issues.length ? (
                <p>
                  {t(
                    "Catalog, parameters, template trials and mapping evidence are current.",
                  )}
                </p>
              ) : (
                <ul className="review-issues">
                  {impact.issues.map((issue, i) => (
                    <li key={`${issue.id}:${issue.action}:${i}`}>
                      <div>
                        <strong>{issue.id ? issue.name : t(issue.name)}</strong>
                        <small>
                          {issue.action === "trial"
                            ? t("Run the saved example and regression cases")
                            : t(issue.status.replaceAll("_", " "))}
                        </small>
                      </div>
                      <div className="button-row">
                        {issue.action === "trial" ? (
                          <>
                            <Button
                              disabled={!!busy}
                              onClick={() =>
                                onRepair("Query templates", issue.id)
                              }
                            >
                              {t("Edit template")}
                            </Button>
                            <Button
                              primary
                              busy={busy === "trial" + issue.id}
                              disabled={!!busy}
                              onClick={() => action("trial", issue.id)}
                            >
                              {t("Run required trial")}
                            </Button>
                          </>
                        ) : issue.action === "check_mapping" ? (
                          <Button
                            primary
                            busy={busy === "check-mapping"}
                            disabled={!!busy}
                            onClick={() => action("check-mapping")}
                          >
                            {t("Check structure")}
                          </Button>
                        ) : (
                          <Button
                            disabled={!!busy}
                            onClick={() =>
                              onRepair(
                                issue.action === "mapping"
                                  ? "Ontology mapping"
                                  : "Catalog",
                              )
                            }
                          >
                            {t("Fix definition")}
                          </Button>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>
              )}
              <p className="help">
                {t(
                  "Trials use real read-only queries. Only the selected template and its configured cases are run.",
                )}
              </p>
            </section>
            <section className="review-section">
              <h3>{t("2. Review changes and linked queries")}</h3>
              {!hasChanges ? (
                <p>{t("Draft and published content already match.")}</p>
              ) : (
                <div className="table-scroll">
                  <table className="review-changes">
                    <thead>
                      <tr>
                        <th>{t("Change")}</th>
                        <th>{t("Effect")}</th>
                        <th>{t("Linked templates")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {impact.changes.map((change, i) => (
                        <tr key={`${change.id}:${i}`}>
                          <td>
                            <strong>
                              {change.id === "overview"
                                ? t("Overview and ontology binding")
                                : change.name}
                            </strong>
                            <small className="block">
                              {t(change.change)} ·{" "}
                              {change.kind === "mapping"
                                ? t("Ontology mapping")
                                : t(
                                    semanticKindLabel(
                                      change.kind || "overview",
                                    ),
                                  )}
                            </small>
                          </td>
                          <td>
                            {change.effect === "execution"
                              ? t("Query execution changes")
                              : t("Business context only")}
                          </td>
                          <td>
                            {change.templates.length
                              ? change.templates
                                  .map(
                                    (id) =>
                                      state.draft.entries.find(
                                        (e) => e.id === id,
                                      )?.name ||
                                      state.published.entries.find(
                                        (e) => e.id === id,
                                      )?.name ||
                                      id,
                                  )
                                  .join(" · ")
                              : "—"}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              <p className="help">
                {t(
                  "Dependencies come from explicit catalog and ontology links. Native query text is not analyzed for complete field lineage.",
                )}
              </p>
            </section>
            <section className="review-section">
              <h3>{t("3. Understand Agent impact")}</h3>
              <p>
                {impact.interrupted_templates
                  ? t(
                      "Active calls for {count} changed or revalidated templates will be cancelled. Clients must refresh their catalog.",
                      { count: impact.interrupted_templates },
                    )
                  : t(
                      "Existing template calls keep their execution version. Business context changes do not cancel running queries.",
                    )}
              </p>
              <p>
                {impact.agents.length
                  ? t("Authorized Agents that may use this source:")
                  : t(
                      "No active Agent is currently authorized for this source.",
                    )}
              </p>
              {impact.agents.length > 0 && (
                <ul className="review-agent-list">
                  {impact.agents.map((a) => (
                    <li key={a.id}>{a.name}</li>
                  ))}
                </ul>
              )}
              <p className="help">
                {t(
                  "This list reflects current grants, not proof that an Agent has used a changed template. Publication rechecks the draft revision and validation evidence.",
                )}
              </p>
            </section>
          </>
        )
      )}
    </Drawer>
  );
}
