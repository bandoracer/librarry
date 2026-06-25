# Architecture

Librarry is split into a Go backend, a React frontend, and Postgres.

## Backend

The backend owns provider credentials, metadata normalization, provider health,
matching policy, acquisition workers, and Postgres persistence. Provider tokens
never need to be exposed to the browser.

Initial API surface:

- `GET /healthz`
- `GET /api/v1/providers/health`
- `GET /api/v1/providers/diagnostics`
- `GET /api/v1/search?query=&type=book&format=any`
- `GET /api/v1/integrations/health`
- `POST /api/v1/integrations/bootstrap`
- `POST /api/v1/releases/search`
- `POST /api/v1/grabs`
- `GET /api/v1/downloads`
- `POST /api/v1/downloads/actions`
- `POST /api/v1/downloads/recover-failed`
- `GET /api/v1/wanted`
- `POST /api/v1/wanted`
- `POST /api/v1/wanted/{id}/search`
- `GET /api/v1/wanted/{id}/releases`
- `POST /api/v1/wanted/{id}/grab`
- `POST /api/v1/wanted/monitor`
- `POST /api/v1/wanted/feed-sync`
- `GET /api/v1/history`
- `GET /api/v1/library/files`
- `POST /api/v1/library/scan`
- `POST /api/v1/library/import`
- `POST /api/v1/library/import-completed`
- `POST /api/v1/settings/validate`

## Metadata Model

The data model separates conceptual works from concrete editions. This is the
main durability choice: a single book can have many ebook, audiobook, print, and
translated editions.

Manual overrides are stored separately and should always win over provider data.
Provider records keep raw provenance so future reconciliation can explain where
data came from and why a match was accepted or sent to review.

## Acquisition

Librarry integrates directly with Prowlarr and qBittorrent. Prowlarr is queried
for book releases through `/api/v1/releases/search`, and qBittorrent is used for
paused grabs through `/api/v1/grabs`. Startup and `/api/v1/integrations/bootstrap`
ensure the book categories exist in qBittorrent.

Download state is reconciled from qBittorrent through `/api/v1/downloads` and
stored in Postgres when database persistence is configured. Torrent actions are
exposed through `/api/v1/downloads/actions` for start, stop, delete, recheck,
and priority changes.

Failed-download recovery can be triggered manually through
`POST /api/v1/downloads/recover-failed` and can also run on an interval in the
API process. Recovery detects qBittorrent error/missing-file states and stale
stalled downloads with no seeders, reopens the linked wanted item, records the
failure on the download row, searches for replacement releases, and optionally
grabs the best approved replacement. Scheduled recovery defaults to search-only;
auto-grab and failed-torrent removal are explicit settings.

Wanted items are stored in Postgres from normalized metadata results. A wanted
item can search releases through Prowlarr, persist scored release decisions, and
record explicit rejection reasons before a candidate is sent to qBittorrent.

The wanted monitor can be triggered manually through
`POST /api/v1/wanted/monitor` and can also run on an interval in the API process.
It selects due wanted items, reuses the same release evaluator as manual search,
records monitor run summaries, writes history events, and optionally sends the
best approved release to qBittorrent when `LIBRARRY_MONITOR_AUTO_GRAB=true`.

Feed sync can be triggered manually through `POST /api/v1/wanted/feed-sync` and
can also run on an interval in the API process. Prowlarr does not provide a
single aggregate RSS endpoint, so Librarry lists RSS-enabled Prowlarr indexers,
pulls each Prowlarr-compatible Torznab/Newznab feed, stores seen releases, and
matches feed entries against wanted items with the same release evaluator used by
manual search. Feed sync defaults to search-only and only sends approved
releases to qBittorrent when `LIBRARRY_FEED_SYNC_AUTO_GRAB=true` or a manual
request sets `autoGrab`.

## Library Import

Library scanning walks the configured ebook and audiobook roots, classifies
supported file extensions, stores file records in Postgres, and keeps a
fingerprint in file metadata for later reconciliation. Manual import accepts a
source file path, optionally ties it to a wanted item, and copies or moves the
file into a sanitized `Author/Title/Title.ext` path below the format root.

Completed-download import refreshes Librarry-tagged qBittorrent items, filters
for completed torrents, locates the best supported ebook or audiobook file below
the torrent save path, imports that file, and records imported/error state on
the download row.

The implemented import path is intentionally conservative: it avoids overwriting
existing files and marks wanted items imported only after the destination file is
persisted. Future work should add OPF, EPUB, audio-tag extraction, richer rename
profiles, conflict policies, and review queues for ambiguous matches.

- `books-ebook`
- `books-audiobook`

Default roots are:

- `/data/media/books/ebooks`
- `/data/media/books/audiobooks`
- `/data/torrents/books`
