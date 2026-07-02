export type ProviderHealth = {
  name: string;
  status: string;
  configured: boolean;
  message: string;
  checkedAt: string;
};

export type SearchResult = {
  provider: string;
  kind: "book" | "author" | "author_works" | "series";
  work: {
    id: string;
    title: string;
    authors?: Array<{ id: string; name: string; providerIds?: string[] }>;
    firstPublishYear?: number;
    description?: string;
    series?: string;
    seriesPosition?: string;
    coverUrl?: string;
    providerIds?: string[];
  };
  edition?: {
    id: string;
    title: string;
    format: "any" | "ebook" | "audiobook";
    language?: string;
    isbns?: string[];
    asin?: string;
    publisher?: string;
    publishedDate?: string;
    providerIds?: string[];
  };
  score: number;
  confidence: "high" | "medium" | "review";
  matchedOn: string[];
  rawSourceKey?: string;
};

export type MetadataSearchType = "book" | "author" | "series";

export type IntegrationHealth = {
  name: string;
  configured: boolean;
  status: string;
  message: string;
};

export type IntegrationSettings = {
  prowlarrUrl: string;
  prowlarrApiKey?: string;
  prowlarrApiKeyConfigured: boolean;
  qbittorrentUrl: string;
  qbittorrentUsername: string;
  qbittorrentPassword?: string;
  qbittorrentPasswordConfigured: boolean;
  transmissionUrl: string;
  transmissionUsername: string;
  transmissionPassword?: string;
  transmissionPasswordConfigured: boolean;
  sabnzbdUrl: string;
  sabnzbdApiKey?: string;
  sabnzbdApiKeyConfigured: boolean;
  sabnzbdUsername: string;
  sabnzbdPassword?: string;
  sabnzbdPasswordConfigured: boolean;
  ebookCategory: string;
  audiobookCategory: string;
  bookTorrentRoot: string;
};

export type IntegrationSettingsResponse = {
  settings: IntegrationSettings;
  persisted: boolean;
  integrations?: IntegrationHealth[];
};

export type LibrarySettings = {
  ebookLibraryRoot: string;
  audiobookLibraryRoot: string;
  namingAuthorFolder: string;
  namingBookFolder: string;
  namingFileName: string;
  namingSpaceReplacement: string;
  standardSearchLanguage: string;
  /** Read-only recycle-bin path (env-configured); absent on older backends. */
  recycleBin?: string;
};

export type LibrarySettingsResponse = {
  settings: LibrarySettings;
  persisted: boolean;
};

export type SystemStatus = {
  appName: string;
  instanceName?: string;
  version: string;
  databaseType: string;
  authentication?: string;
  runtimeName?: string;
  runtimeVersion?: string;
};

export type ReadinessStep = {
  id: string;
  title: string;
  status: "ready" | "warning" | "blocked";
  required: boolean;
  message: string;
  actionLabel?: string;
  targetView?: string;
};

export type ReadinessReport = {
  status: "ready" | "warning" | "blocked";
  summary: string;
  steps: ReadinessStep[];
  generatedAt: string;
};

export type ReadarrCompatibilityCategory = {
  id: string;
  title: string;
  status: "ready" | "partial" | "delegated";
  endpointCount: number;
  message: string;
  examples: string[];
};

export type ReadarrCompatibilityReport = {
  status: string;
  summary: string;
  authMode: "open" | "api_key";
  compatibleRoutes: number;
  readyAreas: number;
  partialAreas: number;
  delegatedAreas: number;
  categories: ReadarrCompatibilityCategory[];
  generatedAt: string;
};

export type ReadarrImportSettings = {
  baseUrl: string;
  apiKey: string;
  importAuthors: boolean;
  importBooks: boolean;
  importQualityProfiles: boolean;
  importRootFolders: boolean;
  importBookFiles: boolean;
  importTags: boolean;
  importLists: boolean;
  importListExclusions: boolean;
  importConfigResources: boolean;
};

export type ReadarrImportItem = {
  id?: string;
  title?: string;
  authorName?: string;
  path?: string;
  qualityProfile?: string;
  status?: string;
  message?: string;
};

export type ReadarrImportSection = {
  name: string;
  count: number;
  imported: number;
  skipped: number;
  errors?: string[];
  items?: ReadarrImportItem[];
};

