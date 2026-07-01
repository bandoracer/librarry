import type {
  DownloadAction,
  DownloadPreferences,
  DownloadResources,
  DownloadStatus,
  IntegrationHealth
} from "../../lib/api";
import { formatSpeed } from "../../lib/format";

/*
 * Pure helpers ported from the legacy single-file App.tsx downloads panel.
 * Everything here is presentation-free so ActivityPage and its sub-components
 * stay focused on composition.
 */

export type DownloadScope = "all" | "librarry";
export type DownloadStateFilter = "all" | "active" | "paused" | "complete" | "failed";
export type BadgeTone = "neutral" | "accent" | "success" | "warn" | "danger" | "info";

/* ------------------------------ identity --------------------------------- */

export function downloadKey(download: DownloadStatus): string {
  return `${download.client || "qBittorrent"}:${download.id}`;
}

/** Reverse of downloadKey: extract the client-reported download id. */
export function downloadKeyID(key: string): string {
  return key.split(":").slice(1).join(":");
}

export function downloadScopeTag(scope: DownloadScope): string {
  return scope === "librarry" ? "librarry" : "";
}

/* ------------------------------ options ---------------------------------- */

export function uniqueDownloadClients(downloads: DownloadStatus[]): string[] {
  return Array.from(new Set(downloads.map((download) => download.client || "qBittorrent").filter(Boolean))).sort((a, b) =>
    a.localeCompare(b)
  );
}

export function uniqueDownloadResourceClients(downloads: DownloadStatus[], integrations: IntegrationHealth[]): string[] {
  const supportedClients = new Set(["qBittorrent", "Transmission", "SABnzbd"]);
  const clients = new Set(supportedClients);
  downloads.forEach((download) => {
    const client = download.client?.trim();
    if (client && supportedClients.has(client)) {
      clients.add(client);
    }
  });
  integrations.forEach((integration) => {
    const name = integration.name.trim();
    if (supportedClients.has(name)) {
      clients.add(name);
    }
  });
  return Array.from(clients).sort((a, b) => a.localeCompare(b));
}

export function uniqueDownloadCategories(downloads: DownloadStatus[], resources?: DownloadResources | null): string[] {
  return Array.from(
    new Set([
      ...downloads.map((download) => download.category).filter(Boolean),
      ...(resources?.categories ?? []).map((category) => category.name).filter(Boolean)
    ])
  ).sort((a, b) => a.localeCompare(b));
}

/* ------------------------------ filtering -------------------------------- */

export function downloadMatchesFilters(
  download: DownloadStatus,
  filters: { client: string; category: string; state: DownloadStateFilter; text: string }
): boolean {
  const client = filters.client.trim().toLowerCase();
  if (client && (download.client || "qBittorrent").toLowerCase() !== client) return false;
  const category = filters.category.trim().toLowerCase();
  if (category && !(download.category || "").toLowerCase().includes(category)) return false;
  if (filters.state !== "all") {
    if (filters.state === "complete") {
      if ((download.progress ?? 0) < 1 && download.importStatus !== "ready" && download.importStatus !== "imported") return false;
    } else if (filters.state === "failed") {
      if (stateTone(download.state) !== "error" && !download.failureReason && !download.importError) return false;
    } else if (stateTone(download.state) !== filters.state) {
      return false;
    }
  }
  const text = filters.text.trim().toLowerCase();
  if (!text) return true;
  const haystack = [download.name, download.id, download.client, download.category, download.savePath, ...(download.tags ?? [])]
    .join(" ")
    .toLowerCase();
  return haystack.includes(text);
}

/* --------------------------- state & summaries ---------------------------- */

export type StateTone = "active" | "paused" | "seeding" | "error" | "idle";

export function stateTone(state: string): StateTone {
  const normalized = state.toLowerCase();
  if (normalized.includes("error") || normalized.includes("fail") || normalized.includes("missing")) return "error";
  if (normalized.includes("stop") || normalized.includes("pause")) return "paused";
  if (normalized.includes("upload") || normalized.includes("seed")) return "seeding";
  if (normalized.includes("download") || normalized.includes("meta")) return "active";
  return "idle";
}

