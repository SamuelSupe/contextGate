import { useState } from "react";
import { api, date, message } from "./api";
import { Button, CopyButton, ErrorNote } from "./components";
import { t } from "./i18n";

type Report = {
  checked_at: string;
  version: string;
  commit: string;
  storage: string;
  checks: {
    check: string;
    status: string;
    server_version?: string;
    tls?: boolean;
    sources?: number;
    agents?: number;
    semantic_versions?: number;
  }[];
  connection_pool: {
    open: number;
    in_use: number;
    idle: number;
    max: number;
    wait_count: number;
  };
};
const checkNames: Record<string, string> = {
  metadata_database: "PostgreSQL connection",
  encryption_key: "Encryption key matches metadata",
  metadata_schema: "Metadata schema",
};

export function Diagnostics() {
  const [report, setReport] = useState<Report | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <section>
      <h2>{t("Deployment diagnostics")}</h2>
      <p className="help">
        {t(
          "Read-only checks for PostgreSQL, the loaded encryption key and connection pool. Reports exclude addresses, credentials, source names and query data.",
        )}
      </p>
      <ErrorNote error={error} />
      <div className="button-row">
        <Button
          busy={busy}
          onClick={async () => {
            setBusy(true);
            setError("");
            try {
              setReport(await api<Report>("/api/settings/diagnostics"));
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          {t("Run diagnostics")}
        </Button>
        {report && <CopyButton text={JSON.stringify(report, null, 2)} />}
      </div>
      {report && (
        <div role="status">
          <p className="help">
            {t("Last checked {time}", { time: date(report.checked_at) })}
          </p>
          <dl className="settings-list">
            {report.checks.map((check) => (
              <div key={check.check}>
                <dt>{t(checkNames[check.check] || check.check)}</dt>
                <dd>
                  {check.status === "passed" ? t("Passed") : t("Failed")}
                  {check.server_version &&
                    ` · PostgreSQL ${check.server_version}`}
                  {check.tls !== undefined &&
                    ` · ${check.tls ? t("TLS enabled") : t("TLS not enabled")}`}
                </dd>
              </div>
            ))}
            <div>
              <dt>{t("Connection pool")}</dt>
              <dd>
                {t("{used} in use / {open} open / {max} maximum", {
                  used: report.connection_pool.in_use,
                  open: report.connection_pool.open,
                  max: report.connection_pool.max,
                })}
              </dd>
            </div>
          </dl>
        </div>
      )}
      <details className="recovery-help">
        <summary>{t("Backup and recovery")}</summary>
        <p>
          {t(
            "Back up PostgreSQL and its matching encryption key together. A database dump alone cannot recover encrypted connections, semantics or OAuth state.",
          )}
        </p>
        <p>
          {t(
            "For the bundled Compose deployment, run from the project directory:",
          )}
        </p>
        <pre>bash scripts/backup-metadata.sh /secure/backups/hub</pre>
        <p>
          {t(
            "Verify a backup by restoring it into an isolated temporary PostgreSQL container:",
          )}
        </p>
        <pre>bash scripts/verify-backup.sh /secure/backups/hub</pre>
        <p className="help">
          {t(
            "Verification checks the restored schema, data counts and encrypted records using the saved key. It never connects to your data sources. Protect backup files like database credentials. Recovery instructions are in docs/operations.md.",
          )}
        </p>
        <p className="help">
          {t(
            "Backup freshness is not monitored by this service. The verification script reports its actual result; running diagnostics is not a restore test.",
          )}
        </p>
      </details>
    </section>
  );
}
