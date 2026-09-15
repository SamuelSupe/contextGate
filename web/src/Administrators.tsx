import { useEffect, useState } from "react";
import { api, date, message, payload, setCSRF } from "./api";
import {
  Button,
  CopyButton,
  Drawer,
  ErrorNote,
  Field,
  Loading,
} from "./components";
import { Brand } from "./Brand";
import { t } from "./i18n";
import { useNavigationGuard } from "./useNavigationGuard";
import type { Administrator, ConfigurationIdentity, Session } from "./types";

export function administratorRole(role: string) {
  return role === "super_admin" ? t("Super administrator") : t("Administrator");
}

export function RequiredPasswordChange({
  administrator,
  onComplete,
}: {
  administrator: Administrator;
  onComplete: (s: Session) => void;
}) {
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <div className="auth">
      <div className="auth-brand">
        <Brand />
      </div>
      <section className="auth-panel">
        <h1>{t("Set your personal password")}</h1>
        <p>
          {t(
            "Welcome, {name}. Replace your temporary password before managing configuration.",
            { name: administrator.display_name },
          )}
        </p>
        <ErrorNote error={error} />
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setError("");
            if (password !== confirmation) {
              setError(t("Passwords do not match"));
              return;
            }
            setBusy(true);
            try {
              const s = await api<Session>("/api/password", {
                method: "POST",
                body: payload({ current_password: current, password }),
              });
              setCSRF(s.csrf);
              onComplete(s);
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          <input
            className="sr-only"
            name="username"
            autoComplete="username"
            value={administrator.username}
            readOnly
            tabIndex={-1}
          />
          <Field label={t("Temporary password")} required>
            <input
              type="password"
              autoComplete="current-password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              required
              disabled={busy}
            />
          </Field>
          <Field
            label={t("New password")}
            required
            hint={t("At least 12 characters")}
          >
            <input
              type="password"
              autoComplete="new-password"
              minLength={12}
              maxLength={256}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              disabled={busy}
            />
          </Field>
          <Field label={t("Confirm new password")} required>
            <input
              type="password"
              autoComplete="new-password"
              value={confirmation}
              onChange={(e) => setConfirmation(e.target.value)}
              required
              disabled={busy}
            />
          </Field>
          <Button primary busy={busy} type="submit">
            {t("Save password and continue")}
          </Button>
          <Button
            disabled={busy}
            onClick={async () => {
              try {
                await api("/api/logout", { method: "POST" });
                location.reload();
              } catch (e) {
                setError(message(e));
              }
            }}
          >
            {t("Sign out")}
          </Button>
        </form>
      </section>
    </div>
  );
}

