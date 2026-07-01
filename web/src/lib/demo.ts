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
} from "./seed";

export const demoModeEnabled = import.meta.env.VITE_LIBRARRY_DEMO === "true";

/**
 * Wraps a fetcher so demo installs render seeded data instead of raw fetch
 * errors when no backend is reachable. Outside demo mode failures propagate
 * untouched — real deployments must never silently show fake data.
 */
export function withDemoFallback<T>(fetcher: () => Promise<T>, fallback: () => T): () => Promise<T> {
  if (!demoModeEnabled) return fetcher;
  return async () => {
    try {
      return await fetcher();
    } catch {
      return fallback();
    }
  };
}

export const demoSeeds = {
  providers: seedProviders,
  integrations: seedIntegrations,
  results: seedResults,
  wantedItems: seedWantedItems,
  wantedMetadataByID: seedWantedMetadataByID,
  wantedMetadataReview: seedWantedMetadataReview,
  downloads: seedDownloads,
  downloadDetailsByKey: seedDownloadDetailsByKey,
  downloadResourcesByClient: seedDownloadResourcesByClient,
  downloadPreferencesByClient: seedDownloadPreferencesByClient,
  readarrCompatibility: seedReadarrCompatibility
};