export type ReadarrImportOutcome = {
  status: "ok" | "partial";
  dryRun: boolean;
  source: string;
  sections: ReadarrImportSection[];
  errors?: string[];
  generatedAt: string;
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
  wantedId?: string;
  wantedTitle?: string;
  wantedAuthor?: string;
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

export type DownloadPreferences = {
  client: string;
  savePath?: string;
  tempPathEnabled?: boolean;
  tempPath?: string;
  startPaused: boolean;
  downloadLimit: number;
  uploadLimit: number;
  alternativeDownloadLimit?: number;
  alternativeUploadLimit?: number;
  speedScheduleEnabled: boolean;
  queueingEnabled: boolean;
  maxActiveDownloads: number;
  maxActiveUploads: number;
  maxActiveTorrents: number;
  librarryPreferenceWriteScope?: string;
};

export type DownloadPreferencesUpdate = Partial<Omit<DownloadPreferences, "client" | "librarryPreferenceWriteScope">> & {
  client?: string;
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
  tags?: number[];
  sourceProvider?: string;
  sourceKey?: string;
  manualOverrides?: ManualOverride[];
  currentReleaseId?: string;
  currentReleaseScore?: number;
  lastSearchAt?: string;
  lastUpgradeSearchAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type MetadataRecordValues = {
  title?: string;
  authorName?: string;
  coverUrl?: string;
  format?: string;
  language?: string;
  publisher?: string;
  publishedDate?: string;
  firstPublishYear?: number;
  isbns?: string[];
  series?: string;
  seriesPosition?: string;
  matchedOn?: string[];
  sourceKey?: string;
};

export type ProviderMetadataRecord = {
  id: string;
  provider: string;
  providerKey: string;
  entityType: string;
  entityId?: string;
  confidence: number;
  fetchedAt: string;
  values: MetadataRecordValues;
};

export type MetadataFieldCandidate = {
  provider: string;
  providerKey: string;
  entityType: string;
  value: string;
  confidence: number;
  fetchedAt: string;
  matchedOn?: string[];
};

export type MetadataCorrectionRequest = {
  fieldName: string;
  value: string;
  reason?: string;
};

export type MetadataCorrectionBatchRequest = {
  corrections: MetadataCorrectionRequest[];
};

export type MetadataFieldEvidence = {
  fieldName: string;
  label: string;
  canonicalValue?: string;
  canonicalSource?: string;
  protected: boolean;
  reviewResolved: boolean;
  conflict: boolean;
  candidates?: MetadataFieldCandidate[];
};

export type MetadataProvenance = {
  wantedItem: WantedItem;
  records: ProviderMetadataRecord[];
  fields: MetadataFieldEvidence[];
  manualOverrides?: ManualOverride[];
  generatedAt: string;
};

export type MetadataReviewItem = {
  wantedItem: WantedItem;
  fields: MetadataFieldEvidence[];
  conflictCount: number;
  protectedCount: number;
  recordCount: number;
  candidateCount: number;
  lastFetchedAt?: string;
};

export type MetadataReviewQueue = {
  items: MetadataReviewItem[];
  generatedAt: string;
};

export type MetadataReviewConfirmRequest = {
  wantedIds?: string[];
  all?: boolean;
};

export type MetadataReviewConfirmOutcome = {
  status: string;
  itemsReviewed: number;
  fieldsConfirmed: number;
  skippedItems: number;
  items?: MetadataProvenance[];
  generatedAt: string;
};

export type ManualOverride = {
  fieldName: string;
  value?: string;
  reason?: string;
  createdAt: string;
  updatedAt: string;
};

export type WantedUpdateRequest = {
  title?: string;
  authorName?: string;
  coverUrl?: string;
  qualityProfile?: string;
  status?: string;
  monitored?: boolean;
  tags?: number[];
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
  missingBookPolicy: AuthorMissingBookPolicy;
  tags?: number[];
  lastSyncAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type AuthorMissingBookPolicy = "all" | "future" | "missing" | "existing" | "first" | "latest" | "none";

export type AuthorUpdateRequest = {
  authorName?: string;
  qualityProfile?: string;
  status?: string;
  monitorNewItems?: boolean;
  missingBookPolicy?: AuthorMissingBookPolicy;
  tags?: number[];
};

export type AuthorMonitorItemResult = {
  subscription: AuthorSubscription;
  resultsFound: number;
  wantedCreated: number;
  skippedCount?: number;
  skippedItems?: AuthorSkippedItem[];
  wantedItems?: WantedItem[];
  error?: string;
};

export type AuthorSkippedItem = {
  result: SearchResult;
  policy: AuthorMissingBookPolicy;
  reason: string;
  reviewId?: string;
};

export type AuthorMetadataReview = {
  id: string;
  authorSubscriptionId?: string;
  provider: string;
  candidateKey: string;
  title: string;
  authorName?: string;
  format: "ebook" | "audiobook";
  qualityProfile: string;
  tags?: number[];
  policy: AuthorMissingBookPolicy;
  reason: string;
  status: string;
  decision?: string;
  wantedId?: string;
  result: SearchResult;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
};

export type AuthorMetadataReviewDecision = {
  review: AuthorMetadataReview;
  wantedItem?: WantedItem;
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

export type AcquisitionQueueSummary = {
  total: number;
  needsSearch: number;
  readyToGrab: number;
  queued: number;
  importReady: number;
  imported: number;
  blocked: number;
};

export type AcquisitionQueueItem = {
  wantedItem: WantedItem;
  state: string;
  nextAction: string;
  releaseCount: number;
  approvedCount: number;
  rejectedCount: number;
  bestRelease?: ReleaseDecision;
  currentRelease?: ReleaseDecision;
  downloads?: DownloadStatus[];
  lastActivityAt?: string;
};

export type AcquisitionQueue = {
  items: AcquisitionQueueItem[];
  summary: AcquisitionQueueSummary;
  generatedAt: string;
};

export type WantedSearchOutcome = {
  wantedItem: WantedItem;
  releases: ReleaseDecision[];
};

type WantedSearchPayload = Omit<WantedSearchOutcome, "releases"> & {
  releases?: ReleaseDecision[] | null;
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
  autoMatched?: boolean;
  import?: LibraryImportOutcome;
  review?: ImportReview;
};

export type CompletedImportOutcome = {
  checked: number;
  imported: number;
  autoMatched: number;
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

export type BlocklistItem = {
  id: string;
  wantedItemId?: string;
  title: string;
  indexer: string;
  protocol: string;
  reason: string;
  source: string;
  infohash?: string;
  createdAt: string;
};

export type BlocklistClearOutcome = {
  removed: number;
};

export type MarkDownloadFailedOutcome = {
  blocklisted: boolean;
  searchTriggered: boolean;
};

export type WantedBulkUpdateRequest = {
  ids: string[];
  set?: {
    monitored?: boolean;
    qualityProfile?: string;
    format?: string;
  };
  delete?: boolean;
};

export type WantedBulkItemResult = {
  id: string;
  status: string;
  error?: string;
};

export type WantedBulkOutcome = {
  results: WantedBulkItemResult[];
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

async function apiError(response: Response, label: string) {
  let detail = "";
  try {
    const payload = (await response.clone().json()) as { error?: unknown; message?: unknown };
    detail = typeof payload.error === "string" ? payload.error : typeof payload.message === "string" ? payload.message : "";
  } catch {
    try {
      detail = (await response.text()).trim();
    } catch {
      detail = "";
    }
  }
  return `${label}: ${detail || response.status}`;
}

function arrayPayload<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function normalizeWantedSearchOutcome(outcome: WantedSearchPayload): WantedSearchOutcome {
  return {
    ...outcome,
    releases: arrayPayload(outcome.releases)
  };
}

export async function fetchProviderHealth(): Promise<ProviderHealth[]> {
  const response = await fetch(`${apiBase}/api/v1/providers/health`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Provider health failed"));
  }
  const payload = (await response.json()) as { providers?: ProviderHealth[] | null };
  return arrayPayload(payload.providers);
}

export async function searchMetadata(query: string, format: string, type: MetadataSearchType = "book", language = "English"): Promise<SearchResult[]> {
  const params = new URLSearchParams({
    query,
    type,
    format,
    language
  });
  const response = await fetch(`${apiBase}/api/v1/search?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Search failed: ${response.status}`);
  }
  const payload = (await response.json()) as { results?: SearchResult[] | null };
  return arrayPayload(payload.results);
}

export async function fetchIntegrationHealth(): Promise<IntegrationHealth[]> {
  const response = await fetch(`${apiBase}/api/v1/integrations/health`);
  if (!response.ok) {
    throw new Error(`Integration health failed: ${response.status}`);
  }
  const payload = (await response.json()) as { integrations?: IntegrationHealth[] | null };
  return arrayPayload(payload.integrations);
}

export async function fetchSystemStatus(): Promise<SystemStatus> {
  const response = await fetch(`${apiBase}/api/v1/system/status`);
  if (!response.ok) {
    throw new Error(await apiError(response, "System status failed"));
  }
  return (await response.json()) as SystemStatus;
}

export async function fetchReadiness(): Promise<ReadinessReport> {
  const response = await fetch(`${apiBase}/api/v1/readiness`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Readiness failed"));
  }
  return (await response.json()) as ReadinessReport;
}

export async function fetchReadarrCompatibility(): Promise<ReadarrCompatibilityReport> {
  const response = await fetch(`${apiBase}/api/v1/readarr/compatibility`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Readarr compatibility failed"));
  }
  return (await response.json()) as ReadarrCompatibilityReport;
}

export async function fetchIntegrationSettings(): Promise<IntegrationSettingsResponse> {
  const response = await fetch(`${apiBase}/api/v1/integrations/config`);
  if (!response.ok) {
    throw new Error(`Integration settings failed: ${response.status}`);
  }
  return (await response.json()) as IntegrationSettingsResponse;
}

export async function saveIntegrationSettings(settings: Partial<IntegrationSettings>): Promise<IntegrationSettingsResponse> {
  const response = await fetch(`${apiBase}/api/v1/integrations/config`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings)
  });
  if (!response.ok) {
    throw new Error(`Integration settings update failed: ${response.status}`);
  }
  return (await response.json()) as IntegrationSettingsResponse;
}

export async function fetchLibrarySettings(): Promise<LibrarySettingsResponse> {
  const response = await fetch(`${apiBase}/api/v1/library/config`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Library settings refresh failed"));
  }
  return (await response.json()) as LibrarySettingsResponse;
}

export async function saveLibrarySettings(settings: LibrarySettings): Promise<LibrarySettingsResponse> {
  const response = await fetch(`${apiBase}/api/v1/library/config`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Library settings update failed"));
  }
  return (await response.json()) as LibrarySettingsResponse;
}

export async function previewReadarrImport(settings: ReadarrImportSettings): Promise<ReadarrImportOutcome> {
  const response = await fetch(`${apiBase}/api/v1/readarr/import/preview`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Readarr import preview failed"));
  }
  return (await response.json()) as ReadarrImportOutcome;
}

export async function runReadarrImport(settings: ReadarrImportSettings): Promise<ReadarrImportOutcome> {
  const response = await fetch(`${apiBase}/api/v1/readarr/import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Readarr import failed"));
  }
  return (await response.json()) as ReadarrImportOutcome;
}

export async function searchReleases(query: string, format: string, language = "English"): Promise<Release[]> {
  const response = await fetch(`${apiBase}/api/v1/releases/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, format, languages: language && language !== "Any" ? [language] : [], limit: 12 })
  });
  if (!response.ok) {
    throw new Error(`Release search failed: ${response.status}`);
  }
  const payload = (await response.json()) as { releases?: Release[] | null };
  return arrayPayload(payload.releases);
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
  const payload = (await response.json()) as { downloads?: DownloadStatus[] | null };
  return arrayPayload(payload.downloads);
}

