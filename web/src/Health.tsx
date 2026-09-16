import { useCallback, useEffect, useState } from "react";
import { api, date, message, payload } from "./api";
import { Button, Empty, ErrorNote, Field, Loading } from "./components";
import { useNavigationGuard } from "./useNavigationGuard";
import { t } from "./i18n";
import type { Agent, Probe } from "./types";

type Config = { revision: string; enabled: boolean; interval_minutes: number };
type Evidence = {
  checked_at: string;
  probe: Probe;
  coverage_limited: boolean;
  structure: {
    namespace: string;
    object: string;
    status: string;
    changes?: {
      column: string;
      change: string;
      before?: string;
      after?: string;
    }[];
    detail_limited?: boolean;
  }[];
};
type SourceHealth = {
  id: string;
  name: string;
  status: string;
  health: Evidence;
  invalid_templates: { id: string; name: string; status: string }[];
};
type Overview = {
  sources: SourceHealth[];
  total: number;
  expiring_agents: Agent[];
  pending_changes: number;
  audit_export: { state: string; pending: number; last_error?: string };
};
export const healthStatusLabels: Record<string, string> = {
  overdue: "Health check overdue",
  structure_incomplete: "Structure needs attention",
  object_missing: "Object not found",
  not_checked: "Health check not run",
  checked: "Health check completed",
  connection_failed: "Health check connection failed",
  structure_changed: "Structure changed",
  stale: "Health check needs refreshing",
  disabled: "Disabled",
  unchanged: "Unchanged",
  baseline: "Baseline recorded",
  changed: "Changed",
  unverified: "Not verified",
  incomplete: "Incomplete",
};

