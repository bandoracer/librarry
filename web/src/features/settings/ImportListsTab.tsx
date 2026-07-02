import React, { useState } from "react";
import { Ban, ListPlus, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
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
  createImportList,
  createImportListExclusion,
  deleteImportList,
  deleteImportListExclusion,
  syncImportList,
  updateImportList,
  type ImportList,
  type ImportListExclusion,
  type ImportListMonitor,
  type ImportListRequest
} from "../../lib/api";
import {
  m6Keys,
  useImportListExclusions,
  useImportLists,
  useInvalidatingMutation,
  useQualityProfiles,
  useRootFolders
} from "../../lib/queries";
import { formatRelativeTime } from "../../lib/format";
import { QueryErrorNotice } from "./controls";
import { errorMessage } from "./helpers";

/* --------------------------------- Form state ------------------------------ */

type ListForm = ImportListRequest & { id?: string };

function emptyListForm(defaultProfile: string): ListForm {
  return {
    name: "",
    type: "hardcover",
    enabled: true,
    settings: { listId: "" },
    monitor: "all",
    qualityProfile: defaultProfile,
    rootFolderId: undefined,
    searchOnAdd: false
  };
}

function listToForm(list: ImportList): ListForm {
  return {
    id: list.id,
    name: list.name,
    type: list.type,
    enabled: list.enabled,
    settings: { ...list.settings },
    monitor: list.monitor,
    qualityProfile: list.qualityProfile,
    rootFolderId: list.rootFolderId,
    searchOnAdd: list.searchOnAdd
  };
}

function formValid(form: ListForm): boolean {
  return Boolean(form.name.trim() && (form.settings.listId ?? "").trim() && form.qualityProfile);
}

function formPayload(form: ListForm): ImportListRequest {
  return {
    name: form.name.trim(),
    type: form.type,
    enabled: form.enabled,
    settings: { ...form.settings, listId: (form.settings.listId ?? "").trim() },
    monitor: form.monitor,
    qualityProfile: form.qualityProfile,
    rootFolderId: form.rootFolderId || undefined,
    searchOnAdd: form.searchOnAdd
  };
}

const monitorLabels: Record<ImportListMonitor, string> = {
  none: "None",
  all: "All list books"
};

/* ----------------------------------- Tab ----------------------------------- */

/**
 * Settings → Import Lists: Hardcover list/shelf sync targets plus the
 * exclusions that stop specific books from ever being re-added by any list.
 */
