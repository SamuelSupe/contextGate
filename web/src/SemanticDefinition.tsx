import { t } from "./i18n";
import type { EntityType, Property, RelationType } from "./ontology-types";
import type { ObjectReference, SemanticEntry } from "./semantic-types";

export function referenceName(reference: ObjectReference) {
  return [reference.namespace, reference.object, reference.field]
    .filter(Boolean)
    .join(".");
}

export function SemanticDefinition({
  entry,
  context = [],
  limited = false,
}: {
  entry: SemanticEntry;
  context?: SemanticEntry[];
  limited?: boolean;
}) {
  const property =
    entry.kind === "property"
      ? (entry.definition as Property | undefined)
      : undefined;
  const entity =
    entry.kind === "entity_type"
      ? (entry.definition as EntityType | undefined)
      : undefined;
  const relation =
    entry.kind === "relation_type"
      ? (entry.definition as RelationType | undefined)
      : undefined;
  const mapping = entry.mapping;
  const references = entry.reference
    ? [entry.reference]
    : mapping && "objects" in mapping
      ? mapping.objects
      : mapping && "reference" in mapping && mapping.reference
        ? [mapping.reference]
        : [];
  const facts = [
    ["Data type", property?.type || entry.data_type],
    ["Unit", property?.unit || entry.unit],
    ["Grain", entry.grain],
    ["Time definition", property?.time_definition || entry.time_definition],
    ["Caveats", entry.caveats],
    ["Allowed values", property?.enums?.join(", ")],
    ["Minimum", property?.minimum],
    ["Maximum", property?.maximum],
  ].filter(([, value]) => value);
  return (
    <>
      {facts.length > 0 && (
        <dl className="business-facts">
          {facts.map(([label, value]) => (
            <div key={label}>
              <dt>{t(label!)}</dt>
              <dd>{value}</dd>
            </div>
          ))}
        </dl>
      )}
      {entity?.identity?.length ? (
        <p>
          <strong>{t("Identity properties")}: </strong>
          {entity.identity
            .map(
              (id) =>
                context.find(
                  (en) => en.kind === "property" && en.definition?.id === id,
                )?.name || id,
            )
            .join(" + ")}
        </p>
      ) : null}
      {!!entry.ancestors?.length && (
        <p>
          <strong>{t("Inheritance")}: </strong>
          {entry.ancestors.map((a) => a.name).join(" → ")}
        </p>
      )}
      {property && (
        <p>
          {property.required ? t("Required") : t("Optional")} ·{" "}
          {property.multiple ? t("Multiple values") : t("Single value")}
          {property.unique ? ` · ${t("Declared unique")}` : ""}
        </p>
      )}
      {relation && (
        <p>
          <strong>{t("Endpoints")}: </strong>
          {relation.from} {relation.directed ? "→" : "↔"} {relation.to} ·{" "}
          {relation.from_cardinality.min}..
          {relation.from_cardinality.max ?? "*"} / {relation.to_cardinality.min}
          ..{relation.to_cardinality.max ?? "*"}
        </p>
      )}
      {(references.length > 0 || mapping) && (
        <section className="definition-mapping">
          <h3>{t("Mapping in this data source")}</h3>
          {references.map((ref) => (
            <p key={referenceName(ref)}>
              <code>{referenceName(ref)}</code>
            </p>
          ))}
          {mapping &&
            "fields" in mapping &&
            mapping.fields?.map((pair, i) => (
              <p key={i}>
                <code>{referenceName(pair.from)}</code> →{" "}
                <code>{referenceName(pair.to)}</code>
              </p>
            ))}
          {mapping && "template_id" in mapping && mapping.template_id && (
            <p>
              {t("Computed by query template")}:{" "}
              <code>{mapping.template_id}</code>
            </p>
          )}
          {mapping?.description && <p>{mapping.description}</p>}
          {mapping?.verification_status && (
            <p className="help">
              {mapping.verification_status === "verified"
                ? t("Discovered in metadata")
                : mapping.verification_status === "query_template"
                  ? t("Computed by query template")
                  : mapping.verification_status === "unverified"
                    ? t("Administrator declared · unverified")
                    : t("Structure needs checking")}
            </p>
          )}
        </section>
      )}
      {entity && (
        <details className="definition-properties" open>
          <summary>
            {t("Mapped properties and relationships")} ({context.length})
          </summary>
          {context.length ? (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>{t("Name")}</th>
                    <th>{t("Definition")}</th>
                    <th>{t("Physical mapping / template")}</th>
                  </tr>
                </thead>
                <tbody>
                  {context.map((en) => {
                    const m = en.mapping;
                    const prop =
                      en.kind === "property"
                        ? (en.definition as Property)
                        : undefined;
                    return (
                      <tr key={en.id}>
                        <td>
                          <strong>{en.name}</strong>
                          <small className="block">
                            {prop ? t(prop.type) : t("Relationship")}
                          </small>
                        </td>
                        <td>{en.description}</td>
                        <td>
                          {m && "reference" in m && m.reference ? (
                            <code>{referenceName(m.reference)}</code>
                          ) : m && "template_id" in m && m.template_id ? (
                            m.template_id
                          ) : (
                            t("Field relationship")
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="help">
              {t("No mapped properties or relationships in this source.")}
            </p>
          )}
          {limited && (
            <p className="help">
              {t(
                "Showing a bounded subset. Open the source mapping to see all fields.",
              )}
            </p>
          )}
        </details>
      )}
      {entry.enums && Object.keys(entry.enums).length > 0 && (
        <dl className="business-facts">
          {Object.entries(entry.enums).map(([key, meaning]) => (
            <div key={key}>
              <dt>{key}</dt>
              <dd>{meaning}</dd>
            </div>
          ))}
        </dl>
      )}
      {(entity || property || relation) && (
        <p className="help">
          {t(
            "Identity, required values and cardinality describe the business model; database values are not verified by these definitions.",
          )}
        </p>
      )}
    </>
  );
}
