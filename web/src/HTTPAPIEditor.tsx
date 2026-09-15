import { HTTPAPIParameters } from "./HTTPAPIParameters";
import { useState } from "react";
import { Button, Field } from "./components";
import { t } from "./i18n";
import type { HTTPAPIConfig, HTTPOperation } from "./types";
import "./http-api.css";

export function initialHTTPAPI(): HTTPAPIConfig {
  return {
    base_url: "",
    probe_operation: "list_customers",
    operations: [
      {
        id: "list_customers",
        name: "List customers",
        method: "GET",
        path: "/customers",
        read_only: false,
        example_json: "{}",
        parameters: [],
        columns: [],
      },
    ],
  };
}

export function httpOperationQuery(op: HTTPOperation) {
  return `{\n  "operation": ${JSON.stringify(op.id)},\n  "named_params": ${op.example_json || "{}"}\n}`;
}

export function HTTPAPIEditor({
  value,
  onChange,
}: {
  value: HTTPAPIConfig;
  onChange: (value: HTTPAPIConfig) => void;
}) {
  const [selected, setSelected] = useState(0);
  const index = Math.min(selected, value.operations.length - 1);
  const operation = value.operations[index];
  function edit(patch: Partial<HTTPOperation>) {
    const operations = value.operations.map((op, i) =>
      i === index ? { ...op, ...patch } : op,
    );
    onChange({
      ...value,
      operations,
      probe_operation:
        patch.id && value.probe_operation === operation.id
          ? patch.id
          : value.probe_operation,
    });
  }
  return (
    <section
      className="form-section http-api-editor"
      id="http-api-operations"
      tabIndex={-1}
    >
      <h3>{t("Read API operations")}</h3>
      <p className="help">
        {t(
          "Only these operations are callable. Agents supply scalar values, never URLs, methods, headers or query fragments. Use read-scoped API credentials.",
        )}
      </p>
      {value.operations.length <= 6 && (
        <div className="http-operation-overview">
          {value.operations.map((op, i) => (
            <Button
              key={i}
              aria-pressed={i === index}
              onClick={() => setSelected(i)}
            >
              <strong>{op.name || op.id || t("New operation")}</strong>
              <small>
                {op.method} {op.path} ·{" "}
                {t("{count} parameters", { count: op.parameters?.length || 0 })}
              </small>
            </Button>
          ))}
        </div>
      )}
      <div className="http-operation-picker">
        {value.operations.length > 6 && (
          <Field label={t("API operation")}>
            <select
              value={index}
              onChange={(e) => setSelected(Number(e.target.value))}
            >
              {value.operations.map((op, i) => (
                <option key={i} value={i}>
                  {op.name || op.id || t("New operation")} · {op.method}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Button
          disabled={value.operations.length >= 40}
          onClick={() => {
            let number = value.operations.length + 1;
            while (
              value.operations.some((op) => op.id === `operation_${number}`)
            )
              number++;
            const op: HTTPOperation = {
              id: `operation_${number}`,
              name: "",
              method: "GET",
              path: "/",
              read_only: false,
              example_json: "{}",
              parameters: [],
              columns: [],
            };
            onChange({ ...value, operations: [...value.operations, op] });
            setSelected(value.operations.length);
          }}
        >
          {t("Add operation")}
        </Button>
      </div>
      {operation && (
        <div className="http-operation-card">
          <div className="field-grid">
            <Field label={t("Operation name")} required>
              <input
                required
                maxLength={120}
                value={operation.name}
                onChange={(e) => edit({ name: e.target.value })}
              />
            </Field>
            <Field
              label={t("Operation ID")}
              required
              hint={t("Stable identifier used by templates and Agents.")}
            >
              <input
                required
                pattern="[A-Za-z_][A-Za-z0-9_-]{0,63}"
                value={operation.id}
                onChange={(e) => edit({ id: e.target.value })}
              />
            </Field>
          </div>
          <Field label={t("Description")}>
            <input
              maxLength={4000}
              value={operation.description || ""}
              onChange={(e) => edit({ description: e.target.value })}
            />
          </Field>
          <div className="field-grid">
            <Field label={t("HTTP method")}>
              <select
                value={operation.method}
                onChange={(e) =>
                  edit({
                    method: e.target.value as HTTPOperation["method"],
                    body_json: e.target.value === "POST" ? "{}" : undefined,
                    parameters: operation.parameters?.filter(
                      (p) => p.in !== "body",
                    ),
                    read_only: false,
                  })
                }
              >
                <option>GET</option>
                <option>POST</option>
              </select>
            </Field>
            <Field
              label={t("Request path")}
              required
              hint={t("Relative to the base URL; for example /customers/{id}.")}
            >
              <input
                required
                placeholder="/customers"
                value={operation.path}
                onChange={(e) =>
                  edit({ path: e.target.value, read_only: false })
                }
              />
            </Field>
          </div>
          {operation.method === "POST" && (
            <Field
              label={t("Fixed JSON request body")}
              hint={t(
                "Only declared scalar value positions can be parameters. No string interpolation.",
              )}
            >
              <textarea
                rows={4}
                spellCheck={false}
                value={operation.body_json || "{}"}
                onChange={(e) =>
                  edit({ body_json: e.target.value, read_only: false })
                }
              />
            </Field>
          )}
          <label className="check-row http-read-declaration">
            <input
              type="checkbox"
              required
              checked={operation.read_only}
              onChange={(e) => edit({ read_only: e.target.checked })}
            />
            <span>
              {t(
                "I confirm this operation only reads data and has no side effects.",
              )}
            </span>
          </label>
          <p className="help">
            {t(
              "ContextGate restricts the request, but cannot prove an arbitrary API is read-only. This declaration is not a verified upstream permission.",
            )}
          </p>
          <HTTPAPIParameters
            parameters={operation.parameters || []}
            method={operation.method}
            onChange={(parameters) => edit({ parameters })}
          />
          <Field
            label={t("Example parameters (JSON)")}
            required
            hint={t(
              "Used for connection checks. Include every required parameter; never include credentials.",
            )}
          >
            <textarea
              required
              rows={3}
              spellCheck={false}
              value={operation.example_json}
              onChange={(e) => edit({ example_json: e.target.value })}
            />
          </Field>
          <Field
            label={t("Response data pointer")}
            hint={t(
              "Leave blank for the full JSON response, or select an array such as /data/items.",
            )}
          >
            <input
              placeholder="/data/items"
              value={operation.response_pointer || ""}
              onChange={(e) => edit({ response_pointer: e.target.value })}
            />
          </Field>
          <details className="advanced">
            <summary>{t("Response fields and pagination")}</summary>
            <p className="help">
              {t(
                "Declare response fields for semantics and ontology mappings. They are not inferred from sample data or independently verified.",
              )}
            </p>
            {operation.columns?.map((column, i) => (
              <div className="http-column" key={i}>
                <Field label={t("Field path")} required>
                  <input
                    required
                    value={column.name}
                    onChange={(e) =>
                      edit({
                        columns: operation.columns?.map((c, at) =>
                          at === i ? { ...c, name: e.target.value } : c,
                        ),
                      })
                    }
                  />
                </Field>
                <Field label={t("Data type")} required>
                  <input
                    required
                    placeholder="string"
                    value={column.type}
                    onChange={(e) =>
                      edit({
                        columns: operation.columns?.map((c, at) =>
                          at === i ? { ...c, type: e.target.value } : c,
                        ),
                      })
                    }
                  />
                </Field>
                <Button
                  aria-label={t("Remove field")}
                  onClick={() =>
                    edit({
                      columns: operation.columns?.filter((_, at) => at !== i),
                    })
                  }
                >
                  ×
                </Button>
              </div>
            ))}
            <Button
              disabled={(operation.columns?.length || 0) >= 200}
              onClick={() =>
                edit({
                  columns: [
                    ...(operation.columns || []),
                    { name: "", type: "string" },
                  ],
                })
              }
            >
              {t("Add response field")}
            </Button>
            <label className="check-row">
              <input
                type="checkbox"
                checked={!!operation.pagination}
                onChange={(e) =>
                  edit({
                    pagination: e.target.checked
                      ? {
                          query_parameter: "cursor",
                          next_pointer: "/next_cursor",
                        }
                      : undefined,
                  })
                }
              />
              {t("Enable token pagination")}
            </label>
            {operation.pagination && (
              <>
                <div className="field-grid">
                  <Field label={t("Cursor query parameter")} required>
                    <input
                      required
                      value={operation.pagination.query_parameter}
                      onChange={(e) =>
                        edit({
                          pagination: {
                            ...operation.pagination!,
                            query_parameter: e.target.value,
                          },
                        })
                      }
                    />
                  </Field>
                  <Field label={t("Next token response pointer")} required>
                    <input
                      required
                      value={operation.pagination.next_pointer}
                      onChange={(e) =>
                        edit({
                          pagination: {
                            ...operation.pagination!,
                            next_pointer: e.target.value,
                          },
                        })
                      }
                    />
                  </Field>
                </div>
                <p className="help">
                  {t(
                    "The last page must return null or an empty token. Returned URLs are never followed. Configure page size as an ordinary bounded query parameter.",
                  )}
                </p>
              </>
            )}
          </details>
          <Button
            disabled={value.operations.length === 1}
            onClick={() => {
              const operations = value.operations.filter(
                (_, at) => at !== index,
              );
              onChange({
                ...value,
                operations,
                probe_operation:
                  value.probe_operation === operation.id
                    ? operations[0].id
                    : value.probe_operation,
              });
              setSelected(0);
            }}
          >
            {t("Remove operation")}
          </Button>
        </div>
      )}
      <Field
        label={t("Connection check operation")}
        hint={t(
          "Test connection and template verification execute this operation using its example parameters.",
        )}
      >
        <select
          value={value.probe_operation}
          onChange={(e) =>
            onChange({ ...value, probe_operation: e.target.value })
          }
        >
          {value.operations.map((op, i) => (
            <option key={i} value={op.id}>
              {op.name || op.id}
            </option>
          ))}
        </select>
      </Field>
    </section>
  );
}
