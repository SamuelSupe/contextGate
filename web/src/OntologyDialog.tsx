import { t } from "./i18n";
import { useState } from "react";
import { api, message, payload } from "./api";
import { Button, Drawer, ErrorNote, Field } from "./components";
import type { OntologyKind } from "./OntologyEditor";
import { useNavigationGuard } from "./useNavigationGuard";
import {
  emptyOntology,
  type OntologyDefinition,
  type OntologyState,
  type OntologyVersion,
} from "./ontology-types";

export function OntologyDialog({
  dialog,
  id,
  state,
  draft,
  version,
  deleteItem,
  busy,
  error,
  setError,
  setDialog,
  action,
  accept,
  save,
  reload,
  navigate,
  notify,
}: {
  dialog: string;
  id?: string;
  state: OntologyState | null;
  draft: OntologyDefinition;
  version: OntologyVersion | null;
  deleteItem: { kind: OntologyKind; id: string } | null;
  busy: boolean;
  error: string;
  setError: (error: string) => void;
  setDialog: (dialog: string) => void;
  action: (fn: () => Promise<void>) => Promise<void>;
  accept: (state: OntologyState) => void;
  save: (draft: OntologyDefinition) => Promise<void>;
  reload: () => Promise<void>;
  navigate: (path: string) => void;
  notify: (message: string) => void;
}) {
  const [importText, setImportText] = useState("");
  const [newName, setNewName] = useState("");
  const [closing, setClosing] = useState(false);
  const releaseNavigation = useNavigationGuard(!!(newName || importText), busy);
  function close() {
    if (busy) return;
    if (newName || importText) {
      setClosing(true);
      return;
    }
    setDialog("");
    setError("");
  }
  const endpoint = `/api/ontologies/${id}`;
  return (
    <Drawer
      wide={dialog === "import" || dialog === "version"}
      title={
        {
          create: t("Create ontology"),
          import: t("Import ontology JSON"),
          publish: t("Publish immutable version"),
          discard: t("Discard draft changes"),
          archive: state?.archived
            ? t("Unarchive ontology")
            : t("Archive ontology"),
          delete: t("Delete ontology"),
          "delete-item": t("Remove draft definition"),
          version: t("Ontology version {version}", {
            version: version?.version,
          }),
          "delete-version": t("Delete version {version}", {
            version: version?.version,
          }),
        }[dialog] || dialog
      }
      onClose={close}
      footer={
        <>
          <Button disabled={busy} onClick={close}>
            {dialog === "version" ? t("Close") : t("Cancel")}
          </Button>
          {dialog !== "version" && (
            <Button
              primary
              busy={busy}
              disabled={dialog === "create" && !newName.trim()}
              onClick={() =>
                action(async () => {
                  if (dialog === "create") {
                    const created = await api<OntologyState>(
                      "/api/ontologies",
                      {
                        method: "POST",
                        body: payload({
                          definition: { ...emptyOntology(), name: newName },
                        }),
                      },
                    );
                    releaseNavigation();
                    navigate(`/ontologies/${created.id}`);
                  } else if (dialog === "import") {
                    const parsed = JSON.parse(importText);
                    if (
                      !parsed.definition ||
                      !Array.isArray(parsed.definition.entities) ||
                      !Array.isArray(parsed.definition.properties) ||
                      !Array.isArray(parsed.definition.relations)
                    )
                      throw new Error(
                        t(
                          "Expected an exported ontology with definition, entities, properties and relations.",
                        ),
                      );
                    if (id && state)
                      accept(
                        await api<OntologyState>(endpoint + "/import", {
                          method: "POST",
                          body: payload({
                            revision: state.revision,
                            definition: parsed.definition,
                          }),
                        }),
                      );
                    else {
                      const created = await api<OntologyState>(
                        "/api/ontologies",
                        {
                          method: "POST",
                          body: payload({
                            id: parsed.id,
                            definition: parsed.definition,
                          }),
                        },
                      );
                      releaseNavigation();
                      navigate(`/ontologies/${created.id}`);
                    }
                  } else if (state) {
                    if (dialog === "delete-item" && deleteItem)
                      await save({
                        ...draft,
                        [deleteItem.kind]: draft[deleteItem.kind].filter(
                          (v) => v.id !== deleteItem.id,
                        ),
                      });
                    else {
                      const deleting =
                        dialog === "delete" || dialog === "delete-version";
                      const suffix =
                        dialog === "delete"
                          ? ""
                          : dialog === "delete-version"
                            ? `/versions/${version?.version}`
                            : `/${dialog}`;
                      const result = await api<OntologyState>(
                        endpoint + suffix,
                        {
                          method: deleting ? "DELETE" : "POST",
                          body: payload({
                            revision: state.revision,
                            ...(dialog === "archive"
                              ? { archived: !state.archived }
                              : {}),
                          }),
                        },
                      );
                      if (dialog === "delete") {
                        releaseNavigation();
                        navigate("/ontologies");
                      } else if (dialog === "delete-version") await reload();
                      else accept(result);
                    }
                  }
                  setDialog("");
                  notify(
                    dialog === "publish"
                      ? t(
                          "Ontology version published. Source versions remain pinned.",
                        )
                      : t("Ontology updated."),
                  );
                })
              }
            >
              {dialog === "create"
                ? t("Create draft")
                : dialog === "import"
                  ? t("Import as draft")
                  : t("Confirm")}
            </Button>
          )}
        </>
      }
    >
      <ErrorNote error={error} />
      {closing && (
        <div className="notice warning" role="alert">
          <div>
            <p>{t("You have unsaved form contents.")}</p>
            <div className="button-row">
              <Button
                autoFocus
                disabled={busy}
                onClick={() => setClosing(false)}
              >
                {t("Keep editing")}
              </Button>
              <Button
                disabled={busy}
                onClick={() => {
                  setDialog("");
                  setError("");
                }}
              >
                {t("Discard changes")}
              </Button>
            </div>
          </div>
        </div>
      )}
      {dialog === "create" ? (
        <Field label={t("Ontology name")} required>
          <input
            disabled={busy}
            autoFocus
            maxLength={256}
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
          />
        </Field>
      ) : dialog === "import" ? (
        <>
          <p>
            {t(
              "Import definitions only. Existing data source bindings stay pinned. Imported definitions require validation and publication.",
            )}
          </p>
          <Field label={t("Ontology JSON")}>
            <textarea
              disabled={busy}
              className="query-editor"
              rows={18}
              value={importText}
              onChange={(e) => setImportText(e.target.value)}
            />
          </Field>
          <Field label={t("Choose JSON file")}>
            <input
              disabled={busy}
              type="file"
              accept=".json,application/json"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) {
                  if (file.size > 600 * 1024) {
                    setError(t("Ontology file exceeds 600 KiB."));
                    return;
                  }
                  file
                    .text()
                    .then(setImportText)
                    .catch((e) => setError(message(e)));
                }
              }}
            />
          </Field>
        </>
      ) : dialog === "version" ? (
        <pre className="query-output">{JSON.stringify(version, null, 2)}</pre>
      ) : (
        <p>
          {dialog === "publish"
            ? t(
                "This creates an immutable version. Each data source must explicitly adopt it and publish its mapping. Definition validation does not validate database values.",
              )
            : dialog === "discard"
              ? t(
                  "Replace the draft with the latest published definition. Unpublished edits will be lost.",
                )
              : dialog === "archive"
                ? t(
                    "Archived ontologies reject new bindings and version adoption. Existing published bindings continue to work.",
                  )
                : dialog === "delete-item"
                  ? t(
                      "Remove this definition from the draft. References must be repaired before publication; published versions stay intact.",
                    )
                  : t(
                      "This deletion cannot be undone. Referenced versions are protected.",
                    )}
        </p>
      )}
    </Drawer>
  );
}
