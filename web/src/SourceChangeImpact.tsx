import { useReadiness } from "./readiness";
import { t } from "./i18n";
import type { Source } from "./types";

export function SourceChangeImpact({
  saved,
  form,
}: {
  saved: Source;
  form: Source;
}) {
  const keys = [
    "kind",
    "version",
    "http_api",
    "host",
    "port",
    "database",
    "username",
    "path",
    "tls_mode",
    "ca_cert",
    "options",
    "auth_mode",
  ] as const;
  const connectionChanged =
    keys.some(
      (key) => JSON.stringify(saved[key]) !== JSON.stringify(form[key]),
    ) ||
    !!(form.password || form.token || form.clear_password || form.clear_token);
  const accessChanged =
    saved.enabled !== form.enabled ||
    saved.query_access_mode !== form.query_access_mode;
  const { data, error } = useReadiness(saved.id, saved.revision);
  if (!connectionChanged && !accessChanged) return null;
  return (
    <div className="source-change-impact" role="status">
      <strong>{t("Saving changes affects live access")}</strong>
      <p>
        {connectionChanged
          ? t(
              "Connection or credential changes cancel active source queries and expire template validation. Trial and publish affected templates again to resume them.",
            )
          : t(
              "Enablement and query access mode apply immediately to subsequent Agent calls and may cancel affected queries.",
            )}
      </p>
      <p>
        {data
          ? t(
              "This source has {templates} published templates and {agents} active authorized Agents.",
              {
                templates: data.templates.length,
                agents: data.active_agents.length,
              },
            )
          : error
            ? t(
                "Impact counts are unavailable. Review the source workspace before saving.",
              )
            : t("Loading current source impact…")}
      </p>
      <small>
        {t(
          "Names, descriptions and query limits do not expire template validation. Queries still follow the latest limits.",
        )}
      </small>
    </div>
  );
}