export async function fetchAcquisitionQueue(options: { status?: string; limit?: number } = {}): Promise<AcquisitionQueue> {
  const params = new URLSearchParams();
  if (options.status) params.set("status", options.status);
  if (options.limit) params.set("limit", String(options.limit));
  const response = await fetch(`${apiBase}/api/v1/acquisition/queue?${params.toString()}`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Acquisition queue refresh failed"));
  }
  return (await response.json()) as AcquisitionQueue;
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

export async function fetchDownloadPreferences(client = "qBittorrent"): Promise<DownloadPreferences> {
  const params = new URLSearchParams();
  if (client) params.set("client", client);
  const response = await fetch(`${apiBase}/api/v1/downloads/preferences?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Download preferences failed: ${response.status}`);
  }
  return (await response.json()) as DownloadPreferences;
}

export async function saveDownloadPreferences(update: DownloadPreferencesUpdate): Promise<DownloadPreferences> {
  const response = await fetch(`${apiBase}/api/v1/downloads/preferences`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(update)
  });
  if (!response.ok) {
    throw new Error(`Download preferences update failed: ${response.status}`);
  }
  return (await response.json()) as DownloadPreferences;
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

export async function fetchWanted(view?: "cutoff-unmet"): Promise<WantedItem[]> {
  const params = new URLSearchParams();
  if (view) params.set("view", view);
  const query = params.toString();
  const response = await fetch(`${apiBase}/api/v1/wanted${query ? `?${query}` : ""}`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted refresh failed"));
  }
  const payload = (await response.json()) as { wanted?: WantedItem[] | null };
  const items = arrayPayload(payload.wanted);
  // Server-defined views (cutoff-unmet) decide membership themselves; the
  // default list hides terminal statuses client-side.
  if (view) return items;
  return items.filter((item) => !["imported", "removed", "ignored"].includes((item.status || "").toLowerCase()));
}

