# Architecture

Librarry is split into a Go backend, a React frontend, and Postgres.

## Backend

The backend owns provider credentials, metadata normalization, provider health,
matching policy, future acquisition workers, and Postgres persistence. Provider
tokens never need to be exposed to the browser.

Initial API surface:

- `GET /healthz`
- `GET /api/v1/providers/health`
- `GET /api/v1/providers/diagnostics`
- `GET /api/v1/search?query=&type=book&format=any`
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

- `books-ebook`
- `books-audiobook`

Default roots are:

- `/data/media/books/ebooks`
- `/data/media/books/audiobooks`
- `/data/torrents/books`
