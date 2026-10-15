import { useEffect, useState } from "react";
import { t } from "./i18n";
import { Brand } from "./Brand";
import { api, message, payload } from "./api";
import { Button, ErrorNote, Loading } from "./components";
import type { Source } from "./types";

interface Consent {
  client_name: string;
  client_id: string;
  redirect_uri: string;
  scope: string;
  sources: Pick<Source, "id" | "name" | "kind">[];
}
export function ConsentPage({ notify }: { notify: (s: string) => void }) {
  const [info, setInfo] = useState<Consent | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const request = new URLSearchParams(location.search).get("request") || "";
  useEffect(() => {
    api<Consent>(`/api/oauth/consent?request=${encodeURIComponent(request)}`)
      .then(setInfo)
      .catch((e) => setError(message(e)));
  }, [request]);
  async function consent(allow: boolean) {
    setBusy(true);
    setError("");
    try {
      const out = await api<{ redirect: string }>("/api/oauth/consent", {
        method: "POST",
        body: payload({ request, allow, sources: selected }),
      });
      notify(allow ? t("Access granted") : t("Access denied"));
      location.assign(out.redirect);
    } catch (e) {
      setError(message(e));
      setBusy(false);
    }
  }
  return (
    <div className="auth">
      <div className="auth-brand">
        <Brand tagline />
      </div>
      <section className="auth-panel consent-panel">
        <h1>{t("Authorize read-only database access")}</h1>
        <ErrorNote error={error} />
        {!info && error ? (
          <a href="/sources">{t("Return to administration")}</a>
        ) : null}
        {!info && !error ? (
          <Loading />
        ) : info ? (
          <>
            <p>
              <strong>{info.client_name}</strong>
              {t(" requests database access through MCP.")}
            </p>
            <dl className="consent-details">
              <dt>Client ID</dt>
              <dd>{info.client_id}</dd>
              <dt>{t("Return URL")}</dt>
              <dd>{info.redirect_uri}</dd>
            </dl>
            <h3>{t("Allowed data sources")}</h3>
            <div className="source-checklist">
              {info.sources.length ? (
                info.sources.map((s) => (
                  <label key={s.id} className="check-row">
                    <input
                      type="checkbox"
                      disabled={busy}
                      checked={selected.includes(s.id)}
                      onChange={(e) =>
                        setSelected(
                          e.target.checked
                            ? [...selected, s.id]
                            : selected.filter((id) => id !== s.id),
                        )
                      }
                    />
                    <span>
                      {s.name}
                      <small>{s.kind}</small>
                    </span>
                  </label>
                ))
              ) : (
                <p className="help">
                  {t(
                    "No data sources are enabled. Add and enable a connection in administration first.",
                  )}
                </p>
              )}
            </div>
            <p className="help">
              {t(
                "Only the selected data sources are shared. Revoke access at any time in Agents.",
              )}
            </p>
            <div className="consent-actions">
              <Button disabled={busy} onClick={() => consent(false)}>
                {t("Deny")}
              </Button>
              <Button
                primary
                busy={busy}
                disabled={!selected.length}
                onClick={() => consent(true)}
              >
                {t("Allow read-only access")}
              </Button>
            </div>
          </>
        ) : null}
      </section>
    </div>
  );
}
