import { t } from "./i18n";
import { useState } from "react";
import { Plus } from "lucide-react";
import { Button, Empty } from "./components";
import type { OntologyDefinition } from "./ontology-types";
import type { OntologyItem, OntologyKind } from "./OntologyEditor";

export function OntologyDefinitions({
  definition,
  disabled,
  onAdd,
  onEdit,
  onRemove,
}: {
  definition: OntologyDefinition;
  disabled: boolean;
  onAdd: (kind: OntologyKind) => void;
  onEdit: (kind: OntologyKind, item: OntologyItem) => void;
  onRemove: (kind: OntologyKind, id: string) => void;
}) {
  const [kind, setKind] = useState<OntologyKind>("entities");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const rows: OntologyItem[] = definition[kind].filter((v) =>
    [v.name, v.id, v.description, ...(v.aliases || [])]
      .join(" ")
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const current = Math.min(page, Math.max(0, Math.ceil(rows.length / 20) - 1));
  const noun =
    kind === "entities"
      ? "entity"
      : kind === "properties"
        ? "property"
        : "relationship";
  const name = (id: string) =>
    definition.entities.find((e) => e.id === id)?.name || id;
  return (
    <>
      <div className="filters">
        <select
          aria-label={t("Definition type")}
          value={kind}
          onChange={(e) => {
            setKind(e.target.value as OntologyKind);
            setPage(0);
          }}
        >
          <option value="entities">{t("Entities")}</option>
          <option value="properties">{t("Properties")}</option>
          <option value="relations">{t("Relationships")}</option>
        </select>
        <input
          aria-label={t("Search ontology definitions")}
          placeholder={t("Search names, IDs and descriptions")}
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
        />
        <Button primary disabled={disabled} onClick={() => onAdd(kind)}>
          <Plus size={16} />
          {t(" Add ")}
          {t(noun)}
        </Button>
      </div>
      {rows.length === 0 ? (
        <Empty
          title={
            search
              ? t("No matching definitions")
              : t("No {kind} defined", { kind: t(kind) })
          }
          description={t(
            "Use Model to build an entity with its properties and relationships.",
          )}
        />
      ) : (
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>{t("Name / ID")}</th>
                <th>{t("Definition")}</th>
                <th>{t("Description")}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {rows.slice(current * 20, (current + 1) * 20).map((v) => (
                <tr key={v.id}>
                  <td>
                    <button
                      className="ontology-text-button"
                      disabled={disabled}
                      onClick={() => onEdit(kind, v)}
                    >
                      {v.name}
                    </button>
                    <small className="block">{v.id}</small>
                  </td>
                  <td>
                    {"entity" in v
                      ? `${name(v.entity)} · ${v.type}${v.multiple ? "[]" : ""}`
                      : "from" in v
                        ? `${name(v.from)} ${v.directed ? "→" : "↔"} ${name(v.to)}`
                        : v.parent
                          ? t("Inherits {value1}", { value1: name(v.parent) })
                          : t("Root entity")}
                  </td>
                  <td className="semantic-description">
                    {v.description || "—"}
                  </td>
                  <td>
                    <div className="button-row">
                      <Button
                        disabled={disabled}
                        onClick={() => onEdit(kind, v)}
                      >
                        {t("Edit")}
                      </Button>
                      <Button
                        disabled={disabled}
                        className="danger"
                        onClick={() => onRemove(kind, v.id)}
                      >
                        {t("Remove")}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {rows.length > 20 && (
        <div className="pagination">
          <Button disabled={current === 0} onClick={() => setPage(current - 1)}>
            {t("Previous")}
          </Button>
          <span>{current + 1}</span>
          <Button
            disabled={(current + 1) * 20 >= rows.length}
            onClick={() => setPage(current + 1)}
          >
            {t("Next")}
          </Button>
        </div>
      )}
    </>
  );
}
