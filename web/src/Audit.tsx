import { t } from "./i18n";
import { useEffect, useState } from "react";
import { api, date, message } from "./api";
import {
  Button,
  CopyButton,
  Drawer,
  Empty,
  ErrorNote,
  Field,
  Loading,
} from "./components";
import type { Agent, Audit, Source } from "./types";

const operationLabels: Record<string, string> = {
  "administrator.setup": "Initialize administrator",
  "administrator.login": "Administrator sign in",
  "administrator.logout": "Administrator sign out",
  "administrator.create": "Create administrator",
  "administrator.update": "Update administrator",
  "administrator.reset_password": "Reset administrator password",
  "administrator.recover_password": "Recover administrator password",
  "configuration_agent.rotate_token": "Rotate configuration token",
  "source.test": "Test connection",
  "health.check": "Check source health",
  "semantics.post.trial": "Trial query template",
  "semantics.post.check-mapping": "Check ontology mapping",
  "configuration_agent.create": "Create configuration access",
  "configuration_agent.revoke": "Revoke configuration access",
  "configuration.create_data_source": "Create data source",
  "configuration.update_data_source": "Update data source",
  "configuration.test_data_source": "Test connection",
  "configuration.discover_source_structure": "Discover structure",
  "configuration.save_semantic_draft": "Save semantic draft",
  "configuration.upsert_semantic_entry": "Save semantic entry",
  "configuration.remove_semantic_entry": "Delete semantic entry",
  "configuration.import_source_structure": "Import structure",
  "configuration.validate_semantic_draft": "Validate semantics",
  "configuration.trial_query_template": "Trial query template",
  "configuration.check_ontology_mapping": "Check ontology mapping",
  "configuration.create_ontology": "Create ontology",
  "configuration.save_ontology_draft": "Save ontology draft",
  "configuration.validate_ontology": "Validate ontology",
  "source.create": "Create data source",
  "source.update": "Update data source",
  "source.delete": "Delete data source",
  "agent.create": "Create Agent",
  "agent.update": "Update Agent access",
  "agent.revoke": "Revoke Agent",
  "agent.rotate_token": "Rotate Agent token",
  "administrator.change_password": "Change administrator password",
  "settings.health": "Update scheduled checks",
  "settings.audit_export": "Update audit export",
  "health.accept_baseline": "Accept structure baseline",
  "semantics.put": "Save semantic draft",
  "semantics.put.entries.{entry}": "Save semantic entry",
  "semantics.delete.entries.{entry}": "Delete semantic entry",
  "semantics.post.publish": "Publish semantics",
  "semantics.post.restore": "Restore semantic draft",
  "semantics.post.discard": "Discard semantic draft",
  "semantics.post.import": "Import semantic draft",
  "semantics.post.import-structure": "Import structure",
  "semantics.delete.versions.{version}": "Delete historical publication",
  "oauth.client.post": "Register OAuth client",
  "oauth.client.delete": "Remove OAuth client",
  "oauth.consent": "Record OAuth consent",
  "ontology.post.": "Create ontology",
  "ontology.put./{ontology}": "Save ontology draft",
  "ontology.delete./{ontology}": "Delete ontology",
  "ontology.post./{ontology}/publish": "Publish ontology",
  "ontology.post./{ontology}/discard": "Discard ontology draft",
  "ontology.post./{ontology}/archive": "Change ontology archive status",
  "ontology.post./{ontology}/import": "Import ontology draft",
  "ontology.delete./{ontology}/versions/{version}": "Delete ontology version",
};

