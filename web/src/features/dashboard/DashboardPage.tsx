import React, { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  CheckCircle2,
  HardDriveDownload,
  RadioTower,
  Rss,
  TrendingUp,
  Users
} from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  InlineNotice,
  LoadingRow,
  PageHeader,
  StatBar,
  ToolbarButton
} from "../../components/ui";
import { useToast } from "../../components/toast";
import {
  keys,
  useAcquisitionQueue,
  useAuthorMetadataReviews,
  useDownloads,
  useHistory,
  useImportReviews,
  useIntegrationHealth,
  useProviderHealth,
  useWantedMetadataReview
} from "../../lib/queries";
import {
  recoverFailedDownloads,
  runAuthorMonitor,
  runUpgradeSearch,
  runWantedFeedSync,
  runWantedMonitor
} from "../../lib/api";
import type { DownloadStatus } from "../../lib/api";
import { demoModeEnabled } from "../../lib/demo";
import { formatRelativeTime } from "../../lib/format";
import { navItems } from "../../app/nav";
import "./dashboard.css";

/*
 * Dashboard: a calm operations overview.
 *
 * Replaces the legacy "review inbox lanes" (App.tsx ~3338-3547) with a single
 * "Needs attention" card that only lists non-zero queues, plus pipeline,
 * health, and recent-activity summaries. Per-item review actions live on the
 * pages each row links to; the dashboard's job is triage and entry points.
 */

type Tone = "neutral" | "accent" | "success" | "warn" | "danger" | "info";

const dashboardSubtitle = navItems.find((item) => item.id === "dashboard")?.subtitle;

/** Mirrors the legacy dashboardFailedDownloads derivation (App.tsx downloadNeedsRecovery). */
function downloadNeedsRecovery(download: DownloadStatus): boolean {
  if (download.failureReason || download.importError) return true;
  const state = (download.state || "").toLowerCase();
  return state.includes("error") || state.includes("fail") || state.includes("missing");
}

function healthTone(item: { configured: boolean; status: string }): Tone {
  if (item.status === "ready") return "success";
  if (!item.configured) return "warn";
  return "danger";
}

function severityTone(severity: string): Tone {
  const normalized = severity.toLowerCase();
  if (normalized.includes("error") || normalized.includes("fail")) return "danger";
  if (normalized.includes("warn")) return "warn";
  if (normalized.includes("success")) return "success";
  return "info";
}

function statusLabel(status: string): string {
  return status.replace(/_/g, " ");
}

function failureMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

/** Compact health list used for both metadata providers and integrations. */
function HealthList(props: {
  title: string;
  items: { name: string; configured: boolean; status: string; message: string }[] | undefined;
  loading: boolean;
  errored: boolean;
  emptyLabel: string;
  configureTo: string;
  configureLabel: string;
}) {
  return (
    <DataTable>
      <thead>
        <tr>
          <th>{props.title}</th>
          <th />
        </tr>
      </thead>
      <tbody>
        {props.loading ? (
          <tr>
            <td colSpan={2}>
              <LoadingRow label={`Checking ${props.title.toLowerCase()}…`} />
            </td>
          </tr>
        ) : props.errored ? (
          <tr>
            <td colSpan={2} className="cell-muted">
              {props.title} health could not be loaded.
            </td>
          </tr>
        ) : props.items && props.items.length ? (
          props.items.map((item) => (
            <tr key={item.name}>
              <td>
                <div className="cell-primary">{item.name}</div>
                {item.status !== "ready" ? <div className="cell-muted">{item.message}</div> : null}
                {!item.configured ? <Link to={props.configureTo}>{props.configureLabel}</Link> : null}
              </td>
              <td className="cell-actions">
                <Badge tone={healthTone(item)}>{statusLabel(item.status)}</Badge>
              </td>
            </tr>
          ))
        ) : (
          <tr>
            <td colSpan={2} className="cell-muted">
              {props.emptyLabel}
            </td>
          </tr>
        )}
      </tbody>
    </DataTable>
  );
}

