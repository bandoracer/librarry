# Provider Setup

## Hardcover

Set `LIBRARRY_HARDCOVER_TOKEN` on the backend. The token is server-side only and
is used for rich metadata search, editions, series, and format hints.

## Open Library

Open Library requires no token. Librarry uses it as the open-data backbone and
cover fallback.

## Google Books

Set `LIBRARRY_GOOGLE_BOOKS_API_KEY` to enable exact fallback lookups. Google
Books should not be used as the primary author/work graph because rate limiting
and author identity are not strong enough for that workflow.

## Local Metadata

Local OPF, EPUB, and audio tag parsing is planned for import workflows. The v1
provider is present as a health/diagnostic source and does not return remote
search results.

