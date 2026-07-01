import React, { useEffect, useMemo, useState } from "react";
import { FolderSearch, RefreshCw, SlidersHorizontal, Tags, Trash2 } from "lucide-react";
import { Badge, Button, Field, FormGrid, InlineNotice, Modal } from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  fetchDownloadPreferences,
  fetchDownloadResources,
  runDownloadCategoryAction,
  runDownloadTagAction,
  saveDownloadPreferences,
  type DownloadPreferences,
  type DownloadResources,
  type DownloadStatus,
  type IntegrationHealth
} from "../../lib/api";
import { demoModeEnabled, demoSeeds } from "../../lib/demo";
import {
  bandwidthInputToBytes,
  downloadPreferenceFormValid,
  downloadPreferencesChanged,
  emptyPreferenceForm,
  formatLimitSpeed,
  preferenceFormFromPreferences,
  queueLimitInputToInt,
  sameFormText,
  splitTagInput,
  uniqueDownloadResourceClients,
  type DownloadPreferenceForm
} from "./lib";

/**
 * Replaces the legacy inline resource + preference panels: per-client
 * categories, tags, and client preferences now live behind "Manage Clients".
 */
export default function ManageClientsModal(props: {
  open: boolean;
  onClose: () => void;
  downloads: DownloadStatus[];
  integrations: IntegrationHealth[];
  categoryFilter: string;
  onCategoryFilterChange: (value: string) => void;
  onTextFilterChange: (value: string) => void;
  onResourcesLoaded: (resources: DownloadResources | null) => void;
}) {
  const toast = useToast();
  const [client, setClient] = useState("qBittorrent");
  const [resources, setResources] = useState<DownloadResources | null>(
    demoModeEnabled ? demoSeeds.downloadResourcesByClient["qBittorrent"] ?? null : null
  );
  const [preferences, setPreferences] = useState<DownloadPreferences | null>(
    demoModeEnabled ? demoSeeds.downloadPreferencesByClient["qBittorrent"] ?? null : null
  );
  const [loadingResources, setLoadingResources] = useState(false);
  const [loadingPreferences, setLoadingPreferences] = useState(false);
  const [savingPreferences, setSavingPreferences] = useState(false);
  const [resourceActionID, setResourceActionID] = useState("");
  const [categoryName, setCategoryName] = useState("");
  const [categoryNewName, setCategoryNewName] = useState("");
  const [categoryPath, setCategoryPath] = useState("");
  const [tagName, setTagName] = useState("");
  const [tagNewName, setTagNewName] = useState("");
  const [prefForm, setPrefForm] = useState<DownloadPreferenceForm>(() =>
    preferences ? preferenceFormFromPreferences(preferences) : emptyPreferenceForm()
  );

  const clientOptions = useMemo(
    () => uniqueDownloadResourceClients(props.downloads, props.integrations),
    [props.downloads, props.integrations]
  );
  const isTransmission = client.toLowerCase() === "transmission";
  const isSABnzbd = client.toLowerCase() === "sabnzbd";
  const isQbittorrent = client.toLowerCase() === "qbittorrent";
  const supportsPreferences = isQbittorrent || isTransmission;
  const configured = Boolean(
    props.integrations.find((integration) => integration.name.toLowerCase() === client.toLowerCase())?.configured
  );

  const categoryMatch = (resources?.categories ?? []).find(
    (category) => category.name.toLowerCase() === categoryName.trim().toLowerCase()
  );
  const categoryHasChanges = isTransmission
    ? Boolean(categoryName.trim() && categoryNewName.trim() && !sameFormText(categoryName, categoryNewName))
    : Boolean(categoryName.trim() && (!categoryMatch || !sameFormText(categoryPath, categoryMatch.savePath)));
  const tagNames = splitTagInput(tagName);
  const tagSet = new Set((resources?.tags ?? []).map((tag) => tag.trim().toLowerCase()));
  const tagHasChanges = isTransmission
    ? tagNames.length === 1 && Boolean(tagNewName.trim()) && !sameFormText(tagNames[0], tagNewName)
    : tagNames.some((tag) => !tagSet.has(tag.toLowerCase()));
  const prefInputsValid = downloadPreferenceFormValid(prefForm, isQbittorrent);
  const prefsHaveChanges = downloadPreferencesChanged(preferences, prefForm, isQbittorrent);
  const busy = Boolean(resourceActionID);

  function hydratePreferences(next: DownloadPreferences | null) {
    setPreferences(next);
    setPrefForm(next ? preferenceFormFromPreferences(next) : emptyPreferenceForm());
  }

  async function refreshResources(silent = false) {
    if (!configured) {
      const fallback = demoModeEnabled
        ? demoSeeds.downloadResourcesByClient[client] ?? { client, categories: [], tags: [] }
        : null;
      setResources(fallback);
      props.onResourcesLoaded(fallback);
      setLoadingResources(false);
      return;
    }
    setLoadingResources(true);
    try {
      const next = await fetchDownloadResources(client);
      setResources(next);
      props.onResourcesLoaded(next);
    } catch (error) {
      const fallback = demoModeEnabled ? demoSeeds.downloadResourcesByClient[client] : undefined;
      if (fallback) {
        setResources(fallback);
        props.onResourcesLoaded(fallback);
      }
      if (!silent) {
        toast.error(error instanceof Error ? error.message : "Download resources refresh failed");
      }
    } finally {
      setLoadingResources(false);
    }
  }

  async function refreshPreferences(silent = false) {
    if (!supportsPreferences || !configured) {
      const fallback = demoModeEnabled ? demoSeeds.downloadPreferencesByClient[client] : undefined;
      hydratePreferences(fallback ?? null);
      setLoadingPreferences(false);
      return;
    }
    setLoadingPreferences(true);
    try {
      hydratePreferences(await fetchDownloadPreferences(client));
    } catch (error) {
      const fallback = demoModeEnabled ? demoSeeds.downloadPreferencesByClient[client] : undefined;
      hydratePreferences(fallback ?? null);
      if (!silent) {
        toast.error(error instanceof Error ? error.message : "Download preferences refresh failed");
      }
    } finally {
      setLoadingPreferences(false);
    }
  }

  useEffect(() => {
    if (!props.open) return;
    void refreshResources(true);
    void refreshPreferences(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.open, client, props.integrations]);

  async function saveCategory() {
    const name = categoryName.trim();
    if (!name || !categoryHasChanges) return;
    const exists = (resources?.categories ?? []).some((category) => category.name.toLowerCase() === name.toLowerCase());
    const action = isTransmission ? "edit" : exists ? "edit" : "create";
    const newName = categoryNewName.trim();
    if (isTransmission && !newName) return;
    setResourceActionID(`category:${action}:${name}`);
    try {
      const result = await runDownloadCategoryAction({
        action,
        name,
        newName: newName || undefined,
        client,
        savePath: categoryPath.trim()
      });
      if (result.resources) {
        setResources(result.resources);
        props.onResourcesLoaded(result.resources);
      } else {
        await refreshResources(true);
      }
      props.onCategoryFilterChange(newName || name);
      setCategoryNewName("");
      toast.success(isTransmission ? `Renamed category ${name} to ${newName}` : `Saved category ${name}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download category resource action failed");
    } finally {
      setResourceActionID("");
    }
  }

  async function deleteCategory(name = categoryName) {
    const category = name.trim();
    if (!category) return;
    setResourceActionID(`category:delete:${category}`);
    try {
      const result = await runDownloadCategoryAction({ action: "delete", name: category, client });
      if (result.resources) {
        setResources(result.resources);
        props.onResourcesLoaded(result.resources);
      } else {
        await refreshResources(true);
      }
      if (categoryName.trim().toLowerCase() === category.toLowerCase()) {
        setCategoryName("");
        setCategoryPath("");
      }
      if (props.categoryFilter.trim().toLowerCase() === category.toLowerCase()) {
        props.onCategoryFilterChange("");
      }
      toast.success(`Deleted category ${category}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download category delete failed");
    } finally {
      setResourceActionID("");
    }
  }

  async function createTag() {
    const names = splitTagInput(tagName);
    if (names.length === 0 || !tagHasChanges) return;
    const newName = tagNewName.trim();
    const action = isTransmission ? "edit" : "create";
    if (isTransmission && (!newName || names.length !== 1)) return;
    setResourceActionID(`tag:create:${names.join(",")}`);
    try {
      const result = await runDownloadTagAction({ action, names, newName: newName || undefined, client });
      if (result.resources) {
        setResources(result.resources);
        props.onResourcesLoaded(result.resources);
      } else {
        await refreshResources(true);
      }
      setTagName("");
      setTagNewName("");
      toast.success(isTransmission ? `Renamed tag ${names[0]} to ${newName}` : `Created ${names.length === 1 ? `tag ${names[0]}` : `${names.length} tags`}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download tag create failed");
    } finally {
      setResourceActionID("");
    }
  }

  async function deleteTag(name = tagName) {
    const names = splitTagInput(name);
    if (names.length === 0) return;
    setResourceActionID(`tag:delete:${names.join(",")}`);
    try {
      const result = await runDownloadTagAction({ action: "delete", names, client });
      if (result.resources) {
        setResources(result.resources);
        props.onResourcesLoaded(result.resources);
      } else {
        await refreshResources(true);
      }
      if (splitTagInput(tagName).some((tag) => names.includes(tag))) {
        setTagName("");
      }
      toast.success(`Deleted ${names.length === 1 ? `tag ${names[0]}` : `${names.length} tags`}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download tag delete failed");
    } finally {
      setResourceActionID("");
    }
  }

  async function savePreferences() {
    if (!supportsPreferences || !prefsHaveChanges || !prefInputsValid) return;
    setSavingPreferences(true);
    try {
      const saved = await saveDownloadPreferences({
        client,
        savePath: prefForm.savePath.trim(),
        tempPathEnabled: prefForm.tempPathEnabled,
        tempPath: prefForm.tempPath.trim(),
        startPaused: prefForm.startPaused,
        queueingEnabled: prefForm.queueingEnabled,
        speedScheduleEnabled: prefForm.speedScheduleEnabled,
        downloadLimit: bandwidthInputToBytes(prefForm.downloadLimitKiB),
        uploadLimit: bandwidthInputToBytes(prefForm.uploadLimitKiB),
        alternativeDownloadLimit: bandwidthInputToBytes(prefForm.altDownloadLimitKiB),
        alternativeUploadLimit: bandwidthInputToBytes(prefForm.altUploadLimitKiB),
        maxActiveDownloads: queueLimitInputToInt(prefForm.maxActiveDownloads),
        maxActiveUploads: queueLimitInputToInt(prefForm.maxActiveUploads),
        maxActiveTorrents: isQbittorrent ? queueLimitInputToInt(prefForm.maxActiveTorrents) : undefined
      });
      hydratePreferences(saved);
      toast.success(`Saved ${client} preferences`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download preferences update failed");
    } finally {
      setSavingPreferences(false);
    }
  }

  function updatePrefForm(changes: Partial<DownloadPreferenceForm>) {
    setPrefForm((current) => ({ ...current, ...changes }));
  }

  return (
    <Modal title="Manage Clients" open={props.open} onClose={props.onClose} wide>
      <div className="activity-modal-section">
        <FormGrid columns={2}>
          <Field label="Client">
            <select value={client} onChange={(event) => setClient(event.target.value)}>
              {clientOptions.map((option) => (
                <option value={option} key={option}>
                  {option}
                </option>
              ))}
            </select>
          </Field>
          <div className="activity-filter-scope">
            <Badge tone={configured ? "success" : "neutral"}>{configured ? "Configured" : "Not configured"}</Badge>
          </div>
        </FormGrid>
        {!configured ? (
          <InlineNotice tone="info">
            {client} is not configured. Add connection details under Settings before managing its categories, tags, or
            preferences.
          </InlineNotice>
        ) : null}
      </div>

      <div className="activity-modal-section" aria-label="Download client categories">
        <h3>Categories</h3>
        <div className="activity-detail-form">
          <Field label="Category">
            <input
              list="activity-resource-category-options"
              value={categoryName}
              onChange={(event) => setCategoryName(event.target.value)}
              placeholder="books-ebook"
            />
          </Field>
          {isTransmission ? (
            <Field label="Rename to">
              <input
                value={categoryNewName}
                onChange={(event) => setCategoryNewName(event.target.value)}
                placeholder="books-audiobook"
              />
            </Field>
          ) : null}
          <Field label="Save path" hint={isTransmission ? "Transmission categories have no save path" : undefined}>
            <input
              disabled={isTransmission}
              value={categoryPath}
              onChange={(event) => setCategoryPath(event.target.value)}
              placeholder="/data/torrents/books"
            />
          </Field>
          <datalist id="activity-resource-category-options">
            {(resources?.categories ?? []).map((category) => (
              <option value={category.name} key={category.name} />
            ))}
          </datalist>
          <Button
            size="sm"
            icon={FolderSearch}
            disabled={!configured || !categoryName.trim() || !categoryHasChanges || busy}
            onClick={() => void saveCategory()}
          >
            {isTransmission ? "Rename category" : "Save category"}
          </Button>
          <Button
            size="sm"
            variant="danger"
            icon={Trash2}
            disabled={!configured || !categoryName.trim() || busy}
            onClick={() => void deleteCategory()}
          >
            Delete category
          </Button>
          <Button
            size="sm"
            variant="ghost"
            icon={RefreshCw}
            busy={loadingResources}
            disabled={!configured || busy}
            onClick={() => void refreshResources()}
          >
            Refresh map
          </Button>
        </div>
        <div className="activity-detail-list" aria-label="Managed categories">
          {(resources?.categories ?? []).slice(0, 8).map((category) => (
            <div className="activity-detail-row" key={category.name}>
              <div className="activity-detail-row-main">
                <strong>{category.name}</strong>
                <span className="cell-muted">{category.savePath || "No save path"}</span>
              </div>
              <div className="activity-detail-row-actions">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setCategoryName(category.name);
                    setCategoryPath(category.savePath || "");
                    props.onCategoryFilterChange(category.name);
                  }}
                >
                  Use
                </Button>
                <Button size="sm" variant="danger" icon={Trash2} disabled={busy} onClick={() => void deleteCategory(category.name)}>
                  Delete
                </Button>
              </div>
            </div>
          ))}
          {(resources?.categories ?? []).length === 0 ? <span className="cell-muted">No managed categories reported.</span> : null}
        </div>
      </div>

      {!isSABnzbd ? (
        <div className="activity-modal-section" aria-label="Download client tags">
          <h3>Tags</h3>
          <div className="activity-detail-form">
            <Field label="Tags" hint="Comma separated">
              <input value={tagName} onChange={(event) => setTagName(event.target.value)} placeholder="librarry, manual" />
            </Field>
            {isTransmission ? (
              <Field label="Rename to">
                <input value={tagNewName} onChange={(event) => setTagNewName(event.target.value)} placeholder="librarry-ui" />
              </Field>
            ) : null}
            <Button
              size="sm"
              icon={Tags}
              disabled={
                !configured ||
                splitTagInput(tagName).length === 0 ||
                !tagHasChanges ||
                (isTransmission && (!tagNewName.trim() || splitTagInput(tagName).length !== 1)) ||
                busy
              }
              onClick={() => void createTag()}
            >
              {isTransmission ? "Rename tag" : "Create tag"}
            </Button>
            <Button
              size="sm"
              variant="danger"
              icon={Trash2}
              disabled={!configured || splitTagInput(tagName).length === 0 || busy}
              onClick={() => void deleteTag()}
            >
              Delete tag
            </Button>
          </div>
          <div className="activity-tag-cloud" aria-label="Managed tags">
            {(resources?.tags ?? []).slice(0, 16).map((tag) => (
              <Button
                size="sm"
                variant="ghost"
                key={tag}
                disabled={busy}
                title={`Filter queue by ${tag}`}
                onClick={() => {
                  setTagName(tag);
                  props.onTextFilterChange(tag);
                }}
              >
                {tag}
              </Button>
            ))}
            {(resources?.tags ?? []).length === 0 ? <span className="cell-muted">No managed tags reported.</span> : null}
          </div>
        </div>
      ) : null}

      {supportsPreferences && configured ? (
        <div className="activity-modal-section" aria-label={`${client} preferences`}>
          <div className="activity-details-head">
            <div>
              <h3>{client} preferences</h3>
              <span className="cell-muted">
                {preferences
                  ? `${formatLimitSpeed(preferences.downloadLimit)} down · ${formatLimitSpeed(preferences.uploadLimit)} up · ${preferences.maxActiveTorrents} active`
                  : loadingPreferences
                    ? "Loading preferences"
                    : "Preferences unavailable"}
              </span>
            </div>
            <div className="activity-detail-row-actions">
              <Button
                size="sm"
                variant="ghost"
                icon={RefreshCw}
                busy={loadingPreferences}
                disabled={savingPreferences}
                onClick={() => void refreshPreferences()}
              >
                Refresh prefs
              </Button>
              <Button
                size="sm"
                icon={SlidersHorizontal}
                busy={savingPreferences}
                disabled={loadingPreferences || !prefInputsValid || !prefsHaveChanges}
                onClick={() => void savePreferences()}
              >
                Save prefs
              </Button>
            </div>
          </div>
          <FormGrid columns={3}>
            <Field label="Save path">
              <input
                value={prefForm.savePath}
                onChange={(event) => updatePrefForm({ savePath: event.target.value })}
                placeholder="/data/torrents/books"
              />
            </Field>
            <Field label="Temp path">
              <input
                disabled={!prefForm.tempPathEnabled}
                value={prefForm.tempPath}
                onChange={(event) => updatePrefForm({ tempPath: event.target.value })}
                placeholder="/data/torrents/incomplete"
              />
            </Field>
            <Field label="Down KiB/s">
              <input
                inputMode="numeric"
                min="0"
                type="number"
                value={prefForm.downloadLimitKiB}
                onChange={(event) => updatePrefForm({ downloadLimitKiB: event.target.value })}
                placeholder="unlimited"
              />
            </Field>
            <Field label="Up KiB/s">
              <input
                inputMode="numeric"
                min="0"
                type="number"
                value={prefForm.uploadLimitKiB}
                onChange={(event) => updatePrefForm({ uploadLimitKiB: event.target.value })}
                placeholder="unlimited"
              />
            </Field>
            <Field label="Alt down KiB/s">
              <input
                inputMode="numeric"
                min="0"
                type="number"
                value={prefForm.altDownloadLimitKiB}
                onChange={(event) => updatePrefForm({ altDownloadLimitKiB: event.target.value })}
                placeholder="unlimited"
              />
            </Field>
            <Field label="Alt up KiB/s">
              <input
                inputMode="numeric"
                min="0"
                type="number"
                value={prefForm.altUploadLimitKiB}
                onChange={(event) => updatePrefForm({ altUploadLimitKiB: event.target.value })}
                placeholder="unlimited"
              />
            </Field>
            <Field label="Active downloads">
              <input
                inputMode="numeric"
                min="-1"
                type="number"
                value={prefForm.maxActiveDownloads}
                onChange={(event) => updatePrefForm({ maxActiveDownloads: event.target.value })}
              />
            </Field>
            <Field label="Active seeds">
              <input
                inputMode="numeric"
                min="-1"
                type="number"
                value={prefForm.maxActiveUploads}
                onChange={(event) => updatePrefForm({ maxActiveUploads: event.target.value })}
              />
            </Field>
            <Field label="Active total" hint={isQbittorrent ? undefined : "qBittorrent only"}>
              <input
                disabled={!isQbittorrent}
                inputMode="numeric"
                min="-1"
                type="number"
                value={prefForm.maxActiveTorrents}
                onChange={(event) => updatePrefForm({ maxActiveTorrents: event.target.value })}
              />
            </Field>
          </FormGrid>
          <div className="activity-preference-toggles">
            <label className="activity-preference-toggle">
              <input
                checked={prefForm.startPaused}
                onChange={(event) => updatePrefForm({ startPaused: event.target.checked })}
                type="checkbox"
              />
              <span>Start paused</span>
            </label>
            <label className="activity-preference-toggle">
              <input
                checked={prefForm.queueingEnabled}
                onChange={(event) => updatePrefForm({ queueingEnabled: event.target.checked })}
                type="checkbox"
              />
              <span>Queueing</span>
            </label>
            <label className="activity-preference-toggle">
              <input
                checked={prefForm.tempPathEnabled}
                onChange={(event) => updatePrefForm({ tempPathEnabled: event.target.checked })}
                type="checkbox"
              />
              <span>Temp path</span>
            </label>
            <label className="activity-preference-toggle">
              <input
                checked={prefForm.speedScheduleEnabled}
                onChange={(event) => updatePrefForm({ speedScheduleEnabled: event.target.checked })}
                type="checkbox"
              />
              <span>Schedule</span>
            </label>
          </div>
        </div>
      ) : null}
    </Modal>
  );
}