export function ImportListsTab() {
  const toast = useToast();
  const listsQuery = useImportLists();
  const exclusionsQuery = useImportListExclusions();
  const profilesQuery = useQualityProfiles();
  const rootFoldersQuery = useRootFolders();

  const lists = listsQuery.data ?? [];
  const exclusions = exclusionsQuery.data ?? [];
  const profiles = profilesQuery.data ?? [];
  const rootFolders = rootFoldersQuery.data ?? [];
  const defaultProfile = profiles[0]?.name ?? "standard";

  const [form, setForm] = useState<ListForm | null>(null);
  const [deleting, setDeleting] = useState<ImportList | null>(null);
  const [syncingId, setSyncingId] = useState<string | null>(null);

  const save = useInvalidatingMutation(
    (request: ListForm) =>
      request.id ? updateImportList(request.id, formPayload(request)) : createImportList(formPayload(request)),
    [m6Keys.importLists]
  );
  const remove = useInvalidatingMutation(deleteImportList, [m6Keys.importLists]);
  const toggle = useInvalidatingMutation(
    (list: ImportList) =>
      updateImportList(list.id, {
        name: list.name,
        type: list.type,
        enabled: !list.enabled,
        settings: list.settings ?? {},
        monitor: list.monitor,
        qualityProfile: list.qualityProfile,
        rootFolderId: list.rootFolderId,
        searchOnAdd: list.searchOnAdd
      }),
    [m6Keys.importLists]
  );

  function update(changes: Partial<ListForm>) {
    setForm((current) => (current ? { ...current, ...changes } : current));
  }

  async function persist() {
    if (!form) return;
    const editing = Boolean(form.id);
    try {
      const saved = await save.mutateAsync(form);
      toast.success(`Import list “${saved.name}” ${editing ? "updated" : "added"}.`);
      setForm(null);
    } catch (error) {
      toast.error(errorMessage(error, editing ? "Import list update failed" : "Import list create failed"));
    }
  }

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await remove.mutateAsync(deleting.id);
      toast.success(`Import list “${deleting.name}” deleted.`);
      setDeleting(null);
    } catch (error) {
      toast.error(errorMessage(error, "Import list delete failed"));
    }
  }

  async function toggleEnabled(list: ImportList) {
    try {
      const saved = await toggle.mutateAsync(list);
      toast.success(`Import list “${saved.name}” ${saved.enabled ? "enabled" : "disabled"}.`);
    } catch (error) {
      toast.error(errorMessage(error, "Import list update failed"));
    }
  }

  async function runSync(list: ImportList) {
    setSyncingId(list.id);
    try {
      const outcome = await syncImportList(list.id);
      if (outcome.accepted) {
        toast.notify(`Sync queued for “${list.name}” — new books land in the review flow.`, "info");
      } else {
        toast.success(outcome.message || `Sync finished for “${list.name}”.`);
      }
      await listsQuery.refetch();
    } catch (error) {
      toast.error(errorMessage(error, `Sync failed for “${list.name}”`));
    } finally {
      setSyncingId(null);
    }
  }

  function rootFolderLabel(id?: string): string {
    if (!id) return "Default";
    const folder = rootFolders.find((candidate) => candidate.id === id);
    return folder ? folder.name || folder.path : "Default";
  }

  return (
    <>
      {listsQuery.isError ? <QueryErrorNotice error={listsQuery.error} fallback="Import lists refresh failed" /> : null}
      <Card
        title="Import Lists"
        subtitle="Sync Hardcover lists and shelves into wanted books automatically."
        padded={lists.length === 0}
        actions={
          <Button size="sm" icon={Plus} onClick={() => setForm(emptyListForm(defaultProfile))}>
            Add List
          </Button>
        }
      >
        {listsQuery.isLoading ? (
          <LoadingRow label="Loading import lists…" />
        ) : lists.length ? (
          <DataTable>
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Enabled</th>
                <th>Monitor</th>
                <th>Profile</th>
                <th>Last Synced</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {lists.map((list) => (
                <tr key={list.id}>
                  <td className="cell-primary">{list.name}</td>
                  <td>
                    <Badge tone="accent">Hardcover</Badge>
                  </td>
                  <td>
                    <input
                      type="checkbox"
                      checked={list.enabled}
                      disabled={toggle.isPending}
                      onChange={() => void toggleEnabled(list)}
                      aria-label={`${list.enabled ? "Disable" : "Enable"} import list ${list.name}`}
                    />
                  </td>
                  <td className="cell-muted">{monitorLabels[list.monitor] ?? list.monitor}</td>
                  <td className="cell-muted">{list.qualityProfile}</td>
                  <td className="cell-muted">{list.lastSyncedAt ? formatRelativeTime(list.lastSyncedAt) : "never"}</td>
                  <td>
                    <div className="cell-actions">
                      <Button
                        size="sm"
                        icon={RefreshCw}
                        busy={syncingId === list.id}
                        disabled={syncingId !== null || !list.enabled}
                        title={list.enabled ? `Sync ${list.name} now` : "Enable the list to sync it"}
                        onClick={() => void runSync(list)}
                      >
                        Sync Now
                      </Button>
                      <IconButton
                        icon={Pencil}
                        size="sm"
                        label={`Edit import list ${list.name}`}
                        onClick={() => setForm(listToForm(list))}
                      />
                      <IconButton
                        icon={Trash2}
                        size="sm"
                        tone="danger"
                        label={`Delete import list ${list.name}`}
                        onClick={() => setDeleting(list)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <EmptyState
            icon={ListPlus}
            title="No import lists yet"
            actions={
              <Button size="sm" variant="primary" icon={Plus} onClick={() => setForm(emptyListForm(defaultProfile))}>
                Add List
              </Button>
            }
          >
            Import lists watch a Hardcover list or shelf and add its books as wanted items on every sync — with your
            choice of monitoring, quality profile, and root folder.
          </EmptyState>
        )}
      </Card>

      <ExclusionsCard exclusions={exclusions} query={exclusionsQuery} />

      <Modal
        title={form?.id ? "Edit Import List" : "Add Import List"}
        open={Boolean(form)}
        onClose={() => setForm(null)}
        footer={
          <>
            <Button onClick={() => setForm(null)}>Cancel</Button>
            <Button
              variant="primary"
              busy={save.isPending}
              disabled={!form || !formValid(form) || save.isPending}
              onClick={() => void persist()}
            >
              {form?.id ? "Save List" : "Add List"}
            </Button>
          </>
        }
      >
        {form ? (
          <FormGrid columns={2}>
            <Field label="Name" hint="Display name for this list.">
              <input
                value={form.name}
                onChange={(event) => update({ name: event.target.value })}
                placeholder="Want to Read"
              />
            </Field>
            <Field label="Type">
              <select value={form.type} disabled aria-label="Import list type">
                <option value="hardcover">Hardcover</option>
              </select>
            </Field>
            <div className="settings-field-wide">
              <Field
                label="Hardcover list ID"
                hint="The list or shelf identifier from its hardcover.app URL (e.g. the slug in hardcover.app/@you/lists/want-to-read). Uses the Hardcover provider token."
              >
                <input
                  value={form.settings.listId ?? ""}
                  onChange={(event) => update({ settings: { ...form.settings, listId: event.target.value } })}
                  placeholder="want-to-read"
                />
              </Field>
            </div>
            <Field label="Monitor" hint="Whether synced books are monitored for release search.">
              <select
                value={form.monitor}
                onChange={(event) => update({ monitor: event.target.value as ImportListMonitor })}
                aria-label="Monitor mode"
              >
                <option value="none">None</option>
                <option value="all">All list books</option>
              </select>
            </Field>
            <Field label="Quality profile">
              <select
                value={form.qualityProfile}
                onChange={(event) => update({ qualityProfile: event.target.value })}
                aria-label="Quality profile"
              >
                {profiles.length === 0 ? <option value={form.qualityProfile}>{form.qualityProfile}</option> : null}
                {profiles.map((profile) => (
                  <option key={`${profile.name}:${profile.mediaFormat}`} value={profile.name}>
                    {profile.name}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Root folder" hint="Where imported books land; Default uses the format's default root.">
              <select
                value={form.rootFolderId ?? ""}
                onChange={(event) => update({ rootFolderId: event.target.value || undefined })}
                aria-label="Root folder"
              >
                <option value="">Default</option>
                {rootFolders.map((folder) => (
                  <option key={folder.id} value={folder.id}>
                    {folder.name || folder.path}
                  </option>
                ))}
              </select>
            </Field>
            <div className="settings-field-wide">
              <label className="settings-check">
                <input
                  type="checkbox"
                  checked={form.searchOnAdd}
                  onChange={(event) => update({ searchOnAdd: event.target.checked })}
                />
                <span>Search for releases when a book is added</span>
              </label>
            </div>
            <div className="settings-field-wide">
              <label className="settings-check">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(event) => update({ enabled: event.target.checked })}
                />
                <span>Enabled</span>
              </label>
            </div>
          </FormGrid>
        ) : null}
      </Modal>

      <Modal
        title="Delete import list"
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
          Delete import list <strong>{deleting?.name}</strong>? Syncing stops immediately; books it already added stay
          in your library and wanted queue.
        </p>
      </Modal>
    </>
  );
}

/* -------------------------------- Exclusions ------------------------------- */

function ExclusionsCard(props: {
  exclusions: ImportListExclusion[];
  query: ReturnType<typeof useImportListExclusions>;
}) {
  const toast = useToast();
  const { exclusions, query } = props;

  const [title, setTitle] = useState("");
  const [authorName, setAuthorName] = useState("");
  const [sourceKey, setSourceKey] = useState("");

  const add = useInvalidatingMutation(createImportListExclusion, [m6Keys.importListExclusions]);
  const remove = useInvalidatingMutation(deleteImportListExclusion, [m6Keys.importListExclusions]);

  const canAdd = Boolean(title.trim() && sourceKey.trim());

  async function addExclusion() {
    if (!canAdd || add.isPending) return;
    try {
      await add.mutateAsync({ title: title.trim(), authorName: authorName.trim(), sourceKey: sourceKey.trim() });
      toast.success(`“${title.trim()}” excluded from import lists.`);
      setTitle("");
      setAuthorName("");
      setSourceKey("");
    } catch (error) {
      toast.error(errorMessage(error, "Exclusion create failed"));
    }
  }

  async function removeExclusion(exclusion: ImportListExclusion) {
    try {
      await remove.mutateAsync(exclusion.id);
      toast.success(`Exclusion for “${exclusion.title}” removed.`);
    } catch (error) {
      toast.error(errorMessage(error, "Exclusion delete failed"));
    }
  }

  return (
    <>
      {query.isError ? <QueryErrorNotice error={query.error} fallback="Import list exclusions refresh failed" /> : null}
      <Card title="List Exclusions" subtitle="Books that import lists must never re-add, even after removal.">
        {query.isLoading ? (
          <LoadingRow label="Loading exclusions…" />
        ) : exclusions.length ? (
          <DataTable>
            <thead>
              <tr>
                <th>Title</th>
                <th>Author</th>
                <th>Source Key</th>
                <th>Added</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {exclusions.map((exclusion) => (
                <tr key={exclusion.id}>
                  <td className="cell-primary">{exclusion.title}</td>
                  <td className="cell-muted">{exclusion.authorName || "—"}</td>
                  <td>
                    <code>{exclusion.sourceKey}</code>
                  </td>
                  <td className="cell-muted">{formatRelativeTime(exclusion.createdAt)}</td>
                  <td>
                    <div className="cell-actions">
                      <IconButton
                        icon={Trash2}
                        size="sm"
                        tone="danger"
                        label={`Remove exclusion for ${exclusion.title}`}
                        disabled={remove.isPending}
                        onClick={() => void removeExclusion(exclusion)}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <EmptyState icon={Ban} title="No exclusions">
            Excluded books are skipped by every import list sync. Add one below, or exclude books from the review flow
            when a list keeps re-adding something you don't want.
          </EmptyState>
        )}
        <div className="settings-mapping-add">
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            placeholder="Title (e.g. The Hobbit)"
            aria-label="Excluded book title"
          />
          <input
            value={authorName}
            onChange={(event) => setAuthorName(event.target.value)}
            placeholder="Author (optional)"
            aria-label="Excluded book author"
          />
          <input
            value={sourceKey}
            onChange={(event) => setSourceKey(event.target.value)}
            placeholder="Source key (e.g. hardcover:the-hobbit)"
            aria-label="Excluded book source key"
          />
          <Button icon={Plus} disabled={!canAdd || add.isPending} busy={add.isPending} onClick={() => void addExclusion()}>
            Add
          </Button>
        </div>
      </Card>
    </>
  );
}
