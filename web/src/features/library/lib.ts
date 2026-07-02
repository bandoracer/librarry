import type { AuthorSubscription, LibraryFile, WantedItem } from "../../lib/api";
import { formatDate } from "../../lib/format";

/*
 * Library feature helpers, ported from the legacy single-file App.tsx
 * (buildLibraryAuthorRows, wantedPresenceMap, summarizeWantedItems,
 * libraryAuthorVisibleForFilter, libraryBookVisibleForFilter, …).
 * Behavior is preserved verbatim unless noted.
 */

/**
 * Readarr-style derived book state. Prefer the backend-computed
 * `WantedItem.derivedState`; `wantedItemBookState` falls back to the legacy
 * status/file inference when it is absent (demo mode, older APIs).
 */
export const bookStates = ["unmonitored", "missing", "downloading", "downloaded", "cutoffUnmet"] as const;
export type WantedPresence = (typeof bookStates)[number];

export type LibraryFormatFilter = "all" | "ebook" | "audiobook";

export type LibraryAuthorRow = {
  key: string;
  authorName: string;
  formats: string[];
  qualityProfiles: string[];
  subscriptionCount: number;
  monitoredBooks: number;
  unmonitoredBooks: number;
  missing: number;
  downloading: number;
  downloaded: number;
  cutoffUnmet: number;
  lastSyncAt?: string;
  monitorNewItems: boolean;
  status: string;
};

export type LibrarySummary = {
  authors: number;
  monitoredAuthors: number;
  monitoredBooks: number;
  missing: number;
  downloading: number;
  downloaded: number;
  files: number;
  manualOverrides: number;
};

export function normalizedWantedText(value?: string): string {
  return (value || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/^(a|an|the)\s+/, "")
    .replace(/\s+/g, " ");
}

/** Stable grouping key for an author name; shared by author rows and books. */
export function libraryAuthorKey(authorName?: string): string {
  const name = (authorName ?? "").trim() || "Unknown author";
  return normalizedWantedText(name) || "unknown-author";
}

function addUnique(values: string[], value?: string) {
  const normalized = (value || "").trim();
  if (normalized && !values.includes(normalized)) values.push(normalized);
}

