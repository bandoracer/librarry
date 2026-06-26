import type { ProviderHealth, ReadarrCompatibilityReport, SearchResult, WantedItem } from "./api";

export const seedProviders: ProviderHealth[] = [
  {
    name: "Hardcover",
    status: "missing_credentials",
    configured: false,
    message: "Set LIBRARRY_HARDCOVER_TOKEN to enable rich metadata.",
    checkedAt: new Date().toISOString()
  },
  {
    name: "Open Library",
    status: "ready",
    configured: true,
    message: "Open API configured as the open-data backbone.",
    checkedAt: new Date().toISOString()
  },
  {
    name: "Google Books",
    status: "missing_credentials",
    configured: false,
    message: "Set LIBRARRY_GOOGLE_BOOKS_API_KEY for exact fallback lookup.",
    checkedAt: new Date().toISOString()
  },
  {
    name: "Local OPF",
    status: "ready",
    configured: true,
    message: "Local metadata will be used during imports.",
    checkedAt: new Date().toISOString()
  }
];

export const seedResults: SearchResult[] = [
  {
    provider: "Open Library",
    kind: "book",
    work: {
      id: "openlibrary:OL21745884W",
      title: "Project Hail Mary",
      authors: [{ id: "openlibrary:OL7234434A", name: "Andy Weir" }],
      firstPublishYear: 2021,
      coverUrl: "https://covers.openlibrary.org/b/id/11200092-L.jpg"
    },
    edition: {
      id: "openlibrary:OL30036715M",
      title: "Project Hail Mary",
      format: "ebook",
      language: "eng",
      isbns: ["9780593135204", "9780593135211"],
      publisher: "Ballantine Books",
      publishedDate: "2021-05-04"
    },
    score: 0.93,
    confidence: "high",
    matchedOn: ["title", "author"]
  },
  {
    provider: "Open Library",
    kind: "book",
    work: {
      id: "openlibrary:OL24593432W",
      title: "Dungeon Crawler Carl",
      authors: [{ id: "openlibrary:OL3101279A", name: "Matt Dinniman" }],
      firstPublishYear: 2020,
      series: "Dungeon Crawler Carl",
      seriesPosition: "1",
      coverUrl: "https://covers.openlibrary.org/b/id/15143022-L.jpg"
    },
    edition: {
      id: "openlibrary:OL32618729M",
      title: "Dungeon Crawler Carl",
      format: "audiobook",
      language: "eng",
      isbns: ["9798217287161"],
      publisher: "Audible Studios",
      publishedDate: "2021-02-01"
    },
    score: 0.86,
    confidence: "medium",
    matchedOn: ["title"]
  }
];

export const seedWantedItems: WantedItem[] = [
  {
    id: "demo-wanted-project-hail-mary",
    workId: "openlibrary:OL21745884W",
    editionId: "openlibrary:OL30036715M",
    title: "Project Hail Mary",
    authorName: "Andy Weir",
    coverUrl: "https://covers.openlibrary.org/b/id/11200092-L.jpg",
    format: "ebook",
    qualityProfile: "standard",
    status: "wanted",
    monitored: true,
    sourceProvider: "Open Library",
    sourceKey: "openlibrary:OL30036715M",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString()
  }
];

export const seedReadarrCompatibility: ReadarrCompatibilityReport = {
  status: "early_alpha",
  summary: "Readarr-style probes, migration, library, wanted, release, queue, import, and common config APIs are available. Remaining work is concentrated in deeper Calibre/embedded metadata behavior and native side effects for some compatibility resources.",
  authMode: "open",
  compatibleRoutes: 90,
  readyAreas: 6,
  partialAreas: 1,
  delegatedAreas: 1,
  generatedAt: new Date().toISOString(),
  categories: [
    {
      id: "system",
      title: "Client probes and system state",
      status: "ready",
      endpointCount: 19,
      message: "Common Arr probes return Readarr-style payloads for status checks, route discovery, filesystem browsing, logs, update, and backup calls.",
      examples: ["/ping", "/api/v1/system/status", "/api/v1/system/routes", "/api/v1/diskspace"]
    },
    {
      id: "library",
      title: "Authors, books, wanted, and files",
      status: "ready",
      endpointCount: 31,
      message: "Author, book, wanted, missing, cutoff, bookfile, calendar, history, parse, rename, and retag surfaces map onto native Librarry state.",
      examples: ["/api/v1/author", "/api/v1/book", "/api/v1/wanted/missing", "/api/v1/bookfile"]
    },
    {
      id: "metadata",
      title: "Metadata lookup and correction",
      status: "ready",
      endpointCount: 6,
      message: "Lookup routes use provider-backed metadata search, while native provenance and manual overrides preserve corrected book facts.",
      examples: ["/api/v1/author/lookup", "/api/v1/book/lookup", "/api/v1/search", "/api/v1/wanted/metadata/review"]
    },
    {
      id: "acquisition",
      title: "Release search, queue, and history",
      status: "ready",
      endpointCount: 17,
      message: "Release search/grab, queue, blocklist, command, history, failed-download recovery, feed sync, and upgrade-search calls target book acquisition workflows.",
      examples: ["/api/v1/release", "/api/v1/queue", "/api/v1/command", "/api/v1/blocklist"]
    },
    {
      id: "resources",
      title: "Config and Arr resources",
      status: "ready",
      endpointCount: 54,
      message: "Profiles, tags, restrictions, import lists, notifications, root folders, path mappings, indexers, and clients persist as compatibility records.",
      examples: ["/api/v1/qualityprofile", "/api/v1/rootfolder", "/api/v1/importlist", "/api/v1/notification"]
    },
    {
      id: "migration",
      title: "Readarr migration",
      status: "ready",
      endpointCount: 2,
      message: "Preview/apply import routes move existing Readarr state into Librarry.",
      examples: ["/api/v1/readarr/import/preview", "/api/v1/readarr/import"]
    },
    {
      id: "calibre-import",
      title: "Calibre and post-import organization",
      status: "partial",
      endpointCount: 9,
      message: "Imports, rename previews, Calibre add-book, set-fields, conversion starts, and refresh exist; richer embedded metadata writes remain work in progress.",
      examples: ["/api/v1/manualimport", "/api/v1/library/import", "/api/v1/rename", "/api/v1/library/calibre/conversions/refresh"]
    },
    {
      id: "download-clients",
      title: "Torrent and Usenet clients",
      status: "delegated",
      endpointCount: 16,
      message: "Librarry sends book releases to external clients and exposes scoped book-job controls. Full client administration stays in those applications.",
      examples: ["/api/v1/downloadclient", "/api/v1/downloads", "/api/v1/downloads/actions", "/api/v1/grabs"]
    }
  ]
};