const hints: Record<string, string> = {
  database_authentication:
    "Check the database authentication method and replace the stored credential if needed.",
  database_permission:
    "Check the configured account's read permissions in the database.",
  database_tls:
    "Check the CA certificate, server hostname and certificate expiry.",
  database_dns: "Check hostname resolution from the ContextGate server.",
  database_connection:
    "Check the database host, port, name and server availability.",
  database_query:
    "Check the query dialect, parameters, namespace and object name.",
  timeout:
    "Check connectivity, reduce the query scope or review the configured timeout.",
  cancelled: "The caller cancelled the query or its authorization changed.",
  not_found: "The source is disabled, missing or outside this Agent's grants.",
  query_denied:
    "The operation is outside the supported read-only query boundary.",
  templates_only:
    "This source accepts published templates only. Discover a template with search_semantics, then call execute_query_template.",
  template_unverified:
    "The template needs a successful trial against the current connection. Trial and publish it again in Semantics.",
  template_changed:
    "The published template changed. Refresh the semantic catalog and use its current execution version.",
};
export function AuditPage({
  superAdmin,
  sources,
  agents,
  navigate,
}: {
  superAdmin: boolean;
  sources: Source[];
  agents: Agent[];
  navigate: (url: string) => void;
}) {
  const [rows, setRows] = useState<Audit[]>([]);
  const [configurationAgents, setConfigurationAgents] = useState<
    { id: string; name: string }[]
  >([]);
  const [administrators, setAdministrators] = useState<
    { id: string; username: string; display_name: string }[]
  >([]);
  const [callerError, setCallerError] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [filters, setFilters] = useState({
    administrator_id: "",
    configuration_agent_id: "",
    channel: "",
    view: "client",
    event_kind: "",
    agent_id: "",
    source_id: "",
    status: "",
    request_id: "",
    from: "",
    until: "",
  });
  const [query, setQuery] = useState("view=client");
  const [history, setHistory] = useState<string[]>([""]);
  const [refresh, setRefresh] = useState(0);
  const [detail, setDetail] = useState<Audit | null>(null);
  const before = history[history.length - 1];
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    const params = new URLSearchParams(query);
    if (before) params.set("before", before);
    api<Audit[]>(`/api/audit?${params}`, { signal: controller.signal })
      .then(setRows)
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [query, before, refresh]);
  useEffect(() => {
    const controller = new AbortController();
    setCallerError("");
    api<{
      administrators: { id: string; username: string; display_name: string }[];
    }>("/api/audit/administrators", { signal: controller.signal })
      .then((data) => setAdministrators(data.administrators))
      .catch((e) => {
        if (!controller.signal.aborted) setCallerError(message(e));
      });
    api<{ agents: { id: string; name: string }[] }>(
      "/api/configuration-agents",
      { signal: controller.signal },
    )
      .then((data) => setConfigurationAgents(data.agents))
      .catch((e) => {
        if (!controller.signal.aborted) setCallerError(message(e));
      });
    return () => controller.abort();
  }, [refresh]);
  function applyFilters(next: typeof filters) {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(next)) {
      if (value)
        params.set(
          key,
          key === "from" || key === "until"
            ? new Date(value).toISOString()
            : value.trim(),
        );
    }
    setFilters(next);
    setQuery(params.toString());
    setHistory([""]);
    setRefresh((n) => n + 1);
  }
  const agentName = (id: string) =>
    id === "admin"
      ? t("Administrator")
      : agents.find((a) => a.id === id)?.name ||
        configurationAgents.find((a) => a.id === id)?.name ||
        id;
  const sourceName = (id: string) =>
    sources.find((s) => s.id === id)?.name || id || t("Service settings");
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Audit log")}</h1>
          <p>{t("Query and configuration activity · Last 30 days")}</p>
        </div>
        <Button busy={loading} onClick={() => setRefresh((n) => n + 1)}>
          {t("Refresh")}
        </Button>
      </div>
      <ErrorNote error={callerError} />
      <div className="audit-views" role="group" aria-label={t("Activity view")}>
        {[
          ["client", "Real Agent calls"],
          ["preview", "Manual previews"],
          ["system", "System checks"],
          ["", "All activity"],
        ].map(([view, label]) => (
          <Button
            key={label}
            primary={filters.view === view}
            aria-pressed={filters.view === view}
            onClick={() => {
              applyFilters({ ...filters, view, event_kind: "" });
            }}
          >
            {t(label)}
          </Button>
        ))}
      </div>

      <form
        className="audit-filters"
        onSubmit={(e) => {
          e.preventDefault();
          applyFilters(filters);
        }}
      >
        <Field label={t("Agent")}>
          <select
            value={filters.agent_id}
            onChange={(e) =>
              setFilters({ ...filters, agent_id: e.target.value })
            }
          >
            <option value="">{t("All callers")}</option>
            <option value="admin">{t("Administrator")}</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
            {configurationAgents.map((a) => (
              <option key={a.id} value={a.id}>
                {t("Configuration Agent: {name}", { name: a.name })}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t("Data source")}>
          <select
            value={filters.source_id}
            onChange={(e) =>
              setFilters({ ...filters, source_id: e.target.value })
            }
          >
            <option value="">{t("All sources")}</option>
            {sources.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t("Result")}>
          <select
            value={filters.status}
            onChange={(e) => setFilters({ ...filters, status: e.target.value })}
          >
            <option value="">{t("All results")}</option>
            <option value="success">{t("Success")}</option>
            <option value="error">{t("Error")}</option>
          </select>
        </Field>
        <details className="advanced audit-advanced-filters">
          <summary>
            {t("More filters")}
            {(filters.from ||
              filters.until ||
              filters.request_id ||
              filters.event_kind ||
              filters.administrator_id ||
              filters.channel ||
              filters.configuration_agent_id) && (
              <small>{t("Filters set")}</small>
            )}
          </summary>
          <div className="audit-time-filters">
            <Field label={t("Event type")}>
              <select
                value={filters.event_kind}
                onChange={(e) =>
                  setFilters({
                    ...filters,
                    view: "",
                    event_kind: e.target.value,
                  })
                }
              >
                <option value="">{t("All events")}</option>
                <option value="query">{t("Query calls")}</option>
                <option value="management">{t("Management changes")}</option>
                <option value="system">{t("System checks")}</option>
                {superAdmin && (
                  <option value="security">{t("Account security")}</option>
                )}
              </select>
            </Field>
            <Field label={t("Administrator")}>
              <select
                value={filters.administrator_id}
                onChange={(e) =>
                  setFilters({
                    ...filters,
                    administrator_id: e.target.value,
                    view: "",
                  })
                }
              >
                <option value="">{t("All administrators")}</option>
                {administrators.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.display_name} · {a.username}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={t("Entry point")}>
              <select
                value={filters.channel}
                onChange={(e) =>
                  setFilters({ ...filters, channel: e.target.value, view: "" })
                }
              >
                <option value="">{t("All entry points")}</option>
                <option value="ui">{t("Management UI")}</option>
                <option value="configuration_mcp">
                  {t("Configuration MCP")}
                </option>
                <option value="cli">CLI</option>
              </select>
            </Field>
            <Field label={t("Configuration identity ID")}>
              <input
                value={filters.configuration_agent_id}
                onChange={(e) =>
                  setFilters({
                    ...filters,
                    configuration_agent_id: e.target.value,
                    view: "",
                  })
                }
                placeholder="cfg_…"
              />
            </Field>

            <Field label={t("From")}>
              <input
                type="datetime-local"
                value={filters.from}
                onChange={(e) =>
                  setFilters({ ...filters, from: e.target.value })
                }
              />
            </Field>
            <Field label={t("Until")}>
              <input
                type="datetime-local"
                value={filters.until}
                min={filters.from || undefined}
                onChange={(e) =>
                  setFilters({ ...filters, until: e.target.value })
                }
              />
            </Field>
            <Field label={t("Request ID")}>
              <input
                value={filters.request_id}
                onChange={(e) =>
                  setFilters({ ...filters, request_id: e.target.value })
                }
              />
            </Field>
          </div>
        </details>
        <div className="button-row">
          <Button primary type="submit">
            {t("Apply filters")}
          </Button>
          <Button
            type="button"
            onClick={() => {
              setFilters({
                view: "",
                administrator_id: "",
                configuration_agent_id: "",
                channel: "",
                event_kind: "",
                agent_id: "",
                source_id: "",
                status: "",
                request_id: "",
                from: "",
                until: "",
              });
              setQuery("");
              setHistory([""]);
            }}
          >
            {t("Clear")}
          </Button>
        </div>
      </form>
      <ErrorNote error={error} />
      {loading ? (
        <Loading />
      ) : error ? null : !rows.length ? (
        <Empty
          title={t("No matching calls")}
          description={t("Adjust the filters or run a query from your Agent.")}
        />
      ) : (
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>{t("Time")}</th>
                <th>{t("Caller")}</th>
                <th>{t("Data source")}</th>
                <th>{t("Operation")}</th>
                <th>{t("Duration")}</th>
                <th>{t("Rows")}</th>
                <th>{t("Result")}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((a) => (
                <tr key={a.id}>
                  <td className="nowrap">{date(a.at)}</td>
                  <td>
                    {a.administrator_username || agentName(a.agent_id)}
                    {a.channel && (
                      <small className="block help">
                        {a.channel === "configuration_mcp"
                          ? t("Configuration MCP")
                          : a.channel === "ui"
                            ? t("Management UI")
                            : a.channel}
                      </small>
                    )}
                    {a.preview || a.event_kind === "system" ? (
                      <small className="block">
                        {a.event_kind === "system"
                          ? t("System check")
                          : t("Manual preview")}
                      </small>
                    ) : null}
                  </td>
                  <td>
                    {a.event_kind === "management" && !a.source_id
                      ? a.resource_id || t("Service settings")
                      : sourceName(a.source_id)}
                  </td>
                  <td>
                    {t(operationLabels[a.operation] || a.operation)}
                    {a.event_kind === "management" && (
                      <small className="block">{t("Management change")}</small>
                    )}
                    {a.template_id && (
                      <small className="block">
                        {a.template_id}
                        {t(" · v")}
                        {a.template_version || t("draft")}
                      </small>
                    )}
                  </td>
                  <td>
                    {a.elapsed_ms}
                    {t(" ms")}
                  </td>
                  <td>{a.rows}</td>
                  <td>
                    <button
                      className={`text-button ${a.error_code ? "amber" : ""}`}
                      onClick={() => setDetail(a)}
                    >
                      {a.error_code === "operation_pending"
                        ? t("Outcome pending")
                        : a.error_code || t("Success")}
                      {t(" · Details")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="button-row pagination">
        <Button
          disabled={loading || history.length < 2}
          onClick={() => setHistory((h) => h.slice(0, -1))}
        >
          {t("Newer records")}
        </Button>
        <span>
          {t("Page ")}
          {history.length}
        </span>
        <Button
          disabled={loading || rows.length < 100}
          onClick={() =>
            setHistory((h) => [...h, String(rows[rows.length - 1].id)])
          }
        >
          {t("Older records")}
        </Button>
      </div>
      {detail ? (
        <Drawer
          title={t(
            detail.event_kind === "management" ||
              detail.event_kind === "security"
              ? "Management change details"
              : "Call details",
          )}
          subtitle={
            detail.error_code
              ? t("Review the diagnostic and suggested action")
              : t(
                  detail.event_kind === "management" ||
                    detail.event_kind === "security"
                    ? "Change completed successfully"
                    : "Call completed successfully",
                )
          }
          onClose={() => setDetail(null)}
        >
          <dl className="settings-list">
            <div>
              <dt>{t("Request ID")}</dt>
              <dd>
                <code>{detail.request_id || t("Legacy record")}</code>
                {detail.request_id ? (
                  <CopyButton text={detail.request_id} />
                ) : null}
              </dd>
            </div>
            <div>
              <dt>{t("Administrator")}</dt>
              <dd>
                {detail.administrator_username ||
                  t("Historical or non-administrator identity")}
                <small className="block help">{detail.administrator_id}</small>
              </dd>
              <dt>{t("Entry point")}</dt>
              <dd>{detail.channel || "—"}</dd>
              <dt>{t("Configuration identity ID")}</dt>
              <dd>{detail.configuration_agent_id || "—"}</dd>
              <dt>{t("Execution Agent")}</dt>
              <dd>{agentName(detail.agent_id)}</dd>
              <dt>{t("Caller")}</dt>
              <dd>
                {detail.administrator_username || agentName(detail.agent_id)}
                {detail.event_kind === "system"
                  ? " · " + t("System check")
                  : detail.preview
                    ? " · " + t("Manual preview")
                    : ""}
              </dd>
            </div>
            <div>
              <dt>{t("Data source")}</dt>
              <dd>{sourceName(detail.source_id)}</dd>
            </div>
            <div>
              <dt>{t("Resource / revision")}</dt>
              <dd>
                <code>
                  {detail.resource_id ||
                    detail.source_id ||
                    t("Service settings")}
                </code>
                {detail.revision ? ` · ${detail.revision}` : ""}
              </dd>
            </div>
            <div>
              <dt>{t("Time")}</dt>
              <dd>{date(detail.at)}</dd>
            </div>
            <div>
              <dt>{t("Operation")}</dt>
              <dd>
                {t(operationLabels[detail.operation] || detail.operation)}
                <small className="block mono">{detail.operation}</small>
              </dd>
              {detail.template_id && (
                <>
                  <dt>{t("Query template")}</dt>
                  <dd>
                    {detail.template_id}
                    {t(" · execution version")}{" "}
                    {detail.template_version || t("draft trial")}
                  </dd>
                </>
              )}
            </div>
            {detail.ontology_id && (
              <div>
                <dt>{t("Ontology")}</dt>
                <dd>
                  {detail.ontology_id}
                  {t(" · version ")}
                  {detail.ontology_version}
                </dd>
              </div>
            )}
            <div>
              <dt>{t("Result")}</dt>
              <dd>{detail.error_code || t("Success")}</dd>
            </div>
            {detail.event_kind !== "management" &&
              detail.event_kind !== "security" && (
                <div>
                  <dt>{t("Database code")}</dt>
                  <dd>{detail.native_code || t("Not reported")}</dd>
                </div>
              )}
            <div>
              <dt>
                {t(
                  detail.event_kind === "management" ||
                    detail.event_kind === "security"
                    ? "Duration"
                    : "Duration / rows",
                )}
              </dt>
              <dd>
                {detail.elapsed_ms}
                {detail.event_kind === "management" ||
                detail.event_kind === "security" ? (
                  t(" ms")
                ) : (
                  <>
                    {t(" ms / ")}
                    {detail.rows}
                  </>
                )}
              </dd>
            </div>
            {detail.event_kind !== "management" &&
              detail.event_kind !== "security" && (
                <div>
                  <dt>{t("Query fingerprint")}</dt>
                  <dd className="mono break-all">
                    {detail.fingerprint || t("Not applicable")}
                  </dd>
                </div>
              )}
          </dl>
          {detail.error_code ? (
            <div className="notice warning">
              {detail.event_kind === "management" ||
              detail.event_kind === "security"
                ? t(
                    "Review the current configuration and refresh before retrying. Use the request ID when reporting the issue.",
                  )
                : (hints[detail.error_code]
                    ? t(hints[detail.error_code])
                    : "") ||
                  t(
                    "Check the database server logs at this time. Use the request ID when reporting the issue.",
                  )}
            </div>
          ) : null}
          {detail.error_code && (
            <div className="button-row">
              {sources.some((s) => s.id === detail.source_id) && (
                <Button
                  onClick={() => {
                    const sourceID = detail.source_id;
                    const url =
                      detail.error_code === "templates_only"
                        ? "/business?source_id=" +
                          encodeURIComponent(sourceID) +
                          "&agent_id=" +
                          encodeURIComponent(detail.agent_id) +
                          "&view=queries"
                        : detail.template_id
                          ? "/sources/" +
                            sourceID +
                            "/semantics?tab=Query%20templates&focus=" +
                            encodeURIComponent(detail.template_id)
                          : "/sources/" + sourceID + "/setup";
                    setDetail(null);
                    navigate(url);
                  }}
                >
                  {detail.error_code === "templates_only"
                    ? t("Find an available query")
                    : detail.template_id
                      ? t("Review templates")
                      : t("Open source workspace")}
                </Button>
              )}
              {agents.some((a) => a.id === detail.agent_id) && (
                <Button
                  onClick={() => {
                    setDetail(null);
                    navigate(
                      "/agents?attention=" +
                        encodeURIComponent(detail.agent_id),
                    );
                  }}
                >
                  {t("Review Agents")}
                </Button>
              )}
            </div>
          )}
          {(detail.event_kind === "management" ||
            detail.event_kind === "security") && (
            <p className="help">
              {t("Submitted field categories")}:{" "}
              {detail.changed_fields || t("Not applicable")}.{" "}
              {t(
                "Field values are never retained. A pending event without an outcome requires checking the current configuration before retrying.",
              )}
            </p>
          )}
          <p className="help">
            {t(
              "Diagnostics contain stable error codes only. Credentials, database error text, query text and parameters are not retained.",
            )}
          </p>
        </Drawer>
      ) : null}
    </>
  );
}
