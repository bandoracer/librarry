import {
  Activity,
  BookOpen,
  CheckSquare,
  ChevronsDown,
  ChevronsUp,
  CheckCircle2,
  Clock3,
  Database,
  Download,
  FilterX,
  FileCheck2,
  FileSearch,
  FolderSearch,
  HardDriveDownload,
  History as HistoryIcon,
  Library,
  Pause,
  Pencil,
  Play,
  RadioTower,
  RefreshCw,
  Search,
  Settings,
  SlidersHorizontal,
  Square,
  Tags,
  Trash2,
  TrendingUp,
  UploadCloud,
  UserPlus
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  applyWantedMetadataCorrection,
  applyWantedMetadataCorrections,
  clearWantedOverride,
  confirmWantedMetadataReviewCanonical,
  createWanted,
  deleteAuthorSubscription,
  deleteWanted,
  fetchAcquisitionQueue,
  fetchAuthorMetadataReviews,
  fetchAuthorSubscriptions,
  fetchDownloadDetails,
  fetchDownloadPreferences,
  fetchDownloadResources,
  fetchIntegrationHealth,
  fetchIntegrationSettings,
  fetchDownloads,
  fetchHistory,
  fetchLibraryFiles,
  fetchLibraryImportReviews,
  fetchLibrarySettings,
  fetchProviderHealth,
  fetchQualityProfiles,
  fetchReadiness,
  fetchReadarrCompatibility,
  fetchSystemStatus,
  fetchWanted,
  fetchWantedMetadata,
  fetchWantedMetadataReview,
  fetchWantedReleases,
  getStoredAPIKey,
  grabManualDownload,
  grabWanted,
  grabRelease,
  importCompletedDownloads,
  importLibraryFile,
  previewReadarrImport,
  recoverFailedDownloads,
  rebalanceDownloads,
  resolveLibraryImportReview,
  resolveLibraryImportReviewsBulk,
  resolveAuthorMetadataReview,
  runAuthorMonitor,
  runUpgradeSearch,
  runWantedFeedSync,
  runReadarrImport,
  runWantedMonitor,
  runDownloadAction,
  runDownloadCategoryAction,
  runDownloadFileAction,
  runDownloadTagAction,
  runDownloadTrackerAction,
  saveQualityProfile,
  saveIntegrationSettings,
  saveLibrarySettings,
  saveDownloadPreferences,
  scanLibrary,
  searchMetadata,
  searchReleases,
  searchWantedReleases,
  setStoredAPIKey,
  subscribeAuthor,
  updateAuthorSubscription,
  updateWanted,
  type AuthorMetadataReview,
  type AuthorMonitorRun,
  type AuthorMissingBookPolicy,
  type AuthorSkippedItem,
  type AuthorSubscription,
  type AcquisitionQueue,
  type AcquisitionQueueItem,
  type DownloadAction,
  type CompletedImportOutcome,
  type DownloadDetails,
  type DownloadFileAction,
  type DownloadPreferences,
  type DownloadRebalancePlan,
  type DownloadResources,
  type DownloadStatus,
  type DownloadTrackerAction,
  type FailedDownloadRun,
  type FeedSyncRun,
  type HistoryEvent,
  type ImportReview,
  type IntegrationHealth,
  type IntegrationSettings,
  type LibraryFile,
  type LibraryImportOutcome,
  type LibraryScanOutcome,
  type LibrarySettings,
  type MetadataSearchType,
  type MetadataCorrectionRequest,
  type MetadataFieldEvidence,
  type MetadataFieldCandidate,
  type MetadataProvenance,
  type MetadataReviewConfirmOutcome,
  type MetadataReviewItem,
  type MetadataReviewQueue,
  type MonitorRun,
  type ProviderHealth,
  type ProviderMetadataRecord,
  type QualityProfile,
  type ReadarrCompatibilityReport,
  type ReadarrImportItem,
  type ReadarrImportOutcome,
  type ReadarrImportSettings,
  type ReadinessReport,
  type ReleaseDecision,
  type Release,
  type ReviewBulkDecisionOutcome,
  type SearchResult,
  type SystemStatus,
  type UpgradeRun,
  type WantedItem
} from "./lib/api";
import {
  seedDownloadDetailsByKey,
  seedDownloadPreferencesByClient,
  seedDownloadResourcesByClient,
  seedDownloads,
  seedIntegrations,
  seedProviders,
  seedReadarrCompatibility,
  seedResults,
  seedWantedItems,
  seedWantedMetadataByID,
  seedWantedMetadataReview
} from "./lib/seed";

const navItems = [
  {
    id: "dashboard",
    label: "Dashboard",
    icon: Activity,
    title: "Operations dashboard",
    description: "Watch provider health, acquisition status, monitor runs, and recent activity."
  },
  {
    id: "library",
    label: "Library",
    icon: Library,
    title: "Library",
    description: "Manage monitored authors, wanted books, missing items, and local files."
  },
  {
    id: "search",
    label: "Search",
    icon: Search,
    title: "Metadata search",
    description: "Resolve works, editions, identifiers, and provider confidence before acquisition."
  },
  {
    id: "wanted",
    label: "Wanted",
    icon: Clock3,
    title: "Wanted queue",
    description: "Review monitored books, author subscriptions, feed sync, upgrade search, and release decisions."
  },
  {
    id: "downloads",
    label: "Queue",
    icon: HardDriveDownload,
    title: "Download activity",
    description: "Review book jobs in external clients and import completed files."
  },
  {
    id: "imports",
    label: "Imports",
    icon: UploadCloud,
    title: "Library imports",
    description: "Scan library roots, import completed downloads, and resolve pending import reviews."
  },
  {
    id: "providers",
    label: "Providers",
    icon: Database,
    title: "Provider health",
    description: "Check metadata providers and acquisition integration readiness."
  },
  {
    id: "settings",
    label: "Settings",
    icon: Settings,
    title: "Release policy",
    description: "Tune quality profiles used by search, feeds, failed-download recovery, and upgrades."
  }
] as const;

type ViewID = (typeof navItems)[number]["id"];
type DownloadScope = "all" | "librarry";
type DownloadStateFilter = "all" | "active" | "paused" | "complete" | "failed";
type LibraryFormatFilter = "all" | "ebook" | "audiobook";
type WantedViewFilter = "missing" | "review" | "wanted" | "grabbed" | "all";
type ReleaseDecisionFilter = "all" | "approved" | "rejected";
type SearchMode = Extract<MetadataSearchType, "book" | "author">;
type SearchConfidenceFilter = "all" | SearchResult["confidence"];
type SearchEvidenceFilter = "all" | "identifiers" | "published" | "series";
type SearchEvidenceTone = "high" | "medium" | "review" | "neutral";
type SearchEvidenceChip = { label: string; tone?: SearchEvidenceTone };
type SearchEvidenceItem = { label: string; value: string; detail: string };

const authorMissingPolicyOptions: AuthorMissingBookPolicy[] = ["all", "future", "none"];
const searchModeOptions: SearchMode[] = ["book", "author"];
const searchConfidenceOptions: SearchResult["confidence"][] = ["high", "medium", "review"];

function firstAuthorName(result: SearchResult) {
  return result.work.authors?.[0]?.name || "Unknown author";
}

function searchResultCanBeWanted(result: SearchResult) {
  return result.kind !== "author";
}

function searchResultKey(result: SearchResult) {
  return `${result.provider}:${result.kind}:${result.work.id}:${result.edition?.id || result.rawSourceKey || ""}`;
}

function searchResultWantedSourceKey(result: SearchResult) {
  return result.edition?.id || result.work.id || result.rawSourceKey || "";
}

function searchResultWantedFormat(result: SearchResult, currentFormat: string) {
  if (result.edition?.format === "audiobook") return "audiobook";
  return wantedFormat(currentFormat);
}

function searchResultExistingWanted(result: SearchResult, items: WantedItem[], currentFormat: string) {
  if (!searchResultCanBeWanted(result)) return undefined;
  const wantedFormatValue = searchResultWantedFormat(result, currentFormat);
  const provider = result.provider.trim().toLowerCase();
  const sourceKey = searchResultWantedSourceKey(result).trim().toLowerCase();
  const title = normalizedWantedText(result.work.title);
  const author = normalizedWantedText(firstAuthorName(result));
  return items.find((item) => {
    if (item.format !== wantedFormatValue) return false;
    const itemProvider = (item.sourceProvider || "").trim().toLowerCase();
    const itemSourceKey = (item.sourceKey || "").trim().toLowerCase();
    if (provider && sourceKey && itemProvider === provider && itemSourceKey === sourceKey) return true;

    const itemTitle = normalizedWantedText(item.title);
    const itemAuthor = normalizedWantedText(item.authorName || "");
    return Boolean(title && itemTitle === title && (!author || !itemAuthor || itemAuthor === author));
  });
}

function searchResultScoreLabel(result: SearchResult) {
  if (!Number.isFinite(result.score) || result.score <= 0) return "unscored";
  return result.score <= 1 ? `${Math.round(result.score * 100)}%` : result.score.toFixed(1);
}

function searchResultMatchChips(result: SearchResult) {
  const chips: SearchEvidenceChip[] = [
    { label: `score ${searchResultScoreLabel(result)}`, tone: result.confidence }
  ];
  result.matchedOn.forEach((field) => chips.push({ label: searchMatchFieldLabel(field), tone: "neutral" }));
  if (result.kind !== "author" && searchResultIdentifierSummary(result, 1)) chips.push({ label: "identifier", tone: "high" });
  if (result.kind !== "author" && searchResultPublishedLabel(result)) chips.push({ label: "published", tone: "neutral" });
  if (result.kind !== "author" && searchResultSeriesLabel(result)) chips.push({ label: "series", tone: "neutral" });
  if (result.kind === "author" && searchResultProviderKey(result)) chips.push({ label: "author id", tone: "neutral" });
  return uniqueEvidenceChips(chips).slice(0, 5);
}

function searchResultEvidenceSummary(result: SearchResult, currentFormat: string): SearchEvidenceItem[] {
  const sourceKey = searchResultSourceIdentity(result);
  const matchedFields = searchResultMatchedFieldsLabel(result);
  if (result.kind === "author") {
    return [
      {
        label: "Match",
        value: `${result.confidence} · ${searchResultScoreLabel(result)}`,
        detail: searchResultConfidenceDescription(result)
      },
      {
        label: "Author identity",
        value: sourceKey,
        detail: "Provider-backed author ID used for monitored-author refreshes."
      },
      {
        label: "Target",
        value: wantedFormat(currentFormat),
        detail: "New wanted items from this author will use this format policy."
      },
      {
        label: "Matched fields",
        value: matchedFields || "Provider rank",
        detail: "Fields that contributed to the normalized match score."
      }
    ];
  }
  return [
    {
      label: "Match",
      value: `${result.confidence} · ${searchResultScoreLabel(result)}`,
      detail: searchResultConfidenceDescription(result)
    },
    {
      label: "Edition evidence",
      value: searchResultEditionSummary(result, currentFormat) || "Any format",
      detail: searchResultEditionSubline(result)
    },
    {
      label: "Matched fields",
      value: matchedFields || "Provider rank",
      detail: "Fields that contributed to the normalized match score."
    },
    {
      label: "Source identity",
      value: sourceKey,
      detail: "Stored with provider records so future corrections can be traced."
    }
  ];
}

function searchResultConfidenceDescription(result: SearchResult) {
  switch (result.confidence) {
    case "high":
      return "Strong enough to create a wanted item without review in normal cases.";
    case "medium":
      return "Likely match; check edition evidence before marking wanted.";
    case "review":
      return "Low-confidence match that should be reviewed before acquisition.";
  }
}

function searchResultMatchedFieldsLabel(result: SearchResult) {
  return Array.from(new Set(result.matchedOn.map(searchMatchFieldLabel).filter(Boolean))).join(", ");
}

function searchMatchFieldLabel(field: string) {
  switch (field.toLowerCase()) {
    case "isbn":
      return "ISBN";
    case "asin":
      return "ASIN";
    case "title":
      return "title";
    case "author":
      return "author";
    case "series":
      return "series";
    default:
      return field;
  }
}