export async function fetchQualityProfiles(): Promise<QualityProfile[]> {
  const response = await fetch(`${apiBase}/api/v1/quality-profiles`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Quality profiles refresh failed"));
  }
  const payload = (await response.json()) as { profiles?: QualityProfile[] | null };
  return arrayPayload(payload.profiles);
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
    throw new Error(await apiError(response, "Author subscriptions refresh failed"));
  }
  const payload = (await response.json()) as { authors?: AuthorSubscription[] | null };
  return arrayPayload(payload.authors);
}

export async function subscribeAuthor(result: SearchResult, format: string, qualityProfile = "standard", missingBookPolicy: AuthorMissingBookPolicy = "all"): Promise<AuthorSubscription> {
  const response = await fetch(`${apiBase}/api/v1/authors`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      result,
      format: format === "audiobook" ? "audiobook" : "ebook",
      qualityProfile,
      monitorNewItems: missingBookPolicy !== "none",
      missingBookPolicy
    })
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Author subscription failed"));
  }
  return (await response.json()) as AuthorSubscription;
}

export async function updateAuthorSubscription(authorID: string, request: AuthorUpdateRequest): Promise<AuthorSubscription> {
  const response = await fetch(`${apiBase}/api/v1/authors/${encodeURIComponent(authorID)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Author subscription update failed"));
  }
  return (await response.json()) as AuthorSubscription;
}

export async function deleteAuthorSubscription(authorID: string): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/authors/${encodeURIComponent(authorID)}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Author subscription remove failed"));
  }
}

export async function fetchAuthorMetadataReviews(status = "pending", limit = 100): Promise<AuthorMetadataReview[]> {
  const params = new URLSearchParams({ status, limit: String(limit) });
  const response = await fetch(`${apiBase}/api/v1/authors/metadata/review?${params.toString()}`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Author metadata review refresh failed"));
  }
  const payload = (await response.json()) as { reviews?: AuthorMetadataReview[] | null };
  return arrayPayload(payload.reviews);
}

export async function resolveAuthorMetadataReview(reviewId: string, action: "wanted" | "ignore"): Promise<AuthorMetadataReviewDecision> {
  const response = await fetch(`${apiBase}/api/v1/authors/metadata/review/${encodeURIComponent(reviewId)}/resolve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action })
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Author metadata review update failed"));
  }
  return (await response.json()) as AuthorMetadataReviewDecision;
}

