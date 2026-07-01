import React, { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  BookOpen,
  CheckCircle2,
  CheckSquare,
  ExternalLink,
  FileSearch,
  RefreshCw,
  Square,
  Trash2
} from "lucide-react";
import {
  applyWantedMetadataCorrection,
  applyWantedMetadataCorrections,
  clearWantedOverride,
  confirmWantedMetadataReviewCanonical,
  deleteWanted,
  fetchWantedMetadata,
  fetchWantedReleases,
  grabWanted,
  importCompletedDownloads,
  recoverFailedDownloads,
  searchWantedReleases,
  updateWanted
} from "../../lib/api";
import type {
  AcquisitionQueueItem,
  MetadataFieldCandidate,
  MetadataFieldEvidence,
  MetadataProvenance,
  MetadataReviewConfirmOutcome,
  ProviderMetadataRecord,
  ReleaseDecision,
  WantedItem
} from "../../lib/api";
import {
  keys,
  useAcquisitionQueue,
  useLibraryFiles,
  useLibrarySettings,
  useQualityProfiles,
  useWanted,
  useWantedMetadataReview
} from "../../lib/queries";
import { demoModeEnabled, demoSeeds } from "../../lib/demo";
import { formatBytes } from "../../lib/format";
import { useToast } from "../../components/toast";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Field,
  FormGrid,
  InlineNotice,
  LoadingRow,
  Modal,
  Segmented,
  StatBar
} from "../../components/ui";
import {
  acquisitionBadgeTone,
  acquisitionQueueActionDisabled,
  acquisitionQueueActionID,
  acquisitionQueueActionIcon,
  acquisitionQueueActionLabel,
  acquisitionQueueStateLabel,
  acquisitionQueueStateTone,
  appErrorMessage,
  errorMessage,
  metadataConfidenceLabel,
  metadataFieldApplicableCandidates,
  metadataFieldCanApply,
  metadataFieldCanConfirmCanonical,
  metadataFieldCanonicalActionID,
  metadataFieldCandidateSummary,
  metadataFieldSourceLabel,
  metadataFieldStatusLabel,
  metadataProvenanceSummary,
  metadataRecordActionID,
  metadataRecordCorrections,
  metadataRecordPrimaryLine,
  metadataRecordSecondaryLine,
  metadataReviewBadgeLabel,
  metadataReviewMap,
  profileKey,
  queueDownloadsForImport,
  queueDownloadsForRecovery,
  releaseActionID,
  releaseDecisionFilters,
  releaseDecisionVisibleForFilter,
  summarizeMetadataReview,
  summarizeReleaseDecisions,
  summarizeWantedItems,
  wantedBadgeLabel,
  wantedEditChanged,
  wantedItemSubtitle,
  wantedItemVisibleForFilter,
  wantedOverrideLabel,
  wantedPresenceMap,
  wantedPresenceTone,
  wantedViewFilters
} from "./lib";
import type { ReleaseDecisionFilter, WantedViewFilter } from "./lib";

/**
 * Books tab: wanted queue master-detail with metadata provenance, release
 * decisions, acquisition queue strip, and the metadata review bulk flow.
 */