export default function DashboardPage() {
  const toast = useToast();
  const client = useQueryClient();
  const [actionError, setActionError] = useState<string | null>(null);

  const providerHealth = useProviderHealth();
  const integrationHealth = useIntegrationHealth();
  const metadataReview = useWantedMetadataReview();
  const authorReviews = useAuthorMetadataReviews();
  const importReviews = useImportReviews();
  const acquisitionQueue = useAcquisitionQueue();
  // Only Librarry-tagged jobs; unrelated client traffic isn't ours to triage.
  const downloads = useDownloads({ tag: "librarry" });
  const history = useHistory(10);

  const invalidateAfterRun = async () => {
    const targets: readonly (readonly unknown[])[] = [
      keys.wanted,
      keys.wantedMetadataReview,
      keys.acquisitionQueue,
      keys.authorMetadataReviews,
      keys.authorSubscriptions,
      keys.downloads(),
      keys.history(10)
    ];
    await Promise.all(targets.map((key) => client.invalidateQueries({ queryKey: key })));
  };

  const reportFailure = (label: string) => (error: unknown) => {
    const message = failureMessage(error, `${label} failed.`);
    toast.error(message);
    if (!demoModeEnabled) setActionError(message);
  };

  const reportSuccess = async (message: string) => {
    setActionError(null);
    toast.success(message);
    await invalidateAfterRun();
  };

  const monitor = useMutation({
    mutationFn: () => runWantedMonitor({}),
    onSuccess: (run) =>
      reportSuccess(
        `Search monitor ${run.status}: checked ${run.wantedChecked}, found ${run.releasesFound} releases, approved ${run.approvedCount}, grabbed ${run.grabbedCount}.`
      ),
    onError: reportFailure("Search monitor")
  });

  const feedSync = useMutation({
    mutationFn: () => runWantedFeedSync({}),
    onSuccess: (run) =>
      reportSuccess(
        `Feed sync ${run.status}: ${run.releasesSeen} feed releases, ${run.matchedCount} matched, ${run.approvedCount} approved, ${run.grabbedCount} grabbed.`
      ),
    onError: reportFailure("Feed sync")
  });

  const upgradeSearch = useMutation({
    mutationFn: () => runUpgradeSearch({}),
    onSuccess: (run) =>
      reportSuccess(
        `Upgrade search ${run.status}: checked ${run.wantedChecked}, ${run.upgradeCount} upgrades, grabbed ${run.grabbedCount}.`
      ),
    onError: reportFailure("Upgrade search")
  });

  const authorMonitor = useMutation({
    mutationFn: () => runAuthorMonitor({}),
    onSuccess: (run) =>
      reportSuccess(
        `Author monitor ${run.status}: checked ${run.authorsChecked} authors, found ${run.itemsFound} items, created ${run.wantedCreated} wanted.`
      ),
    onError: reportFailure("Author monitor")
  });

  const failedDownloads = useMemo(
    () => (downloads.data ?? []).filter(downloadNeedsRecovery),
    [downloads.data]
  );

  const recover = useMutation({
    mutationFn: () =>
      recoverFailedDownloads({
        downloadIds: failedDownloads.map((download) => download.id),
        autoGrab: true,
        force: true
      }),
    onSuccess: (run) =>
      reportSuccess(
        `Recovery ${run.status}: checked ${run.downloadsChecked}, found ${run.replacementsFound} replacements, grabbed ${run.grabbedCount}.`
      ),
    onError: reportFailure("Failed-download recovery")
  });

  /* ----------------------- Needs attention derivations ---------------------- */

  const metadataReviewCount = metadataReview.data?.items.length ?? 0;
  const authorReviewCount = authorReviews.data?.length ?? 0;
  const importReviewCount = importReviews.data?.length ?? 0;
  const blockedItems = useMemo(
    () => (acquisitionQueue.data?.items ?? []).filter((item) => item.state === "blocked"),
    [acquisitionQueue.data]
  );
  const blockedCount = acquisitionQueue.data?.summary.blocked ?? blockedItems.length;

  type AttentionRow = {
    key: string;
    count: number;
    tone: Tone;
    label: string;
    description: string;
    to: string;
    action?: React.ReactNode;
  };

  const allAttentionRows: AttentionRow[] = [
    {
      key: "metadata",
      count: metadataReviewCount,
      tone: "warn",
      label: "Metadata reviews",
      description: "Wanted books with conflicting provider metadata",
      to: "/wanted?filter=review"
    },
    {
      key: "authors",
      count: authorReviewCount,
      tone: "warn",
      label: "Author candidates",
      description: "Books skipped by author monitoring policy, awaiting a decision",
      to: "/wanted?tab=authors"
    },
    {
      key: "imports",
      count: importReviewCount,
      tone: "warn",
      label: "Import reviews",
      description: "Unmatched files waiting on a manual match",
      to: "/imports"
    },
    {
      key: "blocked",
      count: blockedCount,
      tone: "danger",
      label: "Blocked acquisitions",
      description: "Wanted books whose acquisition cannot proceed automatically",
      to: "/wanted"
    },
    {
      key: "failed",
      count: failedDownloads.length,
      tone: "danger",
      label: "Failed downloads",
      description: "Downloads that failed or cannot import; recovery searches for replacements",
      to: "/downloads?state=failed",
      action: (
        <Button
          size="sm"
          icon={HardDriveDownload}
          busy={recover.isPending}
          onClick={() => recover.mutate()}
        >
          Recover
        </Button>
      )
    }
  ];
  const attentionRows = allAttentionRows.filter((row) => row.count > 0);

  const attentionSources = [metadataReview, authorReviews, importReviews, acquisitionQueue, downloads];
  const attentionLoading = attentionSources.some((query) => query.isLoading);
  const attentionErrored = attentionSources.some((query) => query.isError);

  const summary = acquisitionQueue.data?.summary;

  const providersReady = (providerHealth.data ?? []).filter((item) => item.status === "ready").length;
  const integrationsReady = (integrationHealth.data ?? []).filter((item) => item.status === "ready").length;
  const healthSubtitle =
    providerHealth.data || integrationHealth.data
      ? `${providersReady}/${providerHealth.data?.length ?? 0} providers · ${integrationsReady}/${integrationHealth.data?.length ?? 0} integrations ready`
      : undefined;

  const events = history.data ?? [];

  return (
    <>
      <PageHeader
        title="Dashboard"
        subtitle={dashboardSubtitle}
        actions={
          <>
            <ToolbarButton
              icon={RadioTower}
              label="Search Monitor"
              title="Run wanted monitoring across due items"
              busy={monitor.isPending}
              onClick={() => monitor.mutate()}
            />
            <ToolbarButton
              icon={Rss}
              label="Feed Sync"
              title="Match recent indexer feed releases against wanted books"
              busy={feedSync.isPending}
              onClick={() => feedSync.mutate()}
            />
            <ToolbarButton
              icon={TrendingUp}
              label="Upgrade Search"
              title="Search for quality upgrades below cutoff"
              busy={upgradeSearch.isPending}
              onClick={() => upgradeSearch.mutate()}
            />
            <ToolbarButton
              icon={Users}
              label="Author Monitor"
              title="Check monitored authors for new books"
              busy={authorMonitor.isPending}
              onClick={() => authorMonitor.mutate()}
            />
          </>
        }
      />

      {actionError ? (
        <InlineNotice tone="danger" onDismiss={() => setActionError(null)}>
          {actionError}
        </InlineNotice>
      ) : null}

      <div className="dashboard-columns">
        <div className="dashboard-column">
          <Card
            title="Needs attention"
            subtitle="Queues that need a human decision"
            padded={!attentionRows.length}
          >
            {attentionErrored && !demoModeEnabled ? (
              <InlineNotice tone="warn">
                Some review queues could not be loaded; counts below may be incomplete.
              </InlineNotice>
            ) : null}
            {attentionLoading && !attentionRows.length ? (
              <LoadingRow label="Checking review queues…" />
            ) : attentionRows.length ? (
              <DataTable>
                <tbody>
                  {attentionRows.map((row) => (
                    <tr key={row.key}>
                      <td className="dashboard-attention-count">
                        <Badge tone={row.tone}>{row.count}</Badge>
                      </td>
                      <td>
                        <div className="cell-primary">{row.label}</div>
                        <div className="cell-muted">{row.description}</div>
                      </td>
                      <td className="cell-actions">
                        {row.action}
                        <Link
                          className="dashboard-arrow-link"
                          to={row.to}
                          aria-label={`Open ${row.label.toLowerCase()}`}
                          title={`Open ${row.label.toLowerCase()}`}
                        >
                          <ArrowRight size={15} aria-hidden />
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            ) : (
              <EmptyState icon={CheckCircle2} title="All caught up">
                No metadata, import, or acquisition reviews are pending.
              </EmptyState>
            )}
          </Card>

          <Card
            title="Recent activity"
            subtitle="Monitor and grab events"
            actions={<Link to="/downloads/history">View all</Link>}
            padded={!events.length}
          >
            {history.isError && !demoModeEnabled ? (
              <InlineNotice tone="warn">History could not be loaded.</InlineNotice>
            ) : null}
            {history.isLoading ? (
              <LoadingRow label="Loading history…" />
            ) : events.length ? (
              <DataTable>
                <tbody>
                  {events.slice(0, 10).map((event) => (
                    <tr key={event.id}>
                      <td className="dashboard-activity-time cell-muted">
                        {formatRelativeTime(event.createdAt)}
                      </td>
                      <td className="dashboard-activity-type">
                        <Badge tone={severityTone(event.severity)}>{statusLabel(event.eventType)}</Badge>
                      </td>
                      <td>{event.message}</td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            ) : (
              <EmptyState title="No recent activity">
                Monitor, feed sync, and grab events will appear here.
              </EmptyState>
            )}
          </Card>
        </div>

        <div className="dashboard-column">
          <Card
            title="Acquisition pipeline"
            subtitle={summary ? `${summary.total} wanted books tracked` : undefined}
            actions={
              <Link to="/wanted" aria-label="Open wanted queue">
                Wanted
              </Link>
            }
          >
            {acquisitionQueue.isLoading ? (
              <LoadingRow label="Loading queue…" />
            ) : summary ? (
              <StatBar
                stats={[
                  { label: "Search", value: summary.needsSearch },
                  { label: "Ready", value: summary.readyToGrab },
                  { label: "Queued", value: summary.queued },
                  { label: "Import", value: summary.importReady },
                  { label: "Done", value: summary.imported, tone: summary.imported ? "success" : "neutral" },
                  { label: "Blocked", value: summary.blocked, tone: summary.blocked ? "danger" : "neutral" }
                ]}
              />
            ) : (
              <InlineNotice tone="warn">Acquisition queue could not be loaded.</InlineNotice>
            )}
          </Card>

          <Card title="Health" subtitle={healthSubtitle} padded={false}>
            <div className="dashboard-health">
              <HealthList
                title="Providers"
                items={providerHealth.data}
                loading={providerHealth.isLoading}
                errored={providerHealth.isError && !demoModeEnabled}
                emptyLabel="No metadata providers reported."
                configureTo="/providers"
                configureLabel="Configure provider"
              />
              <HealthList
                title="Integrations"
                items={integrationHealth.data}
                loading={integrationHealth.isLoading}
                errored={integrationHealth.isError && !demoModeEnabled}
                emptyLabel="No download or indexer integrations reported."
                configureTo="/settings/connections"
                configureLabel="Configure connection"
              />
            </div>
          </Card>
        </div>
      </div>
    </>
  );
}
