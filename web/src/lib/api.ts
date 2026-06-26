export type ProviderHealth = {
  name: string;
  status: string;
  configured: boolean;
  message: string;
  checkedAt: string;
};

export type SearchResult = {
  provider: string;
  kind: "book" | "author" | "series";
  work: {
    id: string;
    title: string;
    authors?: Array<{ id: string; name: string }>;
    firstPublishYear?: number;
    series?: string;
    seriesPosition?: string;
    coverUrl?: string;
  };
  edition?: {
    id: string;
    title: string;
    format: "any" | "ebook" | "audiobook";
    language?: string;
    isbns?: string[];
    publisher?: string;
    publishedDate?: string;
  };
  score: number;
  confidence: "high" | "medium" | "review";
  matchedOn: string[];
};

export type IntegrationHealth = {
  name: string;
  configured: boolean;
  status: string;
  message: string;
};

export type Release = {
  id: string;
  infoHash?: string;
  indexer: string;
  title: string;
  sizeBytes?: number;
  seeders?: number;
  leechers?: number;
  downloadUrl: string;
  infoUrl?: string;
  protocol: string;
  categories?: string[];
  publishedAt?: string;
};

export type DownloadStatus = {
  client?: string;
  id: string;
  name: string;
  state: string;
  progress: number;
  savePath: string;
  category: string;
  tags?: string[];
  sizeBytes?: number;
  downloadedBytes?: number;
  uploadedBytes?: number;
  downloadRate?: number;
  uploadRate?: number;
  etaSeconds?: number;
  ratio?: number;
  seeders?: number;
  peers?: number;
  addedAt?: string;
  completedAt?: string;
  lastActivityAt?: string;
  lastSeenAt?: string;
  importStatus?: string;
  importedFileId?: string;
  importedAt?: string;
  importError?: string;
  failureReason?: string;
  failedAt?: string;
  retryCount?: number;
  replacementId?: string;
};

export type DownloadDetails = {
  status: DownloadStatus;
  properties?: DownloadProperties;
  files?: DownloadFile[];
  trackers?: DownloadTracker[];
  peers?: DownloadPeer[];
};

export type DownloadProperties = {
  savePath?: string;
  creationDate?: string;
  additionDate?: string;
  completionDate?: string;
  totalSizeBytes?: number;
  totalDownloaded?: number;
  totalUploaded?: number;
  downloadLimit?: number;
  uploadLimit?: number;
  downloadSpeed?: number;
  uploadSpeed?: number;
  etaSeconds?: number;
  ratio?: number;
  connections?: number;
  connectionsLimit?: number;
  timeElapsedSeconds?: number;
  seedingTimeSeconds?: number;
  pieceSizeBytes?: number;
  piecesHave?: number;
  piecesTotal?: number;
  reannounceSeconds?: number;
  createdBy?: string;
  comment?: string;
};

export type DownloadFile = {
  id: number;
  externalId?: string;
  name: string;
  sizeBytes?: number;
  progress: number;
  priority: number;
  availability?: number;
  isSeed?: boolean;
  firstPiece?: number;
  lastPiece?: number;
};

export type DownloadTracker = {
  url: string;
  statusCode: number;
  status: string;
  tier?: number;
  message?: string;
  peers?: number;
  seeds?: number;
  leeches?: number;
  downloads?: number;
};

export type DownloadCategory = {
  name: string;
  savePath?: string;
};

export type DownloadResources = {
  client: string;
  categories: DownloadCategory[];
  tags: string[];
};

export type DownloadPeer = {
  id: string;
  ip: string;
  port?: number;
  client?: string;
  connection?: string;
  country?: string;
  countryCode?: string;
  flags?: string;
  flagsDescription?: string;
  progress?: number;
  relevance?: number;
  downloadRate?: number;
  uploadRate?: number;
  downloadedBytes?: number;
  uploadedBytes?: number;
  files?: string;
};

export type DownloadAction =
  | "start"
  | "stop"
  | "delete"
  | "recheck"
  | "increasePriority"
  | "decreasePriority"
  | "topPriority"
  | "bottomPriority"
  | "setCategory"
  | "setLocation"
  | "setDownloadLimit"
  | "setUploadLimit"
  | "forceStart"
  | "toggleSequential"
  | "toggleFirstLastPiece"
  | "rename"
  | "addTags"
  | "removeTags";