export function BooksTab() {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const toast = useToast();
  const client = useQueryClient();

  const wantedQuery = useWanted();
  const reviewQuery = useWantedMetadataReview();
  const queueQuery = useAcquisitionQueue();
  const profilesQuery = useQualityProfiles();
  const filesQuery = useLibraryFiles("any");
  const librarySettingsQuery = useLibrarySettings();

  const wantedItems = useMemo(() => wantedQuery.data ?? [], [wantedQuery.data]);
  const libraryFiles = useMemo(() => filesQuery.data ?? [], [filesQuery.data]);
  const qualityProfiles = useMemo(() => profilesQuery.data ?? [], [profilesQuery.data]);
  const acquisitionQueue = queueQuery.data ?? null;
  const searchLanguage = librarySettingsQuery.data?.settings.standardSearchLanguage || "English";

  /* ------------------------------ URL contract ----------------------------- */

  const urlFilter = searchParams.get("filter");
  const filter: WantedViewFilter = wantedViewFilters.includes(urlFilter as WantedViewFilter)
    ? (urlFilter as WantedViewFilter)
    : "missing";

  function setFilter(next: WantedViewFilter) {
    setSearchParams(
      (params) => {
        const copy = new URLSearchParams(params);
        if (next === "missing") {
          copy.delete("filter");
        } else {
          copy.set("filter", next);
        }
        return copy;
      },
      { replace: true }
    );
  }

  const [selectedID, setSelectedID] = useState(() => searchParams.get("item") ?? "");

  /* ------------------------------ Local state ------------------------------ */

  const [releases, setReleases] = useState<ReleaseDecision[]>([]);
  const [releaseFilter, setReleaseFilter] = useState<ReleaseDecisionFilter>("all");
  const [releasesError, setReleasesError] = useState("");
  const [metadata, setMetadata] = useState<MetadataProvenance | null>(null);
  const [metadataError, setMetadataError] = useState("");
  const [isLoadingMetadata, setIsLoadingMetadata] = useState(false);
  const [isLoadingReleases, setIsLoadingReleases] = useState(false);
  const [isSearchingReleases, setIsSearchingReleases] = useState(false);

  const [editTitle, setEditTitle] = useState("");
  const [editAuthor, setEditAuthor] = useState("");
  const [editCoverURL, setEditCoverURL] = useState("");
  const [editQualityProfile, setEditQualityProfile] = useState("standard");
  const [editMonitored, setEditMonitored] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isRemoving, setIsRemoving] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [clearingOverrideField, setClearingOverrideField] = useState("");

  const [applyingCandidateID, setApplyingCandidateID] = useState("");
  const [applyingRecordID, setApplyingRecordID] = useState("");
  const [selectedReviewIDs, setSelectedReviewIDs] = useState<string[]>([]);
  const [isConfirmingReviews, setIsConfirmingReviews] = useState(false);
  const [reviewConfirmOutcome, setReviewConfirmOutcome] = useState<MetadataReviewConfirmOutcome | null>(null);

  const [grabbingReleaseID, setGrabbingReleaseID] = useState("");
  const [acquisitionActionID, setAcquisitionActionID] = useState("");

  /* ------------------------------- Derived --------------------------------- */

  const presence = useMemo(() => wantedPresenceMap(wantedItems, libraryFiles), [wantedItems, libraryFiles]);
  const reviewByID = useMemo(() => metadataReviewMap(reviewQuery.data), [reviewQuery.data]);
  const reviewSummary = useMemo(() => summarizeMetadataReview(reviewQuery.data), [reviewQuery.data]);
  const wantedSummary = useMemo(() => summarizeWantedItems(wantedItems, presence), [wantedItems, presence]);
  const wantedStatusCount = useMemo(
    () => wantedItems.filter((item) => (item.status || "").toLowerCase() === "wanted").length,
    [wantedItems]
  );

  const visibleItems = useMemo(
    () => wantedItems.filter((item) => wantedItemVisibleForFilter(item, presence.get(item.id), filter, reviewByID.has(item.id))),
    [wantedItems, presence, filter, reviewByID]
  );
  const visibleReviewIDs = useMemo(
    () => visibleItems.map((item) => item.id).filter((id) => reviewByID.has(id)),
    [visibleItems, reviewByID]
  );
  const selectedReviewSet = useMemo(() => new Set(selectedReviewIDs), [selectedReviewIDs]);
  const selectedReviewCount = visibleReviewIDs.filter((id) => selectedReviewSet.has(id)).length;
  const allReviewsSelected = visibleReviewIDs.length > 0 && visibleReviewIDs.every((id) => selectedReviewSet.has(id));

  const selected = useMemo(
    () => visibleItems.find((item) => item.id === selectedID) ?? visibleItems[0],
    [visibleItems, selectedID]
  );

  const queueRows = useMemo(() => acquisitionQueue?.items ?? [], [acquisitionQueue]);
  const selectedQueueItem = useMemo(
    () => queueRows.find((item) => item.wantedItem.id === selected?.id),
    [queueRows, selected?.id]
  );
  const highlightedQueueItems = useMemo(
    () => queueRows.filter((item) => item.state !== "imported").slice(0, 6),
    [queueRows]
  );

  const selectedProfiles = useMemo(
    () => qualityProfiles.filter((profile) => !selected || profile.mediaFormat === "any" || profile.mediaFormat === selected.format),
    [qualityProfiles, selected]
  );
  const releaseSummary = useMemo(() => summarizeReleaseDecisions(releases), [releases]);
  const visibleReleases = useMemo(
    () => releases.filter((release) => releaseDecisionVisibleForFilter(release, releaseFilter)),
    [releases, releaseFilter]
  );

  const editHasChanges = wantedEditChanged(selected, {
    title: editTitle,
    authorName: editAuthor,
    coverUrl: editCoverURL,
    qualityProfile: editQualityProfile,
    monitored: editMonitored
  });

  /* -------------------------------- Effects -------------------------------- */

  // URL contract: ?item=<id> selects that item on mount (scroll into view);
  // widen the filter when the target is hidden under the default one.
  const urlItemHandled = useRef(false);
  useEffect(() => {
    if (urlItemHandled.current) return;
    const urlItem = searchParams.get("item");
    if (!urlItem) {
      urlItemHandled.current = true;
      return;
    }
    if (!wantedQuery.data) return;
    urlItemHandled.current = true;
    const target = wantedQuery.data.find((item) => item.id === urlItem);
    if (!target) return;
    setSelectedID(urlItem);
    const visible = wantedItemVisibleForFilter(target, presence.get(target.id), filter, reviewByID.has(target.id));
    if (!visible && !searchParams.get("filter")) {
      setFilter("all");
    }
    window.setTimeout(() => {
      document.getElementById(`wanted-item-${urlItem}`)?.scrollIntoView({ block: "nearest" });
    }, 60);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wantedQuery.data]);

  // URL contract: &search=1 auto-runs a release search once for the URL item.
  const autoSearchDone = useRef(false);
  useEffect(() => {
    if (autoSearchDone.current) return;
    if (searchParams.get("search") !== "1") {
      autoSearchDone.current = true;
      return;
    }
    const urlItem = searchParams.get("item");
    if (!urlItem) {
      autoSearchDone.current = true;
      return;
    }
    if (selected?.id !== urlItem) return;
    autoSearchDone.current = true;
    void runReleaseSearch(selected);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected?.id]);

  // Reset the edit form when the selected item (or its server values) change.
  useEffect(() => {
    setEditTitle(selected?.title ?? "");
    setEditAuthor(selected?.authorName ?? "");
    setEditCoverURL(selected?.coverUrl ?? "");
    setEditQualityProfile(selected?.qualityProfile ?? "standard");
    setEditMonitored(selected?.monitored ?? true);
  }, [selected?.id, selected?.title, selected?.authorName, selected?.coverUrl, selected?.qualityProfile, selected?.monitored]);

  // Load metadata provenance for the selected item (demo seeds as fallback).
  useEffect(() => {
    setMetadata(null);
    setMetadataError("");
    if (!selected?.id) return;
    const id = selected.id;
    let canceled = false;
    setIsLoadingMetadata(true);
    fetchWantedMetadata(id)
      .then((provenance) => {
        if (!canceled) setMetadata(provenance);
      })
      .catch((error: unknown) => {
        if (canceled) return;
        const seeded = demoModeEnabled ? demoSeeds.wantedMetadataByID[id] : undefined;
        if (seeded) {
          setMetadata(seeded);
          return;
        }
        setMetadataError(appErrorMessage(errorMessage(error, "Wanted metadata provenance failed")));
      })
      .finally(() => {
        if (!canceled) setIsLoadingMetadata(false);
      });
    return () => {
      canceled = true;
    };
  }, [selected?.id]);

  // Load stored release decisions for the selected item.
  useEffect(() => {
    setReleases([]);
    setReleaseFilter("all");
    setReleasesError("");
    if (!selected?.id) return;
    const id = selected.id;
    let canceled = false;
    setIsLoadingReleases(true);
    fetchWantedReleases(id)
      .then((outcome) => {
        if (!canceled) setReleases(outcome.releases);
      })
      .catch((error: unknown) => {
        if (canceled) return;
        if (demoModeEnabled) return;
        setReleasesError(appErrorMessage(errorMessage(error, "Wanted release decisions refresh failed")));
      })
      .finally(() => {
        if (!canceled) setIsLoadingReleases(false);
      });
    return () => {
      canceled = true;
    };
  }, [selected?.id]);

  // Drop review selections that are no longer in the review queue.
  useEffect(() => {
    const available = new Set((reviewQuery.data?.items ?? []).map((item) => item.wantedItem.id));
    setSelectedReviewIDs((current) => current.filter((id) => available.has(id)));
  }, [reviewQuery.data]);

  /* ------------------------------- Mutations -------------------------------- */

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  function selectItem(item: WantedItem) {
    setSelectedID(item.id);
  }

  async function runReleaseSearch(item: WantedItem | undefined = selected) {
    if (!item) return;
    setIsSearchingReleases(true);
    setReleasesError("");
    try {
      const outcome = await searchWantedReleases(item.id, searchLanguage);
      setSelectedID(item.id);
      setReleases(outcome.releases);
      setReleaseFilter("all");
      const summary = summarizeReleaseDecisions(outcome.releases);
      toast.success(`Release search: ${outcome.releases.length} found · ${summary.approved} approved · ${summary.rejected} rejected`);
      await invalidate(keys.wanted, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted release search failed")));
    } finally {
      setIsSearchingReleases(false);
    }
  }

  async function reloadStoredReleases(item: WantedItem | undefined = selected) {
    if (!item) return;
    setIsLoadingReleases(true);
    setReleasesError("");
    try {
      const outcome = await fetchWantedReleases(item.id);
      setSelectedID(item.id);
      setReleases(outcome.releases);
      await invalidate(keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted release decisions refresh failed")));
    } finally {
      setIsLoadingReleases(false);
    }
  }

  async function saveEdit() {
    const item = selected;
    if (!item || !editTitle.trim() || !editHasChanges) return;
    setIsSaving(true);
    try {
      const updated = await updateWanted(item.id, {
        title: editTitle.trim(),
        authorName: editAuthor.trim(),
        coverUrl: editCoverURL.trim(),
        qualityProfile: editQualityProfile.trim() || "standard",
        monitored: editMonitored
      });
      setMetadata((current) =>
        current ? { ...current, wantedItem: updated, manualOverrides: updated.manualOverrides ?? [] } : current
      );
      setSelectedID(updated.id);
      toast.success(`Saved correction for “${updated.title}”`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted update failed")));
    } finally {
      setIsSaving(false);
    }
  }

  async function removeSelected() {
    const item = selected;
    if (!item) return;
    setIsRemoving(true);
    try {
      await deleteWanted(item.id);
      setConfirmingDelete(false);
      setSelectedID("");
      setReleases([]);
      setMetadata(null);
      toast.success(`Removed “${item.title}” from wanted`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted remove failed")));
    } finally {
      setIsRemoving(false);
    }
  }

  async function clearOverride(fieldName: string) {
    const item = selected;
    if (!item || !fieldName) return;
    setClearingOverrideField(fieldName);
    try {
      const updated = await clearWantedOverride(item.id, fieldName);
      setMetadata((current) =>
        current ? { ...current, wantedItem: updated, manualOverrides: updated.manualOverrides ?? [] } : current
      );
      setSelectedID(updated.id);
      toast.success(`Cleared ${wantedOverrideLabel(fieldName)} override`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted override reset failed")));
    } finally {
      setClearingOverrideField("");
    }
  }

  async function applyCandidate(field: MetadataFieldEvidence, candidate: MetadataFieldCandidate) {
    const item = selected;
    if (!item || !metadataFieldCanApply(field)) return;
    const actionID = `${field.fieldName}:${candidate.provider}:${candidate.value}`;
    setApplyingCandidateID(actionID);
    try {
      const provenance = await applyWantedMetadataCorrection(item.id, {
        fieldName: field.fieldName,
        value: candidate.value
      });
      setMetadata(provenance);
      setSelectedID(provenance.wantedItem.id);
      toast.success(`Applied ${candidate.provider} ${field.label.toLowerCase()}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted metadata correction failed")));
    } finally {
      setApplyingCandidateID("");
    }
  }

  async function confirmCanonical(field: MetadataFieldEvidence) {
    const item = selected;
    if (!item || !metadataFieldCanConfirmCanonical(field)) return;
    setApplyingCandidateID(metadataFieldCanonicalActionID(field));
    try {
      const provenance = await applyWantedMetadataCorrection(item.id, {
        fieldName: field.fieldName,
        value: field.canonicalValue || "",
        reason: "metadata review canonical accepted"
      });
      setMetadata(provenance);
      setSelectedID(provenance.wantedItem.id);
      toast.success(`Kept current ${field.label.toLowerCase()}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted metadata confirmation failed")));
    } finally {
      setApplyingCandidateID("");
    }
  }

  async function applyRecord(record: ProviderMetadataRecord) {
    const item = selected;
    const corrections = metadataRecordCorrections(record, metadata);
    if (!item || corrections.length === 0) return;
    setApplyingRecordID(metadataRecordActionID(record));
    try {
      const provenance = await applyWantedMetadataCorrections(item.id, { corrections });
      setMetadata(provenance);
      setSelectedID(provenance.wantedItem.id);
      toast.success(`Applied ${corrections.length} field${corrections.length === 1 ? "" : "s"} from ${record.provider}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted metadata corrections failed")));
    } finally {
      setApplyingRecordID("");
    }
  }

  function toggleReviewSelection(item: WantedItem) {
    if (!reviewByID.has(item.id)) return;
    setSelectedReviewIDs((current) =>
      current.includes(item.id) ? current.filter((id) => id !== item.id) : [...current, item.id]
    );
  }

  function toggleAllVisibleReviews() {
    setSelectedReviewIDs((current) => {
      const visible = new Set(visibleReviewIDs);
      if (visible.size === 0) return current;
      if (visibleReviewIDs.every((id) => current.includes(id))) {
        return current.filter((id) => !visible.has(id));
      }
      const next = new Set(current);
      visibleReviewIDs.forEach((id) => next.add(id));
      return Array.from(next);
    });
  }

  async function confirmSelectedReviews() {
    const wantedIds = visibleReviewIDs.filter((id) => selectedReviewSet.has(id));
    if (wantedIds.length === 0) return;
    setIsConfirmingReviews(true);
    try {
      const outcome = await confirmWantedMetadataReviewCanonical({ wantedIds });
      setReviewConfirmOutcome(outcome);
      const provenances = outcome.items ?? [];
      const selectedProvenance = provenances.find((entry) => entry.wantedItem.id === selected?.id);
      if (selectedProvenance) {
        setMetadata(selectedProvenance);
        setSelectedID(selectedProvenance.wantedItem.id);
      }
      setSelectedReviewIDs((current) => current.filter((id) => !wantedIds.includes(id)));
      toast.success(`${outcome.fieldsConfirmed} field${outcome.fieldsConfirmed === 1 ? "" : "s"} confirmed across ${outcome.itemsReviewed} item${outcome.itemsReviewed === 1 ? "" : "s"}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Metadata review confirmation failed")));
    } finally {
      setIsConfirmingReviews(false);
    }
  }

  async function grabRelease(release?: ReleaseDecision, force = false) {
    const item = selected;
    if (!item) return;
    const actionID = release ? releaseActionID(release, force) : "auto";
    setGrabbingReleaseID(actionID);
    try {
      const status = await grabWanted(item.id, release?.id, { paused: true, force });
      toast.success(`Grab queued (paused): ${status.name || item.title}`);
      await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted grab failed")));
    } finally {
      setGrabbingReleaseID("");
    }
  }

  async function runQueueAction(item: AcquisitionQueueItem) {
    const actionID = acquisitionQueueActionID(item);
    setAcquisitionActionID(actionID);
    setSelectedID(item.wantedItem.id);
    try {
      switch (item.state) {
        case "needs_search":
          await runReleaseSearch(item.wantedItem);
          break;
        case "ready_to_grab": {
          const status = await grabWanted(item.wantedItem.id, item.bestRelease?.id, { paused: true });
          toast.success(`Grab queued (paused): ${status.name || item.wantedItem.title}`);
          await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
          break;
        }
        case "import_ready": {
          const downloadsToImport = queueDownloadsForImport(item);
          if (!downloadsToImport.length) {
            throw new Error("No completed download is ready to import for this wanted item");
          }
          const outcome = await importCompletedDownloads({
            downloadIds: downloadsToImport.map((download) => download.id).filter(Boolean),
            move: false,
            importMode: "copy",
            conflictAction: "rename",
            limit: 50
          });
          toast.success(`Import: ${outcome.imported} imported · ${outcome.reviewQueued} review · ${outcome.errored} errors`);
          await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.importReviews(), keys.history());
          break;
        }
        case "blocked": {
          const failedDownloads = queueDownloadsForRecovery(item);
          if (failedDownloads.length) {
            const run = await recoverFailedDownloads({
              downloadIds: failedDownloads.map((download) => download.id).filter(Boolean),
              autoGrab: true,
              force: true
            });
            toast.success(`Recovery: ${run.replacementsFound} replacements · ${run.grabbedCount} grabbed · ${run.errorCount} errors`);
            await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
          } else {
            await reloadStoredReleases(item.wantedItem);
          }
          break;
        }
        case "queued":
        case "downloading":
          navigate("/downloads");
          break;
        default:
          setSelectedID(item.wantedItem.id);
      }
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Queue action failed")));
    } finally {
      setAcquisitionActionID("");
    }
  }

  /* -------------------------------- Render ---------------------------------- */

  const queryNotices = [
    wantedQuery.error ? appErrorMessage(errorMessage(wantedQuery.error, "Wanted refresh failed")) : "",
    reviewQuery.error ? appErrorMessage(errorMessage(reviewQuery.error, "Wanted metadata review failed")) : "",
    queueQuery.error ? appErrorMessage(errorMessage(queueQuery.error, "Acquisition queue refresh failed")) : ""
  ].filter(Boolean);

  return (
    <>
      {queryNotices.map((notice) => (
        <InlineNotice key={notice} tone="danger">
          {notice}
        </InlineNotice>
      ))}

      <StatBar
        stats={[
          { label: "Missing", value: wantedSummary.missing, tone: "warn" },
          { label: "Review", value: reviewSummary.items, tone: "info" },
          { label: "Wanted", value: wantedStatusCount, tone: "accent" },
          { label: "Grabbed", value: wantedSummary.grabbed, tone: "success" },
          { label: "Total", value: wantedItems.length }
        ]}
      />

      <div className="wanted-filter-row">
        <Segmented<WantedViewFilter>
          ariaLabel="Wanted queue filter"
          options={wantedViewFilters.map((value) => ({ value, label: value }))}
          value={filter}
          onChange={setFilter}
        />
        {wantedQuery.isFetching && !wantedQuery.isLoading ? <LoadingRow label="Refreshing…" /> : null}
      </div>

      {filter === "review" && visibleReviewIDs.length ? (
        <Card className="wanted-review-bulk-card">
          <div className="wanted-review-bulkbar" aria-label="Metadata review bulk actions">
            <div className="wanted-review-bulkbar-text">
              <strong>{reviewSummary.conflicts} unresolved metadata conflicts</strong>
              <span>
                {selectedReviewCount} selected · {visibleReviewIDs.length} shown
                {reviewConfirmOutcome
                  ? ` · ${reviewConfirmOutcome.fieldsConfirmed} field${reviewConfirmOutcome.fieldsConfirmed === 1 ? "" : "s"} confirmed`
                  : ""}
              </span>
            </div>
            <Button size="sm" icon={allReviewsSelected ? CheckSquare : Square} onClick={toggleAllVisibleReviews}>
              {allReviewsSelected ? "Clear shown" : "Select shown"}
            </Button>
            <Button
              size="sm"
              variant="primary"
              icon={CheckCircle2}
              disabled={selectedReviewCount === 0}
              busy={isConfirmingReviews}
              onClick={() => void confirmSelectedReviews()}
            >
              {isConfirmingReviews ? "Confirming" : "Keep current"}
            </Button>
          </div>
        </Card>
      ) : null}

      <Card
        title="Acquisition queue"
        subtitle="Readarr-style next actions per wanted book"
        actions={
          <Button size="sm" icon={RefreshCw} busy={queueQuery.isFetching} onClick={() => void queueQuery.refetch()}>
            {queueQuery.isFetching ? "Refreshing" : "Refresh queue"}
          </Button>
        }
      >
        <div className="wanted-acquisition-strip" aria-label="Acquisition queue summary">
          {(
            [
              ["Search", acquisitionQueue?.summary.needsSearch ?? 0],
              ["Ready", acquisitionQueue?.summary.readyToGrab ?? 0],
              ["Queued", acquisitionQueue?.summary.queued ?? 0],
              ["Import", acquisitionQueue?.summary.importReady ?? 0],
              ["Done", acquisitionQueue?.summary.imported ?? 0],
              ["Blocked", acquisitionQueue?.summary.blocked ?? 0]
            ] as Array<[string, number]>
          ).map(([label, value]) => (
            <div className="wanted-acquisition-metric" key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
        </div>
        {selectedQueueItem
          ? (() => {
              const ActionIcon = acquisitionQueueActionIcon(selectedQueueItem.state);
              const selectedActionID = acquisitionQueueActionID(selectedQueueItem);
              return (
                <div className="wanted-acquisition-selected">
                  <span className={`wanted-state-dot ${acquisitionQueueStateTone(selectedQueueItem.state)}`} aria-hidden />
                  <div className="wanted-acquisition-selected-text">
                    <strong>{acquisitionQueueStateLabel(selectedQueueItem.state)}</strong>
                    <span>{selectedQueueItem.nextAction}</span>
                  </div>
                  <em>
                    {selectedQueueItem.approvedCount} approved · {selectedQueueItem.rejectedCount} rejected ·{" "}
                    {selectedQueueItem.downloads?.length ?? 0} queued
                  </em>
                  <Button
                    size="sm"
                    icon={ActionIcon}
                    disabled={acquisitionQueueActionDisabled(selectedQueueItem)}
                    busy={acquisitionActionID === selectedActionID}
                    onClick={() => void runQueueAction(selectedQueueItem)}
                  >
                    {acquisitionActionID === selectedActionID ? "Working" : acquisitionQueueActionLabel(selectedQueueItem)}
                  </Button>
                </div>
              );
            })()
          : null}
        {highlightedQueueItems.length ? (
          <div className="wanted-acquisition-rows" aria-label="Book acquisition actions">
            {highlightedQueueItems.map((item) => {
              const ActionIcon = acquisitionQueueActionIcon(item.state);
              const actionID = acquisitionQueueActionID(item);
              return (
                <article className="wanted-acquisition-row" key={item.wantedItem.id}>
                  <span className={`wanted-state-dot ${acquisitionQueueStateTone(item.state)}`} aria-hidden />
                  <button
                    className="wanted-acquisition-row-main"
                    onClick={() => {
                      setFilter("all");
                      selectItem(item.wantedItem);
                    }}
                    type="button"
                  >
                    <strong>{item.wantedItem.title}</strong>
                    <small>{item.wantedItem.authorName || item.wantedItem.format}</small>
                  </button>
                  <Badge tone={acquisitionBadgeTone(item.state)}>{acquisitionQueueStateLabel(item.state)}</Badge>
                  <Button
                    size="sm"
                    icon={ActionIcon}
                    disabled={acquisitionQueueActionDisabled(item)}
                    busy={acquisitionActionID === actionID}
                    onClick={() => void runQueueAction(item)}
                  >
                    {acquisitionActionID === actionID ? "Working" : acquisitionQueueActionLabel(item)}
                  </Button>
                </article>
              );
            })}
          </div>
        ) : null}
      </Card>

      <div className="wanted-grid">
        <Card padded={false} className="wanted-list-card">
          {wantedQuery.isLoading ? (
            <LoadingRow label="Loading wanted items…" />
          ) : visibleItems.length ? (
            <div className="wanted-item-list">
              {visibleItems.map((item) => {
                const itemPresence = presence.get(item.id);
                const review = reviewByID.get(item.id);
                const reviewSelectable = filter === "review" && Boolean(review);
                const reviewSelected = selectedReviewSet.has(item.id);
                const overrideCount = item.manualOverrides?.length ?? 0;
                return (
                  <div className="wanted-item-shell" key={item.id} id={`wanted-item-${item.id}`}>
                    {reviewSelectable ? (
                      <label className="wanted-review-selector" title="Select metadata review">
                        <input
                          checked={reviewSelected}
                          onChange={() => toggleReviewSelection(item)}
                          type="checkbox"
                          aria-label={`Select metadata review for ${item.title}`}
                        />
                      </label>
                    ) : null}
                    <button
                      className={item.id === selected?.id ? "wanted-item-button selected" : "wanted-item-button"}
                      onClick={() => selectItem(item)}
                      type="button"
                    >
                      {item.coverUrl ? (
                        <img
                          className="wanted-item-cover"
                          src={item.coverUrl}
                          alt=""
                          loading="lazy"
                          onError={(event) => {
                            event.currentTarget.style.visibility = "hidden";
                          }}
                        />
                      ) : (
                        <span className="wanted-item-cover-placeholder" aria-hidden>
                          <BookOpen size={16} />
                        </span>
                      )}
                      <span className="wanted-item-text">
                        <strong>{item.title}</strong>
                        <small>{wantedItemSubtitle(item, review)}</small>
                      </span>
                      <span className="wanted-item-badges">
                        <Badge tone={wantedPresenceTone(itemPresence)}>{wantedBadgeLabel(item, itemPresence)}</Badge>
                        {review ? <Badge tone="warn">{metadataReviewBadgeLabel(review)}</Badge> : null}
                        {overrideCount ? (
                          <Badge tone="accent" title={`${overrideCount} manual override${overrideCount === 1 ? "" : "s"}`}>
                            Override · {overrideCount}
                          </Badge>
                        ) : null}
                      </span>
                    </button>
                  </div>
                );
              })}
            </div>
          ) : wantedItems.length ? (
            <EmptyState icon={FileSearch} title="No items match this filter">
              Switch the wanted filter to see more of the queue.
            </EmptyState>
          ) : (
            <EmptyState icon={BookOpen} title="No wanted items">
              Mark a metadata search result wanted to start Readarr-style acquisition planning.
            </EmptyState>
          )}
        </Card>

        <div className="wanted-detail-stack">
          {selected ? (
            <>
              <Card
                title="Metadata correction"
                subtitle={`${selected.sourceProvider || "manual"} · ${selected.format} · ${selected.sourceKey || selected.id}`}
                actions={
                  <label className="wanted-monitor-toggle">
                    <input
                      checked={editMonitored}
                      onChange={(event) => setEditMonitored(event.target.checked)}
                      type="checkbox"
                    />
                    <span>Monitored</span>
                  </label>
                }
              >
                {selected.manualOverrides?.length ? (
                  <div className="wanted-override-list" aria-label="Manual metadata overrides">
                    {selected.manualOverrides.map((override) => (
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

              <Card
                title="Provider provenance"
                subtitle={metadataProvenanceSummary(metadata, isLoadingMetadata)}
                actions={
                  metadata?.manualOverrides?.length ? (
                    <Badge tone="accent">
                      {metadata.manualOverrides.length} override{metadata.manualOverrides.length === 1 ? "" : "s"} protected
                    </Badge>
                  ) : undefined
                }
              >
                {metadataError ? <InlineNotice tone="danger">{metadataError}</InlineNotice> : null}
                {metadata?.fields.length ? (
                  <div className="wanted-provenance-list" aria-label="Metadata field evidence">
                    {metadata.fields.map((field) => (
                      <article className="wanted-field-row" key={field.fieldName}>
                        <div>
                          <strong>{field.label}</strong>
                          <span>{metadataFieldSourceLabel(field)}</span>
                        </div>
                        <div>
                          <strong>{field.canonicalValue || "No canonical value"}</strong>
                          <span>{metadataFieldCandidateSummary(field)}</span>
                          {metadataFieldCanConfirmCanonical(field) || metadataFieldApplicableCandidates(field).length ? (
                            <div className="wanted-field-actions" aria-label={`${field.label} provider candidates`}>
                              {metadataFieldCanConfirmCanonical(field) ? (
                                <Button
                                  size="sm"
                                  busy={applyingCandidateID === metadataFieldCanonicalActionID(field)}
                                  onClick={() => void confirmCanonical(field)}
                                >
                                  {applyingCandidateID === metadataFieldCanonicalActionID(field) ? "Keeping" : "Keep current"}
                                </Button>
                              ) : null}
                              {metadataFieldApplicableCandidates(field).map((candidate) => {
                                const actionID = `${field.fieldName}:${candidate.provider}:${candidate.value}`;
                                return (
                                  <Button
                                    size="sm"
                                    key={`${candidate.provider}:${candidate.providerKey}:${candidate.value}`}
                                    busy={applyingCandidateID === actionID}
                                    title={candidate.value}
                                    onClick={() => void applyCandidate(field, candidate)}
                                  >
                                    {applyingCandidateID === actionID ? "Applying" : `Use ${candidate.provider}`}
                                  </Button>
                                );
                              })}
                            </div>
                          ) : null}
                        </div>
                        <Badge tone={field.conflict ? "warn" : field.protected ? "accent" : field.reviewResolved ? "success" : "neutral"}>
                          {metadataFieldStatusLabel(field)}
                        </Badge>
                      </article>
                    ))}
                  </div>
                ) : null}
                {metadata?.records.length ? (
                  <div className="wanted-provenance-list" aria-label="Provider records">
                    {metadata.records.map((record) => {
                      const corrections = metadataRecordCorrections(record, metadata);
                      const actionID = metadataRecordActionID(record);
                      return (
                        <article className="wanted-record-row" key={actionID}>
                          <div>
                            <strong>{record.provider}</strong>
                            <span>
                              {record.entityType} · {record.providerKey}
                            </span>
                          </div>
                          <div>
                            <strong>{metadataRecordPrimaryLine(record)}</strong>
                            <span>{metadataRecordSecondaryLine(record)}</span>
                          </div>
                          <div className="wanted-record-actions">
                            {corrections.length ? (
                              <Button
                                size="sm"
                                icon={CheckCircle2}
                                disabled={Boolean(applyingRecordID || applyingCandidateID) && applyingRecordID !== actionID}
                                busy={applyingRecordID === actionID}
                                title={`Apply ${corrections.length} metadata field${corrections.length === 1 ? "" : "s"} from ${record.provider}`}
                                onClick={() => void applyRecord(record)}
                              >
                                {applyingRecordID === actionID ? "Applying" : "Use record"}
                              </Button>
                            ) : null}
                            <em>{metadataConfidenceLabel(record.confidence)}</em>
                          </div>
                        </article>
                      );
                    })}
                  </div>
                ) : isLoadingMetadata ? (
                  <LoadingRow label="Loading provenance…" />
                ) : (
                  <EmptyState title="No stored provider records">
                    Create or refresh this item from metadata search to attach provider records.
                  </EmptyState>
                )}
              </Card>

              <Card
                title="Release review"
                subtitle={
                  releases.length
                    ? `${releaseSummary.approved} approved · ${releaseSummary.rejected} rejected · ${releases.length} stored`
                    : "No stored decisions for this wanted item."
                }
                actions={
                  <div className="wanted-release-toolbar">
                    <Segmented<ReleaseDecisionFilter>
                      ariaLabel="Release decision filter"
                      options={releaseDecisionFilters.map((value) => ({ value, label: value }))}
                      value={releaseFilter}
                      onChange={setReleaseFilter}
                    />
                    <Button
                      size="sm"
                      icon={RefreshCw}
                      busy={isLoadingReleases}
                      title="Reload stored release decisions"
                      onClick={() => void reloadStoredReleases()}
                    >
                      {isLoadingReleases ? "Loading" : "Stored"}
                    </Button>
                    <Button size="sm" icon={FileSearch} busy={isSearchingReleases} onClick={() => void runReleaseSearch()}>
                      {isSearchingReleases ? "Searching" : "Search Releases"}
                    </Button>
                    <Button
                      size="sm"
                      variant="primary"
                      icon={CheckCircle2}
                      busy={grabbingReleaseID === "auto"}
                      title="Grab the best approved release for this item"
                      onClick={() => void grabRelease()}
                    >
                      {grabbingReleaseID === "auto" ? "Grabbing" : "Grab Best"}
                    </Button>
                  </div>
                }
              >
                {releasesError ? <InlineNotice tone="danger">{releasesError}</InlineNotice> : null}
                {visibleReleases.length ? (
                  <div className="wanted-release-list">
                    {visibleReleases.map((release) => {
                      const grabID = releaseActionID(release, !release.approved);
                      return (
                        <article className="wanted-release-row" key={release.id || release.sourceId || release.title}>
                          <div className="wanted-release-main">
                            <div className="wanted-release-title">
                              <strong>{release.title}</strong>
                              <Badge tone={release.approved ? "success" : "danger"}>
                                {release.approved ? "Approved" : "Rejected"}
                              </Badge>
                            </div>
                            <p>
                              {release.indexer} · {release.protocol || "release"} · score {release.score.toFixed(1)} ·{" "}
                              {formatBytes(release.sizeBytes ?? 0)} · {release.seeders ?? 0} seeders · {release.leechers ?? 0}{" "}
                              leechers
                            </p>
                            {release.categories?.length ? <small>{release.categories.join(", ")}</small> : null}
                            {release.rejectedReason ? (
                              <small className="wanted-release-rejection">{release.rejectedReason}</small>
                            ) : null}
                          </div>
                          <div className="wanted-release-actions">
                            {release.infoUrl ? (
                              <a className="btn btn-secondary btn-sm" href={release.infoUrl} rel="noreferrer" target="_blank">
                                <ExternalLink size={13} aria-hidden />
                                Details
                              </a>
                            ) : null}
                            <Button
                              size="sm"
                              variant={release.approved ? "secondary" : "danger"}
                              busy={grabbingReleaseID === grabID}
                              onClick={() => void grabRelease(release, !release.approved)}
                            >
                              {grabbingReleaseID === grabID ? "Grabbing" : release.approved ? "Grab paused" : "Force grab"}
                            </Button>
                          </div>
                        </article>
                      );
                    })}
                  </div>
                ) : isLoadingReleases ? (
                  <LoadingRow label="Loading stored release decisions…" />
                ) : (
                  <EmptyState icon={FileSearch} title="No release decisions">
                    Search wanted releases to evaluate candidates for this book.
                  </EmptyState>
                )}
              </Card>
            </>
          ) : (
            <Card>
              <EmptyState icon={BookOpen} title="No wanted item selected">
                Select an item from the list to edit metadata and review releases.
              </EmptyState>
            </Card>
          )}
        </div>
      </div>

      <Modal
        title="Remove wanted item"
        open={confirmingDelete}
        onClose={() => setConfirmingDelete(false)}
        footer={
          <>
            <Button onClick={() => setConfirmingDelete(false)}>Cancel</Button>
            <Button variant="danger" icon={Trash2} busy={isRemoving} onClick={() => void removeSelected()}>
              {isRemoving ? "Removing" : "Remove"}
            </Button>
          </>
        }
      >
        <p>
          Remove <strong>{selected?.title ?? "this item"}</strong> from the wanted queue? Stored release decisions and
          metadata provenance for it are discarded.
        </p>
      </Modal>
    </>
  );
}
