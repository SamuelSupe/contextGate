import { useState } from "react";
import { api, APIError, message, payload } from "./api";
import { Button, Drawer, ErrorNote, Field } from "./components";
import type { Agent, Source } from "./types";
export function AgentEditor({
  agent,
  sources,
  onClose,
  saved,
  reload,
}: {
  agent: Agent | null;
  sources: Source[];
  reload: () => Promise<void>;
  onClose: () => void;
  saved: (result: { agent: Agent; token?: string }) => Promise<void>;
}) {
  const [name, setName] = useState(agent?.name || "");
  const [selected, setSelected] = useState(agent?.sources || []);
  const [expires, setExpires] = useState(() => {
    const date = agent
      ? new Date(agent.expires_at)
      : new Date(Date.now() + 90 * 86400000);
    const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
    return local.toISOString().slice(0, 19);
  });
  const [initialExpires] = useState(expires);
  const [enabled, setEnabled] = useState(agent?.enabled ?? true);
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [error, setError] = useState("");
  return (
    <Drawer
      title={agent ? "Edit Agent grants" : "Create Agent"}
      subtitle="Allow access only to the selected data sources"
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={busy} onClick={onClose}>
            Cancel
          </Button>
          <Button primary busy={busy} form="agent-form" type="submit">
            {agent ? "Save grants" : "Create and generate token"}
          </Button>
        </>
      }
    >
      <ErrorNote error={error} />
      {conflict ? (
        <Button
          busy={busy}
          onClick={async () => {
            setBusy(true);
            try {
              await reload();
              onClose();
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          Close and reload current grants
        </Button>
      ) : null}
      <form
        id="agent-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            const res = await api<{ agent: Agent; token?: string }>(
              agent ? `/api/agents/${agent.id}` : "/api/agents",
              {
                method: agent ? "PUT" : "POST",
                body: payload({
                  revision: agent?.revision,
                  name,
                  sources: selected,
                  enabled,
                  expires_at:
                    agent && expires === initialExpires
                      ? agent.expires_at
                      : new Date(expires).toISOString(),
                }),
              },
            );
            await saved(res);
          } catch (e) {
            setConflict(e instanceof APIError && e.detail.code === "conflict");
            setError(message(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <Field label="Agent name" required>
          <input
            required
            maxLength={120}
            placeholder="e.g. Analytics assistant"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
        <Field
          label="Expires at"
          hint="Date and time in your local time zone."
          required
        >
          <input
            type="datetime-local"
            step="1"
            required
            value={expires}
            onChange={(e) => setExpires(e.target.value)}
          />
        </Field>
        <section className="form-section">
          <h3>
            Granted data sources <small>{selected.length} selected</small>
          </h3>
          <p className="help">
            Database, table and field access is controlled by the account
            configured on each data source.
          </p>
          {selected.some((id) => !sources.some((s) => s.id === id)) ? (
            <div className="notice neutral">
              <p>Some granted data sources were deleted.</p>
              <Button
                onClick={() =>
                  setSelected(
                    selected.filter((id) => sources.some((s) => s.id === id)),
                  )
                }
              >
                Remove deleted grants
              </Button>
            </div>
          ) : null}
          <div className="source-checklist">
            {sources.length ? (
              sources.map((s) => (
                <label className="check-row" key={s.id}>
                  <input
                    type="checkbox"
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
                    <small>
                      {s.kind}
                      {s.enabled ? "" : " · Disabled"}
                    </small>
                  </span>
                </label>
              ))
            ) : (
              <p className="help">Add a data source first.</p>
            )}
          </div>
        </section>
        <label className="check-row">
          <input
            type="checkbox"
            checked={enabled}
            disabled={!!agent?.revoked_at}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          {agent?.revoked_at
            ? "Credential permanently revoked. Issue a new token or authorize OAuth again."
            : "Enable Agent access (uncheck to pause)"}
        </label>
      </form>
    </Drawer>
  );
}
