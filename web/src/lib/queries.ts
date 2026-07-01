import { QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  fetchAcquisitionQueue,
  fetchAuthorMetadataReviews,
  fetchAuthorSubscriptions,
  fetchBlocklist,
  fetchDownloads,
  fetchHistory,
  fetchIntegrationHealth,
  fetchIntegrationSettings,
  fetchLibraryFiles,
  fetchLibraryImportReviews,
  fetchLibrarySettings,
  fetchProviderHealth,
  fetchQualityProfiles,
  fetchReadarrCompatibility,
  fetchReadiness,
  fetchSystemStatus,
  fetchWanted,
  fetchWantedMetadata,
  fetchWantedMetadataReview,
  fetchWantedReleases,
  type DownloadListOptions
} from "./api";
import { demoModeEnabled, demoSeeds, withDemoFallback } from "./demo";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      retry: demoModeEnabled ? 0 : 1,
      refetchOnWindowFocus: !demoModeEnabled
    }
  }
});

/** Query keys shared across features; invalidate through these, not string literals. */
export const keys = {
  providerHealth: ["provider-health"] as const,
  integrationHealth: ["integration-health"] as const,
  integrationSettings: ["integration-settings"] as const,
  librarySettings: ["library-settings"] as const,
  systemStatus: ["system-status"] as const,
  readiness: ["readiness"] as const,
  readarrCompatibility: ["readarr-compatibility"] as const,
  wanted: ["wanted"] as const,
  wantedCutoffUnmet: ["wanted", "cutoff-unmet"] as const,
  wantedMetadataReview: ["wanted-metadata-review"] as const,
  wantedMetadata: (wantedID: string) => ["wanted-metadata", wantedID] as const,
  wantedReleases: (wantedID: string) => ["wanted-releases", wantedID] as const,
  acquisitionQueue: ["acquisition-queue"] as const,
  authorSubscriptions: ["author-subscriptions"] as const,
  authorMetadataReviews: ["author-metadata-reviews"] as const,
  qualityProfiles: ["quality-profiles"] as const,
  downloads: (options?: DownloadListOptions) => ["downloads", options ?? {}] as const,
  blocklist: (limit?: number) => ["blocklist", limit ?? 100] as const,
  history: (limit?: number) => ["history", limit ?? 50] as const,
  libraryFiles: (format?: string) => ["library-files", format ?? "any"] as const,
  importReviews: (status?: string) => ["import-reviews", status ?? "pending"] as const
};

export function useProviderHealth() {
  return useQuery({
    queryKey: keys.providerHealth,
    queryFn: withDemoFallback(fetchProviderHealth, () => demoSeeds.providers),
    refetchInterval: 60_000
  });
}

export function useIntegrationHealth() {
  return useQuery({
    queryKey: keys.integrationHealth,
    queryFn: withDemoFallback(fetchIntegrationHealth, () => demoSeeds.integrations),
    refetchInterval: 60_000
  });
}

export function useIntegrationSettings() {
  return useQuery({ queryKey: keys.integrationSettings, queryFn: fetchIntegrationSettings });
}

export function useLibrarySettings() {
  return useQuery({ queryKey: keys.librarySettings, queryFn: fetchLibrarySettings });
}

export function useSystemStatus() {
  return useQuery({ queryKey: keys.systemStatus, queryFn: fetchSystemStatus, refetchInterval: 60_000 });
}

export function useReadiness() {
  return useQuery({ queryKey: keys.readiness, queryFn: fetchReadiness });
}

export function useReadarrCompatibility() {
  return useQuery({
    queryKey: keys.readarrCompatibility,
    queryFn: withDemoFallback(fetchReadarrCompatibility, () => demoSeeds.readarrCompatibility)
  });
}

export function useWanted() {
  return useQuery({
    queryKey: keys.wanted,
    queryFn: withDemoFallback(fetchWanted, () => demoSeeds.wantedItems),
    refetchInterval: 30_000
  });
}

/** Wanted items whose tracked file scores under the profile cutoff (server-defined view). */
export function useCutoffUnmet(enabled = true) {
  return useQuery({
    queryKey: keys.wantedCutoffUnmet,
    queryFn: withDemoFallback(
      () => fetchWanted("cutoff-unmet"),
      () => []
    ),
    refetchInterval: 30_000,
    enabled
  });
}

export function useWantedMetadataReview() {
  return useQuery({
    queryKey: keys.wantedMetadataReview,
    queryFn: withDemoFallback(fetchWantedMetadataReview, () => demoSeeds.wantedMetadataReview)
  });
}

