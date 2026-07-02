import { knownQualityIds } from "../../lib/api";
import type {
  IntegrationSettings,
  LibrarySettings,
  QualityProfile,
  QualityProfileQuality,
  ReadarrImportItem,
  ReadarrImportOutcome,
  ReadarrImportSettings
} from "../../lib/api";

/*
 * Pure helpers ported from the legacy single-file App.tsx. The changed-
 * detection semantics (trim-insensitive comparison, secrets counting as a
 * change whenever non-empty) are stable behavior — keep them intact.
 */

/* ------------------------------ Form defaults ----------------------------- */

export function emptyIntegrationSettings(): IntegrationSettings {
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

/** Form copy of persisted settings: secrets are never echoed back into inputs. */
export function integrationSettingsForm(settings: IntegrationSettings): IntegrationSettings {
  return {
    ...settings,
    prowlarrApiKey: "",
    qbittorrentPassword: "",
    transmissionPassword: "",
    sabnzbdApiKey: "",
    sabnzbdPassword: ""
  };
}

export function emptyLibrarySettings(): LibrarySettings {
  return {
    ebookLibraryRoot: "/data/media/books/ebooks",
    audiobookLibraryRoot: "/data/media/books/audiobooks",
    namingAuthorFolder: "{Author}",
    namingBookFolder: "{Title}",
    namingFileName: "{Title}{Ext}",
    namingSpaceReplacement: "",
    standardSearchLanguage: "English",
    renameBooks: true
  };
}

export function emptyReadarrImportSettings(): ReadarrImportSettings {
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

export const standardSearchLanguageOptions = [
  "English",
  "Any",
  "German",
  "French",
  "Spanish",
  "Italian",
  "Dutch",
  "Japanese",
  "Chinese",
  "Korean",
  "Portuguese"
];

/* ---------------------------- Changed detection ---------------------------- */

export function normalizedFormText(value?: string | null) {
  return (value ?? "").trim();
}

export function sameFormText(a?: string | null, b?: string | null) {
  return normalizedFormText(a) === normalizedFormText(b);
}

export function normalizedFormTerms(values?: string[]) {
  return (values ?? []).map((value) => normalizedFormText(value)).filter(Boolean);
}

export function sameFormTerms(a?: string[], b?: string[]) {
  const left = normalizedFormTerms(a);
  const right = normalizedFormTerms(b);
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

export function cloneQualityProfile(profile: QualityProfile): QualityProfile {
  return {
    ...profile,
    qualities: profile.qualities.map((quality) => ({ ...quality }))
  };
}

/** Default quality list for a new profile: catalog order, everything allowed. */
export function defaultProfileQualities(mediaFormat: QualityProfile["mediaFormat"]): QualityProfileQuality[] {
  return knownQualityIds(mediaFormat).map((id) => ({ id, allowed: true }));
}

function sameProfileQualities(a: QualityProfileQuality[], b: QualityProfileQuality[]) {
  return a.length === b.length && a.every((quality, index) => quality.id === b[index].id && quality.allowed === b[index].allowed);
}

export function qualityProfileChanged(profile: QualityProfile, saved?: QualityProfile) {
  if (!saved) return true;
  return (
    profile.cutoff !== saved.cutoff ||
    profile.minSeeders !== saved.minSeeders ||
    profile.upgradeAllowed !== saved.upgradeAllowed ||
    !sameProfileQualities(profile.qualities, saved.qualities)
  );
}

export function librarySettingsChanged(form: LibrarySettings, saved: LibrarySettings) {
  return (
    !sameFormText(form.ebookLibraryRoot, saved.ebookLibraryRoot) ||
    !sameFormText(form.audiobookLibraryRoot, saved.audiobookLibraryRoot) ||
    !sameFormText(form.namingAuthorFolder, saved.namingAuthorFolder) ||
    !sameFormText(form.namingBookFolder, saved.namingBookFolder) ||
    !sameFormText(form.namingFileName, saved.namingFileName) ||
    !sameFormText(form.namingSpaceReplacement, saved.namingSpaceReplacement) ||
    (form.renameBooks ?? true) !== (saved.renameBooks ?? true) ||
    (normalizedFormText(form.standardSearchLanguage) || "English") !== (normalizedFormText(saved.standardSearchLanguage) || "English")
  );
}

/** Indexers tab: Prowlarr URL and API key (secret counts as changed when non-empty). */
export function indexerSettingsChanged(form: IntegrationSettings, saved: IntegrationSettings) {
  return Boolean(normalizedFormText(form.prowlarrApiKey)) || !sameFormText(form.prowlarrUrl, saved.prowlarrUrl);
}

/** Download Clients tab: every integration field except the Prowlarr pair. */
export function downloadClientSettingsChanged(form: IntegrationSettings, saved: IntegrationSettings) {
  const secretChanged = Boolean(
    normalizedFormText(form.qbittorrentPassword) ||
      normalizedFormText(form.transmissionPassword) ||
      normalizedFormText(form.sabnzbdApiKey) ||
      normalizedFormText(form.sabnzbdPassword)
  );
  return (
    secretChanged ||
    !sameFormText(form.qbittorrentUrl, saved.qbittorrentUrl) ||
    !sameFormText(form.qbittorrentUsername, saved.qbittorrentUsername) ||
    !sameFormText(form.transmissionUrl, saved.transmissionUrl) ||
    !sameFormText(form.transmissionUsername, saved.transmissionUsername) ||
    !sameFormText(form.sabnzbdUrl, saved.sabnzbdUrl) ||
    !sameFormText(form.sabnzbdUsername, saved.sabnzbdUsername) ||
    !sameFormText(form.ebookCategory, saved.ebookCategory) ||
    !sameFormText(form.audiobookCategory, saved.audiobookCategory) ||
    !sameFormText(form.bookTorrentRoot, saved.bookTorrentRoot)
  );
}

/* ------------------------------ Naming preview ----------------------------- */

/** Tokens supported by the backend naming renderer, in hint order. */
export const libraryNamingTokens = ["{Author}", "{Title}", "{Series}", "{SeriesPosition}", "{Year}", "{Format}", "{Ext}"];

export function libraryNamingPreviewPath(settings: LibrarySettings) {
  const values = {
    Author: "Andy Weir",
    Title: "Project Hail Mary",
    Series: "The Long Way",
    SeriesPosition: "1",
    Year: "2016",
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

export function libraryTemplateSegments(template: string, values: Record<string, string>, replacement: string) {
  const segments = template
    .split(/[\\/]/)
    .map((segment) => renderLibraryTemplate(segment, values, replacement))
    .filter((segment) => segment && segment !== "." && segment !== "..");
  return segments.length ? segments : ["Unknown"];
}

export function renderLibraryTemplate(template: string, values: Record<string, string>, replacement: string) {
  let rendered = template.trim() || "{Title}";
  for (const [key, value] of Object.entries(values)) {
    rendered = rendered.split(`{${key}}`).join(value).split(`{${key.toLowerCase()}}`).join(value);
  }
  rendered = collapseEmptyTemplateSeparators(rendered);
  rendered = rendered.replace(/[<>:"|?*\x00-\x1f]/g, "-").replace(/\s+/g, " ").trim();
  if (replacement) rendered = rendered.split(" ").join(replacement);
  return rendered || "Unknown";
}

/**
 * Cleans up the residue empty token values leave behind so a template like
 * "{Series} - {Title} ({Year})" degrades to "Title" instead of " - Title ()"
 * when a book has no series or year (mirrors the backend renderer).
 */
export function collapseEmptyTemplateSeparators(value: string) {
  let result = value;
  // Empty bracket groups (optionally holding only separators): "()", "[ - ]".
  result = result.replace(/\(\s*[-–—._]*\s*\)/g, " ").replace(/\[\s*[-–—._]*\s*\]/g, " ");
  // Runs of separators collapsed to the first: "Title - - Name" → "Title - Name".
  result = result.replace(/(\s*[-–—_]\s*)(?:[-–—_]\s*)+/g, "$1");
  // Dangling separators at either end: " - Title", "Title -".
  result = result.replace(/^\s*[-–—_]+\s*/, "").replace(/\s*[-–—_]+\s*$/, "");
  return result;
}

/* ----------------------------- Quality profiles ---------------------------- */

export function profileKey(profile: QualityProfile) {
  return profile.id || `${profile.name}:${profile.mediaFormat}`;
}

export function splitTerms(value: string) {
  return value
    .split(",")
    .map((term) => term.trim())
    .filter(Boolean);
}

/* ------------------------------ Readarr import ----------------------------- */

export function readarrImportCount(outcome: ReadarrImportOutcome) {
  return outcome.sections.reduce((total, section) => total + section.count, 0);
}

export function readarrImportImported(outcome: ReadarrImportOutcome) {
  return outcome.sections.reduce((total, section) => total + section.imported, 0);
}

export function readarrImportSectionLabel(name: string) {
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

export function readarrImportItemLabel(item: ReadarrImportItem) {
  const primary = item.title || item.authorName || item.path || item.id || "Imported record";
  const secondary = [item.authorName && item.authorName !== primary ? item.authorName : "", item.qualityProfile, item.status]
    .filter(Boolean)
    .join(" · ");
  return secondary ? `${primary} · ${secondary}` : primary;
}

/* --------------------------------- Errors ---------------------------------- */

export function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

export function isPersistenceRequiredError(message: string) {
  return message.toLowerCase().includes("requires database persistence");
}

export function appErrorMessage(message: string) {
  if (!isPersistenceRequiredError(message)) return message;
  return `${message}. Set LIBRARRY_DATABASE_URL to a Postgres database and restart the API to enable library files, import reviews, wanted queues, author monitoring, and release decisions.`;
}