type AdministratorRow = Administrator & {
  configuration?: ConfigurationIdentity;
};
export function Administrators({
  current,
  notify,
}: {
  current: Administrator;
  notify: (text: string) => void;
}) {
  const [rows, setRows] = useState<AdministratorRow[] | null>(null);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [edit, setEdit] = useState<AdministratorRow | "new" | null>(null);
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [role, setRole] = useState("admin");
  const [enabled, setEnabled] = useState(true);
  const [busy, setBusy] = useState(false);
  const [panelError, setPanelError] = useState("");
  const [secret, setSecret] = useState<{
    username: string;
    password: string;
    expires: string;
  } | null>(null);
  const [saved, setSaved] = useState(false);
  const [confirm, setConfirm] = useState<{
    row: AdministratorRow;
    action: "reset" | "revoke";
  } | null>(null);
  useNavigationGuard(!!secret && !saved, busy);
  async function load() {
    try {
      setRows(
        (
          await api<{ administrators: AdministratorRow[] }>(
            "/api/administrators",
          )
        ).administrators,
      );
      setError("");
    } catch (e) {
      setError(message(e));
    }
  }
  useEffect(() => {
    void load();
  }, []);
  function open(row: AdministratorRow | "new") {
    setEdit(row);
    setPanelError("");
    setUsername(row === "new" ? "" : row.username);
    setDisplayName(row === "new" ? "" : row.display_name);
    setRole(row === "new" ? "admin" : row.role);
    setEnabled(row === "new" ? true : row.enabled);
  }
  function close() {
    if (busy) return;
    if (secret && !saved) {
      setPanelError(t("Save the temporary password before closing."));
      return;
    }
    setEdit(null);
    setSecret(null);
    setConfirm(null);
    setPanelError("");
  }
  function showSecret(a: Administrator, password: string) {
    setSecret({
      username: a.username,
      password,
      expires: a.temporary_expires_at!,
    });
    setSaved(false);
    setEdit(null);
    setConfirm(null);
  }
  const shown = rows?.filter((a) =>
    (a.username + " " + a.display_name)
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  return (
    <section id="settings-administrators" tabIndex={-1}>
      <div className="configuration-heading">
        <div>
          <h2>{t("Administrators")}</h2>
          <p className="help">
            {t(
              "Individual accounts, shared business configuration and attributable changes.",
            )}
          </p>
        </div>
        <Button primary onClick={() => open("new")}>
          {t("Add administrator")}
        </Button>
      </div>
      <ErrorNote error={error} />
      {error && <Button onClick={load}>{t("Retry")}</Button>}
      <Field label={t("Search administrators")}>
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </Field>
      {!rows && !error ? <Loading /> : null}
      {rows && (
        <div className="administrator-list">
          {shown?.map((a) => (
            <article className="administrator-row" key={a.id}>
              <div className="administrator-person">
                <strong>
                  {a.display_name}
                  {a.id === current.id ? ` · ${t("You")}` : ""}
                </strong>
                <span className="help">
                  {a.username} · {administratorRole(a.role)}
                </span>
                <span className="help">
                  {t("Last sign in")}:{" "}
                  {a.last_login_at ? date(a.last_login_at) : t("Never")}
                </span>
              </div>
              <div>
                <span className={`status ${a.enabled ? "green" : "muted"}`}>
                  {!a.enabled
                    ? t("Disabled")
                    : a.must_change_password
                      ? t("Password change required")
                      : t("Active")}
                </span>
                <small className="block help">
                  {t("Configuration MCP")}:{" "}
                  {a.configuration &&
                  !a.configuration.revoked_at &&
                  Date.parse(a.configuration.expires_at) > Date.now()
                    ? t("Active")
                    : t("No active token")}
                </small>
              </div>
              <div className="administrator-actions">
                <Button onClick={() => open(a)}>{t("Edit")}</Button>
                {a.id !== current.id && (
                  <Button
                    onClick={() => {
                      setConfirm({ row: a, action: "reset" });
                      setPanelError("");
                    }}
                  >
                    {t("Reset password")}
                  </Button>
                )}
                {a.configuration &&
                  !a.configuration.revoked_at &&
                  Date.parse(a.configuration.expires_at) > Date.now() && (
                    <Button
                      onClick={() => {
                        setConfirm({ row: a, action: "revoke" });
                        setPanelError("");
                      }}
                    >
                      {t("Revoke MCP token")}
                    </Button>
                  )}
              </div>
            </article>
          ))}
          {!shown?.length && (
            <p className="help">{t("No matching administrators")}</p>
          )}
        </div>
      )}
      {edit && (
        <Drawer
          title={
            edit === "new" ? t("Add administrator") : t("Edit administrator")
          }
          onClose={close}
          footer={
            <>
              <Button disabled={busy} onClick={close}>
                {t("Cancel")}
              </Button>
              <Button
                primary
                busy={busy}
                type="submit"
                form="administrator-edit"
              >
                {t("Save")}
              </Button>
            </>
          }
        >
          <ErrorNote error={panelError} />
          <form
            id="administrator-edit"
            onSubmit={async (e) => {
              e.preventDefault();
              setBusy(true);
              setPanelError("");
              try {
                if (edit === "new") {
                  const result = await api<{
                    administrator: Administrator;
                    temporary_password: string;
                  }>("/api/administrators", {
                    method: "POST",
                    body: payload({
                      username,
                      display_name: displayName,
                      role,
                    }),
                  });
                  showSecret(result.administrator, result.temporary_password);
                } else {
                  await api(`/api/administrators/${edit.id}`, {
                    method: "PUT",
                    body: payload({
                      display_name: displayName,
                      role,
                      enabled,
                      revision: edit.revision,
                    }),
                  });
                  setEdit(null);
                  notify(t("Administrator updated"));
                }
                await load();
              } catch (e) {
                setPanelError(message(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            <Field
              label={t("Username")}
              required
              hint={t(
                "3–64 letters, numbers, dots, underscores or hyphens. Cannot be changed later.",
              )}
            >
              <input
                autoComplete="off"
                value={username}
                required
                minLength={3}
                maxLength={64}
                disabled={edit !== "new" || busy}
                onChange={(e) => setUsername(e.target.value)}
              />
            </Field>
            <Field label={t("Display name")} required>
              <input
                value={displayName}
                required
                maxLength={120}
                disabled={busy}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </Field>
            <Field label={t("Role")}>
              <select
                value={role}
                disabled={busy || (edit !== "new" && edit.id === current.id)}
                onChange={(e) => setRole(e.target.value)}
              >
                <option value="admin">{t("Administrator")}</option>
                <option value="super_admin">{t("Super administrator")}</option>
              </select>
            </Field>
            {edit !== "new" && (
              <label className="check">
                <input
                  type="checkbox"
                  checked={enabled}
                  disabled={busy || edit.id === current.id}
                  onChange={(e) => setEnabled(e.target.checked)}
                />
                {t("Account enabled")}
              </label>
            )}
            <p className="help">
              {edit === "new"
                ? t(
                    "A temporary password is shown once and expires in 24 hours. The administrator must replace it before using the gateway.",
                  )
                : t(
                    "Changing a role or disabling an account signs it out, revokes its configuration token and cancels its running work. Query Agent grants stay valid.",
                  )}
            </p>
          </form>
        </Drawer>
      )}
      {confirm && (
        <Drawer
          title={
            confirm.action === "reset"
              ? t("Reset administrator password")
              : t("Revoke MCP token")
          }
          onClose={close}
          footer={
            <>
              <Button disabled={busy} onClick={close}>
                {t("Cancel")}
              </Button>
              <Button
                primary
                busy={busy}
                onClick={async () => {
                  setBusy(true);
                  setPanelError("");
                  try {
                    if (confirm.action === "reset") {
                      const result = await api<{
                        administrator: Administrator;
                        temporary_password: string;
                      }>(
                        `/api/administrators/${confirm.row.id}/reset-password`,
                        {
                          method: "POST",
                          body: payload({ revision: confirm.row.revision }),
                        },
                      );
                      showSecret(
                        result.administrator,
                        result.temporary_password,
                      );
                    } else {
                      await api(
                        `/api/configuration-agents/${confirm.row.configuration!.id}`,
                        { method: "DELETE" },
                      );
                      setConfirm(null);
                      notify(t("Configuration access revoked"));
                    }
                    await load();
                  } catch (e) {
                    setPanelError(message(e));
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {t("Confirm")}
              </Button>
            </>
          }
        >
          <ErrorNote error={panelError} />
          <p>
            <strong>{confirm.row.display_name}</strong> · {confirm.row.username}
          </p>
          <p>
            {confirm.action === "reset"
              ? t(
                  "This signs out the account, revokes its configuration token and issues a new temporary password.",
                )
              : t(
                  "The current configuration token stops working immediately. The owner must issue a new token.",
                )}
          </p>
        </Drawer>
      )}
      {secret && (
        <Drawer
          title={t("Save the temporary password")}
          onClose={close}
          footer={
            <Button primary disabled={!saved} onClick={close}>
              {t("Done")}
            </Button>
          }
        >
          <ErrorNote error={panelError} />
          <p>
            {t(
              "Share these credentials directly with the administrator. This password cannot be displayed again.",
            )}
          </p>
          <Field label={t("Username")}>
            <code>{secret.username}</code>
            <CopyButton text={secret.username} />
          </Field>
          <Field label={t("Temporary password")}>
            <code className="token-secret">{secret.password}</code>
            <CopyButton text={secret.password} />
          </Field>
          <p className="help">
            {t("Expires")}: {date(secret.expires)}
          </p>
          <label className="check">
            <input
              type="checkbox"
              checked={saved}
              onChange={(e) => setSaved(e.target.checked)}
            />
            {t("I have saved the temporary password")}
          </label>
        </Drawer>
      )}
    </section>
  );
}
