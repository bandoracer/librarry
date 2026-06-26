# Local Development

## Backend

```bash
go test ./...
LIBRARRY_DATABASE_URL=postgres://librarry:librarry@127.0.0.1:5432/librarry?sslmode=disable \
  go run ./backend/cmd/librarry
```

If `LIBRARRY_DATABASE_URL` is omitted, the API starts without persistence so
provider search and health endpoints can be developed independently.

Set `LIBRARRY_API_KEY` to protect the API with Readarr-style API-key auth. When
configured, `/api/` accepts `X-Api-Key`, `apikey`, `apiKey`, or bearer auth;
`/healthz` and `/ping` remain unauthenticated for local and container probes.
The web UI stores the key per browser from Settings.

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
Do not commit SABnzbd or download-client credentials either.

The default TrueNAS port is `192.168.1.221:30200`, with `/mnt/HDD_pool/vault/media-stack`
mounted into the API container as `/data`.

## Acquisition

Prowlarr provides release search. qBittorrent or Transmission handles torrent
releases, and SABnzbd handles Usenet/NZB releases.

```dotenv
LIBRARRY_PROWLARR_URL=
LIBRARRY_PROWLARR_API_KEY=
LIBRARRY_QBITTORRENT_URL=
LIBRARRY_QBITTORRENT_USERNAME=
LIBRARRY_QBITTORRENT_PASSWORD=
LIBRARRY_TRANSMISSION_URL=
LIBRARRY_TRANSMISSION_USERNAME=
LIBRARRY_TRANSMISSION_PASSWORD=
LIBRARRY_SABNZBD_URL=
LIBRARRY_SABNZBD_API_KEY=
LIBRARRY_SABNZBD_USERNAME=
LIBRARRY_SABNZBD_PASSWORD=
```

`LIBRARRY_QBITTORRENT_URL` or `LIBRARRY_TRANSMISSION_URL` is enough for trusted
LAN deployments with download-client auth disabled for the calling host.
SABnzbd requires both URL and API key; username/password are only needed when
SABnzbd itself is protected by basic auth.

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

## Author Monitor

The API can refresh monitored authors on an interval:

```dotenv
LIBRARRY_AUTHOR_MONITOR_ENABLED=true
LIBRARRY_AUTHOR_MONITOR_INTERVAL=6h
LIBRARRY_AUTHOR_MONITOR_SYNC_INTERVAL=24h
LIBRARRY_AUTHOR_MONITOR_LIMIT=50
```

Author subscriptions are available through `GET /api/v1/authors` and can be
created with `POST /api/v1/authors` from a normalized metadata result. Manual
author refreshes are available through `POST /api/v1/authors/monitor`. Author
monitoring only creates or refreshes wanted items; it does not grab releases.

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

## Import Lists

Readarr-compatible import lists are available through `/api/v1/importlist`.
Persisted, enabled `ReadarrImportList` resources can be synced with:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/command \
  -H 'Content-Type: application/json' \
  -d '{"name":"ImportListSync"}'
```

Librarry supports inline list entries in `books`, `items`, `entries`, `titles`,
`queries`, `isbns`, or `fields[].value`. Entries are resolved through metadata
search when providers are configured; otherwise deterministic title/author
records are created so reruns remain idempotent. Import-list exclusions from
`/api/v1/importlistexclusion` are respected before wanted items are created.

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

The download queue UI uses `POST /api/v1/downloads/actions` for single and
selected-row bulk actions. Supported queue actions include start, stop, delete,
recheck, priority movement, force-start, sequential-download toggle,
first/last-piece priority toggle, rename, tag add/remove, category changes, and
location changes for qBittorrent, plus per-torrent download and upload speed
limits. Transmission supports start, stop, delete, recheck, queue movement,
force-start, set location, label-backed category/tag changes, tracker
add/edit/remove, per-torrent speed limits, detail inspection, file-priority
changes, and label-derived category/tag resources. `GET
/api/v1/downloads/resources` reads qBittorrent categories/tags or Transmission
labels depending on the selected `client`. `POST
/api/v1/downloads/categories/actions` creates, updates, or deletes qBittorrent
categories and can rename/delete Transmission category labels.
`POST /api/v1/downloads/tags/actions` creates or deletes qBittorrent tags and
can rename/delete Transmission labels.
`GET` and `PUT /api/v1/downloads/preferences` read and write qBittorrent global
save-path, temp-path, speed-limit, scheduler, paused-add, and queue-cap
preferences.
`GET /api/v1/downloads/{id}` returns qBittorrent and Transmission properties,
files, trackers, and peers; `/api/v1/downloads/{id}/files/actions` changes file
priority for qBittorrent and Transmission, and
`/api/v1/downloads/{id}/trackers/actions` adds, replaces, or removes
qBittorrent or Transmission trackers. SABnzbd supports queue/history detail lookup, queued-file
inspection through `get_files`, and start, stop, delete, rename, category, and
priority actions. This is still a book-acquisition manager, not a full
qBittorrent UI replacement.

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
imports a single source file into the organized format root using `importMode`
values of `copy`, `move`, `hardlink`, or `hardlinkOrCopy`.
`conflictAction` controls duplicate destinations with `rename`/keep-both,
`replace`/overwrite, `skip`, or `fail`; `overwrite=true` is treated as
`replace`.
`POST /api/v1/library/import-completed` imports completed Librarry-tagged
downloads into the same organized roots when they are linked to a
wanted item. Unlinked completed downloads are queued in
`GET /api/v1/library/import-reviews` and resolved through
`POST /api/v1/library/import-reviews/{id}/resolve` or
`POST /api/v1/library/import-reviews/resolve-bulk` with the same import mode and
conflict policy fields. OPF sidecars and embedded
EPUB package metadata plus MP3 ID3 and M4B/MP4 audio tags are extracted during
scan/import and used for title, author, identifiers, language, publisher,
series, album, year, and track evidence before falling back to filename parsing.
Readarr-compatible `/api/v1/retag` previews and applies title, author, language,
and quality tag state on tracked file records for compatibility clients; it does
not rewrite embedded EPUB, MP3, or M4B metadata yet.

If the destination path is inside a Readarr-compatible root folder with
`isCalibreLibrary=true`, Librarry also posts the imported file to the configured
Calibre Content Server add-book endpoint and stores the returned Calibre ID on
the file metadata. It then pushes basic title, author, and identifier metadata
through the Content Server set-fields endpoint. If `outputFormat` is configured
on the root folder, Librarry starts Calibre conversion jobs for missing target
formats and captures an immediate status snapshot. Stored conversion jobs can be
refreshed with `POST /api/v1/library/calibre/conversions/refresh` or the
Readarr-compatible `RefreshCalibreConversions` command. Physical deletion of a
Calibre-backed file calls the Content Server delete-books endpoint before
removing the local file record. Scheduled Calibre conversion completion
polling, richer edition metadata, embedded metadata writes, path refresh after
Calibre renames, and failed-import rollback are not implemented yet.
