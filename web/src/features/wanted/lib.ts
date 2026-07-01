import type { LucideIcon } from "lucide-react";
import {
  BookOpen,
  CheckCircle2,
  Download,
  FileSearch,
  HardDriveDownload,
  RefreshCw,
  UploadCloud
} from "lucide-react";
import type {
  AcquisitionQueueItem,
  AuthorMissingBookPolicy,
  AuthorMonitorRun,
  AuthorSkippedItem,
  AuthorSubscription,
  DownloadStatus,
  FeedSyncRun,
  LibraryFile,
  MetadataCorrectionRequest,
  MetadataFieldCandidate,
  MetadataFieldEvidence,
  MetadataProvenance,
  MetadataReviewItem,
  MetadataReviewQueue,
  MonitorRun,
  ProviderMetadataRecord,
  QualityProfile,
  ReleaseDecision,
  SearchResult,
  UpgradeRun,
  WantedItem
} from "../../lib/api";

/*
 * Pure helpers ported from the legacy single-file App.tsx wanted panel.
 * Everything here is presentation/derivation logic only — no fetching.
 */

export type WantedPresence = "missing" | "grabbed" | "present";
export type WantedViewFilter = "missing" | "review" | "wanted" | "grabbed" | "cutoff-unmet" | "all";
export type ReleaseDecisionFilter = "all" | "approved" | "rejected";

export const wantedViewFilters: WantedViewFilter[] = ["missing", "review", "wanted", "grabbed", "cutoff-unmet", "all"];
export const releaseDecisionFilters: ReleaseDecisionFilter[] = ["all", "approved", "rejected"];

export function wantedViewFilterLabel(filter: WantedViewFilter) {
  return filter === "cutoff-unmet" ? "Cutoff Unmet" : filter;
}

/* ------------------------------ Form helpers ------------------------------ */

export function normalizedFormText(value?: string | null) {
  return (value ?? "").trim();
}

export function sameFormText(a?: string | null, b?: string | null) {
  return normalizedFormText(a) === normalizedFormText(b);
}

export type WantedEditForm = {
  title: string;
  authorName: string;
  coverUrl: string;
  qualityProfile: string;
  monitored: boolean;
};

export function wantedEditChanged(item: WantedItem | undefined, form: WantedEditForm) {
  if (!item) return false;
  return (
    !sameFormText(form.title, item.title) ||
    !sameFormText(form.authorName, item.authorName) ||
    !sameFormText(form.coverUrl, item.coverUrl) ||
    (normalizedFormText(form.qualityProfile) || "standard") !== (normalizedFormText(item.qualityProfile) || "standard") ||
    form.monitored !== item.monitored
  );
}

/* ------------------------------ Error helpers ----------------------------- */

export function isPersistenceRequiredError(message: string) {
  return message.toLowerCase().includes("requires database persistence");
}

export function appErrorMessage(message: string) {
  if (!isPersistenceRequiredError(message)) return message;
  return `${message}. Set LIBRARRY_DATABASE_URL to a Postgres database and restart the API to enable library files, import reviews, wanted queues, author monitoring, and release decisions.`;
}

export function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}

/* --------------------------- Presence / summaries -------------------------- */

export function normalizedWantedText(value?: string) {
  return (value || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/^(a|an|the)\s+/, "")
    .replace(/\s+/g, " ");
}

