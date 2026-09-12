import { useState } from "react";
import { api, message, payload } from "./api";
import { Button, Drawer, ErrorNote, Field } from "./components";
import type { OntologyKind } from "./OntologyEditor";
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
  const endpoint = `/api/ontologies/${id}`;
  return (
    <Drawer
      wide={dialog === "import" || dialog === "version"}
      title={
        {
          create: "Create ontology",
          import: "Import ontology JSON",
          publish: "Publish immutable version",
          discard: "Discard draft changes",
          archive: state?.archived ? "Unarchive ontology" : "Archive ontology",
          delete: "Delete ontology",
          "delete-item": "Remove draft definition",
          version: `Ontology version ${version?.version}`,
          "delete-version": `Delete version ${version?.version}`,
        }[dialog] || dialog
      }
      onClose={() => {
        if (!busy) {
          setDialog("");
          setError("");
        }
      }}
      footer={
        <>
          <Button
            disabled={busy}
            onClick={() => {
              setDialog("");
              setError("");
            }}
          >
            {dialog === "version" ? "Close" : "Cancel"}
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
                        "Expected an exported ontology with definition, entities, properties and relations.",
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
                      if (dialog === "delete") navigate("/ontologies");
                      else if (dialog === "delete-version") await reload();
                      else accept(result);
                    }
                  }
                  setDialog("");
                  notify(
                    dialog === "publish"
                      ? "Ontology version published. Source versions remain pinned."
                      : "Ontology updated.",
                  );
                })
              }
            >
              {dialog === "create"
                ? "Create draft"
                : dialog === "import"
                  ? "Import as draft"
                  : "Confirm"}
            </Button>
          )}
        </>
      }
    >
      <ErrorNote error={error} />
      {dialog === "create" ? (
        <Field label="Ontology name" required>
          <input
            autoFocus
            maxLength={256}
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
          />
        </Field>
      ) : dialog === "import" ? (
        <>
          <p>
            Import definitions only. Existing data source bindings stay pinned.
            Imported definitions require validation and publication.
          </p>
          <Field label="Ontology JSON">
            <textarea
              className="query-editor"
              rows={18}
              value={importText}
              onChange={(e) => setImportText(e.target.value)}
            />
          </Field>
          <Field label="Choose JSON file">
            <input
              type="file"
              accept=".json,application/json"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) {
                  if (file.size > 600 * 1024) {
                    setError("Ontology file exceeds 600 KiB.");
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
            ? "This creates an immutable version. Each data source must explicitly adopt it and publish its mapping. Definition validation does not validate database values."
            : dialog === "discard"
              ? "Replace the draft with the latest published definition. Unpublished edits will be lost."
              : dialog === "archive"
                ? "Archived ontologies reject new bindings and version adoption. Existing published bindings continue to work."
                : dialog === "delete-item"
                  ? "Remove this definition from the draft. References must be repaired before publication; published versions stay intact."
                  : "This deletion cannot be undone. Referenced versions are protected."}
        </p>
      )}
    </Drawer>
  );
}
