import type {
  DownloadDetails,
  DownloadPreferences,
  DownloadResources,
  DownloadStatus,
  IntegrationHealth,
  MetadataProvenance,
  MetadataReviewQueue,
  ProviderHealth,
  ReadarrCompatibilityReport,
  SearchResult,
  WantedItem
} from "./api";

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

export const seedIntegrations: IntegrationHealth[] = [
  {
    name: "Prowlarr",
    configured: true,
    status: "ready",
    message: "Demo mode shows book release search handoff without a live Prowlarr URL."
  },
  {
    name: "qBittorrent",
    configured: true,
    status: "ready",
    message: "Demo mode shows scoped book queue controls; live actions require client settings."
  },
  {
    name: "Transmission",
    configured: true,
    status: "ready",
    message: "Demo mode shows torrent queue visibility for external Transmission jobs."
  },
  {
    name: "SABnzbd",
    configured: true,
    status: "ready",
    message: "Demo mode shows NZB queue visibility for external SABnzbd jobs."
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

const seedFetchedAt = new Date().toISOString();
const seedProjectHailMary = seedWantedItems[0];

export const seedDownloads: DownloadStatus[] = [
  {
    client: "qBittorrent",
    id: "demo-qbit-project-hail-mary",
    name: "Project Hail Mary - Andy Weir [ebook]",
    state: "downloading",
    progress: 0.64,
    savePath: "/data/torrents/books/ebooks",
    category: "books-ebook",
    tags: ["librarry", "wanted:demo-wanted-project-hail-mary", "ebook"],
    sizeBytes: 837_812_224,
    downloadedBytes: 536_199_823,
    uploadedBytes: 91_225_088,
    downloadRate: 2_359_296,
    uploadRate: 121_856,
    etaSeconds: 128,
    ratio: 0.17,
    seeders: 44,
    peers: 9,
    addedAt: seedFetchedAt,
    lastActivityAt: seedFetchedAt,
    lastSeenAt: seedFetchedAt,
    importStatus: "pending"
  },
  {
    client: "Transmission",
    id: "demo-transmission-dungeon-crawler-carl",
    name: "Dungeon Crawler Carl - Matt Dinniman [audiobook]",
    state: "seeding",
    progress: 1,
    savePath: "/data/torrents/books/audiobooks",
    category: "books-audiobook",
    tags: ["librarry", "audiobook", "review"],
    sizeBytes: 1_879_048_192,
    downloadedBytes: 1_879_048_192,
    uploadedBytes: 2_421_669_888,
    downloadRate: 0,
    uploadRate: 283_648,
    etaSeconds: 0,
    ratio: 1.29,
    seeders: 71,
    peers: 12,
    addedAt: seedFetchedAt,
    completedAt: seedFetchedAt,
    lastActivityAt: seedFetchedAt,
    lastSeenAt: seedFetchedAt,
    importStatus: "ready"
  },
  {
    client: "SABnzbd",
    id: "demo-sab-project-hail-mary-audio",
    name: "Project Hail Mary - Andy Weir [audiobook]",
    state: "failed",
    progress: 0.12,
    savePath: "/data/torrents/books/audiobooks",
    category: "books-audiobook",
    tags: ["librarry", "audiobook"],
    sizeBytes: 3_221_225_472,
    downloadedBytes: 386_547_056,
    uploadedBytes: 0,
    downloadRate: 0,
    uploadRate: 0,
    etaSeconds: 0,
    ratio: 0,
    seeders: 0,
    peers: 0,
    addedAt: seedFetchedAt,
    lastSeenAt: seedFetchedAt,
    importStatus: "error",
    importError: "Demo failed job waiting for replacement search",
    failureReason: "Indexer release no longer has enough complete sources",
    failedAt: seedFetchedAt,
    retryCount: 1
  }
];

export const seedDownloadResourcesByClient: Record<string, DownloadResources> = {
  qBittorrent: {
    client: "qBittorrent",
    categories: [
      { name: "books-ebook", savePath: "/data/torrents/books/ebooks" },
      { name: "books-audiobook", savePath: "/data/torrents/books/audiobooks" }
    ],
    tags: ["librarry", "ebook", "audiobook", "manual", "review"]
  },
  Transmission: {
    client: "Transmission",
    categories: [
      { name: "books-ebook", savePath: "/data/torrents/books/ebooks" },
      { name: "books-audiobook", savePath: "/data/torrents/books/audiobooks" }
    ],
    tags: ["librarry", "ebook", "audiobook", "review"]
  },
  SABnzbd: {
    client: "SABnzbd",
    categories: [
      { name: "books-ebook", savePath: "/data/usenet/books/ebooks" },
      { name: "books-audiobook", savePath: "/data/usenet/books/audiobooks" }
    ],
    tags: []
  }
};

export const seedDownloadPreferencesByClient: Record<string, DownloadPreferences> = {
  qBittorrent: {
    client: "qBittorrent",
    savePath: "/data/torrents/books",
    tempPathEnabled: true,
    tempPath: "/data/torrents/incomplete",
    startPaused: true,
    downloadLimit: 0,
    uploadLimit: 1_048_576,
    alternativeDownloadLimit: 2_097_152,
    alternativeUploadLimit: 262_144,
    speedScheduleEnabled: false,
    queueingEnabled: true,
    maxActiveDownloads: 3,
    maxActiveUploads: 4,
    maxActiveTorrents: 8,
    librarryPreferenceWriteScope: "demo"
  },
  Transmission: {
    client: "Transmission",
    savePath: "/data/torrents/books",
    startPaused: true,
    downloadLimit: 0,
    uploadLimit: 1_048_576,
    speedScheduleEnabled: false,
    queueingEnabled: true,
    maxActiveDownloads: 3,
    maxActiveUploads: 4,
    maxActiveTorrents: 7,
    librarryPreferenceWriteScope: "demo"
  }
};

export const seedDownloadDetailsByKey: Record<string, DownloadDetails> = {
  "qBittorrent:demo-qbit-project-hail-mary": {
    status: seedDownloads[0],
    properties: {
      savePath: "/data/torrents/books/ebooks",
      additionDate: seedFetchedAt,
      totalSizeBytes: seedDownloads[0].sizeBytes,
      totalDownloaded: seedDownloads[0].downloadedBytes,
      totalUploaded: seedDownloads[0].uploadedBytes,
      downloadLimit: 0,
      uploadLimit: 1_048_576,
      downloadSpeed: seedDownloads[0].downloadRate,
      uploadSpeed: seedDownloads[0].uploadRate,
      etaSeconds: seedDownloads[0].etaSeconds,
      ratio: seedDownloads[0].ratio,
      connections: 18,
      connectionsLimit: 100,
      pieceSizeBytes: 4_194_304,
      piecesHave: 128,
      piecesTotal: 200,
      reannounceSeconds: 942,
      createdBy: "qBittorrent v4.6",
      comment: "Demo book acquisition job"
    },
    files: [
      {
        id: 0,
        name: "Project Hail Mary/Project Hail Mary - Andy Weir.epub",
        sizeBytes: 18_874_368,
        progress: 1,
        priority: 6,
        availability: 12.4
      },
      {
        id: 1,
        name: "Project Hail Mary/cover.jpg",
        sizeBytes: 1_048_576,
        progress: 1,
        priority: 1,
        availability: 12.4
      },
      {
        id: 2,
        name: "Project Hail Mary/sample.pdf",
        sizeBytes: 817_889_280,
        progress: 0.63,
        priority: 0,
        availability: 4.1
      }
    ],
    trackers: [
      {
        url: "https://tracker.example/announce",
        statusCode: 2,
        status: "working",
        tier: 0,
        message: "Announce OK",
        peers: 53,
        seeds: 44,
        leeches: 9,
        downloads: 128
      }
    ],
    peers: [
      {
        id: "demo-peer-1",
        ip: "198.51.100.24",
        port: 51413,
        client: "qBittorrent",
        country: "US",
        progress: 1,
        downloadRate: 1_572_864,
        uploadRate: 24_576,
        files: "Project Hail Mary.epub"
      },
      {
        id: "demo-peer-2",
        ip: "203.0.113.9",
        port: 6881,
        client: "Transmission",
        country: "CA",
        progress: 0.81,
        downloadRate: 786_432,
        uploadRate: 97_280,
        files: "Project Hail Mary.epub"
      }
    ]
  },
  "Transmission:demo-transmission-dungeon-crawler-carl": {
    status: seedDownloads[1],
    properties: {
      savePath: "/data/torrents/books/audiobooks",
      additionDate: seedFetchedAt,
      completionDate: seedFetchedAt,
      totalSizeBytes: seedDownloads[1].sizeBytes,
      totalDownloaded: seedDownloads[1].downloadedBytes,
      totalUploaded: seedDownloads[1].uploadedBytes,
      uploadSpeed: seedDownloads[1].uploadRate,
      ratio: seedDownloads[1].ratio,
      connections: 12,
      connectionsLimit: 80,
      piecesHave: 448,
      piecesTotal: 448,
      createdBy: "Transmission 4.0"
    },
    files: [
      {
        id: 0,
        name: "Dungeon Crawler Carl/Dungeon Crawler Carl.m4b",
        sizeBytes: 1_876_951_040,
        progress: 1,
        priority: 1,
        availability: 9.8
      },
      {
        id: 1,
        name: "Dungeon Crawler Carl/cover.jpg",
        sizeBytes: 2_097_152,
        progress: 1,
        priority: 1,
        availability: 9.8
      }
    ],
    trackers: [
      {
        url: "https://tracker.example/announce",
        statusCode: 2,
        status: "working",
        tier: 0,
        message: "Seeding",
        peers: 83,
        seeds: 71,
        leeches: 12,
        downloads: 311
      }
    ],
    peers: []
  },
  "SABnzbd:demo-sab-project-hail-mary-audio": {
    status: seedDownloads[2],
    properties: {
      savePath: "/data/usenet/books/audiobooks",
      additionDate: seedFetchedAt,
      totalSizeBytes: seedDownloads[2].sizeBytes,
      totalDownloaded: seedDownloads[2].downloadedBytes,
      downloadSpeed: 0,
      etaSeconds: 0,
      connections: 0,
      connectionsLimit: 12
    },
    files: [
      {
        id: 0,
        externalId: "demo-sab-file-1",
        name: "Project Hail Mary.part01.rar",
        sizeBytes: 104_857_600,
        progress: 1,
        priority: 1
      },
      {
        id: 1,
        externalId: "demo-sab-file-2",
        name: "Project Hail Mary.part02.rar",
        sizeBytes: 104_857_600,
        progress: 0.34,
        priority: 1
      }
    ],
    trackers: [],
    peers: []
  }
};

export const seedWantedMetadataByID: Record<string, MetadataProvenance> = {
  [seedProjectHailMary.id]: {
    wantedItem: seedProjectHailMary,
    generatedAt: seedFetchedAt,
    manualOverrides: [],
    fields: [
      {
        fieldName: "title",
        label: "Title",
        canonicalValue: "Project Hail Mary",
        canonicalSource: "wanted",
        protected: false,
        reviewResolved: false,
        conflict: true,
        candidates: [
          {
            provider: "Hardcover",
            providerKey: "hardcover:project-hail-mary",
            entityType: "work",
            value: "Project Hail Mary",
            confidence: 0.98,
            fetchedAt: seedFetchedAt,
            matchedOn: ["title", "author", "identifier"]
          },
          {
            provider: "Open Library",
            providerKey: "openlibrary:OL21745884W",
            entityType: "work",
            value: "Project Hail Mary: A Novel",
            confidence: 0.84,
            fetchedAt: seedFetchedAt,
            matchedOn: ["title"]
          }
        ]
      }
    ],
    records: [
      {
        id: "demo-provider-hardcover-project-hail-mary",
        provider: "Hardcover",
        providerKey: "hardcover:project-hail-mary",
        entityType: "work",
        confidence: 0.98,
        fetchedAt: seedFetchedAt,
        values: {
          title: "Project Hail Mary",
          authorName: "Andy Weir",
          format: "ebook",
          publisher: "Ballantine Books",
          publishedDate: "2021-05-04",
          isbns: ["9780593135204", "9780593135211"],
          matchedOn: ["title", "author", "identifier"],
          sourceKey: "hardcover:project-hail-mary"
        }
      },
      {
        id: "demo-provider-openlibrary-project-hail-mary",
        provider: "Open Library",
        providerKey: "openlibrary:OL21745884W",
        entityType: "work",
        confidence: 0.84,
        fetchedAt: seedFetchedAt,
        values: {
          title: "Project Hail Mary: A Novel",
          authorName: "Andy Weir",
          format: "ebook",
          publisher: "Ballantine Books",
          publishedDate: "2021-05-04",
          isbns: ["9780593135204", "9780593135211"],
          matchedOn: ["title"],
          sourceKey: "openlibrary:OL21745884W"
        }
      }
    ]
  }
};

export const seedWantedMetadataReview: MetadataReviewQueue = {
  generatedAt: seedFetchedAt,
  items: [
    {
      wantedItem: seedProjectHailMary,
      fields: seedWantedMetadataByID[seedProjectHailMary.id].fields,
      conflictCount: 1,
      protectedCount: 0,
      recordCount: 2,
      candidateCount: 2,
      lastFetchedAt: seedFetchedAt
    }
  ]
};

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