export type DownloadFileAction = "skip" | "normal" | "high" | "max" | "priority";
export type DownloadTrackerAction = "add" | "edit" | "remove";

export type DownloadActionResult = {
  action: DownloadAction;
  ids: string[];
  applied: boolean;
  message?: string;
  downloads?: DownloadStatus[];
};

export type DownloadFileActionResult = {
  action: DownloadFileAction;
  downloadId: string;
  ids: number[];
  priority: number;
  applied: boolean;
  message?: string;
  download?: DownloadDetails;
};

export type DownloadTrackerActionResult = {
  action: DownloadTrackerAction;
  downloadId: string;
  urls?: string[];
  applied: boolean;
  message?: string;
  download?: DownloadDetails;
};

export type DownloadResourceActionResult = {
  action: string;
  client: string;
  applied: boolean;
  message?: string;
  resources?: DownloadResources;
};

export type DownloadRebalancePlan = {
  maxActive: number;
  activeCount: number;
  pausedCount: number;
  completeCount: number;
  failedCount: number;
  startIds: string[];
  stopIds: string[];
  applied: boolean;
  message: string;
  downloads?: DownloadStatus[];
};

export type WantedItem = {
  id: string;
  workId?: string;
  editionId?: string;
  title: string;
  authorName?: string;
  coverUrl?: string;
  format: "ebook" | "audiobook";
  qualityProfile: string;
  status: string;
  monitored: boolean;
  sourceProvider?: string;
  sourceKey?: string;
  currentReleaseId?: string;
  currentReleaseScore?: number;
  lastSearchAt?: string;
  lastUpgradeSearchAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type QualityProfile = {
  id?: string;
  name: string;
  mediaFormat: "any" | "ebook" | "audiobook";
  minScore: number;
  cutoffScore: number;
  minSeeders: number;
  maxSizeBytes: number;
  preferredTerms?: string[];
  requiredTerms?: string[];
  rejectedTerms?: string[];
  preferredScore: number;
  upgradeAllowed: boolean;
  createdAt?: string;
  updatedAt?: string;
};

export type AuthorSubscription = {
  id: string;
  provider: string;
  providerKey: string;
  authorName: string;
  format: "ebook" | "audiobook";
  qualityProfile: string;
  status: string;
  monitorNewItems: boolean;
  lastSyncAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type AuthorMonitorItemResult = {
  subscription: AuthorSubscription;
  resultsFound: number;
  wantedCreated: number;
  wantedItems?: WantedItem[];
  error?: string;
};

export type AuthorMonitorRun = {
  id: string;
  trigger: string;
  status: string;
  authorsChecked: number;
  itemsFound: number;
  wantedCreated: number;
  errorCount: number;
  message?: string;
  items?: AuthorMonitorItemResult[];
  startedAt: string;
  finishedAt?: string;
};

export type ReleaseDecision = {
  id: string;
  wantedItemId: string;
  sourceId: string;
  infoHash?: string;
  indexer: string;
  title: string;
  protocol: string;
  downloadUrl: string;
  infoUrl?: string;
  sizeBytes?: number;
  seeders?: number;
  leechers?: number;
  categories?: string[];
  score: number;
  approved: boolean;
  rejectedReason?: string;
  publishedAt?: string;
  searchedAt: string;
  createdAt: string;
};

export type WantedSearchOutcome = {
  wantedItem: WantedItem;
  releases: ReleaseDecision[];
};

export type MonitorItemResult = {
  wantedItem: WantedItem;
  releasesFound: number;
  approvedCount: number;
  rejectedCount: number;
  grabbedDownload?: DownloadStatus;
  error?: string;
};

export type MonitorRun = {
  id: string;
  trigger: string;
  status: string;
  wantedChecked: number;
  releasesFound: number;
  approvedCount: number;
  rejectedCount: number;
  grabbedCount: number;
  errorCount: number;
  message?: string;
  items?: MonitorItemResult[];
  startedAt: string;
  finishedAt?: string;
};

export type FeedSyncMatch = {
  wantedItem: WantedItem;
  release: ReleaseDecision;
  grabbedDownload?: DownloadStatus;
  error?: string;
};

export type FeedSyncRun = {
  id: string;
  trigger: string;
  status: string;
  releasesSeen: number;
  matchedCount: number;
  approvedCount: number;
  rejectedCount: number;
  grabbedCount: number;
  errorCount: number;
  message?: string;
  matches?: FeedSyncMatch[];
  startedAt: string;
  finishedAt?: string;
};

export type FailedDownloadResult = {
  download: DownloadStatus;
  wantedItem?: WantedItem;
  failureReason: string;
  removed?: boolean;
  replacementReleases?: ReleaseDecision[];
  replacementRelease?: ReleaseDecision;
  replacementDownload?: DownloadStatus;
  error?: string;
};

export type FailedDownloadRun = {
  id: string;
  trigger: string;
  status: string;
  downloadsChecked: number;
  failedCount: number;
  replacementsFound: number;
  grabbedCount: number;
  removedCount: number;
  errorCount: number;
  message?: string;
  items?: FailedDownloadResult[];
  startedAt: string;
  finishedAt?: string;
};

export type UpgradeItemResult = {
  wantedItem: WantedItem;
  currentScore: number;
  cutoffScore: number;
  releasesFound: number;
  upgradeRelease?: ReleaseDecision;
  grabbedDownload?: DownloadStatus;
  error?: string;
};

export type UpgradeRun = {
  id: string;
  trigger: string;
  status: string;
  wantedChecked: number;
  releasesFound: number;
  upgradeCount: number;
  grabbedCount: number;
  errorCount: number;
  message?: string;
  items?: UpgradeItemResult[];
  startedAt: string;
  finishedAt?: string;
};

export type HistoryEvent = {
  id: string;
  eventType: string;
  entityType?: string;
  entityId?: string;
  severity: string;
  message: string;
  data?: Record<string, unknown>;
  createdAt: string;
};

export type LibraryFile = {
  id: string;
  editionId?: string;
  mediaFormat: "ebook" | "audiobook";
  path: string;
  sourcePath?: string;
  title?: string;
  authorName?: string;
  extension?: string;
  sizeBytes?: number;
  checksum?: string;
  importStatus: string;
  metadata?: Record<string, unknown>;
  modifiedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type LibraryScanOutcome = {
  roots: string[];
  scanned: number;
  upserted: number;
  skipped: number;
  files: LibraryFile[];
  errors?: string[];
};

export type LibraryImportOutcome = {
  file: LibraryFile;
  destinationPath: string;
  moved: boolean;
  imported: boolean;
  skipped?: boolean;
  replaced?: boolean;
  hardlinked?: boolean;
  importMode?: string;
  conflictAction?: string;
  conflictPath?: string;
  message?: string;
};

export type ImportReview = {
  id: string;
  sourcePath: string;
  downloadId?: string;
  wantedId?: string;
  mediaFormat: "ebook" | "audiobook" | "unknown";
  title?: string;
  authorName?: string;
  sizeBytes?: number;
  reason: string;
  status: string;
  decision?: string;
  destinationPath?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
};

export type DownloadImportResult = {
  download: DownloadStatus;
  status: string;
  message?: string;
  sourcePath?: string;
  wantedId?: string;
  import?: LibraryImportOutcome;
  review?: ImportReview;
};

export type CompletedImportOutcome = {
  checked: number;
  imported: number;
  reviewQueued: number;
  skipped: number;
  errored: number;
  results: DownloadImportResult[];
};

export type ReviewDecisionOutcome = {
  review: ImportReview;
  import?: LibraryImportOutcome;
};

export type ReviewBulkDecisionResult = {
  id: string;
  status: string;
  message?: string;
  outcome?: ReviewDecisionOutcome;
};

export type ReviewBulkDecisionOutcome = {
  requested: number;
  resolved: number;
  imported: number;
  skipped: number;
  rejected: number;
  errored: number;
  results: ReviewBulkDecisionResult[];
};

const apiBase = import.meta.env.VITE_API_BASE_URL ?? "";
const apiKeyStorageKey = "librarry.apiKey";

export function getStoredAPIKey(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(apiKeyStorageKey)?.trim() ?? "";
}

export function setStoredAPIKey(value: string) {
  if (typeof window === "undefined") return;
  const normalized = value.trim();
  if (normalized) {
    window.localStorage.setItem(apiKeyStorageKey, normalized);
  } else {
    window.localStorage.removeItem(apiKeyStorageKey);
  }
}

async function fetch(input: RequestInfo | URL, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  const apiKey = getStoredAPIKey();
  if (apiKey && !headers.has("X-Api-Key")) {
    headers.set("X-Api-Key", apiKey);
  }
  return globalThis.fetch(input, { ...init, headers });
}

export async function fetchProviderHealth(): Promise<ProviderHealth[]> {
  const response = await fetch(`${apiBase}/api/v1/providers/health`);
  if (!response.ok) {
    throw new Error(`Provider health failed: ${response.status}`);
  }
  const payload = (await response.json()) as { providers: ProviderHealth[] };
  return payload.providers;
}

export async function searchMetadata(query: string, format: string): Promise<SearchResult[]> {
  const params = new URLSearchParams({
    query,
    type: "book",
    format
  });
  const response = await fetch(`${apiBase}/api/v1/search?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Search failed: ${response.status}`);
  }
  const payload = (await response.json()) as { results: SearchResult[] };
  return payload.results;
}

export async function fetchIntegrationHealth(): Promise<IntegrationHealth[]> {
  const response = await fetch(`${apiBase}/api/v1/integrations/health`);
  if (!response.ok) {
    throw new Error(`Integration health failed: ${response.status}`);
  }
  const payload = (await response.json()) as { integrations: IntegrationHealth[] };
  return payload.integrations;
}

export async function searchReleases(query: string, format: string): Promise<Release[]> {
  const response = await fetch(`${apiBase}/api/v1/releases/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, format, limit: 12 })
  });
  if (!response.ok) {
    throw new Error(`Release search failed: ${response.status}`);
  }
  const payload = (await response.json()) as { releases: Release[] };
  return payload.releases;
}

export async function grabRelease(release: Release, format: string): Promise<DownloadStatus> {
  const category = format === "audiobook" ? "books-audiobook" : "books-ebook";
  const response = await fetch(`${apiBase}/api/v1/grabs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      releaseUrl: release.downloadUrl,
      infoHash: release.infoHash,
      title: release.title,
      protocol: release.protocol,
      category,
      paused: true,
      tags: ["librarry", "librarry-ui"]
    })
  });
  if (!response.ok) {
    throw new Error(`Grab failed: ${response.status}`);
  }
  return (await response.json()) as DownloadStatus;
}