export function stateBadgeTone(state: string): BadgeTone {
  switch (stateTone(state)) {
    case "error":
      return "danger";
    case "paused":
      return "warn";
    case "seeding":
      return "success";
    case "active":
      return "info";
    default:
      return "neutral";
  }
}

export type DownloadQueueSummary = { active: number; paused: number; complete: number; failed: number; blocked: number };

export function summarizeDownloads(downloads: DownloadStatus[]): DownloadQueueSummary {
  return downloads.reduce<DownloadQueueSummary>(
    (summary, download) => {
      const tone = stateTone(download.state);
      if (tone === "active") summary.active += 1;
      if (tone === "paused") summary.paused += 1;
      if (tone === "error" || download.failureReason || download.importError) summary.failed += 1;
      if ((download.progress ?? 0) >= 1 || download.importStatus === "ready" || download.importStatus === "imported") {
        summary.complete += 1;
      }
      if (downloadIsPendingPlaceholder(download)) summary.blocked += 1;
      return summary;
    },
    { active: 0, paused: 0, complete: 0, failed: 0, blocked: 0 }
  );
}

export function downloadNeedsRecovery(download: DownloadStatus): boolean {
  return Boolean(download.failureReason || download.importError || stateTone(download.state) === "error");
}

/** Mirrors the legacy queueDownloadsForImport eligibility check. */
export function downloadIsImportEligible(download: DownloadStatus): boolean {
  return (
    download.importStatus !== "imported" &&
    (download.importStatus === "ready" || (download.progress ?? 0) >= 1 || Boolean(download.completedAt))
  );
}

/* --------------------------- capability gating ---------------------------- */

/**
 * Persisted `pending` placeholder rows (grabbed but not yet visible in the
 * client) must not offer client actions until the client reports a real ID.
 */
export function downloadIsPendingPlaceholder(download: DownloadStatus): boolean {
  return download.state.trim().toLowerCase() === "pending";
}

export function supportsDownloadAction(download: DownloadStatus, action: DownloadAction): boolean {
  if (downloadIsPendingPlaceholder(download)) {
    return false;
  }
  const client = (download.client || "qBittorrent").toLowerCase();
  if (client === "sabnzbd") {
    return [
      "start",
      "stop",
      "delete",
      "increasePriority",
      "decreasePriority",
      "topPriority",
      "bottomPriority",
      "setCategory",
      "rename"
    ].includes(action);
  }
  if (qbitOnlyDownloadAction(action)) {
    return client === "qbittorrent";
  }
  return true;
}

function qbitOnlyDownloadAction(action: DownloadAction): boolean {
  return ["toggleSequential", "toggleFirstLastPiece", "rename"].includes(action);
}

export function downloadSupportsQbitManagerActions(download: DownloadStatus): boolean {
  if (downloadIsPendingPlaceholder(download)) {
    return false;
  }
  return (download.client || "qBittorrent").toLowerCase() === "qbittorrent";
}

export function downloadSupportsDetails(download: DownloadStatus): boolean {
  if (downloadIsPendingPlaceholder(download)) {
    return false;
  }
  const client = (download.client || "qBittorrent").toLowerCase();
  return client === "qbittorrent" || client === "transmission" || client === "sabnzbd";
}

export function downloadSupportsTrackerActions(download: DownloadStatus): boolean {
  const client = (download.client || "qBittorrent").toLowerCase();
  return client === "qbittorrent" || client === "transmission";
}

export function downloadSupportsFileActions(download: DownloadStatus): boolean {
  const client = (download.client || "qBittorrent").toLowerCase();
  return client === "qbittorrent" || client === "transmission";
}

/* --------------------------- client health rows --------------------------- */

export type DownloadClientHealthRow = { name: string; status: string; message: string };

export function downloadClientHealthRows(integrations: IntegrationHealth[]): DownloadClientHealthRow[] {
  const clients = ["qBittorrent", "Transmission", "SABnzbd"];
  return clients.map((name) => {
    const health = integrations.find((integration) => integration.name.toLowerCase() === name.toLowerCase());
    if (!health) {
      return { name, status: "unknown", message: "Health not checked yet" };
    }
    return {
      name,
      status: health.configured ? health.status : "not configured",
      message: health.message || (health.configured ? "Configured" : "Not configured")
    };
  });
}