export async function runAuthorMonitor(options: {
  authorIds?: string[];
  providerKeys?: string[];
  force?: boolean;
  limit?: number;
  searchLimit?: number;
} = {}): Promise<AuthorMonitorRun> {
  const response = await fetch(`${apiBase}/api/v1/authors/monitor`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      trigger: "manual",
      authorIds: options.authorIds ?? [],
      providerKeys: options.providerKeys ?? [],
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

export async function createWanted(result: SearchResult, format: string, qualityProfile = "standard", tags: number[] = []): Promise<WantedItem> {
  const wantedFormat = format === "audiobook" ? "audiobook" : "ebook";
  const response = await fetch(`${apiBase}/api/v1/wanted`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      result,
      format: wantedFormat,
      qualityProfile,
      tags
    })
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Mark wanted failed"));
  }
  return (await response.json()) as WantedItem;
}

export async function updateWanted(wantedID: string, request: WantedUpdateRequest): Promise<WantedItem> {
  const response = await fetch(`${apiBase}/api/v1/wanted/${encodeURIComponent(wantedID)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(`Wanted update failed: ${response.status}`);
  }
  return (await response.json()) as WantedItem;
}

export async function deleteWanted(wantedID: string): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/wanted/${encodeURIComponent(wantedID)}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(`Wanted delete failed: ${response.status}`);
  }
}

export async function clearWantedOverride(wantedID: string, fieldName: string): Promise<WantedItem> {
  const response = await fetch(`${apiBase}/api/v1/wanted/${encodeURIComponent(wantedID)}/overrides/${encodeURIComponent(fieldName)}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted override reset failed"));
  }
  return (await response.json()) as WantedItem;
}

export async function fetchWantedMetadata(wantedID: string): Promise<MetadataProvenance> {
  const response = await fetch(`${apiBase}/api/v1/wanted/metadata/${encodeURIComponent(wantedID)}`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted metadata provenance failed"));
  }
  return (await response.json()) as MetadataProvenance;
}

export async function fetchWantedMetadataReview(): Promise<MetadataReviewQueue> {
  const response = await fetch(`${apiBase}/api/v1/wanted/metadata/review`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted metadata review failed"));
  }
  return (await response.json()) as MetadataReviewQueue;
}

export async function confirmWantedMetadataReviewCanonical(request: MetadataReviewConfirmRequest): Promise<MetadataReviewConfirmOutcome> {
  const response = await fetch(`${apiBase}/api/v1/wanted/metadata/review/confirm-canonical`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted metadata review confirmation failed"));
  }
  return (await response.json()) as MetadataReviewConfirmOutcome;
}

export async function applyWantedMetadataCorrection(wantedID: string, request: MetadataCorrectionRequest): Promise<MetadataProvenance> {
  const response = await fetch(`${apiBase}/api/v1/wanted/metadata/${encodeURIComponent(wantedID)}/apply`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted metadata correction failed"));
  }
  return (await response.json()) as MetadataProvenance;
}

export async function applyWantedMetadataCorrections(wantedID: string, request: MetadataCorrectionBatchRequest): Promise<MetadataProvenance> {
  const response = await fetch(`${apiBase}/api/v1/wanted/metadata/${encodeURIComponent(wantedID)}/apply-bulk`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted metadata corrections failed"));
  }
  return (await response.json()) as MetadataProvenance;
}

