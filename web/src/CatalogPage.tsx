import { useState } from "react";
import { ShieldCheck, CheckCircle2, Search } from "lucide-react";
import { t } from "./i18n";
import { capabilityLabel } from "./display";
import { Button, Empty } from "./components";
import type { Capability } from "./types";

export function CatalogPage({ catalog }: { catalog: Capability[] }) {
  const [search, setSearch] = useState("");
  const shown = catalog.filter((c) =>
    (c.name + " " + c.kind).toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <>
      <div className="page-header">
        <div>
          <h1>{t("Data source types")}</h1>
          <p>
            {t("Native query capabilities, protection and tested versions")}
          </p>
        </div>
      </div>
      <div className="filters">
        <div className="search-input">
          <Search size={16} />
          <input
            placeholder={t("Search data source types")}
            aria-label={t("Search data source types")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <span className="help">
          {shown.length}
          {t(" of ")}
          {catalog.length}
          {t(" connectors")}
        </span>
      </div>
      {!shown.length ? (
        <Empty
          title={t("No matching connectors")}
          description={t("Try another name or clear the search.")}
          action={
            <Button onClick={() => setSearch("")}>{t("Clear search")}</Button>
          }
        />
      ) : (
        <div className="table-scroll">
          <table className="catalog-table">
            <thead>
              <tr>
                <th>{t("Database")}</th>
                <th>{t("Agent query tool")}</th>
                <th>{t("Pagination")}</th>
                <th>{t("Tested versions")}</th>
                <th>{t("Protection and limitations")}</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((c) => (
                <tr key={c.kind}>
                  <td className="semibold">{c.name}</td>
                  <td>
                    <code>{c.tool}</code>
                    <small className="block">
                      {c.parameters
                        ? t("Parameter binding")
                        : t("Native parameters")}
                    </small>
                  </td>
                  <td>{capabilityLabel(c.pagination)}</td>
                  <td>
                    {c.verified_versions.length ? (
                      <span className="status green">
                        <CheckCircle2 size={14} />
                        {c.verified_versions.join(", ")}
                      </span>
                    ) : (
                      <span className="muted">{t("Not verified")}</span>
                    )}
                  </td>
                  <td>
                    <span>{capabilityLabel(c.protection)}</span>
                    {c.limitations.map((l) => (
                      <small className="block" key={l}>
                        {capabilityLabel(l)}
                      </small>
                    ))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="notice neutral">
        <ShieldCheck size={19} />
        <p>
          {t(
            "Compatible products are tested individually. Engine protection and account permissions are separate checks. InfluxDB 3 Core uses query API isolation.",
          )}
        </p>
      </div>
    </>
  );
}
