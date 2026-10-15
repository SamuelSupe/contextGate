import { useState } from "react";
import { t, useLocale, setLocale, validLocale } from "./i18n";
import { api, message, payload } from "./api";
import { Button, ErrorNote, Field } from "./components";
import { Brand } from "./Brand";
import type { Session } from "./types";

export function Login({
  initialized,
  onLogin,
}: {
  initialized: boolean;
  onLogin: (s: Session) => void;
}) {
  const locale = useLocale();
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [username, setUsername] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <div className="auth">
      <div className="auth-language">
        <label>
          {t("Display language")}
          <select
            value={locale}
            onChange={(e) => setLocale(validLocale(e.target.value))}
          >
            <option value="en" lang="en">
              English
            </option>
            <option value="zh-CN" lang="zh-CN">
              简体中文
            </option>
          </select>
        </label>
      </div>
      <div className="auth-brand">
        <Brand tagline />
      </div>
      <p className="auth-tagline">
        {t("Understand your business. Query data safely.")}
      </p>
      <section className="auth-panel">
        <h1>
          {initialized ? t("Administrator sign in") : t("Set up administrator")}
        </h1>
        <p>
          {initialized
            ? t("Manage business context, verified queries and Agent access.")
            : t(
                "Enter the one-time setup code from the server log to create your administrator password.",
              )}
        </p>
        {!initialized && (
          <details className="recovery-help">
            <summary>{t("Where do I find the setup code?")}</summary>
            <p>
              {t(
                "Docker Compose: run this command in the deployment directory, then copy the value after First-time setup token:",
              )}
            </p>
            <pre>docker compose logs hub</pre>
            <p>
              {t(
                "Local binary: look in the terminal where you started contextgate serve. The latest startup code is valid until administrator setup completes. Restart the service to generate a new code if needed.",
              )}
            </p>
          </details>
        )}
        <ErrorNote error={error} />
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setError("");
            try {
              const s = await api<Session>(
                initialized ? "/api/login" : "/api/setup",
                {
                  method: "POST",
                  body: payload(
                    initialized
                      ? { username, password }
                      : { username, password, token },
                  ),
                },
              );
              onLogin(s);
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          <Field label={t("Username")} required>
            <input
              name="username"
              autoComplete="username"
              required
              maxLength={64}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </Field>
          {!initialized ? (
            <Field label={t("One-time setup code")} required>
              <input
                autoComplete="off"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
              />
            </Field>
          ) : null}
          <Field
            label={
              initialized
                ? t("Administrator password")
                : t("Create administrator password")
            }
            required
            hint={
              !initialized
                ? t(
                    "At least 12 characters. Passwords are never stored in plain text.",
                  )
                : undefined
            }
          >
            <input
              type="password"
              autoComplete={initialized ? "current-password" : "new-password"}
              minLength={initialized ? 1 : 12}
              maxLength={256}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </Field>
          <Button primary busy={busy} type="submit">
            {initialized ? t("Sign in") : t("Create administrator")}
          </Button>
        </form>
        {initialized ? (
          <details className="recovery-help">
            <summary>{t("Forgot your password?")}</summary>
            <p>
              {t("On the server, stop the service and run")}{" "}
              <code>
                contextgate reset-password --username USERNAME --data-dir DIR
                --password-stdin
              </code>{" "}
              {t(
                "with the same MCPDBHUB_DATABASE_URL, key directory and master key as the service. Supply the new password through standard input, then restart the service.",
              )}
            </p>
            <p>
              {t(
                "The selected account is signed out and its configuration token is revoked. Other accounts and query Agent grants are retained.",
              )}
            </p>
          </details>
        ) : null}
      </section>
      <p className="auth-foot">
        {t("Read-only queries · Individual grants · Auditable access")}
      </p>
    </div>
  );
}
