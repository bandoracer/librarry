import React, { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Trash2 } from "lucide-react";
import { clearWantedOverride, deleteWanted, updateWanted } from "../../../lib/api";
import type { MetadataProvenance, WantedItem } from "../../../lib/api";
import { keys, useQualityProfiles } from "../../../lib/queries";
import { useToast } from "../../../components/toast";
import { Button, Card, Field, FormGrid, Modal } from "../../../components/ui";
import { appErrorMessage, errorMessage, profileKey, wantedEditChanged, wantedOverrideLabel } from "../lib";
import "../wanted.css";

/**
 * Metadata-correction edit form for one wanted item: title/author/cover/
 * profile fields, monitored toggle, manual-override reset chips, and delete
 * with confirmation. Extracted from the legacy BooksTab detail panel; owns
 * its own mutations and cache invalidation.
 */
export function WantedEditForm(props: { item: WantedItem; onDeleted?: () => void }) {
  const { item } = props;
  const toast = useToast();
  const client = useQueryClient();
  const profilesQuery = useQualityProfiles();

  const [editTitle, setEditTitle] = useState(item.title ?? "");
  const [editAuthor, setEditAuthor] = useState(item.authorName ?? "");
  const [editCoverURL, setEditCoverURL] = useState(item.coverUrl ?? "");
  const [editQualityProfile, setEditQualityProfile] = useState(item.qualityProfile ?? "standard");
  const [editMonitored, setEditMonitored] = useState(item.monitored ?? true);
  const [isSaving, setIsSaving] = useState(false);
  const [isRemoving, setIsRemoving] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [clearingOverrideField, setClearingOverrideField] = useState("");

  // Reset the form when the item (or its server values) change, e.g. after a
  // provenance correction refreshes the wanted list.
  useEffect(() => {
    setEditTitle(item.title ?? "");
    setEditAuthor(item.authorName ?? "");
    setEditCoverURL(item.coverUrl ?? "");
    setEditQualityProfile(item.qualityProfile ?? "standard");
    setEditMonitored(item.monitored ?? true);
  }, [item.id, item.title, item.authorName, item.coverUrl, item.qualityProfile, item.monitored]);

  const qualityProfiles = profilesQuery.data ?? [];
  const selectedProfiles = qualityProfiles.filter(
    (profile) => profile.mediaFormat === "any" || profile.mediaFormat === item.format
  );

  const editHasChanges = wantedEditChanged(item, {
    title: editTitle,
    authorName: editAuthor,
    coverUrl: editCoverURL,
    qualityProfile: editQualityProfile,
    monitored: editMonitored
  });

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  function patchMetadataCache(updated: WantedItem) {
    client.setQueryData<MetadataProvenance | undefined>(keys.wantedMetadata(item.id), (current) =>
      current ? { ...current, wantedItem: updated, manualOverrides: updated.manualOverrides ?? [] } : current
    );
  }

  async function saveEdit() {
    if (!editTitle.trim() || !editHasChanges) return;
    setIsSaving(true);
    try {
      const updated = await updateWanted(item.id, {
        title: editTitle.trim(),
        authorName: editAuthor.trim(),
        coverUrl: editCoverURL.trim(),
        qualityProfile: editQualityProfile.trim() || "standard",
        monitored: editMonitored
      });
      patchMetadataCache(updated);
      toast.success(`Saved correction for “${updated.title}”`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted update failed")));
    } finally {
      setIsSaving(false);
    }
  }

  async function removeItem() {
    setIsRemoving(true);
    try {
      await deleteWanted(item.id);
      setConfirmingDelete(false);
      client.removeQueries({ queryKey: keys.wantedMetadata(item.id) });
      client.removeQueries({ queryKey: keys.wantedReleases(item.id) });
      toast.success(`Removed “${item.title}” from wanted`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
      props.onDeleted?.();
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted remove failed")));
    } finally {
      setIsRemoving(false);
    }
  }

  async function clearOverride(fieldName: string) {
    if (!fieldName) return;
    setClearingOverrideField(fieldName);
    try {
      const updated = await clearWantedOverride(item.id, fieldName);
      patchMetadataCache(updated);
      toast.success(`Cleared ${wantedOverrideLabel(fieldName)} override`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted override reset failed")));
    } finally {
      setClearingOverrideField("");
    }
  }

  return (
    <>
      <Card
        title="Metadata correction"
        subtitle={`${item.sourceProvider || "manual"} · ${item.format} · ${item.sourceKey || item.id}`}
        actions={
          <label className="wanted-monitor-toggle">
            <input checked={editMonitored} onChange={(event) => setEditMonitored(event.target.checked)} type="checkbox" />
            <span>Monitored</span>
          </label>
        }
      >
        {item.manualOverrides?.length ? (
          <div className="wanted-override-list" aria-label="Manual metadata overrides">
            {item.manualOverrides.map((override) => (
              <button
                className="wanted-override-chip"
                disabled={clearingOverrideField === override.fieldName}
                key={override.fieldName}
                onClick={() => void clearOverride(override.fieldName)}
                title={`Clear ${wantedOverrideLabel(override.fieldName)} override`}
                type="button"
              >
                <span>
                  <strong>{wantedOverrideLabel(override.fieldName)}</strong>
                  <small>{override.value || "protected"}</small>
                </span>
                <em>{clearingOverrideField === override.fieldName ? "Clearing" : "Reset"}</em>
              </button>
            ))}
          </div>
        ) : null}
        <FormGrid columns={2}>
          <Field label="Title">
            <input value={editTitle} onChange={(event) => setEditTitle(event.target.value)} placeholder="Book title" />
          </Field>
          <Field label="Author">
            <input value={editAuthor} onChange={(event) => setEditAuthor(event.target.value)} placeholder="Author name" />
          </Field>
          <Field label="Cover URL">
            <input
              value={editCoverURL}
              onChange={(event) => setEditCoverURL(event.target.value)}
              placeholder="https://covers.example/book.jpg"
            />
          </Field>
          <Field label="Quality profile">
            <select value={editQualityProfile} onChange={(event) => setEditQualityProfile(event.target.value)}>
              {selectedProfiles.length ? (
                selectedProfiles.map((profile) => (
                  <option key={profileKey(profile)} value={profile.name}>
                    {profile.name} · {profile.mediaFormat}
                  </option>
                ))
              ) : (
                <option value={editQualityProfile || "standard"}>{editQualityProfile || "standard"}</option>
              )}
            </select>
          </Field>
        </FormGrid>
        <div className="form-actions">
          <Button
            variant="primary"
            icon={CheckCircle2}
            disabled={!editTitle.trim() || !editHasChanges}
            busy={isSaving}
            onClick={() => void saveEdit()}
          >
            {isSaving ? "Saving" : "Save correction"}
          </Button>
          <Button variant="danger" icon={Trash2} disabled={isRemoving} onClick={() => setConfirmingDelete(true)}>
            Remove wanted
          </Button>
        </div>
      </Card>

      <Modal
        title="Remove wanted item"
        open={confirmingDelete}
        onClose={() => setConfirmingDelete(false)}
        footer={
          <>
            <Button onClick={() => setConfirmingDelete(false)}>Cancel</Button>
            <Button variant="danger" icon={Trash2} busy={isRemoving} onClick={() => void removeItem()}>
              {isRemoving ? "Removing" : "Remove"}
            </Button>
          </>
        }
      >
        <p>
          Remove <strong>{item.title}</strong> from the wanted queue? Stored release decisions and metadata provenance
          for it are discarded.
        </p>
      </Modal>
    </>
  );
}
