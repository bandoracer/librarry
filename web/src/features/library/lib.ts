import type { AuthorSubscription, LibraryFile, WantedItem } from "../../lib/api";

/*
 * Library feature helpers, ported from the legacy single-file App.tsx
 * (buildLibraryAuthorRows, wantedPresenceMap, summarizeWantedItems,
 * libraryAuthorVisibleForFilter, libraryBookVisibleForFilter, …).
 * Behavior is preserved verbatim unless noted.
 */

/** Presence of a wanted item relative to the on-disk library. */
export type WantedPresence = "missing" | "grabbed" | "present";

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
  grabbed: number;
  present: number;
  lastSyncAt?: string;
  monitorNewItems: boolean;
  status: string;
};

export type LibrarySummary = {
  authors: number;
  monitoredAuthors: number;
  monitoredBooks: number;
  missing: number;
  grabbed: number;
  present: number;
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

export function wantedPresenceMap(items: WantedItem[], files: LibraryFile[]): Map<string, WantedPresence> {
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

export function summarizeWantedItems(items: WantedItem[], presence: Map<string, WantedPresence>) {
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
    const row = rowFor(item.authorName);
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

/** Missing books sort first, then grabbed, then present (legacy ordering). */
export function libraryPresenceRank(presence?: WantedPresence): number {
  switch (presence) {
    case "grabbed":
      return 1;
    case "present":
      return 2;
    default:
      return 0;
  }
}

export function presenceLabel(presence: WantedPresence): string {
  if (presence === "present") return "Present";
  if (presence === "grabbed") return "Grabbed";
  return "Missing";
}

export function presenceTone(presence: WantedPresence): "danger" | "info" | "success" {
  if (presence === "present") return "success";
  if (presence === "grabbed") return "info";
  return "danger";
}

/** Monitor-policy badge for an author row. */
export function authorMonitorBadge(row: LibraryAuthorRow): { label: string; tone: "neutral" | "accent" | "warn" } {
  if (row.subscriptionCount === 0) return { label: "manual", tone: "neutral" };
  if (row.status && row.status !== "active" && row.status !== "manual") return { label: row.status, tone: "warn" };
  return row.monitorNewItems ? { label: "monitor new", tone: "accent" } : { label: "existing only", tone: "neutral" };
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
