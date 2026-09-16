# Metadata Strategy

Readarr's failure mode was metadata centralization. Librarry avoids that by
normalizing multiple providers into a local canonical model with provenance.

## Provider Order

1. Hardcover is the primary rich provider for modern book, edition, series,
   ebook, and audiobook metadata. It requires a backend-only token.
2. Open Library is the open-data backbone for works, authors, editions, ISBNs,
   and covers. Author identity lookup uses the Open Library author search API,
   while monitoring verifies the author and traverses every works page. Hardcover
   monitoring likewise follows stable author IDs and paginated book contributions.
   Limited/name-only search results cannot establish complete author coverage.
3. Google Books runs after primary providers only when no suitable exact ISBN or
   full-title match exists. Returned matches are validated, including ISBN
   checksums/equivalence, title/subtitle, known format and language. It never runs
   for author, bibliography or series queries. See [provider setup](provider-setup.md)
   for exact lookup rules and limits.
4. Local OPF sidecars, embedded EPUB package metadata, MP3 ID3 tags, and
   M4B/MP4 metadata atoms are high-confidence import evidence.
5. Manual overrides always win.

## Matching Policy

Exact identifiers beat fuzzy matching:

- ISBN, ASIN, and provider IDs are high-confidence matches.
- Search results from different providers are clustered before ranking when
  they share an exact ISBN or compatible normalized title/author evidence.
  The primary result keeps the best provider identity while merged provider IDs,
  identifiers, dates, publisher fields, covers, and match evidence are retained
  as provenance.
- Title plus author identity can be medium-confidence.
- Author subscriptions require stable provider identities and match those IDs or
  explicit provider aliases. A matching name cannot override a different ID.
  Same-name author search results remain separate without common identity evidence.
- Original publication dates are separate from edition dates and drive whole-work
  first/latest policies when available. Non-writing or unknown Hardcover credits
  enter review before policies select eligible works.
- Ambiguous title-only matches go to manual review.
- Format selection is explicit: ebook and audiobook editions must not silently
  collapse into the same file target.

## Sources Not In Core

Goodreads, Amazon, and Audible scraping are intentionally excluded from core.
They can be future optional plugins only if the legal and operational risk is
made explicit.
