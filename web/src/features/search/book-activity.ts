import type { BookMatch, SearchResult, WantedItem } from "../../lib/api";
import { searchResultKey } from "./lib";

export type BookActivity = {
  result: SearchResult;
  format: "ebook" | "audiobook";
  phase: "adding" | "searching" | "starting" | "queued" | "saved" | "unavailable" | "error";
  item?: WantedItem;
  message?: string;
};

export function activityMatches(activity: BookActivity, result: SearchResult, format: BookActivity["format"]) {
  if (activity.format !== format) return false;
  // Same-work editions and verified aliases share a lock. Names never establish identity.
  const workIDs = new Set([activity.result.work.id, ...(activity.result.work.providerIds ?? [])].filter(Boolean));
  const editionIDs = new Set([activity.result.edition?.id, ...(activity.result.edition?.providerIds ?? [])].filter(Boolean));
  return [result.work.id, ...(result.work.providerIds ?? [])].some(id => Boolean(id) && workIDs.has(id)) ||
    [result.edition?.id, ...(result.edition?.providerIds ?? [])].some(id => Boolean(id) && editionIDs.has(id)) ||
    searchResultKey(activity.result) === searchResultKey(result);
}

export function activityBusy(activity?: BookActivity) {
  return activity?.phase === "adding" || activity?.phase === "searching" || activity?.phase === "starting";
}

export function activityLabel(activity: BookActivity) {
  return { adding: "Adding", searching: "Finding download", starting: "Starting download", queued: "Queued", saved: "Saved", unavailable: "No download found", error: "Needs attention" }[activity.phase];
}

export function savedBookLabel(match: BookMatch) {
  if (match.total > 1) return `${match.total} saved matches`;
  const book = match.books[0];
  if (book?.status === "removed") return "Removed";
  if (book?.status === "ignored") return "Ignored";
  if (book?.derivedState === "downloaded") return "In library";
  if (book?.derivedState === "downloading") return "Downloading";
  if (book?.status === "grabbed") return "Queued";
  return "Tracked";
}

export function bookActivityLabel(activity?: BookActivity, match?: BookMatch) {
  // Fresh durable evidence can supersede a completed local action, but an old
  // wanted row must not erase its queued/error result while the client catches up.
  const saved = match?.total ? savedBookLabel(match) : "";
  if (activityBusy(activity)) return activityLabel(activity!);
  if (saved && saved !== "Tracked") return saved;
  return activity ? activityLabel(activity) : saved;
}

export function bookActivityTone(label: string) {
  if (["Adding", "Finding download", "Starting download", "Downloading"].includes(label)) return "info";
  if (["Needs attention", "No download found"].includes(label)) return "warn";
  if (["Queued", "Saved", "In library"].includes(label)) return "success";
  return "neutral";
}
