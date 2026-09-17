import type { BookCollection, BookCollectionOptions } from "./api";
import { demoSeeds } from "./demo";
import { compareLibraryBooks, libraryBookMatchesMonitorFilter, libraryBookVisibleForFilter, wantedPresenceMap } from "../features/library/lib";

// Explicit demo builds keep the seeded browsing contract. Production errors
// propagate through withDemoFallback and never reach this function.
export function demoBookCollection(options: BookCollectionOptions): BookCollection {
  const all = demoSeeds.wantedItems.filter(book => !["removed", "ignored"].includes(book.status));
  const presence = wantedPresenceMap(all, []);
  const counts: Record<string, number> = {};
  all.forEach(book => { const state = presence.get(book.id) ?? "unknown"; counts[state] = (counts[state] ?? 0) + 1; });
  const matching = all.filter(book => libraryBookVisibleForFilter(book, presence.get(book.id), options.q ?? "", options.format ?? "all") && libraryBookMatchesMonitorFilter(book, options.monitor ?? "all") && (!options.state || options.state === "all" || presence.get(book.id) === options.state))
    .sort((a, b) => compareLibraryBooks(a, b, presence, options.sort ?? "status"));
  const start = options.cursor?.startsWith("demo:") ? Number(options.cursor.slice(5)) || 0 : 0;
  const end = start + (options.limit ?? 100);
  return { books: matching.slice(start, end).map(book => ({ ...book, derivedState: presence.get(book.id) })), total: all.length, filtered: matching.length, counts, recordedFiles: 0, downloads: "notConfigured", observedAt: new Date().toISOString(), nextCursor: end < matching.length ? `demo:${end}` : undefined };
}
