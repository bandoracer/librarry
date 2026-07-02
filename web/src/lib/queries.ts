import { QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  fetchAcquisitionQueue,
  fetchAuthorMetadataReviews,
  fetchAuthorSubscriptions,
  fetchBlocklist,
  fetchDiskSpace,
  fetchDownloads,
  fetchHistory,
  fetchIntegrationHealth,
  fetchIntegrationSettings,
  fetchLibraryFiles,
  fetchLibraryImportReviews,
  fetchLibrarySettings,
  fetchNotificationTargets,
  fetchProviderHealth,
  fetchQualityProfiles,
  fetchReadarrCompatibility,
  fetchReadiness,
  fetchRemotePathMappings,
  fetchRootFolders,
  fetchSystemHealth,
  fetchSystemStatus,
  fetchSystemTasks,
  fetchWanted,
  fetchWantedMetadata,
  fetchWantedMetadataReview,
  fetchWantedReleases,
  knownQualityIds,
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
  importReviews: (status?: string) => ["import-reviews", status ?? "pending"] as const,
  rootFolders: ["root-folders"] as const,
  remotePathMappings: ["remote-path-mappings"] as const
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
          qualities: knownQualityIds("any").map((id) => ({ id, allowed: true })),
          cutoff: "epub",
          upgradeAllowed: true,
          minSeeders: 1
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

export function useRootFolders() {
  return useQuery({
    queryKey: keys.rootFolders,
    queryFn: withDemoFallback(fetchRootFolders, () => [])
  });
}

export function useRemotePathMappings() {
  return useQuery({
    queryKey: keys.remotePathMappings,
    queryFn: withDemoFallback(fetchRemotePathMappings, () => [])
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

/* --------------------- M5 operability: tasks/health/notify ----------------- */

/**
 * Keys for the M5 operability surface, appended alongside the shared `keys`
 * map (kept separate so parallel milestones don't collide inside one object).
 */
export const operabilityKeys = {
  systemTasks: ["system-tasks"] as const,
  systemHealth: ["system-health"] as const,
  diskSpace: ["disk-space"] as const,
  notificationTargets: ["notification-targets"] as const
};

/**
 * M5 endpoints have no demo seeds — demo installs fall back to empty lists and
 * render the (real) empty states, matching the root-folders convention.
 */
export function useSystemTasks() {
  return useQuery({
    queryKey: operabilityKeys.systemTasks,
    queryFn: withDemoFallback(fetchSystemTasks, () => []),
    refetchInterval: 15_000
  });
}

export function useSystemHealth() {
  return useQuery({
    queryKey: operabilityKeys.systemHealth,
    queryFn: withDemoFallback(fetchSystemHealth, () => []),
    refetchInterval: 60_000
  });
}

export function useDiskSpace() {
  return useQuery({
    queryKey: operabilityKeys.diskSpace,
    queryFn: withDemoFallback(fetchDiskSpace, () => []),
    refetchInterval: 60_000
  });
}

export function useNotificationTargets() {
  return useQuery({
    queryKey: operabilityKeys.notificationTargets,
    queryFn: withDemoFallback(fetchNotificationTargets, () => [])
  });
}

/* ------------- M6 long tail: calendar/auth/lists/tags/backups -------------- */

import {
  fetchAuthStatus,
  fetchBackups,
  fetchCalendar,
  fetchImportListExclusions,
  fetchImportLists,
  fetchTags,
  type AuthStatus
} from "./api";

/**
 * Keys for the M6 surface, appended alongside `keys`/`operabilityKeys` (kept
 * separate so parallel milestones don't collide inside one object).
 */
export const m6Keys = {
  calendar: (start: string, end: string, unmonitored: boolean) => ["calendar", start, end, unmonitored] as const,
  authStatus: ["auth-status"] as const,
  importLists: ["import-lists"] as const,
  importListExclusions: ["import-list-exclusions"] as const,
  tags: ["tags"] as const,
  backups: ["backups"] as const
};

/**
 * Monitored-release calendar for a date window (inclusive, YYYY-MM-DD).
 * Demo installs fall back to an empty month per the M5/root-folders convention.
 */
export function useCalendar(start: string, end: string, unmonitored: boolean) {
  return useQuery({
    queryKey: m6Keys.calendar(start, end, unmonitored),
    queryFn: withDemoFallback(
      () => fetchCalendar(start, end, unmonitored),
      () => []
    ),
    refetchInterval: 60_000,
    enabled: Boolean(start && end)
  });
}

/**
 * Session/auth state, polled every minute. Any fetch failure resolves to an
 * open install ({method:"none", authenticated:true}) so installs without the
 * auth endpoints — or with an unreachable API — are never locked out.
 */
export function useAuthStatus() {
  return useQuery<AuthStatus>({
    queryKey: m6Keys.authStatus,
    queryFn: async () => {
      try {
        return await fetchAuthStatus();
      } catch {
        return { method: "none", authenticated: true };
      }
    },
    refetchInterval: 60_000,
    retry: 0
  });
}

export function useImportLists() {
  return useQuery({
    queryKey: m6Keys.importLists,
    queryFn: withDemoFallback(fetchImportLists, () => [])
  });
}

export function useImportListExclusions() {
  return useQuery({
    queryKey: m6Keys.importListExclusions,
    queryFn: withDemoFallback(fetchImportListExclusions, () => [])
  });
}

export function useTags() {
  return useQuery({
    queryKey: m6Keys.tags,
    queryFn: withDemoFallback(fetchTags, () => [])
  });
}

export function useBackups() {
  return useQuery({
    queryKey: m6Keys.backups,
    queryFn: withDemoFallback(fetchBackups, () => [])
  });
}

/* ------------------- Release profiles / quality definitions ---------------- */

import { fetchQualityDefinitions, fetchReleaseProfiles } from "./api";

/**
 * Keys for the Quality settings surface, appended alongside the other key
 * maps (kept separate so parallel milestones don't collide inside one object).
 */
export const qualityKeys = {
  releaseProfiles: ["release-profiles"] as const,
  qualityDefinitions: ["quality-definitions"] as const
};

export function useReleaseProfiles() {
  return useQuery({
    queryKey: qualityKeys.releaseProfiles,
    queryFn: withDemoFallback(fetchReleaseProfiles, () => [])
  });
}

export function useQualityDefinitions() {
  return useQuery({
    queryKey: qualityKeys.qualityDefinitions,
    queryFn: withDemoFallback(fetchQualityDefinitions, () => [])
  });
}

/* --------------------------- Metadata profiles (wave B) --------------------- */

import { fetchMetadataProfiles } from "./api";

/**
 * Keys for the wave-B metadata-profile surface, appended alongside the other
 * key maps (kept separate so parallel milestones don't collide in one object).
 */
export const metadataProfileKeys = {
  metadataProfiles: ["metadata-profiles"] as const
};

/** Demo installs fall back to an empty list per the root-folders convention. */
export function useMetadataProfiles() {
  return useQuery({
    queryKey: metadataProfileKeys.metadataProfiles,
    queryFn: withDemoFallback(fetchMetadataProfiles, () => [])
  });
}
