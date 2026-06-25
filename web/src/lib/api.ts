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
  lastSeenAt?: string;
};

export type DownloadAction =
  | "start"
  | "stop"
  | "delete"
  | "recheck"
  | "increasePriority"
  | "decreasePriority"
  | "topPriority"
  | "bottomPriority";

export type DownloadActionResult = {
  action: DownloadAction;
  ids: string[];
  applied: boolean;
  message?: string;
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
  sourceProvider?: string;
  sourceKey?: string;
  lastSearchAt?: string;
  createdAt: string;
  updatedAt: string;
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

const apiBase = import.meta.env.VITE_API_BASE_URL ?? "";

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

export async function fetchDownloads(tag = "librarry"): Promise<DownloadStatus[]> {
  const params = new URLSearchParams();
  if (tag) params.set("tag", tag);
  const response = await fetch(`${apiBase}/api/v1/downloads?${params.toString()}`);
  if (!response.ok) {
    throw new Error(`Download refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { downloads: DownloadStatus[] };
  return payload.downloads;
}

export async function runDownloadAction(
  action: DownloadAction,
  ids: string[],
  options: { deleteFiles?: boolean } = {}
): Promise<DownloadActionResult> {
  const response = await fetch(`${apiBase}/api/v1/downloads/actions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      action,
      ids,
      deleteFiles: options.deleteFiles ?? false
    })
  });
  if (!response.ok) {
    throw new Error(`Download action failed: ${response.status}`);
  }
  return (await response.json()) as DownloadActionResult;
}

export async function fetchWanted(): Promise<WantedItem[]> {
  const response = await fetch(`${apiBase}/api/v1/wanted?status=wanted`);
  if (!response.ok) {
    throw new Error(`Wanted refresh failed: ${response.status}`);
  }
  const payload = (await response.json()) as { wanted: WantedItem[] };
  return payload.wanted;
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