export async function grabManualDownload(request: {
  releaseUrl?: string;
  file?: File;
  title?: string;
  format?: string;
  client?: string;
  paused?: boolean;
}): Promise<DownloadStatus> {
  const format = request.format === "audiobook" ? "audiobook" : "ebook";
  const category = format === "audiobook" ? "books-audiobook" : "books-ebook";
  if (request.file) {
    const form = new FormData();
    if (request.releaseUrl) form.set("releaseUrl", request.releaseUrl);
    form.set("file", request.file);
    form.set("uploadName", request.file.name);
    if (request.title) form.set("title", request.title);
    if (request.client && request.client !== "SABnzbd") form.set("client", request.client);
    form.set("protocol", "torrent");
    form.set("category", category);
    form.set("paused", String(request.paused ?? true));
    form.set("tags", "librarry,librarry-ui,manual");
    const response = await fetch(`${apiBase}/api/v1/grabs`, {
      method: "POST",
      body: form
    });
    if (!response.ok) {
      throw new Error(`Manual upload failed: ${response.status}`);
    }
    return (await response.json()) as DownloadStatus;
  }
  const response = await fetch(`${apiBase}/api/v1/grabs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      client: request.client || undefined,
      releaseUrl: request.releaseUrl ?? "",
      title: request.title,
      protocol: protocolForURL(request.releaseUrl ?? ""),
      category,
      paused: request.paused ?? true,
      tags: ["librarry", "librarry-ui", "manual"]
    })
  });
  if (!response.ok) {
    throw new Error(`Manual grab failed: ${response.status}`);
  }
  return (await response.json()) as DownloadStatus;
}

export type DownloadListOptions = {
  tag?: string;
  client?: string;
  category?: string;
};

export async function fetchDownloads(options: string | DownloadListOptions = ""): Promise<DownloadStatus[]> {
  const params = new URLSearchParams();
  const normalized = typeof options === "string" ? { tag: options } : options;
  if (normalized.tag) params.set("tag", normalized.tag);
  if (normalized.client) params.set("client", normalized.client);
  if (normalized.category) params.set("category", normalized.category);
  const response = await fetch(`${apiBase}/api/v1/downloads?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Download refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { downloads: DownloadStatus[] };
  return payload.downloads;
}

export async function fetchDownloadDetails(id: string, client?: string): Promise<DownloadDetails> {
  const params = new URLSearchParams();
  if (client) params.set("client", client);
  const response = await fetch(`${apiBase}/api/v1/downloads/${encodeURIComponent(id)}?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Download details failed: ${response.status}`);
  }
  return (await response.json()) as DownloadDetails;
}

