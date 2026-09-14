import { Button, Field } from "./components";
import { t } from "./i18n";
import type { RegressionCase } from "./semantic-types";

export function RegressionEditor({
  tests,
  update,
}: {
  tests: RegressionCase[];
  update: (cases: RegressionCase[]) => void;
}) {
  const edit = (index: number, values: Partial<RegressionCase>) =>
    update(tests.map((c, i) => (i === index ? { ...c, ...values } : c)));
  return (
    <section className="regression-editor">
      <h3>{t("Regression checks")}</h3>
      <p className="help">
        {t(
          "Add up to 10 parameter sets with expected row counts, columns or exact JSON values. Trials run all cases; failed checks block publication. Results are not saved.",
        )}
      </p>
      {tests.map((test, i) => (
        <fieldset key={i} className="regression-case">
          <legend>{t("Case {number}", { number: i + 1 })}</legend>
          <Field label={t("Case name")} required>
            <input
              required
              value={test.name}
              maxLength={120}
              onChange={(e) => edit(i, { name: e.target.value })}
            />
          </Field>
          <Field label={t("Parameters (JSON)")} required>
            <textarea
              required
              className="mono"
              rows={3}
              value={test.parameters_json}
              onChange={(e) => edit(i, { parameters_json: e.target.value })}
            />
          </Field>
          <div className="field-grid">
            <Field label={t("Minimum rows")}>
              <input
                type="number"
                min={0}
                max={10000}
                value={test.min_rows ?? ""}
                onChange={(e) =>
                  edit(i, {
                    min_rows:
                      e.target.value === ""
                        ? undefined
                        : Number(e.target.value),
                  })
                }
              />
            </Field>
            <Field label={t("Maximum rows")}>
              <input
                type="number"
                min={0}
                max={10000}
                value={test.max_rows ?? ""}
                onChange={(e) =>
                  edit(i, {
                    max_rows:
                      e.target.value === ""
                        ? undefined
                        : Number(e.target.value),
                  })
                }
              />
            </Field>
          </div>
          <details>
            <summary>{t("Expected columns and values")}</summary>
            <p className="help">
              {t(
                "Native type is optional. Value pointers start at the result data array: /0/0 for the first table cell, or /0/total for a document field. Use the exact lossless JSON encoding for large numbers. Truncated or paginated results cannot pass a regression case.",
              )}
            </p>
            {(test.columns || []).map((column, n) => (
              <div className="regression-row" key={n}>
                <Field label={t("Column name")}>
                  <input
                    required
                    value={column.name}
                    onChange={(e) =>
                      edit(i, {
                        columns: test.columns?.map((c, j) =>
                          j === n ? { ...c, name: e.target.value } : c,
                        ),
                      })
                    }
                  />
                </Field>
                <Field label={t("Native type")}>
                  <input
                    value={column.type}
                    onChange={(e) =>
                      edit(i, {
                        columns: test.columns?.map((c, j) =>
                          j === n ? { ...c, type: e.target.value } : c,
                        ),
                      })
                    }
                  />
                </Field>
                <Button
                  onClick={() =>
                    edit(i, {
                      columns: test.columns?.filter((_, j) => j !== n),
                    })
                  }
                >
                  {t("Remove")}
                </Button>
              </div>
            ))}
            <Button
              disabled={(test.columns?.length || 0) >= 100}
              onClick={() =>
                edit(i, {
                  columns: [...(test.columns || []), { name: "", type: "" }],
                })
              }
            >
              {t("Add expected column")}
            </Button>
            {(test.values || []).map((value, n) => (
              <div className="regression-row" key={n}>
                <Field label={t("Result JSON Pointer")}>
                  <input
                    required
                    value={value.pointer}
                    placeholder="/0/0"
                    onChange={(e) =>
                      edit(i, {
                        values: test.values?.map((v, j) =>
                          j === n ? { ...v, pointer: e.target.value } : v,
                        ),
                      })
                    }
                  />
                </Field>
                <Field label={t("Expected value (JSON)")}>
                  <input
                    required
                    className="mono"
                    value={value.expected_json}
                    onChange={(e) =>
                      edit(i, {
                        values: test.values?.map((v, j) =>
                          j === n ? { ...v, expected_json: e.target.value } : v,
                        ),
                      })
                    }
                  />
                </Field>
                <Button
                  onClick={() =>
                    edit(i, { values: test.values?.filter((_, j) => j !== n) })
                  }
                >
                  {t("Remove")}
                </Button>
              </div>
            ))}
            <Button
              disabled={(test.values?.length || 0) >= 50}
              onClick={() =>
                edit(i, {
                  values: [
                    ...(test.values || []),
                    { pointer: "", expected_json: "null" },
                  ],
                })
              }
            >
              {t("Add expected value")}
            </Button>
          </details>
          <Button
            className="danger"
            onClick={() => update(tests.filter((_, n) => i !== n))}
          >
            {t("Remove case")}
          </Button>
        </fieldset>
      ))}
      <Button
        disabled={tests.length >= 10}
        onClick={() =>
          update([
            ...tests,
            { name: `Case ${tests.length + 1}`, parameters_json: "{}" },
          ])
        }
      >
        {t("Add regression case")}
      </Button>
    </section>
  );
}
