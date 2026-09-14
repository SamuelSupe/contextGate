import { t } from "./i18n";
import type { QueryResult } from "./types";
function cell(value: unknown) {
  return value === null
    ? "null"
    : typeof value === "object"
      ? JSON.stringify(value)
      : String(value ?? "");
}
export function ResultTable({ result }: { result: QueryResult }) {
  const first = result.data[0];
  const keys =
    !Array.isArray(first) && typeof first === "object" && first
      ? [
          ...new Set(
            result.data.flatMap((row) =>
              row && typeof row === "object" && !Array.isArray(row)
                ? Object.keys(row)
                : [],
            ),
          ),
        ]
      : [];
  const columns = result.columns?.length
    ? result.columns
    : keys.length
      ? keys.map((name) => ({ name, type: "" }))
      : Array.from(
          { length: Array.isArray(first) ? first.length : 1 },
          (_, i) => ({ name: t("Value {index}", { index: i + 1 }), type: "" }),
        );
  return (
    <div className="table-scroll result-table">
      <table>
        <thead>
          <tr>
            {columns.map((c, i) => (
              <th key={i}>
                {c.name}
                <small className="block">{c.type}</small>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {result.data.map((row, i) => (
            <tr key={i}>
              {columns.map((c, j) => (
                <td key={j}>
                  {cell(
                    Array.isArray(row)
                      ? row[j]
                      : row && typeof row === "object"
                        ? (row as Record<string, unknown>)[c.name]
                        : row,
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