export async function fetchDownloadResources(client = "qBittorrent"): Promise<DownloadResources> {
  const params = new URLSearchParams();
  if (client) params.set("client", client);
  const response = await fetch(`${apiBase}/api/v1/downloads/resources?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Download resources failed: ${response.status}`);
  }
  return (await response.json()) as DownloadResources;
}

export async function runDownloadCategoryAction(options: {
  action: "create" | "edit" | "delete";
  name: string;
  newName?: string;
  savePath?: string;
  client?: string;
}): Promise<DownloadResourceActionResult> {
  const response = await fetch(`${apiBase}/api/v1/downloads/categories/actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      client: options.client ?? "qBittorrent",
      action: options.action,
      name: options.name,
      newName: options.newName,
      savePath: options.savePath
    })
  });
  if (!response.ok) {
    throw new Error(`Download category action failed: ${response.status}`);
  }
  return (await response.json()) as DownloadResourceActionResult;
}

export async function runDownloadTagAction(options: {
  action: "create" | "edit" | "delete";
  names: string[];
  newName?: string;
  client?: string;
}): Promise<DownloadResourceActionResult> {
  const response = await fetch(`${apiBase}/api/v1/downloads/tags/actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      client: options.client ?? "qBittorrent",
      action: options.action,
      names: options.names,
      newName: options.newName
    })
  });
  if (!response.ok) {
    throw new Error(`Download tag action failed: ${response.status}`);
  }
  return (await response.json()) as DownloadResourceActionResult;
}

