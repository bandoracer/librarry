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

## Source-Build Compose

The default [deploy/docker-compose.yml](../deploy/docker-compose.yml) is the
public install file and pulls published images. For local development from a
checkout, use the source-build compose file:

```bash
cd deploy
cp .env.example .env
docker compose -f docker-compose.build.yml up --build
```

## Deployment Templates

Public deployment instructions live in [deployment.md](deployment.md). The repo
includes install files for generic Docker Compose, TrueNAS Custom Apps, and
Unraid Docker Compose Manager.

Do not commit real Prowlarr API keys, SABnzbd credentials, download-client
credentials, provider tokens, or database passwords.

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

These environment variables are startup defaults. With Postgres enabled, the
Settings page can save Prowlarr, qBittorrent, Transmission, and SABnzbd
connection details through `PUT /api/v1/integrations/config`; those records are
stored in `compat_resources`, applied to the running acquisition service
immediately, and reloaded on restart.

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
LIBRARRY_MONITOR_AUTO_GRAB=true
```

Manual monitor runs are available through `POST /api/v1/wanted/monitor`.
Scheduled runs auto-grab the best approved release by default (arr parity,
owner decision 2026-07-01); the blocklist keeps failed releases from being
re-grabbed. Set `LIBRARRY_MONITOR_AUTO_GRAB=false` to restore search-only
review-first runs that still record release decisions and history.

Manual wanted search uses `POST /api/v1/wanted/{id}/search` to persist scored
release decisions. The web UI reloads those decisions with
`GET /api/v1/wanted/releases/{id}` when a wanted item is selected, so operators
can review previous approved and rejected candidates without rerunning indexer
search. Native wanted grabs still reject failed policy decisions by default; a
manual request can pass `force: true` to `POST /api/v1/wanted/{id}/grab` to send
a selected rejected release to the download client and record that override in
history.

## Author Monitor

The API can refresh monitored authors on an interval:

```dotenv
LIBRARRY_AUTHOR_MONITOR_ENABLED=true
LIBRARRY_AUTHOR_MONITOR_INTERVAL=6h
LIBRARRY_AUTHOR_MONITOR_SYNC_INTERVAL=24h
LIBRARRY_AUTHOR_MONITOR_LIMIT=50
```

Author subscriptions are available through `GET /api/v1/authors` and can be
created with `POST /api/v1/authors` from a normalized metadata result. The web
Search view can switch between book candidates and author identities; author
mode is the preferred path for monitoring an author before choosing individual
books. The web app runs a targeted author monitor after saving a subscription,
and each author row has a refresh action that sends that author's subscription
ID/provider key to `POST /api/v1/authors/monitor`. Set
`missingBookPolicy` to `all`, `future`, `none`, `missing`, `existing`,
`first`, or `latest` to control which discovered books become wanted items:
`all` backfills the visible bibliography, `future` only takes books published
after the subscription, `none` only syncs author metadata, `missing` takes
books without a tracked library file, `existing` takes books with a library
file plus future releases, `first` only takes the earliest discovered book,
and `latest` takes the most recent book plus future releases. Existing subscriptions can
be changed with `PATCH /api/v1/authors/{id}` and soft-removed with
`DELETE /api/v1/authors/{id}`. Manual author refreshes are
available through `POST /api/v1/authors/monitor`; monitor results report
metadata hits, wanted items created, and entries skipped by policy. For Open
Library-backed subscriptions, the monitor uses the stored author ID against the
works-by-author endpoint before falling back to a name-based book search. Skipped
entries are persisted in `author_metadata_reviews` and include the normalized
metadata result and skip reason so the web UI can review them and mark
individual books wanted without changing the author policy. Use
`GET /api/v1/authors/metadata/review` to inspect pending candidates and
`POST /api/v1/authors/metadata/review/{id}/resolve` with `{"action":"wanted"}`
or `{"action":"ignore"}` to resolve one. Author monitoring does not grab
releases.

### Author Add-Filters

Each subscription carries optional metadata filters that run before the
missing-book policy, so noisy candidates never reach the review queue as
actionable rows — they become skipped entries with explicit reasons instead
(`language-filtered`, `term-filtered`, `missing-isbn`, `below-min-pages`):

- `allowedLanguages` (string list): when set, candidates with a known,
  non-matching edition language are skipped. Unknown languages pass.
- `mustNotContain` (string list): candidates whose title contains any term are
  skipped.
- `skipMissingIsbn` (bool): candidates without an ISBN are skipped.
- `minPages` (int): candidates whose provider-supplied page count is below the
  minimum are skipped. Unknown page counts pass.

The fields appear on `AuthorSubscription` JSON and are accepted in both the
subscribe (`POST /api/v1/authors`) and update (`PATCH /api/v1/authors/{id}`)
payloads.

## Feed Sync

The API can also poll Prowlarr-compatible indexer RSS feeds on an interval:

```dotenv
LIBRARRY_FEED_SYNC_ENABLED=true
LIBRARRY_FEED_SYNC_INTERVAL=15m
LIBRARRY_FEED_SYNC_LIMIT=100
LIBRARRY_FEED_SYNC_AUTO_GRAB=true
```

Manual feed runs are available through `POST /api/v1/wanted/feed-sync`.
Scheduled feed runs auto-grab approved matches by default (arr parity, owner
decision 2026-07-01). Set `LIBRARRY_FEED_SYNC_AUTO_GRAB=false` to keep feed
runs search-only while still recording feed releases, matched release
decisions, and history.

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
`/api/v1/importlistexclusion` are respected before wanted items are created,
and the native exclusions below apply to compat syncs too.

### Native Import Lists (Hardcover)

Native import lists sync a Hardcover list/shelf into wanted items. They need
`LIBRARRY_HARDCOVER_TOKEN` and store `settings.listId` (the numeric Hardcover
list id; optional `settings.format` picks `ebook`/`audiobook`, default ebook).
Each list carries auto-add options: `monitor` (`all`|`none`), `qualityProfile`,
`rootFolderId`, and `searchOnAdd` (search only — review-first stays intact,
nothing is grabbed).

- `GET/POST /api/v1/import-lists`, `PUT/DELETE /api/v1/import-lists/{id}`
- `POST /api/v1/import-lists/{id}/sync` runs one list now and returns the
  outcome (created / already tracked / excluded / errors per entry)
- `GET/POST /api/v1/import-lists/exclusions`,
  `DELETE /api/v1/import-lists/exclusions/{id}` — exclusions match by source
  key (`hardcover:<id>`) or title (+ optional author) and suppress entries in
  both native and compat syncs

The scheduled `import-list-sync` task syncs every enabled list:

```dotenv
LIBRARRY_IMPORT_LIST_SYNC_INTERVAL=24h
```

Entries dedupe against already-tracked books by provider identity
(provider + source key + format), so re-syncs are idempotent.

## Upgrades

Upgrade search can run on an interval:

```dotenv
LIBRARRY_UPGRADE_SEARCH_ENABLED=true
LIBRARRY_UPGRADE_SEARCH_INTERVAL=12h
LIBRARRY_UPGRADE_SEARCH_LIMIT=50
LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB=true
LIBRARRY_UPGRADE_SEARCH_MIN_DELTA=5
```

Manual upgrade runs are available through `POST /api/v1/wanted/upgrades`.
Upgrade search compares grabbed/imported items against profile cutoffs and
grabs qualifying replacements by default (arr parity, owner decision
2026-07-01). Set `LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB=false` for search-only
runs. `GET /api/v1/wanted?view=cutoff-unmet` lists items with a tracked
library file whose current release score is still below the profile cutoff.

## Failed Downloads

Failed-download recovery can run on an interval:

```dotenv
LIBRARRY_FAILED_DOWNLOAD_ENABLED=true
LIBRARRY_FAILED_DOWNLOAD_INTERVAL=30m
LIBRARRY_FAILED_DOWNLOAD_STALLED_AGE=24h
LIBRARRY_FAILED_DOWNLOAD_LIMIT=50
LIBRARRY_FAILED_DOWNLOAD_AUTO_GRAB=true
LIBRARRY_FAILED_DOWNLOAD_REMOVE=true
LIBRARRY_FAILED_DOWNLOAD_DELETE_FILES=false
```

Manual recovery runs are available through
`POST /api/v1/downloads/recover-failed`. The recovery path detects
qBittorrent error/missing-file states and stale stalled downloads with no
seeders, marks the linked wanted item wanted again, blocklists the failed
release identity (source `auto-failed`) so evaluation rejects it with reason
`blocklisted`, searches for replacements, and — by default (arr parity, owner
decision 2026-07-01) — grabs the best approved replacement and removes the
failed download from the client. Set the auto-grab/remove flags to `false` to
restore review-first recovery.

## Blocklist

The blocklist stores release identities (infohash, download URL hash, or
title+indexer) that release evaluation must reject:

- `GET /api/v1/librarry/blocklist?limit=` returns `{"items":[...]}`.
- `POST /api/v1/librarry/blocklist` with `{"downloadId","client","reason"}`
  blocklists the release behind a queue download (source `queue-remove`).
- `DELETE /api/v1/librarry/blocklist/{id}` removes one entry.
- `POST /api/v1/librarry/blocklist/clear` with `{"ids":[...]}` removes the
  listed entries; empty ids clears everything. Returns `{"removed":N}`.
- `POST /api/v1/downloads/mark-failed` with
  `{"id","client","blocklist":bool,"research":bool}` marks a download failed,
  optionally blocklists it (source `history-mark-failed`), and optionally
  re-searches the linked wanted item. Returns
  `{"blocklisted":bool,"searchTriggered":bool}`.

The Readarr-compatible `GET /api/v1/blocklist` and its delete routes are
backed by the same table.

## Calibre Conversion Refresh

Calibre conversion status polling can run on an interval when database
persistence and Calibre root-folder settings are available:

```dotenv
LIBRARRY_CALIBRE_REFRESH_ENABLED=true
LIBRARRY_CALIBRE_REFRESH_INTERVAL=15m
LIBRARRY_CALIBRE_REFRESH_LIMIT=200
LIBRARRY_CALIBRE_REFRESH_MAX_ATTEMPTS=1
```

Manual refreshes are available through
`POST /api/v1/library/calibre/conversions/refresh` or the Readarr-compatible
`RefreshCalibreConversions` command. Scheduled refreshes poll stored conversion
jobs and update completed or failed conversion metadata; they do not start new
conversion jobs.

## Completed Download Handling

Completed librarry-tagged downloads import automatically, matching arr
completed-download handling. Downloads linked to a wanted item import
directly; unlinked files are auto-matched against wanted metadata and fall
back to the import review queue. The worker requires database persistence and
runs every minute by default:

```dotenv
LIBRARRY_COMPLETED_IMPORT_ENABLED=true
LIBRARRY_COMPLETED_IMPORT_INTERVAL=1m
LIBRARRY_COMPLETED_IMPORT_LIMIT=50
LIBRARRY_COMPLETED_IMPORT_MODE=hardlinkOrCopy
LIBRARRY_COMPLETED_REMOVE_ENABLED=true
```

`hardlinkOrCopy` keeps torrents seeding without duplicating disk space when
the torrent root and library roots share a filesystem, and falls back to copy
when they do not. Set the mode to `copy` or `move` to override. The manual
"Import Completed" action in the Activity queue remains available for
immediate imports.

`LIBRARRY_COMPLETED_REMOVE_ENABLED=true` (arr parity, default on) deletes an
imported download — with its data — once the client reports seeding has
finished (qBittorrent `stoppedUP`/`pausedUP`, Transmission stopped and done).
Imports use hardlink-or-copy, so the library copy survives. Set it to `false`
to leave finished torrents in the client.

## Downloads

The download queue UI uses `POST /api/v1/downloads/actions` for single and
selected-row bulk actions. Supported queue actions include start, stop, delete,
recheck, priority movement, force-start, sequential-download toggle,
first/last-piece priority toggle, rename, tag add/remove, category changes, and
location changes for qBittorrent, plus per-torrent download and upload speed
limits. Transmission supports start, stop, delete, recheck, queue movement,
force-start, set location, label-backed category/tag changes, tracker
add/edit/remove, per-torrent speed limits, detail inspection, file-priority
changes, and label-derived category/tag resources. `GET
/api/v1/downloads/resources` reads qBittorrent categories/tags, Transmission
labels, or SABnzbd categories depending on the selected `client`. `POST
/api/v1/downloads/categories/actions` creates, updates, or deletes qBittorrent
categories, creates/updates/deletes SABnzbd categories, and can rename/delete
Transmission category labels.
`POST /api/v1/downloads/tags/actions` creates or deletes qBittorrent tags and
can rename/delete Transmission labels.
`GET` and `PUT /api/v1/downloads/preferences` read and write qBittorrent and
Transmission global save-path, temp or incomplete path, speed-limit, scheduler,
paused-add, and queue-cap preferences.
`GET /api/v1/downloads/{id}` returns qBittorrent and Transmission properties,
files, trackers, and peers; `/api/v1/downloads/{id}/files/actions` changes file
priority for qBittorrent and Transmission, and
`/api/v1/downloads/{id}/trackers/actions` adds, replaces, or removes
qBittorrent or Transmission trackers. SABnzbd supports queue/history detail
lookup, queued-file inspection through `get_files`, category namespace
administration, and start, stop, delete, rename, category, and priority actions.
This is still a book-acquisition manager, not a full
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
LIBRARRY_STANDARD_SEARCH_LANGUAGE=English
LIBRARRY_RECYCLE_BIN=
LIBRARRY_RECYCLE_BIN_RETENTION=168h
LIBRARRY_IMPORT_EXTRA_FILES=.cue
```

