# Local Development

## Backend

```bash
go test ./...
LIBRARRY_DATABASE_URL=postgres://librarry:librarry@127.0.0.1:5432/librarry?sslmode=disable \
  go run ./backend/cmd/librarry
```

If `LIBRARRY_DATABASE_URL` is omitted, the API starts without persistence so
provider search and health endpoints can be developed independently.

## Frontend

```bash
cd web
npm install
npm run dev
```

The Vite dev server proxies `/api` to `http://127.0.0.1:8080`.

## Compose

```bash
cd deploy
cp .env.example .env
docker compose up --build
```

## TrueNAS Custom App

`deploy/truenas/install.yaml` is a paste-ready shape for TrueNAS custom apps,
but it intentionally contains placeholder secrets. Build `librarry-api:local`
and `librarry-web:local` on the TrueNAS host or replace the images with registry
tags before installing the custom app. Do not commit a real Prowlarr API key.

The default TrueNAS port is `192.168.1.221:30200`, with `/mnt/HDD_pool/vault/media-stack`
mounted into the API container as `/data`.

## Quality Profiles

Quality profiles are available through `GET /api/v1/quality-profiles` and can be
updated with `POST /api/v1/quality-profiles`. Default ebook and audiobook
profiles are seeded by migrations for `standard` and `large`.

The evaluator uses these rows for manual wanted searches, scheduled monitoring,
feed sync, failed-download recovery, and upgrade search.

## Wanted Monitor

The API can run wanted monitoring on an interval:

```dotenv
LIBRARRY_MONITOR_ENABLED=true
LIBRARRY_MONITOR_INTERVAL=30m
LIBRARRY_MONITOR_SEARCH_INTERVAL=6h
LIBRARRY_MONITOR_LIMIT=50
LIBRARRY_MONITOR_AUTO_GRAB=false
```

Manual monitor runs are available through `POST /api/v1/wanted/monitor`.
`LIBRARRY_MONITOR_AUTO_GRAB=false` keeps scheduled runs search-only while still
recording release decisions and history.

## Feed Sync

The API can also poll Prowlarr-compatible indexer RSS feeds on an interval:

```dotenv
LIBRARRY_FEED_SYNC_ENABLED=true
LIBRARRY_FEED_SYNC_INTERVAL=15m
LIBRARRY_FEED_SYNC_LIMIT=100
LIBRARRY_FEED_SYNC_AUTO_GRAB=false
```

Manual feed runs are available through `POST /api/v1/wanted/feed-sync`.
`LIBRARRY_FEED_SYNC_AUTO_GRAB=false` keeps scheduled feed runs search-only while
still recording feed releases, matched release decisions, and history.

## Upgrades

Upgrade search can run on an interval:

```dotenv
LIBRARRY_UPGRADE_SEARCH_ENABLED=true
LIBRARRY_UPGRADE_SEARCH_INTERVAL=12h
LIBRARRY_UPGRADE_SEARCH_LIMIT=50
LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB=false
LIBRARRY_UPGRADE_SEARCH_MIN_DELTA=5
```

Manual upgrade runs are available through `POST /api/v1/wanted/upgrades`.
Upgrade search compares grabbed/imported items against profile cutoffs and only
queues replacements when `autoGrab` is true.

## Failed Downloads

Failed-download recovery can run on an interval:

```dotenv
LIBRARRY_FAILED_DOWNLOAD_ENABLED=true
LIBRARRY_FAILED_DOWNLOAD_INTERVAL=30m
LIBRARRY_FAILED_DOWNLOAD_STALLED_AGE=24h
LIBRARRY_FAILED_DOWNLOAD_LIMIT=50
LIBRARRY_FAILED_DOWNLOAD_AUTO_GRAB=false
LIBRARRY_FAILED_DOWNLOAD_REMOVE=false
LIBRARRY_FAILED_DOWNLOAD_DELETE_FILES=false
```

Manual recovery runs are available through
`POST /api/v1/downloads/recover-failed`. The recovery path detects
qBittorrent error/missing-file states and stale stalled downloads with no
seeders, marks the linked wanted item wanted again, searches for replacements,
and only grabs or removes torrents when explicitly requested.

## Library Roots

Library scans and manual imports use these roots by default:

```dotenv
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
LIBRARRY_NAMING_AUTHOR_FOLDER={Author}
LIBRARRY_NAMING_BOOK_FOLDER={Title}
LIBRARRY_NAMING_FILE_NAME={Title}{Ext}
LIBRARRY_NAMING_SPACE_REPLACEMENT=
```

`POST /api/v1/library/scan` indexes existing files. `POST /api/v1/library/import`
copies or moves a single source file into the organized format root.
`POST /api/v1/library/import-completed` imports completed Librarry-tagged
qBittorrent downloads into the same organized roots when they are linked to a
wanted item. Unlinked completed downloads are queued in
`GET /api/v1/library/import-reviews` and resolved through
`POST /api/v1/library/import-reviews/{id}/resolve`.
