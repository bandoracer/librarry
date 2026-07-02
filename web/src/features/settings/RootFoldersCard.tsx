import React, { useState } from "react";
import { FolderTree, Pencil, Plus, Trash2 } from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  Field,
  FormGrid,
  IconButton,
  LoadingRow,
  Modal
} from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  createRootFolder,
  deleteRootFolder,
  updateRootFolder,
  type AuthorMissingBookPolicy,
  type RootFolder,
  type RootFolderCalibreSettings,
  type RootFolderRequest
} from "../../lib/api";
import { keys, useInvalidatingMutation, useQualityProfiles, useRootFolders } from "../../lib/queries";
import { formatBytes } from "../../lib/format";
import { authorMissingPolicyLabel, authorMissingPolicyOptions } from "../wanted/lib";
import { QueryErrorNotice, SecretInput } from "./controls";
import { errorMessage } from "./helpers";

/** Editable string form of the per-root Calibre settings (port as text). */
type CalibreForm = {
  enabled: boolean;
  host: string;
  port: string;
  urlBase: string;
  username: string;
  password: string;
  library: string;
  convertFormats: string;
  outputProfile: string;
  useSsl: boolean;
};

type RootFolderForm = Omit<RootFolderRequest, "calibre"> & { id?: string; calibre: CalibreForm };

function emptyCalibreForm(): CalibreForm {
  return {
    enabled: false,
    host: "",
    port: "8081",
    urlBase: "",
    username: "",
    password: "",
    library: "",
    convertFormats: "",
    outputProfile: "",
    useSsl: false
  };
}

/** Stored settings → form; the password is never echoed back (blank = keep). */
function calibreToForm(calibre?: RootFolderCalibreSettings): CalibreForm {
  if (!calibre) return emptyCalibreForm();
  return {
    enabled: calibre.enabled,
    host: calibre.host ?? "",
    port: calibre.port ? String(calibre.port) : "8081",
    urlBase: calibre.urlBase ?? "",
    username: calibre.username ?? "",
    password: "",
    library: calibre.library ?? "",
    convertFormats: calibre.convertFormats ?? "",
    outputProfile: calibre.outputProfile ?? "",
    useSsl: Boolean(calibre.useSsl)
  };
}

function calibrePayload(form: CalibreForm): RootFolderCalibreSettings {
  return {
    enabled: form.enabled,
    host: form.host.trim(),
    port: Math.max(0, Math.round(Number(form.port) || 0)),
    urlBase: form.urlBase.trim(),
    username: form.username.trim(),
    // Blank means "keep the stored password" on update (backend contract).
    password: form.password,
    library: form.library.trim(),
    convertFormats: form.convertFormats.trim(),
    outputProfile: form.outputProfile.trim(),
    useSsl: form.useSsl
  };
}

function rootFolderPayload(form: RootFolderForm): RootFolderRequest {
  return {
    name: form.name,
    path: form.path,
    mediaFormat: form.mediaFormat,
    defaultQualityProfile: form.defaultQualityProfile,
    defaultMissingBookPolicy: form.defaultMissingBookPolicy,
    defaultTags: form.defaultTags,
    isDefault: form.isDefault,
    calibre: calibrePayload(form.calibre)
  };
}

function emptyRootFolderForm(): RootFolderForm {
  return {
    name: "",
    path: "",
    mediaFormat: "ebook",
    defaultQualityProfile: "standard",
    defaultMissingBookPolicy: "all",
    defaultTags: "",
    isDefault: false,
    calibre: emptyCalibreForm()
  };
}

/**
 * Settings → Media Management: native multi-root CRUD. Each root carries a
 * format affinity plus the defaults (quality profile, missing-book policy)
 * applied when new books land under it.
 */