function stringMetadataValue(value: unknown): string {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function libraryFileCountsAsPresent(file: LibraryFile): boolean {
  const status = (file.importStatus || "").toLowerCase();
  return Boolean(file.path) && (status === "" || status === "available" || status === "imported");
}

function libraryFileMatchesWanted(item: WantedItem, file: LibraryFile): boolean {
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

function wantedItemHasLibraryFile(item: WantedItem, files: LibraryFile[]): boolean {
  return files.some((file) => libraryFileCountsAsPresent(file) && libraryFileMatchesWanted(item, file));
}

function normalizedDerivedState(value?: string): WantedPresence | undefined {
  return bookStates.includes(value as WantedPresence) ? (value as WantedPresence) : undefined;
}

/** Derived book state: backend `derivedState` first, legacy status/file inference as fallback. */
export function wantedItemBookState(item: WantedItem, files: LibraryFile[]): WantedPresence {
  const derived = normalizedDerivedState(item.derivedState);
  if (derived) return derived;
  if (wantedItemHasLibraryFile(item, files)) return "downloaded";
  if ((item.status || "").toLowerCase() === "grabbed") return "downloading";
  return "missing";
}

export function wantedPresenceMap(items: WantedItem[], files: LibraryFile[]): Map<string, WantedPresence> {
  const entries = new Map<string, WantedPresence>();
  items.forEach((item) => {
    entries.set(item.id, wantedItemBookState(item, files));
  });
  return entries;
}

export function summarizeWantedItems(items: WantedItem[], presence: Map<string, WantedPresence>) {
  return items.reduce(
    (summary, item) => {
      const state = presence.get(item.id) ?? "missing";
      summary[state] += 1;
      return summary;
    },
    { missing: 0, downloading: 0, downloaded: 0, cutoffUnmet: 0, unmonitored: 0 }
  );
}

export function buildLibraryAuthorRows(
  subscriptions: AuthorSubscription[],
  items: WantedItem[],
  presence: Map<string, WantedPresence>
): LibraryAuthorRow[] {
  const rows = new Map<string, LibraryAuthorRow>();

  function rowFor(authorName?: string) {
    const name = (authorName ?? "").trim() || "Unknown author";
    const key = libraryAuthorKey(name);
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
      downloading: 0,
      downloaded: 0,
      cutoffUnmet: 0,
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
    const row = rowFor(item.authorName);
    addUnique(row.formats, item.format);
    addUnique(row.qualityProfiles, item.qualityProfile);
    if (item.monitored) {
      row.monitoredBooks += 1;
    } else {
      row.unmonitoredBooks += 1;
    }
    switch (presence.get(item.id) ?? "missing") {
      case "downloaded":
        row.downloaded += 1;
        break;
      case "downloading":
        row.downloading += 1;
        break;
      case "cutoffUnmet":
        row.cutoffUnmet += 1;
        break;
      case "unmonitored":
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

export function libraryAuthorVisibleForFilter(row: LibraryAuthorRow, textFilter: string, formatFilter: LibraryFormatFilter): boolean {
  if (formatFilter !== "all" && !row.formats.includes(formatFilter)) return false;
  const query = normalizedWantedText(textFilter);
  if (!query) return true;
  return normalizedWantedText(`${row.authorName} ${row.formats.join(" ")} ${row.qualityProfiles.join(" ")}`).includes(query);
}

export function libraryBookVisibleForFilter(
  item: WantedItem,
  presence: WantedPresence | undefined,
  textFilter: string,
  formatFilter: LibraryFormatFilter
): boolean {
  if (formatFilter !== "all" && item.format !== formatFilter) return false;
  const query = normalizedWantedText(textFilter);
  if (!query) return true;
  return normalizedWantedText(
    `${item.title} ${item.authorName ?? ""} ${item.qualityProfile} ${item.sourceProvider ?? ""} ${presence ?? "missing"}`
  ).includes(query);
}

/** Status sort order: Missing → Downloading → Cutoff Unmet → Downloaded, unmonitored last. */
export function libraryPresenceRank(presence?: WantedPresence): number {
  switch (presence) {
    case "downloading":
      return 1;
    case "cutoffUnmet":
      return 2;
    case "downloaded":
      return 3;
    case "unmonitored":
      return 4;
    default:
      return 0;
  }
}

export function presenceLabel(presence: WantedPresence): string {
  switch (presence) {
    case "downloaded":
      return "Downloaded";
    case "downloading":
      return "Downloading";
    case "cutoffUnmet":
      return "Cutoff Unmet";
    case "unmonitored":
      return "Unmonitored";
    default:
      return "Missing";
  }
}

export function presenceTone(presence: WantedPresence): "danger" | "info" | "success" | "warn" | "neutral" {
  switch (presence) {
    case "downloaded":
      return "success";
    case "downloading":
      return "info";
    case "cutoffUnmet":
      return "warn";
    case "unmonitored":
      return "neutral";
    default:
      return "danger";
  }
}

/** Monitor-policy badge for an author row. */
export function authorMonitorBadge(row: LibraryAuthorRow): { label: string; tone: "neutral" | "accent" | "warn" } {
  if (row.subscriptionCount === 0) return { label: "manual", tone: "neutral" };
  if (row.status && row.status !== "active" && row.status !== "manual") return { label: row.status, tone: "warn" };
  return row.monitorNewItems ? { label: "monitor new", tone: "accent" } : { label: "existing only", tone: "neutral" };
}

/* ------------------------------ View modes -------------------------------- */

export type LibraryViewMode = "table" | "posters" | "overview";

export const libraryViewModes: LibraryViewMode[] = ["table", "posters", "overview"];

const libraryViewStorageKey = "librarry.libraryView";

export function loadLibraryViewMode(): LibraryViewMode {
  if (typeof window === "undefined") return "table";
  const stored = window.localStorage.getItem(libraryViewStorageKey);
  return libraryViewModes.includes(stored as LibraryViewMode) ? (stored as LibraryViewMode) : "table";
}

export function storeLibraryViewMode(view: LibraryViewMode) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(libraryViewStorageKey, view);
}

/* ---------------------------- Sort & filters ------------------------------- */

export type LibrarySortMode = "status" | "title" | "author" | "added";

export const librarySortLabels: Record<LibrarySortMode, string> = {
  status: "Status",
  title: "Title",
  author: "Author",
  added: "Recently Added"
};

export type LibraryMonitorFilter = "all" | "monitored" | "unmonitored";

export function libraryBookMatchesMonitorFilter(item: WantedItem, filter: LibraryMonitorFilter): boolean {
  if (filter === "monitored") return item.monitored;
  if (filter === "unmonitored") return !item.monitored;
  return true;
}

function safeTimestamp(iso?: string): number {
  const value = Date.parse(iso ?? "");
  return Number.isNaN(value) ? 0 : value;
}

function authorTitleCompare(a: WantedItem, b: WantedItem): number {
  return `${a.authorName ?? ""} ${a.title}`.localeCompare(`${b.authorName ?? ""} ${b.title}`);
}

/** Comparator for the Library books list. "status" orders by derived state:
 *  missing → downloading → cutoff unmet → downloaded (unmonitored last),
 *  author+title inside. */
export function compareLibraryBooks(
  a: WantedItem,
  b: WantedItem,
  presence: Map<string, WantedPresence>,
  sort: LibrarySortMode
): number {
  switch (sort) {
    case "title": {
      const delta = a.title.localeCompare(b.title);
      return delta !== 0 ? delta : (a.authorName ?? "").localeCompare(b.authorName ?? "");
    }
    case "author":
      return authorTitleCompare(a, b);
    case "added": {
      const delta = safeTimestamp(b.createdAt) - safeTimestamp(a.createdAt);
      return delta !== 0 ? delta : authorTitleCompare(a, b);
    }
    default: {
      const stateDelta = libraryPresenceRank(presence.get(a.id)) - libraryPresenceRank(presence.get(b.id));
      if (stateDelta !== 0) return stateDelta;
      return authorTitleCompare(a, b);
    }
  }
}

/* -------------------------------- Routes ----------------------------------- */

/** Route to the author detail page for an author name (wanted-only or subscribed). */
export function libraryAuthorPath(authorName?: string): string {
  return `/library/author/${encodeURIComponent(libraryAuthorKey(authorName))}`;
}

/** Route to the book detail page for a wanted item. */
export function libraryBookPath(wantedID: string): string {
  return `/library/book/${encodeURIComponent(wantedID)}`;
}

/** Muted descriptor line for Overview rows and the book page header. */
export function libraryBookOverviewLine(item: WantedItem): string {
  const overrideCount = item.manualOverrides?.length ?? 0;
  return [
    item.sourceProvider || "manual",
    item.format,
    item.qualityProfile,
    item.monitored ? "monitored" : "unmonitored",
    item.createdAt ? `added ${formatDate(item.createdAt)}` : "",
    overrideCount ? `${overrideCount} override${overrideCount === 1 ? "" : "s"}` : ""
  ]
    .filter(Boolean)
    .join(" · ");
}

export function isPersistenceRequiredError(message: string): boolean {
  return message.toLowerCase().includes("requires database persistence");
}

/** Expands persistence-required errors with the remediation hint (legacy appErrorMessage). */
export function libraryErrorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error ?? "Request failed");
  if (!isPersistenceRequiredError(message)) return message;
  return `${message}. Set LIBRARRY_DATABASE_URL to a Postgres database and restart the API to enable library files, import reviews, wanted queues, author monitoring, and release decisions.`;
}