export async function searchWantedReleases(wantedID: string, language = "English"): Promise<WantedSearchOutcome> {
  const response = await fetch(`${apiBase}/api/v1/wanted/${encodeURIComponent(wantedID)}/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ limit: 20, language })
  });
  if (!response.ok) {
    throw new Error(`Wanted release search failed: ${response.status}`);
  }
  return normalizeWantedSearchOutcome((await response.json()) as WantedSearchPayload);
}

export async function fetchWantedReleases(wantedID: string): Promise<WantedSearchOutcome> {
  const response = await fetch(`${apiBase}/api/v1/wanted/releases/${encodeURIComponent(wantedID)}`);
  if (!response.ok) {
    throw new Error(`Wanted release decisions refresh failed: ${response.status}`);
  }
  return normalizeWantedSearchOutcome((await response.json()) as WantedSearchPayload);
}

export async function grabWanted(wantedID: string, releaseID?: string, options: { paused?: boolean; force?: boolean } = {}): Promise<DownloadStatus> {
  const response = await fetch(`${apiBase}/api/v1/wanted/${encodeURIComponent(wantedID)}/grab`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      releaseId: releaseID,
      paused: options.paused ?? true,
      force: options.force ?? false
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
    throw new Error(await apiError(response, "History refresh failed"));
  }
  const payload = (await response.json()) as { events?: HistoryEvent[] | null };
  return arrayPayload(payload.events);
}

export async function fetchLibraryFiles(format = "any", limit = 100): Promise<LibraryFile[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (format && format !== "any") params.set("format", format);
  const response = await fetch(`${apiBase}/api/v1/library/files?${params.toString()}`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Library files refresh failed"));
  }
  const payload = (await response.json()) as { files?: LibraryFile[] | null };
  return arrayPayload(payload.files);
}

export async function fetchLibraryImportReviews(status = "pending", limit = 100): Promise<ImportReview[]> {
  const params = new URLSearchParams({ status, limit: String(limit) });
  const response = await fetch(`${apiBase}/api/v1/library/import-reviews?${params.toString()}`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Import review refresh failed"));
  }
  const payload = (await response.json()) as { reviews?: ImportReview[] | null };
  return arrayPayload(payload.reviews);
}

export async function scanLibrary(format = "any", options: { root?: string; limit?: number } = {}): Promise<LibraryScanOutcome> {
  const root = options.root?.trim();
  const body: { format: string; limit: number; root?: string } = {
    format,
    limit: options.limit ?? 1000
  };
  if (root) body.root = root;
  const response = await fetch(`${apiBase}/api/v1/library/scan`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Library scan failed"));
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
    throw new Error(await apiError(response, "Library import failed"));
  }
  return (await response.json()) as LibraryImportOutcome;
}

export async function importCompletedDownloads(options: {
  downloadIds?: string[];
  move?: boolean;
  importMode?: "copy" | "move" | "hardlink" | "hardlinkOrCopy";
  conflictAction?: "rename" | "replace" | "skip" | "fail";
  overwrite?: boolean;
  autoMatch?: boolean;
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
      autoMatch: options.autoMatch ?? true,
      limit: options.limit ?? 50
    })
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Completed import failed"));
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
    throw new Error(await apiError(response, "Import review update failed"));
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
    throw new Error(await apiError(response, "Bulk import review update failed"));
  }
  return (await response.json()) as ReviewBulkDecisionOutcome;
}

export async function fetchBlocklist(limit = 100): Promise<BlocklistItem[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  const response = await fetch(`${apiBase}/api/v1/librarry/blocklist?${params.toString()}`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Blocklist refresh failed"));
  }
  const payload = (await response.json()) as { items?: BlocklistItem[] | null };
  return arrayPayload(payload.items);
}

export async function removeBlocklistItem(id: string): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/librarry/blocklist/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Blocklist remove failed"));
  }
}

export async function clearBlocklist(ids?: string[]): Promise<BlocklistClearOutcome> {
  const response = await fetch(`${apiBase}/api/v1/librarry/blocklist/clear`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ids: ids ?? [] })
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Blocklist clear failed"));
  }
  return (await response.json()) as BlocklistClearOutcome;
}

export async function blocklistDownload(options: { downloadId: string; client?: string }): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/librarry/blocklist`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ downloadId: options.downloadId, client: options.client })
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Blocklist release failed"));
  }
}

export async function markDownloadFailed(options: {
  id: string;
  client?: string;
  blocklist: boolean;
  research: boolean;
}): Promise<MarkDownloadFailedOutcome> {
  const response = await fetch(`${apiBase}/api/v1/downloads/mark-failed`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(options)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Mark as failed request failed"));
  }
  return (await response.json()) as MarkDownloadFailedOutcome;
}

export async function bulkUpdateWanted(request: WantedBulkUpdateRequest): Promise<WantedBulkOutcome> {
  const response = await fetch(`${apiBase}/api/v1/wanted/bulk`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Wanted bulk update failed"));
  }
  return (await response.json()) as WantedBulkOutcome;
}

/* ------------------------------ Root folders ------------------------------- */

export type RootFolder = {
  id: string;
  name: string;
  path: string;
  mediaFormat: "ebook" | "audiobook";
  defaultQualityProfile: string;
  defaultMissingBookPolicy: AuthorMissingBookPolicy;
  /** Serialized tag list — the backend stores and echoes a plain string. */
  defaultTags?: string;
  isDefault: boolean;
  accessible: boolean;
  freeSpaceBytes?: number;
  createdAt: string;
};