Naming templates accept `{Author}`, `{Title}`, `{Series}`, `{SeriesPosition}`,
`{Year}` (first published), `{Format}`, and `{Ext}`. Tokens with no value
collapse cleanly — separators and brackets that only decorated an empty token
are removed, so `{Series} #{SeriesPosition} - {Title}` renders as just the
title for standalone books. Series and year come from the wanted item's
provider metadata (manual overrides win) or embedded file metadata during
scans and renames.

### Multiple root folders

The two env roots above are the seed values for the native multi-root model.
`GET /api/v1/library/root-folders` lists root folders (auto-seeding "Ebooks"
and "Audiobooks" rows from the configured legacy roots the first time it runs
against an empty table) and returns `{"rootFolders": [...]}` with
`accessible` and `freeSpaceBytes` computed per path. `POST
/api/v1/library/root-folders` creates a root (`name`, `path`, `mediaFormat` of
`ebook`/`audiobook`, optional `defaultQualityProfile`,
`defaultMissingBookPolicy`, `defaultTags`, `isDefault`); `PUT
/api/v1/library/root-folders/{id}` updates one, and `DELETE
/api/v1/library/root-folders/{id}` removes one. Deleting the last root of a
format while tracked files still live under it is refused with `409` and a
reason. Marking a root `isDefault` clears the flag from other roots of the
same format.