export async function runDownloadAction(
  action: DownloadAction,
  ids: string[],
  options: {
    client?: string;
    deleteFiles?: boolean;
    category?: string;
    savePath?: string;
    name?: string;
    tags?: string[];
    forceStart?: boolean;
    downloadLimit?: number;
    uploadLimit?: number;
  } = {}
): Promise<DownloadActionResult> {
  const response = await fetch(`${apiBase}/api/v1/downloads/actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      action,
      client: options.client,
      ids,
      deleteFiles: options.deleteFiles ?? false,
      category: options.category,
      savePath: options.savePath,
      name: options.name,
      tags: options.tags,
      forceStart: options.forceStart,
      downloadLimit: options.downloadLimit,
      uploadLimit: options.uploadLimit
    })
  });
  if (!response.ok) {
    throw new Error(`Download action failed: ${response.status}`);
  }
  return (await response.json()) as DownloadActionResult;
}

export async function runDownloadFileAction(
  id: string,
  action: DownloadFileAction,
  ids: number[],
  options: { priority?: number; client?: string } = {}
): Promise<DownloadFileActionResult> {
  const response = await fetch(`${apiBase}/api/v1/downloads/${encodeURIComponent(id)}/files/actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      client: options.client,
      action,
      ids,
      priority: options.priority
    })
  });
  if (!response.ok) {
    throw new Error(`Download file action failed: ${response.status}`);
  }
  return (await response.json()) as DownloadFileActionResult;
}