export function RootFoldersCard() {
  const toast = useToast();
  const query = useRootFolders();
  const profiles = useQualityProfiles();
  const folders = query.data ?? [];

  const [form, setForm] = useState<RootFolderForm | null>(null);
  const [deleting, setDeleting] = useState<RootFolder | null>(null);

  const save = useInvalidatingMutation(
    (request: RootFolderForm) =>
      request.id
        ? updateRootFolder(request.id, rootFolderPayload(request))
        : createRootFolder(rootFolderPayload(request)),
    [keys.rootFolders, keys.readiness]
  );
  const remove = useInvalidatingMutation(deleteRootFolder, [keys.rootFolders, keys.readiness]);

  function update(changes: Partial<RootFolderForm>) {
    setForm((current) => (current ? { ...current, ...changes } : current));
  }

  function updateCalibre(changes: Partial<CalibreForm>) {
    setForm((current) => (current ? { ...current, calibre: { ...current.calibre, ...changes } } : current));
  }

  function openEditor(folder?: RootFolder) {
    setForm(
      folder
        ? {
            id: folder.id,
            name: folder.name,
            path: folder.path,
            mediaFormat: folder.mediaFormat,
            defaultQualityProfile: folder.defaultQualityProfile || "standard",
            defaultMissingBookPolicy: folder.defaultMissingBookPolicy || "all",
            defaultTags: folder.defaultTags ?? "",
            isDefault: folder.isDefault,
            calibre: calibreToForm(folder.calibre)
          }
        : emptyRootFolderForm()
    );
  }

  async function persist() {
    if (!form) return;
    const editing = Boolean(form.id);
    try {
      const saved = await save.mutateAsync(form);
      toast.success(`Root folder “${saved.name || saved.path}” ${editing ? "updated" : "added"}.`);
      setForm(null);
    } catch (error) {
      toast.error(errorMessage(error, editing ? "Root folder update failed" : "Root folder create failed"));
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await remove.mutateAsync(deleting.id);
      toast.success(`Root folder “${deleting.name || deleting.path}” deleted.`);
      setDeleting(null);
    } catch (error) {
      // 409 refusals carry the reason (e.g. files still tracked under the root).
      toast.error(errorMessage(error, "Root folder delete failed"));
    }
  }

  const formValid = Boolean(form && form.name.trim() && form.path.trim());

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Root folders refresh failed" /> : null}
      <Card
        title="Root Folders"
        subtitle={`${folders.length} configured · imports pick a destination by root format and defaults`}
        padded={folders.length === 0}
        actions={
          <Button size="sm" icon={Plus} onClick={() => openEditor()}>
            Add Root Folder
          </Button>
        }
      >
        {query.isLoading ? (
          <LoadingRow label="Loading root folders…" />
        ) : folders.length ? (
          <DataTable className="settings-roots-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Path</th>
                <th>Format</th>
                <th>Defaults</th>
                <th>Free space</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {folders.map((folder) => (
                <tr key={folder.id}>
                  <td>
                    <div className="settings-root-name">
                      <span
                        className={`settings-root-dot${folder.accessible ? " ok" : " bad"}`}
                        title={folder.accessible ? "Accessible" : "Not accessible"}
                        role="img"
                        aria-label={folder.accessible ? "Accessible" : "Not accessible"}
                      />
                      <span className="cell-primary">{folder.name || folder.path}</span>
                      {folder.isDefault ? <Badge tone="accent">Default</Badge> : null}
                    </div>
                  </td>
                  <td>
                    <code className="settings-root-path" title={folder.path}>
                      {folder.path}
                    </code>
                  </td>
                  <td>
                    <Badge>{folder.mediaFormat}</Badge>
                  </td>
                  <td>
                    <span className="cell-muted">
                      {folder.defaultQualityProfile || "standard"} ·{" "}
                      {authorMissingPolicyLabel(folder.defaultMissingBookPolicy || "all")}
                    </span>{" "}
                    {folder.calibre?.enabled ? <Badge tone="info">Calibre</Badge> : null}
                  </td>
                  <td>{formatBytes(folder.freeSpaceBytes)}</td>
                  <td>
                    <div className="cell-actions">
                      <IconButton icon={Pencil} size="sm" label={`Edit root folder ${folder.name || folder.path}`} onClick={() => openEditor(folder)} />
                      <IconButton
                        icon={Trash2}
                        size="sm"
                        tone="danger"
                        label={`Delete root folder ${folder.name || folder.path}`}
                        onClick={() => setDeleting(folder)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <EmptyState
            icon={FolderTree}
            title="No root folders yet"
            actions={
              <Button size="sm" variant="primary" icon={Plus} onClick={() => openEditor()}>
                Add Root Folder
              </Button>
            }
          >
            Add a root folder to give imports a destination per format. Until roots exist, the legacy ebook and
            audiobook root fields below act as the defaults.
          </EmptyState>
        )}
      </Card>

      <Modal
        title={form?.id ? "Edit Root Folder" : "Add Root Folder"}
        open={Boolean(form)}
        onClose={() => setForm(null)}
        footer={
          <>
            <Button onClick={() => setForm(null)}>Cancel</Button>
            <Button variant="primary" busy={save.isPending} disabled={!formValid || save.isPending} onClick={() => void persist()}>
              {form?.id ? "Save Root Folder" : "Add Root Folder"}
            </Button>
          </>
        }
      >
        {form ? (
          <FormGrid columns={2}>
            <Field label="Name" hint="Display name for this root.">
              <input value={form.name} onChange={(event) => update({ name: event.target.value })} placeholder="Ebooks" />
            </Field>
            <Field label="Format" hint="Which media format imports into this root.">
              <select
                value={form.mediaFormat}
                onChange={(event) => update({ mediaFormat: event.target.value as RootFolder["mediaFormat"] })}
              >
                <option value="ebook">ebook</option>
                <option value="audiobook">audiobook</option>
              </select>
            </Field>
            <div className="settings-field-wide">
              <Field label="Path" hint="Absolute path on the server running Librarry.">
                <input
                  value={form.path}
                  onChange={(event) => update({ path: event.target.value })}
                  placeholder="/data/media/books/ebooks"
                />
              </Field>
            </div>
            <Field label="Default quality profile" hint="Applied to books added under this root.">
              <select
                value={form.defaultQualityProfile}
                onChange={(event) => update({ defaultQualityProfile: event.target.value })}
              >
                {(profiles.data ?? []).map((profile) => (
                  <option key={`${profile.name}:${profile.mediaFormat}`} value={profile.name}>
                    {profile.name} · {profile.mediaFormat}
                  </option>
                ))}
                {(profiles.data ?? []).every((profile) => profile.name !== form.defaultQualityProfile) ? (
                  <option value={form.defaultQualityProfile}>{form.defaultQualityProfile}</option>
                ) : null}
              </select>
            </Field>
            <Field label="Default missing-book policy" hint="Monitor mode for authors added under this root.">
              <select
                value={form.defaultMissingBookPolicy}
                onChange={(event) => update({ defaultMissingBookPolicy: event.target.value as AuthorMissingBookPolicy })}
              >
                {authorMissingPolicyOptions.map((policy) => (
                  <option key={policy} value={policy}>
                    {authorMissingPolicyLabel(policy)}
                  </option>
                ))}
              </select>
            </Field>
            <div className="settings-field-wide">
              <label className="settings-check">
                <input
                  type="checkbox"
                  checked={form.isDefault}
                  onChange={(event) => update({ isDefault: event.target.checked })}
                />
                <span>Default root for its format</span>
              </label>
            </div>
            <div className="settings-field-wide">
              <label className="settings-check">
                <input
                  type="checkbox"
                  checked={form.calibre.enabled}
                  onChange={(event) => updateCalibre({ enabled: event.target.checked })}
                />
                <span>Use Calibre content server for this root</span>
              </label>
            </div>
            {form.calibre.enabled ? (
              <>
                <Field label="Calibre host" hint="Hostname or IP of the Calibre content server.">
                  <input
                    value={form.calibre.host}
                    onChange={(event) => updateCalibre({ host: event.target.value })}
                    placeholder="localhost"
                  />
                </Field>
                <Field label="Port">
                  <input
                    type="number"
                    min={0}
                    value={form.calibre.port}
                    onChange={(event) => updateCalibre({ port: event.target.value })}
                    placeholder="8081"
                  />
                </Field>
                <Field label="URL base" hint="Optional path prefix when Calibre sits behind a proxy.">
                  <input
                    value={form.calibre.urlBase}
                    onChange={(event) => updateCalibre({ urlBase: event.target.value })}
                    placeholder="/calibre"
                  />
                </Field>
                <Field label="Username">
                  <input
                    value={form.calibre.username}
                    onChange={(event) => updateCalibre({ username: event.target.value })}
                    autoComplete="off"
                  />
                </Field>
                <Field
                  label="Password"
                  hint={form.id ? "Leave blank to keep the stored password." : undefined}
                >
                  <SecretInput
                    value={form.calibre.password}
                    onChange={(value) => updateCalibre({ password: value })}
                    placeholder={form.id ? "unchanged" : ""}
                    ariaLabel="Calibre password"
                  />
                </Field>
                <Field label="Library" hint="Calibre library name; blank uses the server default.">
                  <input
                    value={form.calibre.library}
                    onChange={(event) => updateCalibre({ library: event.target.value })}
                    placeholder="Calibre Library"
                  />
                </Field>
                <Field label="Convert formats" hint="Formats Calibre converts imports to (comma-separated).">
                  <input
                    value={form.calibre.convertFormats}
                    onChange={(event) => updateCalibre({ convertFormats: event.target.value })}
                    placeholder="epub, azw3"
                  />
                </Field>
                <Field label="Output profile" hint="Calibre conversion output profile.">
                  <input
                    value={form.calibre.outputProfile}
                    onChange={(event) => updateCalibre({ outputProfile: event.target.value })}
                    placeholder="tablet"
                  />
                </Field>
                <div className="settings-field-wide">
                  <label className="settings-check">
                    <input
                      type="checkbox"
                      checked={form.calibre.useSsl}
                      onChange={(event) => updateCalibre({ useSsl: event.target.checked })}
                    />
                    <span>Connect over SSL</span>
                  </label>
                </div>
              </>
            ) : null}
          </FormGrid>
        ) : null}
      </Modal>

      <Modal
        title="Delete root folder"
        open={Boolean(deleting)}
        onClose={() => setDeleting(null)}
        footer={
          <>
            <Button onClick={() => setDeleting(null)}>Cancel</Button>
            <Button variant="danger" icon={Trash2} busy={remove.isPending} onClick={() => void confirmDelete()}>
              Delete
            </Button>
          </>
        }
      >
        <p>
          Delete root folder <strong>{deleting?.name || deleting?.path}</strong>? Files on disk are not touched. The
          server refuses the delete (with a reason) while tracked files still live under it.
        </p>
      </Modal>
    </>
  );
}