Scans walk every root of the requested format. Imports pick the wanted item's
pinned root when set (`rootFolderId` on `PUT /api/v1/wanted/{id}`), then the
format's default root, then the legacy config roots. `GET`/`PUT
/api/v1/library/config` keep working: the legacy two-root fields map onto the
per-format default root folders.

### Remote path mappings

When a download client reports paths Librarry cannot reach (split hosts,
different Docker mounts), add a remote path mapping: `GET
/api/v1/library/remote-path-mappings` returns `{"mappings": [...]}`, with
`POST`, `PUT /{id}`, and `DELETE /{id}` for CRUD. Each mapping has `host`
(download client name, empty matches every client), `remotePrefix`, and
`localPrefix`. The longest matching remote prefix wins and is applied as a
dumb prefix rewrite before completed-download import reads the client's save
path.

### Recycle bin

Set `LIBRARRY_RECYCLE_BIN` to a folder to keep deleted or replaced library
files instead of removing them: files move into
`<bin>/<yyyy-mm-dd>/<original-name>` (falling back to copy+delete across
filesystems, and to a plain delete when the bin is unusable). Day folders
older than `LIBRARRY_RECYCLE_BIN_RETENTION` (default `168h`) are purged during
the completed-download-import worker tick. The active bin path is surfaced as
`recycleBin` in `GET /api/v1/library/config`.

### Import extras

`LIBRARRY_IMPORT_EXTRA_FILES` (comma-separated extensions, default `.cue`)
copies sibling files that share the imported source's basename into the
destination folder, renamed to match the organized file (audiobook cue sheets
survive imports). Extras are best-effort: they are not tracked and failures
only log at debug.

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
conflict policy fields. Review rows include confidence and evidence metadata
from filename parsing, OPF/embedded/audio tags, source file facts, and download
context so operators can see why manual review is required. OPF sidecars and embedded
EPUB package metadata plus MP3 ID3 and M4B/MP4 audio tags are extracted during
scan/import and used for title, author, identifiers, language, publisher,
series, album, year, and track evidence before falling back to filename parsing.
Readarr-compatible `/api/v1/retag` previews and applies title, author, language,
and quality tag state on tracked file records for compatibility clients; it does
not rewrite embedded EPUB, MP3, or M4B metadata yet.
`LIBRARRY_STANDARD_SEARCH_LANGUAGE` defaults provider metadata search, manual
release search, and wanted-item release search to English. Operators can change
the persisted preference from Settings; explicit wanted-item language overrides
still win for release scoring.

If the destination path is inside a Readarr-compatible root folder with
`isCalibreLibrary=true`, Librarry also posts the imported file to the configured
Calibre Content Server add-book endpoint and stores the returned Calibre ID on
the file metadata. It then pushes basic title, author, and identifier metadata
through the Content Server set-fields endpoint. If `outputFormat` is configured
on the root folder, Librarry starts Calibre conversion jobs for missing target
formats and captures an immediate status snapshot. Stored conversion jobs can be
refreshed with `POST /api/v1/library/calibre/conversions/refresh` or the
Readarr-compatible `RefreshCalibreConversions` command, and the API process can
poll those jobs on an interval with `LIBRARRY_CALIBRE_REFRESH_ENABLED=true`.
Physical deletion of a Calibre-backed file calls the Content Server
delete-books endpoint before removing the local file record. Richer edition
metadata, embedded metadata writes, path refresh after Calibre renames, and
failed-import rollback are not implemented yet.

## System Tasks

Every background worker (wanted monitor, author monitor, feed sync,
failed-download recovery, upgrade search, calibre refresh, completed-download
import, health check) runs through a scheduler registry that records interval,
last run, a one-line outcome summary, last error, and next run.

- `GET /api/v1/system/tasks` lists task status records:
  `{"tasks":[{id,name,interval,lastRunAt,lastOutcome,lastError,nextRunAt,running}]}`.
- `POST /api/v1/system/tasks/{id}/run` triggers a manual run and returns
  `202 {"started":true}`. While a run is in flight the endpoint returns
  `409 {"error":"task is running"}`; unknown ids return 404.

Task ids: `wanted-monitor`, `author-monitor`, `feed-sync`,
`failed-download-recovery`, `upgrade-search`, `calibre-refresh`,
`completed-import`, `health-check`. Workers disabled through their
`LIBRARRY_*_ENABLED` flags are not registered and do not appear in the list.

## Notifications

Native notification targets are persisted in `notification_targets` and fan
out grabs, imports, upgrades, download failures, and health issues from both
API-triggered actions and scheduled worker runs (the Readarr-compatible
`/api/v1/notification` webhook resources keep working in parallel).

- `GET /api/v1/notifications` →
  `{"targets":[{id,name,type,settings,triggers:{onGrab,onImport,onUpgrade,onDownloadFailure,onHealthIssue},enabled,createdAt}]}`.
- `POST /api/v1/notifications` creates a target (response `{"target":...}`),
  `PUT /api/v1/notifications/{id}` updates, `DELETE /api/v1/notifications/{id}`
  removes, and `POST /api/v1/notifications/{id}/test` sends a test event and
  returns `{"ok":bool,"error"?}`.

Provider types and their settings keys:

- `webhook`: `url` (required), optional `authorization` header value. Sends an
  arr-ish JSON payload with `eventType`, `title`, `message`, and `fields`.
- `ntfy`: `url` and/or `topic` (server defaults to `https://ntfy.sh`),
  optional `token` (Bearer) and `priority`. Message body plus `X-Title` header.