/**
 * Metadata provenance for one wanted item. Demo installs fall back to the
 * seeded provenance for known demo IDs (matches the legacy BooksTab behavior);
 * other failures surface as query errors.
 */
export function useWantedMetadata(wantedID: string) {
  return useQuery({
    queryKey: keys.wantedMetadata(wantedID),
    queryFn: async () => {
      try {
        return await fetchWantedMetadata(wantedID);
      } catch (error) {
        const seeded = demoModeEnabled ? demoSeeds.wantedMetadataByID[wantedID] : undefined;
        if (seeded) return seeded;
        throw error;
      }
    },
    enabled: Boolean(wantedID)
  });
}

/** Stored release decisions for one wanted item (empty list in demo mode on failure). */
export function useWantedReleases(wantedID: string) {
  return useQuery({
    queryKey: keys.wantedReleases(wantedID),
    queryFn: withDemoFallback(
      async () => (await fetchWantedReleases(wantedID)).releases,
      () => []
    ),
    enabled: Boolean(wantedID)
  });
}

export function useAcquisitionQueue() {
  return useQuery({
    queryKey: keys.acquisitionQueue,
    queryFn: withDemoFallback(
      () => fetchAcquisitionQueue(),
      () => ({
        generatedAt: new Date().toISOString(),
        summary: { total: 0, needsSearch: 0, readyToGrab: 0, queued: 0, importReady: 0, imported: 0, blocked: 0 },
        items: []
      })
    ),
    refetchInterval: 30_000
  });
}

export function useAuthorSubscriptions() {
  return useQuery({
    queryKey: keys.authorSubscriptions,
    queryFn: withDemoFallback(
      () => fetchAuthorSubscriptions(),
      () => []
    )
  });
}

export function useAuthorMetadataReviews() {
  return useQuery({
    queryKey: keys.authorMetadataReviews,
    queryFn: withDemoFallback(
      () => fetchAuthorMetadataReviews(),
      () => []
    )
  });
}

export function useQualityProfiles() {
  return useQuery({
    queryKey: keys.qualityProfiles,
    queryFn: withDemoFallback(
      fetchQualityProfiles,
      () => [
        {
          name: "standard",
          mediaFormat: "any" as const,
          minScore: 0,
          cutoffScore: 80,
          minSeeders: 1,
          maxSizeBytes: 800 * 1024 * 1024,
          preferredTerms: [],
          requiredTerms: [],
          rejectedTerms: [],
          preferredScore: 10,
          upgradeAllowed: true
        }
      ]
    )
  });
}

export function useDownloads(options?: DownloadListOptions) {
  return useQuery({
    queryKey: keys.downloads(options),
    queryFn: withDemoFallback(
      () => fetchDownloads(options ?? ""),
      () => demoSeeds.downloads
    ),
    refetchInterval: 10_000
  });
}

export function useBlocklist(limit = 100) {
  return useQuery({
    queryKey: keys.blocklist(limit),
    queryFn: withDemoFallback(
      () => fetchBlocklist(limit),
      () => []
    ),
    refetchInterval: 30_000
  });
}

export function useHistory(limit = 50) {
  return useQuery({
    queryKey: keys.history(limit),
    queryFn: withDemoFallback(
      () => fetchHistory(limit),
      () => []
    ),
    refetchInterval: 30_000
  });
}

export function useLibraryFiles(format = "any") {
  return useQuery({
    queryKey: keys.libraryFiles(format),
    queryFn: withDemoFallback(
      () => fetchLibraryFiles(format),
      () => []
    )
  });
}

export function useImportReviews(status = "pending") {
  return useQuery({
    queryKey: keys.importReviews(status),
    queryFn: withDemoFallback(
      () => fetchLibraryImportReviews(status),
      () => []
    ),
    refetchInterval: 30_000
  });
}

/**
 * Convenience wrapper for imperative mutations that should refresh a set of
 * shared queries when they land. Features pass their own mutationFn.
 */
export function useInvalidatingMutation<TArgs, TResult>(
  mutationFn: (args: TArgs) => Promise<TResult>,
  invalidate: readonly (readonly unknown[])[]
) {
  const client = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: async () => {
      await Promise.all(invalidate.map((key) => client.invalidateQueries({ queryKey: key })));
    }
  });
}

export type APIState = "live" | "offline" | "demo";

/** Global connection indicator: live when the status endpoint answers. */
export function useAPIState(): APIState {
  const status = useQuery({
    queryKey: keys.systemStatus,
    queryFn: fetchSystemStatus,
    refetchInterval: 60_000,
    retry: 0
  });
  if (status.isSuccess) return "live";
  if (status.isError) return demoModeEnabled ? "demo" : "offline";
  return demoModeEnabled ? "demo" : "live";
}