export async function runDownloadTrackerAction(
  id: string,
  action: DownloadTrackerAction,
  options: { client?: string; urls?: string[]; url?: string; originalUrl?: string; newUrl?: string } = {}
): Promise<DownloadTrackerActionResult> {
  const response = await fetch(`${apiBase}/api/v1/downloads/${encodeURIComponent(id)}/trackers/actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      client: options.client,
      action,
      urls: options.urls,
      url: options.url,
      originalUrl: options.originalUrl,
      newUrl: options.newUrl
    })
  });
  if (!response.ok) {
    throw new Error(`Download tracker action failed: ${response.status}`);
  }
  return (await response.json()) as DownloadTrackerActionResult;
}

function protocolForURL(value: string) {
  const normalized = value.trim().toLowerCase();
  if (normalized.includes(".nzb") || normalized.includes("t=get") || normalized.includes("apikey=")) return "usenet";
  return "torrent";
}

export async function rebalanceDownloads(options: {
  maxActive: number;
  client?: string;
  tag?: string;
  category?: string;
  dryRun?: boolean;
  stopOverflow?: boolean;
}): Promise<DownloadRebalancePlan> {
  const response = await fetch(`${apiBase}/api/v1/downloads/rebalance`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      maxActive: options.maxActive,
      client: options.client,
      tag: options.tag ?? "librarry",
      category: options.category,
      dryRun: options.dryRun ?? false,
      stopOverflow: options.stopOverflow ?? true
    })
  });
  if (!response.ok) {
    throw new Error(`Queue rebalance failed: ${response.status}`);
  }
  return (await response.json()) as DownloadRebalancePlan;
}

export async function recoverFailedDownloads(options: {
  downloadIds?: string[];
  autoGrab?: boolean;
  paused?: boolean;
  removeFailed?: boolean;
  deleteFailedFiles?: boolean;
  force?: boolean;
  limit?: number;
  searchLimit?: number;
  minStalledMinutes?: number;
} = {}): Promise<FailedDownloadRun> {
  const response = await fetch(`${apiBase}/api/v1/downloads/recover-failed`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      trigger: "manual",
      downloadIds: options.downloadIds ?? [],
      limit: options.limit ?? 50,
      searchLimit: options.searchLimit ?? 20,
      minStalledMinutes: options.minStalledMinutes ?? 1440,
      autoGrab: options.autoGrab ?? false,
      paused: options.paused ?? true,
      removeFailed: options.removeFailed ?? false,
      deleteFailedFiles: options.deleteFailedFiles ?? false,
      force: options.force ?? false
    })
  });
  if (!response.ok) {
    throw new Error(`Failed-download recovery failed: ${response.status}`);
  }
  return (await response.json()) as FailedDownloadRun;
}

export async function fetchWanted(): Promise<WantedItem[]> {
  const response = await fetch(`${apiBase}/api/v1/wanted?status=wanted`);
  if (!response.ok) {
    throw new Error(`Wanted refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { wanted: WantedItem[] };
  return payload.wanted;
}

export async function fetchQualityProfiles(): Promise<QualityProfile[]> {
  const response = await fetch(`${apiBase}/api/v1/quality-profiles`);
  if (!response.ok) {
    throw new Error(`Quality profiles refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { profiles: QualityProfile[] };
  return payload.profiles;
}

export async function saveQualityProfile(profile: QualityProfile): Promise<QualityProfile> {
  const response = await fetch(`${apiBase}/api/v1/quality-profiles`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(profile)
  });
  if (!response.ok) {
    throw new Error(`Quality profile save failed: ${response.status}`);
  }
  return (await response.json()) as QualityProfile;
}

export async function fetchAuthorSubscriptions(status = "monitored"): Promise<AuthorSubscription[]> {
  const params = new URLSearchParams({ status });
  const response = await fetch(`${apiBase}/api/v1/authors?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Author subscriptions refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { authors: AuthorSubscription[] };
  return payload.authors;
}

export async function subscribeAuthor(result: SearchResult, format: string, qualityProfile = "standard"): Promise<AuthorSubscription> {
  const response = await fetch(`${apiBase}/api/v1/authors`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      result,
      format: format === "audiobook" ? "audiobook" : "ebook",
      qualityProfile,
      monitorNewItems: true
    })
  });
  if (!response.ok) {
    throw new Error(`Author subscription failed: ${response.status}`);
  }
  return (await response.json()) as AuthorSubscription;
}

