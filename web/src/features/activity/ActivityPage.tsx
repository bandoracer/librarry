import React, { useMemo, useState } from "react";
import { NavLink, useLocation, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  ChevronDown,
  ChevronsDown,
  ChevronsUp,
  ChevronUp,
  Download,
  FileCheck2,
  FileSearch,
  FilterX,
  HardDriveDownload,
  Inbox,
  Pause,
  Play,
  Plus,
  RefreshCw,
  Settings,
  SlidersHorizontal,
  Trash2,
  UploadCloud
} from "lucide-react";
import {
  Badge,
  Button,
  DataTable,
  EmptyState,
  Field,
  IconButton,
  InlineNotice,
  LoadingRow,
  Modal,
  PageHeader,
  ProgressBar,
  Segmented,
  StatBar,
  TabNav,
  Toolbar,
  ToolbarButton
} from "../../components/ui";
import { useToast } from "../../components/toast";
import { keys, useDownloads, useIntegrationHealth } from "../../lib/queries";
import { demoModeEnabled, demoSeeds } from "../../lib/demo";
import { formatBytes, formatPercent, formatSpeed } from "../../lib/format";
import {
  fetchDownloadDetails,
  importCompletedDownloads,
  rebalanceDownloads,
  recoverFailedDownloads,
  runDownloadAction,
  type DownloadAction,
  type DownloadDetails,
  type DownloadResources,
  type DownloadStatus
} from "../../lib/api";
import {
  boundedPositiveInt,
  downloadActionToast,
  downloadClientHealthRows,
  downloadIsImportEligible,
  downloadKey,
  downloadKeyID,
  downloadMatchesFilters,
  downloadScopeTag,
  downloadSupportsDetails,
  downloadSupportsQbitManagerActions,
  formatDuration,
  integrationBadgeTone,
  stateBadgeTone,
  summarizeDownloads,
  supportsDownloadAction,
  uniqueDownloadCategories,
  uniqueDownloadClients,
  type DownloadScope,
  type DownloadStateFilter
} from "./lib";
import AddDownloadModal from "./AddDownloadModal";
import ManageClientsModal from "./ManageClientsModal";
import DownloadDetailsPanel from "./DownloadDetailsPanel";
import HistoryTab from "./HistoryTab";
import "./activity.css";

type RemovalRequest = { ids: string[]; client?: string; label: string };

const activityTabs = [
  { label: "Queue", to: "/downloads", end: true },
  { label: "History", to: "/downloads/history" }
];

export default function ActivityPage() {
  const location = useLocation();
  const isHistory = location.pathname.replace(/\/+$/, "").endsWith("/history");

  return (
    <>
      <PageHeader title="Activity" subtitle="Download queue and history across your configured clients." />
      <TabNav
        tabs={activityTabs}
        render={(tab) => (
          <NavLink key={tab.to} to={tab.to} end={tab.end} className={({ isActive }) => (isActive ? "active" : undefined)}>
            {tab.label}
          </NavLink>
        )}
      />
      {isHistory ? <HistoryTab /> : <QueueTab />}
    </>
  );
}