/** Create/update payload: server owns id, accessibility, free space, createdAt. */
export type RootFolderRequest = Omit<RootFolder, "id" | "accessible" | "freeSpaceBytes" | "createdAt">;

export async function fetchRootFolders(): Promise<RootFolder[]> {
  const response = await fetch(`${apiBase}/api/v1/library/root-folders`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Root folders refresh failed"));
  }
  const payload = (await response.json()) as { rootFolders?: RootFolder[] | null };
  return arrayPayload(payload.rootFolders);
}

export async function createRootFolder(request: RootFolderRequest): Promise<RootFolder> {
  const response = await fetch(`${apiBase}/api/v1/library/root-folders`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Root folder create failed"));
  }
  const payload = (await response.json()) as { rootFolder: RootFolder };
  return payload.rootFolder;
}

export async function updateRootFolder(id: string, request: RootFolderRequest): Promise<RootFolder> {
  const response = await fetch(`${apiBase}/api/v1/library/root-folders/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Root folder update failed"));
  }
  const payload = (await response.json()) as { rootFolder: RootFolder };
  return payload.rootFolder;
}

/** Delete a root folder; a 409 refusal surfaces its {"error"} reason in the thrown message. */
export async function deleteRootFolder(id: string): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/library/root-folders/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Root folder delete failed"));
  }
}

/* --------------------------- Remote path mappings -------------------------- */

export type RemotePathMapping = {
  id: string;
  host: string;
  remotePrefix: string;
  localPrefix: string;
  createdAt: string;
};

export type RemotePathMappingRequest = Omit<RemotePathMapping, "id" | "createdAt">;

export async function fetchRemotePathMappings(): Promise<RemotePathMapping[]> {
  const response = await fetch(`${apiBase}/api/v1/library/remote-path-mappings`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Remote path mappings refresh failed"));
  }
  const payload = (await response.json()) as { mappings?: RemotePathMapping[] | null };
  return arrayPayload(payload.mappings);
}

export async function createRemotePathMapping(request: RemotePathMappingRequest): Promise<RemotePathMapping> {
  const response = await fetch(`${apiBase}/api/v1/library/remote-path-mappings`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Remote path mapping create failed"));
  }
  const payload = (await response.json()) as { mapping: RemotePathMapping };
  return payload.mapping;
}

export async function updateRemotePathMapping(id: string, request: RemotePathMappingRequest): Promise<RemotePathMapping> {
  const response = await fetch(`${apiBase}/api/v1/library/remote-path-mappings/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Remote path mapping update failed"));
  }
  const payload = (await response.json()) as { mapping: RemotePathMapping };
  return payload.mapping;
}

export async function deleteRemotePathMapping(id: string): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/library/remote-path-mappings/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Remote path mapping delete failed"));
  }
}

/* ------------------------------ Library rename ----------------------------- */

/*
 * Mirrors backend/internal/library/types.go (RenameFilesRequest /
 * RenameFilesOutcome, served by POST /api/v1/library/files/rename[/preview]):
 * the request selects files by id or path — there is no server-side format
 * selector — so callers resolve ids via fetchLibraryFiles(format) first.
 */

export type LibraryRenameRequest = {
  ids?: string[];
  paths?: string[];
  overwrite?: boolean;
};

export type LibraryRenamePreviewItem = {
  file: LibraryFile;
  sourcePath: string;
  destinationPath: string;
  relativePath: string;
  exists: boolean;
  noop: boolean;
};

export type LibraryRenameResult = {
  preview: LibraryRenamePreviewItem;
  file?: LibraryFile;
  status: string;
  message?: string;
};

export type LibraryRenameOutcome = {
  requested: number;
  renamed: number;
  skipped: number;
  errored: number;
  previews: LibraryRenamePreviewItem[];
  results: LibraryRenameResult[];
};

type LibraryRenamePayload = Omit<LibraryRenameOutcome, "previews" | "results"> & {
  previews?: LibraryRenamePreviewItem[] | null;
  results?: LibraryRenameResult[] | null;
};

function normalizeLibraryRenameOutcome(payload: LibraryRenamePayload): LibraryRenameOutcome {
  return {
    ...payload,
    previews: arrayPayload(payload.previews),
    results: arrayPayload(payload.results)
  };
}

export async function previewLibraryRename(request: LibraryRenameRequest): Promise<LibraryRenameOutcome> {
  const response = await fetch(`${apiBase}/api/v1/library/files/rename/preview`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Rename preview failed"));
  }
  return normalizeLibraryRenameOutcome((await response.json()) as LibraryRenamePayload);
}

export async function renameLibraryFiles(request: LibraryRenameRequest): Promise<LibraryRenameOutcome> {
  const response = await fetch(`${apiBase}/api/v1/library/files/rename`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Rename failed"));
  }
  return normalizeLibraryRenameOutcome((await response.json()) as LibraryRenamePayload);
}

/* ---------------------- System tasks / health / disk (M5) ------------------ */