export function HealthPage({
  superAdmin,
  navigate,
  reloadSources,
}: {
  superAdmin: boolean;
  navigate: (path: string) => void;
  reloadSources: () => Promise<void>;
}) {
  const [view, setView] = useState<Overview | null>(null);
  const [saved, setSaved] = useState<Config | null>(null);
  const [config, setConfig] = useState<Config | null>(null);
  const [page, setPage] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [loading, setLoading] = useState(true);
  const [confirm, setConfirm] = useState("");
  const load = useCallback(
    async (signal?: AbortSignal) => {
      const [overview, settings] = await Promise.all([
        api<Overview>(`/api/health?offset=${page * 20}`, { signal }),
        superAdmin
          ? api<Config>("/api/settings/health", { signal })
          : Promise.resolve(null),
      ]);
      setView(overview);
      setSaved(settings);
      setConfig(settings);
    },
    [page, superAdmin],
  );
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    load(controller.signal)
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [load]);
  async function action(name: string, fn: () => Promise<void>) {
    setBusy(name);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy("");
    }
  }
  const dirty = JSON.stringify(saved) !== JSON.stringify(config);
  useNavigationGuard(dirty, !!busy);
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Health")}</h1>
        </div>
        <Button
          disabled={!!busy || dirty}
          onClick={() => action("refresh", () => load())}
        >
          {t("Refresh")}
        </Button>
      </div>
      <ErrorNote error={error} />
      {loading ? (
        <Loading />
      ) : !view || (superAdmin && !config) ? (
        <Button onClick={() => action("retry", () => load())}>
          {t("Retry")}
        </Button>
      ) : (
        <>
          {superAdmin && config && (
            <details className="health-schedule">
              <summary>
                {t("Scheduled checks")} ·{" "}
                {saved?.enabled
                  ? t("Every {minutes} minutes", {
                      minutes: saved.interval_minutes,
                    })
                  : t("Off")}
              </summary>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  action("settings", async () => {
                    const value = await api<Config>("/api/settings/health", {
                      method: "PUT",
                      body: payload(config),
                    });
                    setSaved(value);
                    setConfig(value);
                  });
                }}
              >
                <p className="help">
                  {t(
                    "Off by default. Enabled checks run sequentially against enabled sources, with a 30-second budget per source. They inspect connections and up to 20 published object references, never business samples or template results.",
                  )}
                </p>
                <label className="check">
                  <input
                    type="checkbox"
                    checked={config.enabled}
                    disabled={!!busy}
                    onChange={(e) =>
                      setConfig({ ...config, enabled: e.target.checked })
                    }
                  />
                  {t("Enable scheduled read-only checks")}
                </label>
                <Field label={t("Interval (minutes)")}>
                  <input
                    type="number"
                    required
                    min={5}
                    max={1440}
                    disabled={!!busy}
                    value={config.interval_minutes}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        interval_minutes: Number(e.target.value),
                      })
                    }
                  />
                </Field>
                <div className="button-row">
                  <Button
                    primary
                    type="submit"
                    busy={busy === "settings"}
                    disabled={!dirty || !!busy}
                  >
                    {t("Save settings")}
                  </Button>
                  <Button
                    disabled={!dirty || !!busy}
                    onClick={() => setConfig(saved)}
                  >
                    {t("Discard changes")}
                  </Button>
                </div>
              </form>
            </details>
          )}
          {view.pending_changes > 0 && (
            <div className="notice warning">
              <p>
                {t(
                  "{count} management operations have no recorded outcome. Check the audit log before retrying a change.",
                  { count: view.pending_changes },
                )}
              </p>
              <Button onClick={() => navigate("/audit")}>
                {t("Open audit log")}
              </Button>
            </div>
          )}
          {view.audit_export.last_error && (
            <div className="notice warning">
              <p>
                {t(
                  "Audit export needs attention. {count} records are pending.",
                  { count: view.audit_export.pending },
                )}
              </p>
              <Button onClick={() => navigate("/settings")}>
                {t("Review export settings")}
              </Button>
            </div>
          )}
          {view.expiring_agents.length > 0 && (
            <section className="health-card">
              <h2>{t("Agent credentials expiring within 7 days")}</h2>
              {view.expiring_agents.map((agent) => (
                <div className="health-expiring-agent" key={agent.id}>
                  <span>
                    <strong>{agent.name}</strong> · {date(agent.expires_at)}
                  </span>
                  <Button
                    onClick={() =>
                      navigate(
                        "/agents?attention=" + encodeURIComponent(agent.id),
                      )
                    }
                  >
                    {t("Review credential")}
                  </Button>
                </div>
              ))}
            </section>
          )}
          {!view.sources.length ? (
            <Empty
              title={t("No data sources to check")}
              description={t(
                "Add a data source, then check its connection and publish the objects you want to monitor.",
              )}
              action={
                <Button primary onClick={() => navigate("/sources")}>
                  {t("Data sources")}
                </Button>
              }
            />
          ) : (
            <div className="health-sources">
              {view.sources.map((source) => (
                <section key={source.id} className="health-card">
                  <div className="health-card-header">
                    <div>
                      <h2>{source.name}</h2>
                      <span
                        className={`status ${["connection_failed", "structure_changed", "structure_incomplete", "stale"].includes(source.status) || source.invalid_templates.length ? "amber" : ""}`}
                      >
                        {t(healthStatusLabels[source.status] || source.status)}
                      </span>
                      <p className="help">
                        {source.health.checked_at &&
                        !source.health.checked_at.startsWith("0001")
                          ? t("Last checked {time}", {
                              time: date(source.health.checked_at),
                            })
                          : t("No health check evidence yet")}
                      </p>
                    </div>
                    <div className="button-row">
                      <Button
                        busy={busy === source.id}
                        disabled={
                          !!busy || dirty || source.status === "disabled"
                        }
                        onClick={() =>
                          action(source.id, async () => {
                            await api(`/api/health/${source.id}/check`, {
                              method: "POST",
                              body: "{}",
                            });
                            await Promise.all([load(), reloadSources()]);
                          })
                        }
                      >
                        {t("Check now")}
                      </Button>
                      <Button
                        onClick={() => navigate(`/sources/${source.id}/setup`)}
                      >
                        {t("Open workspace")}
                      </Button>
                    </div>
                  </div>
                  {source.health.probe?.error && (
                    <p className="amber">{source.health.probe.error.code}</p>
                  )}
                  {!!source.invalid_templates.length && (
                    <div className="notice warning">
                      <p>
                        {t(
                          "{count} published templates need a new trial or publication.",
                          { count: source.invalid_templates.length },
                        )}
                      </p>
                      <Button
                        onClick={() =>
                          navigate(
                            `/sources/${source.id}/semantics?tab=Query%20templates`,
                          )
                        }
                      >
                        {t("Review templates")}
                      </Button>
                    </div>
                  )}
                  {!!source.health.structure?.length && (
                    <details>
                      <summary>{t("Structure evidence")}</summary>
                      <ul>
                        {source.health.structure.map((item) => (
                          <li
                            key={JSON.stringify([item.namespace, item.object])}
                          >
                            <code>
                              {[item.namespace, item.object]
                                .filter(Boolean)
                                .join(".")}
                            </code>{" "}
                            ·{" "}
                            {t(healthStatusLabels[item.status] || item.status)}
                            <ul>
                              {item.changes?.map((change) => (
                                <li key={change.column}>
                                  <code>{change.column}</code> ·{" "}
                                  {t(change.change)} ·{" "}
                                  {change.before ? `${change.before} → ` : ""}
                                  {change.after || "∅"}
                                </li>
                              ))}
                            </ul>
                            {item.detail_limited && (
                              <p className="help">
                                {t(
                                  "Field differences are incomplete for this object. Inspect its structure in the workspace before accepting the baseline.",
                                )}
                              </p>
                            )}
                          </li>
                        ))}
                      </ul>
                      <p className="help">
                        {t(
                          "This compares discoverable names and native types. Unverified fields and unlisted objects are not covered. A successful check does not prove that all queries or business data are correct.",
                        )}
                      </p>
                      {source.health.coverage_limited && (
                        <p className="amber">
                          {t(
                            "Only the first 20 published objects were checked.",
                          )}
                        </p>
                      )}
                    </details>
                  )}
                  {source.status === "structure_changed" && (
                    <div className="baseline-actions">
                      {confirm === source.id ? (
                        <>
                          <p>
                            {t(
                              "Accept these discovered names and types as the new baseline? This does not revalidate templates or mappings.",
                            )}
                          </p>
                          <div className="button-row">
                            <Button
                              primary
                              disabled={!!busy || dirty}
                              onClick={() =>
                                action("baseline", async () => {
                                  await api(
                                    `/api/health/${source.id}/baseline`,
                                    {
                                      method: "POST",
                                      body: payload({
                                        checked_at: source.health.checked_at,
                                      }),
                                    },
                                  );
                                  setConfirm("");
                                  await load();
                                })
                              }
                            >
                              {t("Accept baseline")}
                            </Button>
                            <Button onClick={() => setConfirm("")}>
                              {t("Cancel")}
                            </Button>
                          </div>
                        </>
                      ) : (
                        <Button
                          disabled={!!busy}
                          onClick={() => setConfirm(source.id)}
                        >
                          {t("Review new baseline")}
                        </Button>
                      )}
                    </div>
                  )}
                </section>
              ))}
            </div>
          )}
          {view.total > 20 && (
            <div className="button-row pagination">
              <Button
                disabled={page === 0 || !!busy || dirty}
                onClick={() => setPage((n) => n - 1)}
              >
                {t("Previous")}
              </Button>
              <span>{t("Page {number}", { number: page + 1 })}</span>
              <Button
                disabled={(page + 1) * 20 >= view.total || !!busy || dirty}
                onClick={() => setPage((n) => n + 1)}
              >
                {t("Next")}
              </Button>
            </div>
          )}
        </>
      )}
    </>
  );
}