export function integrationBadgeTone(status: string): BadgeTone {
  const normalized = status.toLowerCase();
  if (normalized.includes("ready") || normalized.includes("ok")) return "success";
  if (normalized.includes("missing") || normalized.includes("not configured") || normalized.includes("unknown")) return "neutral";
  if (normalized.includes("error") || normalized.includes("failed")) return "danger";
  return "info";
}

/* ------------------------------- form text -------------------------------- */

export function normalizedFormText(value?: string | null): string {
  return (value ?? "").trim();
}

export function sameFormText(a?: string | null, b?: string | null): boolean {
  return normalizedFormText(a) === normalizedFormText(b);
}

export function splitTagInput(value: string): string[] {
  const seen = new Set<string>();
  return value
    .split(",")
    .map((tag) => tag.trim())
    .filter((tag) => {
      if (!tag || seen.has(tag)) return false;
      seen.add(tag);
      return true;
    });
}

export function boundedPositiveInt(value: string, fallback: number, max: number): number {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback;
  return Math.min(parsed, max);
}

/* ---------------------------- units & display ----------------------------- */

export function limitBytesToKiBInput(value?: number): string {
  if (!value || value < 0) return "";
  return String(Math.round(value / 1024));
}

export function bandwidthInputToBytes(value: string): number {
  const normalized = value.trim();
  if (!normalized) return 0;
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed) || parsed < 0) return -1;
  return Math.round(parsed * 1024);
}

export function queueLimitInputToInt(value: string): number {
  const normalized = value.trim();
  if (!normalized) return -1;
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed) || parsed < -1) return -2;
  return Math.trunc(parsed);
}

export function formatLimitSpeed(bytesPerSecond?: number): string {
  if (!bytesPerSecond || bytesPerSecond < 0) return "unlimited";
  return formatSpeed(bytesPerSecond);
}

export function formatDuration(seconds?: number): string {
  if (!seconds || seconds < 0 || !Number.isFinite(seconds)) return "unknown";
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (hours < 24) return remainingMinutes ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return remainingHours ? `${days}d ${remainingHours}h` : `${days}d`;
}

export function filePriorityLabel(priority: number): string {
  switch (priority) {
    case -1:
      return "low";
    case 0:
      return "skipped";
    case 1:
      return "normal";
    case 6:
      return "high";
    case 7:
      return "max";
    default:
      return `priority ${priority}`;
  }
}

/* --------------------------- preference forms ----------------------------- */

export type DownloadPreferenceForm = {
  savePath: string;
  tempPath: string;
  tempPathEnabled: boolean;
  startPaused: boolean;
  queueingEnabled: boolean;
  speedScheduleEnabled: boolean;
  downloadLimitKiB: string;
  uploadLimitKiB: string;
  altDownloadLimitKiB: string;
  altUploadLimitKiB: string;
  maxActiveDownloads: string;
  maxActiveUploads: string;
  maxActiveTorrents: string;
};

export function emptyPreferenceForm(): DownloadPreferenceForm {
  return {
    savePath: "",
    tempPath: "",
    tempPathEnabled: false,
    startPaused: false,
    queueingEnabled: true,
    speedScheduleEnabled: false,
    downloadLimitKiB: "",
    uploadLimitKiB: "",
    altDownloadLimitKiB: "",
    altUploadLimitKiB: "",
    maxActiveDownloads: "-1",
    maxActiveUploads: "-1",
    maxActiveTorrents: "-1"
  };
}

export function preferenceFormFromPreferences(preferences: DownloadPreferences): DownloadPreferenceForm {
  return {
    savePath: preferences.savePath || "",
    tempPath: preferences.tempPath || "",
    tempPathEnabled: Boolean(preferences.tempPathEnabled),
    startPaused: Boolean(preferences.startPaused),
    queueingEnabled: Boolean(preferences.queueingEnabled),
    speedScheduleEnabled: Boolean(preferences.speedScheduleEnabled),
    downloadLimitKiB: limitBytesToKiBInput(preferences.downloadLimit),
    uploadLimitKiB: limitBytesToKiBInput(preferences.uploadLimit),
    altDownloadLimitKiB: limitBytesToKiBInput(preferences.alternativeDownloadLimit),
    altUploadLimitKiB: limitBytesToKiBInput(preferences.alternativeUploadLimit),
    maxActiveDownloads: String(preferences.maxActiveDownloads ?? -1),
    maxActiveUploads: String(preferences.maxActiveUploads ?? -1),
    maxActiveTorrents: String(preferences.maxActiveTorrents ?? -1)
  };
}

