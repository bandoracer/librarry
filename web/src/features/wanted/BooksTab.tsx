import React, { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  BookOpen,
  CheckCircle2,
  CheckSquare,
  FileSearch,
  RefreshCw,
  Square,
  TrendingUp
} from "lucide-react";
import {
  confirmWantedMetadataReviewCanonical,
  grabWanted,
  importCompletedDownloads,
  recoverFailedDownloads,
  runUpgradeSearch
} from "../../lib/api";
import type { AcquisitionQueueItem, MetadataReviewConfirmOutcome, WantedItem } from "../../lib/api";
import {
  keys,
  useAcquisitionQueue,
  useCutoffUnmet,
  useLibraryFiles,
  useWanted,
  useWantedMetadataReview
} from "../../lib/queries";
import { useToast } from "../../components/toast";
import { Badge, Button, Card, EmptyState, InlineNotice, LoadingRow, Segmented, StatBar } from "../../components/ui";
import { WantedEditForm } from "./components/WantedEditForm";
import { ProvenancePanel } from "./components/ProvenancePanel";
import { ReleasesPanel, useWantedReleaseSearch } from "./components/ReleasesPanel";
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
  metadataReviewBadgeLabel,
  metadataReviewMap,
  queueDownloadsForImport,
  queueDownloadsForRecovery,
  summarizeMetadataReview,
  summarizeWantedItems,
  upgradeRunSummary,
  wantedBadgeLabel,
  wantedItemSubtitle,
  wantedItemVisibleForFilter,
  wantedPresenceMap,
  wantedPresenceTone,
  wantedViewFilterLabel,
  wantedViewFilters
} from "./lib";
import type { WantedViewFilter } from "./lib";

/**
 * Books tab: wanted queue master-detail with metadata provenance, release
 * decisions, acquisition queue strip, and the metadata review bulk flow.
 * The selected-item detail is composed from the shared components in
 * ./components (edit form, provenance panel, releases panel).
 */