- `discord`: `webhookUrl` (required). Sends an embeds payload with
  severity-colored embeds and inline fields.
- `telegram`: `botToken` and `chatId` (required). Calls the Bot API
  `sendMessage` method.

Secrets: telegram `botToken` values are redacted to their last 4 characters in
GET responses, and a blank (or redacted) `botToken` on PUT keeps the stored
credential. Webhook/ntfy/discord URLs are operator-entered endpoints and are
returned as stored. Health-issue notifications are opt-in per target
(`triggers.onHealthIssue` defaults to false); all other triggers default on.
Delivery uses a 10 second timeout and failures are logged, never fatal.

## Health & Disk Space

- `GET /api/v1/system/health` evaluates continuous health checks and returns
  `{"checks":[{id,severity,name,message}]}` with severity `ok`, `warning`, or
  `error` for every evaluated rule: database persistence (warning when
  missing), indexer configured/reachable (error), download client
  configured/reachable (error), root folders present and accessible (error per
  root), completed-import enabled (warning when disabled), low disk per root
  filesystem (<1 GiB error, <5 GiB warning), and quality profiles present
  (warning). The same evaluator runs on the 5 minute `health-check` task, and
  checks that transition from ok to warning/error dispatch `healthIssue`
  notifications.
- `GET /api/v1/system/diskspace` returns
  `{"disks":[{path,label,freeBytes,totalBytes}]}` for every root folder plus
  the book torrent root, deduplicated by backing filesystem.