function QueueTab() {
  const toast = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  /* -------------------------------- filters ------------------------------- */
  const [scope, setScope] = useState<DownloadScope>("all");
  const [clientFilter, setClientFilter] = useState("");
  const [stateFilter, setStateFilter] = useState<DownloadStateFilter>("all");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [textFilter, setTextFilter] = useState("");
  const [rebalanceMax, setRebalanceMax] = useState("3");

  /* ------------------------------- selection ------------------------------ */
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);

  /* --------------------------------- busy --------------------------------- */
  const [actionID, setActionID] = useState("");
  const [importing, setImporting] = useState(false);
  const [recovering, setRecovering] = useState(false);
  const [rebalancing, setRebalancing] = useState(false);

  /* -------------------------------- surfaces ------------------------------ */
  const [addOpen, setAddOpen] = useState(false);
  const [manageOpen, setManageOpen] = useState(false);
  const [removal, setRemoval] = useState<RemovalRequest | null>(null);
  const [expandedKey, setExpandedKey] = useState("");
  const [details, setDetails] = useState<DownloadDetails | null>(null);
  const [detailsLoading, setDetailsLoading] = useState(false);
  const [lastResources, setLastResources] = useState<DownloadResources | null>(null);

  /* --------------------------------- data --------------------------------- */
  const listOptions = useMemo(() => ({ tag: downloadScopeTag(scope) }), [scope]);
  const downloadsQuery = useDownloads(listOptions);
  const downloads = useMemo(() => downloadsQuery.data ?? [], [downloadsQuery.data]);
  const integrationsQuery = useIntegrationHealth();
  const integrations = useMemo(() => integrationsQuery.data ?? [], [integrationsQuery.data]);

  const filteredDownloads = useMemo(
    () =>
      downloads.filter((download) =>
        downloadMatchesFilters(download, {
          client: clientFilter,
          category: categoryFilter,
          state: stateFilter,
          text: textFilter
        })
      ),
    [downloads, clientFilter, categoryFilter, stateFilter, textFilter]
  );
  const clientOptions = useMemo(() => uniqueDownloadClients(downloads), [downloads]);
  const categoryOptions = useMemo(() => uniqueDownloadCategories(downloads, lastResources), [downloads, lastResources]);
  const healthRows = useMemo(() => downloadClientHealthRows(integrations), [integrations]);

  const selectableKeys = useMemo(() => filteredDownloads.map(downloadKey).filter(Boolean), [filteredDownloads]);
  const selectedKeySet = useMemo(() => new Set(selectedKeys), [selectedKeys]);
  const selectedDownloads = useMemo(
    () => filteredDownloads.filter((download) => selectedKeySet.has(downloadKey(download))),
    [filteredDownloads, selectedKeySet]
  );
  const selectedIDs = selectedDownloads.map((download) => download.id);
  const allSelected = selectableKeys.length > 0 && selectableKeys.every((key) => selectedKeySet.has(key));
  const canStart = selectedDownloads.every((download) => supportsDownloadAction(download, "start"));
  const canStop = selectedDownloads.every((download) => supportsDownloadAction(download, "stop"));
  const canRecheck = selectedDownloads.every((download) => supportsDownloadAction(download, "recheck"));
  const canPriority = selectedDownloads.every((download) => supportsDownloadAction(download, "topPriority"));
  const canForce = selectedDownloads.length > 0 && selectedDownloads.every((download) => supportsDownloadAction(download, "forceStart"));
  const canQbit = selectedDownloads.length > 0 && selectedDownloads.every(downloadSupportsQbitManagerActions);

  const stats = useMemo(() => summarizeDownloads(filteredDownloads), [filteredDownloads]);
  const boundedMax = boundedPositiveInt(rebalanceMax, 3, 25);

  /* ------------------------------- helpers -------------------------------- */

  function refreshQueue() {
    void queryClient.invalidateQueries({ queryKey: keys.acquisitionQueue });
    return downloadsQuery.refetch();
  }

  function invalidateRelated() {
    void queryClient.invalidateQueries({ queryKey: keys.acquisitionQueue });
    void queryClient.invalidateQueries({ queryKey: keys.wanted });
    void queryClient.invalidateQueries({ queryKey: keys.history(100) });
    void queryClient.invalidateQueries({ queryKey: keys.importReviews() });
  }

  function changeScope(next: DownloadScope) {
    if (next === scope) return;
    setScope(next);
    setSelectedKeys([]);
    setExpandedKey("");
    setDetails(null);
  }

  function clearFilters() {
    setTextFilter("");
    setClientFilter("");
    setCategoryFilter("");
    setStateFilter("all");
    changeScope("all");
  }

  function toggleSelection(download: DownloadStatus) {
    const key = downloadKey(download);
    if (!key) return;
    setSelectedKeys((current) => (current.includes(key) ? current.filter((item) => item !== key) : [...current, key]));
  }

  function toggleAll() {
    setSelectedKeys((current) => {
      const next = new Set(current);
      const everyVisibleSelected = selectableKeys.length > 0 && selectableKeys.every((key) => next.has(key));
      for (const key of selectableKeys) {
        if (everyVisibleSelected) {
          next.delete(key);
        } else {
          next.add(key);
        }
      }
      return Array.from(next);
    });
  }

  /* ------------------------------- mutations ------------------------------- */

  async function applyActionToIDs(
    action: DownloadAction,
    ids: string[],
    options: { client?: string; deleteFiles?: boolean; forceStart?: boolean } = {}
  ) {
    const actionIDs = ids.filter(Boolean);
    if (!actionIDs.length) return;
    const isBulk = actionIDs.length > 1;
    setActionID(isBulk ? `bulk:${action}` : `${actionIDs[0]}:${action}`);
    try {
      const result = await runDownloadAction(action, actionIDs, {
        client: options.client,
        deleteFiles: options.deleteFiles ?? false,
        forceStart: options.forceStart
      });
      if (action === "delete") {
        const deleted = new Set(actionIDs);
        setSelectedKeys((current) => current.filter((key) => !deleted.has(downloadKeyID(key))));
        if (details && deleted.has(details.status.id)) {
          setDetails(null);
          setExpandedKey("");
        }
      }
      toast.success(downloadActionToast(action, actionIDs.length, result.applied ? undefined : result.message));
      if (
        action !== "delete" &&
        details?.status.id &&
        actionIDs.includes(details.status.id) &&
        downloadSupportsDetails(details.status)
      ) {
        try {
          setDetails(await fetchDownloadDetails(details.status.id, details.status.client));
        } catch {
          /* details refresh is best-effort; queue refetch below still runs */
        }
      }
      await refreshQueue();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Download action failed");
    } finally {
      setActionID("");
    }
  }

  async function runCompletedImport(target: DownloadStatus | DownloadStatus[]) {
    const targets = Array.isArray(target) ? target : [target];
    const downloadIDs = targets.map((item) => item.id).filter(Boolean);
    const busyID = downloadIDs.length > 1 ? "bulk:import" : downloadIDs[0] ? `${downloadIDs[0]}:import` : "";
    setImporting(true);
    if (busyID) setActionID(busyID);
    try {
      const outcome = await importCompletedDownloads({
        downloadIds: downloadIDs,
        move: false,
        importMode: "copy",
        conflictAction: "rename",
        overwrite: false,
        limit: 50
      });
      toast.notify(
        `Import: ${outcome.imported} imported · ${outcome.autoMatched} auto-matched · ${outcome.reviewQueued} queued for review · ${outcome.skipped} skipped · ${outcome.errored} errored (${outcome.checked} checked)`,
        outcome.errored > 0 ? "warn" : "success"
      );
      invalidateRelated();
      await downloadsQuery.refetch();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Completed import failed");
    } finally {
      setImporting(false);
      if (busyID) setActionID("");
    }
  }

  async function runFailedRecovery(
    target: DownloadStatus | DownloadStatus[],
    options: { autoGrab?: boolean; force?: boolean } = {}
  ) {
    const targets = Array.isArray(target) ? target : [target];
    const downloadIDs = targets.map((item) => item.id).filter(Boolean);
    const busyID = downloadIDs.length > 1 ? "bulk:recover" : downloadIDs[0] ? `${downloadIDs[0]}:recover` : "";
    setRecovering(true);
    if (busyID) setActionID(busyID);
    try {
      const run = await recoverFailedDownloads({
        downloadIds: downloadIDs,
        autoGrab: options.autoGrab ?? false,
        paused: true,
        removeFailed: false,
        force: options.force ?? downloadIDs.length > 0
      });
      toast.notify(
        `Recovery ${run.status}: ${run.downloadsChecked} checked · ${run.failedCount} failed · ${run.replacementsFound} replacements · ${run.grabbedCount} grabbed · ${run.removedCount} removed`,
        run.errorCount > 0 ? "warn" : "success"
      );
      invalidateRelated();
      await downloadsQuery.refetch();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed-download recovery failed");
    } finally {
      setRecovering(false);
      if (busyID) setActionID("");
    }
  }

  async function runQueueRebalance() {
    const maxActive = boundedPositiveInt(rebalanceMax, 3, 25);
    setRebalanceMax(String(maxActive));
    setRebalancing(true);
    try {
      const plan = await rebalanceDownloads({
        maxActive,
        client: clientFilter.trim() || undefined,
        tag: downloadScopeTag(scope),
        category: categoryFilter.trim() || undefined,
        dryRun: false,
        stopOverflow: true
      });
      toast.success(
        `${plan.applied ? "Queue balanced" : "Queue plan"}: active ${plan.activeCount}/${plan.maxActive} · start ${plan.startIds.length} · stop ${plan.stopIds.length}`
      );
      await refreshQueue();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Queue rebalance failed");
    } finally {
      setRebalancing(false);
    }
  }

  async function loadDetails(download: DownloadStatus, key: string) {
    setDetailsLoading(true);
    try {
      setDetails(await fetchDownloadDetails(download.id, download.client));
    } catch (error) {
      const fallback = demoModeEnabled
        ? (demoSeeds.downloadDetailsByKey as Record<string, DownloadDetails | undefined>)[key]
        : undefined;
      if (fallback) {
        setDetails(fallback);
        toast.notify(
          error instanceof Error ? `${error.message}. Showing demo details.` : "Download details failed. Showing demo details.",
          "warn"
        );
      } else {
        toast.error(error instanceof Error ? error.message : "Download details failed");
        setExpandedKey("");
      }
    } finally {
      setDetailsLoading(false);
    }
  }

  async function openDetails(download: DownloadStatus) {
    const key = downloadKey(download);
    if (expandedKey === key) {
      setExpandedKey("");
      setDetails(null);
      return;
    }
    if (!download.id || !downloadSupportsDetails(download)) return;
    setExpandedKey(key);
    setDetails(null);
    await loadDetails(download, key);
  }

  function reloadDetails() {
    if (!details) return;
    const target = downloads.find((download) => downloadKey(download) === expandedKey) ?? details.status;
    void loadDetails(target, expandedKey || downloadKey(target));
  }

  /* -------------------------------- render -------------------------------- */

  const queueError =
    downloadsQuery.isError && !demoModeEnabled
      ? downloadsQuery.error instanceof Error
        ? downloadsQuery.error.message
        : "Download refresh failed"
      : "";

  return (
    <>
      <Toolbar align="start">
        <ToolbarButton icon={RefreshCw} label="Refresh" busy={downloadsQuery.isFetching} onClick={() => void refreshQueue()} />
        <ToolbarButton
          icon={UploadCloud}
          label="Import Completed"
          busy={importing}
          disabled={filteredDownloads.length === 0}
          onClick={() => void runCompletedImport(filteredDownloads)}
        />
        <ToolbarButton
          icon={HardDriveDownload}
          label="Recover Failed"
          busy={recovering}
          disabled={filteredDownloads.length === 0}
          onClick={() => void runFailedRecovery(filteredDownloads, { autoGrab: true })}
        />
        <ToolbarButton
          icon={SlidersHorizontal}
          label="Balance Queue"
          title={`Keep up to ${boundedMax} downloads active`}
          busy={rebalancing}
          disabled={filteredDownloads.length === 0}
          onClick={() => void runQueueRebalance()}
        />
        <ToolbarButton icon={Plus} label="Add Download" tone="accent" onClick={() => setAddOpen(true)} />
        <ToolbarButton icon={Settings} label="Manage Clients" onClick={() => setManageOpen(true)} />
      </Toolbar>

      <div className="activity-health-strip" aria-label="Download client status">
        {healthRows.map((client) => (
          <Badge key={client.name} tone={integrationBadgeTone(client.status)} title={client.message}>
            {client.name}: {client.status}
          </Badge>
        ))}
      </div>

      <StatBar
        stats={[
          { label: "Active", value: stats.active, tone: "accent" },
          { label: "Paused", value: stats.paused },
          { label: "Complete", value: stats.complete, tone: "success" },
          { label: "Failed", value: stats.failed, tone: stats.failed > 0 ? "danger" : "neutral" },
          { label: "Blocked", value: stats.blocked, tone: stats.blocked > 0 ? "warn" : "neutral" },
          { label: "Selected", value: selectedDownloads.length }
        ]}
      />

      {queueError ? <InlineNotice tone="danger">{queueError}. Keeping current queue data.</InlineNotice> : null}

      <div className="activity-filters" aria-label="Download filters">
        <div className="activity-filter-find">
          <Field label="Find">
            <input
              value={textFilter}
              onChange={(event) => setTextFilter(event.target.value)}
              placeholder="Title, hash, tag, or path"
            />
          </Field>
        </div>
        <Field label="Client">
          <select value={clientFilter} onChange={(event) => setClientFilter(event.target.value)}>
            <option value="">All clients</option>
            {clientOptions.map((client) => (
              <option value={client} key={client}>
                {client}
              </option>
            ))}
          </select>
        </Field>
        <Field label="State">
          <select value={stateFilter} onChange={(event) => setStateFilter(event.target.value as DownloadStateFilter)}>
            <option value="all">All states</option>
            <option value="active">Active</option>
            <option value="paused">Paused</option>
            <option value="complete">Complete</option>
            <option value="failed">Failed</option>
          </select>
        </Field>
        <Field label="Category">
          <input
            list="activity-category-options"
            value={categoryFilter}
            onChange={(event) => setCategoryFilter(event.target.value)}
            placeholder="Any category"
          />
        </Field>
        <datalist id="activity-category-options">
          {categoryOptions.map((category) => (
            <option value={category} key={category} />
          ))}
        </datalist>
        <Field label="Max active" hint="Used by Balance Queue">
          <input
            className="activity-filter-max"
            inputMode="numeric"
            min="1"
            max="25"
            type="number"
            value={rebalanceMax}
            onChange={(event) => setRebalanceMax(event.target.value)}
          />
        </Field>
        <div className="activity-filter-scope">
          <Segmented
            ariaLabel="Download queue scope"
            options={[
              { value: "all", label: "All clients" },
              { value: "librarry", label: "Librarry" }
            ]}
            value={scope}
            onChange={changeScope}
          />
        </div>
      </div>

      {selectedDownloads.length > 0 ? (
        <div className="activity-bulkbar" aria-label="Bulk download actions">
          <span className="cell-muted activity-bulkbar-count">{selectedDownloads.length} selected</span>
          <Button size="sm" icon={Play} disabled={!canStart || Boolean(actionID)} onClick={() => void applyActionToIDs("start", selectedIDs)}>
            Start
          </Button>
          <Button size="sm" icon={Pause} disabled={!canStop || Boolean(actionID)} onClick={() => void applyActionToIDs("stop", selectedIDs)}>
            Stop
          </Button>
          <Button size="sm" icon={RefreshCw} disabled={!canRecheck || Boolean(actionID)} onClick={() => void applyActionToIDs("recheck", selectedIDs)}>
            Recheck
          </Button>
          <Button size="sm" icon={ChevronsUp} disabled={!canPriority || Boolean(actionID)} onClick={() => void applyActionToIDs("topPriority", selectedIDs)}>
            Top
          </Button>
          <Button size="sm" icon={ChevronUp} disabled={!canPriority || Boolean(actionID)} onClick={() => void applyActionToIDs("increasePriority", selectedIDs)}>
            Up
          </Button>
          <Button size="sm" icon={ChevronDown} disabled={!canPriority || Boolean(actionID)} onClick={() => void applyActionToIDs("decreasePriority", selectedIDs)}>
            Down
          </Button>
          <Button size="sm" icon={ChevronsDown} disabled={!canPriority || Boolean(actionID)} onClick={() => void applyActionToIDs("bottomPriority", selectedIDs)}>
            Bottom
          </Button>
          <Button
            size="sm"
            icon={Play}
            disabled={!canForce || Boolean(actionID)}
            onClick={() => void applyActionToIDs("forceStart", selectedIDs, { forceStart: true })}
          >
            Force
          </Button>
          <Button
            size="sm"
            icon={SlidersHorizontal}
            title="Toggle sequential download (qBittorrent)"
            disabled={!canQbit || Boolean(actionID)}
            onClick={() => void applyActionToIDs("toggleSequential", selectedIDs)}
          >
            Sequential
          </Button>
          <Button
            size="sm"
            icon={FileCheck2}
            title="Toggle first/last piece priority (qBittorrent)"
            disabled={!canQbit || Boolean(actionID)}
            onClick={() => void applyActionToIDs("toggleFirstLastPiece", selectedIDs)}
          >
            First/last
          </Button>
          <Button size="sm" icon={UploadCloud} disabled={importing} onClick={() => void runCompletedImport(selectedDownloads)}>
            Import
          </Button>
          <Button
            size="sm"
            icon={HardDriveDownload}
            disabled={recovering}
            onClick={() => void runFailedRecovery(selectedDownloads, { autoGrab: true, force: true })}
          >
            Recover
          </Button>
          <Button
            size="sm"
            variant="danger"
            icon={Trash2}
            disabled={Boolean(actionID)}
            onClick={() => setRemoval({ ids: selectedIDs, label: `${selectedDownloads.length} selected downloads` })}
          >
            Remove…
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setSelectedKeys([])}>
            Clear
          </Button>
        </div>
      ) : null}

      {downloadsQuery.isLoading ? (
        <LoadingRow label="Loading downloads…" />
      ) : filteredDownloads.length === 0 ? (
        <EmptyState
          icon={Inbox}
          title={downloads.length === 0 ? "No downloads returned from configured clients" : "No downloads match the current filters"}
          actions={
            <>
              <Button size="sm" icon={RefreshCw} onClick={() => void refreshQueue()}>
                Refresh
              </Button>
              {downloads.length === 0 ? (
                <Button size="sm" icon={Settings} onClick={() => navigate("/settings")}>
                  Client settings
                </Button>
              ) : (
                <Button size="sm" icon={FilterX} onClick={clearFilters}>
                  Clear filters
                </Button>
              )}
            </>
          }
        >
          {downloads.length === 0
            ? "Manual adds, wanted grabs, feed grabs, and completed-download imports will appear here after a client accepts work."
            : "Clear filters or switch scope to review the full acquisition queue."}
        </EmptyState>
      ) : (
        <DataTable>
          <thead>
            <tr>
              <th>
                <input
                  type="checkbox"
                  aria-label={allSelected ? "Clear all downloads" : "Select all downloads"}
                  checked={allSelected}
                  onChange={toggleAll}
                />
              </th>
              <th>Title</th>
              <th>Client</th>
              <th>Category</th>
              <th>State</th>
              <th>Progress</th>
              <th>Size</th>
              <th>Speed / ETA</th>
              <th aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            {filteredDownloads.map((download) => {
              const key = downloadKey(download);
              const isSelected = selectedKeySet.has(key);
              const busy = actionID.startsWith(`${download.id}:`) || (actionID.startsWith("bulk:") && isSelected);
              const expanded = expandedKey === key;
              return (
                <React.Fragment key={key}>
                  <tr className={isSelected ? "selected" : undefined}>
                    <td>
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => toggleSelection(download)}
                        aria-label={`Select ${download.name || download.id}`}
                      />
                    </td>
                    <td className="cell-primary">
                      <div className="activity-table-title">
                        <strong title={download.name || download.id}>{download.name || download.id}</strong>
                        <span className="cell-muted" title={download.savePath}>
                          {download.savePath || "—"}
                        </span>
                        {download.failureReason || download.importError || download.retryCount ? (
                          <span className="activity-title-flags">
                            {download.failureReason ? (
                              <Badge tone="danger" title={download.failureReason}>
                                failed
                              </Badge>
                            ) : null}
                            {download.importError ? (
                              <Badge tone="danger" title={download.importError}>
                                import error
                              </Badge>
                            ) : null}
                            {download.retryCount ? <Badge tone="warn">{download.retryCount} retries</Badge> : null}
                          </span>
                        ) : null}
                      </div>
                    </td>
                    <td>
                      <Badge tone="neutral">{download.client || "qBittorrent"}</Badge>
                    </td>
                    <td className="cell-muted">{download.category || "—"}</td>
                    <td>
                      <div className="activity-state-cell">
                        <Badge tone={stateBadgeTone(download.state)}>{download.state}</Badge>
                        <span className="cell-muted">import {download.importStatus || "pending"}</span>
                      </div>
                    </td>
                    <td>
                      <div className="activity-progress-cell">
                        <ProgressBar
                          value={download.progress ?? 0}
                          tone={
                            download.failureReason || download.importError
                              ? "danger"
                              : (download.progress ?? 0) >= 1
                                ? "success"
                                : "neutral"
                          }
                        />
                        <span className="cell-muted">{formatPercent(download.progress ?? 0)}</span>
                      </div>
                    </td>
                    <td className="cell-muted">
                      {formatBytes(download.downloadedBytes ?? 0)} / {formatBytes(download.sizeBytes ?? 0)}
                    </td>
                    <td className="cell-muted">
                      <div className="activity-state-cell">
                        <span>
                          {formatSpeed(download.downloadRate ?? 0)} ↓ · {formatSpeed(download.uploadRate ?? 0)} ↑
                        </span>
                        <span>
                          ETA {formatDuration(download.etaSeconds)} · ratio {(download.ratio ?? 0).toFixed(2)} ·{" "}
                          {download.seeders ?? 0} seeders
                        </span>
                      </div>
                    </td>
                    <td className="cell-actions">
                      <IconButton
                        icon={Play}
                        label="Start"
                        size="sm"
                        disabled={busy || !supportsDownloadAction(download, "start")}
                        onClick={() => void applyActionToIDs("start", [download.id], { client: download.client })}
                      />
                      <IconButton
                        icon={Pause}
                        label="Pause"
                        size="sm"
                        disabled={busy || !supportsDownloadAction(download, "stop")}
                        onClick={() => void applyActionToIDs("stop", [download.id], { client: download.client })}
                      />
                      <IconButton
                        icon={RefreshCw}
                        label="Recheck"
                        size="sm"
                        disabled={busy || !supportsDownloadAction(download, "recheck")}
                        onClick={() => void applyActionToIDs("recheck", [download.id], { client: download.client })}
                      />
                      <IconButton
                        icon={ChevronUp}
                        label="Increase priority"
                        size="sm"
                        disabled={busy || !supportsDownloadAction(download, "increasePriority")}
                        onClick={() => void applyActionToIDs("increasePriority", [download.id], { client: download.client })}
                      />
                      <IconButton
                        icon={FileSearch}
                        label={expanded ? "Hide details" : "Details"}
                        size="sm"
                        tone={expanded ? "accent" : "neutral"}
                        disabled={busy || !downloadSupportsDetails(download)}
                        onClick={() => void openDetails(download)}
                      />
                      <IconButton
                        icon={UploadCloud}
                        label="Import completed download"
                        size="sm"
                        disabled={busy || importing || !downloadIsImportEligible(download)}
                        onClick={() => void runCompletedImport(download)}
                      />
                      <IconButton
                        icon={HardDriveDownload}
                        label="Retry failed download"
                        size="sm"
                        disabled={busy || recovering}
                        onClick={() => void runFailedRecovery(download, { autoGrab: true, force: true })}
                      />
                      <IconButton
                        icon={Trash2}
                        label="Remove download"
                        size="sm"
                        tone="danger"
                        disabled={busy}
                        onClick={() =>
                          setRemoval({ ids: [download.id], client: download.client, label: download.name || download.id })
                        }
                      />
                    </td>
                  </tr>
                  {expanded ? (
                    <tr>
                      <td colSpan={9}>
                        <DownloadDetailsPanel
                          details={details}
                          loading={detailsLoading}
                          busyID={actionID}
                          setBusyID={setActionID}
                          onDetails={setDetails}
                          onReload={reloadDetails}
                          onQueueRefresh={refreshQueue}
                          onAction={(action, options) =>
                            applyActionToIDs(action, [download.id], { client: download.client, ...options })
                          }
                          onRequestDelete={() =>
                            setRemoval({ ids: [download.id], client: download.client, label: download.name || download.id })
                          }
                        />
                      </td>
                    </tr>
                  ) : null}
                </React.Fragment>
              );
            })}
          </tbody>
        </DataTable>
      )}

      <AddDownloadModal open={addOpen} onClose={() => setAddOpen(false)} onAdded={refreshQueue} />
      <ManageClientsModal
        open={manageOpen}
        onClose={() => setManageOpen(false)}
        downloads={downloads}
        integrations={integrations}
        categoryFilter={categoryFilter}
        onCategoryFilterChange={setCategoryFilter}
        onTextFilterChange={setTextFilter}
        onResourcesLoaded={setLastResources}
      />

      <Modal
        title="Remove downloads"
        open={Boolean(removal)}
        onClose={() => setRemoval(null)}
        footer={
          removal ? (
            <>
              <Button variant="ghost" onClick={() => setRemoval(null)}>
                Cancel
              </Button>
              <Button
                variant="secondary"
                icon={Trash2}
                onClick={() => {
                  void applyActionToIDs("delete", removal.ids, { client: removal.client, deleteFiles: false });
                  setRemoval(null);
                }}
              >
                Remove
              </Button>
              <Button
                variant="danger"
                icon={Trash2}
                onClick={() => {
                  void applyActionToIDs("delete", removal.ids, { client: removal.client, deleteFiles: true });
                  setRemoval(null);
                }}
              >
                Delete with data
              </Button>
            </>
          ) : null
        }
      >
        {removal ? (
          <>
            <p>
              Remove <strong>{removal.label}</strong> from the download client?
            </p>
            <p className="cell-muted">
              “Remove” keeps downloaded files on disk. “Delete with data” also deletes the downloaded files.
            </p>
          </>
        ) : null}
      </Modal>
    </>
  );
}
