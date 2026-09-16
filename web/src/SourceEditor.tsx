import { CloudSQLFields, isCloudSQL } from "./CloudSQLFields";
import { HTTPAPIEditor, initialHTTPAPI } from "./HTTPAPIEditor";
import { SourceChangeImpact } from "./SourceChangeImpact";
import { t } from "./i18n";
import { useEffect, useRef, useState } from "react";
import { api, APIError, message, payload } from "./api";
import { Button, Drawer, ErrorNote, Field, Protection } from "./components";
import type { Source, Capability, Probe } from "./types";
function initial(): Source {
  return {
    id: "",
    name: "",
    kind: "postgres",
    host: "",
    port: 5432,
    database: "",
    username: "",
    password: "",
    token: "",
    tls_mode: "verify",
    enabled: true,
    auth_mode: "password",
    limits: {
      timeout_seconds: 30,
      max_rows: 1000,
      max_bytes: 5 * 1024 * 1024,
      concurrency: 4,
    },
    revision: "0",
    query_revision: "0",
    has_secret: false,
    options: {},
  };
}
export function SourceEditor({
  source,
  catalog,
  onClose,
  onSaved,
  initialQueryAccessMode,
}: {
  source: Source | null;
  catalog: Capability[];
  initialQueryAccessMode?: Source["query_access_mode"];
  onClose: () => void;
  onSaved: (
    notice: string,
    close?: boolean,
    sourceID?: string,
  ) => Promise<void>;
}) {
  const [form, setForm] = useState<Source>(() =>
    source
      ? structuredClone(source)
      : { ...initial(), query_access_mode: initialQueryAccessMode },
  );
  const [savedSource, setSavedSource] = useState(source);
  const [probe, setProbe] = useState(source?.probe);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const errorRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (error) errorRef.current?.scrollIntoView({ block: "start" });
  }, [error]);
  const httpAPI = form.kind === "http_api";
  const cloudSQL = isCloudSQL(form.kind);
  const local = ["sqlite", "duckdb"].includes(form.kind);
  const field = <K extends keyof Source>(k: K, v: Source[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setProbe(undefined);
  };
  const option = (key: string, v: string) => {
    setForm((f) => ({ ...f, options: { ...f.options, [key]: v } }));
    setProbe(undefined);
  };
  async function test() {
    setBusy("test");
    setError("");
    try {
      setProbe(
        await api<Probe>("/api/sources/test", {
          method: "POST",
          body: payload(form),
        }),
      );
    } catch (e) {
      setProbe(undefined);
      setError(message(e));
    } finally {
      setBusy("");
    }
  }
  async function save() {
    setBusy("save");
    setError("");
    try {
      const saved = await api<Source>(
        savedSource ? `/api/sources/${savedSource.id}` : "/api/sources",
        { method: savedSource ? "PUT" : "POST", body: payload(form) },
      );
      setSavedSource(saved);
      setForm(saved);
      let checked: Probe;
      try {
        checked = await api<Probe>(`/api/sources/${saved.id}/test`, {
          method: "POST",
          body: "{}",
        });
      } catch (e) {
        setProbe(
          e instanceof APIError
            ? {
                connected: false,
                permission_status: "unverified",
                protection: "",
                evidence: [],
                checked_at: new Date().toISOString(),
                error: e.detail,
              }
            : undefined,
        );
        setError(
          t("Configuration saved, but the connection check failed. {value1}", {
            value1: message(e),
          }),
        );
        await onSaved("", false, saved.id);
        return;
      }
      setProbe(checked);
      await onSaved(
        t("Configuration saved and connection check passed."),
        true,
        saved.id,
      );
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy("");
    }
  }
  return (
    <Drawer
      wide={httpAPI}
      title={savedSource ? t("Configure data source") : t("Add data source")}
      onClose={onClose}
      footer={
        <>
          <Button busy={busy === "test"} disabled={!!busy} onClick={test}>
            {t("Test connection")}
          </Button>
          <Button
            primary
            busy={busy === "save"}
            disabled={!!busy}
            form="source-form"
            type="submit"
          >
            {t("Save configuration")}
          </Button>
        </>
      }
    >
      {httpAPI && (
        <nav
          className="source-config-navigation"
          aria-label={t("Source configuration sections")}
        >
          {[
            ["source-connection", "Connection"],
            ["http-api-operations", "API operations"],
            ["source-query-policy", "Query access and limits"],
          ].map(([id, label]) => (
            <Button
              key={id}
              onClick={() => {
                const section = document.getElementById(id);
                section?.scrollIntoView({ block: "start" });
                section?.focus({ preventScroll: true });
              }}
            >
              {t(label)}
            </Button>
          ))}
        </nav>
      )}
      <div ref={errorRef}>
        <ErrorNote error={error} />
      </div>
      {savedSource && <SourceChangeImpact saved={savedSource} form={form} />}
      <form
        id="source-form"
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        <h3 id="source-connection" tabIndex={-1}>
          {t("Connection")}
        </h3>
        <Field label={t("Name")} required>
          <input
            required
            maxLength={120}
            value={form.name}
            onChange={(e) => field("name", e.target.value)}
          />
        </Field>
        <Field label={t("Source type")} required>
          <select
            value={form.kind}
            onChange={(e) => {
              const c = catalog.find((c) => c.kind === e.target.value)!;
              const options: Record<string, string> =
                c.kind === "bigquery"
                  ? { location: "US", maximum_bytes_billed: "1073741824" }
                  : c.kind === "snowflake"
                    ? {
                        schema: "PUBLIC",
                        token_type: "PROGRAMMATIC_ACCESS_TOKEN",
                      }
                    : c.kind === "databricks"
                      ? { schema: "default" }
                      : {};
              setForm((f) => ({
                ...f,
                kind: c.kind,
                port: c.port,
                host:
                  c.kind === "bigquery"
                    ? "bigquery.googleapis.com"
                    : isCloudSQL(c.kind) || isCloudSQL(f.kind)
                      ? ""
                      : f.host,
                tls_mode: isCloudSQL(c.kind) ? "verify" : f.tls_mode,
                version:
                  c.kind === "influxdb"
                    ? "2"
                    : c.kind === "http_api" || isCloudSQL(c.kind)
                      ? "1"
                      : undefined,
                http_api: c.kind === "http_api" ? initialHTTPAPI() : undefined,
                options,
                auth_mode:
                  c.kind === "bigquery"
                    ? "service_account"
                    : isCloudSQL(c.kind) ||
                        ["influxdb", "http_api"].includes(c.kind)
                      ? "token"
                      : "password",
                clear_password: false,
                clear_token: false,
                password: "",
                token: "",
              }));
              setProbe(undefined);
            }}
          >
            {catalog.map((c) => (
              <option key={c.kind} value={c.kind}>
                {c.name}
              </option>
            ))}
          </select>
        </Field>
        {(cloudSQL || form.kind === "redshift") && (
          <p className="notice">
            {t(
              "Preview connector — implemented, but not verified against a real cloud environment. Review the source limitations before enabling Agent access.",
            )}
          </p>
        )}
        {form.kind === "influxdb" ? (
          <Field label={t("InfluxDB version")}>
            <select
              value={form.version}
              onChange={(e) => {
                const version = e.target.value;
                setForm((f) => ({
                  ...f,
                  version,
                  port: [8086, 8181].includes(f.port || 0)
                    ? version === "3"
                      ? 8181
                      : 8086
                    : f.port,
                  auth_mode: version === "1" ? "password" : "token",
                }));
                setProbe(undefined);
              }}
            >
              <option value="1">{t("1.x — InfluxQL")}</option>
              <option value="2">{t("2.x — Flux")}</option>
              <option value="3">{t("3 Core — SQL / InfluxQL")}</option>
            </select>
          </Field>
        ) : null}
        {local ? (
          <Field
            label={t("Database file path")}
            required
            hint={t(
              "The file must already exist in the server database directory shown in Settings.",
            )}
          >
            <input
              required
              placeholder="/databases/analytics.db"
              value={form.path || ""}
              onChange={(e) => field("path", e.target.value)}
            />
          </Field>
        ) : (
          <>
            {httpAPI ? (
              <Field
                label={t("Base URL")}
                required
                hint={t(
                  "Fixed HTTP(S) address. Do not include credentials or query parameters.",
                )}
              >
                <input
                  required
                  type="url"
                  placeholder="https://api.example.com/v1"
                  value={form.http_api?.base_url || ""}
                  onChange={(e) => {
                    const base_url = e.target.value;
                    setForm((f) => ({
                      ...f,
                      http_api: {
                        ...(f.http_api || initialHTTPAPI()),
                        base_url,
                      },
                      tls_mode: base_url.startsWith("http://")
                        ? "disable"
                        : "verify",
                    }));
                    setProbe(undefined);
                  }}
                />
              </Field>
            ) : (
              <div className="field-grid host-port">
                <Field label={t("Host")} required>
                  <input
                    required
                    placeholder={
                      form.kind === "snowflake"
                        ? "org-account.snowflakecomputing.com"
                        : form.kind === "databricks"
                          ? "your-workspace.cloud.databricks.com"
                          : form.kind === "redshift"
                            ? "cluster.region.redshift.amazonaws.com"
                            : "db.internal"
                    }
                    readOnly={form.kind === "bigquery"}
                    value={form.host || ""}
                    onChange={(e) => field("host", e.target.value)}
                  />
                </Field>
                <Field label={t("Port")} required>
                  <input
                    required
                    type="number"
                    min={1}
                    max={65535}
                    readOnly={cloudSQL}
                    value={form.port || ""}
                    onChange={(e) => field("port", Number(e.target.value))}
                  />
                </Field>
              </div>
            )}
            <div className="field-grid">
              {!httpAPI && (
                <Field
                  required={cloudSQL}
                  label={
                    form.kind === "bigquery"
                      ? t("Billing project ID")
                      : form.kind === "databricks"
                        ? t("Catalog")
                        : ["elasticsearch", "opensearch"].includes(form.kind)
                          ? t("Index / index pattern")
                          : ["cassandra", "scylla"].includes(form.kind)
                            ? t("Keyspace")
                            : ["redis", "valkey"].includes(form.kind)
                              ? t("Database index")
                              : t("Database")
                  }
                >
                  <input
                    required={cloudSQL}
                    value={form.database || ""}
                    onChange={(e) => field("database", e.target.value)}
                  />
                </Field>
              )}
              {form.auth_mode === "password" ? (
                <Field label={t("Username")}>
                  <input
                    autoComplete="off"
                    value={form.username || ""}
                    onChange={(e) => field("username", e.target.value)}
                  />
                </Field>
              ) : null}
            </div>
            <Field label={t("Authentication method")}>
              <select
                value={form.auth_mode || "password"}
                onChange={(e) => {
                  setForm((f) => ({
                    ...f,
                    auth_mode: e.target.value as Source["auth_mode"],
                    password: "",
                    token: "",
                    clear_password: false,
                    clear_token: false,
                  }));
                  setProbe(undefined);
                }}
              >
                {!cloudSQL && (
                  <>
                    <option value="none">{t("None")}</option>
                    <option value="password">
                      {t("Username and password")}
                    </option>
                  </>
                )}
                {form.kind === "bigquery" && (
                  <option value="service_account">
                    {t("Google service account JSON")}
                  </option>
                )}
                {[
                  "influxdb",
                  "elasticsearch",
                  "opensearch",
                  "http_api",
                  "snowflake",
                  "databricks",
                  "bigquery",
                ].includes(form.kind) ? (
                  <option value="token">{t("Token")}</option>
                ) : null}
              </select>
            </Field>
            {form.auth_mode !== "none" ? (
              <>
                <Field
                  label={
                    form.auth_mode === "service_account"
                      ? t("Google service account JSON")
                      : form.auth_mode === "token"
                        ? t("Access token")
                        : t("Password")
                  }
                  hint={
                    savedSource
                      ? t(
                          "Leave blank to keep the stored credential for this method. Enter a value to replace it, or select Clear below.",
                        )
                      : t(
                          "Stored encrypted and never returned to query Agents.",
                        )
                  }
                >
                  {form.auth_mode === "service_account" ? (
                    <textarea
                      rows={6}
                      autoComplete="off"
                      spellCheck={false}
                      disabled={form.clear_password}
                      value={form.password || ""}
                      placeholder={
                        savedSource
                          ? t("Leave blank to keep stored service account JSON")
                          : '{"type":"service_account", ...}'
                      }
                      onChange={(e) => {
                        setForm((f) => ({
                          ...f,
                          password: e.target.value,
                          clear_password: false,
                        }));
                        setProbe(undefined);
                      }}
                    />
                  ) : (
                    <input
                      type="password"
                      autoComplete="new-password"
                      disabled={
                        form.auth_mode === "token"
                          ? form.clear_token
                          : form.clear_password
                      }
                      value={
                        (form.auth_mode === "token"
                          ? form.token
                          : form.password) || ""
                      }
                      onChange={(e) => {
                        const token = form.auth_mode === "token";
                        setForm((f) => ({
                          ...f,
                          [token ? "token" : "password"]: e.target.value,
                          [token ? "clear_token" : "clear_password"]: false,
                        }));
                        setProbe(undefined);
                      }}
                    />
                  )}
                </Field>
                {savedSource && (
                  <label className="check-row">
                    <input
                      type="checkbox"
                      checked={
                        !!(form.auth_mode === "token"
                          ? form.clear_token
                          : form.clear_password)
                      }
                      onChange={(e) => {
                        const token = form.auth_mode === "token";
                        setForm((f) => ({
                          ...f,
                          [token ? "clear_token" : "clear_password"]:
                            e.target.checked,
                          [token ? "token" : "password"]: "",
                        }));
                        setProbe(undefined);
                      }}
                    />
                    {t("Clear the stored credential")}
                  </label>
                )}
                <p className="help">
                  {t(
                    "Changing the authentication method removes credentials for the previous method.",
                  )}
                </p>
              </>
            ) : source?.has_secret || source?.username ? (
              <p className="help">
                {t("Saving removes any stored username, password and token.")}
              </p>
            ) : null}
            {httpAPI && form.auth_mode === "token" && (
              <Field
                label={t("API key header (optional)")}
                hint={t(
                  "Leave blank for Authorization: Bearer. For an API key, use an X- header such as X-API-Key.",
                )}
              >
                <input
                  placeholder="X-API-Key"
                  value={form.http_api?.token_header || ""}
                  onChange={(e) =>
                    field("http_api", {
                      ...(form.http_api || initialHTTPAPI()),
                      token_header: e.target.value,
                    })
                  }
                />
              </Field>
            )}
            {cloudSQL && (
              <CloudSQLFields
                source={form}
                option={option}
                version={(value) => field("version", value)}
              />
            )}
            <Field label={t("TLS mode")} required>
              <select
                value={form.tls_mode}
                disabled={cloudSQL}
                onChange={(e) => field("tls_mode", e.target.value)}
              >
                <option value="verify">{t("Verify certificate")}</option>
                <option value="disable">
                  {t("Disable TLS (unencrypted)")}
                </option>
              </select>
            </Field>
            <details className="advanced">
              <summary>{t("Advanced connection options")}</summary>
              {form.tls_mode === "verify" ? (
                <Field label={t("Custom CA certificate (PEM)")}>
                  <textarea
                    rows={4}
                    value={form.ca_cert || ""}
                    onChange={(e) => field("ca_cert", e.target.value)}
                  />
                </Field>
              ) : null}
              {form.kind === "mongodb" ? (
                <>
                  <Field label={t("Authentication database (authSource)")}>
                    <input
                      value={form.options?.auth_source || ""}
                      onChange={(e) => option("auth_source", e.target.value)}
                    />
                  </Field>
                  <Field label={t("Replica set name")}>
                    <input
                      value={form.options?.replica_set || ""}
                      onChange={(e) => option("replica_set", e.target.value)}
                    />
                  </Field>
                </>
              ) : null}
              {["redis", "valkey"].includes(form.kind) ? (
                <Field label={t("Sentinel master (optional)")}>
                  <input
                    value={form.options?.sentinel_master || ""}
                    onChange={(e) => option("sentinel_master", e.target.value)}
                  />
                </Field>
              ) : null}
            </details>
          </>
        )}
        {httpAPI && (
          <>
            <Field
              label={t("API contract version")}
              required
              hint={t(
                "Changing the contract version expires template verification.",
              )}
            >
              <input
                required
                maxLength={120}
                value={form.version || ""}
                onChange={(e) => field("version", e.target.value)}
              />
            </Field>
            <HTTPAPIEditor
              value={form.http_api || initialHTTPAPI()}
              onChange={(value) => field("http_api", value)}
            />
          </>
        )}
        {form.kind === "influxdb" && form.version === "2" ? (
          <div className="field-grid">
            <Field label={t("Organization (org)")} required>
              <input
                required
                value={form.options?.org || ""}
                onChange={(e) => option("org", e.target.value)}
              />
            </Field>
            <Field label={t("Bucket")} required>
              <input
                required
                value={form.options?.bucket || ""}
                onChange={(e) => option("bucket", e.target.value)}
              />
            </Field>
          </div>
        ) : null}
        <section className="form-section">
          <h3>{t("Read-only verification")}</h3>
          <Protection probe={probe} detail />
          {!probe ? (
            <p className="help">
              {t(
                "Connection checks inspect available read-only protection without attempting any writes.",
              )}
            </p>
          ) : null}
        </section>
        <section
          className="form-section"
          id="source-query-policy"
          tabIndex={-1}
        >
          <h3>{t("Query access")}</h3>
          <Field
            label={t("Agent query access")}
            hint={t(
              "Templates only allows Agents to run published, verified templates. Structure discovery remains available.",
            )}
          >
            <select
              value={form.query_access_mode || "native_and_templates"}
              onChange={(e) =>
                field(
                  "query_access_mode",
                  e.target.value as Source["query_access_mode"],
                )
              }
            >
              <option value="native_and_templates">
                {t("Native queries and templates")}
              </option>
              <option value="templates_only">{t("Templates only")}</option>
            </select>
          </Field>
          <h3>{t("Query limits")}</h3>
          <div className="field-grid">
            <Field label={t("Timeout")} required>
              <div className="unit-input">
                <input
                  type="number"
                  required
                  min={1}
                  max={120}
                  value={form.limits.timeout_seconds}
                  onChange={(e) =>
                    field("limits", {
                      ...form.limits,
                      timeout_seconds: Number(e.target.value),
                    })
                  }
                />
                <span>{t("seconds")}</span>
              </div>
            </Field>
            <Field label={t("Maximum rows")} required>
              <input
                type="number"
                required
                min={1}
                max={10000}
                value={form.limits.max_rows}
                onChange={(e) =>
                  field("limits", {
                    ...form.limits,
                    max_rows: Number(e.target.value),
                  })
                }
              />
            </Field>
          </div>
          <details className="advanced">
            <summary>{t("Response size and concurrency")}</summary>
            <div className="field-grid">
              <Field label={t("Response limit (MiB)")}>
                <input
                  type="number"
                  min={1}
                  max={20}
                  value={form.limits.max_bytes / 1024 / 1024}
                  onChange={(e) =>
                    field("limits", {
                      ...form.limits,
                      max_bytes: Number(e.target.value) * 1024 * 1024,
                    })
                  }
                />
              </Field>
              <Field label={t("Data source concurrency")}>
                <input
                  type="number"
                  min={1}
                  max={16}
                  value={form.limits.concurrency}
                  onChange={(e) =>
                    field("limits", {
                      ...form.limits,
                      concurrency: Number(e.target.value),
                    })
                  }
                />
              </Field>
            </div>
          </details>
        </section>
        <label className="check-row">
          <input
            type="checkbox"
            checked={form.enabled}
            onChange={(e) => field("enabled", e.target.checked)}
          />
          {t("Enable this data source for authorized Agents")}
        </label>
      </form>
    </Drawer>
  );
}
