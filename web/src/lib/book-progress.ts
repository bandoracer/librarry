import type { WantedItem } from "./api";

/** Detailed activity complements the compatible coarse presence/filter states. */
export function bookProgressLabel(book?: WantedItem): string {
  if (!book || ["removed", "ignored"].includes(book.status)) return "";
  if (["downloaded", "cutoffUnmet"].includes(book.derivedState ?? "")) return "";
  if (book.importReviewId) return "Needs import review";
  return ({ import_ready: "Waiting for import", stalled: "Stalled", waiting_metadata: "Waiting for metadata", paused: "Paused", failed: "Needs attention", queued: "Queued", downloading: "Downloading" } as Record<string, string>)[book.downloadState ?? ""] ?? "";
}
