import { t } from "./i18n";
import { useCallback, useEffect, useState } from "react";
import { api, date, message, payload } from "./api";
import {
  Button,
  CopyButton,
  Drawer,
  Empty,
  ErrorNote,
  Loading,
} from "./components";
import { OAuthClientForm } from "./OAuthClientForm";
import type { OAuthClientConfig, Settings } from "./types";

type Action = {
  client: OAuthClientConfig;
  kind: "toggle" | "rotate" | "delete";
};
export function OAuthClient({
  settings,
  onClose,
  notify,
  reloadAgents,
}: {
  settings: Settings | null;
  onClose: () => void;
  notify: (text: string) => void;
  reloadAgents: () => Promise<void>;
}) {
  const [clients, setClients] = useState<OAuthClientConfig[] | null>(null);
  const [editing, setEditing] = useState<
    OAuthClientConfig | null | undefined
  >();
  const [credential, setCredential] = useState<Record<string, unknown> | null>(
    null,
  );
  const [action, setAction] = useState<Action | null>(null);
  const [search, setSearch] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const refresh = useCallback(async () => {
    setError("");
    try {
      setClients(await api<OAuthClientConfig[]>("/api/oauth/clients"));
    } catch (e) {
      setError(message(e));
    }
  }, []);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  async function saved(result: Record<string, unknown>) {
    setEditing(undefined);
    if (result.client_secret || result.client_id)
      setCredential(
        result.client
          ? {
              ...(result.client as Record<string, unknown>),
              client_secret: result.client_secret,
            }
          : result,
      );
    else notify(t("OAuth client updated"));
    await refresh();
    try {
      await reloadAgents();
    } catch (e) {
      setError(message(e));
    }
  }
  function back() {
    setEditing(undefined);
    setAction(null);
    void refresh();
  }
  const shown =
    clients?.filter((c) =>
      `${c.client_name} ${c.client_id}`
        .toLowerCase()
        .includes(search.toLowerCase()),
    ) || [];
  const actionLabel = t(
    action?.kind === "delete"
      ? "Delete client"
      : action?.kind === "rotate"
        ? "Rotate client secret"
        : action?.client.enabled
          ? "Disable client"
          : "Enable client",
  );
  return (
    <Drawer
      title={t("OAuth clients")}
      subtitle={t("Manage registered clients and their credentials")}
      wide
      onClose={() => {
        if (busy) return;
        if (credential?.client_secret) setConfirmClose(true);
        else onClose();
      }}
    >
      <div className="connection-strip">
        <code>{settings?.mcp_url}</code>
        <CopyButton text={settings?.mcp_url || ""} />
      </div>
      <ErrorNote error={error} />
      {credential ? (
        <section className="form-section">
          <h3>{t("Client configuration")}</h3>
          <p className="notice success">
            {credential.client_secret
              ? t("Save this client secret now. It is shown only once.")
              : t(
                  "Client registered. Copy the configuration to your MCP client.",
                )}
          </p>
          <pre>{JSON.stringify(credential, null, 2)}</pre>
          <div className="button-row">
            <CopyButton
              text={JSON.stringify(credential, null, 2)}
              onCopied={() => notify(t("Client configuration copied"))}
            />
            <Button
              primary
              onClick={() => {
                setCredential(null);
                setConfirmClose(false);
              }}
            >
              {t("I saved the configuration")}
            </Button>
          </div>
          {confirmClose ? (
            <div className="notice warning">
              <p>
                {t("The secret cannot be shown again. Save it before closing.")}
              </p>
              <Button
                onClick={() => {
                  setCredential(null);
                  onClose();
                }}
              >
                {t("Close without saving")}
              </Button>
            </div>
          ) : null}
        </section>
      ) : action ? (
        <section className="form-section">
          <h3>{actionLabel}</h3>
          <strong>{action.client.client_name}</strong>
          <p>
            {action.kind === "toggle" && !action.client.enabled
              ? t(
                  "Allow new authorizations. Previously revoked credentials remain invalid; clients must authorize again.",
                )
              : t(
                  "Existing access tokens, refresh tokens and pending authorizations will be revoked. Running queries will be cancelled; Agent identities and audit history are retained.",
                )}
          </p>
          {action.kind === "rotate" ? (
            <p>
              {t(
                "The old secret stops working immediately. Save the replacement secret and update your client.",
              )}
            </p>
          ) : null}
          {action.kind === "delete" ? (
            <p>
              {t(
                "Registration capacity is released. A client can register again and request new consent. Use Disable to keep a metadata document client blocked.",
              )}
            </p>
          ) : null}
          <div className="button-row">
            <Button disabled={busy} onClick={back}>
              {t("Cancel")}
            </Button>
            <Button
              primary
              busy={busy}
              onClick={async () => {
                setBusy(true);
                setError("");
                try {
                  const c = action.client;
                  const path = `/api/oauth/clients/${encodeURIComponent(c.client_id)}`;
                  const result = await api<Record<string, unknown>>(
                    path + (action.kind === "rotate" ? "/secret" : ""),
                    {
                      method:
                        action.kind === "delete"
                          ? "DELETE"
                          : action.kind === "rotate"
                            ? "POST"
                            : "PUT",
                      body: payload(
                        action.kind === "toggle"
                          ? {
                              revision: c.revision,
                              client_name: c.client_name,
                              redirect_uris: c.redirect_uris,
                              enabled: !c.enabled,
                            }
                          : { revision: c.revision },
                      ),
                    },
                  );
                  setAction(null);
                  await saved(result);
                  notify(
                    t("{actionLabel} completed", { actionLabel: actionLabel }),
                  );
                } catch (e) {
                  setError(message(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              {actionLabel}
            </Button>
          </div>
        </section>
      ) : editing !== undefined ? (
        <OAuthClientForm
          key={editing?.client_id || "new"}
          client={editing}
          busy={busy}
          setBusy={setBusy}
          saved={saved}
          cancel={back}
        />
      ) : (
        <>
          <div className="filters">
            <input
              aria-label={t("Search OAuth clients")}
              placeholder={t("Search OAuth clients")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <Button onClick={() => void refresh()}>
              {t("Refresh clients")}
            </Button>
            <Button
              primary
              onClick={() => {
                setError("");
                setEditing(null);
              }}
            >
              {t("Register client")}
            </Button>
          </div>
          {clients === null ? (
            error ? null : (
              <Loading />
            )
          ) : !shown.length ? (
            <Empty
              title={
                clients?.length
                  ? t("No matching clients")
                  : t("No OAuth clients")
              }
              description={t(
                "Register a client or connect an MCP client that supports OAuth discovery.",
              )}
            />
          ) : (
            <div className="table-scroll">
              <table className="oauth-clients-table">
                <thead>
                  <tr>
                    <th>{t("Client")}</th>
                    <th>{t("Authentication")}</th>
                    <th>{t("Status")}</th>
                    <th>{t("Actions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {shown.map((c) => (
                    <tr key={c.client_id}>
                      <td>
                        <strong>{c.client_name}</strong>
                        <code className="block client-id">{c.client_id}</code>
                        <CopyButton text={c.client_id} />
                        <small className="block">
                          {c.metadata_document
                            ? t("Metadata document")
                            : t("Registered client")}{" "}
                          ·{" "}
                          {c.created_at.startsWith("0001-")
                            ? t("Registration date unavailable")
                            : date(c.created_at)}
                        </small>
                      </td>
                      <td>
                        {c.token_endpoint_auth_method === "none"
                          ? t("Public · PKCE")
                          : c.token_endpoint_auth_method ===
                              "client_secret_basic"
                            ? t("Secret · Basic")
                            : t("Secret · POST")}
                      </td>
                      <td>
                        <span
                          className={`status ${c.enabled ? "green" : "muted"}`}
                        >
                          {c.enabled ? t("Enabled") : t("Disabled")}
                        </span>
                      </td>
                      <td>
                        <div className="row-actions wrap">
                          <button
                            className="text-button"
                            onClick={() => {
                              setError("");
                              setEditing(c);
                            }}
                          >
                            {t("Edit client")}
                          </button>
                          <button
                            className="text-button"
                            onClick={() => {
                              setError("");
                              setAction({ client: c, kind: "toggle" });
                            }}
                          >
                            {c.enabled ? t("Disable") : t("Enable")}
                          </button>
                          {c.token_endpoint_auth_method !== "none" ? (
                            <button
                              className="text-button"
                              onClick={() => {
                                setError("");
                                setAction({ client: c, kind: "rotate" });
                              }}
                            >
                              {t("Rotate secret")}
                            </button>
                          ) : null}
                          <button
                            className="text-button danger"
                            onClick={() => {
                              setError("");
                              setAction({ client: c, kind: "delete" });
                            }}
                          >
                            {t("Delete")}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </Drawer>
  );
}
