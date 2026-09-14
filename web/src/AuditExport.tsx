import { t } from "./i18n";
import { useEffect, useState } from "react";
import { api, date, message, payload } from "./api";
import { Button, ErrorNote, Field, Loading } from "./components";
import { useNavigationGuard } from "./useNavigationGuard";

type ExportConfig = {
  enabled: boolean;
  protocol: "http/protobuf" | "grpc";
  endpoint: string;
  service_name: string;
  ca_pem: string;
  revision: number;
};
type ExportView = {
  config: ExportConfig;
  headers_configured: boolean;
  status: {
    state: string;
    pending: number;
    accepted: number;
    rejected: number;
    last_attempt?: string;
    last_success?: string;
    last_error?: string;
    next_attempt?: string;
  };
};
type TestResult = { accepted: boolean; message: string; checked_at: string };
const endpoint = "/api/settings/audit-export";

export function AuditExport({ notify }: { notify: (text: string) => void }) {
  const [view, setView] = useState<ExportView | null>(null);
  const [config, setConfig] = useState<ExportConfig | null>(null);
  const [savedConfig, setSavedConfig] = useState<ExportConfig | null>(null);
  const [headers, setHeaders] = useState("");
  const [clearHeaders, setClearHeaders] = useState(false);
  const [error, setError] = useState("");
  const [statusError, setStatusError] = useState("");
  const [busy, setBusy] = useState(false);
  const [test, setTest] = useState<TestResult | null>(null);
  const dirty =
    !!config &&
    (JSON.stringify(config) !== JSON.stringify(savedConfig) ||
      !!headers ||
      clearHeaders);
  useNavigationGuard(dirty, busy);

  function adopt(value: ExportView) {
    setView(value);
    setConfig(value.config);
    setSavedConfig(value.config);
    setHeaders("");
    setClearHeaders(false);
    setTest(null);
    setStatusError("");
  }
  useEffect(() => {
    const abort = new AbortController();
    api<ExportView>(endpoint, { signal: abort.signal })
      .then(adopt)
      .catch((e) => {
        if (!abort.signal.aborted) setError(message(e));
      });
    return () => abort.abort();
  }, []);

  useEffect(() => {
    if (!view?.config.enabled) return;
    const abort = new AbortController();
    let polling = false;
    const timer = setInterval(async () => {
      if (polling) return;
      polling = true;
      try {
        const value = await api<ExportView>(endpoint, { signal: abort.signal });
        setView((current) =>
          !current || value.config.revision >= current.config.revision
            ? value
            : current,
        );
        setStatusError("");
      } catch (e) {
        if (!abort.signal.aborted) setStatusError(message(e));
      } finally {
        polling = false;
      }
    }, 5000);
    return () => {
      clearInterval(timer);
      abort.abort();
    };
  }, [view?.config.enabled]);

  function edit(patch: Partial<ExportConfig>) {
    setConfig((current) => (current ? { ...current, ...patch } : current));
    setTest(null);
  }
  async function submit(testOnly: boolean) {
    setError("");
    setTest(null);
    let parsed: Record<string, string> | undefined;
    if (headers.trim()) {
      try {
        const value: unknown = JSON.parse(headers);
        if (
          !value ||
          typeof value !== "object" ||
          Array.isArray(value) ||
          Object.values(value).some((v) => typeof v !== "string")
        )
          throw new Error();
        parsed = value as Record<string, string>;
      } catch {
        setError(
          t(
            'Headers must be a JSON object with string values, such as {"Authorization":"Bearer …"}.',
          ),
        );
        return;
      }
    }
    setBusy(true);
    try {
      const body = payload({
        ...config,
        headers: parsed,
        clear_headers: clearHeaders,
      });
      if (testOnly) {
        setTest(
          await api<TestResult>(endpoint + "/test", { method: "POST", body }),
        );
      } else {
        adopt(await api<ExportView>(endpoint, { method: "PUT", body }));
        notify(t("Audit log export settings saved"));
      }
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function reload() {
    setError("");
    setBusy(true);
    try {
      adopt(await api<ExportView>(endpoint));
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section>
      <h2>{t("Audit log export")}</h2>
      <p className="help">
        {t(
          "Send persisted audit events to an OpenTelemetry Collector or an OTLP Logs endpoint. Query text, parameters, results and database credentials are excluded.",
        )}
      </p>
      <ErrorNote error={error} />
      {!config || !view ? (
        <>
          {error ? (
            <Button onClick={reload} busy={busy}>
              {t("Reload export settings")}
            </Button>
          ) : (
            <Loading />
          )}
        </>
      ) : (
        <>
          <form
            className="otlp-form"
            onSubmit={(e) => {
              e.preventDefault();
              void submit(false);
            }}
          >
            <fieldset disabled={busy} className="otlp-fields">
              <label className="check-row">
                <input
                  type="checkbox"
                  checked={config.enabled}
                  onChange={(e) => edit({ enabled: e.target.checked })}
                />
                {t("Enable OTLP audit export")}
              </label>
              <div className="otlp-grid">
                <Field label={t("Protocol")}>
                  <select
                    value={config.protocol}
                    onChange={(e) =>
                      edit({
                        protocol: e.target.value as ExportConfig["protocol"],
                      })
                    }
                  >
                    <option value="http/protobuf">
                      {t("OTLP HTTP / protobuf")}
                    </option>
                    <option value="grpc">{t("OTLP gRPC")}</option>
                  </select>
                </Field>
                <Field
                  label={t("Service name")}
                  hint={t("OTLP resource service.name")}
                >
                  <input
                    value={config.service_name}
                    maxLength={128}
                    placeholder={t("contextgate")}
                    onChange={(e) => edit({ service_name: e.target.value })}
                  />
                </Field>
              </div>
              <Field
                label={t("Endpoint")}
                required={config.enabled}
                hint={
                  config.protocol === "grpc"
                    ? t(
                        "Use an HTTP(S) origin with a port and no path. HTTPS enables TLS.",
                      )
                    : t(
                        "Use the full logs URL. An origin without a path defaults to /v1/logs. HTTPS enables TLS.",
                      )
                }
              >
                <input
                  type="url"
                  required={config.enabled}
                  value={config.endpoint}
                  maxLength={2048}
                  placeholder={
                    config.protocol === "grpc"
                      ? "http://collector:4317"
                      : "http://collector:4318/v1/logs"
                  }
                  onChange={(e) => edit({ endpoint: e.target.value })}
                />
              </Field>
              <details className="otlp-advanced">
                <summary>{t("Authentication and TLS")}</summary>
                <Field
                  label={t("Headers (JSON)")}
                  hint={
                    view.headers_configured
                      ? t(
                          "Headers are stored encrypted. Leave blank to keep them; new JSON replaces all headers. Stored values are never shown.",
                        )
                      : t(
                          "Optional authentication or custom headers. Values are stored encrypted and are never returned by the API.",
                        )
                  }
                >
                  <textarea
                    value={headers}
                    rows={3}
                    maxLength={20000}
                    autoComplete="off"
                    spellCheck={false}
                    placeholder={'{"Authorization":"Bearer <TOKEN>"}'}
                    onChange={(e) => {
                      setHeaders(e.target.value);
                      setTest(null);
                    }}
                  />
                </Field>
                {view.headers_configured && (
                  <label className="check-row">
                    <input
                      type="checkbox"
                      checked={clearHeaders}
                      onChange={(e) => {
                        setClearHeaders(e.target.checked);
                        setTest(null);
                      }}
                    />
                    {t("Clear stored headers")}
                  </label>
                )}
                <Field
                  label={t("CA certificates (PEM)")}
                  hint={t(
                    "Optional CA certificates added to system trust. Certificate verification is always enabled for HTTPS.",
                  )}
                >
                  <textarea
                    value={config.ca_pem}
                    rows={3}
                    maxLength={262144}
                    autoComplete="off"
                    spellCheck={false}
                    placeholder={t("-----BEGIN CERTIFICATE-----")}
                    onChange={(e) => edit({ ca_pem: e.target.value })}
                  />
                </Field>
              </details>
              <p className="help">
                {t(
                  "Enabling starts with new events. Disabling stops delivery and clears the export backlog; local audit records stay available for 30 days. Changes to an enabled destination also apply to queued events.",
                )}
              </p>
              <div className="otlp-actions">
                <Button type="submit" primary busy={busy}>
                  {t("Save export settings")}
                </Button>
                <Button
                  type="button"
                  disabled={busy || !config.endpoint}
                  onClick={() => void submit(true)}
                >
                  {t("Send test log")}
                </Button>
                <Button type="button" disabled={busy} onClick={reload}>
                  {dirty ? t("Discard changes") : t("Reload settings")}
                </Button>
              </div>
              {dirty && (
                <p className="help" role="status">
                  {t(
                    "Unsaved export settings. Save or discard before leaving.",
                  )}
                </p>
              )}
            </fieldset>
          </form>
          {test && (
            <div
              className={`notice ${test.accepted ? "success" : "error"}`}
              role="status"
            >
              <span>
                {t(test.message)} {date(test.checked_at)}
                {t(". The test uses the current form without saving it.")}
              </span>
            </div>
          )}
          <details
            className="otlp-delivery"
            open={
              view.config.enabled || !!view.status.last_error || !!statusError
            }
          >
            <summary>
              {t("Delivery status · ")}
              <span className="capitalize">{t(view.status.state)}</span>
            </summary>
            {statusError && (
              <div className="notice warning" role="status">
                {t(
                  "Status refresh failed. Displaying the last received status: ",
                )}
                {statusError}
              </div>
            )}
            <dl className="settings-list">
              <div>
                <dt>{t("State")}</dt>
                <dd className="capitalize">{t(view.status.state)}</dd>
              </div>
              <div>
                <dt>{t("Pending events")}</dt>
                <dd>{view.status.pending}</dd>
              </div>
              <div>
                <dt>{t("Accepted / rejected")}</dt>
                <dd>
                  {view.status.accepted} / {view.status.rejected}
                </dd>
              </div>
              <div>
                <dt>{t("Last accepted batch")}</dt>
                <dd>{date(view.status.last_success)}</dd>
              </div>
              {view.status.next_attempt && (
                <div>
                  <dt>{t("Next retry")}</dt>
                  <dd>{date(view.status.next_attempt)}</dd>
                </div>
              )}
            </dl>
            {view.status.last_error && (
              <div className="notice warning" role="status">
                {view.status.last_error}
                {view.status.state === "blocked"
                  ? t(
                      " Correct and save the settings to resume subsequent events.",
                    )
                  : ""}
              </div>
            )}
            <p className="help">
              {t(
                "Transient failures retry from a saved checkpoint while records remain in local retention. Acknowledgement loss can cause duplicates; use service.instance.id and mcpdbhub.audit.id to deduplicate. Permanent or partial rejections are counted and not retried.",
              )}
            </p>
          </details>
        </>
      )}
    </section>
  );
}