## Calendar & iCal

Wanted items persist a confident `releaseDate` (yyyy-mm-dd) when the metadata
result carries a full publication date; year-only books stay off the calendar.
Existing rows were backfilled from edition publish dates in migration 0024.

- `GET /api/v1/librarry/calendar?start=&end=&unmonitored=true|false` returns
  `{"items":[{wantedId,title,authorName,releaseDate,status,monitored,coverUrl}]}`.
  `start`/`end` accept RFC3339 or `yyyy-mm-dd`; the default window is the start
  of the current month minus 7 days through today plus 60 days. Unmonitored
  items are excluded unless `unmonitored=true`.
- `GET /feed/v1/calendar.ics?apikey=&pastDays=&futureDays=` serves all-day
  VEVENTs (UID = wanted id) for external calendar apps. `/feed/` bypasses
  session auth but requires the `apikey` query parameter whenever
  `LIBRARRY_API_KEY` is set (and stays blocked under forms/basic auth without
  an API key, since calendar apps cannot log in).
- The Readarr-compatible `GET /api/v1/calendar` serves the same real
  release-dated items.

## Authentication

Arr-parity in-app auth for the API (`none` default, `basic`, `forms`):

```dotenv
LIBRARRY_AUTH_METHOD=forms
LIBRARRY_AUTH_USERNAME=admin
LIBRARRY_AUTH_PASSWORD=change-me
```

