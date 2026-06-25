import type { ProviderHealth, SearchResult } from "./api";

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
      isbns: ["9780593135204", "9780593135211"]
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
      coverUrl: "https://covers.openlibrary.org/b/id/15143022-L.jpg"
    },
    edition: {
      id: "openlibrary:OL32618729M",
      title: "Dungeon Crawler Carl",
      format: "audiobook",
      language: "eng",
      isbns: ["9798217287161"]
    },
    score: 0.86,
    confidence: "medium",
    matchedOn: ["title"]
  }
];