function uniqueEvidenceChips(chips: SearchEvidenceChip[]) {
  const seen = new Set<string>();
  return chips.filter((chip) => {
    const key = chip.label.toLowerCase();
    if (!chip.label || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function searchResultWantedReviewReasons(result: SearchResult) {
  if (!searchResultCanBeWanted(result)) return [];
  const reasons: string[] = [];
  const matched = new Set(result.matchedOn.map((field) => field.toLowerCase()));
  const hasIdentifier = Boolean(result.edition?.asin || result.edition?.isbns?.length);
  const matchedIdentifier = matched.has("isbn") || matched.has("asin") || matched.has("identifier");
  const matchedTitleAndAuthor = matched.has("title") && matched.has("author");

  if (result.confidence === "review") {
    reasons.push("Provider match is low confidence.");
  } else if (result.confidence === "medium") {
    reasons.push("Provider match is medium confidence.");
  }
  if (!hasIdentifier && !matchedIdentifier) {
    reasons.push("No ISBN or ASIN evidence is attached to this edition.");
  }
  if (!matchedIdentifier && !matchedTitleAndAuthor) {
    reasons.push("The match did not include both title and author evidence.");
  }
  return Array.from(new Set(reasons));
}

function searchResultNeedsWantedReview(result: SearchResult) {
  return searchResultWantedReviewReasons(result).length > 0;
}

function uniqueSearchProviders(results: SearchResult[]) {
  return Array.from(new Set(results.map((result) => result.provider).filter(Boolean))).sort((a, b) => a.localeCompare(b));
}

function searchResultVisibleForFilters(
  result: SearchResult,
  filters: { provider: string; confidence: SearchConfidenceFilter; evidence: SearchEvidenceFilter }
) {
  if (filters.provider && result.provider !== filters.provider) return false;
  if (filters.confidence !== "all" && result.confidence !== filters.confidence) return false;
  return searchResultHasEvidence(result, filters.evidence);
}

function searchResultHasEvidence(result: SearchResult, evidence: SearchEvidenceFilter) {
  if (evidence === "all") return true;
  if (result.kind === "author") return false;
  switch (evidence) {
    case "identifiers":
      return Boolean(result.edition?.asin || result.edition?.isbns?.length);
    case "published":
      return Boolean(result.edition?.publisher || result.edition?.publishedDate || result.work.firstPublishYear);
    case "series":
      return Boolean(result.work.series || result.work.seriesPosition);
  }
}

function searchResultTitle(result: SearchResult) {
  return result.kind === "author" ? firstAuthorName(result) : result.work.title;
}

function searchResultSubtitle(result: SearchResult) {
  if (result.kind === "author") {
    return result.work.description || result.rawSourceKey || result.provider;
  }
  const parts = compactStringList([
    firstAuthorName(result),
    searchResultSeriesLabel(result),
    result.work.firstPublishYear ? String(result.work.firstPublishYear) : ""
  ]);
  return parts.join(" · ");
}

function searchResultProviderKey(result: SearchResult) {
  return result.work.authors?.[0]?.id || result.rawSourceKey || result.work.providerIds?.[0] || result.work.id || "Unknown";
}

function searchResultSourceIdentity(result: SearchResult) {
  if (result.kind === "author") return searchResultProviderKey(result);
  return searchResultWantedSourceKey(result) || result.work.providerIds?.[0] || "Unknown";
}

function searchResultEditionSummary(result: SearchResult, currentFormat: string) {
  if (result.kind === "author") {
    return wantedFormat(currentFormat);
  }
  return compactStringList([
    result.edition?.format || "any",
    languageLabel(result.edition?.language)
  ]).join(" · ");
}

function searchResultEditionSubline(result: SearchResult) {
  if (result.kind === "author") {
    return searchResultProviderKey(result);
  }
  return compactStringList([
    searchResultPublishedLabel(result),
    searchResultIdentifierSummary(result, 1),
    searchResultSeriesLabel(result)
  ]).join(" · ") || "No edition evidence";
}

function searchResultPublishedLabel(result: SearchResult) {
  return compactStringList([
    result.edition?.publishedDate || (result.work.firstPublishYear ? String(result.work.firstPublishYear) : ""),
    result.edition?.publisher
  ]).join(" · ");
}

function searchResultIdentifierLabel(result: SearchResult, limit = 2) {
  return searchResultIdentifierSummary(result, limit) || "None";
}

function searchResultIdentifierSummary(result: SearchResult, limit = 2) {
  const identifiers = compactStringList([
    ...(result.edition?.isbns ?? []).slice(0, limit),
    result.edition?.asin ? `ASIN ${result.edition.asin}` : ""
  ]);
  const remaining = Math.max(0, (result.edition?.isbns?.length ?? 0) - limit);
  if (!identifiers.length) return "";
  return remaining > 0 ? `${identifiers.join(", ")} +${remaining} more` : identifiers.join(", ");
}

function searchResultSeriesLabel(result: SearchResult) {
  if (!result.work.series) return "";
  return compactStringList([
    result.work.series,
    result.work.seriesPosition ? `#${result.work.seriesPosition}` : ""
  ]).join(" ");
}

function languageLabel(value?: string) {
  const normalized = (value || "").trim().toLowerCase();
  const labels: Record<string, string> = {
    en: "English",
    eng: "English",
    es: "Spanish",
    spa: "Spanish",
    fr: "French",
    fre: "French",
    fra: "French",
    de: "German",
    ger: "German",
    deu: "German",
    it: "Italian",
    ita: "Italian",
    ja: "Japanese",
    jpn: "Japanese",
    pt: "Portuguese",
    por: "Portuguese"
  };
  return labels[normalized] || value || "";
}

function compactStringList(values: Array<string | number | null | undefined>) {
  return values.map((value) => String(value ?? "").trim()).filter(Boolean);
}

function emptyIntegrationSettings(): IntegrationSettings {
  return {
    prowlarrUrl: "",
    prowlarrApiKey: "",
    prowlarrApiKeyConfigured: false,
    qbittorrentUrl: "",
    qbittorrentUsername: "",
    qbittorrentPassword: "",
    qbittorrentPasswordConfigured: false,
    transmissionUrl: "",
    transmissionUsername: "",
    transmissionPassword: "",
    transmissionPasswordConfigured: false,
    sabnzbdUrl: "",
    sabnzbdApiKey: "",
    sabnzbdApiKeyConfigured: false,
    sabnzbdUsername: "",
    sabnzbdPassword: "",
    sabnzbdPasswordConfigured: false,
    ebookCategory: "books-ebook",
    audiobookCategory: "books-audiobook",
    bookTorrentRoot: "/data/torrents/books"
  };
}

function integrationSettingsForm(settings: IntegrationSettings): IntegrationSettings {
  return {
    ...settings,
    prowlarrApiKey: "",
    qbittorrentPassword: "",
    transmissionPassword: "",
    sabnzbdApiKey: "",
    sabnzbdPassword: ""
  };
}

function emptyLibrarySettings(): LibrarySettings {
  return {
    ebookLibraryRoot: "/data/media/books/ebooks",
    audiobookLibraryRoot: "/data/media/books/audiobooks",
    namingAuthorFolder: "{Author}",
    namingBookFolder: "{Title}",
    namingFileName: "{Title}{Ext}",
    namingSpaceReplacement: ""
  };
}

function emptyReadarrImportSettings(): ReadarrImportSettings {
  return {
    baseUrl: "",
    apiKey: "",
    importAuthors: true,
    importBooks: true,
    importQualityProfiles: true,
    importRootFolders: true,
    importBookFiles: true,
    importTags: true,
    importLists: true,
    importListExclusions: true,
    importConfigResources: true
  };
}

export function App() {
  const [activeView, setActiveView] = useState<ViewID>("library");
  const [providers, setProviders] = useState<ProviderHealth[]>(seedProviders);
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [readiness, setReadiness] = useState<ReadinessReport | null>(null);
  const [readarrCompatibility, setReadarrCompatibility] = useState<ReadarrCompatibilityReport>(seedReadarrCompatibility);
  const [results, setResults] = useState<SearchResult[]>(seedResults);
  const [integrations, setIntegrations] = useState<IntegrationHealth[]>(seedIntegrations);
  const [integrationSettings, setIntegrationSettings] = useState<IntegrationSettings>(() => emptyIntegrationSettings());
  const [integrationForm, setIntegrationForm] = useState<IntegrationSettings>(() => emptyIntegrationSettings());
  const [integrationSettingsPersisted, setIntegrationSettingsPersisted] = useState(false);
  const [librarySettings, setLibrarySettings] = useState<LibrarySettings>(() => emptyLibrarySettings());
  const [librarySettingsForm, setLibrarySettingsForm] = useState<LibrarySettings>(() => emptyLibrarySettings());
  const [librarySettingsPersisted, setLibrarySettingsPersisted] = useState(false);
  const [readarrImportForm, setReadarrImportForm] = useState<ReadarrImportSettings>(() => emptyReadarrImportSettings());
  const [readarrImportOutcome, setReadarrImportOutcome] = useState<ReadarrImportOutcome | null>(null);
  const [releases, setReleases] = useState<Release[]>([]);
  const [wantedItems, setWantedItems] = useState<WantedItem[]>(seedWantedItems);
  const [wantedReleases, setWantedReleases] = useState<ReleaseDecision[]>([]);
  const [wantedMetadata, setWantedMetadata] = useState<MetadataProvenance | null>(null);
  const [wantedMetadataReview, setWantedMetadataReview] = useState<MetadataReviewQueue | null>(seedWantedMetadataReview);
  const [acquisitionQueue, setAcquisitionQueue] = useState<AcquisitionQueue | null>(null);
  const [downloads, setDownloads] = useState<DownloadStatus[]>(seedDownloads);
  const [historyEvents, setHistoryEvents] = useState<HistoryEvent[]>([]);
  const [libraryFiles, setLibraryFiles] = useState<LibraryFile[]>([]);
  const [importReviews, setImportReviews] = useState<ImportReview[]>([]);
  const [qualityProfiles, setQualityProfiles] = useState<QualityProfile[]>([]);
  const [authorSubscriptions, setAuthorSubscriptions] = useState<AuthorSubscription[]>([]);
  const [libraryScan, setLibraryScan] = useState<LibraryScanOutcome | null>(null);
  const [libraryImport, setLibraryImport] = useState<LibraryImportOutcome | null>(null);
  const [completedImport, setCompletedImport] = useState<CompletedImportOutcome | null>(null);
  const [bulkReviewOutcome, setBulkReviewOutcome] = useState<ReviewBulkDecisionOutcome | null>(null);
  const [metadataReviewConfirmOutcome, setMetadataReviewConfirmOutcome] = useState<MetadataReviewConfirmOutcome | null>(null);
  const [monitorRun, setMonitorRun] = useState<MonitorRun | null>(null);
  const [authorMonitorRun, setAuthorMonitorRun] = useState<AuthorMonitorRun | null>(null);
  const [authorMetadataReviews, setAuthorMetadataReviews] = useState<AuthorMetadataReview[]>([]);
  const [feedSyncRun, setFeedSyncRun] = useState<FeedSyncRun | null>(null);
  const [failedDownloadRun, setFailedDownloadRun] = useState<FailedDownloadRun | null>(null);
  const [queueRebalancePlan, setQueueRebalancePlan] = useState<DownloadRebalancePlan | null>(null);
  const [upgradeRun, setUpgradeRun] = useState<UpgradeRun | null>(null);
  const [downloadStatus, setDownloadStatus] = useState<DownloadStatus | null>(null);
  const [downloadDetails, setDownloadDetails] = useState<DownloadDetails | null>(null);
  const [downloadResources, setDownloadResources] = useState<DownloadResources | null>(seedDownloadResourcesByClient.qBittorrent);
  const [downloadPreferences, setDownloadPreferences] = useState<DownloadPreferences | null>(seedDownloadPreferencesByClient.qBittorrent);
  const [selectedID, setSelectedID] = useState(seedResults[0] ? searchResultKey(seedResults[0]) : "");
  const [pendingWantedReview, setPendingWantedReview] = useState<SearchResult | null>(null);
  const [selectedWantedID, setSelectedWantedID] = useState("");
  const [selectedDownloadKeys, setSelectedDownloadKeys] = useState<string[]>([]);
  const [selectedMetadataReviewIDs, setSelectedMetadataReviewIDs] = useState<string[]>([]);
  const [selectedImportReviewIDs, setSelectedImportReviewIDs] = useState<string[]>([]);
  const [selectedImportReviewWantedIDs, setSelectedImportReviewWantedIDs] = useState<Record<string, string>>({});
  const [downloadScope, setDownloadScope] = useState<DownloadScope>("all");
  const [wantedViewFilter, setWantedViewFilter] = useState<WantedViewFilter>("missing");
  const [wantedReleaseFilter, setWantedReleaseFilter] = useState<ReleaseDecisionFilter>("all");
  const [authorMissingPolicy, setAuthorMissingPolicy] = useState<AuthorMissingBookPolicy>("all");
  const [downloadClientFilter, setDownloadClientFilter] = useState("");
  const [downloadResourceClient, setDownloadResourceClient] = useState("qBittorrent");
  const [downloadCategoryFilter, setDownloadCategoryFilter] = useState("");
  const [downloadStateFilter, setDownloadStateFilter] = useState<DownloadStateFilter>("all");
  const [downloadTextFilter, setDownloadTextFilter] = useState("");
  const [libraryFormatFilter, setLibraryFormatFilter] = useState<LibraryFormatFilter>("all");
  const [libraryTextFilter, setLibraryTextFilter] = useState("");
  const [searchMode, setSearchMode] = useState<SearchMode>("book");
  const [bookQuery, setBookQuery] = useState("Project Hail Mary");
  const [authorQuery, setAuthorQuery] = useState("Andy Weir");
  const [searchFiltersOpen, setSearchFiltersOpen] = useState(false);
  const [searchProviderFilter, setSearchProviderFilter] = useState("");
  const [searchConfidenceFilter, setSearchConfidenceFilter] = useState<SearchConfidenceFilter>("all");
  const [searchEvidenceFilter, setSearchEvidenceFilter] = useState<SearchEvidenceFilter>("all");
  const [importPath, setImportPath] = useState("");
  const [libraryScanRoot, setLibraryScanRoot] = useState("");
  const [libraryImportMode, setLibraryImportMode] = useState<"copy" | "move" | "hardlink" | "hardlinkOrCopy">("copy");
  const [libraryConflictAction, setLibraryConflictAction] = useState<"rename" | "replace" | "skip" | "fail">("rename");
  const [format, setFormat] = useState("any");
  const [apiState, setAPIState] = useState<"checking" | "live" | "offline">("checking");
  const [apiKeyInput, setAPIKeyInput] = useState(() => getStoredAPIKey());
  const [isSearching, setIsSearching] = useState(false);
  const [isSearchingReleases, setIsSearchingReleases] = useState(false);
  const [isMarkingWanted, setIsMarkingWanted] = useState(false);
  const [isSearchingWanted, setIsSearchingWanted] = useState(false);
  const [isRefreshingAcquisitionQueue, setIsRefreshingAcquisitionQueue] = useState(false);
  const [acquisitionActionID, setAcquisitionActionID] = useState("");
  const [isLoadingWantedReleases, setIsLoadingWantedReleases] = useState(false);
  const [isLoadingWantedMetadata, setIsLoadingWantedMetadata] = useState(false);
  const [isSavingWantedEdit, setIsSavingWantedEdit] = useState(false);
  const [isRemovingWanted, setIsRemovingWanted] = useState(false);
  const [isConfirmingMetadataReviews, setIsConfirmingMetadataReviews] = useState(false);
  const [applyingMetadataCandidateID, setApplyingMetadataCandidateID] = useState("");
  const [applyingMetadataRecordID, setApplyingMetadataRecordID] = useState("");
  const [isRunningMonitor, setIsRunningMonitor] = useState(false);
  const [isSubscribingAuthor, setIsSubscribingAuthor] = useState(false);
  const [isRunningAuthorMonitor, setIsRunningAuthorMonitor] = useState(false);
  const [authorMonitorTargetKey, setAuthorMonitorTargetKey] = useState("");
  const [updatingAuthorID, setUpdatingAuthorID] = useState("");
  const [removingAuthorID, setRemovingAuthorID] = useState("");
  const [markingAuthorSkippedKey, setMarkingAuthorSkippedKey] = useState("");
  const [authorReviewActionID, setAuthorReviewActionID] = useState("");
  const [isRunningFeedSync, setIsRunningFeedSync] = useState(false);
  const [isRunningUpgrade, setIsRunningUpgrade] = useState(false);
  const [isScanningLibrary, setIsScanningLibrary] = useState(false);
  const [isImportingLibrary, setIsImportingLibrary] = useState(false);
  const [isImportingCompleted, setIsImportingCompleted] = useState(false);
  const [isRecoveringFailed, setIsRecoveringFailed] = useState(false);
  const [isRebalancingDownloads, setIsRebalancingDownloads] = useState(false);
  const [isRefreshingDownloads, setIsRefreshingDownloads] = useState(false);
  const [isLoadingDownloadDetails, setIsLoadingDownloadDetails] = useState(false);
  const [isAddingDownload, setIsAddingDownload] = useState(false);
  const [isLoadingDownloadResources, setIsLoadingDownloadResources] = useState(false);
  const [isLoadingDownloadPreferences, setIsLoadingDownloadPreferences] = useState(false);
  const [isSavingDownloadPreferences, setIsSavingDownloadPreferences] = useState(false);
  const [isSavingIntegrationSettings, setIsSavingIntegrationSettings] = useState(false);
  const [isSavingLibrarySettings, setIsSavingLibrarySettings] = useState(false);
  const [isPreviewingReadarrImport, setIsPreviewingReadarrImport] = useState(false);
  const [isRunningReadarrImport, setIsRunningReadarrImport] = useState(false);
  const [savingProfileID, setSavingProfileID] = useState("");
  const [reviewActionID, setReviewActionID] = useState("");
  const [downloadActionID, setDownloadActionID] = useState("");
  const [downloadResourceActionID, setDownloadResourceActionID] = useState("");
  const [clearingWantedOverrideField, setClearingWantedOverrideField] = useState("");
  const [grabbingWantedReleaseID, setGrabbingWantedReleaseID] = useState("");
  const [trackerURL, setTrackerURL] = useState("");
  const [downloadNameInput, setDownloadNameInput] = useState("");
  const [downloadTagsInput, setDownloadTagsInput] = useState("");
  const [downloadCategoryInput, setDownloadCategoryInput] = useState("");
  const [downloadSavePathInput, setDownloadSavePathInput] = useState("");
  const [resourceCategoryName, setResourceCategoryName] = useState("");
  const [resourceCategoryNewName, setResourceCategoryNewName] = useState("");
  const [resourceCategoryPath, setResourceCategoryPath] = useState("");
  const [resourceTagName, setResourceTagName] = useState("");
  const [resourceTagNewName, setResourceTagNewName] = useState("");
  const [downloadRebalanceMax, setDownloadRebalanceMax] = useState("3");
  const [preferenceSavePath, setPreferenceSavePath] = useState("");
  const [preferenceTempPath, setPreferenceTempPath] = useState("");
  const [preferenceTempPathEnabled, setPreferenceTempPathEnabled] = useState(false);
  const [preferenceStartPaused, setPreferenceStartPaused] = useState(false);
  const [preferenceQueueingEnabled, setPreferenceQueueingEnabled] = useState(true);
  const [preferenceSpeedScheduleEnabled, setPreferenceSpeedScheduleEnabled] = useState(false);
  const [preferenceDownloadLimitKiB, setPreferenceDownloadLimitKiB] = useState("");
  const [preferenceUploadLimitKiB, setPreferenceUploadLimitKiB] = useState("");
  const [preferenceAltDownloadLimitKiB, setPreferenceAltDownloadLimitKiB] = useState("");
  const [preferenceAltUploadLimitKiB, setPreferenceAltUploadLimitKiB] = useState("");
  const [preferenceMaxActiveDownloads, setPreferenceMaxActiveDownloads] = useState("-1");
  const [preferenceMaxActiveUploads, setPreferenceMaxActiveUploads] = useState("-1");
  const [preferenceMaxActiveTorrents, setPreferenceMaxActiveTorrents] = useState("-1");
  const [manualGrabURL, setManualGrabURL] = useState("");
  const [manualGrabFile, setManualGrabFile] = useState<File | null>(null);
  const [manualGrabFileInputKey, setManualGrabFileInputKey] = useState(0);
  const [manualGrabTitle, setManualGrabTitle] = useState("");
  const [manualGrabFormat, setManualGrabFormat] = useState("ebook");
  const [manualGrabClient, setManualGrabClient] = useState("");
  const [wantedEditTitle, setWantedEditTitle] = useState("");
  const [wantedEditAuthor, setWantedEditAuthor] = useState("");
  const [wantedEditCoverURL, setWantedEditCoverURL] = useState("");
  const [wantedEditQualityProfile, setWantedEditQualityProfile] = useState("standard");
  const [wantedEditMonitored, setWantedEditMonitored] = useState(true);
  const [downloadLimitKiB, setDownloadLimitKiB] = useState("");
  const [uploadLimitKiB, setUploadLimitKiB] = useState("");
  const [releaseError, setReleaseError] = useState("");
  const [wantedError, setWantedError] = useState("");
  const [acquisitionError, setAcquisitionError] = useState("");
  const [monitorError, setMonitorError] = useState("");
  const [authorError, setAuthorError] = useState("");
  const [feedError, setFeedError] = useState("");
  const [upgradeError, setUpgradeError] = useState("");
  const [historyError, setHistoryError] = useState("");
  const [libraryError, setLibraryError] = useState("");
  const [downloadError, setDownloadError] = useState("");
  const [settingsError, setSettingsError] = useState("");
  const [settingsNotice, setSettingsNotice] = useState("");

  function downloadListOptions(scope = downloadScope) {
    return {
      tag: downloadScopeTag(scope),
      client: downloadClientFilter.trim(),
      category: downloadCategoryFilter.trim()
    };
  }

  useEffect(() => {
    Promise.all([fetchProviderHealth(), fetchIntegrationHealth(), fetchSystemStatus(), fetchReadiness(), fetchReadarrCompatibility()])
      .then(([nextProviders, nextIntegrations, nextStatus, nextReadiness, nextCompatibility]) => {
        setProviders(nextProviders);
        setIntegrations(nextIntegrations);
        setSystemStatus(nextStatus);
        setReadiness(nextReadiness);
        setReadarrCompatibility(nextCompatibility);
        setAPIState("live");
      })
      .catch(() => {
        setAPIState("offline");
      });
    fetchIntegrationSettings()
      .then((response) => {
        setIntegrationSettings(response.settings);
        setIntegrationForm(integrationSettingsForm(response.settings));
        setIntegrationSettingsPersisted(response.persisted);
        if (response.integrations?.length) {
          setIntegrations(response.integrations);
        }
      })
      .catch((error) => {
        setSettingsError(error instanceof Error ? error.message : "Integration settings refresh failed");
      });
    fetchLibrarySettings()
      .then((response) => {
        setLibrarySettings(response.settings);
        setLibrarySettingsForm(response.settings);
        setLibrarySettingsPersisted(response.persisted);
      })
      .catch((error) => {
        setSettingsError(error instanceof Error ? error.message : "Library settings refresh failed");
      });
    fetchDownloads(downloadListOptions(downloadScope))
      .then(setDownloads)
      .catch((error) => {
        setDownloadError(error instanceof Error ? `${error.message}. Showing demo acquisition jobs.` : "Download refresh failed. Showing demo acquisition jobs.");
      });
    fetchWanted()
      .then((items) => {
        setWantedItems(items);
        setSelectedWantedID(items[0]?.id ?? "");
      })
      .catch((error) => {
        setWantedError(error instanceof Error ? error.message : "Wanted refresh failed");
      });
    fetchAcquisitionQueue({ status: "all", limit: 100 })
      .then((queue) => {
        setAcquisitionQueue(queue);
        setWantedItems((current) => mergeWanted(current, queue.items.map((item) => item.wantedItem)));
        setSelectedWantedID((current) => current || queue.items[0]?.wantedItem.id || "");
      })
      .catch((error) => {
        setAcquisitionError(error instanceof Error ? error.message : "Acquisition queue refresh failed");
      });
    fetchWantedMetadataReview()
      .then(setWantedMetadataReview)
      .catch(() => null);
    fetchQualityProfiles()
      .then(setQualityProfiles)
      .catch((error) => {
        setSettingsError(error instanceof Error ? error.message : "Quality profiles refresh failed");
      });
    fetchAuthorSubscriptions()
      .then(setAuthorSubscriptions)
      .catch((error) => {
        setAuthorError(error instanceof Error ? error.message : "Author subscriptions refresh failed");
      });
    fetchAuthorMetadataReviews()
      .then(setAuthorMetadataReviews)
      .catch(() => {
        setAuthorMetadataReviews([]);
      });
    fetchHistory()
      .then(setHistoryEvents)
      .catch((error) => {
        setHistoryError(error instanceof Error ? error.message : "History refresh failed");
      });
    fetchLibraryFiles()
      .then(setLibraryFiles)
      .catch((error) => {
        setLibraryError(error instanceof Error ? error.message : "Library refresh failed");
      });
    fetchLibraryImportReviews()
      .then(setImportReviews)
      .catch((error) => {
        setLibraryError(error instanceof Error ? error.message : "Import review refresh failed");
      });
  }, []);

  useEffect(() => {
    const availableIDs = new Set((wantedMetadataReview?.items ?? []).map((review) => review.wantedItem.id).filter(Boolean));
    setSelectedMetadataReviewIDs((current) => current.filter((id) => availableIDs.has(id)));
  }, [wantedMetadataReview]);

  useEffect(() => {
    const availableIDs = new Set(importReviews.map((review) => review.id).filter(Boolean));
    setSelectedImportReviewIDs((current) => current.filter((id) => availableIDs.has(id)));
    setSelectedImportReviewWantedIDs((current) => {
      const next: Record<string, string> = {};
      Object.entries(current).forEach(([id, wantedID]) => {
        if (availableIDs.has(id)) next[id] = wantedID;
      });
      return next;
    });
  }, [importReviews]);

  useEffect(() => {
    refreshDownloadResources(true);
    refreshDownloadPreferences(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [downloadResourceClient, integrations]);

  const searchProviderOptions = useMemo(() => uniqueSearchProviders(results), [results]);
  const visibleSearchResults = useMemo(
    () => results.filter((result) => searchResultVisibleForFilters(result, {
      provider: searchProviderFilter,
      confidence: searchConfidenceFilter,
      evidence: searchEvidenceFilter
    })),
    [results, searchConfidenceFilter, searchEvidenceFilter, searchProviderFilter]
  );
  const activeSearchFilterCount = [
    searchProviderFilter,
    searchConfidenceFilter !== "all" ? searchConfidenceFilter : "",
    searchEvidenceFilter !== "all" ? searchEvidenceFilter : ""
  ].filter(Boolean).length;
  const wantedBySearchKey = useMemo(() => {
    const entries = new Map<string, WantedItem>();
    results.forEach((result) => {
      const item = searchResultExistingWanted(result, wantedItems, format);
      if (item) entries.set(searchResultKey(result), item);
    });
    return entries;
  }, [format, results, wantedItems]);
  const selected = useMemo(
    () => visibleSearchResults.find((result) => searchResultKey(result) === selectedID || result.work.id === selectedID) ?? visibleSearchResults[0] ?? results[0],
    [results, selectedID, visibleSearchResults]
  );
  const selectedSearchKey = selected ? searchResultKey(selected) : "";
  const selectedExistingWanted = selectedSearchKey ? wantedBySearchKey.get(selectedSearchKey) : undefined;
  const query = searchMode === "author" ? authorQuery : bookQuery;
  const selectedIsBookCandidate = Boolean(selected && searchResultCanBeWanted(selected));
  const selectedCanBeWanted = selectedIsBookCandidate && !selectedExistingWanted;
  const selectedCanSearchReleases = selectedIsBookCandidate;
  const selectedWantedReviewReasons = selected && selectedCanBeWanted ? searchResultWantedReviewReasons(selected) : [];
  const pendingWantedReviewKey = pendingWantedReview ? searchResultKey(pendingWantedReview) : "";
  const wantedPresence = useMemo(() => wantedPresenceMap(wantedItems, libraryFiles), [wantedItems, libraryFiles]);
  const wantedMetadataReviewByID = useMemo(() => metadataReviewMap(wantedMetadataReview), [wantedMetadataReview]);
  const metadataReviewSummary = useMemo(() => summarizeMetadataReview(wantedMetadataReview), [wantedMetadataReview]);
  const wantedSummary = useMemo(() => summarizeWantedItems(wantedItems, wantedPresence), [wantedItems, wantedPresence]);
  const authorSubscriptionStatsByKey = useMemo(
    () => buildAuthorSubscriptionStatsMap(authorSubscriptions, wantedItems, wantedPresence, wantedMetadataReviewByID),
    [authorSubscriptions, wantedItems, wantedMetadataReviewByID, wantedPresence]
  );
  const libraryAuthorRows = useMemo(
    () => buildLibraryAuthorRows(authorSubscriptions, wantedItems, wantedPresence),
    [authorSubscriptions, wantedItems, wantedPresence]
  );
  const librarySummary = useMemo(
    () => ({
      authors: libraryAuthorRows.length,
      monitoredAuthors: authorSubscriptions.length,
      monitoredBooks: wantedItems.filter((item) => item.monitored).length,
      missing: wantedSummary.missing,
      grabbed: wantedSummary.grabbed,
      present: wantedSummary.present,
      files: libraryFiles.length,
      manualOverrides: wantedItems.reduce((count, item) => count + (item.manualOverrides?.length ?? 0), 0)
    }),
    [authorSubscriptions.length, libraryAuthorRows.length, libraryFiles.length, wantedItems, wantedSummary]
  );
  const visibleLibraryAuthorRows = useMemo(
    () => libraryAuthorRows.filter((row) => libraryAuthorVisibleForFilter(row, libraryTextFilter, libraryFormatFilter)),
    [libraryAuthorRows, libraryFormatFilter, libraryTextFilter]
  );
  const visibleLibraryBooks = useMemo(
    () => wantedItems
      .filter((item) => libraryBookVisibleForFilter(item, wantedPresence.get(item.id), libraryTextFilter, libraryFormatFilter))
      .sort((a, b) => {
        const stateDelta = libraryPresenceRank(wantedPresence.get(a.id)) - libraryPresenceRank(wantedPresence.get(b.id));
        if (stateDelta !== 0) return stateDelta;
        return `${a.authorName ?? ""} ${a.title}`.localeCompare(`${b.authorName ?? ""} ${b.title}`);
      }),
    [libraryFormatFilter, libraryTextFilter, wantedItems, wantedPresence]
  );
  const visibleWantedItems = useMemo(
    () => wantedItems.filter((item) => wantedItemVisibleForFilter(item, wantedPresence.get(item.id), wantedViewFilter, wantedMetadataReviewByID.has(item.id))),
    [wantedItems, wantedMetadataReviewByID, wantedPresence, wantedViewFilter]
  );
  const visibleMetadataReviewIDs = useMemo(
    () => visibleWantedItems.map((item) => item.id).filter((id) => wantedMetadataReviewByID.has(id)),
    [visibleWantedItems, wantedMetadataReviewByID]
  );
  const selectedMetadataReviewSet = useMemo(() => new Set(selectedMetadataReviewIDs), [selectedMetadataReviewIDs]);
  const selectedMetadataReviewCount = visibleMetadataReviewIDs.filter((id) => selectedMetadataReviewSet.has(id)).length;
  const allMetadataReviewsSelected = visibleMetadataReviewIDs.length > 0 && visibleMetadataReviewIDs.every((id) => selectedMetadataReviewSet.has(id));
  const selectedWanted = useMemo(
    () => visibleWantedItems.find((item) => item.id === selectedWantedID) ?? visibleWantedItems[0],
    [visibleWantedItems, selectedWantedID]
  );
  const acquisitionQueueRows = acquisitionQueue?.items ?? [];
  const selectedAcquisitionQueueItem = useMemo(
    () => acquisitionQueueRows.find((item) => item.wantedItem.id === selectedWanted?.id),
    [acquisitionQueueRows, selectedWanted?.id]
  );
  const highlightedAcquisitionItems = useMemo(
    () => acquisitionQueueRows
      .filter((item) => item.state !== "imported")
      .slice(0, 6),
    [acquisitionQueueRows]
  );
  const selectedWantedQualityProfiles = useMemo(
    () => qualityProfiles.filter((profile) => !selectedWanted || profile.mediaFormat === "any" || profile.mediaFormat === selectedWanted.format),
    [qualityProfiles, selectedWanted]
  );
  const wantedReleaseSummary = useMemo(() => summarizeReleaseDecisions(wantedReleases), [wantedReleases]);
  const visibleWantedReleases = useMemo(
    () => wantedReleases.filter((release) => releaseDecisionVisibleForFilter(release, wantedReleaseFilter)),
    [wantedReleases, wantedReleaseFilter]
  );
  const filteredDownloads = useMemo(
    () => downloads.filter((download) => downloadMatchesFilters(download, {
      client: downloadClientFilter,
      category: downloadCategoryFilter,
      state: downloadStateFilter,
      text: downloadTextFilter
    })),
    [downloads, downloadCategoryFilter, downloadClientFilter, downloadStateFilter, downloadTextFilter]
  );
  const downloadClientOptions = useMemo(() => uniqueDownloadClients(downloads), [downloads]);
  const downloadResourceClientOptions = useMemo(() => uniqueDownloadResourceClients(downloads, integrations), [downloads, integrations]);
  const resourceClientIsTransmission = downloadResourceClient.toLowerCase() === "transmission";
  const resourceClientIsSABnzbd = downloadResourceClient.toLowerCase() === "sabnzbd";
  const resourceClientIsQbittorrent = downloadResourceClient.toLowerCase() === "qbittorrent";
  const resourceClientSupportsPreferences = resourceClientIsQbittorrent || resourceClientIsTransmission;
  const resourceClientConfigured = Boolean(integrations.find((integration) => integration.name.toLowerCase() === downloadResourceClient.toLowerCase())?.configured);
  const downloadCategoryOptions = useMemo(() => uniqueDownloadCategories(downloads, downloadResources), [downloads, downloadResources]);
  const downloadIntegrationStatuses = useMemo(() => downloadClientHealthRows(integrations), [integrations]);
  const selectableDownloadKeys = useMemo(() => filteredDownloads.map(downloadKey).filter(Boolean), [filteredDownloads]);
  const selectedDownloadKeySet = useMemo(() => new Set(selectedDownloadKeys), [selectedDownloadKeys]);
  const selectedDownloads = useMemo(() => filteredDownloads.filter((download) => selectedDownloadKeySet.has(downloadKey(download))), [filteredDownloads, selectedDownloadKeySet]);
  const selectedActionDownloadIDs = selectedDownloads.map((download) => download.id);
  const allDownloadsSelected = selectableDownloadKeys.length > 0 && selectableDownloadKeys.every((key) => selectedDownloadKeySet.has(key));
  const selectedDownloadsSupportRecheck = selectedDownloads.every((download) => supportsDownloadAction(download, "recheck"));
  const selectedDownloadsSupportPriority = selectedDownloads.every((download) => supportsDownloadAction(download, "topPriority"));
  const selectedDownloadsSupportForceStart = selectedDownloads.length > 0 && selectedDownloads.every((download) => supportsDownloadAction(download, "forceStart"));
  const selectedDownloadsSupportQbitControls = selectedDownloads.length > 0 && selectedDownloads.every(downloadSupportsQbitManagerActions);
  const downloadQueueStats = useMemo(() => summarizeDownloads(filteredDownloads), [filteredDownloads]);
  const dashboardMetadataReviews = (wantedMetadataReview?.items ?? []).slice(0, 4);
  const dashboardAuthorReviews = authorMetadataReviews.slice(0, 4);
  const dashboardImportReviews = importReviews.slice(0, 4);
  const dashboardBlockedItems = acquisitionQueueRows.filter((item) => item.state === "blocked").slice(0, 4);
  const dashboardFailedDownloads = downloads.filter(downloadNeedsRecovery);
  const dashboardFailedDownloadRows = dashboardFailedDownloads.slice(0, 4);
  const reviewInboxTotal =
    metadataReviewSummary.items +
    authorMetadataReviews.length +
    importReviews.length +
    (acquisitionQueue?.summary.blocked ?? dashboardBlockedItems.length) +
    dashboardFailedDownloads.length;
  const visibleImportReviews = useMemo(() => importReviews.slice(0, 6), [importReviews]);
  const selectableImportReviewIDs = useMemo(() => visibleImportReviews.map((review) => review.id).filter(Boolean), [visibleImportReviews]);
  const selectedImportReviewSet = useMemo(() => new Set(selectedImportReviewIDs), [selectedImportReviewIDs]);
  const selectedImportReviews = useMemo(
    () => visibleImportReviews.filter((review) => selectedImportReviewSet.has(review.id)),
    [selectedImportReviewSet, visibleImportReviews]
  );
  const allImportReviewsSelected = selectableImportReviewIDs.length > 0 && selectableImportReviewIDs.every((id) => selectedImportReviewSet.has(id));
  const selectedImportReviewsCanImport =
    selectedImportReviews.length > 0 && selectedImportReviews.every((review) => Boolean(importReviewResolvedWantedID(review, selectedImportReviewWantedIDs)));
  const libraryNamingPreview = useMemo(() => libraryNamingPreviewPath(librarySettingsForm), [librarySettingsForm]);
  const selectedAuthorFormat = selected ? wantedFormat(selected.edition?.format ?? format) : wantedFormat(format);
  const selectedAuthorSubscription = useMemo(() => {
    const author = selected?.work.authors?.[0];
    if (!author) return undefined;
    const authorID = author.id.trim().toLowerCase();
    const authorName = author.name.trim().toLowerCase();
    return authorSubscriptions.find((subscription) => {
      if (subscription.format !== selectedAuthorFormat) return false;
      const providerKey = subscription.providerKey.trim().toLowerCase();
      const subscriptionName = subscription.authorName.trim().toLowerCase();
      return Boolean((authorID && providerKey === authorID) || (authorName && subscriptionName === authorName));
    });
  }, [authorSubscriptions, selected, selectedAuthorFormat]);

  function clearSearchFilters() {
    setSearchProviderFilter("");
    setSearchConfidenceFilter("all");
    setSearchEvidenceFilter("all");
  }

  function selectSearchResult(result: SearchResult) {
    setSelectedID(searchResultKey(result));
    if (pendingWantedReview && searchResultKey(pendingWantedReview) !== searchResultKey(result)) {
      setPendingWantedReview(null);
    }
  }

  useEffect(() => {
    if (searchProviderFilter && !searchProviderOptions.includes(searchProviderFilter)) {
      setSearchProviderFilter("");
    }
  }, [searchProviderFilter, searchProviderOptions]);

  useEffect(() => {
    if (pendingWantedReview && !results.some((result) => searchResultKey(result) === searchResultKey(pendingWantedReview))) {
      setPendingWantedReview(null);
    }
  }, [pendingWantedReview, results]);

  useEffect(() => {
    if (!visibleWantedItems.length) {
      setSelectedWantedID("");
      setWantedReleases([]);
      return;
    }
    if (!visibleWantedItems.some((item) => item.id === selectedWantedID)) {
      setSelectedWantedID(visibleWantedItems[0].id);
      setWantedReleases([]);
    }
  }, [visibleWantedItems, selectedWantedID]);

  useEffect(() => {
    setWantedEditTitle(selectedWanted?.title ?? "");
    setWantedEditAuthor(selectedWanted?.authorName ?? "");
    setWantedEditCoverURL(selectedWanted?.coverUrl ?? "");
    setWantedEditQualityProfile(selectedWanted?.qualityProfile ?? "standard");
    setWantedEditMonitored(selectedWanted?.monitored ?? true);
    setWantedMetadata(null);
  }, [selectedWanted?.id, selectedWanted?.title, selectedWanted?.authorName, selectedWanted?.coverUrl, selectedWanted?.qualityProfile, selectedWanted?.monitored]);

  useEffect(() => {
    if (activeView !== "wanted" || !selectedWanted?.id) return;
    let canceled = false;
    setIsLoadingWantedMetadata(true);
    fetchWantedMetadata(selectedWanted.id)
      .then((provenance) => {
        if (canceled) return;
        setWantedMetadata(provenance);
        setWantedItems((current) => mergeWanted(current, [provenance.wantedItem]));
        setAPIState("live");
      })
      .catch((error) => {
        if (canceled) return;
        const seeded = seedWantedMetadataByID[selectedWanted.id];
        if (seeded) {
          setWantedMetadata(seeded);
          return;
        }
        setWantedError(error instanceof Error ? error.message : "Wanted metadata provenance failed");
      })
      .finally(() => {
        if (!canceled) setIsLoadingWantedMetadata(false);
      });
    return () => {
      canceled = true;
    };
  }, [activeView, selectedWanted?.id]);

  useEffect(() => {
    if (activeView !== "wanted" || !selectedWanted?.id) return;
    let canceled = false;
    setIsLoadingWantedReleases(true);
    fetchWantedReleases(selectedWanted.id)
      .then((outcome) => {
        if (canceled) return;
        setWantedItems((current) => mergeWanted(current, [outcome.wantedItem]));
        setWantedReleases(outcome.releases);
        setWantedReleaseFilter("all");
        setAPIState("live");
      })
      .catch((error) => {
        if (!canceled) setWantedError(error instanceof Error ? error.message : "Wanted release decisions refresh failed");
      })
      .finally(() => {
        if (!canceled) setIsLoadingWantedReleases(false);
      });
    return () => {
      canceled = true;
    };
  }, [activeView, selectedWanted?.id]);

  async function runSearch() {
    if (!query.trim()) return;
    setIsSearching(true);
    try {
      const nextResults = await searchMetadata(query, searchMode === "author" ? "any" : format, searchMode);
      setResults(nextResults);
      setSelectedID(nextResults[0] ? searchResultKey(nextResults[0]) : "");
      setPendingWantedReview(null);
      setReleases([]);
      setAPIState("live");
    } catch {
      setAPIState("offline");
    } finally {
      setIsSearching(false);
    }
  }

  async function runReleaseSearch() {
    if (!selected || !searchResultCanBeWanted(selected)) return;
    const releaseQuery = selected?.work.title ?? query;
    if (!releaseQuery.trim()) return;
    setIsSearchingReleases(true);
    setReleaseError("");
    try {
      const nextReleases = await searchReleases(releaseQuery, selected?.edition?.format ?? format);
      setReleases(nextReleases);
    } catch (error) {
      setReleaseError(error instanceof Error ? error.message : "Release search failed");
    } finally {
      setIsSearchingReleases(false);
    }
  }

  async function runGrab(release: Release) {
    setReleaseError("");
    try {
      const status = await grabRelease(release, selected?.edition?.format ?? format);
      setDownloadStatus(status);
      await refreshDownloads();
    } catch (error) {
      setReleaseError(error instanceof Error ? error.message : "Grab failed");
    }
  }

  async function addManualDownload() {
    const releaseUrl = manualGrabURL.trim();
    if (!releaseUrl && !manualGrabFile) return;
    setIsAddingDownload(true);
    setDownloadError("");
    try {
      const status = await grabManualDownload({
        releaseUrl: releaseUrl || undefined,
        file: manualGrabFile ?? undefined,
        title: manualGrabTitle.trim() || undefined,
        format: manualGrabFormat,
        client: manualGrabClient || undefined,
        paused: true
      });
      setDownloadStatus(status);
      setManualGrabURL("");
      setManualGrabFile(null);
      setManualGrabFileInputKey((current) => current + 1);
      setManualGrabTitle("");
      await refreshDownloads();
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Manual grab failed");
    } finally {
      setIsAddingDownload(false);
    }
  }

  async function markWantedResult(result = selected, options: { force?: boolean } = {}) {
    if (!result || !searchResultCanBeWanted(result)) return;
    const existingWanted = searchResultExistingWanted(result, wantedItems, format);
    if (existingWanted) {
      setSelectedID(searchResultKey(result));
      setPendingWantedReview(null);
      openWantedItem(existingWanted);
      return;
    }
    if (!options.force && searchResultNeedsWantedReview(result)) {
      setWantedError("");
      setSelectedID(searchResultKey(result));
      setPendingWantedReview(result);
      return;
    }
    setIsMarkingWanted(true);
    setWantedError("");
    setPendingWantedReview(null);
    setSelectedID(searchResultKey(result));
    setActiveView("wanted");
    try {
      const item = await createWanted(result, result.edition?.format ?? format);
      setWantedItems((current) => mergeWanted(current, [item]));
      setSelectedWantedID(item.id);
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Mark wanted failed");
    } finally {
      setIsMarkingWanted(false);
    }
  }

  async function subscribeSelectedAuthor() {
    await subscribeAuthorResult(selected);
  }

  async function subscribeAuthorResult(result = selected) {
    if (!result?.work.authors?.[0]) return;
    setIsSubscribingAuthor(true);
    setAuthorError("");
    setSelectedID(result.work.id);
    setActiveView("wanted");
    try {
      const subscription = await subscribeAuthor(result, result.edition?.format ?? format, "standard", authorMissingPolicy);
      setAuthorSubscriptions((current) => mergeAuthorSubscriptions(current, [subscription]));
      setAPIState("live");
      await runAuthorSubscriptionMonitor(authorSubscriptionMonitorOptions(subscription));
    } catch (error) {
      setAuthorError(error instanceof Error ? error.message : "Author subscription failed");
    } finally {
      setIsSubscribingAuthor(false);
    }
  }

  async function updateAuthorMissingPolicy(subscription: AuthorSubscription, missingBookPolicy: AuthorMissingBookPolicy) {
    if (!subscription.id || subscription.missingBookPolicy === missingBookPolicy) return;
    setUpdatingAuthorID(subscription.id);
    setAuthorError("");
    try {
      const updated = await updateAuthorSubscription(subscription.id, { missingBookPolicy });
      setAuthorSubscriptions((current) => mergeAuthorSubscriptions(current, [updated]));
      setAPIState("live");
    } catch (error) {
      setAuthorError(error instanceof Error ? error.message : "Author subscription update failed");
    } finally {
      setUpdatingAuthorID("");
    }
  }

  async function removeAuthorSubscription(subscription: AuthorSubscription) {
    if (!subscription.id) return;
    setRemovingAuthorID(subscription.id);
    setAuthorError("");
    try {
      await deleteAuthorSubscription(subscription.id);
      setAuthorSubscriptions((current) => current.filter((item) => authorSubscriptionKey(item) !== authorSubscriptionKey(subscription)));
      setAuthorMetadataReviews((current) => current.filter((review) => review.authorSubscriptionId !== subscription.id));
      setAPIState("live");
    } catch (error) {
      setAuthorError(error instanceof Error ? error.message : "Author subscription remove failed");
    } finally {
      setRemovingAuthorID("");
    }
  }

  async function markSkippedAuthorCandidateWanted(subscription: AuthorSubscription, skipped: AuthorSkippedItem) {
    const key = authorSkippedItemKey(subscription, skipped);
    setMarkingAuthorSkippedKey(key);
    setAuthorError("");
    try {
      if (skipped.reviewId) {
        const outcome = await resolveAuthorMetadataReview(skipped.reviewId, "wanted");
        setAuthorMetadataReviews((current) => current.filter((item) => item.id !== outcome.review.id));
        if (outcome.wantedItem) {
          setWantedItems((current) => mergeWanted(current, [outcome.wantedItem!]));
          setSelectedWantedID(outcome.wantedItem.id);
        }
      } else {
        const wantedFormat = skipped.result.edition?.format === "audiobook" ? "audiobook" : subscription.format;
        const item = await createWanted(skipped.result, wantedFormat, subscription.qualityProfile, subscription.tags ?? []);
        setWantedItems((current) => mergeWanted(current, [item]));
        setSelectedWantedID(item.id);
      }
      await refreshWantedAndHistory();
      setAPIState("live");
    } catch (error) {
      setAuthorError(error instanceof Error ? error.message : "Mark skipped book wanted failed");
    } finally {
      setMarkingAuthorSkippedKey("");
    }
  }

  async function runWantedReleaseSearch(item = selectedWanted) {
    if (!item) return;
    setIsSearchingWanted(true);
    setWantedError("");
    try {
      const outcome = await searchWantedReleases(item.id);
      setWantedItems((current) => mergeWanted(current, [outcome.wantedItem]));
      setWantedReleases(outcome.releases);
      setWantedReleaseFilter("all");
      setSelectedWantedID(item.id);
      await refreshAcquisitionQueue({ quiet: true });
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted release search failed");
    } finally {
      setIsSearchingWanted(false);
    }
  }

  function openWantedItem(item: WantedItem) {
    setSelectedWantedID(item.id);
    setWantedViewFilter("all");
    setWantedReleaseFilter("all");
    setWantedReleases([]);
    setWantedMetadata(null);
    setActiveView("wanted");
  }

  function openImportReview(review: ImportReview) {
    if (review.id) {
      setSelectedImportReviewIDs([review.id]);
    }
    setActiveView("imports");
  }

  function openDownloadActivity(download: DownloadStatus) {
    setDownloadTextFilter(download.name || download.id);
    setDownloadStateFilter("all");
    setDownloadScope("all");
    setActiveView("downloads");
  }

  function openAuthorSubscriptionBooks(subscription: AuthorSubscription) {
    const stats = authorSubscriptionStatsByKey.get(authorSubscriptionKey(subscription));
    if (!stats?.firstWantedItem) return;
    openWantedItem(stats.firstWantedItem);
  }

  async function loadWantedReleaseDecisions(item = selectedWanted) {
    if (!item) return;
    setIsLoadingWantedReleases(true);
    setWantedError("");
    try {
      const outcome = await fetchWantedReleases(item.id);
      setWantedItems((current) => mergeWanted(current, [outcome.wantedItem]));
      setWantedReleases(outcome.releases);
      setSelectedWantedID(item.id);
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted release decisions refresh failed");
    } finally {
      setIsLoadingWantedReleases(false);
    }
  }

  async function saveWantedEdit() {
    const item = selectedWanted;
    if (!item || !wantedEditTitle.trim()) return;
    setIsSavingWantedEdit(true);
    setWantedError("");
    try {
      const updated = await updateWanted(item.id, {
        title: wantedEditTitle.trim(),
        authorName: wantedEditAuthor.trim(),
        coverUrl: wantedEditCoverURL.trim(),
        qualityProfile: wantedEditQualityProfile.trim() || "standard",
        monitored: wantedEditMonitored
      });
      setWantedItems((current) => mergeWanted(current, [updated]));
      setWantedMetadata((current) =>
        current ? { ...current, wantedItem: updated, manualOverrides: updated.manualOverrides ?? [] } : current
      );
      setSelectedWantedID(updated.id);
      setWantedReleases([]);
      await refreshWantedMetadataReview();
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted update failed");
    } finally {
      setIsSavingWantedEdit(false);
    }
  }

  async function removeSelectedWanted() {
    const item = selectedWanted;
    if (!item) return;
    setIsRemovingWanted(true);
    setWantedError("");
    try {
      await deleteWanted(item.id);
      setWantedItems((current) => current.filter((candidate) => candidate.id !== item.id));
      setWantedReleases([]);
      setWantedMetadata(null);
      setSelectedWantedID("");
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted remove failed");
    } finally {
      setIsRemovingWanted(false);
    }
  }

  async function clearSelectedWantedOverride(fieldName: string) {
    const item = selectedWanted;
    if (!item || !fieldName) return;
    setClearingWantedOverrideField(fieldName);
    setWantedError("");
    try {
      const updated = await clearWantedOverride(item.id, fieldName);
      setWantedItems((current) => mergeWanted(current, [updated]));
      setWantedMetadata((current) =>
        current ? { ...current, wantedItem: updated, manualOverrides: updated.manualOverrides ?? [] } : current
      );
      setSelectedWantedID(updated.id);
      setWantedReleases([]);
      await refreshWantedMetadataReview();
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted override reset failed");
    } finally {
      setClearingWantedOverrideField("");
    }
  }

  async function applySelectedMetadataCandidate(field: MetadataFieldEvidence, candidate: MetadataFieldCandidate) {
    const item = selectedWanted;
    if (!item || !metadataFieldCanApply(field)) return;
    const actionID = `${field.fieldName}:${candidate.provider}:${candidate.value}`;
    setApplyingMetadataCandidateID(actionID);
    setWantedError("");
    try {
      const provenance = await applyWantedMetadataCorrection(item.id, {
        fieldName: field.fieldName,
        value: candidate.value
      });
      setWantedMetadata(provenance);
      setWantedItems((current) => mergeWanted(current, [provenance.wantedItem]));
      setSelectedWantedID(provenance.wantedItem.id);
      await refreshWantedMetadataReview();
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted metadata correction failed");
    } finally {
      setApplyingMetadataCandidateID("");
    }
  }

  async function confirmSelectedMetadataCanonical(field: MetadataFieldEvidence) {
    const item = selectedWanted;
    if (!item || !metadataFieldCanConfirmCanonical(field)) return;
    const actionID = metadataFieldCanonicalActionID(field);
    setApplyingMetadataCandidateID(actionID);
    setWantedError("");
    try {
      const provenance = await applyWantedMetadataCorrection(item.id, {
        fieldName: field.fieldName,
        value: field.canonicalValue || "",
        reason: "metadata review canonical accepted"
      });
      setWantedMetadata(provenance);
      setWantedItems((current) => mergeWanted(current, [provenance.wantedItem]));
      setSelectedWantedID(provenance.wantedItem.id);
      await refreshWantedMetadataReview();
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted metadata confirmation failed");
    } finally {
      setApplyingMetadataCandidateID("");
    }
  }

  function toggleMetadataReviewSelection(item: WantedItem) {
    if (!wantedMetadataReviewByID.has(item.id)) return;
    setSelectedMetadataReviewIDs((current) => current.includes(item.id)
      ? current.filter((id) => id !== item.id)
      : [...current, item.id]
    );
  }

  function toggleAllVisibleMetadataReviews() {
    setSelectedMetadataReviewIDs((current) => {
      const visible = new Set(visibleMetadataReviewIDs);
      if (visible.size === 0) return current;
      if (visibleMetadataReviewIDs.every((id) => current.includes(id))) {
        return current.filter((id) => !visible.has(id));
      }
      const next = new Set(current);
      visibleMetadataReviewIDs.forEach((id) => next.add(id));
      return Array.from(next);
    });
  }

  async function confirmSelectedMetadataReviews() {
    const wantedIds = visibleMetadataReviewIDs.filter((id) => selectedMetadataReviewSet.has(id));
    if (wantedIds.length === 0) return;
    setIsConfirmingMetadataReviews(true);
    setWantedError("");
    try {
      const outcome = await confirmWantedMetadataReviewCanonical({ wantedIds });
      setMetadataReviewConfirmOutcome(outcome);
      const provenances = outcome.items ?? [];
      if (provenances.length) {
        setWantedItems((current) => mergeWanted(current, provenances.map((item) => item.wantedItem)));
        const selectedProvenance = provenances.find((item) => item.wantedItem.id === selectedWanted?.id);
        if (selectedProvenance) {
          setWantedMetadata(selectedProvenance);
          setSelectedWantedID(selectedProvenance.wantedItem.id);
        }
      }
      setSelectedMetadataReviewIDs((current) => current.filter((id) => !wantedIds.includes(id)));
      await refreshWantedMetadataReview();
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Metadata review confirmation failed");
    } finally {
      setIsConfirmingMetadataReviews(false);
    }
  }

  async function applySelectedMetadataRecord(record: ProviderMetadataRecord) {
    const item = selectedWanted;
    const corrections = metadataRecordCorrections(record, wantedMetadata);
    if (!item || corrections.length === 0) return;
    const actionID = metadataRecordActionID(record);
    setApplyingMetadataRecordID(actionID);
    setWantedError("");
    try {
      const provenance = await applyWantedMetadataCorrections(item.id, { corrections });
      setWantedMetadata(provenance);
      setWantedItems((current) => mergeWanted(current, [provenance.wantedItem]));
      setSelectedWantedID(provenance.wantedItem.id);
      await refreshWantedMetadataReview();
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted metadata corrections failed");
    } finally {
      setApplyingMetadataRecordID("");
    }
  }

  async function grabWantedRelease(release?: ReleaseDecision, force = false) {
    const item = selectedWanted;
    if (!item) return;
    const actionID = release ? releaseActionID(release, force) : "auto";
    setGrabbingWantedReleaseID(actionID);
    setWantedError("");
    try {
      const status = await grabWanted(item.id, release?.id, { paused: true, force });
      setDownloadStatus(status);
      await refreshDownloads();
      await refreshWantedAndHistory();
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted grab failed");
    } finally {
      setGrabbingWantedReleaseID("");
    }
  }

  async function runMonitor(options: { force?: boolean; autoGrab?: boolean }) {
    setIsRunningMonitor(true);
    setMonitorError("");
    try {
      const run = await runWantedMonitor({
        force: options.force ?? false,
        autoGrab: options.autoGrab ?? false,
        paused: true
      });
      setMonitorRun(run);
      setWantedItems((current) => mergeWanted(current, run.items?.map((item) => item.wantedItem) ?? []));
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
    } catch (error) {
      setMonitorError(error instanceof Error ? error.message : "Wanted monitor failed");
    } finally {
      setIsRunningMonitor(false);
    }
  }

  async function runAuthorSubscriptionMonitor(options: { authorIds?: string[]; providerKeys?: string[]; force?: boolean; targetKey?: string } = {}) {
    setIsRunningAuthorMonitor(true);
    setAuthorMonitorTargetKey(options.targetKey ?? (options.authorIds?.[0] || options.providerKeys?.[0] || ""));
    setAuthorError("");
    try {
      const run = await runAuthorMonitor({
        authorIds: options.authorIds ?? [],
        providerKeys: options.providerKeys ?? [],
        force: options.force ?? false
      });
      setAuthorMonitorRun(run);
      const created = run.items?.flatMap((item) => item.wantedItems ?? []) ?? [];
      if (created.length) {
        setWantedItems((current) => mergeWanted(current, created));
      }
      const [nextSubscriptions, nextReviews] = await Promise.all([fetchAuthorSubscriptions(), fetchAuthorMetadataReviews().catch(() => []), refreshWantedAndHistory()]);
      setAuthorSubscriptions(nextSubscriptions);
      setAuthorMetadataReviews(nextReviews);
      setAPIState("live");
    } catch (error) {
      setAuthorError(error instanceof Error ? error.message : "Author monitor failed");
    } finally {
      setIsRunningAuthorMonitor(false);
      setAuthorMonitorTargetKey("");
    }
  }

  async function runFeedSync(options: { autoGrab?: boolean }) {
    setIsRunningFeedSync(true);
    setFeedError("");
    try {
      const run = await runWantedFeedSync({
        format,
        autoGrab: options.autoGrab ?? false,
        paused: true
      });
      setFeedSyncRun(run);
      setWantedItems((current) => mergeWanted(current, run.matches?.map((match) => match.wantedItem) ?? []));
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setFeedError(error instanceof Error ? error.message : "Feed sync failed");
    } finally {
      setIsRunningFeedSync(false);
    }
  }

  async function runUpgrades(options: { autoGrab?: boolean }) {
    setIsRunningUpgrade(true);
    setUpgradeError("");
    try {
      const run = await runUpgradeSearch({
        autoGrab: options.autoGrab ?? false,
        paused: true,
        force: true
      });
      setUpgradeRun(run);
      setWantedItems((current) => mergeWanted(current, run.items?.map((item) => item.wantedItem) ?? []));
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setUpgradeError(error instanceof Error ? error.message : "Upgrade search failed");
    } finally {
      setIsRunningUpgrade(false);
    }
  }

  async function refreshWantedAndHistory() {
    setHistoryError("");
    setWantedError("");
    try {
      const [nextWanted, nextHistory, nextReview, nextQueue] = await Promise.all([
        fetchWanted(),
        fetchHistory(),
        fetchWantedMetadataReview().catch(() => null),
        fetchAcquisitionQueue({ status: "all", limit: 100 }).catch(() => null)
      ]);
      setWantedItems(nextWanted);
      setHistoryEvents(nextHistory);
      setWantedMetadataReview(nextReview);
      if (nextQueue) {
        setAcquisitionQueue(nextQueue);
        setWantedItems((current) => mergeWanted(current, nextQueue.items.map((item) => item.wantedItem)));
      }
      setSelectedWantedID((current) => current || nextWanted[0]?.id || "");
    } catch (error) {
      const message = error instanceof Error ? error.message : "Refresh failed";
      setHistoryError(message);
      setWantedError(message);
    }
  }

  async function refreshAcquisitionQueue(options: { quiet?: boolean } = {}) {
    if (!options.quiet) {
      setIsRefreshingAcquisitionQueue(true);
    }
    setAcquisitionError("");
    try {
      const queue = await fetchAcquisitionQueue({ status: "all", limit: 100 });
      setAcquisitionQueue(queue);
      setWantedItems((current) => mergeWanted(current, queue.items.map((item) => item.wantedItem)));
      setSelectedWantedID((current) => current || queue.items[0]?.wantedItem.id || "");
      setAPIState("live");
    } catch (error) {
      setAcquisitionError(error instanceof Error ? error.message : "Acquisition queue refresh failed");
    } finally {
      if (!options.quiet) {
        setIsRefreshingAcquisitionQueue(false);
      }
    }
  }

  async function runAcquisitionQueueAction(item: AcquisitionQueueItem) {
    const actionID = acquisitionQueueActionID(item);
    setAcquisitionActionID(actionID);
    setAcquisitionError("");
    setSelectedWantedID(item.wantedItem.id);
    try {
      switch (item.state) {
        case "needs_search":
          await runWantedReleaseSearch(item.wantedItem);
          break;
        case "ready_to_grab": {
          const status = await grabWanted(item.wantedItem.id, item.bestRelease?.id, { paused: true });
          setDownloadStatus(status);
          await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
          break;
        }
        case "import_ready": {
          const downloadsToImport = queueDownloadsForImport(item);
          if (!downloadsToImport.length) {
            throw new Error("No completed download is ready to import for this wanted item");
          }
          await runCompletedImport(downloadsToImport);
          break;
        }
        case "blocked": {
          const failedDownloads = queueDownloadsForRecovery(item);
          if (failedDownloads.length) {
            await runFailedRecovery(failedDownloads, { autoGrab: true, force: true });
          } else {
            await loadWantedReleaseDecisions(item.wantedItem);
          }
          break;
        }
        case "queued":
        case "downloading":
          setIsRefreshingDownloads(true);
          try {
            setDownloads(await fetchDownloads({ tag: "librarry" }));
          } finally {
            setIsRefreshingDownloads(false);
          }
          setDownloadScope("librarry");
          setDownloadTextFilter(item.wantedItem.title);
          setActiveView("downloads");
          break;
        default:
          openWantedItem(item.wantedItem);
      }
    } catch (error) {
      setAcquisitionError(error instanceof Error ? error.message : "Queue action failed");
    } finally {
      setAcquisitionActionID("");
    }
  }

  async function refreshWantedMetadataReview() {
    try {
      setWantedMetadataReview(await fetchWantedMetadataReview());
    } catch {
      setWantedMetadataReview(null);
    }
  }

  async function refreshAuthorMetadataReviews() {
    try {
      setAuthorMetadataReviews(await fetchAuthorMetadataReviews());
    } catch {
      setAuthorMetadataReviews([]);
    }
  }

  async function resolveAuthorReview(review: AuthorMetadataReview, action: "wanted" | "ignore") {
    if (!review.id) return;
    setAuthorReviewActionID(`${review.id}:${action}`);
    setAuthorError("");
    try {
      const outcome = await resolveAuthorMetadataReview(review.id, action);
      setAuthorMetadataReviews((current) => current.filter((item) => item.id !== outcome.review.id));
      if (outcome.wantedItem) {
        setWantedItems((current) => mergeWanted(current, [outcome.wantedItem!]));
        setSelectedWantedID(outcome.wantedItem.id);
      }
      await refreshWantedAndHistory();
      setAPIState("live");
    } catch (error) {
      setAuthorError(error instanceof Error ? error.message : "Author metadata review update failed");
    } finally {
      setAuthorReviewActionID("");
    }
  }

  async function runLibraryScan(nextFormat = format, options: { root?: string } = {}) {
    setIsScanningLibrary(true);
    setLibraryError("");
    try {
      const outcome = await scanLibrary(nextFormat, { root: options.root });
      setLibraryScan(outcome);
      setLibraryFiles((current) => mergeLibraryFiles(current, outcome.files));
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Library scan failed");
    } finally {
      setIsScanningLibrary(false);
    }
  }

  async function runLibraryImport() {
    if (!importPath.trim()) return;
    setIsImportingLibrary(true);
    setLibraryError("");
    try {
      const outcome = await importLibraryFile({
        sourcePath: importPath.trim(),
        wantedId: selectedWanted?.id,
        format: selectedWanted?.format ?? (format === "any" ? "ebook" : format),
        move: libraryImportMode === "move",
        importMode: libraryImportMode,
        conflictAction: libraryConflictAction,
        overwrite: libraryConflictAction === "replace"
      });
      setLibraryImport(outcome);
      if (outcome.imported) {
        setLibraryFiles((current) => mergeLibraryFiles(current, [outcome.file]));
      }
      const nextWanted = await fetchWanted();
      setWantedItems(nextWanted);
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Library import failed");
    } finally {
      setIsImportingLibrary(false);
    }
  }

  async function runCompletedImport(download?: DownloadStatus | DownloadStatus[]) {
    const targets = Array.isArray(download) ? download : download ? [download] : [];
    const downloadIDs = targets.map((item) => item.id).filter(Boolean);
    const actionID = downloadIDs.length > 1 ? "bulk:import" : downloadIDs[0] ? `${downloadIDs[0]}:import` : "";
    setIsImportingCompleted(true);
    if (actionID) {
      setDownloadActionID(actionID);
    }
    setLibraryError("");
    try {
      const outcome = await importCompletedDownloads({
        downloadIds: downloadIDs,
        move: libraryImportMode === "move",
        importMode: libraryImportMode,
        conflictAction: libraryConflictAction,
        overwrite: libraryConflictAction === "replace",
        limit: 50
      });
      setCompletedImport(outcome);
      const importedFiles = outcome.results.flatMap((result) => (result.import?.imported ? [result.import.file] : []));
      if (importedFiles.length) {
        setLibraryFiles((current) => mergeLibraryFiles(current, importedFiles));
      }
      const [reviews] = await Promise.all([fetchLibraryImportReviews(), refreshDownloads(), refreshWantedAndHistory()]);
      setImportReviews(reviews);
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Completed import failed");
    } finally {
      setIsImportingCompleted(false);
      if (actionID) {
        setDownloadActionID("");
      }
    }
  }

  async function runResolveImportReview(review: ImportReview, action: "import" | "skip" | "reject", wantedID = importReviewResolvedWantedID(review, selectedImportReviewWantedIDs)) {
    const actionID = `${review.id}:${action}`;
    setReviewActionID(actionID);
    setLibraryError("");
    setBulkReviewOutcome(null);
    try {
      const nextFormat = review.mediaFormat === "unknown" ? importReviewResolvedFormat(review, wantedID, format) : review.mediaFormat;
      const outcome = await resolveLibraryImportReview(review.id, {
        action,
        wantedId: action === "import" ? wantedID : review.wantedId,
        format: nextFormat,
        move: libraryImportMode === "move",
        importMode: libraryImportMode,
        conflictAction: libraryConflictAction,
        overwrite: libraryConflictAction === "replace"
      });
      if (outcome.import?.imported) {
        setLibraryImport(outcome.import);
        setLibraryFiles((current) => mergeLibraryFiles(current, [outcome.import!.file]));
      }
      setImportReviews((current) => current.filter((item) => item.id !== review.id));
      setSelectedImportReviewIDs((current) => current.filter((id) => id !== review.id));
      setSelectedImportReviewWantedIDs((current) => {
        const next = { ...current };
        delete next[review.id];
        return next;
      });
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Import review update failed");
    } finally {
      setReviewActionID("");
    }
  }

  async function runResolveImportReviewsBulk(action: "import" | "skip" | "reject") {
    const reviewIDs = selectedImportReviews.map((review) => review.id).filter(Boolean);
    if (!reviewIDs.length) return;
    const singleReview = selectedImportReviews.length === 1 ? selectedImportReviews[0] : undefined;
    const singleReviewWantedID = singleReview ? importReviewResolvedWantedID(singleReview, selectedImportReviewWantedIDs) : "";
    const singleReviewFormat = singleReview
      ? singleReview.mediaFormat === "unknown"
        ? importReviewResolvedFormat(singleReview, singleReviewWantedID, format)
        : singleReview.mediaFormat
      : undefined;
    setReviewActionID(`bulk:${action}`);
    setLibraryError("");
    setBulkReviewOutcome(null);
    try {
      const outcome = await resolveLibraryImportReviewsBulk({
        ids: reviewIDs,
        action,
        wantedId: action === "import" && singleReview ? singleReviewWantedID : undefined,
        format: action === "import" ? singleReviewFormat : undefined,
        move: libraryImportMode === "move",
        importMode: libraryImportMode,
        conflictAction: libraryConflictAction,
        overwrite: libraryConflictAction === "replace"
      });
      setBulkReviewOutcome(outcome);
      const importedFiles = outcome.results.flatMap((result) => (result.outcome?.import?.imported ? [result.outcome.import.file] : []));
      if (importedFiles.length) {
        setLibraryImport(outcome.results.find((result) => result.outcome?.import?.imported)?.outcome?.import ?? null);
        setLibraryFiles((current) => mergeLibraryFiles(current, importedFiles));
      }
      const resolvedIDs = new Set(outcome.results.filter((result) => result.status !== "error").map((result) => result.id));
      setSelectedImportReviewIDs((current) => current.filter((id) => !resolvedIDs.has(id)));
      const [reviews] = await Promise.all([fetchLibraryImportReviews(), refreshDownloads(), refreshWantedAndHistory()]);
      setImportReviews(reviews);
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Bulk import review update failed");
    } finally {
      setReviewActionID("");
    }
  }

  function toggleImportReviewSelection(review: ImportReview) {
    if (!review.id) return;
    setSelectedImportReviewIDs((current) => (current.includes(review.id) ? current.filter((id) => id !== review.id) : [...current, review.id]));
  }

  function toggleAllImportReviews() {
    setSelectedImportReviewIDs((current) => {
      const next = new Set(current);
      const everyVisibleSelected = selectableImportReviewIDs.length > 0 && selectableImportReviewIDs.every((id) => next.has(id));
      for (const id of selectableImportReviewIDs) {
        if (everyVisibleSelected) {
          next.delete(id);
        } else {
          next.add(id);
        }
      }
      return Array.from(next);
    });
  }

  async function runFailedRecovery(download?: DownloadStatus | DownloadStatus[], options: { autoGrab?: boolean; removeFailed?: boolean; force?: boolean } = {}) {
    const targets = Array.isArray(download) ? download : download ? [download] : [];
    const downloadIDs = targets.map((item) => item.id).filter(Boolean);
    const actionID = downloadIDs.length > 1 ? "bulk:recover" : downloadIDs[0] ? `${downloadIDs[0]}:recover` : "";
    setIsRecoveringFailed(true);
    if (actionID) {
      setDownloadActionID(actionID);
    }
    setDownloadError("");
    try {
      const run = await recoverFailedDownloads({
        downloadIds: downloadIDs,
        autoGrab: options.autoGrab ?? false,
        paused: true,
        removeFailed: options.removeFailed ?? false,
        force: options.force ?? downloadIDs.length > 0
      });
      setFailedDownloadRun(run);
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Failed-download recovery failed");
    } finally {
      setIsRecoveringFailed(false);
      if (actionID) {
        setDownloadActionID("");
      }
    }
  }

  async function refreshDownloads() {
    setIsRefreshingDownloads(true);
    setDownloadError("");
    try {
      const nextDownloads = await fetchDownloads(downloadListOptions());
      setDownloads(nextDownloads);
      if (resourceClientConfigured) {
        await refreshDownloadResources(true);
      }
      await refreshAcquisitionQueue({ quiet: true });
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? `${error.message}. Keeping current queue data.` : "Download refresh failed. Keeping current queue data.");
    } finally {
      setIsRefreshingDownloads(false);
    }
  }

  async function refreshDownloadResources(silent = false) {
    if (!resourceClientConfigured) {
      setDownloadResources(seedDownloadResourcesByClient[downloadResourceClient] ?? { client: downloadResourceClient, categories: [], tags: [] });
      setIsLoadingDownloadResources(false);
      return;
    }
    setIsLoadingDownloadResources(true);
    if (!silent) {
      setDownloadError("");
    }
    try {
      const resources = await fetchDownloadResources(downloadResourceClient);
      setDownloadResources(resources);
    } catch (error) {
      const fallback = seedDownloadResourcesByClient[downloadResourceClient];
      if (fallback) setDownloadResources(fallback);
      if (!silent) {
        setDownloadError(error instanceof Error ? error.message : "Download resources refresh failed");
      }
    } finally {
      setIsLoadingDownloadResources(false);
    }
  }

  function hydrateDownloadPreferenceForm(preferences: DownloadPreferences) {
    setPreferenceSavePath(preferences.savePath || "");
    setPreferenceTempPath(preferences.tempPath || "");
    setPreferenceTempPathEnabled(Boolean(preferences.tempPathEnabled));
    setPreferenceStartPaused(Boolean(preferences.startPaused));
    setPreferenceQueueingEnabled(Boolean(preferences.queueingEnabled));
    setPreferenceSpeedScheduleEnabled(Boolean(preferences.speedScheduleEnabled));
    setPreferenceDownloadLimitKiB(limitBytesToKiBInput(preferences.downloadLimit));
    setPreferenceUploadLimitKiB(limitBytesToKiBInput(preferences.uploadLimit));
    setPreferenceAltDownloadLimitKiB(limitBytesToKiBInput(preferences.alternativeDownloadLimit));
    setPreferenceAltUploadLimitKiB(limitBytesToKiBInput(preferences.alternativeUploadLimit));
    setPreferenceMaxActiveDownloads(String(preferences.maxActiveDownloads ?? -1));
    setPreferenceMaxActiveUploads(String(preferences.maxActiveUploads ?? -1));
    setPreferenceMaxActiveTorrents(String(preferences.maxActiveTorrents ?? -1));
  }

  async function refreshDownloadPreferences(silent = false) {
    if (!resourceClientSupportsPreferences || !resourceClientConfigured) {
      const fallback = seedDownloadPreferencesByClient[downloadResourceClient];
      setDownloadPreferences(fallback ?? null);
      if (fallback) hydrateDownloadPreferenceForm(fallback);
      setIsLoadingDownloadPreferences(false);
      return;
    }
    setIsLoadingDownloadPreferences(true);
    if (!silent) {
      setDownloadError("");
    }
    try {
      const preferences = await fetchDownloadPreferences(downloadResourceClient);
      setDownloadPreferences(preferences);
      hydrateDownloadPreferenceForm(preferences);
    } catch (error) {
      const fallback = seedDownloadPreferencesByClient[downloadResourceClient];
      setDownloadPreferences(fallback ?? null);
      if (fallback) hydrateDownloadPreferenceForm(fallback);
      if (!silent) {
        setDownloadError(error instanceof Error ? error.message : "Download preferences refresh failed");
      }
    } finally {
      setIsLoadingDownloadPreferences(false);
    }
  }

  async function applyDownloadPreferences() {
    if (!resourceClientSupportsPreferences) return;
    const downloadLimit = bandwidthInputToBytes(preferenceDownloadLimitKiB);
    const uploadLimit = bandwidthInputToBytes(preferenceUploadLimitKiB);
    const altDownloadLimit = bandwidthInputToBytes(preferenceAltDownloadLimitKiB);
    const altUploadLimit = bandwidthInputToBytes(preferenceAltUploadLimitKiB);
    const maxActiveDownloads = queueLimitInputToInt(preferenceMaxActiveDownloads);
    const maxActiveUploads = queueLimitInputToInt(preferenceMaxActiveUploads);
    const maxActiveTorrents = queueLimitInputToInt(preferenceMaxActiveTorrents);
    if ([downloadLimit, uploadLimit, altDownloadLimit, altUploadLimit].some((value) => value < 0) || [maxActiveDownloads, maxActiveUploads, maxActiveTorrents].some((value) => value < -1)) {
      setDownloadError("Download preferences contain invalid limits");
      return;
    }
    setIsSavingDownloadPreferences(true);
    setDownloadError("");
    try {
      const preferences = await saveDownloadPreferences({
        client: downloadResourceClient,
        savePath: preferenceSavePath.trim(),
        tempPathEnabled: preferenceTempPathEnabled,
        tempPath: preferenceTempPath.trim(),
        startPaused: preferenceStartPaused,
        queueingEnabled: preferenceQueueingEnabled,
        speedScheduleEnabled: preferenceSpeedScheduleEnabled,
        downloadLimit,
        uploadLimit,
        alternativeDownloadLimit: altDownloadLimit,
        alternativeUploadLimit: altUploadLimit,
        maxActiveDownloads,
        maxActiveUploads,
        maxActiveTorrents: resourceClientIsQbittorrent ? maxActiveTorrents : undefined
      });
      setDownloadPreferences(preferences);
      hydrateDownloadPreferenceForm(preferences);
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download preferences update failed");
    } finally {
      setIsSavingDownloadPreferences(false);
    }
  }

  async function changeDownloadScope(scope: DownloadScope) {
    if (scope === downloadScope) return;
    setDownloadScope(scope);
    setSelectedDownloadKeys([]);
    setDownloadDetails(null);
    setIsRefreshingDownloads(true);
    setDownloadError("");
    try {
      const nextDownloads = await fetchDownloads(downloadListOptions(scope));
      setDownloads(nextDownloads);
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download refresh failed");
    } finally {
      setIsRefreshingDownloads(false);
    }
  }

  async function runQueueRebalance() {
    const maxActive = boundedPositiveInt(downloadRebalanceMax, 3, 25);
    setDownloadRebalanceMax(String(maxActive));
    setIsRebalancingDownloads(true);
    setDownloadError("");
    try {
      const plan = await rebalanceDownloads({
        maxActive,
        client: downloadClientFilter.trim() || undefined,
        tag: downloadScopeTag(downloadScope),
        category: downloadCategoryFilter.trim() || undefined,
        dryRun: false,
        stopOverflow: true
      });
      setQueueRebalancePlan(plan);
      if (plan.downloads?.length) {
        setDownloads(plan.downloads);
      } else {
        await refreshDownloads();
      }
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Queue rebalance failed");
    } finally {
      setIsRebalancingDownloads(false);
    }
  }

  async function saveDownloadCategoryResource() {
    const name = resourceCategoryName.trim();
    if (!name) return;
    const existing = (downloadResources?.categories ?? []).some((category) => category.name.toLowerCase() === name.toLowerCase());
    const action = resourceClientIsTransmission ? "edit" : existing ? "edit" : "create";
    const newName = resourceCategoryNewName.trim();
    if (resourceClientIsTransmission && !newName) return;
    setDownloadResourceActionID(`category:${action}:${name}`);
    setDownloadError("");
    try {
      const result = await runDownloadCategoryAction({
        action,
        name,
        newName: newName || undefined,
        client: downloadResourceClient,
        savePath: resourceCategoryPath.trim()
      });
      if (result.resources) {
        setDownloadResources(result.resources);
      } else {
        await refreshDownloadResources(true);
      }
      setDownloadCategoryFilter(newName || name);
      setResourceCategoryNewName("");
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download category resource action failed");
    } finally {
      setDownloadResourceActionID("");
    }
  }

  async function deleteDownloadCategoryResource(name = resourceCategoryName) {
    const category = name.trim();
    if (!category) return;
    setDownloadResourceActionID(`category:delete:${category}`);
    setDownloadError("");
    try {
      const result = await runDownloadCategoryAction({
        action: "delete",
        name: category,
        client: downloadResourceClient
      });
      if (result.resources) {
        setDownloadResources(result.resources);
      } else {
        await refreshDownloadResources(true);
      }
      if (resourceCategoryName.trim().toLowerCase() === category.toLowerCase()) {
        setResourceCategoryName("");
        setResourceCategoryPath("");
      }
      if (downloadCategoryFilter.trim().toLowerCase() === category.toLowerCase()) {
        setDownloadCategoryFilter("");
      }
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download category delete failed");
    } finally {
      setDownloadResourceActionID("");
    }
  }

  async function createDownloadTagResource() {
    const names = splitTagInput(resourceTagName);
    if (names.length === 0) return;
    setDownloadResourceActionID(`tag:create:${names.join(",")}`);
    setDownloadError("");
    try {
      const newName = resourceTagNewName.trim();
      const action = resourceClientIsTransmission ? "edit" : "create";
      if (resourceClientIsTransmission && (!newName || names.length !== 1)) return;
      const result = await runDownloadTagAction({
        action,
        names,
        newName: newName || undefined,
        client: downloadResourceClient
      });
      if (result.resources) {
        setDownloadResources(result.resources);
      } else {
        await refreshDownloadResources(true);
      }
      setResourceTagName("");
      setResourceTagNewName("");
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download tag create failed");
    } finally {
      setDownloadResourceActionID("");
    }
  }

  async function deleteDownloadTagResource(name = resourceTagName) {
    const names = splitTagInput(name);
    if (names.length === 0) return;
    setDownloadResourceActionID(`tag:delete:${names.join(",")}`);
    setDownloadError("");
    try {
      const result = await runDownloadTagAction({ action: "delete", names, client: downloadResourceClient });
      if (result.resources) {
        setDownloadResources(result.resources);
      } else {
        await refreshDownloadResources(true);
      }
      if (splitTagInput(resourceTagName).some((tag) => names.includes(tag))) {
        setResourceTagName("");
      }
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download tag delete failed");
    } finally {
      setDownloadResourceActionID("");
    }
  }

  async function openDownloadDetails(download: DownloadStatus) {
    if (!download.id) return;
    setIsLoadingDownloadDetails(true);
    setDownloadError("");
    try {
      const details = await fetchDownloadDetails(download.id, download.client);
      setDownloadDetails(details);
      setDownloadLimitKiB(limitBytesToKiBInput(details.properties?.downloadLimit));
      setUploadLimitKiB(limitBytesToKiBInput(details.properties?.uploadLimit));
      setDownloadNameInput(details.status.name || "");
      setDownloadTagsInput((details.status.tags ?? []).join(", "));
      setDownloadCategoryInput(details.status.category || "");
      setDownloadSavePathInput(details.status.savePath || details.properties?.savePath || "");
      setAPIState("live");
    } catch (error) {
      const fallback = seedDownloadDetailsByKey[downloadKey(download)];
      if (fallback) {
        setDownloadDetails(fallback);
        setDownloadLimitKiB(limitBytesToKiBInput(fallback.properties?.downloadLimit));
        setUploadLimitKiB(limitBytesToKiBInput(fallback.properties?.uploadLimit));
        setDownloadNameInput(fallback.status.name || "");
        setDownloadTagsInput((fallback.status.tags ?? []).join(", "));
        setDownloadCategoryInput(fallback.status.category || "");
        setDownloadSavePathInput(fallback.status.savePath || fallback.properties?.savePath || "");
        setDownloadError(error instanceof Error ? `${error.message}. Showing demo details.` : "Download details failed. Showing demo details.");
      } else {
        setDownloadError(error instanceof Error ? error.message : "Download details failed");
      }
    } finally {
      setIsLoadingDownloadDetails(false);
    }
  }

  async function applyDownloadFileAction(action: DownloadFileAction, ids: number[]) {
    const downloadID = downloadDetails?.status.id;
    if (!downloadID || ids.length === 0) return;
    setDownloadActionID(`${downloadID}:file:${action}`);
    setDownloadError("");
    try {
      const result = await runDownloadFileAction(downloadID, action, ids, { client: downloadDetails?.status.client });
      if (result.download) {
        setDownloadDetails(result.download);
      } else {
        const refreshed = await fetchDownloadDetails(downloadID, downloadDetails?.status.client);
        setDownloadDetails(refreshed);
      }
      await refreshDownloads();
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download file action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  async function applyDownloadTrackerAction(action: DownloadTrackerAction, currentURL = "") {
    const downloadID = downloadDetails?.status.id;
    if (!downloadID) return;
    const nextURL = trackerURL.trim();
    if (action === "add" && !nextURL) return;
    if (action === "edit" && (!currentURL || !nextURL)) return;
    if (action === "remove" && !currentURL) return;
    setDownloadActionID(`${downloadID}:tracker:${action}:${currentURL || nextURL}`);
    setDownloadError("");
    try {
      const result = await runDownloadTrackerAction(downloadID, action, {
        client: downloadDetails?.status.client,
        urls: action === "add" ? [nextURL] : undefined,
        url: action === "remove" ? currentURL : undefined,
        originalUrl: action === "edit" ? currentURL : undefined,
        newUrl: action === "edit" ? nextURL : undefined
      });
      if (result.download) {
        setDownloadDetails(result.download);
      } else {
        const refreshed = await fetchDownloadDetails(downloadID, downloadDetails?.status.client);
        setDownloadDetails(refreshed);
      }
      if (action !== "remove") {
        setTrackerURL("");
      }
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download tracker action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  async function applyDownloadBandwidthLimit(kind: "download" | "upload") {
    const downloadID = downloadDetails?.status.id;
    if (!downloadID) return;
    const text = kind === "download" ? downloadLimitKiB : uploadLimitKiB;
    const limit = bandwidthInputToBytes(text);
    if (limit < 0) return;
    const action: DownloadAction = kind === "download" ? "setDownloadLimit" : "setUploadLimit";
    setDownloadActionID(`${downloadID}:${action}`);
    setDownloadError("");
    try {
      await runDownloadAction(action, [downloadID], {
        client: downloadDetails?.status.client,
        downloadLimit: kind === "download" ? limit : undefined,
        uploadLimit: kind === "upload" ? limit : undefined
      });
      const refreshed = await fetchDownloadDetails(downloadID, downloadDetails?.status.client);
      setDownloadDetails(refreshed);
      setDownloadLimitKiB(limitBytesToKiBInput(refreshed.properties?.downloadLimit));
      setUploadLimitKiB(limitBytesToKiBInput(refreshed.properties?.uploadLimit));
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download bandwidth action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  async function applyDownloadCategory() {
    const downloadID = downloadDetails?.status.id;
    if (!downloadID) return;
    const category = downloadCategoryInput.trim();
    if (!category) return;
    setDownloadActionID(`${downloadID}:setCategory`);
    setDownloadError("");
    try {
      await runDownloadAction("setCategory", [downloadID], { client: downloadDetails?.status.client, category });
      const refreshed = await fetchDownloadDetails(downloadID, downloadDetails?.status.client);
      setDownloadDetails(refreshed);
      setDownloadCategoryInput(refreshed.status.category || category);
      await refreshDownloads();
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download category action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  async function applyDownloadLocation() {
    const downloadID = downloadDetails?.status.id;
    if (!downloadID) return;
    const savePath = downloadSavePathInput.trim();
    if (!savePath) return;
    setDownloadActionID(`${downloadID}:setLocation`);
    setDownloadError("");
    try {
      await runDownloadAction("setLocation", [downloadID], { client: downloadDetails?.status.client, savePath });
      const refreshed = await fetchDownloadDetails(downloadID, downloadDetails?.status.client);
      setDownloadDetails(refreshed);
      setDownloadSavePathInput(refreshed.status.savePath || refreshed.properties?.savePath || savePath);
      await refreshDownloads();
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download location action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  async function applyDownloadRename() {
    const downloadID = downloadDetails?.status.id;
    const name = downloadNameInput.trim();
    if (!downloadID || !name) return;
    setDownloadActionID(`${downloadID}:rename`);
    setDownloadError("");
    try {
      await runDownloadAction("rename", [downloadID], { client: downloadDetails?.status.client, name });
      const refreshed = await fetchDownloadDetails(downloadID, downloadDetails?.status.client);
      setDownloadDetails(refreshed);
      setDownloadNameInput(refreshed.status.name || name);
      await refreshDownloads();
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download rename failed");
    } finally {
      setDownloadActionID("");
    }
  }

  async function applyDownloadTags(action: "addTags" | "removeTags") {
    const downloadID = downloadDetails?.status.id;
    const tags = splitTagInput(downloadTagsInput);
    if (!downloadID || tags.length === 0) return;
    setDownloadActionID(`${downloadID}:${action}`);
    setDownloadError("");
    try {
      await runDownloadAction(action, [downloadID], { client: downloadDetails?.status.client, tags });
      const refreshed = await fetchDownloadDetails(downloadID, downloadDetails?.status.client);
      setDownloadDetails(refreshed);
      setDownloadTagsInput((refreshed.status.tags ?? []).join(", "));
      await refreshDownloads();
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download tag action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  async function applyDownloadAction(action: DownloadAction, download: DownloadStatus, deleteFiles = false) {
    await applyDownloadActionToIDs(action, [download.id], deleteFiles, { client: download.client });
  }

  async function applyDownloadActionToIDs(action: DownloadAction, ids: string[], deleteFiles = false, options: { client?: string; forceStart?: boolean } = {}) {
    const actionIDs = ids.filter(Boolean);
    if (!actionIDs.length) return;
    const isBulk = actionIDs.length > 1;
    setDownloadActionID(isBulk ? `bulk:${action}` : `${actionIDs[0]}:${action}`);
    setDownloadError("");
    try {
      const result = await runDownloadAction(action, actionIDs, { client: options.client, deleteFiles, forceStart: options.forceStart });
      if (result.downloads?.length) {
        setDownloads((current) => mergeDownloads(current, result.downloads ?? []));
      } else if (action === "delete") {
        const deleted = new Set(actionIDs);
        setDownloads((current) => current.filter((item) => !deleted.has(item.id)));
        setSelectedDownloadKeys((current) => current.filter((key) => !deleted.has(key.split(":").slice(1).join(":"))));
        setDownloadDetails((current) => (current && deleted.has(current.status.id) ? null : current));
      } else {
        await refreshDownloads();
      }
      if (action !== "delete" && downloadDetails?.status.id && actionIDs.includes(downloadDetails.status.id) && downloadSupportsDetails(downloadDetails.status)) {
        const refreshed = await fetchDownloadDetails(downloadDetails.status.id, downloadDetails.status.client);
        setDownloadDetails(refreshed);
        setDownloadNameInput(refreshed.status.name || "");
        setDownloadTagsInput((refreshed.status.tags ?? []).join(", "));
        setDownloadCategoryInput(refreshed.status.category || "");
        setDownloadSavePathInput(refreshed.status.savePath || refreshed.properties?.savePath || "");
      }
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  function toggleDownloadSelection(download: DownloadStatus) {
    const key = downloadKey(download);
    if (!key) return;
    setSelectedDownloadKeys((current) => (current.includes(key) ? current.filter((item) => item !== key) : [...current, key]));
  }

  function toggleAllDownloads() {
    setSelectedDownloadKeys((current) => {
      const next = new Set(current);
      const everyVisibleSelected = selectableDownloadKeys.length > 0 && selectableDownloadKeys.every((key) => next.has(key));
      for (const key of selectableDownloadKeys) {
        if (everyVisibleSelected) {
          next.delete(key);
        } else {
          next.add(key);
        }
      }
      return Array.from(next);
    });
  }

  function updateQualityProfile(profile: QualityProfile, changes: Partial<QualityProfile>) {
    const key = profileKey(profile);
    setQualityProfiles((current) => current.map((item) => (profileKey(item) === key ? { ...item, ...changes } : item)));
  }

  async function persistQualityProfile(profile: QualityProfile) {
    const key = profileKey(profile);
    setSavingProfileID(key);
    setSettingsError("");
    try {
      const saved = await saveQualityProfile(profile);
      setQualityProfiles((current) => current.map((item) => (profileKey(item) === key ? saved : item)));
      setReadiness(await fetchReadiness());
      setAPIState("live");
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : "Quality profile save failed");
    } finally {
      setSavingProfileID("");
    }
  }

  function updateIntegrationForm(changes: Partial<IntegrationSettings>) {
    setIntegrationForm((current) => ({ ...current, ...changes }));
  }

  function updateLibrarySettingsForm(changes: Partial<LibrarySettings>) {
    setLibrarySettingsForm((current) => ({ ...current, ...changes }));
  }

  function updateReadarrImportForm(changes: Partial<ReadarrImportSettings>) {
    setReadarrImportForm((current) => ({ ...current, ...changes }));
  }

  async function previewExistingReadarrImport() {
    setIsPreviewingReadarrImport(true);
    setSettingsError("");
    setSettingsNotice("");
    try {
      const outcome = await previewReadarrImport(readarrImportForm);
      setReadarrImportOutcome(outcome);
      setSettingsNotice(`Readarr preview found ${readarrImportCount(outcome)} importable records.`);
      setAPIState("live");
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : "Readarr import preview failed");
    } finally {
      setIsPreviewingReadarrImport(false);
    }
  }

  async function applyExistingReadarrImport() {
    setIsRunningReadarrImport(true);
    setSettingsError("");
    setSettingsNotice("");
    try {
      const outcome = await runReadarrImport(readarrImportForm);
      setReadarrImportOutcome(outcome);
      await Promise.allSettled([
        fetchQualityProfiles().then(setQualityProfiles),
        fetchWanted().then(setWantedItems),
        fetchAuthorSubscriptions().then(setAuthorSubscriptions),
        fetchReadiness().then(setReadiness)
      ]);
      setSettingsNotice(`Readarr import ${outcome.status === "partial" ? "partially completed" : "completed"}: ${readarrImportImported(outcome)} records written.`);
      setAPIState("live");
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : "Readarr import failed");
    } finally {
      setIsRunningReadarrImport(false);
    }
  }

  async function persistIntegrationSettings() {
    setIsSavingIntegrationSettings(true);
    setSettingsError("");
    setSettingsNotice("");
    try {
      const response = await saveIntegrationSettings(integrationForm);
      setIntegrationSettings(response.settings);
      setIntegrationForm(integrationSettingsForm(response.settings));
      setIntegrationSettingsPersisted(response.persisted);
      if (response.integrations?.length) {
        setIntegrations(response.integrations);
      } else {
        setIntegrations(await fetchIntegrationHealth());
      }
      await Promise.allSettled([refreshDownloads(), refreshDownloadResources(true), refreshDownloadPreferences(true)]);
      setReadiness(await fetchReadiness());
      setSettingsNotice(response.persisted ? "Integration settings saved and applied." : "Integration settings applied for this process. Add Postgres to persist them.");
      setAPIState("live");
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : "Integration settings save failed");
    } finally {
      setIsSavingIntegrationSettings(false);
    }
  }

  async function persistLibrarySettings() {
    setIsSavingLibrarySettings(true);
    setSettingsError("");
    setSettingsNotice("");
    try {
      const response = await saveLibrarySettings(librarySettingsForm);
      setLibrarySettings(response.settings);
      setLibrarySettingsForm(response.settings);
      setLibrarySettingsPersisted(response.persisted);
      setReadiness(await fetchReadiness());
      setSettingsNotice(response.persisted ? "Library settings saved and applied." : "Library settings applied for this process. Add Postgres to persist them.");
      setAPIState("live");
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : "Library settings save failed");
    } finally {
      setIsSavingLibrarySettings(false);
    }
  }

  async function saveAPIKeySetting() {
    setStoredAPIKey(apiKeyInput);
    setSettingsError("");
    setSettingsNotice(apiKeyInput.trim() ? "API key saved for this browser." : "API key cleared for this browser.");
    try {
      const [nextProviders, nextIntegrations, nextStatus, nextReadiness, nextCompatibility] = await Promise.all([fetchProviderHealth(), fetchIntegrationHealth(), fetchSystemStatus(), fetchReadiness(), fetchReadarrCompatibility()]);
      setProviders(nextProviders);
      setIntegrations(nextIntegrations);
      setSystemStatus(nextStatus);
      setReadiness(nextReadiness);
      setReadarrCompatibility(nextCompatibility);
      setAPIState("live");
    } catch (error) {
      setAPIState("offline");
      setSettingsError(error instanceof Error ? error.message : "API key saved, but the API is still unavailable");
    }
  }

  const readyCount = providers.filter((provider) => provider.status === "ready").length;
  const integrationReadyCount = integrations.filter((integration) => integration.status === "ready").length;
  const databaseReady = systemStatus?.databaseType ? systemStatus.databaseType !== "none" : false;
  const databaseLabel = systemStatus ? (databaseReady ? `${systemStatus.databaseType} persistence` : "No database") : "Database unknown";
  const activeNav = navItems.find((item) => item.id === activeView) ?? navItems[0];

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <Library size={22} />
          </div>
          <div>
            <strong>Librarry</strong>
            <span>Readarr replacement</span>
          </div>
        </div>

        <nav className="nav-list" aria-label="Primary navigation">
          {navItems.map((item) => (
            <button className={item.id === activeView ? "nav-item active" : "nav-item"} key={item.id} onClick={() => setActiveView(item.id)} type="button">
              <item.icon size={17} />
              <span>{item.label}</span>
            </button>
          ))}
        </nav>

        <div className="sidebar-footer">
          <span className={apiState === "live" ? "status-dot ready" : "status-dot muted"} />
          <span>{apiState === "live" ? "API connected" : apiState === "checking" ? "Checking API" : "Demo data"}</span>
        </div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div>
            <h1>{activeNav.title}</h1>
            <p>{activeNav.description}</p>
          </div>
          <div className="topbar-status-group">
            <div className="topbar-status ready">
              <CheckCircle2 size={18} />
              <span>{readyCount}/{providers.length} providers ready</span>
            </div>
            <div className={databaseReady ? "topbar-status ready" : "topbar-status warn"}>
              <Database size={18} />
              <span>{databaseLabel}</span>
            </div>
          </div>
        </header>

        {systemStatus && !databaseReady ? (
          <section className="readiness-callout" aria-label="Persistence setup">
            <div>
              <strong>Database persistence is required for the Readarr workflow.</strong>
              <p>
                Wanted queues, author monitoring, release decisions, import reviews, history, and persisted integration settings need Postgres.
                Set <code>LIBRARRY_DATABASE_URL</code> and restart the API.
              </p>
            </div>
            <button className="secondary-action compact" onClick={() => setActiveView("settings")} type="button">
              <Settings size={16} />
              Settings
            </button>
          </section>
        ) : null}

        {readiness && (activeView === "dashboard" || activeView === "providers" || activeView === "settings") ? (
          <section className={`setup-checklist ${readiness.status}`} aria-label="Readarr workflow setup">
            <div className="setup-checklist-header">
              <div>
                <h2>Readarr workflow setup</h2>
                <p>{readiness.summary}</p>
              </div>
              <span className={`setup-status-badge ${readiness.status}`}>{readiness.status.replace(/_/g, " ")}</span>
            </div>
            <div className="setup-step-grid">
              {readiness.steps.map((step) => (
                <article className={`setup-step ${step.status}`} key={step.id}>
                  <div className="setup-step-main">
                    <span className={`status-dot ${step.status === "ready" ? "ready" : "warn"}`} />
                    <div>
                      <strong>{step.title}</strong>
                      <span>{step.message}</span>
                    </div>
                  </div>
                  <div className="setup-step-side">
                    {step.required ? <em>Required</em> : <em>Optional</em>}
                    {step.actionLabel && step.targetView ? (
                      <button className="secondary-action compact" onClick={() => setActiveView(readinessTargetView(step.targetView!))} type="button">
                        {step.actionLabel}
                      </button>
                    ) : null}
                  </div>
                </article>
              ))}
            </div>
          </section>
        ) : null}

        {(activeView === "dashboard" || activeView === "providers" || activeView === "settings") ? (
          <section className="readarr-compatibility-panel" aria-label="Readarr compatibility status">
            <div className="readarr-compatibility-header">
              <div>
                <h2>Readarr compatibility</h2>
                <p>{readarrCompatibility.summary}</p>
              </div>
              <div className="readarr-compatibility-metrics" aria-label="Compatibility summary">
                <article>
                  <span>Routes</span>
                  <strong>{readarrCompatibility.compatibleRoutes}</strong>
                </article>
                <article>
                  <span>Ready</span>
                  <strong>{readarrCompatibility.readyAreas}</strong>
                </article>
                <article>
                  <span>Partial</span>
                  <strong>{readarrCompatibility.partialAreas}</strong>
                </article>
                <article>
                  <span>Auth</span>
                  <strong>{readarrCompatibilityAuthLabel(readarrCompatibility.authMode)}</strong>
                </article>
              </div>
            </div>
            <div className="readarr-compatibility-grid">
              {readarrCompatibility.categories.map((category) => (
                <article className={`readarr-compatibility-card ${category.status}`} key={category.id}>
                  <div className="readarr-compatibility-card-heading">
                    <span className={`status-dot ${category.status === "ready" ? "ready" : "warn"}`} />
                    <strong>{category.title}</strong>
                    <em>{readarrCompatibilityStatusLabel(category.status)}</em>
                  </div>
                  <p>{category.message}</p>
                  <div className="readarr-compatibility-examples">
                    <span>{category.endpointCount} endpoints</span>
                    {category.examples.slice(0, 3).map((example) => (
                      <code key={example}>{example}</code>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          </section>
        ) : null}

        <section className="search-strip" aria-label="Metadata search controls" hidden={activeView !== "search"}>
          <div className="segmented" role="group" aria-label="Search type">
            {searchModeOptions.map((option) => (
              <button
                className={searchMode === option ? "selected" : ""}
                key={option}
                onClick={() => {
                  setSearchMode(option);
                  setResults([]);
                  setSelectedID("");
                  setPendingWantedReview(null);
                  setReleases([]);
                  clearSearchFilters();
                }}
                type="button"
              >
                {option}
              </button>
            ))}
          </div>
          <div className="search-input">
            <Search size={18} />
            <input
              value={query}
              onChange={(event) => {
                if (searchMode === "author") {
                  setAuthorQuery(event.target.value);
                  return;
                }
                setBookQuery(event.target.value);
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter") runSearch();
              }}
              placeholder={searchMode === "author" ? "Search author name" : "Search title, author, series, or ISBN"}
            />
          </div>
          <div className="segmented" role="group" aria-label={searchMode === "author" ? "Target format" : "Format"}>
            {["any", "ebook", "audiobook"].map((option) => (
              <button
                className={format === option ? "selected" : ""}
                key={option}
                onClick={() => setFormat(option)}
                type="button"
              >
                {option}
              </button>
            ))}
          </div>
          <button className="primary-action" onClick={runSearch} type="button">
            <FileSearch size={17} />
            <span>{isSearching ? "Searching" : searchMode === "author" ? "Find author" : "Search"}</span>
          </button>
        </section>

        <section className="review-inbox" aria-label="Review inbox" hidden={activeView !== "dashboard"}>
          <div className="panel-heading review-inbox-heading">
            <div>
              <h2>Review inbox</h2>
              <p>
                {reviewInboxTotal
                  ? `${reviewInboxTotal} book workflow items need review or recovery.`
                  : "No metadata, import, or acquisition reviews are pending."}
              </p>
            </div>
            <div className="monitor-actions">
              <button className="secondary-action compact" disabled={isRefreshingAcquisitionQueue} onClick={() => refreshAcquisitionQueue()} type="button">
                <RefreshCw size={16} />
                {isRefreshingAcquisitionQueue ? "Refreshing" : "Refresh queue"}
              </button>
              <button className="secondary-action compact" onClick={refreshWantedAndHistory} type="button">
                <HistoryIcon size={16} />
                Refresh reviews
              </button>
            </div>
          </div>
          <div className="review-inbox-metrics">
            {[
              ["Metadata", metadataReviewSummary.items, `${metadataReviewSummary.conflicts} conflicts`],
              ["Authors", authorMetadataReviews.length, "skipped candidates"],
              ["Imports", importReviews.length, "pending files"],
              ["Blocked", acquisitionQueue?.summary.blocked ?? dashboardBlockedItems.length, "book acquisitions"],
              ["Failed", dashboardFailedDownloads.length, "downloads"]
            ].map(([label, value, detail]) => (
              <div className="review-inbox-metric" key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
                <em>{detail}</em>
              </div>
            ))}
          </div>
          <div className="review-inbox-lanes">
            <article className="review-lane">
              <div className="review-lane-heading">
                <div>
                  <strong>Metadata review</strong>
                  <span>{metadataReviewSummary.protected} protected fields</span>
                </div>
                <button className="secondary-action compact" onClick={() => setActiveView("wanted")} type="button">
                  Wanted
                </button>
              </div>
              <div className="review-lane-list">
                {dashboardMetadataReviews.length ? (
                  dashboardMetadataReviews.map((review) => (
                    <button className="review-inbox-row" key={review.wantedItem.id} onClick={() => openWantedItem(review.wantedItem)} type="button">
                      <div>
                        <strong>{review.wantedItem.title}</strong>
                        <span>
                          {review.wantedItem.authorName || review.wantedItem.format} · {review.conflictCount} conflict{review.conflictCount === 1 ? "" : "s"}
                        </span>
                      </div>
                      <em>{review.recordCount} records</em>
                    </button>
                  ))
                ) : (
                  <div className="review-inbox-empty">No wanted metadata conflicts.</div>
                )}
              </div>
            </article>

            <article className="review-lane">
              <div className="review-lane-heading">
                <div>
                  <strong>Author candidates</strong>
                  <span>{authorMetadataReviews.length} skipped by policy</span>
                </div>
                <button className="secondary-action compact" onClick={() => setActiveView("wanted")} type="button">
                  Authors
                </button>
              </div>
              <div className="review-lane-list">
                {dashboardAuthorReviews.length ? (
                  dashboardAuthorReviews.map((review) => {
                    const wantedActionID = `${review.id}:wanted`;
                    const ignoreActionID = `${review.id}:ignore`;
                    return (
                      <article className="review-inbox-row action-row" key={review.id}>
                        <div>
                          <strong>{review.title || review.result.work.title || "Untitled"}</strong>
                          <span>
                            {review.authorName || firstAuthorName(review.result)} · {authorSkippedDateLabel(review.result)} · {review.reason}
                          </span>
                        </div>
                        <div className="review-inbox-actions">
                          <button className="secondary-action compact" disabled={Boolean(authorReviewActionID)} onClick={() => resolveAuthorReview(review, "wanted")} type="button">
                            {authorReviewActionID === wantedActionID ? "Marking" : "Wanted"}
                          </button>
                          <button className="secondary-action compact danger-outline" disabled={Boolean(authorReviewActionID)} onClick={() => resolveAuthorReview(review, "ignore")} type="button">
                            {authorReviewActionID === ignoreActionID ? "Ignoring" : "Ignore"}
                          </button>
                        </div>
                      </article>
                    );
                  })
                ) : (
                  <div className="review-inbox-empty">No author candidates pending.</div>
                )}
              </div>
            </article>

            <article className="review-lane">
              <div className="review-lane-heading">
                <div>
                  <strong>Import review</strong>
                  <span>{importReviews.length} unmatched files</span>
                </div>
                <button className="secondary-action compact" onClick={() => setActiveView("imports")} type="button">
                  Imports
                </button>
              </div>
              <div className="review-lane-list">
                {dashboardImportReviews.length ? (
                  dashboardImportReviews.map((review) => (
                    <button className="review-inbox-row" key={review.id} onClick={() => openImportReview(review)} type="button">
                      <div>
                        <strong>{review.title || review.sourcePath.split("/").pop() || "Pending import"}</strong>
                        <span>
                          {review.authorName || "Unknown author"} · {review.mediaFormat} · {review.reason}
                        </span>
                      </div>
                      <em>{formatBytes(review.sizeBytes ?? 0)}</em>
                    </button>
                  ))
                ) : (
                  <div className="review-inbox-empty">No import reviews pending.</div>
                )}
              </div>
            </article>

            <article className="review-lane">
              <div className="review-lane-heading">
                <div>
                  <strong>Blocked acquisition</strong>
                  <span>{acquisitionQueue?.summary.blocked ?? dashboardBlockedItems.length} books blocked</span>
                </div>
                <button className="secondary-action compact" onClick={() => setActiveView("wanted")} type="button">
                  Wanted
                </button>
              </div>
              <div className="review-lane-list">
                {dashboardBlockedItems.length ? (
                  dashboardBlockedItems.map((item) => {
                    const ActionIcon = acquisitionQueueActionIcon(item.state);
                    const actionID = acquisitionQueueActionID(item);
                    return (
                      <article className="review-inbox-row action-row" key={item.wantedItem.id}>
                        <button className="review-inbox-row-main" onClick={() => openWantedItem(item.wantedItem)} type="button">
                          <strong>{item.wantedItem.title}</strong>
                          <span>{item.nextAction}</span>
                        </button>
                        <button
                          className="secondary-action compact"
                          disabled={acquisitionQueueActionDisabled(item) || acquisitionActionID === actionID}
                          onClick={() => runAcquisitionQueueAction(item)}
                          type="button"
                        >
                          <ActionIcon size={15} />
                          {acquisitionActionID === actionID ? "Working" : acquisitionQueueActionLabel(item)}
                        </button>
                      </article>
                    );
                  })
                ) : (
                  <div className="review-inbox-empty">No blocked book acquisitions.</div>
                )}
              </div>
            </article>

            <article className="review-lane">
              <div className="review-lane-heading">
                <div>
                  <strong>Failed downloads</strong>
                  <span>{dashboardFailedDownloads.length} need recovery</span>
                </div>
                <button className="secondary-action compact" onClick={() => setActiveView("downloads")} type="button">
                  Queue
                </button>
              </div>
              <div className="review-lane-list">
                {dashboardFailedDownloadRows.length ? (
                  dashboardFailedDownloadRows.map((download) => {
                    const actionID = `${download.id}:recover`;
                    return (
                      <article className="review-inbox-row action-row" key={downloadKey(download)}>
                        <button className="review-inbox-row-main" onClick={() => openDownloadActivity(download)} type="button">
                          <strong>{download.name || download.id}</strong>
                          <span>{download.failureReason || download.importError || download.state}</span>
                        </button>
                        <button className="secondary-action compact" disabled={isRecoveringFailed} onClick={() => runFailedRecovery(download, { autoGrab: true, force: true })} type="button">
                          <HardDriveDownload size={15} />
                          {downloadActionID === actionID ? "Recovering" : "Recover"}
                        </button>
                      </article>
                    );
                  })
                ) : (
                  <div className="review-inbox-empty">No failed downloads.</div>
                )}
              </div>
            </article>
          </div>
        </section>

        <section className="provider-grid" aria-label="Provider health" hidden={activeView !== "dashboard" && activeView !== "providers"}>
          {providers.map((provider) => (
            <article className="provider-tile" key={provider.name}>
              <div className="provider-header">
                <span className={provider.status === "ready" ? "status-dot ready" : "status-dot warn"} />
                <strong>{provider.name}</strong>
              </div>
              <p>{provider.message}</p>
              <span className="provider-state">{provider.status.replace(/_/g, " ")}</span>
            </article>
          ))}
        </section>

        <section
          className="integration-strip"
          aria-label="Acquisition integrations"
          hidden={activeView !== "dashboard" && activeView !== "providers" && activeView !== "downloads"}
        >
          <div className="integration-summary">
            <HardDriveDownload size={18} />
            <span>{integrationReadyCount}/{integrations.length || 2} acquisition integrations ready</span>
          </div>
          {integrations.map((integration) => (
            <div className="integration-pill" key={integration.name}>
              <span className={integration.status === "ready" ? "status-dot ready" : "status-dot warn"} />
              <strong>{integration.name}</strong>
              <span>{integration.message}</span>
            </div>
          ))}
        </section>

        <section className="library-panel library-overview" aria-label="Library management" hidden={activeView !== "library"}>
          <div className="panel-heading library-heading">
            <div>
              <h2>Authors and books</h2>
              <p>
                {wantedItems.length
                  ? `${librarySummary.authors} authors · ${librarySummary.monitoredBooks} monitored books · ${librarySummary.missing} missing.`
                  : "Search metadata, monitor authors, and mark books wanted to build the library plan."}
              </p>
            </div>
            <div className="library-toolbar">
              <div className="library-search">
                <Search size={16} />
                <input
                  value={libraryTextFilter}
                  onChange={(event) => setLibraryTextFilter(event.target.value)}
                  placeholder="Filter authors or books"
                />
              </div>
              <div className="segmented compact" role="group" aria-label="Library format">
                {(["all", "ebook", "audiobook"] as LibraryFormatFilter[]).map((option) => (
                  <button
                    className={libraryFormatFilter === option ? "selected" : ""}
                    key={option}
                    onClick={() => setLibraryFormatFilter(option)}
                    type="button"
                  >
                    {option}
                  </button>
                ))}
              </div>
              <button className="secondary-action compact" disabled={isRunningAuthorMonitor} onClick={() => runAuthorSubscriptionMonitor({ force: false })} type="button">
                <RadioTower size={16} />
                Due authors
              </button>
              <button className="secondary-action compact" disabled={isRunningAuthorMonitor} onClick={() => runAuthorSubscriptionMonitor({ force: true })} type="button">
                <RefreshCw size={16} />
                Force authors
              </button>
            </div>
          </div>
          {wantedError ? <div className={isPersistenceRequiredError(wantedError) ? "inline-note" : "inline-error"}>{appErrorMessage(wantedError)}</div> : null}
          {authorError ? <div className={isPersistenceRequiredError(authorError) ? "inline-note" : "inline-error"}>{appErrorMessage(authorError)}</div> : null}
          {libraryError ? <div className={isPersistenceRequiredError(libraryError) ? "inline-note" : "inline-error"}>{appErrorMessage(libraryError)}</div> : null}
          <div className="library-grid library-overview-metrics">
            {[
              ["Authors", librarySummary.authors],
              ["Monitored authors", librarySummary.monitoredAuthors],
              ["Monitored books", librarySummary.monitoredBooks],
              ["Missing", librarySummary.missing],
              ["Grabbed", librarySummary.grabbed],
              ["Present", librarySummary.present],
              ["Files", librarySummary.files],
              ["Overrides", librarySummary.manualOverrides]
            ].map(([label, value]) => (
              <div className="library-metric" key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
          <div className="library-management-grid">
            <div className="library-author-browser">
              <div className="library-section-heading">
                <strong>Author monitoring</strong>
                <span>{visibleLibraryAuthorRows.length} shown</span>
              </div>
              {visibleLibraryAuthorRows.length ? (
                visibleLibraryAuthorRows.map((row) => (
                  <article className="library-author-row" key={row.key}>
                    <div>
                      <strong>{row.authorName}</strong>
                      <span>
                        {row.formats.join(", ")} · {row.qualityProfiles.join(", ") || "standard"}
                      </span>
                    </div>
                    <div className="library-author-counts">
                      <span>{row.missing} missing</span>
                      <span>{row.present} present</span>
                      <span>{row.grabbed} grabbed</span>
                    </div>
                    <em>{row.lastSyncAt ? formatDateTime(row.lastSyncAt) : row.subscriptionCount ? "never synced" : "manual"}</em>
                  </article>
                ))
              ) : (
                <div className="wanted-empty">No authors match this library filter.</div>
              )}
            </div>
            <div className="library-book-browser">
              <div className="library-section-heading">
                <strong>Monitored books</strong>
                <span>{visibleLibraryBooks.length} shown</span>
              </div>
              {visibleLibraryBooks.length ? (
                visibleLibraryBooks.slice(0, 80).map((item) => {
                  const presence = wantedPresence.get(item.id) ?? "missing";
                  return (
                    <article className={`library-book-row ${presence}`} key={item.id}>
                      <div className="library-book-cover">
                        {item.coverUrl ? <img src={item.coverUrl} alt="" /> : <BookOpen size={18} />}
                      </div>
                      <div>
                        <strong>{item.title}</strong>
                        <span>
                          {item.authorName || "Unknown author"} · {item.format} · {item.qualityProfile}
                        </span>
                        <small>
                          {item.sourceProvider || "manual"} · {item.monitored ? "monitored" : "unmonitored"}
                          {item.manualOverrides?.length ? ` · ${item.manualOverrides.length} override${item.manualOverrides.length === 1 ? "" : "s"}` : ""}
                        </small>
                      </div>
                      <em className={`wanted-badge ${presence}`}>{wantedBadgeLabel(item, presence)}</em>
                      <div className="library-book-actions">
                        <button className="secondary-action compact" onClick={() => openWantedItem(item)} type="button">
                          <Pencil size={16} />
                          Review
                        </button>
                        <button
                          className="secondary-action compact"
                          disabled={isSearchingWanted}
                          onClick={async () => {
                            openWantedItem(item);
                            await runWantedReleaseSearch(item);
                          }}
                          type="button"
                        >
                          <FileSearch size={16} />
                          {isSearchingWanted && selectedWantedID === item.id ? "Searching" : "Search"}
                        </button>
                      </div>
                    </article>
                  );
                })
              ) : (
                <div className="wanted-empty">No monitored books match this library filter.</div>
              )}
            </div>
          </div>
        </section>

        <div className="content-grid" hidden={activeView !== "search"}>
          <section className="results-panel" aria-label="Search results">
            <div className="panel-heading">
              <div>
                <h2>{searchMode === "author" ? "Author identities" : "Candidate matches"}</h2>
                <p>
                  {searchMode === "author"
                    ? `${visibleSearchResults.length} of ${results.length} author records shown.`
                    : `${visibleSearchResults.length} of ${results.length} normalized results shown.`}
                </p>
              </div>
              <button
                className={searchFiltersOpen ? "icon-button active" : "icon-button"}
                type="button"
                aria-label="Filter results"
                aria-expanded={searchFiltersOpen}
                onClick={() => setSearchFiltersOpen((open) => !open)}
              >
                <SlidersHorizontal size={18} />
                {activeSearchFilterCount ? <span className="filter-count">{activeSearchFilterCount}</span> : null}
              </button>
            </div>

            <div className="search-filter-panel" hidden={!searchFiltersOpen}>
              <label>
                <span>Provider</span>
                <select value={searchProviderFilter} onChange={(event) => setSearchProviderFilter(event.target.value)}>
                  <option value="">All providers</option>
                  {searchProviderOptions.map((provider) => (
                    <option key={provider} value={provider}>{provider}</option>
                  ))}
                </select>
              </label>
              <label>
                <span>Confidence</span>
                <select value={searchConfidenceFilter} onChange={(event) => setSearchConfidenceFilter(event.target.value as SearchConfidenceFilter)}>
                  <option value="all">All confidence</option>
                  {searchConfidenceOptions.map((confidence) => (
                    <option key={confidence} value={confidence}>{confidence}</option>
                  ))}
                </select>
              </label>
              <label>
                <span>Evidence</span>
                <select value={searchEvidenceFilter} onChange={(event) => setSearchEvidenceFilter(event.target.value as SearchEvidenceFilter)}>
                  <option value="all">All evidence</option>
                  <option value="identifiers">ISBN or ASIN</option>
                  <option value="published">Publisher or date</option>
                  <option value="series">Series position</option>
                </select>
              </label>
              <button className="secondary-action compact" disabled={!activeSearchFilterCount} onClick={clearSearchFilters} type="button">
                <FilterX size={16} />
                Clear
              </button>
            </div>

            <div className="result-table" role="table">
              <div className="table-row table-head" role="row">
                <span>{searchMode === "author" ? "Author" : "Title"}</span>
                <span>{searchMode === "author" ? "Target" : "Edition"}</span>
                <span>Source</span>
                <span>Confidence</span>
                <span>Action</span>
              </div>
              {visibleSearchResults.map((result) => {
                const resultKey = searchResultKey(result);
                const existingWanted = wantedBySearchKey.get(resultKey);
                const resultNeedsReview = searchResultNeedsWantedReview(result);
                return (
                  <div className={resultKey === selectedSearchKey ? "table-row result-row selected" : "table-row result-row"} key={resultKey} role="row">
                    <button className="title-cell result-select" onClick={() => selectSearchResult(result)} type="button">
                      {result.kind === "author" ? <UserPlus size={16} /> : <BookOpen size={16} />}
                      <span>
                        <strong>{searchResultTitle(result)}</strong>
                        <small>{searchResultSubtitle(result)}</small>
                        <span className="result-chip-row" aria-hidden="true">
                          {searchResultMatchChips(result).map((chip) => (
                            <em className={`result-chip ${chip.tone ?? "neutral"}`} key={chip.label}>{chip.label}</em>
                          ))}
                        </span>
                      </span>
                    </button>
                    <span className="edition-cell">
                      <strong>{searchResultEditionSummary(result, format)}</strong>
                      <small>{searchResultEditionSubline(result)}</small>
                    </span>
                    <span>{result.provider}</span>
                    <span>
                      <em className={`confidence ${result.confidence}`}>{result.confidence}</em>
                    </span>
                    <span className="row-action">
                      {searchResultCanBeWanted(result) ? (
                        <button
                          className={existingWanted ? "row-action-button tracked" : resultNeedsReview ? "row-action-button review" : "row-action-button"}
                          disabled={isMarkingWanted}
                          onClick={() => {
                            if (existingWanted) {
                              openWantedItem(existingWanted);
                              return;
                            }
                            markWantedResult(result);
                          }}
                          type="button"
                        >
                          {existingWanted ? "Open" : isMarkingWanted && resultKey === selectedSearchKey ? "Marking" : resultNeedsReview ? "Review" : "Mark"}
                        </button>
                      ) : null}
                      <button
                        className="row-action-button"
                        disabled={isSubscribingAuthor || !result.work.authors?.length}
                        onClick={() => subscribeAuthorResult(result)}
                        type="button"
                      >
                        {isSubscribingAuthor && resultKey === selectedSearchKey ? "Saving" : result.kind === "author" ? "Monitor" : "Author"}
                      </button>
                    </span>
                  </div>
                );
              })}
              {!visibleSearchResults.length ? (
                <div className="search-empty" role="row">
                  <strong>No metadata candidates match the current filters.</strong>
                  <span>Clear filters or run a broader provider search.</span>
                </div>
              ) : null}
            </div>
          </section>

          <aside className="detail-panel" aria-label="Selected book details">
            {selected ? (
              <>
                <div className="cover-frame">
                  {selected.work.coverUrl ? <img src={selected.work.coverUrl} alt="" /> : selected.kind === "author" ? <UserPlus size={42} /> : <BookOpen size={42} />}
                </div>
                <h2>{searchResultTitle(selected)}</h2>
                <p className="detail-author">{searchResultSubtitle(selected)}</p>

                <div className="search-evidence-summary" aria-label="Selected metadata evidence">
                  {searchResultEvidenceSummary(selected, format).map((item) => (
                    <article className="search-evidence-item" key={item.label}>
                      <span>{item.label}</span>
                      <strong>{item.value}</strong>
                      <small>{item.detail}</small>
                    </article>
                  ))}
                </div>

                {selectedExistingWanted ? (
                  <div className="wanted-existing-callout" aria-label="Existing wanted item">
                    <strong>Already tracked</strong>
                    <span>{wantedBadgeLabel(selectedExistingWanted, wantedPresence.get(selectedExistingWanted.id))}</span>
                    <button className="secondary-action compact" onClick={() => openWantedItem(selectedExistingWanted)} type="button">
                      Open wanted
                    </button>
                  </div>
                ) : null}

                {selectedCanBeWanted && selectedWantedReviewReasons.length ? (
                  <div className={pendingWantedReviewKey === selectedSearchKey ? "wanted-review-gate active" : "wanted-review-gate"} aria-label="Wanted metadata review">
                    <strong>{pendingWantedReviewKey === selectedSearchKey ? "Confirm metadata choice" : "Review before marking wanted"}</strong>
                    <span>This candidate can become wanted, but the match evidence is not strong enough for a blind add.</span>
                    <ul>
                      {selectedWantedReviewReasons.map((reason) => (
                        <li key={reason}>{reason}</li>
                      ))}
                    </ul>
                    <div className="wanted-review-gate-actions">
                      <button className="secondary-action compact" disabled={isMarkingWanted} onClick={() => markWantedResult(selected, { force: true })} type="button">
                        {isMarkingWanted ? "Marking" : "Confirm mark"}
                      </button>
                      {pendingWantedReviewKey === selectedSearchKey ? (
                        <button className="secondary-action compact ghost" disabled={isMarkingWanted} onClick={() => setPendingWantedReview(null)} type="button">
                          Cancel
                        </button>
                      ) : null}
                    </div>
                  </div>
                ) : null}

                <dl className="detail-list">
                  <div>
                    <dt>Provider</dt>
                    <dd>{selected.provider}</dd>
                  </div>
                  {selected.kind === "author" ? (
                    <div>
                      <dt>Provider ID</dt>
                      <dd>{searchResultProviderKey(selected)}</dd>
                    </div>
                  ) : (
                    <div>
                      <dt>First published</dt>
                      <dd>{selected.work.firstPublishYear ?? "Unknown"}</dd>
                    </div>
                  )}
                  <div>
                    <dt>{selected.kind === "author" ? "Target format" : "Format"}</dt>
                    <dd>{selected.kind === "author" ? wantedFormat(format) : selected.edition?.format ?? "Any"}</dd>
                  </div>
                  {selected.kind === "author" ? null : (
                    <div>
                      <dt>Edition</dt>
                      <dd>{selected.edition?.title || selected.work.title}</dd>
                    </div>
                  )}
                  {selected.kind === "author" ? null : (
                    <div>
                      <dt>Language</dt>
                      <dd>{languageLabel(selected.edition?.language) || "Unknown"}</dd>
                    </div>
                  )}
                  {selected.kind === "author" ? null : (
                    <div>
                      <dt>Published</dt>
                      <dd>{compactStringList([selected.edition?.publishedDate, selected.edition?.publisher]).join(" · ") || selected.work.firstPublishYear || "Unknown"}</dd>
                    </div>
                  )}
                  {selected.kind === "author" ? (
                    <div>
                      <dt>Top work</dt>
                      <dd>{selected.work.description || "Unknown"}</dd>
                    </div>
                  ) : (
                    <div>
                      <dt>Identifiers</dt>
                      <dd>{searchResultIdentifierLabel(selected, 4)}</dd>
                    </div>
                  )}
                  {selected.kind === "author" ? null : (
                    <div>
                      <dt>Series</dt>
                      <dd>{searchResultSeriesLabel(selected) || "None"}</dd>
                    </div>
                  )}
                  <div>
                    <dt>Matched on</dt>
                    <dd>{selected.matchedOn.join(", ")}</dd>
                  </div>
                </dl>

                <div className="author-policy-control" aria-label="Author missing-book policy">
                  <span>Author policy</span>
                  <div className="segmented compact" role="group" aria-label="Author missing-book policy">
                    {authorMissingPolicyOptions.map((policy) => (
                      <button
                        className={authorMissingPolicy === policy ? "selected" : ""}
                        key={policy}
                        onClick={() => setAuthorMissingPolicy(policy)}
                        type="button"
                      >
                        {authorMissingPolicyLabel(policy)}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="detail-actions">
                  {selectedExistingWanted ? (
                    <button className="secondary-action" onClick={() => openWantedItem(selectedExistingWanted)} type="button">
                      <HardDriveDownload size={17} />
                      <span>Open wanted</span>
                    </button>
                  ) : selectedCanBeWanted ? (
                    <button className="secondary-action" onClick={() => markWantedResult(selected)} disabled={isMarkingWanted} type="button">
                      <HardDriveDownload size={17} />
                      <span>{isMarkingWanted ? "Marking" : selectedWantedReviewReasons.length ? "Review wanted" : "Mark wanted"}</span>
                    </button>
                  ) : null}
                  <button className="secondary-action" onClick={subscribeSelectedAuthor} disabled={isSubscribingAuthor || !selected.work.authors?.length} type="button">
                    <UserPlus size={17} />
                    <span>{isSubscribingAuthor ? "Saving" : selectedAuthorSubscription ? "Refresh author" : "Monitor author"}</span>
                  </button>
                  {selectedCanSearchReleases ? (
                    <button className="secondary-action" onClick={runReleaseSearch} type="button">
                      <Download size={17} />
                      {isSearchingReleases ? "Searching releases" : "Search releases"}
                    </button>
                  ) : null}
                </div>
                {authorError ? <div className={isPersistenceRequiredError(authorError) ? "inline-note detail-error" : "inline-error detail-error"}>{appErrorMessage(authorError)}</div> : null}
              </>
            ) : (
              <div className="empty-detail">Select a result.</div>
            )}
          </aside>
        </div>

        <section className="release-panel" aria-label="Release search results" hidden={activeView !== "search" || searchMode === "author"}>
          <div className="panel-heading">
            <div>
              <h2>Release search</h2>
              <p>
                {releases.length
                  ? `${releases.length} Prowlarr releases ready for paused download-client grab.`
                  : "Search releases from the selected metadata match."}
              </p>
            </div>
            {downloadStatus ? <span className="download-state">Queued: {downloadStatus.category}</span> : null}
          </div>
          {releaseError ? <div className="inline-error">{releaseError}</div> : null}
          <div className="release-list">
            {releases.map((release) => (
              <article className="release-row" key={release.id}>
                <div>
                  <strong>{release.title}</strong>
                  <span>
                    {release.indexer} · {release.protocol} · {formatBytes(release.sizeBytes ?? 0)} · {release.seeders ?? 0} seeders
                  </span>
                </div>
                <button className="secondary-action compact" onClick={() => runGrab(release)} type="button">
                  Grab paused
                </button>
              </article>
            ))}
          </div>
        </section>

        <section className="wanted-panel" aria-label="Wanted queue" hidden={activeView !== "wanted"}>
          <div className="panel-heading">
            <div>
              <h2>Wanted queue</h2>
              <p>
                {wantedItems.length
                  ? `${wantedSummary.missing} missing · ${metadataReviewSummary.items} metadata review · ${wantedSummary.grabbed} grabbed · ${wantedSummary.present} present · ${visibleWantedItems.length} shown.`
                  : "Mark a metadata result wanted to start Readarr-style acquisition planning."}
              </p>
            </div>
            <div className="monitor-actions">
              <div className="segmented wanted-scope" aria-label="Wanted queue filter">
                {(["missing", "review", "wanted", "grabbed", "all"] as WantedViewFilter[]).map((scope) => (
                  <button
                    className={wantedViewFilter === scope ? "selected" : ""}
                    key={scope}
                    onClick={() => setWantedViewFilter(scope)}
                    type="button"
                  >
                    {scope}
                  </button>
                ))}
              </div>
              <button className="secondary-action compact" disabled={!selectedWanted || isSearchingWanted} onClick={() => runWantedReleaseSearch()} type="button">
                <FileSearch size={16} />
                {isSearchingWanted ? "Searching" : "Search wanted"}
              </button>
            </div>
          </div>
          {wantedError ? <div className={isPersistenceRequiredError(wantedError) ? "inline-note" : "inline-error"}>{appErrorMessage(wantedError)}</div> : null}
          {acquisitionError ? <div className={isPersistenceRequiredError(acquisitionError) ? "inline-note" : "inline-error"}>{appErrorMessage(acquisitionError)}</div> : null}
          {wantedViewFilter === "review" && visibleMetadataReviewIDs.length ? (
            <div className="metadata-review-bulkbar" aria-label="Metadata review bulk actions">
              <div>
                <strong>{metadataReviewSummary.conflicts} unresolved metadata conflicts</strong>
                <span>{selectedMetadataReviewCount} selected · {visibleMetadataReviewIDs.length} shown</span>
              </div>
              <button className="secondary-action compact" onClick={toggleAllVisibleMetadataReviews} type="button">
                {allMetadataReviewsSelected ? <CheckSquare size={16} /> : <Square size={16} />}
                {allMetadataReviewsSelected ? "Clear shown" : "Select shown"}
              </button>
              <button className="secondary-action compact keep-current" disabled={selectedMetadataReviewCount === 0 || isConfirmingMetadataReviews} onClick={confirmSelectedMetadataReviews} type="button">
                <CheckCircle2 size={16} />
                {isConfirmingMetadataReviews ? "Confirming" : "Keep current"}
              </button>
              {metadataReviewConfirmOutcome ? (
                <em>{metadataReviewConfirmOutcome.fieldsConfirmed} field{metadataReviewConfirmOutcome.fieldsConfirmed === 1 ? "" : "s"} confirmed</em>
              ) : null}
            </div>
          ) : null}
          <div className="acquisition-overview" aria-label="Readarr acquisition queue summary">
            <div className="acquisition-summary-strip">
              {[
                ["Search", acquisitionQueue?.summary.needsSearch ?? 0],
                ["Ready", acquisitionQueue?.summary.readyToGrab ?? 0],
                ["Queued", acquisitionQueue?.summary.queued ?? 0],
                ["Import", acquisitionQueue?.summary.importReady ?? 0],
                ["Done", acquisitionQueue?.summary.imported ?? 0],
                ["Blocked", acquisitionQueue?.summary.blocked ?? 0]
              ].map(([label, value]) => (
                <div className="acquisition-metric" key={label}>
                  <span>{label}</span>
                  <strong>{value}</strong>
                </div>
              ))}
              <button className="secondary-action compact" disabled={isRefreshingAcquisitionQueue} onClick={() => refreshAcquisitionQueue()} type="button">
                <RefreshCw size={15} />
                {isRefreshingAcquisitionQueue ? "Refreshing" : "Refresh queue"}
              </button>
            </div>
            {selectedAcquisitionQueueItem ? (() => {
              const ActionIcon = acquisitionQueueActionIcon(selectedAcquisitionQueueItem.state);
              const selectedActionID = acquisitionQueueActionID(selectedAcquisitionQueueItem);
              return (
                <div className={`acquisition-selected ${acquisitionQueueStateTone(selectedAcquisitionQueueItem.state)}`}>
                  <div>
                    <strong>{acquisitionQueueStateLabel(selectedAcquisitionQueueItem.state)}</strong>
                    <span>{selectedAcquisitionQueueItem.nextAction}</span>
                  </div>
                  <em>
                    {selectedAcquisitionQueueItem.approvedCount} approved · {selectedAcquisitionQueueItem.rejectedCount} rejected · {selectedAcquisitionQueueItem.downloads?.length ?? 0} queued
                  </em>
                  <button
                    className="secondary-action compact"
                    disabled={acquisitionQueueActionDisabled(selectedAcquisitionQueueItem) || acquisitionActionID === selectedActionID}
                    onClick={() => runAcquisitionQueueAction(selectedAcquisitionQueueItem)}
                    type="button"
                  >
                    <ActionIcon size={15} />
                    {acquisitionActionID === selectedActionID ? "Working" : acquisitionQueueActionLabel(selectedAcquisitionQueueItem)}
                  </button>
                </div>
              );
            })() : null}
            {highlightedAcquisitionItems.length ? (
              <div className="acquisition-row-list" aria-label="Book acquisition actions">
                {highlightedAcquisitionItems.map((item) => {
                  const ActionIcon = acquisitionQueueActionIcon(item.state);
                  const actionID = acquisitionQueueActionID(item);
                  return (
                    <article className="acquisition-row" key={item.wantedItem.id}>
                      <span className={`acquisition-state-dot ${acquisitionQueueStateTone(item.state)}`} />
                      <button className="acquisition-row-main" onClick={() => openWantedItem(item.wantedItem)} type="button">
                        <strong>{item.wantedItem.title}</strong>
                        <small>{item.wantedItem.authorName || item.wantedItem.format}</small>
                      </button>
                      <button
                        className="secondary-action compact acquisition-row-action"
                        disabled={acquisitionQueueActionDisabled(item) || acquisitionActionID === actionID}
                        onClick={() => runAcquisitionQueueAction(item)}
                        type="button"
                      >
                        <ActionIcon size={15} />
                        {acquisitionActionID === actionID ? "Working" : acquisitionQueueActionLabel(item)}
                      </button>
                    </article>
                  );
                })}
              </div>
            ) : null}
          </div>
          <div className="wanted-grid">
            <div className="wanted-list">
              {visibleWantedItems.length ? (
                visibleWantedItems.map((item) => {
                  const presence = wantedPresence.get(item.id);
                  const review = wantedMetadataReviewByID.get(item.id);
                  const reviewSelectable = wantedViewFilter === "review" && Boolean(review);
                  const reviewSelected = selectedMetadataReviewSet.has(item.id);
                  return (
                    <div className={reviewSelectable ? "wanted-item-shell selectable" : "wanted-item-shell"} key={item.id}>
                      {reviewSelectable ? (
                        <label className="metadata-review-selector" title="Select metadata review">
                          <input checked={reviewSelected} onChange={() => toggleMetadataReviewSelection(item)} type="checkbox" aria-label={`Select metadata review for ${item.title}`} />
                        </label>
                      ) : null}
                      <button
                        className={item.id === selectedWanted?.id ? "wanted-item selected" : "wanted-item"}
                        onClick={() => {
                          setSelectedWantedID(item.id);
                          setWantedReleaseFilter("all");
                          setWantedReleases([]);
                        }}
                        type="button"
                      >
                        <span>
                          <strong>{item.title}</strong>
                          <small>{wantedItemSubtitle(item, review)}</small>
                        </span>
                        <span className="wanted-badges">
                          <em className={`wanted-badge ${presence ?? "missing"}`}>{wantedBadgeLabel(item, presence)}</em>
                          {review ? <em className="wanted-badge review">{metadataReviewBadgeLabel(review)}</em> : null}
                        </span>
                      </button>
                    </div>
                  );
                })
              ) : (
                <div className="wanted-empty">No items match this wanted filter.</div>
              )}
            </div>
            <div className="wanted-release-list">
              {selectedWanted ? (
                <div className="wanted-edit-panel">
                  <div className="wanted-edit-header">
                    <div>
                      <strong>Metadata correction</strong>
                      <span>
                        {selectedWanted.sourceProvider || "manual"} · {selectedWanted.format} · {selectedWanted.sourceKey || selectedWanted.id}
                      </span>
                    </div>
                    <label className="wanted-monitor-toggle">
                      <input checked={wantedEditMonitored} onChange={(event) => setWantedEditMonitored(event.target.checked)} type="checkbox" />
                      <span>Monitored</span>
                    </label>
                  </div>
                  {selectedWanted.manualOverrides?.length ? (
                    <div className="wanted-override-list" aria-label="Manual metadata overrides">
                      {selectedWanted.manualOverrides.map((override) => (
                        <button
                          className="wanted-override-chip"
                          disabled={clearingWantedOverrideField === override.fieldName}
                          key={override.fieldName}
                          onClick={() => clearSelectedWantedOverride(override.fieldName)}
                          title={`Clear ${wantedOverrideLabel(override.fieldName)} override`}
                          type="button"
                        >
                          <span>
                            <strong>{wantedOverrideLabel(override.fieldName)}</strong>
                            <small>{override.value || "protected"}</small>
                          </span>
                          <em>{clearingWantedOverrideField === override.fieldName ? "Clearing" : "Reset"}</em>
                        </button>
                      ))}
                    </div>
                  ) : null}
                  <div className="metadata-provenance-panel" aria-label="Metadata provenance">
                    <div className="metadata-provenance-heading">
                      <div>
                        <strong>Provider provenance</strong>
                        <span>
                          {metadataProvenanceSummary(wantedMetadata, isLoadingWantedMetadata)}
                        </span>
                      </div>
                      {wantedMetadata?.manualOverrides?.length ? (
                        <em>{wantedMetadata.manualOverrides.length} override{wantedMetadata.manualOverrides.length === 1 ? "" : "s"} protected</em>
                      ) : null}
                    </div>
                    {wantedMetadata?.fields.length ? (
                      <div className="metadata-field-list" aria-label="Metadata field evidence">
                        {wantedMetadata.fields.map((field) => (
                          <article className={`metadata-field-row${field.conflict ? " conflict" : ""}`} key={field.fieldName}>
                            <div>
                              <strong>{field.label}</strong>
                              <span>{metadataFieldSourceLabel(field)}</span>
                            </div>
                            <div>
                              <strong>{field.canonicalValue || "No canonical value"}</strong>
                              <span>{metadataFieldCandidateSummary(field)}</span>
                              {metadataFieldCanConfirmCanonical(field) || metadataFieldApplicableCandidates(field).length ? (
                                <div className="metadata-field-candidate-actions" aria-label={`${field.label} provider candidates`}>
                                  {metadataFieldCanConfirmCanonical(field) ? (
                                    <button
                                      className="secondary-action compact keep-current"
                                      disabled={applyingMetadataCandidateID === metadataFieldCanonicalActionID(field)}
                                      onClick={() => confirmSelectedMetadataCanonical(field)}
                                      type="button"
                                    >
                                      {applyingMetadataCandidateID === metadataFieldCanonicalActionID(field) ? "Keeping" : "Keep current"}
                                    </button>
                                  ) : null}
                                  {metadataFieldApplicableCandidates(field).map((candidate) => {
                                    const actionID = `${field.fieldName}:${candidate.provider}:${candidate.value}`;
                                    return (
                                      <button
                                        className="secondary-action compact"
                                        disabled={applyingMetadataCandidateID === actionID}
                                        key={`${candidate.provider}:${candidate.providerKey}:${candidate.value}`}
                                        onClick={() => applySelectedMetadataCandidate(field, candidate)}
                                        type="button"
                                      >
                                        {applyingMetadataCandidateID === actionID ? "Applying" : `Use ${candidate.provider}`}
                                      </button>
                                    );
                                  })}
                                </div>
                              ) : null}
                            </div>
                            <em>{field.conflict ? "review" : field.reviewResolved ? "confirmed" : field.protected ? "protected" : "ok"}</em>
                          </article>
                        ))}
                      </div>
                    ) : null}
                    {wantedMetadata?.records.length ? (
                      <>
                        <div className="metadata-section-label">Provider records</div>
                        <div className="metadata-record-list">
                          {wantedMetadata.records.map((record) => {
                            const corrections = metadataRecordCorrections(record, wantedMetadata);
                            const actionID = metadataRecordActionID(record);
                            return (
                              <article className="metadata-record-row" key={actionID}>
                                <div>
                                  <strong>{record.provider}</strong>
                                  <span>{record.entityType} · {record.providerKey}</span>
                                </div>
                                <div>
                                  <strong>{metadataRecordPrimaryLine(record)}</strong>
                                  <span>{metadataRecordSecondaryLine(record)}</span>
                                </div>
                                <div className="metadata-record-actions">
                                  {corrections.length ? (
                                    <button
                                      className="secondary-action compact"
                                      disabled={Boolean(applyingMetadataRecordID || applyingMetadataCandidateID)}
                                      onClick={() => applySelectedMetadataRecord(record)}
                                      title={`Apply ${corrections.length} metadata field${corrections.length === 1 ? "" : "s"} from ${record.provider}`}
                                      type="button"
                                    >
                                      <CheckCircle2 size={15} />
                                      {applyingMetadataRecordID === actionID ? "Applying" : "Use record"}
                                    </button>
                                  ) : null}
                                  <em>{metadataConfidenceLabel(record.confidence)}</em>
                                </div>
                              </article>
                            );
                          })}
                        </div>
                      </>
                    ) : (
                      <div className="metadata-provenance-empty">
                        {isLoadingWantedMetadata ? "Loading provenance." : "Create or refresh this item from metadata search to attach provider records."}
                      </div>
                    )}
                  </div>
                  <div className="wanted-edit-grid">
                    <label>
                      <span>Title</span>
                      <input value={wantedEditTitle} onChange={(event) => setWantedEditTitle(event.target.value)} placeholder="Book title" />
                    </label>
                    <label>
                      <span>Author</span>
                      <input value={wantedEditAuthor} onChange={(event) => setWantedEditAuthor(event.target.value)} placeholder="Author name" />
                    </label>
                    <label className="wide">
                      <span>Cover URL</span>
                      <input value={wantedEditCoverURL} onChange={(event) => setWantedEditCoverURL(event.target.value)} placeholder="https://covers.example/book.jpg" />
                    </label>
                    <label>
                      <span>Quality profile</span>
                      <select value={wantedEditQualityProfile} onChange={(event) => setWantedEditQualityProfile(event.target.value)}>
                        {selectedWantedQualityProfiles.length ? (
                          selectedWantedQualityProfiles.map((profile) => (
                            <option key={profileKey(profile)} value={profile.name}>
                              {profile.name} · {profile.mediaFormat}
                            </option>
                          ))
                        ) : (
                          <option value={wantedEditQualityProfile || "standard"}>{wantedEditQualityProfile || "standard"}</option>
                        )}
                      </select>
                    </label>
                  </div>
                  <div className="wanted-edit-actions">
                    <button className="secondary-action compact" disabled={isSavingWantedEdit || !wantedEditTitle.trim()} onClick={saveWantedEdit} type="button">
                      <CheckCircle2 size={16} />
                      {isSavingWantedEdit ? "Saving" : "Save correction"}
                    </button>
                    <button className="secondary-action compact danger-outline" disabled={isRemovingWanted} onClick={removeSelectedWanted} type="button">
                      <Trash2 size={16} />
                      {isRemovingWanted ? "Removing" : "Remove wanted"}
                    </button>
                  </div>
                </div>
              ) : null}
              <div className="wanted-release-review-bar">
                <div>
                  <strong>Release review</strong>
                  <span>
                    {wantedReleases.length
                      ? `${wantedReleaseSummary.approved} approved · ${wantedReleaseSummary.rejected} rejected · ${wantedReleases.length} stored`
                      : "No stored decisions for this wanted item."}
                  </span>
                </div>
                <div className="wanted-release-review-actions">
                  <div className="segmented compact" aria-label="Release decision filter">
                    {(["all", "approved", "rejected"] as ReleaseDecisionFilter[]).map((scope) => (
                      <button className={wantedReleaseFilter === scope ? "selected" : ""} key={scope} onClick={() => setWantedReleaseFilter(scope)} type="button">
                        {scope}
                      </button>
                    ))}
                  </div>
                  <button className="secondary-action compact" disabled={!selectedWanted || isLoadingWantedReleases} onClick={() => loadWantedReleaseDecisions()} type="button">
                    <RefreshCw size={15} />
                    {isLoadingWantedReleases ? "Loading" : "Previous"}
                  </button>
                </div>
              </div>
              {visibleWantedReleases.length ? (
                visibleWantedReleases.map((release) => (
                  <article className={release.approved ? "wanted-release approved" : "wanted-release rejected"} key={release.id}>
                    <div>
                      <div className="wanted-release-title">
                        <strong>{release.title}</strong>
                        <span>{release.approved ? "Approved" : "Rejected"}</span>
                      </div>
                      <p>
                        {release.indexer} · {release.protocol || "release"} · score {release.score.toFixed(1)} · {formatBytes(release.sizeBytes ?? 0)} · {release.seeders ?? 0} seeders · {release.leechers ?? 0} leechers
                      </p>
                      {release.categories?.length ? <small>{release.categories.join(", ")}</small> : null}
                      {release.rejectedReason ? <small className="wanted-release-rejection">{release.rejectedReason}</small> : null}
                    </div>
                    <div className="wanted-release-actions">
                      {release.infoUrl ? (
                        <a className="secondary-action compact" href={release.infoUrl} rel="noreferrer" target="_blank">
                          Details
                        </a>
                      ) : null}
                      <button
                        className={release.approved ? "secondary-action compact" : "secondary-action compact danger-outline"}
                        disabled={grabbingWantedReleaseID === releaseActionID(release, !release.approved)}
                        onClick={() => grabWantedRelease(release, !release.approved)}
                        type="button"
                      >
                        {grabbingWantedReleaseID === releaseActionID(release, !release.approved)
                          ? "Grabbing"
                          : release.approved
                            ? "Grab paused"
                            : "Force grab"}
                      </button>
                    </div>
                  </article>
                ))
              ) : (
                <div className="wanted-empty">
                  {isLoadingWantedReleases
                    ? "Loading stored release decisions."
                    : selectedWanted
                      ? "Search wanted releases to evaluate candidates."
                      : "No wanted item selected."}
                </div>
              )}
            </div>
          </div>
        </section>

        <section className="author-panel" aria-label="Author subscriptions" hidden={activeView !== "wanted"}>
          <div className="panel-heading">
            <div>
              <h2>Author subscriptions</h2>
              <p>
                {authorMonitorRun
                  ? `${authorMonitorRun.status}: checked ${authorMonitorRun.authorsChecked}, created ${authorMonitorRun.wantedCreated} wanted items.`
                  : authorSubscriptions.length
                    ? `${authorSubscriptions.length} monitored authors can create wanted items from metadata search.`
                    : "Monitor authors for new or missing books before release acquisition."}
              </p>
            </div>
            <div className="monitor-actions">
              <button className="secondary-action compact" disabled={isRunningAuthorMonitor} onClick={() => runAuthorSubscriptionMonitor({ force: false })} type="button">
                <RadioTower size={16} />
                Due authors
              </button>
              <button className="secondary-action compact" disabled={isRunningAuthorMonitor} onClick={() => runAuthorSubscriptionMonitor({ force: true })} type="button">
                <RefreshCw size={16} />
                Force authors
              </button>
            </div>
          </div>
          {authorError ? <div className={isPersistenceRequiredError(authorError) ? "inline-note" : "inline-error"}>{appErrorMessage(authorError)}</div> : null}
          <div className="author-grid">
            <div className="author-list">
              {authorSubscriptions.length ? (
                authorSubscriptions.map((subscription) => {
                  const policy = normalizedAuthorMissingPolicy(subscription.missingBookPolicy);
                  const updating = updatingAuthorID === subscription.id;
                  const monitorKey = authorSubscriptionKey(subscription);
                  const refreshingAuthor = authorMonitorTargetKey === monitorKey;
                  const removingAuthor = removingAuthorID === subscription.id;
                  const stats = authorSubscriptionStatsByKey.get(monitorKey) ?? emptyAuthorSubscriptionStats();
                  return (
                    <article className="author-row" key={subscription.id || `${subscription.provider}:${subscription.providerKey}:${subscription.format}`}>
                      <div className="author-row-main">
                        <strong>{subscription.authorName}</strong>
                        <span>
                          {subscription.provider} · {subscription.format} · {subscription.qualityProfile}
                        </span>
                        <small className="author-row-stats">{authorSubscriptionStatsSummary(stats)}</small>
                        <div className="author-row-counts" aria-label={`${subscription.authorName} wanted book status`}>
                          {authorSubscriptionStatsBadges(stats).map(([label, value]) => (
                            <span key={label}>{value} {label}</span>
                          ))}
                        </div>
                      </div>
                      <div className="author-row-controls">
                        <select
                          aria-label={`${subscription.authorName} missing-book policy`}
                          disabled={updating}
                          onChange={(event) => updateAuthorMissingPolicy(subscription, event.target.value as AuthorMissingBookPolicy)}
                          value={policy}
                        >
                          {authorMissingPolicyOptions.map((option) => (
                            <option key={option} value={option}>
                              {authorMissingPolicyLabel(option)}
                            </option>
                          ))}
                        </select>
                        <div className="author-row-actions">
                          <button
                            aria-label={`Open ${subscription.authorName} wanted books`}
                            className="icon-button"
                            disabled={!stats.firstWantedItem}
                            onClick={() => openAuthorSubscriptionBooks(subscription)}
                            title="Open wanted books"
                            type="button"
                          >
                            <BookOpen size={15} />
                          </button>
                          <button
                            aria-label={`Refresh ${subscription.authorName}`}
                            className="icon-button"
                            disabled={isRunningAuthorMonitor}
                            onClick={() => runAuthorSubscriptionMonitor(authorSubscriptionMonitorOptions(subscription))}
                            title={refreshingAuthor ? "Refreshing author" : "Refresh author"}
                            type="button"
                          >
                            <RefreshCw size={15} />
                          </button>
                          <button
                            aria-label={`Remove ${subscription.authorName}`}
                            className="icon-button danger"
                            disabled={!subscription.id || Boolean(removingAuthorID)}
                            onClick={() => removeAuthorSubscription(subscription)}
                            title={removingAuthor ? "Removing author" : "Remove author"}
                            type="button"
                          >
                            <Trash2 size={15} />
                          </button>
                        </div>
                        <em>{subscription.lastSyncAt ? formatDateTime(subscription.lastSyncAt) : "never synced"}</em>
                      </div>
                    </article>
                  );
                })
              ) : (
                <div className="wanted-empty">Select a search result and monitor its author.</div>
              )}
            </div>
            <div className="author-monitor-results">
              {authorMonitorRun ? (
                <>
                  <div className="author-monitor-summary">
                    <strong>{authorMonitorRun.status}</strong>
                    <span>
                      {authorMonitorRun.authorsChecked} checked · {authorMonitorRun.itemsFound} metadata hits · {authorMonitorRun.wantedCreated} wanted · {authorMonitorRun.errorCount} errors
                    </span>
                  </div>
                  {authorMonitorRun.items?.slice(0, 8).map((item) => (
                    <article className={item.error ? "author-monitor-row error" : "author-monitor-row"} key={item.subscription.id || item.subscription.providerKey}>
                      <div>
                        <strong>{item.subscription.authorName}</strong>
                        <span>
                          {item.resultsFound} hits · {item.wantedCreated} wanted · {item.skippedCount ?? 0} skipped
                          {item.error ? ` · ${item.error}` : ""}
                        </span>
                      </div>
                      <em>{item.subscription.format}</em>
                      {item.skippedItems?.length ? (
                        <div className="author-skipped-list">
                          {item.skippedItems.slice(0, 4).map((skipped) => {
                            const skippedKey = authorSkippedItemKey(item.subscription, skipped);
                            const busy = markingAuthorSkippedKey === skippedKey;
                            return (
                              <div className="author-skipped-row" key={skippedKey}>
                                <div>
                                  <strong>{skipped.result.work.title || skipped.result.edition?.title || "Untitled"}</strong>
                                  <span>
                                    {firstAuthorName(skipped.result)} · {authorSkippedDateLabel(skipped.result)} · {skipped.reason}
                                  </span>
                                </div>
                                <button className="secondary-action compact" disabled={Boolean(markingAuthorSkippedKey)} onClick={() => markSkippedAuthorCandidateWanted(item.subscription, skipped)} type="button">
                                  {busy ? "Marking" : "Mark wanted"}
                                </button>
                              </div>
                            );
                          })}
                        </div>
                      ) : null}
                    </article>
                  ))}
                </>
              ) : (
                <div className="wanted-empty">Run due authors to refresh monitored writers.</div>
              )}
              <div className="author-review-queue" aria-label="Author metadata review queue">
                <div className="author-review-heading">
                  <div>
                    <strong>Author review queue</strong>
                    <span>
                      {authorMetadataReviews.length
                        ? `${authorMetadataReviews.length} skipped metadata candidate${authorMetadataReviews.length === 1 ? "" : "s"} need review.`
                        : "No skipped author candidates are pending review."}
                    </span>
                  </div>
                  <button className="secondary-action compact" disabled={Boolean(authorReviewActionID)} onClick={refreshAuthorMetadataReviews} type="button">
                    <RefreshCw size={16} />
                    Refresh
                  </button>
                </div>
                {authorMetadataReviews.slice(0, 6).map((review) => {
                  const wantedActionID = `${review.id}:wanted`;
                  const ignoreActionID = `${review.id}:ignore`;
                  return (
                    <article className="author-review-row" key={review.id}>
                      <div>
                        <strong>{review.title || review.result.work.title || "Untitled"}</strong>
                        <span>
                          {review.authorName || firstAuthorName(review.result)} · {authorSkippedDateLabel(review.result)} · {review.reason}
                        </span>
                      </div>
                      <div className="author-review-actions">
                        <button className="secondary-action compact" disabled={Boolean(authorReviewActionID)} onClick={() => resolveAuthorReview(review, "wanted")} type="button">
                          {authorReviewActionID === wantedActionID ? "Marking" : "Mark wanted"}
                        </button>
                        <button className="secondary-action compact danger-outline" disabled={Boolean(authorReviewActionID)} onClick={() => resolveAuthorReview(review, "ignore")} type="button">
                          {authorReviewActionID === ignoreActionID ? "Ignoring" : "Ignore"}
                        </button>
                      </div>
                    </article>
                  );
                })}
              </div>
            </div>
          </div>
        </section>

        <section className="library-panel" aria-label="Library import" hidden={activeView !== "imports"}>
          <div className="panel-heading">
            <div>
              <h2>Library</h2>
              <p>
                {libraryScan
                  ? `${libraryScan.upserted} files indexed from ${libraryScan.roots.length} roots.`
                  : `${libraryFiles.length} tracked files from library scans and imports.`}
              </p>
            </div>
            <div className="library-actions">
              <button className="secondary-action compact" disabled={isScanningLibrary} onClick={() => runLibraryScan("ebook")} type="button">
                <FolderSearch size={16} />
                Ebook roots
              </button>
              <button className="secondary-action compact" disabled={isScanningLibrary} onClick={() => runLibraryScan("audiobook")} type="button">
                <FolderSearch size={16} />
                Audio roots
              </button>
              <button className="secondary-action compact" disabled={isScanningLibrary} onClick={() => runLibraryScan("any")} type="button">
                <RefreshCw size={16} />
                All roots
              </button>
              <button className="secondary-action compact" disabled={isImportingCompleted} onClick={() => runCompletedImport()} type="button">
                <HardDriveDownload size={16} />
                Completed
              </button>
            </div>
          </div>
          {libraryError ? <div className={isPersistenceRequiredError(libraryError) ? "inline-note" : "inline-error"}>{appErrorMessage(libraryError)}</div> : null}
          <div className="library-scan-row">
            <label className="library-scan-input">
              <FolderSearch size={17} />
              <span>Root</span>
              <input value={libraryScanRoot} onChange={(event) => setLibraryScanRoot(event.target.value)} placeholder="/data/media/books/ebooks" />
            </label>
            <div className="library-scan-buttons" role="group" aria-label="Scan custom library root">
              <button className="secondary-action compact" disabled={!libraryScanRoot.trim() || isScanningLibrary} onClick={() => runLibraryScan("ebook", { root: libraryScanRoot })} type="button">
                Ebooks
              </button>
              <button className="secondary-action compact" disabled={!libraryScanRoot.trim() || isScanningLibrary} onClick={() => runLibraryScan("audiobook", { root: libraryScanRoot })} type="button">
                Audio
              </button>
              <button className="secondary-action compact" disabled={!libraryScanRoot.trim() || isScanningLibrary} onClick={() => runLibraryScan("any", { root: libraryScanRoot })} type="button">
                All
              </button>
            </div>
          </div>
          {libraryScan?.roots.length ? (
            <div className="library-scan-result" aria-label="Last scanned roots">
              <strong>{isScanningLibrary ? "Scanning" : "Last scan"}</strong>
              <div>
                {libraryScan.roots.map((root) => (
                  <span title={root} key={root}>
                    {root}
                  </span>
                ))}
              </div>
            </div>
          ) : null}
          <div className="library-import-row">
            <div className="library-import-input">
              <FileCheck2 size={17} />
              <input
                value={importPath}
                onChange={(event) => setImportPath(event.target.value)}
                placeholder="Source file path to import into the selected wanted book"
              />
            </div>
            <button className="primary-action" disabled={!importPath.trim() || isImportingLibrary} onClick={runLibraryImport} type="button">
              <UploadCloud size={17} />
              {isImportingLibrary ? "Importing" : "Import"}
            </button>
          </div>
          <div className="library-import-options">
            <label>
              <span>Mode</span>
              <select value={libraryImportMode} onChange={(event) => setLibraryImportMode(event.target.value as typeof libraryImportMode)}>
                <option value="copy">Copy</option>
                <option value="move">Move</option>
                <option value="hardlink">Hardlink</option>
                <option value="hardlinkOrCopy">Hardlink or copy</option>
              </select>
            </label>
            <label>
              <span>Conflict</span>
              <select value={libraryConflictAction} onChange={(event) => setLibraryConflictAction(event.target.value as typeof libraryConflictAction)}>
                <option value="rename">Keep both</option>
                <option value="replace">Replace</option>
                <option value="skip">Skip</option>
                <option value="fail">Fail</option>
              </select>
            </label>
          </div>
          {libraryImport ? (
            <div className="library-import-result">
              <strong>{libraryImport.skipped ? "Import skipped" : libraryImport.file.title || "Imported file"}</strong>
              <span>{libraryImport.message || libraryImport.destinationPath}</span>
            </div>
          ) : null}
          {completedImport ? (
            <div className="library-import-result">
              <strong>
                Completed import: {completedImport.imported} imported, {completedImport.autoMatched} auto, {completedImport.reviewQueued} review, {completedImport.skipped} skipped,{" "}
                {completedImport.errored} errors
              </strong>
              <span>{completedImport.checked} downloads checked from the Librarry queue.</span>
            </div>
          ) : null}
          {bulkReviewOutcome ? (
            <div className="library-import-result">
              <strong>
                Bulk review: {bulkReviewOutcome.resolved} resolved, {bulkReviewOutcome.imported} imported, {bulkReviewOutcome.skipped} skipped,{" "}
                {bulkReviewOutcome.rejected} rejected, {bulkReviewOutcome.errored} errors
              </strong>
              <span>{bulkReviewOutcome.requested} pending review items processed.</span>
            </div>
          ) : null}
          <div className="library-grid">
            {[
              ["Tracked", libraryFiles.length],
              ["Imported", libraryFiles.filter((file) => file.importStatus === "imported").length],
              ["Review", importReviews.length],
              ["Ebooks", libraryFiles.filter((file) => file.mediaFormat === "ebook").length],
              ["Audiobooks", libraryFiles.filter((file) => file.mediaFormat === "audiobook").length],
              ["Scanned", libraryScan?.scanned ?? 0],
              ["Skipped", libraryScan?.skipped ?? 0]
            ].map(([label, value]) => (
              <div className="library-metric" key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
          {importReviews.length ? (
            <div className="import-review-list">
              <div className="import-review-bulkbar" aria-label="Bulk import review actions">
                <button className="secondary-action compact" disabled={visibleImportReviews.length === 0 || Boolean(reviewActionID)} onClick={toggleAllImportReviews} type="button">
                  {allImportReviewsSelected ? <CheckSquare size={16} /> : <Square size={16} />}
                  {allImportReviewsSelected ? "Clear all" : "Select all"}
                </button>
                <span>{selectedImportReviews.length} selected</span>
                <button
                  className="secondary-action compact"
                  disabled={selectedImportReviews.length === 0 || !selectedImportReviewsCanImport || Boolean(reviewActionID)}
                  onClick={() => runResolveImportReviewsBulk("import")}
                  title={selectedImportReviewsCanImport ? "Import selected reviews" : "Select a wanted item for each review before bulk import"}
                  type="button"
                >
                  <CheckCircle2 size={16} />
                  {reviewActionID === "bulk:import" ? "Importing" : "Import"}
                </button>
                <button className="secondary-action compact danger-outline" disabled={selectedImportReviews.length === 0 || Boolean(reviewActionID)} onClick={() => runResolveImportReviewsBulk("skip")} type="button">
                  <Trash2 size={16} />
                  {reviewActionID === "bulk:skip" ? "Skipping" : "Skip"}
                </button>
                <button className="secondary-action compact danger-outline" disabled={selectedImportReviews.length === 0 || Boolean(reviewActionID)} onClick={() => runResolveImportReviewsBulk("reject")} type="button">
                  <Trash2 size={16} />
                  {reviewActionID === "bulk:reject" ? "Rejecting" : "Reject"}
                </button>
              </div>
              {visibleImportReviews.map((review) => {
                const evidenceChips = importReviewEvidenceChips(review);
                const suggestedWanted = importReviewSuggestedWanted(review);
                const wantedCandidates = importReviewWantedCandidates(review);
                const resolvedWantedID = importReviewResolvedWantedID(review, selectedImportReviewWantedIDs);
                const requiresWantedChoice = !resolvedWantedID && wantedCandidates.length > 0;
                return (
                  <article className={selectedImportReviewSet.has(review.id) ? "import-review-row selected" : "import-review-row"} key={review.id}>
                    <label className="import-review-select" title="Select import review">
                      <input checked={selectedImportReviewSet.has(review.id)} onChange={() => toggleImportReviewSelection(review)} type="checkbox" aria-label={`Select ${review.title || review.sourcePath}`} />
                    </label>
                    <div>
                      <strong>{review.title || review.sourcePath.split("/").pop() || "Pending import"}</strong>
                      <span>
                        {review.authorName || "Unknown author"} · {review.mediaFormat} · {formatBytes(review.sizeBytes ?? 0)} · {review.reason}
                      </span>
                      <small>{review.sourcePath}</small>
                      {suggestedWanted ? <small className="import-review-suggestion">Suggested: {suggestedWanted}</small> : null}
                      {wantedCandidates.length ? (
                        <label className="import-review-candidate">
                          <span>{requiresWantedChoice ? "Choose wanted match" : "Wanted match"}</span>
                          <select
                            value={resolvedWantedID}
                            onChange={(event) => setSelectedImportReviewWantedIDs((current) => ({ ...current, [review.id]: event.target.value }))}
                            disabled={Boolean(review.wantedId)}
                          >
                            <option value="">{requiresWantedChoice ? "Select a wanted item" : "No wanted item"}</option>
                            {wantedCandidates.map((candidate) => {
                              const wantedID = stringMetadataValue(candidate.wantedId);
                              return (
                                <option value={wantedID} key={wantedID || importReviewCandidateLabel(candidate)}>
                                  {importReviewCandidateOptionLabel(candidate)}
                                </option>
                              );
                            })}
                          </select>
                        </label>
                      ) : null}
                      {evidenceChips.length ? (
                        <div className="import-review-evidence" aria-label="Import review evidence">
                          {evidenceChips.map((chip) => (
                            <span key={chip}>{chip}</span>
                          ))}
                        </div>
                      ) : null}
                    </div>
                    <div className="import-review-actions">
                      <button
                        className="secondary-action compact"
                        disabled={Boolean(reviewActionID) || !resolvedWantedID}
                        onClick={() => runResolveImportReview(review, "import", resolvedWantedID)}
                        title={resolvedWantedID ? "Import review into selected wanted item" : "Select a wanted item before importing"}
                        type="button"
                      >
                        <CheckCircle2 size={16} />
                        {reviewActionID === `${review.id}:import` ? "Importing" : "Import"}
                      </button>
                      <button
                        className="secondary-action compact danger-outline"
                        disabled={Boolean(reviewActionID)}
                        onClick={() => runResolveImportReview(review, "skip")}
                        type="button"
                      >
                        <Trash2 size={16} />
                        Skip
                      </button>
                      <button
                        className="secondary-action compact danger-outline"
                        disabled={Boolean(reviewActionID)}
                        onClick={() => runResolveImportReview(review, "reject")}
                        type="button"
                      >
                        <Trash2 size={16} />
                        Reject
                      </button>
                    </div>
                  </article>
                );
              })}
            </div>
          ) : null}
          <div className="library-file-list">
            {libraryFiles.slice(0, 8).map((file) => (
              <article className="library-file-row" key={file.id || file.path}>
                <div>
                  <strong>{file.title || file.path.split("/").pop()}</strong>
                  <span>
                    {file.authorName || "Unknown author"} · {file.mediaFormat} · {file.importStatus} · {formatBytes(file.sizeBytes ?? 0)}
                  </span>
                  <small>{file.path}</small>
                </div>
                <em>{file.extension || "file"}</em>
              </article>
            ))}
          </div>
        </section>

        <section className="monitor-panel" aria-label="Wanted monitor" hidden={activeView !== "dashboard" && activeView !== "wanted"}>
          <div className="panel-heading">
            <div>
              <h2>Monitor</h2>
              <p>
                {monitorRun
                  ? `${monitorRun.status}: checked ${monitorRun.wantedChecked}, approved ${monitorRun.approvedCount}, grabbed ${monitorRun.grabbedCount}.`
                  : "Run Readarr-style wanted monitoring across due items or force a full wanted scan."}
              </p>
            </div>
            <div className="monitor-actions">
              <button className="secondary-action compact" disabled={isRunningMonitor} onClick={() => runMonitor({ force: false })} type="button">
                <RadioTower size={16} />
                Due scan
              </button>
              <button className="secondary-action compact" disabled={isRunningMonitor} onClick={() => runMonitor({ force: true })} type="button">
                <RefreshCw size={16} />
                Force scan
              </button>
              <button className="secondary-action compact danger-outline" disabled={isRunningMonitor} onClick={() => runMonitor({ force: true, autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                Scan + grab paused
              </button>
              <button className="secondary-action compact" disabled={isRunningFeedSync} onClick={() => runFeedSync({})} type="button">
                <RadioTower size={16} />
                Feed
              </button>
              <button className="secondary-action compact danger-outline" disabled={isRunningFeedSync} onClick={() => runFeedSync({ autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                Feed + grab
              </button>
              <button className="secondary-action compact" disabled={isRunningUpgrade} onClick={() => runUpgrades({})} type="button">
                <TrendingUp size={16} />
                Upgrades
              </button>
              <button className="secondary-action compact danger-outline" disabled={isRunningUpgrade} onClick={() => runUpgrades({ autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                Upgrade + grab
              </button>
            </div>
          </div>
          {monitorError ? <div className={isPersistenceRequiredError(monitorError) ? "inline-note" : "inline-error"}>{appErrorMessage(monitorError)}</div> : null}
          {feedError ? <div className={isPersistenceRequiredError(feedError) ? "inline-note" : "inline-error"}>{appErrorMessage(feedError)}</div> : null}
          {upgradeError ? <div className={isPersistenceRequiredError(upgradeError) ? "inline-note" : "inline-error"}>{appErrorMessage(upgradeError)}</div> : null}
          <div className="monitor-grid">
            {[
              ["Wanted checked", monitorRun?.wantedChecked ?? 0],
              ["Releases found", monitorRun?.releasesFound ?? 0],
              ["Approved", monitorRun?.approvedCount ?? 0],
              ["Rejected", monitorRun?.rejectedCount ?? 0],
              ["Grabbed", monitorRun?.grabbedCount ?? 0],
              ["Errors", monitorRun?.errorCount ?? 0]
            ].map(([label, value]) => (
              <div className="monitor-metric" key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
          {monitorRun?.items?.length ? (
            <div className="monitor-results">
              {monitorRun.items.map((item) => (
                <article className={item.error ? "monitor-result error" : "monitor-result"} key={item.wantedItem.id}>
                  <strong>{item.wantedItem.title}</strong>
                  <span>
                    {item.releasesFound} releases · {item.approvedCount} approved · {item.rejectedCount} rejected
                  </span>
                  {item.grabbedDownload ? <em>Queued {item.grabbedDownload.category}</em> : null}
                  {item.error ? <small>{item.error}</small> : null}
                </article>
              ))}
            </div>
          ) : null}
          {upgradeRun ? (
            <div className="upgrade-results">
              <div className="upgrade-summary">
                <strong>{upgradeRun.status}</strong>
                <span>
                  {upgradeRun.wantedChecked} checked · {upgradeRun.releasesFound} releases · {upgradeRun.upgradeCount} upgrades · {upgradeRun.grabbedCount} grabbed
                </span>
              </div>
              {upgradeRun.items?.slice(0, 8).map((item) => (
                <article className={item.upgradeRelease ? "upgrade-row approved" : item.error ? "upgrade-row rejected" : "upgrade-row"} key={item.wantedItem.id}>
                  <div>
                    <strong>{item.wantedItem.title}</strong>
                    <span>
                      current {item.currentScore.toFixed(1)} / cutoff {item.cutoffScore.toFixed(1)}
                      {item.upgradeRelease ? ` · ${item.upgradeRelease.title} (${item.upgradeRelease.score.toFixed(1)})` : item.error ? ` · ${item.error}` : " · no upgrade"}
                    </span>
                  </div>
                  {item.grabbedDownload ? <em>Queued</em> : null}
                </article>
              ))}
            </div>
          ) : null}
          {feedSyncRun ? (
            <div className="feed-sync-results">
              <div className="feed-sync-summary">
                <strong>{feedSyncRun.status}</strong>
                <span>
                  {feedSyncRun.releasesSeen} feed releases · {feedSyncRun.matchedCount} matches · {feedSyncRun.approvedCount} approved · {feedSyncRun.grabbedCount} grabbed
                </span>
              </div>
              {feedSyncRun.matches?.slice(0, 8).map((match) => (
                <article className={match.release.approved ? "feed-sync-row approved" : "feed-sync-row rejected"} key={`${match.wantedItem.id}-${match.release.id || match.release.sourceId}`}>
                  <div>
                    <strong>{match.release.title}</strong>
                    <span>
                      {match.wantedItem.title} · score {match.release.score.toFixed(1)} · {match.release.approved ? "approved" : match.release.rejectedReason || "rejected"}
                    </span>
                  </div>
                  {match.grabbedDownload ? <em>Queued</em> : null}
                </article>
              ))}
            </div>
          ) : null}
        </section>

        <section className="downloads-panel" aria-label="Download activity" hidden={activeView !== "dashboard" && activeView !== "downloads"}>
          <div className="panel-heading">
            <div>
              <h2>Download activity</h2>
              <p>
                {filteredDownloads.length
                  ? `${filteredDownloads.length} of ${downloads.length} ${downloadScope === "all" ? "download-client" : "Librarry-tagged"} items visible; ${selectedDownloads.length} selected for queue actions.`
                  : downloadScope === "all"
                    ? "No download-client items are currently visible."
                    : "No Librarry-tagged downloads are currently visible."}
              </p>
            </div>
            <div className="download-toolbar">
              <div className="segmented download-scope" aria-label="Download queue scope">
                <button className={downloadScope === "all" ? "selected" : ""} onClick={() => changeDownloadScope("all")} type="button">
                  All clients
                </button>
                <button className={downloadScope === "librarry" ? "selected" : ""} onClick={() => changeDownloadScope("librarry")} type="button">
                  Librarry
                </button>
              </div>
              <button className="secondary-action compact" onClick={refreshDownloads} type="button">
                <RefreshCw size={16} />
                {isRefreshingDownloads ? "Refreshing" : "Refresh"}
              </button>
              <button className="secondary-action compact" disabled={isImportingCompleted || filteredDownloads.length === 0} onClick={() => runCompletedImport(filteredDownloads)} type="button">
                <UploadCloud size={16} />
                {isImportingCompleted ? "Importing" : "Import done"}
              </button>
              <button className="secondary-action compact" disabled={isRecoveringFailed || filteredDownloads.length === 0} onClick={() => runFailedRecovery(filteredDownloads, { autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                {isRecoveringFailed ? "Recovering" : "Recover failed"}
              </button>
              <button className="secondary-action compact" disabled={isRebalancingDownloads || filteredDownloads.length === 0} onClick={runQueueRebalance} type="button">
                <SlidersHorizontal size={16} />
                {isRebalancingDownloads ? "Balancing" : `Balance ${boundedPositiveInt(downloadRebalanceMax, 3, 25)}`}
              </button>
            </div>
          </div>
          <div className="download-client-strip" aria-label="Download client status">
            {downloadIntegrationStatuses.map((client) => (
              <article className="download-client-status" key={client.name}>
                <div>
                  <strong>{client.name}</strong>
                  <span>{client.message}</span>
                </div>
                <em className={`download-badge ${integrationStatusTone(client.status)}`}>{client.status}</em>
              </article>
            ))}
            <button className="secondary-action compact" onClick={() => setActiveView("settings")} type="button">
              <Settings size={16} />
              Clients
            </button>
          </div>
          <div className="download-filterbar" aria-label="Download filters">
            <label>
              <span>Find</span>
              <input value={downloadTextFilter} onChange={(event) => setDownloadTextFilter(event.target.value)} placeholder="Title, hash, tag, or path" />
            </label>
            <label>
              <span>Client</span>
              <select value={downloadClientFilter} onChange={(event) => setDownloadClientFilter(event.target.value)}>
                <option value="">All clients</option>
                {downloadClientOptions.map((client) => (
                  <option value={client} key={client}>
                    {client}
                  </option>
                ))}
              </select>
            </label>
            <label>
              <span>State</span>
              <select value={downloadStateFilter} onChange={(event) => setDownloadStateFilter(event.target.value as DownloadStateFilter)}>
                <option value="all">All states</option>
                <option value="active">Active</option>
                <option value="paused">Paused</option>
                <option value="complete">Complete</option>
                <option value="failed">Failed</option>
              </select>
            </label>
            <label>
              <span>Category</span>
              <input list="download-category-options" value={downloadCategoryFilter} onChange={(event) => setDownloadCategoryFilter(event.target.value)} placeholder="Any category" />
            </label>
            <label>
              <span>Max active</span>
              <input inputMode="numeric" min="1" max="25" value={downloadRebalanceMax} onChange={(event) => setDownloadRebalanceMax(event.target.value)} type="number" />
            </label>
            <datalist id="download-category-options">
              {downloadCategoryOptions.map((category) => (
                <option value={category} key={category} />
              ))}
            </datalist>
          </div>
          <div className="download-resource-panel" aria-label="Download client categories and tags">
            <div className="download-resource-editor">
              <label>
                <span>Client</span>
                <select value={downloadResourceClient} onChange={(event) => setDownloadResourceClient(event.target.value)}>
                  {downloadResourceClientOptions.map((client) => (
                    <option value={client} key={client}>
                      {client}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Category</span>
                <input list="download-category-options" value={resourceCategoryName} onChange={(event) => setResourceCategoryName(event.target.value)} placeholder="books-ebook" />
              </label>
              {resourceClientIsTransmission ? (
                <label>
                  <span>Rename to</span>
                  <input value={resourceCategoryNewName} onChange={(event) => setResourceCategoryNewName(event.target.value)} placeholder="books-audiobook" />
                </label>
              ) : null}
              <label>
                <span>Save path</span>
                <input disabled={resourceClientIsTransmission} value={resourceCategoryPath} onChange={(event) => setResourceCategoryPath(event.target.value)} placeholder="/data/torrents/books" />
              </label>
              <button
                className="secondary-action compact"
                disabled={!resourceClientConfigured || !resourceCategoryName.trim() || (resourceClientIsTransmission && !resourceCategoryNewName.trim()) || Boolean(downloadResourceActionID)}
                onClick={saveDownloadCategoryResource}
                type="button"
              >
                <FolderSearch size={16} />
                {resourceClientIsTransmission ? "Rename cat" : "Save cat"}
              </button>
              <button className="secondary-action compact danger-outline" disabled={!resourceClientConfigured || !resourceCategoryName.trim() || Boolean(downloadResourceActionID)} onClick={() => deleteDownloadCategoryResource()} type="button">
                <Trash2 size={16} />
                Delete cat
              </button>
              <button className="secondary-action compact" disabled={!resourceClientConfigured || isLoadingDownloadResources || Boolean(downloadResourceActionID)} onClick={() => refreshDownloadResources()} type="button">
                <RefreshCw size={16} />
                {isLoadingDownloadResources ? "Loading" : "Refresh map"}
              </button>
            </div>
            {!resourceClientIsSABnzbd ? (
              <div className="download-resource-editor tag-editor">
                <label>
                  <span>Tags</span>
                  <input value={resourceTagName} onChange={(event) => setResourceTagName(event.target.value)} placeholder="librarry, manual" />
                </label>
                {resourceClientIsTransmission ? (
                  <label>
                    <span>Rename to</span>
                    <input value={resourceTagNewName} onChange={(event) => setResourceTagNewName(event.target.value)} placeholder="librarry-ui" />
                  </label>
                ) : null}
                <button
                  className="secondary-action compact"
                  disabled={
                    !resourceClientConfigured ||
                    splitTagInput(resourceTagName).length === 0 ||
                    (resourceClientIsTransmission && (!resourceTagNewName.trim() || splitTagInput(resourceTagName).length !== 1)) ||
                    Boolean(downloadResourceActionID)
                  }
                  onClick={createDownloadTagResource}
                  type="button"
                >
                  <Tags size={16} />
                  {resourceClientIsTransmission ? "Rename tag" : "Create tag"}
                </button>
                <button className="secondary-action compact danger-outline" disabled={!resourceClientConfigured || splitTagInput(resourceTagName).length === 0 || Boolean(downloadResourceActionID)} onClick={() => deleteDownloadTagResource()} type="button">
                  <Trash2 size={16} />
                  Delete tag
                </button>
              </div>
            ) : null}
            <div className="download-resource-lists">
              <div className="download-resource-list" aria-label="Managed categories">
                {(downloadResources?.categories ?? []).slice(0, 8).map((category) => (
                  <article className="download-resource-row" key={category.name}>
                    <div>
                      <strong>{category.name}</strong>
                      <span>{category.savePath || "No save path"}</span>
                    </div>
                    <button
                      className="secondary-action compact"
                      onClick={() => {
                        setResourceCategoryName(category.name);
                        setResourceCategoryPath(category.savePath || "");
                        setDownloadCategoryFilter(category.name);
                      }}
                      type="button"
                    >
                      Use
                    </button>
                    <button className="icon-button danger" disabled={Boolean(downloadResourceActionID)} onClick={() => deleteDownloadCategoryResource(category.name)} type="button" aria-label={`Delete category ${category.name}`} title="Delete category">
                      <Trash2 size={16} />
                    </button>
                  </article>
                ))}
              </div>
              <div className="download-tag-cloud" aria-label="Managed tags">
                {(downloadResources?.tags ?? []).slice(0, 16).map((tag) => (
                  <button
                    className="download-tag-chip"
                    disabled={Boolean(downloadResourceActionID)}
                    key={tag}
                    onClick={() => {
                      setResourceTagName(tag);
                      setDownloadTextFilter(tag);
                    }}
                    type="button"
                  >
                    {tag}
                  </button>
                ))}
              </div>
            </div>
          </div>
          {resourceClientSupportsPreferences && resourceClientConfigured ? (
            <div className="download-preference-panel" aria-label={`${downloadResourceClient} preferences`}>
              <div className="download-preference-header">
                <strong>{downloadResourceClient}</strong>
                <span>
                  {downloadPreferences
                    ? `${formatLimitSpeed(downloadPreferences.downloadLimit)} down · ${formatLimitSpeed(downloadPreferences.uploadLimit)} up · ${downloadPreferences.maxActiveTorrents} active`
                    : isLoadingDownloadPreferences
                      ? "Loading preferences"
                      : "Preferences unavailable"}
                </span>
              </div>
              <div className="download-preference-grid">
                <label>
                  <span>Save path</span>
                  <input value={preferenceSavePath} onChange={(event) => setPreferenceSavePath(event.target.value)} placeholder="/data/torrents/books" />
                </label>
                <label>
                  <span>Temp path</span>
                  <input disabled={!preferenceTempPathEnabled} value={preferenceTempPath} onChange={(event) => setPreferenceTempPath(event.target.value)} placeholder="/data/torrents/incomplete" />
                </label>
                <label>
                  <span>Down KiB/s</span>
                  <input inputMode="numeric" min="0" value={preferenceDownloadLimitKiB} onChange={(event) => setPreferenceDownloadLimitKiB(event.target.value)} placeholder="unlimited" type="number" />
                </label>
                <label>
                  <span>Up KiB/s</span>
                  <input inputMode="numeric" min="0" value={preferenceUploadLimitKiB} onChange={(event) => setPreferenceUploadLimitKiB(event.target.value)} placeholder="unlimited" type="number" />
                </label>
                <label>
                  <span>Alt down</span>
                  <input inputMode="numeric" min="0" value={preferenceAltDownloadLimitKiB} onChange={(event) => setPreferenceAltDownloadLimitKiB(event.target.value)} placeholder="unlimited" type="number" />
                </label>
                <label>
                  <span>Alt up</span>
                  <input inputMode="numeric" min="0" value={preferenceAltUploadLimitKiB} onChange={(event) => setPreferenceAltUploadLimitKiB(event.target.value)} placeholder="unlimited" type="number" />
                </label>
                <label>
                  <span>Active DL</span>
                  <input inputMode="numeric" min="-1" value={preferenceMaxActiveDownloads} onChange={(event) => setPreferenceMaxActiveDownloads(event.target.value)} type="number" />
                </label>
                <label>
                  <span>Active seed</span>
                  <input inputMode="numeric" min="-1" value={preferenceMaxActiveUploads} onChange={(event) => setPreferenceMaxActiveUploads(event.target.value)} type="number" />
                </label>
                <label>
                  <span>Active total</span>
                  <input disabled={!resourceClientIsQbittorrent} inputMode="numeric" min="-1" value={preferenceMaxActiveTorrents} onChange={(event) => setPreferenceMaxActiveTorrents(event.target.value)} type="number" />
                </label>
              </div>
              <div className="download-preference-toggles">
                <label>
                  <input checked={preferenceStartPaused} onChange={(event) => setPreferenceStartPaused(event.target.checked)} type="checkbox" />
                  <span>Start paused</span>
                </label>
                <label>
                  <input checked={preferenceQueueingEnabled} onChange={(event) => setPreferenceQueueingEnabled(event.target.checked)} type="checkbox" />
                  <span>Queueing</span>
                </label>
                <label>
                  <input checked={preferenceTempPathEnabled} onChange={(event) => setPreferenceTempPathEnabled(event.target.checked)} type="checkbox" />
                  <span>Temp path</span>
                </label>
                <label>
                  <input checked={preferenceSpeedScheduleEnabled} onChange={(event) => setPreferenceSpeedScheduleEnabled(event.target.checked)} type="checkbox" />
                  <span>Schedule</span>
                </label>
                <button className="secondary-action compact" disabled={isLoadingDownloadPreferences || isSavingDownloadPreferences} onClick={() => refreshDownloadPreferences()} type="button">
                  <RefreshCw size={16} />
                  {isLoadingDownloadPreferences ? "Loading" : "Refresh prefs"}
                </button>
                <button className="secondary-action compact" disabled={isSavingDownloadPreferences || isLoadingDownloadPreferences} onClick={applyDownloadPreferences} type="button">
                  <SlidersHorizontal size={16} />
                  {isSavingDownloadPreferences ? "Saving" : "Save prefs"}
                </button>
              </div>
            </div>
          ) : null}
          {downloadError ? <div className="inline-error">{downloadError}</div> : null}
          <div className="manual-grab-panel" aria-label="Manual download add">
            <div className="manual-grab-main">
              <label>
                <span>Magnet, torrent, or NZB URL</span>
                <input
                  value={manualGrabURL}
                  onChange={(event) => setManualGrabURL(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") addManualDownload();
                  }}
                  placeholder="magnet:?xt=... or https://indexer.example/download/..."
                />
              </label>
              <label>
                <span>Display title</span>
                <input value={manualGrabTitle} onChange={(event) => setManualGrabTitle(event.target.value)} placeholder="Optional queue name" />
              </label>
              <label>
                <span>Torrent file</span>
                <input
                  key={manualGrabFileInputKey}
                  accept=".torrent,application/x-bittorrent"
                  onChange={(event) => {
                    const file = event.currentTarget.files?.[0] ?? null;
                    setManualGrabFile(file);
                    if (file && manualGrabClient === "SABnzbd") {
                      setManualGrabClient("");
                    }
                  }}
                  type="file"
                />
              </label>
            </div>
            <div className="manual-grab-options">
              <select value={manualGrabFormat} onChange={(event) => setManualGrabFormat(event.target.value)} aria-label="Download format">
                <option value="ebook">Ebook</option>
                <option value="audiobook">Audiobook</option>
              </select>
              <select value={manualGrabClient} onChange={(event) => setManualGrabClient(event.target.value)} aria-label="Download client">
                <option value="">Auto client</option>
                <option value="qBittorrent">qBittorrent</option>
                <option value="Transmission">Transmission</option>
                <option value="SABnzbd" disabled={Boolean(manualGrabFile)}>SABnzbd</option>
              </select>
              <button className="primary-action" disabled={(!manualGrabURL.trim() && !manualGrabFile) || isAddingDownload} onClick={addManualDownload} type="button">
                <Download size={17} />
                {isAddingDownload ? "Adding" : "Add paused"}
              </button>
            </div>
          </div>
          <div className="download-queue-strip" aria-label="Download queue summary">
            {[
              ["Active", downloadQueueStats.active],
              ["Paused", downloadQueueStats.paused],
              ["Complete", downloadQueueStats.complete],
              ["Failed", downloadQueueStats.failed],
              ["Selected", selectedDownloads.length]
            ].map(([label, value]) => (
              <div className="download-queue-metric" key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
          <div className="download-bulkbar" aria-label="Bulk download actions">
            <button className="secondary-action compact" disabled={filteredDownloads.length === 0} onClick={toggleAllDownloads} type="button">
              {allDownloadsSelected ? <CheckSquare size={16} /> : <Square size={16} />}
              {allDownloadsSelected ? "Clear all" : "Select all"}
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("start", selectedActionDownloadIDs)} type="button">
              <Play size={16} />
              Start
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("stop", selectedActionDownloadIDs)} type="button">
              <Pause size={16} />
              Stop
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || !selectedDownloadsSupportRecheck || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("recheck", selectedActionDownloadIDs)} type="button">
              <RefreshCw size={16} />
              Recheck
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || !selectedDownloadsSupportPriority || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("topPriority", selectedActionDownloadIDs)} type="button">
              <ChevronsUp size={16} />
              Top
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || !selectedDownloadsSupportPriority || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("decreasePriority", selectedActionDownloadIDs)} type="button">
              <ChevronsDown size={16} />
              Lower
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || !selectedDownloadsSupportPriority || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("bottomPriority", selectedActionDownloadIDs)} type="button">
              <ChevronsDown size={16} />
              Bottom
            </button>
            <button className="secondary-action compact" disabled={!selectedDownloadsSupportForceStart || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("forceStart", selectedActionDownloadIDs, false, { forceStart: true })} type="button">
              <Play size={16} />
              Force
            </button>
            <button className="secondary-action compact" disabled={!selectedDownloadsSupportQbitControls || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("toggleSequential", selectedActionDownloadIDs)} type="button">
              <SlidersHorizontal size={16} />
              Seq
            </button>
            <button className="secondary-action compact" disabled={!selectedDownloadsSupportQbitControls || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("toggleFirstLastPiece", selectedActionDownloadIDs)} type="button">
              <FileCheck2 size={16} />
              Edges
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || isImportingCompleted} onClick={() => runCompletedImport(selectedDownloads)} type="button">
              <UploadCloud size={16} />
              Import
            </button>
            <button className="secondary-action compact" disabled={selectedDownloads.length === 0 || isRecoveringFailed} onClick={() => runFailedRecovery(selectedDownloads, { autoGrab: true, force: true })} type="button">
              <HardDriveDownload size={16} />
              Recover
            </button>
            <button className="secondary-action compact danger-outline" disabled={selectedDownloads.length === 0 || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("delete", selectedActionDownloadIDs, false)} type="button">
              <Trash2 size={16} />
              Remove
            </button>
            <button className="secondary-action compact danger-outline" disabled={selectedDownloads.length === 0 || Boolean(downloadActionID)} onClick={() => applyDownloadActionToIDs("delete", selectedActionDownloadIDs, true)} type="button">
              <Trash2 size={16} />
              Delete data
            </button>
          </div>
          {failedDownloadRun ? (
            <div className="failed-download-results">
              <div className="failed-download-summary">
                <strong>{failedDownloadRun.status}</strong>
                <span>
                  {failedDownloadRun.downloadsChecked} checked · {failedDownloadRun.failedCount} failed · {failedDownloadRun.replacementsFound} replacements · {failedDownloadRun.grabbedCount} grabbed · {failedDownloadRun.removedCount} removed
                </span>
              </div>
              {failedDownloadRun.items?.slice(0, 6).map((item) => (
                <article className={item.error ? "failed-download-row error" : "failed-download-row"} key={`${item.download.id}-${item.failureReason}`}>
                  <div>
                    <strong>{item.download.name || item.download.id}</strong>
                    <span>
                      {item.failureReason}
                      {item.replacementRelease ? ` · replacement ${item.replacementRelease.title}` : ""}
                    </span>
                  </div>
                  {item.replacementDownload ? <em>Queued</em> : item.error ? <em>Error</em> : null}
                </article>
              ))}
            </div>
          ) : null}
          {queueRebalancePlan ? (
            <div className="queue-rebalance-result">
              <strong>{queueRebalancePlan.applied ? "Queue balanced" : "Queue plan"}</strong>
              <span>
                active {queueRebalancePlan.activeCount}/{queueRebalancePlan.maxActive} · start {queueRebalancePlan.startIds.length} · stop {queueRebalancePlan.stopIds.length}
              </span>
            </div>
          ) : null}
          {downloadDetails || isLoadingDownloadDetails ? (
            <div className="download-detail-panel">
              <div className="download-detail-header">
                <div>
                  <strong>{downloadDetails?.status.name || "Loading download details"}</strong>
                  <span>
                    {downloadDetails
                      ? `${downloadDetails.status.client || "qBittorrent"} · ${downloadDetails.status.category || "uncategorized"} · ${formatBytes(downloadDetails.status.sizeBytes ?? downloadDetails.properties?.totalSizeBytes ?? 0)}`
                      : "Fetching download state"}
                  </span>
                </div>
                {downloadDetails ? (
                  <button className="secondary-action compact" disabled={isLoadingDownloadDetails} onClick={() => openDownloadDetails(downloadDetails.status)} type="button">
                    <RefreshCw size={16} />
                    {isLoadingDownloadDetails ? "Loading" : "Refresh details"}
                  </button>
                ) : null}
              </div>
              {downloadDetails ? (
                <>
                  <div className="download-detail-metrics">
                    {[
                      ["Progress", `${Math.round((downloadDetails.status.progress ?? 0) * 100)}%`],
                      ["ETA", formatDuration(downloadDetails.properties?.etaSeconds ?? downloadDetails.status.etaSeconds)],
                      ["Speed", `${formatSpeed(downloadDetails.properties?.downloadSpeed ?? downloadDetails.status.downloadRate ?? 0)} down`],
                      ["Peers", `${downloadDetails.status.seeders ?? 0} seeders · ${downloadDetails.status.peers ?? 0} peers`],
                      ["Pieces", `${downloadDetails.properties?.piecesHave ?? 0}/${downloadDetails.properties?.piecesTotal ?? 0}`],
                      ["Connections", `${downloadDetails.properties?.connections ?? 0}/${downloadDetails.properties?.connectionsLimit ?? 0}`]
                    ].map(([label, value]) => (
                      <div className="download-detail-metric" key={label}>
                        <span>{label}</span>
                        <strong>{value}</strong>
                      </div>
                    ))}
                  </div>
                  <div className="download-action-tools">
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID)} onClick={() => applyDownloadAction("start", downloadDetails.status)} type="button">
                      <Play size={16} />
                      Start
                    </button>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID)} onClick={() => applyDownloadAction("stop", downloadDetails.status)} type="button">
                      <Pause size={16} />
                      Stop
                    </button>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !supportsDownloadAction(downloadDetails.status, "recheck")} onClick={() => applyDownloadAction("recheck", downloadDetails.status)} type="button">
                      <RefreshCw size={16} />
                      Recheck
                    </button>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !supportsDownloadAction(downloadDetails.status, "increasePriority")} onClick={() => applyDownloadAction("increasePriority", downloadDetails.status)} type="button">
                      <ChevronsUp size={16} />
                      Raise
                    </button>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !supportsDownloadAction(downloadDetails.status, "decreasePriority")} onClick={() => applyDownloadAction("decreasePriority", downloadDetails.status)} type="button">
                      <ChevronsDown size={16} />
                      Lower
                    </button>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !supportsDownloadAction(downloadDetails.status, "forceStart")} onClick={() => applyDownloadActionToIDs("forceStart", [downloadDetails.status.id], false, { forceStart: true })} type="button">
                      <Play size={16} />
                      Force
                    </button>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !supportsDownloadAction(downloadDetails.status, "toggleSequential")} onClick={() => applyDownloadAction("toggleSequential", downloadDetails.status)} type="button">
                      <SlidersHorizontal size={16} />
                      Seq
                    </button>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !supportsDownloadAction(downloadDetails.status, "toggleFirstLastPiece")} onClick={() => applyDownloadAction("toggleFirstLastPiece", downloadDetails.status)} type="button">
                      <FileCheck2 size={16} />
                      Edges
                    </button>
                  </div>
                  <div className="download-management-tools">
                    <label>
                      <span>Name</span>
                      <input value={downloadNameInput} onChange={(event) => setDownloadNameInput(event.target.value)} placeholder="Torrent display name" />
                    </label>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !downloadNameInput.trim() || !supportsDownloadAction(downloadDetails.status, "rename")} onClick={applyDownloadRename} type="button">
                      <Pencil size={16} />
                      Rename
                    </button>
                    <label>
                      <span>Tags</span>
                      <input value={downloadTagsInput} onChange={(event) => setDownloadTagsInput(event.target.value)} placeholder="librarry, audiobook" />
                    </label>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || splitTagInput(downloadTagsInput).length === 0 || !supportsDownloadAction(downloadDetails.status, "addTags")} onClick={() => applyDownloadTags("addTags")} type="button">
                      <Tags size={16} />
                      Add tags
                    </button>
                    <button className="secondary-action compact danger-outline" disabled={Boolean(downloadActionID) || splitTagInput(downloadTagsInput).length === 0 || !supportsDownloadAction(downloadDetails.status, "removeTags")} onClick={() => applyDownloadTags("removeTags")} type="button">
                      <Tags size={16} />
                      Remove tags
                    </button>
                    <label>
                      <span>Category</span>
                      <input value={downloadCategoryInput} onChange={(event) => setDownloadCategoryInput(event.target.value)} placeholder="books-ebook" />
                    </label>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !downloadCategoryInput.trim() || !supportsDownloadAction(downloadDetails.status, "setCategory")} onClick={applyDownloadCategory} type="button">
                      Set category
                    </button>
                    <label>
                      <span>Save path</span>
                      <input value={downloadSavePathInput} onChange={(event) => setDownloadSavePathInput(event.target.value)} placeholder="/data/torrents/books" />
                    </label>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || !downloadSavePathInput.trim() || !supportsDownloadAction(downloadDetails.status, "setLocation")} onClick={applyDownloadLocation} type="button">
                      Move
                    </button>
                    <button className="secondary-action compact danger-outline" disabled={Boolean(downloadActionID)} onClick={() => applyDownloadAction("delete", downloadDetails.status, true)} type="button">
                      Delete data
                    </button>
                  </div>
                  <div className="download-bandwidth-tools">
                    <label>
                      <span>Download KiB/s</span>
                      <input inputMode="numeric" min="0" value={downloadLimitKiB} onChange={(event) => setDownloadLimitKiB(event.target.value)} placeholder="unlimited" type="number" />
                    </label>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || bandwidthInputToBytes(downloadLimitKiB) < 0 || !supportsDownloadAction(downloadDetails.status, "setDownloadLimit")} onClick={() => applyDownloadBandwidthLimit("download")} type="button">
                      Set down
                    </button>
                    <label>
                      <span>Upload KiB/s</span>
                      <input inputMode="numeric" min="0" value={uploadLimitKiB} onChange={(event) => setUploadLimitKiB(event.target.value)} placeholder="unlimited" type="number" />
                    </label>
                    <button className="secondary-action compact" disabled={Boolean(downloadActionID) || bandwidthInputToBytes(uploadLimitKiB) < 0 || !supportsDownloadAction(downloadDetails.status, "setUploadLimit")} onClick={() => applyDownloadBandwidthLimit("upload")} type="button">
                      Set up
                    </button>
                  </div>
                  <div className="download-tracker-tools">
                    <input value={trackerURL} onChange={(event) => setTrackerURL(event.target.value)} placeholder="https://tracker.example/announce" />
                    <button className="secondary-action compact" disabled={!downloadSupportsTrackerActions(downloadDetails.status) || !trackerURL.trim() || Boolean(downloadActionID)} onClick={() => applyDownloadTrackerAction("add")} type="button">
                      Add tracker
                    </button>
                  </div>
                  {downloadDetails.trackers?.length ? (
                    <div className="download-tracker-list">
                      {downloadDetails.trackers.map((tracker) => {
                        const trackerBusy = downloadActionID.startsWith(`${downloadDetails.status.id}:tracker:`);
                        return (
                          <article className="download-tracker-row" key={`${tracker.url}-${tracker.tier ?? 0}`}>
                            <div>
                              <strong>{tracker.url || "tracker"}</strong>
                              <span>{tracker.message || tracker.status}</span>
                            </div>
                            <span className={`download-badge ${tracker.status === "working" ? "seeding" : tracker.status === "not_working" ? "error" : "idle"}`}>
                              {tracker.status}
                            </span>
                            <em>
                              {tracker.seeds ?? 0} seeds · {tracker.leeches ?? 0} leeches
                            </em>
                            <div className="tracker-action-buttons">
                              <button className="secondary-action compact" disabled={!downloadSupportsTrackerActions(downloadDetails.status) || trackerBusy || !trackerURL.trim()} onClick={() => applyDownloadTrackerAction("edit", tracker.url)} type="button">
                                Replace
                              </button>
                              <button className="secondary-action compact danger-outline" disabled={!downloadSupportsTrackerActions(downloadDetails.status) || trackerBusy} onClick={() => applyDownloadTrackerAction("remove", tracker.url)} type="button">
                                Remove
                              </button>
                            </div>
                          </article>
                        );
                      })}
                    </div>
                  ) : null}
                  {downloadDetails.peers?.length ? (
                    <div className="download-peer-list">
                      {downloadDetails.peers.slice(0, 20).map((peer) => (
                        <article className="download-peer-row" key={peer.id || `${peer.ip}:${peer.port ?? 0}`}>
                          <div>
                            <strong>
                              {peer.ip}
                              {peer.port ? `:${peer.port}` : ""}
                            </strong>
                            <span>{peer.client || peer.connection || peer.country || "peer"}</span>
                          </div>
                          <span className="download-badge idle">{Math.round((peer.progress ?? 0) * 100)}%</span>
                          <em>
                            {formatSpeed(peer.downloadRate ?? 0)} down · {formatSpeed(peer.uploadRate ?? 0)} up
                          </em>
                        </article>
                      ))}
                    </div>
                  ) : null}
                  {downloadDetails.files?.length ? (
                    <div className="download-file-list">
                      {downloadDetails.files.map((file) => {
                        const fileBusy = downloadActionID.startsWith(`${downloadDetails.status.id}:file:`);
                        return (
                          <article className="download-file-row" key={`${downloadDetails.status.id}:${file.id}`}>
                            <div className="download-file-main">
                              <strong>{file.name}</strong>
                              <span>
                                {formatBytes(file.sizeBytes ?? 0)} · {Math.round((file.progress ?? 0) * 100)}% · {filePriorityLabel(file.priority)}
                                {file.availability ? ` · ${file.availability.toFixed(1)} available` : ""}
                              </span>
                              <div className="progress-track" aria-label={`File progress ${Math.round((file.progress ?? 0) * 100)} percent`}>
                                <span style={{ width: `${Math.max(0, Math.min(100, (file.progress ?? 0) * 100))}%` }} />
                              </div>
                            </div>
                            <div className="file-action-buttons">
                              <button className="secondary-action compact" disabled={fileBusy || !downloadSupportsFileActions(downloadDetails.status)} onClick={() => applyDownloadFileAction("skip", [file.id])} type="button">
                                Skip
                              </button>
                              <button className="secondary-action compact" disabled={fileBusy || !downloadSupportsFileActions(downloadDetails.status)} onClick={() => applyDownloadFileAction("normal", [file.id])} type="button">
                                Normal
                              </button>
                              <button className="secondary-action compact" disabled={fileBusy || !downloadSupportsFileActions(downloadDetails.status)} onClick={() => applyDownloadFileAction("high", [file.id])} type="button">
                                High
                              </button>
                              <button className="secondary-action compact" disabled={fileBusy || !downloadSupportsFileActions(downloadDetails.status)} onClick={() => applyDownloadFileAction("max", [file.id])} type="button">
                                Max
                              </button>
                            </div>
                          </article>
                        );
                      })}
                    </div>
                  ) : null}
                </>
              ) : (
                <div className="empty-detail">Loading torrent details</div>
              )}
            </div>
          ) : null}
          <div className="download-list">
            {filteredDownloads.length === 0 ? (
              <div className="download-empty">
                <strong>{downloads.length === 0 ? "No downloads returned from configured clients." : "No downloads match the current filters."}</strong>
                <span>
                  {downloads.length === 0
                    ? "Manual adds, wanted grabs, feed grabs, and completed-download imports will appear here after a client accepts work."
                    : "Clear filters or switch scope to review the full acquisition queue."}
                </span>
                <div>
                  <button className="secondary-action compact" onClick={refreshDownloads} type="button">
                    <RefreshCw size={16} />
                    Refresh
                  </button>
                  {downloads.length === 0 ? (
                    <button className="secondary-action compact" onClick={() => setActiveView("settings")} type="button">
                      <Settings size={16} />
                      Client settings
                    </button>
                  ) : (
                    <button
                      className="secondary-action compact"
                      onClick={() => {
                        setDownloadTextFilter("");
                        setDownloadClientFilter("");
                        setDownloadCategoryFilter("");
                        setDownloadStateFilter("all");
                        changeDownloadScope("all");
                      }}
                      type="button"
                    >
                      <FilterX size={16} />
                      Clear filters
                    </button>
                  )}
                </div>
              </div>
            ) : filteredDownloads.map((download) => {
              const selectedDownload = selectedDownloadKeySet.has(downloadKey(download));
              const busy = downloadActionID.startsWith(`${download.id}:`) || (downloadActionID.startsWith("bulk:") && selectedDownload);
              return (
                <article className={selectedDownload ? "download-row selected" : "download-row"} key={downloadKey(download)}>
                  <label className="download-select" title="Select download">
                    <input checked={selectedDownload} onChange={() => toggleDownloadSelection(download)} type="checkbox" aria-label={`Select ${download.name || download.id}`} />
                  </label>
                  <div className="download-main">
                    <div className="download-title-line">
                      <strong>{download.name || download.id}</strong>
                      <span className={`download-badge ${stateTone(download.state)}`}>{download.state}</span>
                    </div>
                    <div className="download-meta">
                      <span>{download.client || "qBittorrent"}</span>
                      <span>{download.category || "uncategorized"}</span>
                      <span>{formatBytes(download.downloadedBytes ?? 0)} / {formatBytes(download.sizeBytes ?? 0)}</span>
                      <span>{formatSpeed(download.downloadRate ?? 0)} down</span>
                      <span>{formatSpeed(download.uploadRate ?? 0)} up</span>
                      <span>ratio {(download.ratio ?? 0).toFixed(2)}</span>
                      <span>{download.seeders ?? 0} seeders</span>
                      <span>import {download.importStatus || "pending"}</span>
                      {download.retryCount ? <span>{download.retryCount} retries</span> : null}
                    </div>
                    {download.importError ? <div className="download-import-error">{download.importError}</div> : null}
                    {download.failureReason ? <div className="download-import-error">{download.failureReason}</div> : null}
                    <div className="progress-track" aria-label={`Download progress ${Math.round((download.progress ?? 0) * 100)} percent`}>
                      <span style={{ width: `${Math.max(0, Math.min(100, (download.progress ?? 0) * 100))}%` }} />
                    </div>
                    <div className="download-path">{download.savePath}</div>
                  </div>
                  <div className="download-actions">
                    <button className="icon-button" disabled={busy} onClick={() => applyDownloadAction("start", download)} type="button" aria-label="Start download" title="Start">
                      <Play size={16} />
                    </button>
                    <button className="icon-button" disabled={busy} onClick={() => applyDownloadAction("stop", download)} type="button" aria-label="Stop download" title="Stop">
                      <Pause size={16} />
                    </button>
                    <button className="icon-button" disabled={busy || !supportsDownloadAction(download, "recheck")} onClick={() => applyDownloadAction("recheck", download)} type="button" aria-label="Recheck download" title="Recheck">
                      <RefreshCw size={16} />
                    </button>
                    <button className="icon-button" disabled={busy || !supportsDownloadAction(download, "increasePriority")} onClick={() => applyDownloadAction("increasePriority", download)} type="button" aria-label="Increase priority" title="Increase priority">
                      <span className="priority-glyph">+</span>
                    </button>
                    <button className="icon-button" disabled={busy || !downloadSupportsDetails(download)} onClick={() => openDownloadDetails(download)} type="button" aria-label="Download details" title="Details">
                      <FileSearch size={16} />
                    </button>
                    <button className="icon-button" disabled={busy || isImportingCompleted} onClick={() => runCompletedImport(download)} type="button" aria-label="Import completed download" title="Import completed">
                      <UploadCloud size={16} />
                    </button>
                    <button className="icon-button" disabled={busy || isRecoveringFailed} onClick={() => runFailedRecovery(download, { autoGrab: true, force: true })} type="button" aria-label="Retry failed download" title="Retry failed download">
                      <HardDriveDownload size={16} />
                    </button>
                    <button className="icon-button danger" disabled={busy} onClick={() => applyDownloadAction("delete", download, false)} type="button" aria-label="Remove download" title="Remove without deleting files">
                      <Trash2 size={16} />
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        </section>

        <section className="settings-panel" aria-label="Settings" hidden={activeView !== "settings"}>
          <div className="panel-heading">
            <div>
              <h2>Settings</h2>
              <p>{qualityProfiles.length} release policy profiles used by search, feeds, recovery, and upgrades.</p>
            </div>
            <button className="secondary-action compact" onClick={() => fetchQualityProfiles().then(setQualityProfiles).catch((error) => setSettingsError(error instanceof Error ? error.message : "Quality profiles refresh failed"))} type="button">
              <RefreshCw size={16} />
              Refresh
            </button>
          </div>
          <div className="settings-auth-row">
            <div className="settings-auth-title">
              <strong>API key</strong>
              <span>{getStoredAPIKey() ? "Saved in this browser" : "Not saved"}</span>
            </div>
            <label className="settings-api-key">
              <span>Key</span>
              <input
                autoComplete="off"
                type="password"
                value={apiKeyInput}
                onChange={(event) => setAPIKeyInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") saveAPIKeySetting();
                }}
                placeholder="Readarr-compatible API key"
              />
            </label>
            <button className="secondary-action compact" onClick={saveAPIKeySetting} type="button">
              <CheckCircle2 size={16} />
              Save key
            </button>
            <button
              className="secondary-action compact danger-outline"
              onClick={() => {
                setAPIKeyInput("");
                setStoredAPIKey("");
                setSettingsError("");
                setSettingsNotice("API key cleared for this browser.");
              }}
              type="button"
            >
              <Trash2 size={16} />
              Clear
            </button>
          </div>
          {settingsNotice ? <div className="inline-note">{settingsNotice}</div> : null}
          {settingsError ? <div className={isPersistenceRequiredError(settingsError) ? "inline-note" : "inline-error"}>{appErrorMessage(settingsError)}</div> : null}
          <div className="integration-settings-panel library-settings-panel">
            <div className="integration-settings-header">
              <div>
                <strong>Library roots and naming</strong>
                <span>
                  {librarySettingsPersisted ? "Postgres" : "runtime"} · ebooks {librarySettings.ebookLibraryRoot || "unset"} · audio{" "}
                  {librarySettings.audiobookLibraryRoot || "unset"}
                </span>
              </div>
              <div className="integration-settings-actions">
                <button
                  className="secondary-action compact"
                  disabled={isSavingLibrarySettings}
                  onClick={() =>
                    fetchLibrarySettings()
                      .then((response) => {
                        setLibrarySettings(response.settings);
                        setLibrarySettingsForm(response.settings);
                        setLibrarySettingsPersisted(response.persisted);
                      })
                      .catch((error) => setSettingsError(error instanceof Error ? error.message : "Library settings refresh failed"))
                  }
                  type="button"
                >
                  <RefreshCw size={16} />
                  Refresh
                </button>
                <button
                  className="primary-action compact"
                  disabled={
                    isSavingLibrarySettings ||
                    !librarySettingsForm.ebookLibraryRoot.trim() ||
                    !librarySettingsForm.audiobookLibraryRoot.trim() ||
                    !librarySettingsForm.namingAuthorFolder.trim() ||
                    !librarySettingsForm.namingBookFolder.trim() ||
                    !librarySettingsForm.namingFileName.trim()
                  }
                  onClick={persistLibrarySettings}
                  type="button"
                >
                  <CheckCircle2 size={16} />
                  {isSavingLibrarySettings ? "Saving" : "Save library"}
                </button>
              </div>
            </div>
            <div className="integration-settings-grid library-settings-grid">
              <label className="wide">
                <span>Ebook library root</span>
                <input value={librarySettingsForm.ebookLibraryRoot} onChange={(event) => updateLibrarySettingsForm({ ebookLibraryRoot: event.target.value })} placeholder="/data/media/books/ebooks" />
              </label>
              <label className="wide">
                <span>Audiobook library root</span>
                <input value={librarySettingsForm.audiobookLibraryRoot} onChange={(event) => updateLibrarySettingsForm({ audiobookLibraryRoot: event.target.value })} placeholder="/data/media/books/audiobooks" />
              </label>
              <label>
                <span>Author folder</span>
                <input value={librarySettingsForm.namingAuthorFolder} onChange={(event) => updateLibrarySettingsForm({ namingAuthorFolder: event.target.value })} placeholder="{Author}" />
              </label>
              <label>
                <span>Book folder</span>
                <input value={librarySettingsForm.namingBookFolder} onChange={(event) => updateLibrarySettingsForm({ namingBookFolder: event.target.value })} placeholder="{Title}" />
              </label>
              <label>
                <span>File name</span>
                <input value={librarySettingsForm.namingFileName} onChange={(event) => updateLibrarySettingsForm({ namingFileName: event.target.value })} placeholder="{Title}{Ext}" />
              </label>
              <label>
                <span>Space replacement</span>
                <input value={librarySettingsForm.namingSpaceReplacement} onChange={(event) => updateLibrarySettingsForm({ namingSpaceReplacement: event.target.value })} placeholder="Optional" />
              </label>
              <div className="library-naming-preview">
                <span>Preview</span>
                <strong title={libraryNamingPreview}>{libraryNamingPreview}</strong>
              </div>
            </div>
          </div>
          <div className="integration-settings-panel readarr-import-panel">
            <div className="integration-settings-header">
              <div>
                <strong>Readarr migration</strong>
                <span>
                  {readarrImportOutcome
                    ? `${readarrImportCount(readarrImportOutcome)} records found · ${readarrImportImported(readarrImportOutcome)} written`
                    : "Preview and import an existing Readarr instance"}
                </span>
              </div>
              <div className="integration-settings-actions">
                <button
                  className="secondary-action compact"
                  disabled={!readarrImportForm.baseUrl.trim() || isPreviewingReadarrImport || isRunningReadarrImport}
                  onClick={previewExistingReadarrImport}
                  type="button"
                >
                  <Search size={16} />
                  {isPreviewingReadarrImport ? "Previewing" : "Preview"}
                </button>
                <button
                  className="primary-action compact"
                  disabled={!readarrImportForm.baseUrl.trim() || isPreviewingReadarrImport || isRunningReadarrImport}
                  onClick={applyExistingReadarrImport}
                  type="button"
                >
                  <UploadCloud size={16} />
                  {isRunningReadarrImport ? "Importing" : "Import"}
                </button>
              </div>
            </div>
            <div className="integration-settings-grid readarr-import-grid">
              <label className="wide">
                <span>Readarr URL</span>
                <input value={readarrImportForm.baseUrl} onChange={(event) => updateReadarrImportForm({ baseUrl: event.target.value })} placeholder="http://readarr:8787" />
              </label>
              <label>
                <span>Readarr API key</span>
                <input autoComplete="off" type="password" value={readarrImportForm.apiKey} onChange={(event) => updateReadarrImportForm({ apiKey: event.target.value })} placeholder="API key" />
              </label>
              <div className="readarr-import-options wide">
                <label>
                  <input checked={readarrImportForm.importAuthors} onChange={(event) => updateReadarrImportForm({ importAuthors: event.target.checked })} type="checkbox" />
                  <span>Authors</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importBooks} onChange={(event) => updateReadarrImportForm({ importBooks: event.target.checked })} type="checkbox" />
                  <span>Books</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importQualityProfiles} onChange={(event) => updateReadarrImportForm({ importQualityProfiles: event.target.checked })} type="checkbox" />
                  <span>Quality profiles</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importRootFolders} onChange={(event) => updateReadarrImportForm({ importRootFolders: event.target.checked })} type="checkbox" />
                  <span>Root folders</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importBookFiles} onChange={(event) => updateReadarrImportForm({ importBookFiles: event.target.checked })} type="checkbox" />
                  <span>Book files</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importTags} onChange={(event) => updateReadarrImportForm({ importTags: event.target.checked })} type="checkbox" />
                  <span>Tags</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importLists} onChange={(event) => updateReadarrImportForm({ importLists: event.target.checked })} type="checkbox" />
                  <span>Import lists</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importListExclusions} onChange={(event) => updateReadarrImportForm({ importListExclusions: event.target.checked })} type="checkbox" />
                  <span>List exclusions</span>
                </label>
                <label>
                  <input checked={readarrImportForm.importConfigResources} onChange={(event) => updateReadarrImportForm({ importConfigResources: event.target.checked })} type="checkbox" />
                  <span>Config resources</span>
                </label>
              </div>
            </div>
            {readarrImportOutcome ? (
              <div className="readarr-import-results">
                {readarrImportOutcome.sections.map((section) => (
                  <div className="readarr-import-result" key={section.name}>
                    <div className="readarr-import-result-heading">
                      <strong>{readarrImportSectionLabel(section.name)}</strong>
                      <span>
                        {section.count} found · {section.imported} imported · {section.skipped} skipped
                      </span>
                    </div>
                    {(section.items ?? []).length ? (
                      <div className="readarr-import-items">
                        {(section.items ?? []).slice(0, 6).map((item, index) => (
                          <span key={`${section.name}-${item.id ?? index}`} title={readarrImportItemLabel(item)}>
                            {readarrImportItemLabel(item)}
                          </span>
                        ))}
                      </div>
                    ) : null}
                    {section.errors?.length ? <div className="inline-error detail-error">{section.errors.slice(0, 2).join("; ")}</div> : null}
                  </div>
                ))}
              </div>
            ) : null}
          </div>
          <div className="integration-settings-panel">
            <div className="integration-settings-header">
              <div>
                <strong>Indexer and download clients</strong>
                <span>
                  {integrationSettingsPersisted ? "Postgres" : "runtime"} · {integrationSettings.prowlarrUrl ? "indexer set" : "no indexer"} ·{" "}
                  {integrationSettings.qbittorrentUrl || integrationSettings.transmissionUrl || integrationSettings.sabnzbdUrl ? "client set" : "no client"}
                </span>
              </div>
              <div className="integration-settings-actions">
                <button
                  className="secondary-action compact"
                  disabled={isSavingIntegrationSettings}
                  onClick={() =>
                    fetchIntegrationSettings()
                      .then((response) => {
                        setIntegrationSettings(response.settings);
                        setIntegrationForm(integrationSettingsForm(response.settings));
                        setIntegrationSettingsPersisted(response.persisted);
                      })
                      .catch((error) => setSettingsError(error instanceof Error ? error.message : "Integration settings refresh failed"))
                  }
                  type="button"
                >
                  <RefreshCw size={16} />
                  Refresh
                </button>
                <button className="primary-action compact" disabled={isSavingIntegrationSettings} onClick={persistIntegrationSettings} type="button">
                  <CheckCircle2 size={16} />
                  {isSavingIntegrationSettings ? "Saving" : "Save integrations"}
                </button>
              </div>
            </div>
            <div className="integration-settings-grid">
              <label className="wide">
                <span>Prowlarr URL</span>
                <input value={integrationForm.prowlarrUrl} onChange={(event) => updateIntegrationForm({ prowlarrUrl: event.target.value })} placeholder="http://prowlarr:9696" />
              </label>
              <label>
                <span>Prowlarr API key {integrationSettings.prowlarrApiKeyConfigured ? "saved" : ""}</span>
                <input autoComplete="off" type="password" value={integrationForm.prowlarrApiKey ?? ""} onChange={(event) => updateIntegrationForm({ prowlarrApiKey: event.target.value })} placeholder={integrationSettings.prowlarrApiKeyConfigured ? "Leave blank to keep" : "API key"} />
              </label>
              <label className="wide">
                <span>qBittorrent URL</span>
                <input value={integrationForm.qbittorrentUrl} onChange={(event) => updateIntegrationForm({ qbittorrentUrl: event.target.value })} placeholder="http://qbittorrent:8080" />
              </label>
              <label>
                <span>qBittorrent user</span>
                <input value={integrationForm.qbittorrentUsername} onChange={(event) => updateIntegrationForm({ qbittorrentUsername: event.target.value })} placeholder="admin" />
              </label>
              <label>
                <span>qBittorrent password {integrationSettings.qbittorrentPasswordConfigured ? "saved" : ""}</span>
                <input autoComplete="off" type="password" value={integrationForm.qbittorrentPassword ?? ""} onChange={(event) => updateIntegrationForm({ qbittorrentPassword: event.target.value })} placeholder={integrationSettings.qbittorrentPasswordConfigured ? "Leave blank to keep" : "Password"} />
              </label>
              <label className="wide">
                <span>Transmission URL</span>
                <input value={integrationForm.transmissionUrl} onChange={(event) => updateIntegrationForm({ transmissionUrl: event.target.value })} placeholder="http://transmission:9091" />
              </label>
              <label>
                <span>Transmission user</span>
                <input value={integrationForm.transmissionUsername} onChange={(event) => updateIntegrationForm({ transmissionUsername: event.target.value })} placeholder="Optional" />
              </label>
              <label>
                <span>Transmission password {integrationSettings.transmissionPasswordConfigured ? "saved" : ""}</span>
                <input autoComplete="off" type="password" value={integrationForm.transmissionPassword ?? ""} onChange={(event) => updateIntegrationForm({ transmissionPassword: event.target.value })} placeholder={integrationSettings.transmissionPasswordConfigured ? "Leave blank to keep" : "Password"} />
              </label>
              <label className="wide">
                <span>SABnzbd URL</span>
                <input value={integrationForm.sabnzbdUrl} onChange={(event) => updateIntegrationForm({ sabnzbdUrl: event.target.value })} placeholder="http://sabnzbd:8080" />
              </label>
              <label>
                <span>SABnzbd API key {integrationSettings.sabnzbdApiKeyConfigured ? "saved" : ""}</span>
                <input autoComplete="off" type="password" value={integrationForm.sabnzbdApiKey ?? ""} onChange={(event) => updateIntegrationForm({ sabnzbdApiKey: event.target.value })} placeholder={integrationSettings.sabnzbdApiKeyConfigured ? "Leave blank to keep" : "API key"} />
              </label>
              <label>
                <span>SABnzbd user</span>
                <input value={integrationForm.sabnzbdUsername} onChange={(event) => updateIntegrationForm({ sabnzbdUsername: event.target.value })} placeholder="Optional" />
              </label>
              <label>
                <span>SABnzbd password {integrationSettings.sabnzbdPasswordConfigured ? "saved" : ""}</span>
                <input autoComplete="off" type="password" value={integrationForm.sabnzbdPassword ?? ""} onChange={(event) => updateIntegrationForm({ sabnzbdPassword: event.target.value })} placeholder={integrationSettings.sabnzbdPasswordConfigured ? "Leave blank to keep" : "Password"} />
              </label>
              <label>
                <span>Ebook category</span>
                <input value={integrationForm.ebookCategory} onChange={(event) => updateIntegrationForm({ ebookCategory: event.target.value })} placeholder="books-ebook" />
              </label>
              <label>
                <span>Audiobook category</span>
                <input value={integrationForm.audiobookCategory} onChange={(event) => updateIntegrationForm({ audiobookCategory: event.target.value })} placeholder="books-audiobook" />
              </label>
              <label className="wide">
                <span>Torrent root</span>
                <input value={integrationForm.bookTorrentRoot} onChange={(event) => updateIntegrationForm({ bookTorrentRoot: event.target.value })} placeholder="/data/torrents/books" />
              </label>
            </div>
          </div>
          <div className="quality-profile-list">
            {qualityProfiles.map((profile) => {
              const key = profileKey(profile);
              return (
                <article className="quality-profile-row" key={key}>
                  <div className="quality-profile-title">
                    <strong>{profile.name}</strong>
                    <span>{profile.mediaFormat}</span>
                  </div>
                  <label>
                    <span>Min</span>
                    <input
                      inputMode="decimal"
                      value={profile.minScore}
                      onChange={(event) => updateQualityProfile(profile, { minScore: Number(event.target.value) || 0 })}
                    />
                  </label>
                  <label>
                    <span>Cutoff</span>
                    <input
                      inputMode="decimal"
                      value={profile.cutoffScore}
                      onChange={(event) => updateQualityProfile(profile, { cutoffScore: Number(event.target.value) || 0 })}
                    />
                  </label>
                  <label>
                    <span>Seeds</span>
                    <input
                      inputMode="numeric"
                      value={profile.minSeeders}
                      onChange={(event) => updateQualityProfile(profile, { minSeeders: Number(event.target.value) || 0 })}
                    />
                  </label>
                  <label>
                    <span>GB</span>
                    <input
                      inputMode="decimal"
                      value={bytesToGiB(profile.maxSizeBytes)}
                      onChange={(event) => updateQualityProfile(profile, { maxSizeBytes: giBToBytes(Number(event.target.value) || 0) })}
                    />
                  </label>
                  <label className="quality-terms">
                    <span>Prefer</span>
                    <input
                      value={(profile.preferredTerms ?? []).join(", ")}
                      onChange={(event) => updateQualityProfile(profile, { preferredTerms: splitTerms(event.target.value) })}
                    />
                  </label>
                  <label className="quality-terms">
                    <span>Reject</span>
                    <input
                      value={(profile.rejectedTerms ?? []).join(", ")}
                      onChange={(event) => updateQualityProfile(profile, { rejectedTerms: splitTerms(event.target.value) })}
                    />
                  </label>
                  <button className="secondary-action compact" disabled={Boolean(savingProfileID)} onClick={() => persistQualityProfile(profile)} type="button">
                    <CheckCircle2 size={16} />
                    {savingProfileID === key ? "Saving" : "Save"}
                  </button>
                </article>
              );
            })}
          </div>
        </section>

        <section className="history-panel" aria-label="Activity history" hidden={activeView !== "dashboard"}>
          <div className="panel-heading">
            <div>
              <h2>History</h2>
              <p>
                {historyEvents.length
                  ? `${historyEvents.length} recent monitor and grab events.`
                  : "No monitor or grab history recorded yet."}
              </p>
            </div>
            <button className="secondary-action compact" onClick={refreshWantedAndHistory} type="button">
              <HistoryIcon size={16} />
              Refresh
            </button>
          </div>
          {historyError ? <div className={isPersistenceRequiredError(historyError) ? "inline-note" : "inline-error"}>{appErrorMessage(historyError)}</div> : null}
          <div className="history-list">
            {historyEvents.map((event) => (
              <article className={`history-row ${event.severity}`} key={event.id}>
                <div>
                  <strong>{event.message}</strong>
                  <span>{event.eventType.replace(/_/g, " ")} · {formatDateTime(event.createdAt)}</span>
                </div>
                <em>{event.severity}</em>
              </article>
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}

function formatBytes(bytes: number) {
  if (!bytes) return "unknown size";
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[unit]}`;
}

function formatSpeed(bytesPerSecond: number) {
  if (!bytesPerSecond) return "0 B/s";
  return `${formatBytes(bytesPerSecond)}/s`;
}

function formatLimitSpeed(bytesPerSecond?: number) {
  if (!bytesPerSecond || bytesPerSecond < 0) return "unlimited";
  return formatSpeed(bytesPerSecond);
}

function formatDuration(seconds?: number) {
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

function filePriorityLabel(priority: number) {
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

function formatDateTime(value: string) {
  if (!value) return "unknown time";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "unknown time";
  return date.toLocaleString();
}

function mergeDownloads(current: DownloadStatus[], next: DownloadStatus[]) {
  const byID = new Map(current.map((download) => [downloadKey(download), download]));
  for (const download of next) {
    byID.set(downloadKey(download), download);
  }
  return Array.from(byID.values());
}

function downloadKey(download: DownloadStatus) {
  return `${download.client || "qBittorrent"}:${download.id}`;
}

function downloadScopeTag(scope: DownloadScope) {
  return scope === "librarry" ? "librarry" : "";
}

function uniqueDownloadClients(downloads: DownloadStatus[]) {
  return Array.from(new Set(downloads.map((download) => download.client || "qBittorrent").filter(Boolean))).sort((a, b) => a.localeCompare(b));
}

function uniqueDownloadResourceClients(downloads: DownloadStatus[], integrations: IntegrationHealth[]) {
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

function readinessTargetView(value: string): ViewID {
  return navItems.some((item) => item.id === value) ? (value as ViewID) : "settings";
}

function readarrCompatibilityStatusLabel(status: string) {
  switch (status) {
    case "ready":
      return "Ready";
    case "partial":
      return "Partial";
    case "delegated":
      return "Delegated";
    default:
      return status.replace(/_/g, " ");
  }
}

function readarrCompatibilityAuthLabel(mode: string) {
  return mode === "api_key" ? "API key" : "Open";
}

function downloadClientHealthRows(integrations: IntegrationHealth[]) {
  const clients = ["qBittorrent", "Transmission", "SABnzbd"];
  return clients.map((name) => {
    const health = integrations.find((integration) => integration.name.toLowerCase() === name.toLowerCase());
    if (!health) {
      return {
        name,
        status: "unknown",
        message: "Health not checked yet"
      };
    }
    return {
      name,
      status: health.configured ? health.status : "not configured",
      message: health.message || (health.configured ? "Configured" : "Not configured")
    };
  });
}

function integrationStatusTone(status: string) {
  const normalized = status.toLowerCase();
  if (normalized.includes("ready") || normalized.includes("ok")) return "seeding";
  if (normalized.includes("missing") || normalized.includes("not configured") || normalized.includes("unknown")) return "idle";
  if (normalized.includes("error") || normalized.includes("failed")) return "error";
  return "active";
}

function acquisitionQueueStateLabel(state: string) {
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

function acquisitionQueueStateTone(state: string) {
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

function acquisitionQueueActionID(item: AcquisitionQueueItem) {
  return `${item.wantedItem.id}:${item.state}`;
}

function acquisitionQueueActionLabel(item: AcquisitionQueueItem) {
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

function acquisitionQueueActionIcon(state: string) {
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

function acquisitionQueueActionDisabled(item: AcquisitionQueueItem) {
  if (item.state === "imported") return true;
  if (item.state === "import_ready") return queueDownloadsForImport(item).length === 0;
  return false;
}

function queueDownloadsForImport(item: AcquisitionQueueItem) {
  return (item.downloads ?? []).filter((download) =>
    download.importStatus !== "imported" &&
    (download.importStatus === "ready" || (download.progress ?? 0) >= 1 || Boolean(download.completedAt))
  );
}

function queueDownloadsForRecovery(item: AcquisitionQueueItem) {
  return (item.downloads ?? []).filter(downloadNeedsRecovery);
}

function downloadNeedsRecovery(download: DownloadStatus) {
  return Boolean(download.failureReason || download.importError || stateTone(download.state) === "error");
}

function uniqueDownloadCategories(downloads: DownloadStatus[], resources?: DownloadResources | null) {
  return Array.from(new Set([
    ...downloads.map((download) => download.category).filter(Boolean),
    ...(resources?.categories ?? []).map((category) => category.name).filter(Boolean)
  ])).sort((a, b) => a.localeCompare(b));
}

function downloadMatchesFilters(
  download: DownloadStatus,
  filters: { client: string; category: string; state: DownloadStateFilter; text: string }
) {
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
  const haystack = [
    download.name,
    download.id,
    download.client,
    download.category,
    download.savePath,
    ...(download.tags ?? [])
  ].join(" ").toLowerCase();
  return haystack.includes(text);
}

function boundedPositiveInt(value: string, fallback: number, max: number) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback;
  return Math.min(parsed, max);
}

function supportsDownloadAction(download: DownloadStatus, action: DownloadAction) {
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

function qbitOnlyDownloadAction(action: DownloadAction) {
  return [
    "toggleSequential",
    "toggleFirstLastPiece",
    "rename"
  ].includes(action);
}

function downloadSupportsQbitManagerActions(download: DownloadStatus) {
  return (download.client || "qBittorrent").toLowerCase() === "qbittorrent";
}

function downloadSupportsDetails(download: DownloadStatus) {
  const client = (download.client || "qBittorrent").toLowerCase();
  return client === "qbittorrent" || client === "transmission" || client === "sabnzbd";
}

function downloadSupportsTrackerActions(download: DownloadStatus) {
  const client = (download.client || "qBittorrent").toLowerCase();
  return client === "qbittorrent" || client === "transmission";
}

function downloadSupportsFileActions(download: DownloadStatus) {
  const client = (download.client || "qBittorrent").toLowerCase();
  return client === "qbittorrent" || client === "transmission";
}

function splitTagInput(value: string) {
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

function importReviewEvidenceChips(review: ImportReview) {
  const chips: string[] = [];
  const confidence = stringMetadataValue(review.metadata?.matchConfidence);
  if (confidence) {
    chips.push(`Confidence ${confidence}`);
  }
  const suggestedWanted = importReviewSuggestedWanted(review);
  if (suggestedWanted) {
    chips.push(`Suggested ${suggestedWanted}`);
  }
  const matchedFields = importReviewSuggestedWantedMatchedFields(review);
  if (matchedFields.length) {
    chips.push(`Matched ${matchedFields.join(", ")}`);
  }
  const evidence = review.metadata?.reviewEvidence;
  if (Array.isArray(evidence)) {
    evidence.slice(0, 3).forEach((item) => {
      if (!item || typeof item !== "object") return;
      const payload = item as Record<string, unknown>;
      const label = stringMetadataValue(payload.label) || stringMetadataValue(payload.source);
      const value = stringMetadataValue(payload.value);
      if (label && value) {
        chips.push(`${label}: ${value}`);
      } else if (label) {
        chips.push(label);
      }
    });
  }
  return Array.from(new Set(chips)).slice(0, 4);
}

function importReviewSuggestedWanted(review: ImportReview) {
  const candidate = importReviewSuggestedCandidate(review);
  if (!candidate) return "";
  return importReviewCandidateLabel(candidate);
}

function importReviewSuggestedWantedMatchedFields(review: ImportReview) {
  const candidate = importReviewSuggestedCandidate(review);
  return candidate ? importReviewCandidateMatchedFields(candidate) : [];
}

function importReviewSuggestedCandidate(review: ImportReview) {
  const candidates = importReviewWantedCandidates(review);
  const suggestedID = stringMetadataValue(review.metadata?.suggestedWantedId) || review.wantedId;
  if (!suggestedID) return undefined;
  return candidates.find((item) => stringMetadataValue(item.wantedId) === suggestedID);
}

function importReviewResolvedWantedID(review: ImportReview, selections: Record<string, string>) {
  return review.wantedId || selections[review.id] || stringMetadataValue(review.metadata?.suggestedWantedId);
}

function importReviewResolvedFormat(review: ImportReview, wantedID: string, fallbackFormat: string) {
  const candidate = importReviewWantedCandidates(review).find((item) => stringMetadataValue(item.wantedId) === wantedID);
  const candidateFormat = stringMetadataValue(candidate?.format);
  return candidateFormat || (fallbackFormat === "any" ? "ebook" : fallbackFormat);
}

function importReviewCandidateOptionLabel(candidate: Record<string, unknown>) {
  const label = importReviewCandidateLabel(candidate);
  const matched = importReviewCandidateMatchedFields(candidate);
  const score = Number(stringMetadataValue(candidate.score));
  const scoreLabel = Number.isFinite(score) && score > 0 ? score.toFixed(2) : "";
  const suffix = [matched.length ? matched.join("/") : "", scoreLabel].filter(Boolean).join(" · ");
  return suffix ? `${label} (${suffix})` : label;
}

function importReviewCandidateLabel(candidate: Record<string, unknown>) {
  const wantedID = stringMetadataValue(candidate.wantedId);
  const title = stringMetadataValue(candidate.title);
  const authorName = stringMetadataValue(candidate.authorName);
  if (title && authorName) return `${title} by ${authorName}`;
  return title || authorName || wantedID || "Wanted item";
}

function importReviewCandidateMatchedFields(candidate: Record<string, unknown>) {
  const fields = candidate?.matchedFields;
  if (!Array.isArray(fields)) return [];
  return Array.from(
    new Set(
      fields
        .map((field) => importReviewMatchedFieldLabel(stringMetadataValue(field)))
        .filter(Boolean)
    )
  );
}

function importReviewMatchedFieldLabel(field: string) {
  switch (field) {
    case "isbn":
      return "ISBN";
    case "author":
      return "author";
    case "format":
      return "format";
    case "title":
      return "title";
    default:
      return field;
  }
}

function importReviewWantedCandidates(review: ImportReview) {
  const candidates = review.metadata?.wantedCandidates;
  if (!Array.isArray(candidates)) return [];
  return candidates.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object");
}

function stringMetadataValue(value: unknown) {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function isPersistenceRequiredError(message: string) {
  return message.toLowerCase().includes("requires database persistence");
}

function appErrorMessage(message: string) {
  if (!isPersistenceRequiredError(message)) return message;
  return `${message}. Set LIBRARRY_DATABASE_URL to a Postgres database and restart the API to enable library files, import reviews, wanted queues, author monitoring, and release decisions.`;
}

type WantedPresence = "missing" | "grabbed" | "present";

type LibraryAuthorRow = {
  key: string;
  authorName: string;
  formats: string[];
  qualityProfiles: string[];
  subscriptionCount: number;
  monitoredBooks: number;
  unmonitoredBooks: number;
  missing: number;
  grabbed: number;
  present: number;
  lastSyncAt?: string;
  monitorNewItems: boolean;
  status: string;
};

type AuthorSubscriptionStats = {
  total: number;
  missing: number;
  grabbed: number;
  present: number;
  review: number;
  firstWantedItem?: WantedItem;
};

function buildLibraryAuthorRows(subscriptions: AuthorSubscription[], items: WantedItem[], presence: Map<string, WantedPresence>) {
  const rows = new Map<string, LibraryAuthorRow>();

  function rowFor(authorName: string) {
    const name = authorName.trim() || "Unknown author";
    const key = normalizedWantedText(name) || "unknown-author";
    const existing = rows.get(key);
    if (existing) return existing;
    const created: LibraryAuthorRow = {
      key,
      authorName: name,
      formats: [],
      qualityProfiles: [],
      subscriptionCount: 0,
      monitoredBooks: 0,
      unmonitoredBooks: 0,
      missing: 0,
      grabbed: 0,
      present: 0,
      monitorNewItems: false,
      status: "manual"
    };
    rows.set(key, created);
    return created;
  }

  subscriptions.forEach((subscription) => {
    const row = rowFor(subscription.authorName);
    row.subscriptionCount += 1;
    row.monitorNewItems = row.monitorNewItems || subscription.monitorNewItems;
    row.status = subscription.status || row.status;
    addUnique(row.formats, subscription.format);
    addUnique(row.qualityProfiles, subscription.qualityProfile);
    if (!row.lastSyncAt || (subscription.lastSyncAt && Date.parse(subscription.lastSyncAt) > Date.parse(row.lastSyncAt))) {
      row.lastSyncAt = subscription.lastSyncAt;
    }
  });

  items.forEach((item) => {
    const row = rowFor(item.authorName || "Unknown author");
    addUnique(row.formats, item.format);
    addUnique(row.qualityProfiles, item.qualityProfile);
    if (item.monitored) {
      row.monitoredBooks += 1;
    } else {
      row.unmonitoredBooks += 1;
    }
    switch (presence.get(item.id) ?? "missing") {
      case "present":
        row.present += 1;
        break;
      case "grabbed":
        row.grabbed += 1;
        break;
      default:
        row.missing += 1;
        break;
    }
  });

  return Array.from(rows.values()).sort((a, b) => {
    const missingDelta = b.missing - a.missing;
    if (missingDelta !== 0) return missingDelta;
    return a.authorName.localeCompare(b.authorName);
  });
}

function buildAuthorSubscriptionStatsMap(
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

function emptyAuthorSubscriptionStats(): AuthorSubscriptionStats {
  return {
    total: 0,
    missing: 0,
    grabbed: 0,
    present: 0,
    review: 0
  };
}

function authorSubscriptionStatsSummary(stats: AuthorSubscriptionStats) {
  if (stats.total === 0) {
    return "No wanted books yet. Refresh author metadata to create tracked books.";
  }
  const parts = [`${stats.total} tracked`, `${stats.missing} missing`];
  if (stats.grabbed > 0) parts.push(`${stats.grabbed} grabbed`);
  if (stats.present > 0) parts.push(`${stats.present} present`);
  if (stats.review > 0) parts.push(`${stats.review} review`);
  return parts.join(" · ");
}

function authorSubscriptionStatsBadges(stats: AuthorSubscriptionStats): Array<[string, number]> {
  if (stats.total === 0) return [["tracked", 0]];
  const badges: Array<[string, number]> = [
    ["missing", stats.missing],
    ["grabbed", stats.grabbed],
    ["present", stats.present],
    ["review", stats.review]
  ];
  return badges.filter(([, value]) => value > 0);
}

function libraryAuthorVisibleForFilter(row: LibraryAuthorRow, textFilter: string, formatFilter: LibraryFormatFilter) {
  if (formatFilter !== "all" && !row.formats.includes(formatFilter)) return false;
  const query = normalizedWantedText(textFilter);
  if (!query) return true;
  return normalizedWantedText(`${row.authorName} ${row.formats.join(" ")} ${row.qualityProfiles.join(" ")}`).includes(query);
}

function libraryBookVisibleForFilter(item: WantedItem, presence: WantedPresence | undefined, textFilter: string, formatFilter: LibraryFormatFilter) {
  if (formatFilter !== "all" && item.format !== formatFilter) return false;
  const query = normalizedWantedText(textFilter);
  if (!query) return true;
  return normalizedWantedText(`${item.title} ${item.authorName ?? ""} ${item.qualityProfile} ${item.sourceProvider ?? ""} ${presence ?? "missing"}`).includes(query);
}

function libraryPresenceRank(presence?: WantedPresence) {
  switch (presence) {
    case "missing":
    case undefined:
      return 0;
    case "grabbed":
      return 1;
    case "present":
      return 2;
  }
}

function addUnique(values: string[], value?: string) {
  const normalized = (value || "").trim();
  if (normalized && !values.includes(normalized)) values.push(normalized);
}

function wantedPresenceMap(items: WantedItem[], files: LibraryFile[]) {
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

function summarizeWantedItems(items: WantedItem[], presence: Map<string, WantedPresence>) {
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

function metadataReviewMap(queue: MetadataReviewQueue | null) {
  return new Map((queue?.items ?? []).map((item) => [item.wantedItem.id, item]));
}

function summarizeMetadataReview(queue: MetadataReviewQueue | null) {
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

function summarizeReleaseDecisions(releases: ReleaseDecision[]) {
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

function releaseDecisionVisibleForFilter(release: ReleaseDecision, filter: ReleaseDecisionFilter) {
  if (filter === "approved") return release.approved;
  if (filter === "rejected") return !release.approved;
  return true;
}

function releaseActionID(release: ReleaseDecision, force: boolean) {
  return `${release.id || release.sourceId || release.title}:${force ? "force" : "grab"}`;
}

function wantedItemVisibleForFilter(item: WantedItem, presence: WantedPresence | undefined, filter: WantedViewFilter, hasMetadataReview: boolean) {
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
    default:
      return true;
  }
}

function wantedItemSubtitle(item: WantedItem, review?: MetadataReviewItem) {
  const parts = [item.authorName || "Unknown author"];
  if (review?.conflictCount) {
    parts.push(`${review.conflictCount} metadata conflict${review.conflictCount === 1 ? "" : "s"}`);
  } else if (review?.protectedCount) {
    parts.push(`${review.protectedCount} protected field${review.protectedCount === 1 ? "" : "s"}`);
  }
  return parts.join(" · ");
}

function wantedBadgeLabel(item: WantedItem, presence: WantedPresence | undefined) {
  const state = presence ?? "missing";
  if (state === "present") return `Present · ${item.format}`;
  if (state === "grabbed") return `Grabbed · ${item.format}`;
  return `Missing · ${item.format}`;
}

function metadataReviewBadgeLabel(review: MetadataReviewItem) {
  if (review.conflictCount > 0) return `Review · ${review.conflictCount}`;
  return `Protected · ${review.protectedCount}`;
}

function wantedOverrideLabel(fieldName: string) {
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

function metadataProvenanceSummary(metadata: MetadataProvenance | null, loading: boolean) {
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

function metadataFieldSourceLabel(field: MetadataFieldEvidence) {
  if (field.reviewResolved) return "Confirmed current value";
  if (field.protected) return "Manual override";
  if (field.canonicalSource === "wanted") return "Canonical wanted value";
  if (field.candidates?.length) return "Provider candidates only";
  return "No provider evidence";
}

function metadataFieldCandidateSummary(field: MetadataFieldEvidence) {
  if (!field.candidates?.length) return "No provider candidates.";
  const candidates = field.candidates.slice(0, 3).map((candidate) => `${candidate.provider}: ${candidate.value}`);
  const extra = field.candidates.length - candidates.length;
  return extra > 0 ? `${candidates.join(" · ")} · ${extra} more` : candidates.join(" · ");
}

function metadataFieldCanApply(field: MetadataFieldEvidence) {
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

function metadataFieldCanConfirmCanonical(field: MetadataFieldEvidence) {
  return field.conflict && metadataFieldCanApply(field) && Boolean(normalizeMetadataValue(field.canonicalValue));
}

function metadataFieldCanonicalActionID(field: MetadataFieldEvidence) {
  return `${field.fieldName}:canonical:${field.canonicalValue ?? ""}`;
}

function metadataFieldApplicableCandidates(field: MetadataFieldEvidence) {
  if (!metadataFieldCanApply(field)) return [];
  const canonical = normalizeMetadataValue(field.canonicalValue);
  const seen = new Set<string>();
  return (field.candidates ?? []).filter((candidate) => {
    const value = normalizeMetadataValue(candidate.value);
    if (!value || value === canonical || seen.has(value)) return false;
    seen.add(value);
    return true;
  }).slice(0, 3);
}

function metadataRecordActionID(record: ProviderMetadataRecord) {
  return record.id || `${record.provider}:${record.providerKey}:${record.entityType}`;
}

function metadataRecordCorrections(record: ProviderMetadataRecord, metadata: MetadataProvenance | null): MetadataCorrectionRequest[] {
  const canonicalByField = new Map((metadata?.fields ?? []).map((field) => [field.fieldName, normalizeMetadataValue(field.canonicalValue)]));
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

function normalizeMetadataValue(value?: string) {
  return (value || "").trim().toLowerCase().replace(/\s+/g, " ");
}

function metadataRecordPrimaryLine(record: ProviderMetadataRecord) {
  const title = record.values.title || "Untitled";
  const author = record.values.authorName;
  return author ? `${title} by ${author}` : title;
}

function metadataRecordSecondaryLine(record: ProviderMetadataRecord) {
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

function metadataConfidenceLabel(confidence: number) {
  if (!Number.isFinite(confidence) || confidence <= 0) return "unscored";
  if (confidence <= 1) return `${Math.round(confidence * 100)}%`;
  return confidence.toFixed(1);
}

function wantedItemHasLibraryFile(item: WantedItem, files: LibraryFile[]) {
  return files.some((file) => libraryFileCountsAsPresent(file) && libraryFileMatchesWanted(item, file));
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

function normalizedWantedText(value?: string) {
  return (value || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/^(a|an|the)\s+/, "")
    .replace(/\s+/g, " ");
}

function summarizeDownloads(downloads: DownloadStatus[]) {
  return downloads.reduce(
    (summary, download) => {
      const tone = stateTone(download.state);
      if (tone === "active") summary.active += 1;
      if (tone === "paused") summary.paused += 1;
      if (tone === "error" || download.failureReason || download.importError) summary.failed += 1;
      if ((download.progress ?? 0) >= 1 || download.importStatus === "ready" || download.importStatus === "imported") {
        summary.complete += 1;
      }
      return summary;
    },
    { active: 0, paused: 0, complete: 0, failed: 0 }
  );
}

function mergeWanted(current: WantedItem[], next: WantedItem[]) {
  const byID = new Map(current.map((item) => [item.id, item]));
  for (const item of next) {
    byID.set(item.id, item);
  }
  return Array.from(byID.values()).sort((a, b) => {
    const aTime = Date.parse(a.createdAt || "");
    const bTime = Date.parse(b.createdAt || "");
    return bTime - aTime;
  });
}

function mergeAuthorSubscriptions(current: AuthorSubscription[], next: AuthorSubscription[]) {
  const byID = new Map(current.map((item) => [authorSubscriptionKey(item), item]));
  for (const item of next) {
    byID.set(authorSubscriptionKey(item), item);
  }
  return Array.from(byID.values()).sort((a, b) => a.authorName.localeCompare(b.authorName));
}

function authorSubscriptionKey(subscription: AuthorSubscription) {
  return subscription.id || `${subscription.provider}:${subscription.providerKey}:${subscription.format}`;
}

function authorSubscriptionMonitorOptions(subscription: AuthorSubscription) {
  return {
    authorIds: subscription.id ? [subscription.id] : [],
    providerKeys: subscription.providerKey ? [subscription.providerKey] : [],
    force: true,
    targetKey: authorSubscriptionKey(subscription)
  };
}

function normalizedAuthorMissingPolicy(policy?: string): AuthorMissingBookPolicy {
  if (policy === "future" || policy === "none") return policy;
  return "all";
}

function authorMissingPolicyLabel(policy: AuthorMissingBookPolicy) {
  switch (policy) {
    case "future":
      return "Future";
    case "none":
      return "None";
    default:
      return "All";
  }
}

function authorSkippedItemKey(subscription: AuthorSubscription, skipped: AuthorSkippedItem) {
  return [
    subscription.id || subscription.providerKey || subscription.authorName,
    skipped.result.provider,
    skipped.result.work.id || skipped.result.edition?.id || skipped.result.work.title,
    skipped.result.edition?.publishedDate || skipped.result.work.firstPublishYear || "undated"
  ].join(":");
}

function authorSkippedDateLabel(result: SearchResult) {
  return result.edition?.publishedDate || (result.work.firstPublishYear ? String(result.work.firstPublishYear) : "undated");
}

function mergeLibraryFiles(current: LibraryFile[], next: LibraryFile[]) {
  const byPath = new Map(current.map((file) => [file.path, file]));
  for (const file of next) {
    byPath.set(file.path, file);
  }
  return Array.from(byPath.values()).sort((a, b) => {
    const aTime = Date.parse(a.updatedAt || "");
    const bTime = Date.parse(b.updatedAt || "");
    return bTime - aTime;
  });
}

function profileKey(profile: QualityProfile) {
  return profile.id || `${profile.name}:${profile.mediaFormat}`;
}

function readarrImportCount(outcome: ReadarrImportOutcome) {
  return outcome.sections.reduce((total, section) => total + section.count, 0);
}

function readarrImportImported(outcome: ReadarrImportOutcome) {
  return outcome.sections.reduce((total, section) => total + section.imported, 0);
}

function readarrImportSectionLabel(name: string) {
  switch (name) {
    case "qualityProfiles":
      return "Quality profiles";
    case "rootFolders":
      return "Root folders";
    case "authors":
      return "Authors";
    case "books":
      return "Books";
    case "bookFiles":
      return "Book files";
    case "tags":
      return "Tags";
    case "importLists":
      return "Import lists";
    case "importListExclusions":
      return "List exclusions";
    case "delayProfiles":
      return "Delay profiles";
    case "languageProfiles":
      return "Language profiles";
    case "metadataProfiles":
      return "Metadata profiles";
    case "metadataConsumers":
      return "Metadata consumers";
    case "customFormats":
      return "Custom formats";
    case "restrictions":
      return "Restrictions";
    case "notifications":
      return "Notifications";
    case "remotePathMappings":
      return "Remote paths";
    case "downloadClients":
      return "Download clients";
    case "indexers":
      return "Indexers";
    default:
      return name;
  }
}

function readarrImportItemLabel(item: ReadarrImportItem) {
  const primary = item.title || item.authorName || item.path || item.id || "Imported record";
  const secondary = [item.authorName && item.authorName !== primary ? item.authorName : "", item.qualityProfile, item.status].filter(Boolean).join(" · ");
  return secondary ? `${primary} · ${secondary}` : primary;
}

function splitTerms(value: string) {
  return value
    .split(",")
    .map((term) => term.trim())
    .filter(Boolean);
}

function bytesToGiB(bytes: number) {
  if (!bytes) return 0;
  return Number((bytes / 1024 / 1024 / 1024).toFixed(2));
}

function giBToBytes(value: number) {
  if (!value || value < 0) return 0;
  return Math.round(value * 1024 * 1024 * 1024);
}

function libraryNamingPreviewPath(settings: LibrarySettings) {
  const values = {
    Author: "Andy Weir",
    Title: "Project Hail Mary",
    Format: "ebook",
    Ext: ".epub"
  };
  const replacement = settings.namingSpaceReplacement.trim();
  const root = settings.ebookLibraryRoot.trim() || "/data/media/books/ebooks";
  const authorSegments = libraryTemplateSegments(settings.namingAuthorFolder || "{Author}", values, replacement);
  const bookSegments = libraryTemplateSegments(settings.namingBookFolder || "{Title}", values, replacement);
  let fileName = renderLibraryTemplate(settings.namingFileName || "{Title}{Ext}", values, replacement);
  if (!fileName.toLowerCase().endsWith(values.Ext)) fileName += values.Ext;
  return [root, ...authorSegments, ...bookSegments, fileName].join("/");
}

function libraryTemplateSegments(template: string, values: Record<string, string>, replacement: string) {
  const segments = template
    .split(/[\\/]/)
    .map((segment) => renderLibraryTemplate(segment, values, replacement))
    .filter((segment) => segment && segment !== "." && segment !== "..");
  return segments.length ? segments : ["Unknown"];
}

function renderLibraryTemplate(template: string, values: Record<string, string>, replacement: string) {
  let rendered = template.trim() || "{Title}";
  for (const [key, value] of Object.entries(values)) {
    rendered = rendered.split(`{${key}}`).join(value).split(`{${key.toLowerCase()}}`).join(value);
  }
  rendered = rendered.replace(/[<>:"|?*\x00-\x1f]/g, "-").replace(/\s+/g, " ").trim();
  if (replacement) rendered = rendered.split(" ").join(replacement);
  return rendered || "Unknown";
}

function limitBytesToKiBInput(value?: number) {
  if (!value || value < 0) return "";
  return String(Math.round(value / 1024));
}

function bandwidthInputToBytes(value: string) {
  const normalized = value.trim();
  if (!normalized) return 0;
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed) || parsed < 0) return -1;
  return Math.round(parsed * 1024);
}

function queueLimitInputToInt(value: string) {
  const normalized = value.trim();
  if (!normalized) return -1;
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed) || parsed < -1) return -2;
  return Math.trunc(parsed);
}

function wantedFormat(format: string) {
  return format === "audiobook" ? "audiobook" : "ebook";
}

function stateTone(state: string) {
  const normalized = state.toLowerCase();
  if (normalized.includes("error") || normalized.includes("fail") || normalized.includes("missing")) return "error";
  if (normalized.includes("stop") || normalized.includes("pause")) return "paused";
  if (normalized.includes("upload") || normalized.includes("seed")) return "seeding";
  if (normalized.includes("download") || normalized.includes("meta")) return "active";
  return "idle";
}