function stringMetadataValue(value: unknown) {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function libraryFileCountsAsPresent(file: LibraryFile) {
  const status = (file.importStatus || "").toLowerCase();
  return Boolean(file.path) && (status === "" || status === "available" || status === "imported");
}

function libraryFileMatchesWanted(item: WantedItem, file: LibraryFile) {
  const wantedID = stringMetadataValue(file.metadata?.wantedId) || stringMetadataValue(file.metadata?.librarryWantedId);
  if (wantedID && [item.id, item.workId, item.editionId, item.sourceKey].filter(Boolean).includes(wantedID)) return true;
  if (item.editionId && file.editionId === item.editionId) return true;
  if (item.format && file.mediaFormat && item.format !== file.mediaFormat) return false;
  const itemTitle = normalizedWantedText(item.title);
  const fileTitle = normalizedWantedText(file.title);
  if (!itemTitle || itemTitle !== fileTitle) return false;
  const itemAuthor = normalizedWantedText(item.authorName);
  const fileAuthor = normalizedWantedText(file.authorName);
  return !itemAuthor || !fileAuthor || itemAuthor === fileAuthor;
}

function wantedItemHasLibraryFile(item: WantedItem, files: LibraryFile[]) {
  return files.some((file) => libraryFileCountsAsPresent(file) && libraryFileMatchesWanted(item, file));
}

export function wantedPresenceMap(items: WantedItem[], files: LibraryFile[]) {
  const entries = new Map<string, WantedPresence>();
  items.forEach((item) => {
    const status = (item.status || "").toLowerCase();
    if (wantedItemHasLibraryFile(item, files)) {
      entries.set(item.id, "present");
    } else if (status === "grabbed") {
      entries.set(item.id, "grabbed");
    } else {
      entries.set(item.id, "missing");
    }
  });
  return entries;
}

export function summarizeWantedItems(items: WantedItem[], presence: Map<string, WantedPresence>) {
  return items.reduce(
    (summary, item) => {
      const state = presence.get(item.id) ?? "missing";
      if (state === "present") summary.present += 1;
      if (state === "grabbed") summary.grabbed += 1;
      if (state === "missing") summary.missing += 1;
      return summary;
    },
    { missing: 0, grabbed: 0, present: 0 }
  );
}

export function metadataReviewMap(queue: MetadataReviewQueue | null | undefined) {
  return new Map((queue?.items ?? []).map((item) => [item.wantedItem.id, item]));
}

export function summarizeMetadataReview(queue: MetadataReviewQueue | null | undefined) {
  return (queue?.items ?? []).reduce(
    (summary, item) => {
      summary.items += 1;
      summary.conflicts += item.conflictCount;
      summary.protected += item.protectedCount;
      return summary;
    },
    { items: 0, conflicts: 0, protected: 0 }
  );
}

export function wantedItemVisibleForFilter(
  item: WantedItem,
  presence: WantedPresence | undefined,
  filter: WantedViewFilter,
  hasMetadataReview: boolean
) {
  const status = (item.status || "").toLowerCase();
  switch (filter) {
    case "missing":
      return (presence ?? "missing") !== "present";
    case "review":
      return hasMetadataReview;
    case "wanted":
      return status === "wanted";
    case "grabbed":
      return status === "grabbed";
    // "cutoff-unmet" membership is server-defined (fetchWanted view); the
    // Books tab swaps the list wholesale instead of filtering client-side.
    default:
      return true;
  }
}

export function wantedItemSubtitle(item: WantedItem, review?: MetadataReviewItem) {
  const parts = [item.authorName || "Unknown author"];
  if (review?.conflictCount) {
    parts.push(`${review.conflictCount} metadata conflict${review.conflictCount === 1 ? "" : "s"}`);
  } else if (review?.protectedCount) {
    parts.push(`${review.protectedCount} protected field${review.protectedCount === 1 ? "" : "s"}`);
  }
  return parts.join(" · ");
}

export function wantedBadgeLabel(item: WantedItem, presence: WantedPresence | undefined) {
  const state = presence ?? "missing";
  if (state === "present") return `Present · ${item.format}`;
  if (state === "grabbed") return `Grabbed · ${item.format}`;
  return `Missing · ${item.format}`;
}

export function wantedPresenceTone(presence: WantedPresence | undefined): "success" | "info" | "warn" {
  if (presence === "present") return "success";
  if (presence === "grabbed") return "info";
  return "warn";
}

export function metadataReviewBadgeLabel(review: MetadataReviewItem) {
  if (review.conflictCount > 0) return `Review · ${review.conflictCount}`;
  return `Protected · ${review.protectedCount}`;
}

export function wantedOverrideLabel(fieldName: string) {
  switch (fieldName) {
    case "title":
      return "Title";
    case "author_name":
      return "Author";
    case "cover_url":
      return "Cover";
    case "quality_profile":
      return "Quality";
    case "language":
      return "Language";
    case "publisher":
      return "Publisher";
    case "published_date":
      return "Published";
    case "series":
      return "Series";
    case "series_position":
      return "Series position";
    case "isbn":
      return "ISBN";
    default:
      return fieldName.replace(/_/g, " ");
  }
}

/* ------------------------------- Releases --------------------------------- */

export function summarizeReleaseDecisions(releases: ReleaseDecision[]) {
  return releases.reduce(
    (summary, release) => {
      if (release.approved) {
        summary.approved += 1;
      } else {
        summary.rejected += 1;
      }
      return summary;
    },
    { approved: 0, rejected: 0 }
  );
}

export function releaseDecisionVisibleForFilter(release: ReleaseDecision, filter: ReleaseDecisionFilter) {
  if (filter === "approved") return release.approved;
  if (filter === "rejected") return !release.approved;
  return true;
}

export function releaseActionID(release: ReleaseDecision, force: boolean) {
  return `${release.id || release.sourceId || release.title}:${force ? "force" : "grab"}`;
}

/* --------------------------- Metadata provenance --------------------------- */

export function metadataProvenanceSummary(metadata: MetadataProvenance | null, loading: boolean) {
  if (loading) return "Loading stored provider records.";
  if (!metadata) return "No stored provider records for this wanted item.";
  const conflicts = metadata.fields.filter((field) => field.conflict).length;
  const protectedFields = metadata.fields.filter((field) => field.protected).length;
  if (conflicts > 0) {
    return `${metadata.records.length} provider records, ${conflicts} field${conflicts === 1 ? "" : "s"} need review.`;
  }
  if (protectedFields > 0) {
    return `${metadata.records.length} provider records, ${protectedFields} protected field${protectedFields === 1 ? "" : "s"}.`;
  }
  if (metadata.records.length > 0) return `${metadata.records.length} stored provider records.`;
  return "No stored provider records for this wanted item.";
}

export function metadataFieldSourceLabel(field: MetadataFieldEvidence) {
  if (field.reviewResolved) return "Confirmed current value";
  if (field.protected) return "Manual override";
  if (field.canonicalSource === "wanted") return "Canonical wanted value";
  if (field.candidates?.length) return "Provider candidates only";
  return "No provider evidence";
}

export function metadataFieldCandidateSummary(field: MetadataFieldEvidence) {
  if (!field.candidates?.length) return "No provider candidates.";
  const candidates = field.candidates.slice(0, 3).map((candidate) => `${candidate.provider}: ${candidate.value}`);
  const extra = field.candidates.length - candidates.length;
  return extra > 0 ? `${candidates.join(" · ")} · ${extra} more` : candidates.join(" · ");
}

export function metadataFieldCanApply(field: MetadataFieldEvidence) {
  return [
    "title",
    "author_name",
    "cover_url",
    "quality_profile",
    "language",
    "publisher",
    "published_date",
    "series",
    "series_position",
    "isbn"
  ].includes(field.fieldName);
}

export function metadataFieldCanConfirmCanonical(field: MetadataFieldEvidence) {
  return field.conflict && metadataFieldCanApply(field) && Boolean(normalizeMetadataValue(field.canonicalValue));
}

export function metadataFieldCanonicalActionID(field: MetadataFieldEvidence) {
  return `${field.fieldName}:canonical:${field.canonicalValue ?? ""}`;
}

export function metadataFieldApplicableCandidates(field: MetadataFieldEvidence): MetadataFieldCandidate[] {
  if (!metadataFieldCanApply(field)) return [];
  const canonical = normalizeMetadataValue(field.canonicalValue);
  const seen = new Set<string>();
  return (field.candidates ?? [])
    .filter((candidate) => {
      const value = normalizeMetadataValue(candidate.value);
      if (!value || value === canonical || seen.has(value)) return false;
      seen.add(value);
      return true;
    })
    .slice(0, 3);
}

export function metadataFieldStatusLabel(field: MetadataFieldEvidence) {
  return field.conflict ? "review" : field.reviewResolved ? "confirmed" : field.protected ? "protected" : "ok";
}

export function metadataRecordActionID(record: ProviderMetadataRecord) {
  return record.id || `${record.provider}:${record.providerKey}:${record.entityType}`;
}

export function metadataRecordCorrections(
  record: ProviderMetadataRecord,
  metadata: MetadataProvenance | null
): MetadataCorrectionRequest[] {
  const canonicalByField = new Map(
    (metadata?.fields ?? []).map((field) => [field.fieldName, normalizeMetadataValue(field.canonicalValue)])
  );
  const values: MetadataCorrectionRequest[] = [
    { fieldName: "title", value: record.values.title ?? "" },
    { fieldName: "author_name", value: record.values.authorName ?? "" },
    { fieldName: "cover_url", value: record.values.coverUrl ?? "" },
    { fieldName: "language", value: record.values.language ?? "" },
    { fieldName: "publisher", value: record.values.publisher ?? "" },
    { fieldName: "published_date", value: record.values.publishedDate ?? "" },
    { fieldName: "series", value: record.values.series ?? "" },
    { fieldName: "series_position", value: record.values.seriesPosition ?? "" },
    { fieldName: "isbn", value: metadataRecordISBNValue(record) }
  ];
  const seen = new Set<string>();
  return values.filter((correction) => {
    const fieldName = correction.fieldName;
    const value = (correction.value || "").trim();
    const normalized = normalizeMetadataValue(value);
    if (!value || !normalized || seen.has(fieldName)) return false;
    seen.add(fieldName);
    return canonicalByField.get(fieldName) !== normalized;
  });
}

function metadataRecordISBNValue(record: ProviderMetadataRecord) {
  return (record.values.isbns ?? []).map((isbn) => isbn.trim()).filter(Boolean).join(", ");
}

export function normalizeMetadataValue(value?: string) {
  return (value || "").trim().toLowerCase().replace(/\s+/g, " ");
}

export function metadataRecordPrimaryLine(record: ProviderMetadataRecord) {
  const title = record.values.title || "Untitled";
  const author = record.values.authorName;
  return author ? `${title} by ${author}` : title;
}

export function metadataRecordSecondaryLine(record: ProviderMetadataRecord) {
  const parts = [
    record.values.format,
    record.values.language,
    record.values.publisher,
    record.values.publishedDate,
    record.values.isbns?.slice(0, 2).join(", "),
    record.values.matchedOn?.length ? `matched ${record.values.matchedOn.join(", ")}` : ""
  ].filter(Boolean);
  return parts.length ? parts.join(" · ") : "No normalized values extracted.";
}

export function metadataConfidenceLabel(confidence: number) {
  if (!Number.isFinite(confidence) || confidence <= 0) return "unscored";
  if (confidence <= 1) return `${Math.round(confidence * 100)}%`;
  return confidence.toFixed(1);
}

/* --------------------------- Acquisition queue ----------------------------- */

export function acquisitionQueueStateLabel(state: string) {
  switch (state) {
    case "needs_search":
      return "Needs search";
    case "ready_to_grab":
      return "Ready to grab";
    case "downloading":
      return "Downloading";
    case "queued":
      return "Queued";
    case "import_ready":
      return "Import ready";
    case "imported":
      return "Imported";
    case "blocked":
      return "Blocked";
    default:
      return state || "Unknown";
  }
}

export type AcquisitionTone = "ready" | "active" | "done" | "blocked" | "idle";

export function acquisitionQueueStateTone(state: string): AcquisitionTone {
  switch (state) {
    case "ready_to_grab":
    case "import_ready":
      return "ready";
    case "downloading":
    case "queued":
      return "active";
    case "imported":
      return "done";
    case "blocked":
      return "blocked";
    default:
      return "idle";
  }
}

export function acquisitionBadgeTone(state: string): "success" | "info" | "danger" | "neutral" | "accent" {
  switch (acquisitionQueueStateTone(state)) {
    case "ready":
      return "accent";
    case "active":
      return "info";
    case "done":
      return "success";
    case "blocked":
      return "danger";
    default:
      return "neutral";
  }
}

export function acquisitionQueueActionID(item: AcquisitionQueueItem) {
  return `${item.wantedItem.id}:${item.state}`;
}

export function acquisitionQueueActionLabel(item: AcquisitionQueueItem) {
  switch (item.state) {
    case "needs_search":
      return "Search";
    case "ready_to_grab":
      return "Grab";
    case "import_ready":
      return "Import";
    case "blocked":
      return queueDownloadsForRecovery(item).length ? "Recover" : "Review";
    case "queued":
    case "downloading":
      return "View queue";
    case "imported":
      return "Complete";
    default:
      return "Open";
  }
}

export function acquisitionQueueActionIcon(state: string): LucideIcon {
  switch (state) {
    case "needs_search":
      return FileSearch;
    case "ready_to_grab":
      return HardDriveDownload;
    case "import_ready":
      return UploadCloud;
    case "blocked":
      return RefreshCw;
    case "queued":
    case "downloading":
      return Download;
    case "imported":
      return CheckCircle2;
    default:
      return BookOpen;
  }
}

export function acquisitionQueueActionDisabled(item: AcquisitionQueueItem) {
  if (item.state === "imported") return true;
  if (item.state === "import_ready") return queueDownloadsForImport(item).length === 0;
  return false;
}

export function queueDownloadsForImport(item: AcquisitionQueueItem) {
  return (item.downloads ?? []).filter(
    (download) =>
      download.importStatus !== "imported" &&
      (download.importStatus === "ready" || (download.progress ?? 0) >= 1 || Boolean(download.completedAt))
  );
}

export function queueDownloadsForRecovery(item: AcquisitionQueueItem) {
  return (item.downloads ?? []).filter(downloadNeedsRecovery);
}

function downloadNeedsRecovery(download: DownloadStatus) {
  return Boolean(download.failureReason || download.importError || downloadStateTone(download.state) === "error");
}

function downloadStateTone(state: string) {
  const normalized = state.toLowerCase();
  if (normalized.includes("error") || normalized.includes("fail") || normalized.includes("missing")) return "error";
  if (normalized.includes("stop") || normalized.includes("pause")) return "paused";
  if (normalized.includes("upload") || normalized.includes("seed")) return "seeding";
  if (normalized.includes("download") || normalized.includes("meta")) return "active";
  return "idle";
}

/* ----------------------------- Quality profiles ---------------------------- */

export function profileKey(profile: QualityProfile) {
  return profile.id || `${profile.name}:${profile.mediaFormat}`;
}

/* ---------------------------- Author subscriptions ------------------------- */

export type AuthorSubscriptionStats = {
  total: number;
  missing: number;
  grabbed: number;
  present: number;
  review: number;
  firstWantedItem?: WantedItem;
};

export function authorSubscriptionKey(subscription: AuthorSubscription) {
  return subscription.id || `${subscription.provider}:${subscription.providerKey}:${subscription.format}`;
}

export function authorSubscriptionMonitorOptions(subscription: AuthorSubscription) {
  return {
    authorIds: subscription.id ? [subscription.id] : [],
    providerKeys: subscription.providerKey ? [subscription.providerKey] : [],
    force: true,
    targetKey: authorSubscriptionKey(subscription)
  };
}

export const authorMissingPolicyOptions: AuthorMissingBookPolicy[] = [
  "all",
  "future",
  "missing",
  "existing",
  "first",
  "latest",
  "none"
];

export function normalizedAuthorMissingPolicy(policy?: string): AuthorMissingBookPolicy {
  if (
    policy === "future" ||
    policy === "missing" ||
    policy === "existing" ||
    policy === "first" ||
    policy === "latest" ||
    policy === "none"
  ) {
    return policy;
  }
  return "all";
}

export function authorMissingPolicyLabel(policy: AuthorMissingBookPolicy) {
  switch (policy) {
    case "future":
      return "Future Books";
    case "missing":
      return "Missing Books";
    case "existing":
      return "Existing Books";
    case "first":
      return "First Book";
    case "latest":
      return "Latest Book";
    case "none":
      return "None";
    default:
      return "All Books";
  }
}

export function emptyAuthorSubscriptionStats(): AuthorSubscriptionStats {
  return {
    total: 0,
    missing: 0,
    grabbed: 0,
    present: 0,
    review: 0
  };
}

export function buildAuthorSubscriptionStatsMap(
  subscriptions: AuthorSubscription[],
  items: WantedItem[],
  presence: Map<string, WantedPresence>,
  reviews: Map<string, MetadataReviewItem>
) {
  const statsByKey = new Map<string, AuthorSubscriptionStats>();

  subscriptions.forEach((subscription) => {
    const authorName = normalizedWantedText(subscription.authorName);
    const stats = emptyAuthorSubscriptionStats();
    items.forEach((item) => {
      if (item.format !== subscription.format) return;
      if (normalizedWantedText(item.authorName) !== authorName) return;
      stats.total += 1;
      stats.firstWantedItem ??= item;
      if (reviews.has(item.id)) stats.review += 1;
      switch (presence.get(item.id) ?? "missing") {
        case "present":
          stats.present += 1;
          break;
        case "grabbed":
          stats.grabbed += 1;
          break;
        default:
          stats.missing += 1;
          break;
      }
    });
    statsByKey.set(authorSubscriptionKey(subscription), stats);
  });

  return statsByKey;
}

export function authorSubscriptionStatsSummary(stats: AuthorSubscriptionStats) {
  if (stats.total === 0) {
    return "No wanted books yet. Refresh author metadata to create tracked books.";
  }
  const parts = [`${stats.total} tracked`, `${stats.missing} missing`];
  if (stats.grabbed > 0) parts.push(`${stats.grabbed} grabbed`);
  if (stats.present > 0) parts.push(`${stats.present} present`);
  if (stats.review > 0) parts.push(`${stats.review} review`);
  return parts.join(" · ");
}

export function authorSubscriptionStatsBadges(stats: AuthorSubscriptionStats): Array<[string, number]> {
  if (stats.total === 0) return [["tracked", 0]];
  const badges: Array<[string, number]> = [
    ["missing", stats.missing],
    ["grabbed", stats.grabbed],
    ["present", stats.present],
    ["review", stats.review]
  ];
  return badges.filter(([, value]) => value > 0);
}

export function firstAuthorName(result: SearchResult) {
  return result.work.authors?.[0]?.name || "Unknown author";
}

export function authorSkippedItemKey(subscription: AuthorSubscription, skipped: AuthorSkippedItem) {
  return [
    subscription.id || subscription.providerKey || subscription.authorName,
    skipped.result.provider,
    skipped.result.work.id || skipped.result.edition?.id || skipped.result.work.title,
    skipped.result.edition?.publishedDate || skipped.result.work.firstPublishYear || "undated"
  ].join(":");
}

export function authorSkippedDateLabel(result: SearchResult) {
  return result.edition?.publishedDate || (result.work.firstPublishYear ? String(result.work.firstPublishYear) : "undated");
}

/* ------------------------------ Run summaries ------------------------------ */

export function monitorRunSummary(run: MonitorRun) {
  return `Monitor run: ${run.wantedChecked} item${run.wantedChecked === 1 ? "" : "s"}, ${run.grabbedCount} grabbed, ${run.errorCount} error${run.errorCount === 1 ? "" : "s"}`;
}

export function feedSyncRunSummary(run: FeedSyncRun) {
  return `Feed sync: ${run.releasesSeen} release${run.releasesSeen === 1 ? "" : "s"} seen, ${run.matchedCount} matched, ${run.grabbedCount} grabbed, ${run.errorCount} error${run.errorCount === 1 ? "" : "s"}`;
}

export function upgradeRunSummary(run: UpgradeRun) {
  return `Upgrade search: ${run.wantedChecked} checked, ${run.upgradeCount} upgrade${run.upgradeCount === 1 ? "" : "s"}, ${run.grabbedCount} grabbed, ${run.errorCount} error${run.errorCount === 1 ? "" : "s"}`;
}

export function authorMonitorRunSummary(run: AuthorMonitorRun) {
  return `Author monitor: ${run.authorsChecked} checked, ${run.itemsFound} metadata hit${run.itemsFound === 1 ? "" : "s"}, ${run.wantedCreated} wanted, ${run.errorCount} error${run.errorCount === 1 ? "" : "s"}`;
}
