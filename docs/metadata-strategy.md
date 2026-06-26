# Metadata Strategy

Readarr's failure mode was metadata centralization. Librarry avoids that by
normalizing multiple providers into a local canonical model with provenance.

## Provider Order

1. Hardcover is the primary rich provider for modern book, edition, series,
   ebook, and audiobook metadata. It requires a backend-only token.
2. Open Library is the open-data backbone for works, authors, editions, ISBNs,
   and covers.
3. Google Books is an exact fallback for ISBN and title lookups. It should not be
   used for author bibliography crawling.
4. Local OPF sidecars, embedded EPUB package metadata, MP3 ID3 tags, and
   M4B/MP4 metadata atoms are high-confidence import evidence.
5. Manual overrides always win.

## Matching Policy

Exact identifiers beat fuzzy matching:

- ISBN, ASIN, and provider IDs are high-confidence matches.
- Title plus author identity can be medium-confidence.
- Ambiguous title-only matches go to manual review.
- Format selection is explicit: ebook and audiobook editions must not silently
  collapse into the same file target.

## Sources Not In Core

Goodreads, Amazon, and Audible scraping are intentionally excluded from core.
They can be future optional plugins only if the legal and operational risk is
made explicit.
