import React, { useEffect, useMemo, useState } from "react";
import { CheckCircle2, FolderPen, RefreshCw } from "lucide-react";
import { Button, Card, Field, FormGrid, InlineNotice, LoadingRow } from "../../components/ui";
import { useToast } from "../../components/toast";
import { saveLibrarySettings, type LibrarySettings } from "../../lib/api";
import { keys, useInvalidatingMutation, useLibrarySettings } from "../../lib/queries";
import { QueryErrorNotice } from "./controls";
import { RootFoldersCard } from "./RootFoldersCard";
import { RenameFilesModal } from "./RenameFilesModal";
import {
  emptyLibrarySettings,
  errorMessage,
  libraryNamingPreviewPath,
  libraryNamingTokens,
  librarySettingsChanged,
  standardSearchLanguageOptions
} from "./helpers";

const namingTokensHint = `Tokens: ${libraryNamingTokens.join(", ")}.`;

/**
 * Media Management → root folders, library roots, naming templates, search
 * language, a live naming preview, the recycle-bin status, and rename tools.
 */
export function MediaTab() {
  const toast = useToast();
  const query = useLibrarySettings();
  const defaults = useMemo(() => emptyLibrarySettings(), []);
  const saved = query.data?.settings ?? defaults;
  const persisted = query.data?.persisted ?? false;

  const [form, setForm] = useState<LibrarySettings>(defaults);
  const [renameOpen, setRenameOpen] = useState(false);
  const fetched = query.data?.settings;
  useEffect(() => {
    if (fetched) setForm(fetched);
  }, [fetched]);

  const save = useInvalidatingMutation(saveLibrarySettings, [keys.librarySettings, keys.readiness]);

  const hasChanges = librarySettingsChanged(form, saved);
  const requiredFilled = Boolean(
    form.ebookLibraryRoot.trim() &&
      form.audiobookLibraryRoot.trim() &&
      form.namingAuthorFolder.trim() &&
      form.namingBookFolder.trim() &&
      form.namingFileName.trim()
  );
  const preview = useMemo(() => libraryNamingPreviewPath(form), [form]);

  function update(changes: Partial<LibrarySettings>) {
    setForm((current) => ({ ...current, ...changes }));
  }

  async function persist() {
    if (!hasChanges) return;
    try {
      const response = await save.mutateAsync({ ...form, renameBooks: form.renameBooks ?? true });
      setForm(response.settings);
      if (response.persisted) {
        toast.success("Library settings saved and applied.");
      } else {
        toast.notify("Library settings applied for this process. Add Postgres to persist them.", "warn");
      }
    } catch (error) {
      toast.error(errorMessage(error, "Library settings save failed"));
    }
  }

  async function refresh() {
    const result = await query.refetch();
    if (result.data) {
      setForm(result.data.settings);
    } else if (result.error) {
      toast.error(errorMessage(result.error, "Library settings refresh failed"));
    }
  }

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Library settings refresh failed" /> : null}
      {query.isSuccess && !persisted ? (
        <InlineNotice tone="warn">
          No database persistence — these settings apply to the running process only and reset on restart. Set
          LIBRARRY_DATABASE_URL to keep them.
        </InlineNotice>
      ) : null}
      <RootFoldersCard />
      <Card
        title="Library roots and naming"
        subtitle={`${persisted ? "Postgres" : "runtime"} · ebooks ${saved.ebookLibraryRoot || "unset"} · audio ${
          saved.audiobookLibraryRoot || "unset"
        } · search ${saved.standardSearchLanguage || "English"}`}
        actions={
          <Button size="sm" icon={FolderPen} onClick={() => setRenameOpen(true)} title="Preview file renames against the naming templates">
            Preview Rename
          </Button>
        }
      >
        {query.isLoading ? (
          <LoadingRow label="Loading library settings…" />
        ) : (
          <>
            <FormGrid columns={2}>
              <div className="settings-field-wide">
                <Field label="Ebook library root" hint="Destination root for imported ebooks.">
                  <input
                    value={form.ebookLibraryRoot}
                    onChange={(event) => update({ ebookLibraryRoot: event.target.value })}
                    placeholder="/data/media/books/ebooks"
                  />
                </Field>
              </div>
              <div className="settings-field-wide">
                <Field label="Audiobook library root" hint="Destination root for imported audiobooks.">
                  <input
                    value={form.audiobookLibraryRoot}
                    onChange={(event) => update({ audiobookLibraryRoot: event.target.value })}
                    placeholder="/data/media/books/audiobooks"
                  />
                </Field>
              </div>
              <Field label="Author folder" hint={namingTokensHint}>
                <input
                  value={form.namingAuthorFolder}
                  onChange={(event) => update({ namingAuthorFolder: event.target.value })}
                  placeholder="{Author}"
                />
              </Field>
              <Field label="Book folder" hint={`${namingTokensHint} Empty tokens collapse without dangling separators.`}>
                <input
                  value={form.namingBookFolder}
                  onChange={(event) => update({ namingBookFolder: event.target.value })}
                  placeholder="{Title}"
                />
              </Field>
              <Field label="File name" hint={`${namingTokensHint} The extension is appended when the template omits {Ext}.`}>
                <input
                  value={form.namingFileName}
                  onChange={(event) => update({ namingFileName: event.target.value })}
                  placeholder="{Title}{Ext}"
                />
              </Field>
              <Field label="Space replacement" hint="Optional — replaces spaces in generated folder and file names.">
                <input
                  value={form.namingSpaceReplacement}
                  onChange={(event) => update({ namingSpaceReplacement: event.target.value })}
                  placeholder="Optional"
                />
              </Field>
              <Field label="Rename Books" hint="Rename imported files against the naming templates.">
                <div className="settings-check">
                  <input
                    type="checkbox"
                    checked={form.renameBooks ?? true}
                    onChange={(event) => update({ renameBooks: event.target.checked })}
                  />
                  <span>{(form.renameBooks ?? true) ? "Enabled" : "Disabled"}</span>
                </div>
              </Field>
              <Field label="Search language" hint="Default language filter for standard metadata searches.">
                <select
                  value={form.standardSearchLanguage || "English"}
                  onChange={(event) => update({ standardSearchLanguage: event.target.value })}
                >
                  {standardSearchLanguageOptions.map((language) => (
                    <option key={language} value={language}>
                      {language === "Any" ? "Any language" : language}
                    </option>
                  ))}
                </select>
              </Field>
              <div className="settings-field-wide settings-naming-preview">
                <span className="field-label">Preview</span>
                <code title={preview}>{preview}</code>
              </div>
              <div className="settings-field-wide settings-naming-preview">
                <span className="field-label">Recycle bin</span>
                <code>{saved.recycleBin?.trim() || "disabled"}</code>
                <span className="field-hint">
                  Deleted and replaced library files move here instead of being removed. Configured via the
                  LIBRARRY_RECYCLE_BIN environment variable — restart the API to change it.
                </span>
              </div>
            </FormGrid>
            <div className="form-actions">
              <Button
                variant="primary"
                icon={CheckCircle2}
                busy={save.isPending}
                disabled={save.isPending || !hasChanges || !requiredFilled}
                onClick={() => void persist()}
              >
                Save library
              </Button>
              <Button icon={RefreshCw} disabled={save.isPending} onClick={() => void refresh()}>
                Refresh
              </Button>
            </div>
          </>
        )}
      </Card>
      <RenameFilesModal open={renameOpen} onClose={() => setRenameOpen(false)} />
    </>
  );
}