export async function runAuthorMonitor(options: {
  force?: boolean;
  limit?: number;
  searchLimit?: number;
} = {}): Promise<AuthorMonitorRun> {
  const response = await fetch(`${apiBase}/api/v1/authors/monitor`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      trigger: "manual",
      force: options.force ?? false,
      limit: options.limit ?? 50,
      searchLimit: options.searchLimit ?? 20
    })
  });
  if (!response.ok) {
    throw new Error(`Author monitor failed: ${response.status}`);
  }
  return (await response.json()) as AuthorMonitorRun;
}

export async function createWanted(result: SearchResult, format: string): Promise<WantedItem> {
  const wantedFormat = format === "audiobook" ? "audiobook" : "ebook";
  const response = await fetch(`${apiBase}/api/v1/wanted`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      result,
      format: wantedFormat,
      qualityProfile: "standard"
    })
  });
  if (!response.ok) {
    throw new Error(`Mark wanted failed: ${response.status}`);
  }
  return (await response.json()) as WantedItem;
}

export async function searchWantedReleases(wantedID: string): Promise<WantedSearchOutcome> {
  const response = await fetch(`${apiBase}/api/v1/wanted/${wantedID}/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ limit: 20 })
  });
  if (!response.ok) {
    throw new Error(`Wanted release search failed: ${response.status}`);
  }
  return (await response.json()) as WantedSearchOutcome;
}

export async function grabWanted(wantedID: string, releaseID?: string): Promise<DownloadStatus> {
  const response = await fetch(`${apiBase}/api/v1/wanted/${wantedID}/grab`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      releaseId: releaseID,
      paused: true
    })
  });
  if (!response.ok) {
    throw new Error(`Wanted grab failed: ${response.status}`);
  }
  return (await response.json()) as DownloadStatus;
}

export async function runWantedMonitor(options: {
  force?: boolean;
  autoGrab?: boolean;
  paused?: boolean;
  limit?: number;
} = {}): Promise<MonitorRun> {
  const response = await fetch(`${apiBase}/api/v1/wanted/monitor`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      trigger: "manual",
      limit: options.limit ?? 50,
      searchLimit: 20,
      force: options.force ?? false,
      autoGrab: options.autoGrab ?? false,
      paused: options.paused ?? true
    })
  });
  if (!response.ok) {
    throw new Error(`Wanted monitor failed: ${response.status}`);
  }
  return (await response.json()) as MonitorRun;
}

export async function runWantedFeedSync(options: {
  format?: string;
  autoGrab?: boolean;
  paused?: boolean;
  limit?: number;
} = {}): Promise<FeedSyncRun> {
  const response = await fetch(`${apiBase}/api/v1/wanted/feed-sync`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      trigger: "manual",
      format: options.format ?? "any",
      limit: options.limit ?? 100,
      autoGrab: options.autoGrab ?? false,
      paused: options.paused ?? true
    })
  });
  if (!response.ok) {
    throw new Error(`Feed sync failed: ${response.status}`);
  }
  return (await response.json()) as FeedSyncRun;
}

export async function runUpgradeSearch(options: {
  wantedIds?: string[];
  autoGrab?: boolean;
  paused?: boolean;
  force?: boolean;
  limit?: number;
  searchLimit?: number;
  minScoreDelta?: number;
} = {}): Promise<UpgradeRun> {
  const response = await fetch(`${apiBase}/api/v1/wanted/upgrades`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      trigger: "manual",
      wantedIds: options.wantedIds ?? [],
      limit: options.limit ?? 50,
      searchLimit: options.searchLimit ?? 20,
      minScoreDelta: options.minScoreDelta ?? 5,
      autoGrab: options.autoGrab ?? false,
      paused: options.paused ?? true,
      force: options.force ?? false
    })
  });
  if (!response.ok) {
    throw new Error(`Upgrade search failed: ${response.status}`);
  }
  return (await response.json()) as UpgradeRun;
}

export async function fetchHistory(limit = 50): Promise<HistoryEvent[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  const response = await fetch(`${apiBase}/api/v1/librarry/history?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`History refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { events: HistoryEvent[] };
  return payload.events;
}

export async function fetchLibraryFiles(format = "any", limit = 100): Promise<LibraryFile[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (format && format !== "any") params.set("format", format);
  const response = await fetch(`${apiBase}/api/v1/library/files?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Library files refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { files: LibraryFile[] };
  return payload.files;
}

export async function fetchLibraryImportReviews(status = "pending", limit = 100): Promise<ImportReview[]> {
  const params = new URLSearchParams({ status, limit: String(limit) });
  const response = await fetch(`${apiBase}/api/v1/library/import-reviews?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Import review refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { reviews: ImportReview[] };
  return payload.reviews;
}

export async function scanLibrary(format = "any"): Promise<LibraryScanOutcome> {
  const response = await fetch(`${apiBase}/api/v1/library/scan`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ format, limit: 1000 })
  });
  if (!response.ok) {
    throw new Error(`Library scan failed: ${response.status}`);
  }
  return (await response.json()) as LibraryScanOutcome;
}

export async function importLibraryFile(options: {
  sourcePath: string;
  wantedId?: string;
  format?: string;
  move?: boolean;
  importMode?: "copy" | "move" | "hardlink" | "hardlinkOrCopy";
  conflictAction?: "rename" | "replace" | "skip" | "fail";
  overwrite?: boolean;
}): Promise<LibraryImportOutcome> {
  const response = await fetch(`${apiBase}/api/v1/library/import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(options)
  });
  if (!response.ok) {
    throw new Error(`Library import failed: ${response.status}`);
  }
  return (await response.json()) as LibraryImportOutcome;
}

