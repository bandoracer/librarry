import React, { useState } from "react";
import { Search, UploadCloud } from "lucide-react";
import { Badge, Button, Card, DataTable, Field, FormGrid, InlineNotice, Modal } from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  previewReadarrImport,
  runReadarrImport,
  type ReadarrImportOutcome,
  type ReadarrImportSettings
} from "../../lib/api";
import { keys, useInvalidatingMutation } from "../../lib/queries";
import { SecretInput } from "./controls";
import {
  emptyReadarrImportSettings,
  errorMessage,
  readarrImportCount,
  readarrImportImported,
  readarrImportItemLabel,
  readarrImportSectionLabel
} from "./helpers";

type ReadarrScopeKey =
  | "importAuthors"
  | "importBooks"
  | "importQualityProfiles"
  | "importRootFolders"
  | "importBookFiles"
  | "importTags"
  | "importLists"
  | "importListExclusions"
  | "importConfigResources";

const scopeOptions: { key: ReadarrScopeKey; label: string }[] = [
  { key: "importAuthors", label: "Authors" },
  { key: "importBooks", label: "Books" },
  { key: "importQualityProfiles", label: "Quality profiles" },
  { key: "importRootFolders", label: "Root folders" },
  { key: "importBookFiles", label: "Book files" },
  { key: "importTags", label: "Tags" },
  { key: "importLists", label: "Import lists" },
  { key: "importListExclusions", label: "List exclusions" },
  { key: "importConfigResources", label: "Config resources" }
];

/**
 * Import → Readarr migration. Preview is a dry run; Import writes to the
 * database and therefore sits behind a confirmation modal.
 */
export function ImportTab() {
  const toast = useToast();
  const [form, setForm] = useState<ReadarrImportSettings>(() => emptyReadarrImportSettings());
  const [outcome, setOutcome] = useState<ReadarrImportOutcome | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const preview = useInvalidatingMutation(previewReadarrImport, []);
  const run = useInvalidatingMutation(runReadarrImport, [keys.qualityProfiles, keys.wanted, keys.authorSubscriptions, keys.readiness]);

  const busy = preview.isPending || run.isPending;
  const canSubmit = Boolean(form.baseUrl.trim()) && !busy;

  function update(changes: Partial<ReadarrImportSettings>) {
    setForm((current) => ({ ...current, ...changes }));
  }

  async function handlePreview() {
    if (!canSubmit) return;
    try {
      const result = await preview.mutateAsync(form);
      setOutcome(result);
      toast.success(`Readarr preview found ${readarrImportCount(result)} importable records.`);
    } catch (error) {
      toast.error(errorMessage(error, "Readarr import preview failed"));
    }
  }

  async function handleImport() {
    setConfirmOpen(false);
    if (!form.baseUrl.trim()) return;
    try {
      const result = await run.mutateAsync(form);
      setOutcome(result);
      toast.success(
        `Readarr import ${result.status === "partial" ? "partially completed" : "completed"}: ${readarrImportImported(result)} records written.`
      );
    } catch (error) {
      toast.error(errorMessage(error, "Readarr import failed"));
    }
  }

  return (
    <>
      <Card
        title="Readarr migration"
        subtitle={
          outcome
            ? `${readarrImportCount(outcome)} records found · ${readarrImportImported(outcome)} written`
            : "Preview and import an existing Readarr instance"
        }
      >
        <FormGrid columns={2}>
          <div className="settings-field-wide">
            <Field label="Readarr URL" hint="Base URL of the Readarr instance to migrate from.">
              <input
                value={form.baseUrl}
                onChange={(event) => update({ baseUrl: event.target.value })}
                placeholder="http://readarr:8787"
              />
            </Field>
          </div>
          <Field label="Readarr API key" hint="Found under Settings → General in Readarr.">
            <SecretInput
              value={form.apiKey}
              onChange={(value) => update({ apiKey: value })}
              placeholder="API key"
              ariaLabel="Readarr API key"
            />
          </Field>
          <div className="settings-field-wide">
            <Field label="Import scope" hint="Resources copied from Readarr into Librarry.">
              <div className="settings-scope-grid">
                {scopeOptions.map((option) => (
                  <label className="settings-scope-option" key={option.key}>
                    <input
                      type="checkbox"
                      checked={form[option.key]}
                      onChange={(event) => setForm((current) => ({ ...current, [option.key]: event.target.checked }))}
                    />
                    <span>{option.label}</span>
                  </label>
                ))}
              </div>
            </Field>
          </div>
        </FormGrid>
        <div className="form-actions">
          <Button icon={Search} busy={preview.isPending} disabled={!canSubmit} onClick={() => void handlePreview()}>
            Preview
          </Button>
          <Button variant="primary" icon={UploadCloud} busy={run.isPending} disabled={!canSubmit} onClick={() => setConfirmOpen(true)}>
            Import
          </Button>
        </div>
      </Card>

      {outcome ? (
        <Card
          title="Import outcome"
          subtitle={`Source ${outcome.source || "Readarr"}`}
          actions={<Badge tone={outcome.dryRun ? "info" : outcome.status === "partial" ? "warn" : "success"}>{outcome.dryRun ? "Dry run — nothing written" : outcome.status === "partial" ? "Partially applied" : "Applied"}</Badge>}
          padded={false}
        >
          {outcome.errors?.length ? (
            <div className="settings-outcome-errors">
              <InlineNotice tone="danger">{outcome.errors.slice(0, 3).join("; ")}</InlineNotice>
            </div>
          ) : null}
          <DataTable>
            <thead>
              <tr>
                <th>Section</th>
                <th>Action</th>
                <th>Found</th>
                <th>Imported</th>
                <th>Skipped</th>
                <th>Details</th>
              </tr>
            </thead>
            <tbody>
              {outcome.sections.map((section) => {
                const items = section.items ?? [];
                return (
                  <tr key={section.name}>
                    <td className="cell-primary">{readarrImportSectionLabel(section.name)}</td>
                    <td>{outcome.dryRun ? "Preview" : "Import"}</td>
                    <td>{section.count}</td>
                    <td>{section.imported}</td>
                    <td>{section.skipped}</td>
                    <td>
                      {section.errors?.length ? (
                        <div className="settings-import-errors">
                          <Badge tone="danger" title={section.errors.join("; ")}>
                            {section.errors.length} error{section.errors.length === 1 ? "" : "s"}
                          </Badge>
                          <span>{section.errors.slice(0, 2).join("; ")}</span>
                        </div>
                      ) : null}
                      {items.length ? (
                        <div className="settings-import-items">
                          {items.slice(0, 6).map((item, index) => (
                            <span className="cell-muted" key={`${section.name}-${item.id ?? index}`} title={readarrImportItemLabel(item)}>
                              {readarrImportItemLabel(item)}
                            </span>
                          ))}
                          {items.length > 6 ? <span className="cell-muted">…and {items.length - 6} more</span> : null}
                        </div>
                      ) : null}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </DataTable>
        </Card>
      ) : null}

      <Modal
        title="Run Readarr import"
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        footer={
          <>
            <Button onClick={() => setConfirmOpen(false)}>Cancel</Button>
            <Button variant="primary" icon={UploadCloud} onClick={() => void handleImport()}>
              Import
            </Button>
          </>
        }
      >
        <p>
          This writes to the database. Records from the selected scopes are created or updated in Librarry. Run Preview first for a
          dry-run summary of what will be imported.
        </p>
      </Modal>
    </>
  );
}
