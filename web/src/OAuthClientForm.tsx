import { t } from "./i18n";
import { useState } from "react";
import { api, message, payload } from "./api";
import { Button, ErrorNote, Field } from "./components";
import type { OAuthClientConfig } from "./types";

export function OAuthClientForm({
  client,
  busy,
  setBusy,
  saved,
  cancel,
}: {
  client: OAuthClientConfig | null;
  busy: boolean;
  setBusy: (busy: boolean) => void;
  saved: (result: Record<string, unknown>) => Promise<void>;
  cancel: () => void;
}) {
  const [name, setName] = useState(client?.client_name || "");
  const [redirect, setRedirect] = useState(
    client?.redirect_uris.join("\n") || "",
  );
  const [method, setMethod] = useState(
    client?.token_endpoint_auth_method || "none",
  );
  const [error, setError] = useState("");
  return (
    <form
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError("");
        try {
          const configuration = {
            client_name: name,
            redirect_uris: redirect
              .split("\n")
              .map((v) => v.trim())
              .filter(Boolean),
            ...(client
              ? { revision: client.revision, enabled: client.enabled }
              : { token_endpoint_auth_method: method }),
          };
          const result = await api<Record<string, unknown>>(
            client
              ? `/api/oauth/clients/${encodeURIComponent(client.client_id)}`
              : "/api/oauth/clients",
            { method: client ? "PUT" : "POST", body: payload(configuration) },
          );
          await saved(result);
        } catch (e) {
          setError(message(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      <h3>{client ? t("Edit client") : t("Register a client")}</h3>
      <ErrorNote error={error} />
      {client ? (
        <p className="help">
          {t(
            "Changing redirect URIs revokes existing OAuth credentials and cancels running queries. Clients must authorize again.",
          )}
        </p>
      ) : null}
      {client?.metadata_document ? (
        <p className="notice neutral">
          {t("Name and redirect URIs come from the client metadata document.")}
        </p>
      ) : null}
      <Field label={t("Client name")} required>
        <input
          autoFocus
          required
          maxLength={120}
          value={name}
          disabled={busy || client?.metadata_document}
          onChange={(e) => setName(e.target.value)}
        />
      </Field>
      <Field
        label={t("Redirect URIs")}
        required
        hint={t(
          "One URI per line. HTTPS, loopback IP HTTP, and reverse-domain native app schemes are supported.",
        )}
      >
        <textarea
          required
          rows={4}
          value={redirect}
          disabled={busy || client?.metadata_document}
          onChange={(e) => setRedirect(e.target.value)}
        />
      </Field>
      <Field label={t("Client authentication")}>
        <select
          value={method}
          disabled={busy || !!client}
          onChange={(e) => setMethod(e.target.value)}
        >
          <option value="none">{t("Public client — PKCE")}</option>
          <option value="client_secret_basic">
            {t("Client secret — Basic")}
          </option>
          <option value="client_secret_post">
            {t("Client secret — POST")}
          </option>
        </select>
      </Field>
      <div className="button-row">
        <Button type="button" disabled={busy} onClick={cancel}>
          {t("Back to clients")}
        </Button>
        <Button primary busy={busy} type="submit">
          {client ? t("Save client") : t("Register client")}
        </Button>
      </div>
    </form>
  );
}