`LIBRARRY_AUTH_USERNAME`/`LIBRARRY_AUTH_PASSWORD` seed or update the single
user row (bcrypt) at startup. API keys (`X-Api-Key`, `apikey`, bearer) keep
working for every method so Readarr-compatible clients never break. With
`none` + `LIBRARRY_API_KEY`, the pre-M6 behavior is unchanged (key required on
`/api/*`).

- `GET /api/v1/auth/status` → `{"method","authenticated","username"?}` (always
  reachable; the web UI gates on it)
- `POST /api/v1/login` `{"username","password","rememberMe"?}` → sets the
  HttpOnly `librarry_session` cookie (30 days with `rememberMe`, browser
  session otherwise); invalid credentials return 401
- `POST /api/v1/logout` clears the session
- `PUT /api/v1/auth/config` `{"method","username"?,"password"?}` switches the
  method at runtime (persisted across restarts; an explicit
  `LIBRARRY_AUTH_METHOD` env wins at boot). A blank password keeps the stored
  one.

Only the API enforces auth: the nginx-served static UI bundle remains publicly
reachable and the UI itself redirects to its sign-in screen based on
`auth/status`. Front Librarry with a reverse proxy (Cosmos/CF Access) if the
static assets themselves must be private.

## Tags

Native tags live in the `tags` table; wanted items and author subscriptions
store comma-separated tag labels. Renaming or deleting a tag rewrites the
label across both columns in one transaction, and labels written through
wanted/author endpoints are auto-registered in the tags table.

- `GET /api/v1/tags` → `{"tags":[{id,label,wantedCount,authorCount}]}` (`id`
  is a stable integer hash; counts aggregate by label)
- `POST /api/v1/tags` `{"label"}`, `PUT /api/v1/tags/{id}` `{"label"}`,
  `DELETE /api/v1/tags/{id}`
- Wanted and author update payloads accept `"tags": ["label", ...]` (legacy
  integer tag ids from compat clients are mapped back to labels)

## Backups

`pg_dump`-based database backups (custom format, restore with `pg_restore`):

```dotenv
LIBRARRY_BACKUP_ENABLED=true
LIBRARRY_BACKUP_INTERVAL=168h
LIBRARRY_BACKUP_RETENTION=4
LIBRARRY_BACKUP_DIR=/config/backups
```

- `POST /api/v1/librarry/backups` runs a backup now and returns
  `{"backup":{name,sizeBytes,createdAt}}`; installs without a database or
  without `pg_dump` answer `501`
- `GET /api/v1/librarry/backups` → `{"backups":[...]}` (also served on the
  compat `GET /api/v1/system/backup`)
- `DELETE /api/v1/librarry/backups/{name}` (names are sanitized to
  `librarry-YYYYMMDD-HHMMSS.dump` basenames)

The scheduled `backup` task creates a dump every interval and prunes to the
newest `LIBRARRY_BACKUP_RETENTION` files. The API image ships
`postgresql16-client`; mount `LIBRARRY_BACKUP_DIR` to keep dumps outside the
container. The database password travels to `pg_dump` via the child process
environment and is never logged.