export function BooksTab() {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const toast = useToast();
  const client = useQueryClient();

  const wantedQuery = useWanted();
  const reviewQuery = useWantedMetadataReview();
  const queueQuery = useAcquisitionQueue();
  const filesQuery = useLibraryFiles("any");
  const releaseSearch = useWantedReleaseSearch();

  const wantedItems = useMemo(() => wantedQuery.data ?? [], [wantedQuery.data]);
  const libraryFiles = useMemo(() => filesQuery.data ?? [], [filesQuery.data]);
  const acquisitionQueue = queueQuery.data ?? null;

  /* ------------------------------ URL contract ----------------------------- */

  const urlFilter = searchParams.get("filter");
  const filter: WantedViewFilter = wantedViewFilters.includes(urlFilter as WantedViewFilter)
    ? (urlFilter as WantedViewFilter)
    : "missing";
  const isCutoffView = filter === "cutoff-unmet";
  const cutoffQuery = useCutoffUnmet(isCutoffView);
  const cutoffItems = useMemo(() => cutoffQuery.data ?? [], [cutoffQuery.data]);

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

  const [selectedReviewIDs, setSelectedReviewIDs] = useState<string[]>([]);
  const [isConfirmingReviews, setIsConfirmingReviews] = useState(false);
  const [reviewConfirmOutcome, setReviewConfirmOutcome] = useState<MetadataReviewConfirmOutcome | null>(null);

  const [acquisitionActionID, setAcquisitionActionID] = useState("");

  const [selectedUpgradeIDs, setSelectedUpgradeIDs] = useState<string[]>([]);
  const [isRunningUpgradeSelected, setIsRunningUpgradeSelected] = useState(false);

  /* ------------------------------- Derived --------------------------------- */

  const presence = useMemo(() => wantedPresenceMap(wantedItems, libraryFiles), [wantedItems, libraryFiles]);
  const reviewByID = useMemo(() => metadataReviewMap(reviewQuery.data), [reviewQuery.data]);
  const reviewSummary = useMemo(() => summarizeMetadataReview(reviewQuery.data), [reviewQuery.data]);
  const wantedSummary = useMemo(() => summarizeWantedItems(wantedItems, presence), [wantedItems, presence]);
  const wantedStatusCount = useMemo(
    () => wantedItems.filter((item) => (item.status || "").toLowerCase() === "wanted").length,
    [wantedItems]
  );

  // Cutoff Unmet swaps the list for the server-defined view; every other
  // filter derives client-side from the shared wanted list.
  const visibleItems = useMemo(
    () =>
      isCutoffView
        ? cutoffItems
        : wantedItems.filter((item) => wantedItemVisibleForFilter(item, presence.get(item.id), filter, reviewByID.has(item.id))),
    [isCutoffView, cutoffItems, wantedItems, presence, filter, reviewByID]
  );
  const selectedUpgradeSet = useMemo(() => new Set(selectedUpgradeIDs), [selectedUpgradeIDs]);
  const selectedUpgradeCount = useMemo(
    () => cutoffItems.filter((item) => selectedUpgradeSet.has(item.id)).length,
    [cutoffItems, selectedUpgradeSet]
  );
  const allUpgradesSelected = cutoffItems.length > 0 && cutoffItems.every((item) => selectedUpgradeSet.has(item.id));
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
    void releaseSearch.run(selected);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected?.id]);

  // Drop review selections that are no longer in the review queue.
  useEffect(() => {
    const available = new Set((reviewQuery.data?.items ?? []).map((item) => item.wantedItem.id));
    setSelectedReviewIDs((current) => current.filter((id) => available.has(id)));
  }, [reviewQuery.data]);

  // Drop upgrade selections that left the cutoff-unmet view.
  useEffect(() => {
    const available = new Set((cutoffQuery.data ?? []).map((item) => item.id));
    setSelectedUpgradeIDs((current) => current.filter((id) => available.has(id)));
  }, [cutoffQuery.data]);

  /* ------------------------------- Mutations -------------------------------- */

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  function selectItem(item: WantedItem) {
    setSelectedID(item.id);
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
      (outcome.items ?? []).forEach((provenance) => {
        client.setQueryData(keys.wantedMetadata(provenance.wantedItem.id), provenance);
      });
      setSelectedReviewIDs((current) => current.filter((id) => !wantedIds.includes(id)));
      toast.success(`${outcome.fieldsConfirmed} field${outcome.fieldsConfirmed === 1 ? "" : "s"} confirmed across ${outcome.itemsReviewed} item${outcome.itemsReviewed === 1 ? "" : "s"}`);
      await invalidate(keys.wanted, keys.wantedMetadataReview, keys.acquisitionQueue);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Metadata review confirmation failed")));
    } finally {
      setIsConfirmingReviews(false);
    }
  }

  function toggleUpgradeSelection(item: WantedItem) {
    setSelectedUpgradeIDs((current) =>
      current.includes(item.id) ? current.filter((id) => id !== item.id) : [...current, item.id]
    );
  }

  function toggleAllUpgrades() {
    setSelectedUpgradeIDs(allUpgradesSelected ? [] : cutoffItems.map((item) => item.id));
  }

  async function runUpgradeSearchSelected() {
    const wantedIds = cutoffItems.map((item) => item.id).filter((id) => selectedUpgradeSet.has(id));
    if (!wantedIds.length) return;
    setIsRunningUpgradeSelected(true);
    toast.notify(`Upgrade search started for ${wantedIds.length} book${wantedIds.length === 1 ? "" : "s"}…`, "info");
    try {
      const run = await runUpgradeSearch({ wantedIds, autoGrab: false, paused: true, force: true });
      toast.success(upgradeRunSummary(run));
      await invalidate(keys.wanted, keys.wantedCutoffUnmet, keys.acquisitionQueue, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Upgrade search failed")));
    } finally {
      setIsRunningUpgradeSelected(false);
    }
  }

  async function runQueueAction(item: AcquisitionQueueItem) {
    const actionID = acquisitionQueueActionID(item);
    setAcquisitionActionID(actionID);
    setSelectedID(item.wantedItem.id);
    try {
      switch (item.state) {
        case "needs_search":
          await releaseSearch.run(item.wantedItem);
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
            await invalidate(keys.wantedReleases(item.wantedItem.id), keys.acquisitionQueue);
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
    queueQuery.error ? appErrorMessage(errorMessage(queueQuery.error, "Acquisition queue refresh failed")) : "",
    isCutoffView && cutoffQuery.error ? appErrorMessage(errorMessage(cutoffQuery.error, "Cutoff unmet refresh failed")) : ""
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
          options={wantedViewFilters.map((value) => ({ value, label: wantedViewFilterLabel(value) }))}
          value={filter}
          onChange={setFilter}
        />
        {wantedQuery.isFetching && !wantedQuery.isLoading ? <LoadingRow label="Refreshing…" /> : null}
      </div>

      {isCutoffView && cutoffItems.length ? (
        <Card className="wanted-review-bulk-card">
          <div className="wanted-review-bulkbar" aria-label="Cutoff unmet bulk actions">
            <div className="wanted-review-bulkbar-text">
              <strong>
                {cutoffItems.length} book{cutoffItems.length === 1 ? "" : "s"} below quality cutoff
              </strong>
              <span>
                {selectedUpgradeCount} selected · {cutoffItems.length} shown
              </span>
            </div>
            <Button size="sm" icon={allUpgradesSelected ? CheckSquare : Square} onClick={toggleAllUpgrades}>
              {allUpgradesSelected ? "Clear shown" : "Select shown"}
            </Button>
            <Button
              size="sm"
              variant="primary"
              icon={TrendingUp}
              disabled={selectedUpgradeCount === 0}
              busy={isRunningUpgradeSelected}
              onClick={() => void runUpgradeSearchSelected()}
            >
              {isRunningUpgradeSelected ? "Searching" : "Upgrade Search Selected"}
            </Button>
          </div>
        </Card>
      ) : null}

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
          {(isCutoffView ? cutoffQuery.isLoading : wantedQuery.isLoading) ? (
            <LoadingRow label={isCutoffView ? "Loading cutoff unmet books…" : "Loading wanted items…"} />
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
                    {isCutoffView ? (
                      <label className="wanted-review-selector" title="Select for upgrade search">
                        <input
                          checked={selectedUpgradeSet.has(item.id)}
                          onChange={() => toggleUpgradeSelection(item)}
                          type="checkbox"
                          aria-label={`Select ${item.title} for upgrade search`}
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
          ) : isCutoffView ? (
            <EmptyState icon={TrendingUp} title="No books below their quality cutoff">
              Books with a tracked file scoring under their quality profile’s cutoff appear here so you can run upgrade
              searches for better releases.
            </EmptyState>
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
              <WantedEditForm item={selected} onDeleted={() => setSelectedID("")} />
              <ProvenancePanel key={`provenance-${selected.id}`} item={selected} />
              <ReleasesPanel key={`releases-${selected.id}`} item={selected} />
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
    </>
  );
}