export async function importCompletedDownloads(options: {
  downloadIds?: string[];
  move?: boolean;
  importMode?: "copy" | "move" | "hardlink" | "hardlinkOrCopy";
  conflictAction?: "rename" | "replace" | "skip" | "fail";
  overwrite?: boolean;
  limit?: number;
} = {}): Promise<CompletedImportOutcome> {
  const response = await fetch(`${apiBase}/api/v1/library/import-completed`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      downloadIds: options.downloadIds ?? [],
      move: options.move ?? false,
      importMode: options.importMode,
      conflictAction: options.conflictAction,
      overwrite: options.overwrite ?? false,
      limit: options.limit ?? 50
    })
  });
  if (!response.ok) {
    throw new Error(`Completed import failed: ${response.status}`);
  }
  return (await response.json()) as CompletedImportOutcome;
}

export async function resolveLibraryImportReview(
  reviewId: string,
  options: {
    action: "import" | "skip" | "reject";
    wantedId?: string;
    format?: string;
    move?: boolean;
    importMode?: "copy" | "move" | "hardlink" | "hardlinkOrCopy";
    conflictAction?: "rename" | "replace" | "skip" | "fail";
    overwrite?: boolean;
  }
): Promise<ReviewDecisionOutcome> {
  const response = await fetch(`${apiBase}/api/v1/library/import-reviews/${encodeURIComponent(reviewId)}/resolve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(options)
  });
  if (!response.ok) {
    throw new Error(`Import review update failed: ${response.status}`);
  }
  return (await response.json()) as ReviewDecisionOutcome;
}

export async function resolveLibraryImportReviewsBulk(options: {
  ids?: string[];
  action: "import" | "skip" | "reject";
  wantedId?: string;
  format?: string;
  move?: boolean;
  importMode?: "copy" | "move" | "hardlink" | "hardlinkOrCopy";
  conflictAction?: "rename" | "replace" | "skip" | "fail";
  overwrite?: boolean;
  status?: string;
  limit?: number;
}): Promise<ReviewBulkDecisionOutcome> {
  const response = await fetch(`${apiBase}/api/v1/library/import-reviews/resolve-bulk`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(options)
  });
  if (!response.ok) {
    throw new Error(`Bulk import review update failed: ${response.status}`);
  }
  return (await response.json()) as ReviewBulkDecisionOutcome;
}