export function downloadPreferenceFormLimits(form: DownloadPreferenceForm, includeMaxActiveTorrents: boolean) {
  return {
    downloadLimit: bandwidthInputToBytes(form.downloadLimitKiB),
    uploadLimit: bandwidthInputToBytes(form.uploadLimitKiB),
    alternativeDownloadLimit: bandwidthInputToBytes(form.altDownloadLimitKiB),
    alternativeUploadLimit: bandwidthInputToBytes(form.altUploadLimitKiB),
    maxActiveDownloads: queueLimitInputToInt(form.maxActiveDownloads),
    maxActiveUploads: queueLimitInputToInt(form.maxActiveUploads),
    maxActiveTorrents: includeMaxActiveTorrents ? queueLimitInputToInt(form.maxActiveTorrents) : -1
  };
}

export function downloadPreferenceFormValid(form: DownloadPreferenceForm, includeMaxActiveTorrents: boolean): boolean {
  const limits = downloadPreferenceFormLimits(form, includeMaxActiveTorrents);
  return (
    [limits.downloadLimit, limits.uploadLimit, limits.alternativeDownloadLimit, limits.alternativeUploadLimit].every(
      (value) => value >= 0
    ) && [limits.maxActiveDownloads, limits.maxActiveUploads, limits.maxActiveTorrents].every((value) => value >= -1)
  );
}

export function downloadPreferencesChanged(
  preferences: DownloadPreferences | null,
  form: DownloadPreferenceForm,
  includeMaxActiveTorrents: boolean
): boolean {
  if (!preferences || !downloadPreferenceFormValid(form, includeMaxActiveTorrents)) return false;
  const limits = downloadPreferenceFormLimits(form, includeMaxActiveTorrents);
  return (
    !sameFormText(form.savePath, preferences.savePath) ||
    form.tempPathEnabled !== Boolean(preferences.tempPathEnabled) ||
    !sameFormText(form.tempPath, preferences.tempPath) ||
    form.startPaused !== Boolean(preferences.startPaused) ||
    form.queueingEnabled !== Boolean(preferences.queueingEnabled) ||
    form.speedScheduleEnabled !== Boolean(preferences.speedScheduleEnabled) ||
    limits.downloadLimit !== (preferences.downloadLimit ?? 0) ||
    limits.uploadLimit !== (preferences.uploadLimit ?? 0) ||
    limits.alternativeDownloadLimit !== (preferences.alternativeDownloadLimit ?? 0) ||
    limits.alternativeUploadLimit !== (preferences.alternativeUploadLimit ?? 0) ||
    limits.maxActiveDownloads !== (preferences.maxActiveDownloads ?? -1) ||
    limits.maxActiveUploads !== (preferences.maxActiveUploads ?? -1) ||
    (includeMaxActiveTorrents && limits.maxActiveTorrents !== (preferences.maxActiveTorrents ?? -1))
  );
}

/* ------------------------------ toast labels ------------------------------ */

const downloadActionLabels: Record<DownloadAction, string> = {
  start: "Started",
  stop: "Stopped",
  delete: "Removed",
  recheck: "Recheck queued for",
  increasePriority: "Raised priority for",
  decreasePriority: "Lowered priority for",
  topPriority: "Moved to top",
  bottomPriority: "Moved to bottom",
  setCategory: "Category updated for",
  setLocation: "Location updated for",
  setDownloadLimit: "Download limit updated for",
  setUploadLimit: "Upload limit updated for",
  forceStart: "Force-started",
  toggleSequential: "Toggled sequential download for",
  toggleFirstLastPiece: "Toggled first/last piece priority for",
  rename: "Renamed",
  addTags: "Added tags to",
  removeTags: "Removed tags from"
};

export function downloadActionToast(action: DownloadAction, count: number, message?: string): string {
  const label = downloadActionLabels[action] ?? action;
  const subject = count === 1 ? "1 download" : `${count} downloads`;
  return message?.trim() ? `${label} ${subject} — ${message.trim()}` : `${label} ${subject}`;
}
