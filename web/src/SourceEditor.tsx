import { useState } from "react";
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
}: {
  source: Source | null;
  catalog: Capability[];
  onClose: () => void;
  onSaved: (notice: string, close?: boolean) => Promise<void>;
}) {
  const [form, setForm] = useState<Source>(() =>
    source ? structuredClone(source) : initial(),
  );
  const [savedSource, setSavedSource] = useState(source);
  const [probe, setProbe] = useState(source?.probe);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
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
          `Configuration saved, but the connection check failed. ${message(e)}`,
        );
        await onSaved("", false);
        return;
      }
      setProbe(checked);
      await onSaved("Configuration saved and connection check passed.");
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy("");
    }
  }
  return (
    <Drawer
      title={savedSource ? "Configure data source" : "Add data source"}
      subtitle="Configure the connection, credentials and query limits"
      onClose={onClose}
      footer={
        <>
          <Button busy={busy === "test"} disabled={!!busy} onClick={test}>
            Test connection
          </Button>
          <Button
            primary
            busy={busy === "save"}
            disabled={!!busy}
            form="source-form"
            type="submit"
          >
            Save configuration
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      <form
        id="source-form"
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        <Field label="Name" required>
          <input
            required
            maxLength={120}
            value={form.name}
            onChange={(e) => field("name", e.target.value)}
          />
        </Field>
        <Field label="Database type" required>
          <select
            value={form.kind}
            onChange={(e) => {
              const c = catalog.find((c) => c.kind === e.target.value)!;
              setForm((f) => ({
                ...f,
                kind: c.kind,
                port: c.port,
                version: c.kind === "influxdb" ? "2" : undefined,
                options: {},
                auth_mode: c.kind === "influxdb" ? "token" : "password",
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
        {form.kind === "influxdb" ? (
          <Field label="InfluxDB version">
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
              <option value="1">1.x — InfluxQL</option>
              <option value="2">2.x — Flux</option>
              <option value="3">3 Core — SQL / InfluxQL</option>
            </select>
          </Field>
        ) : null}
        {local ? (
          <Field
            label="Database file path"
            required
            hint="The file must already exist in the server database directory shown in Settings."
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
            <div className="field-grid host-port">
              <Field label="Host" required>
                <input
                  required
                  placeholder="db.internal"
                  value={form.host || ""}
                  onChange={(e) => field("host", e.target.value)}
                />
              </Field>
              <Field label="Port" required>
                <input
                  required
                  type="number"
                  min={1}
                  max={65535}
                  value={form.port || ""}
                  onChange={(e) => field("port", Number(e.target.value))}
                />
              </Field>
            </div>
            <div className="field-grid">
              <Field
                label={
                  ["elasticsearch", "opensearch"].includes(form.kind)
                    ? "Index / index pattern"
                    : ["cassandra", "scylla"].includes(form.kind)
                      ? "Keyspace"
                      : ["redis", "valkey"].includes(form.kind)
                        ? "Database index"
                        : "Database"
                }
              >
                <input
                  value={form.database || ""}
                  onChange={(e) => field("database", e.target.value)}
                />
              </Field>
              {form.auth_mode === "password" ? (
                <Field label="Username">
                  <input
                    autoComplete="off"
                    value={form.username || ""}
                    onChange={(e) => field("username", e.target.value)}
                  />
                </Field>
              ) : null}
            </div>
            <Field label="Authentication method">
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
                <option value="none">None</option>
                <option value="password">Username and password</option>
                {["influxdb", "elasticsearch", "opensearch"].includes(
                  form.kind,
                ) ? (
                  <option value="token">Token</option>
                ) : null}
              </select>
            </Field>
            {form.auth_mode !== "none" ? (
              <>
                <Field
                  label={
                    form.auth_mode === "token" ? "Access token" : "Password"
                  }
                  hint="Leave blank to keep the stored credential for this method. Enter a value to replace it, or select Clear below."
                >
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
                </Field>
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
                  Clear the stored credential
                </label>
                <p className="help">
                  Changing the authentication method removes credentials for the
                  previous method.
                </p>
              </>
            ) : (
              <p className="help">
                Saving removes any stored username, password and token.
              </p>
            )}
            <Field label="TLS mode" required>
              <select
                value={form.tls_mode}
                onChange={(e) => field("tls_mode", e.target.value)}
              >
                <option value="verify">Verify certificate</option>
                <option value="disable">Disable TLS (unencrypted)</option>
              </select>
            </Field>
            <details className="advanced">
              <summary>Advanced connection options</summary>
              {form.tls_mode === "verify" ? (
                <Field label="Custom CA certificate (PEM)">
                  <textarea
                    rows={4}
                    value={form.ca_cert || ""}
                    onChange={(e) => field("ca_cert", e.target.value)}
                  />
                </Field>
              ) : null}
              {form.kind === "mongodb" ? (
                <>
                  <Field label="Authentication database (authSource)">
                    <input
                      value={form.options?.auth_source || ""}
                      onChange={(e) => option("auth_source", e.target.value)}
                    />
                  </Field>
                  <Field label="Replica set name">
                    <input
                      value={form.options?.replica_set || ""}
                      onChange={(e) => option("replica_set", e.target.value)}
                    />
                  </Field>
                </>
              ) : null}
              {["redis", "valkey"].includes(form.kind) ? (
                <Field label="Sentinel master (optional)">
                  <input
                    value={form.options?.sentinel_master || ""}
                    onChange={(e) => option("sentinel_master", e.target.value)}
                  />
                </Field>
              ) : null}
            </details>
          </>
        )}
        {form.kind === "influxdb" && form.version === "2" ? (
          <div className="field-grid">
            <Field label="Organization (org)" required>
              <input
                required
                value={form.options?.org || ""}
                onChange={(e) => option("org", e.target.value)}
              />
            </Field>
            <Field label="Bucket" required>
              <input
                required
                value={form.options?.bucket || ""}
                onChange={(e) => option("bucket", e.target.value)}
              />
            </Field>
          </div>
        ) : null}
        <section className="form-section">
          <h3>Read-only verification</h3>
          <Protection probe={probe} detail />
          {!probe ? (
            <p className="help">
              Connection checks inspect available read-only protection without
              attempting any writes.
            </p>
          ) : null}
        </section>
        <section className="form-section">
          <h3>Query limits</h3>
          <div className="field-grid">
            <Field label="Timeout" required>
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
                <span>seconds</span>
              </div>
            </Field>
            <Field label="Maximum rows" required>
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
            <summary>Response size and concurrency</summary>
            <div className="field-grid">
              <Field label="Response limit (MiB)">
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
              <Field label="Data source concurrency">
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
          Enable this data source for authorized Agents
        </label>
      </form>
    </Drawer>
  );
}