/** One scheduler-registered worker: interval cadence plus last/next run facts. */
export type SystemTask = {
  id: string;
  name: string;
  interval: string;
  lastRunAt?: string;
  lastOutcome?: string;
  lastError?: string;
  nextRunAt?: string;
  running: boolean;
};

export async function fetchSystemTasks(): Promise<SystemTask[]> {
  const response = await fetch(`${apiBase}/api/v1/system/tasks`);
  if (!response.ok) {
    throw new Error(await apiError(response, "System tasks refresh failed"));
  }
  const payload = (await response.json()) as { tasks?: SystemTask[] | null };
  return arrayPayload(payload.tasks);
}

export type RunSystemTaskOutcome = {
  started: boolean;
  /** True when the server answered 409 — the task is already running. */
  alreadyRunning?: boolean;
  message?: string;
};

/**
 * Trigger a task run. 202 → {started:true}. A 409 (already running) is a
 * semi-expected outcome, so it is returned rather than thrown; every other
 * failure throws.
 */
export async function runSystemTask(id: string): Promise<RunSystemTaskOutcome> {
  const response = await fetch(`${apiBase}/api/v1/system/tasks/${encodeURIComponent(id)}/run`, {
    method: "POST"
  });
  if (response.status === 409) {
    return { started: false, alreadyRunning: true, message: await apiError(response, "Task already running") };
  }
  if (!response.ok) {
    throw new Error(await apiError(response, "Task run failed"));
  }
  const payload = (await response.json()) as { started?: boolean };
  return { started: Boolean(payload.started) };
}

export type SystemHealthSeverity = "ok" | "warning" | "error";

export type SystemHealthCheck = {
  id: string;
  severity: SystemHealthSeverity;
  name: string;
  message: string;
};

export async function fetchSystemHealth(): Promise<SystemHealthCheck[]> {
  const response = await fetch(`${apiBase}/api/v1/system/health`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Health checks refresh failed"));
  }
  const payload = (await response.json()) as { checks?: SystemHealthCheck[] | null };
  return arrayPayload(payload.checks);
}

export type DiskSpaceEntry = {
  path: string;
  label: string;
  freeBytes: number;
  totalBytes: number;
};

export async function fetchDiskSpace(): Promise<DiskSpaceEntry[]> {
  const response = await fetch(`${apiBase}/api/v1/system/diskspace`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Disk space refresh failed"));
  }
  const payload = (await response.json()) as { disks?: DiskSpaceEntry[] | null };
  return arrayPayload(payload.disks);
}

/* --------------------------- Notification targets (M5) --------------------- */

export type NotificationTargetType = "webhook" | "ntfy" | "discord" | "telegram";

export type NotificationTriggers = {
  onGrab: boolean;
  onImport: boolean;
  onUpgrade: boolean;
  onDownloadFailure: boolean;
  onHealthIssue: boolean;
};

export type NotificationTarget = {
  id: string;
  name: string;
  type: NotificationTargetType;
  /** Schema-driven per-type fields (url, topic, token, webhookUrl, botToken, chatId…). */
  settings: Record<string, string>;
  triggers: NotificationTriggers;
  enabled: boolean;
  createdAt: string;
};

/**
 * Create/update payload: server owns id and createdAt. On update, a blank
 * secret setting means "keep the stored value" (backend contract) — secrets
 * are never echoed back for re-submission.
 */
export type NotificationTargetRequest = Omit<NotificationTarget, "id" | "createdAt">;

export async function fetchNotificationTargets(): Promise<NotificationTarget[]> {
  const response = await fetch(`${apiBase}/api/v1/notifications`);
  if (!response.ok) {
    throw new Error(await apiError(response, "Notification targets refresh failed"));
  }
  const payload = (await response.json()) as { targets?: NotificationTarget[] | null };
  return arrayPayload(payload.targets);
}

export async function createNotificationTarget(request: NotificationTargetRequest): Promise<NotificationTarget> {
  const response = await fetch(`${apiBase}/api/v1/notifications`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Notification target create failed"));
  }
  const payload = (await response.json()) as { target: NotificationTarget };
  return payload.target;
}

export async function updateNotificationTarget(
  id: string,
  request: NotificationTargetRequest
): Promise<NotificationTarget> {
  const response = await fetch(`${apiBase}/api/v1/notifications/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request)
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Notification target update failed"));
  }
  const payload = (await response.json()) as { target: NotificationTarget };
  return payload.target;
}

export async function deleteNotificationTarget(id: string): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/notifications/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Notification target delete failed"));
  }
}

export type NotificationTestOutcome = {
  ok: boolean;
  error?: string;
};

export async function testNotificationTarget(id: string): Promise<NotificationTestOutcome> {
  const response = await fetch(`${apiBase}/api/v1/notifications/${encodeURIComponent(id)}/test`, {
    method: "POST"
  });
  if (!response.ok) {
    throw new Error(await apiError(response, "Notification test failed"));
  }
  return (await response.json()) as NotificationTestOutcome;
}
