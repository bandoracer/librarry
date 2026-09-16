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

## Reproducible verification

Use Go 1.26.8 and Node 22 (`mise install`). A Docker context with permission to
start disposable containers is required for the integration command:

```bash
scripts/test-integration.sh
scripts/check-deployment.sh
cd web
npm ci
npm test
npm run build
```

`test-integration.sh` starts and removes its own Postgres 16 container. Every Go
integration test creates a unique database and drops only that database. An
explicit `LIBRARRY_TEST_DATABASE_URL` can reuse a disposable server; production
`LIBRARRY_DATABASE_URL` is never used by the test harness. Without the test URL,
plain `go test` reports integration cases as skipped rather than qualified.

For browser tests, supply a disposable database URL, install Chromium, and run:

```bash
cd web
npx playwright install chromium
LIBRARRY_TEST_DATABASE_URL=postgres://postgres:librarry-test@127.0.0.1:15432/librarry_test?sslmode=disable npm run test:browser
```

The browser harness starts isolated API/Vite processes on 18182/15173 with all
acquisition automation disabled and no inherited provider/client credentials.
It applies migrations to the supplied disposable database. Browser artifacts go
under `output/playwright/`. CI runs Go vet/race/Postgres tests, frontend tests and
build, browser checks, and deployment configuration checks before building images.

After building `librarry-api:stabilization` and `librarry-web:stabilization`, run
`python3 scripts/test-packaged.py`. Set `DOCKER_CONTEXT` to select a test engine
and optionally `EXPECTED_COMMIT` to assert the packaged source SHA. The script
creates a unique network, Postgres database container, app containers and media
fixture directory. It verifies the web proxy, exact import, chapter sets, missing-payload
rejection, source retention, scan identity, retry and isolated database restore,
also exercises forms login, cookie/restart persistence and Basic authentication,
then removes only those resources. This is a fixture restore, not a backup of
production media. CI runs this qualification and scans candidate images before
allowing multi-platform publication. Fixable high/critical image findings fail
the qualification job; lower severity and module-only findings need separate review.

Malformed boolean, numeric and duration environment settings fail startup before
database/client work; an invalid auto-grab flag cannot silently become the default
`true`. Durations accept unit strings or the legacy integer-minute format and
must be positive. Completed import mode accepts hardlinkOrCopy, hardlink or copy.

Configured forms/basic authentication requires Postgres and a usable user at
startup. Unknown methods or an unavailable persisted auth setting are errors;
only explicit/default `none` starts open. UI auth changes commit credentials, session revocation, and method in one
transaction before changing enforcement. Credential changes revoke old sessions;
a login racing a password change cannot create a session using the old password.
Environment-owned auth methods and credentials are marked in Settings and cannot
be overwritten through the UI. Other settings still have their existing persisted precedence;
a unified source/precedence UI remains part of the stabilization plan.

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

Librarry uses Readarr's quality model: each quality profile carries an ordered
quality ladder (`qualities`, most-preferred first, each entry
`{"id","allowed"}`) plus a `cutoff` quality at which upgrade searching stops.
Known qualities are `azw3`, `epub`, `mobi`, `pdf`, `unknownText` for ebooks
and `flac`, `m4b`, `mp3`, `unknownAudio` for audiobooks. Profiles are
available through `GET /api/v1/quality-profiles` and saved with
`POST /api/v1/quality-profiles`; the JSON shape is
`{id,name,mediaFormat,qualities,cutoff,upgradeAllowed,minSeeders,createdAt,updatedAt}`
(legacy numeric score fields are still accepted and echoed for compatibility
readers but no longer drive evaluation). Default ebook and audiobook profiles
are seeded by migrations for `standard` and `large` with the full ladder
allowed and cutoffs of `epub` / `m4b`.

Release profiles carry required/ignored/preferred release terms
(`GET /api/v1/release-profiles` returns `{"profiles":[...]}`,
`POST /api/v1/release-profiles` returns `{"profile":...}`,
`PUT /api/v1/release-profiles/{id}`, `DELETE /api/v1/release-profiles/{id}`).
`required` and `ignored` are string arrays; `preferred` is an array of
`{"term","score"}`. Missing required terms and present ignored terms reject a
release; matched preferred term scores are summed into the release score.
Migration 0027 converts any legacy per-profile terms into one seeded
"Migrated terms" release profile.

Quality definitions set per-quality size windows
(`GET /api/v1/quality-definitions` returns
`{"definitions":[{quality,title,minSizeMB,maxSizeMB}]}`;
`PUT /api/v1/quality-definitions` accepts a bulk array or the same envelope;
`0` means unbounded). Releases outside the window are rejected with
`below minimum size for <quality>` / `above maximum size for <quality>`.

The evaluator uses these rows for manual wanted searches, scheduled monitoring,
feed sync, failed-download recovery, and upgrade search. Releases are ranked by
ladder position first, then summed preferred-word score, then seeders, and the
persisted score is the composite
`(ladderLen - ladderIndex) * 1000 + preferredScore`. Every rejection carries an
explainable reason (for example `quality not allowed: pdf`,
`missing required term: retail`, `ignored term: screener`).

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
`all` backfills the complete verified bibliography, `future` only takes books
whose original publication is on or after the subscription's UTC date, `none`
disables bibliography monitoring, `missing` takes
books without a tracked library file, `existing` takes books with a library
file plus future releases, `first` takes the unambiguously earliest dated book,
and `latest` takes the most recent published book plus books published since
the subscription cutoff. Future releases cannot displace the latest published
book. Original work dates/years take precedence over edition dates. Missing,
overlapping or insufficiently precise dates enter review when the policy needs
an ordering or cutoff that they cannot establish. Existing subscriptions can
be changed with `PATCH /api/v1/authors/{id}` and soft-removed with
`DELETE /api/v1/authors/{id}`. Manual author refreshes are
available through `POST /api/v1/authors/monitor`; monitor results report
metadata hits, wanted items created, and entries skipped by policy. For Open
Library-backed subscriptions, the monitor verifies the stored author ID and
traverses the works-by-author endpoint; failed or incomplete traversal never
falls back to an unverified name match. Skipped
entries are persisted in `author_metadata_reviews` and include the normalized
metadata result and skip reason so the web UI can review them and mark
individual books wanted without changing the author policy. Use
`GET /api/v1/authors/metadata/review` to inspect pending candidates and
`POST /api/v1/authors/metadata/review/{id}/resolve` with `{"action":"wanted"}`
or `{"action":"ignore"}` to resolve one. Author monitoring does not grab
releases.

Saved book exclusions apply before first/latest selection. Ignoring a review
excludes that provider work for the subscription's format, even if its preferred
edition, title or monitoring policy later changes. Existing tracked books,
including manual corrections and removed entries, survive add-only refreshes.
Monitoring re-reads author settings, metadata profiles and exclusions after
provider IO. A stop/removal during that request prevents additions, and a
subscription revision change prevents recording a stale successful sync.
Changes during the subsequent candidate-write loop are not yet serialized with
every insertion. File-based policies still use recorded associations rather than
the native status evidence described below; author-file-policy adoption remains open.

Native author subscriptions accept `rootFolderId` on create and update. A root
must exist and match the subscription's ebook/audiobook format. Updates omit this
field to preserve it, or send `""` to return to the format default. A repeated
create without a root preserves a previously saved destination. Automatic new
books inherit the author root, quality profile and tags together; already tracked
books keep their own settings. Pending reviews store the root from their latest
evaluation and use that root when marked wanted, even if the author defaults
subsequently change. The review queue displays this destination. Deleting a root
clears these references, matching existing wanted-book root behavior.

Add New allows choosing these defaults for an author and immediately runs a
targeted refresh after saving. A failed refresh keeps the subscription visible
and reports the failure. Refresh Author uses the stored subscription without
re-saving form defaults. Library → Authors → Edit author settings changes the
root and quality profile for future additions. Readarr root-path/ID mapping and
migration inheritance remain separate compatibility qualification work under S16.

Author details use `GET /api/v1/library/authors/{key}?limit=100&cursor=...`.
Use a subscription or canonical author UUID for a direct link. Old normalized
name links remain supported; multiple matching identities return `choices`
instead of combining books. An explicit `name:` group contains records without
a usable author association or with a manual author-name override. Every response
has array-valued `books`, `subscriptions` and `choices`. Missing records return
404, invalid page arguments 400, and unavailable persistence 503.

The web page distinguishes unavailable from missing authors, shows total books
separately from current-page status counts, and provides Previous/Next controls.
Search Page searches only monitored books on the displayed page. Imported and
unmonitored books are included; removed/ignored books are excluded. Stable-data
paging reaches older records beyond prior collection caps. Concurrent title or
membership edits can change subsequent pages; refresh to restart the view.

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

### Metadata Profiles

Named, reusable add-filter sets (Readarr-style metadata profiles) carry the
same four filters. Migration 0028 seeds one no-filter "Standard" profile.

- `GET /api/v1/metadata-profiles` returns `{"profiles":[{id,name,
  allowedLanguages,mustNotContain,skipMissingIsbn,minPages,createdAt,
  updatedAt}]}`.
- `POST /api/v1/metadata-profiles` and `PUT /api/v1/metadata-profiles/{id}`
  accept the same shape (name required, unique) and return `{"profile":...}`.
- `DELETE /api/v1/metadata-profiles/{id}` returns `200 {}`, or `409
  {"error":"metadata profile is in use"}` while author subscriptions still
  reference the profile.

Author subscribe/update payloads and `AuthorSubscription` JSON gain
`metadataProfileId` (blank on update clears it; unknown ids are rejected with
`400`). During author monitor runs the referenced profile's filters replace
the per-author filter fields wholesale — profile wins when present, the
per-author overrides still apply when no profile is set. Skip reasons are
unchanged.

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

The scheduled `import-list-sync` task syncs every enabled list. Its enable flag
defaults to true and is independent of feed sync. Disabling scheduling does not
remove the explicit per-list or compatibility sync commands:

```dotenv
LIBRARRY_IMPORT_LIST_SYNC_ENABLED=true
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

- `GET /api/v1/librarry/blocklist?limit=` returns `{"items":[...]}`. Items
  include `wantedId`, `wantedTitle`, and `wantedAuthor` joined from the linked
  wanted item (empty strings when the entry is not linked).
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
LIBRARRY_RENAME_BOOKS=true
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

`LIBRARRY_RENAME_BOOKS` controls whether imports apply the naming templates.
Librarry defaults to `true` (note Readarr ships with renaming off). With
renaming off, imports still land in the author folder but keep the original
source filename — the book folder and file-name templates are skipped. The
toggle is exposed as `renameBooks` on `GET`/`PUT /api/v1/library/config`
(omitting the key on `PUT` leaves the stored value alone), and explicit
rename operations through `POST /api/v1/library/files/rename` keep using the
templates regardless.

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
pinned root when set (`rootFolderId` on `PUT /api/v1/wanted/{id}`, or at add
time via `rootFolderId` on `POST /api/v1/wanted` — validated to exist and to
match the wanted format, `400` otherwise), then the format's default root,
then the legacy config roots. `GET`/`PUT /api/v1/library/config` keep
working: the legacy two-root fields map onto the per-format default root
folders.

### Calibre-managed root folders

Native root folders carry an optional `calibre` object on `GET`/`POST`/`PUT`:
`{enabled,host,port,urlBase,username,password,library,convertFormats,
outputProfile,useSsl}` (migration 0029). The Calibre content server requires
authentication, so `enabled:true` needs `host`, `port`, `username`, and
`password` (`400` otherwise). The password is redacted to `""` in responses;
sending a blank password on `PUT` keeps the stored credential
(notification-target pattern).

Imports whose destination resolves to a Calibre-managed root skip the
move/hardlink+naming path entirely: the source file is posted to the root's
Calibre Content Server (add-book, metadata set-fields, and conversion jobs
for `convertFormats`). Calibre reports only the new book id — never a library
path — so the tracked file keeps its source path with import status
`calibre`. Rename endpoints skip files under Calibre-managed roots; the
preview row carries `reason: "managed by Calibre"`. The older
Readarr-compatible `isCalibreLibrary` root-folder metadata flow below keeps
working unchanged.

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
container. The Docker Compose examples mount persistent app config at `/config`,
so the default `/config/backups` path survives container recreation. The
database password travels to `pg_dump` via the child process environment and is
never logged.

### Inspecting interrupted imports

On the stabilization branch, Library Import → Import recovery shows saved native
completed-download plans, their source/destination files, attempts, failure reason,
and cleanup state. Retry import resumes that exact plan. Changing naming or mode
settings does not rewrite an in-flight plan. Changed bytes, missing sidecars, or a
removed book stop the retry and retain the original download.

The recovery API is `GET /api/v1/library/import-recovery`; retry is
`POST /api/v1/library/import-operations/{id}/retry`. Legacy link issues are visible
but require an explicit metadata correction; they never gain verification merely
from migration. Each collection has independent Previous/Next controls and exact
matching totals. Use “Show only unfinished imports and Calibre handoffs” to omit
completed work; pending manual/replacement cleanup remains included. Legacy issues
always show unresolved links. Return to first pages to see newly created records.

The endpoint accepts `limit=1..100` (default 100), `unfinishedOnly=true|false`, and
independent `operationsCursor`, `calibreCursor`, `issuesCursor` values from each
collection's `nextCursor`. Cursors are opaque and bound to collection/filter;
reset them when changing the filter. Records sort by creation time and identity,
so heartbeats and retries do not move them between pages. One response uses a
consistent database snapshot; consecutive pages reflect live changes, not a frozen
export. Resolved/deleted rows may disappear and newer records appear on page one.

Native recovery shows observed time, last recorded activity, verified manifest-file
count and lease purpose/state. A held lease is ownership evidence, not byte
progress. An expired lease does not prove the old process has stopped. Large files
can take time between journal updates. The UI disables transfer retry while the
snapshot has a held lease; the backend always rechecks ownership and saved bytes.
After commit, a lease for pending local cleanup is labeled as a cleanup lease;
retry stays disabled while it is held. Finished work has no active lease purpose. Reading recovery does not probe files or clients.

Completed imports support copy, hardlink and hardlinkOrCopy. For destination
conflicts keep both is the default; reviewed replacement requires a current preview.
Identifiable chapter sets import together with their sidecars. Uncertain sets and
multi-book packs appear above the import table as a complete file list. Assign
books per file (or apply one book to all), explicitly retain unwanted files,
confirm the assignments, then preview destinations and import. Editing assignments
invalidates the preview. A server-side content/path change also requires a new
preview. Required retained files block automatic source deletion.

Skip/Reject stop automatic import of that client/download without deleting files.
Use Resolved → Reopen review to reconsider. Reopening returns to manual review;
the next worker run does not silently import it. Native manual imports share the
durable execution engine; remote Calibre handoff recovery remains separate work.


Native completed-import recovery records temporary copies before writing them.
Retry reclaims an interrupted operation's recorded stage before resuming its
manifest. The recovery panel shows these temporary paths when present. Files
without journal ownership are retained, including older `.librarry-copy-*` and
`.librarry-import-*` leftovers; no directory-wide cleanup is performed.


Manual file import also creates an operation, with an optional wanted book.
Copy/hardlink/move retries reuse that operation rather than creating another
renamed copy. Move removes sources only after the manifest and records commit.
Configured same-basename extras are required members, so sidecar failures remain
visible. CUE/playlist sidecars preserve the media basename to keep references valid.

For manual Replace, the original remains at a recorded recovery path until the
new file is verified and committed. If a configured recycle bin is unavailable,
cleanup reports an error and retains that previous file. Repair the bin and use
Imports → Import recovery → Retry cleanup. A committed manual copy with completed
cleanup still retains its source; the UI distinguishes this from a completed move.

## Acquisition recovery

Postgres-backed acquisitions persist a reservation before sending to qBittorrent,
Transmission or SABnzbd. Manual adds, wanted grabs, monitor/feed/upgrade/recovery
paths share this boundary. A book's wanted row is its format-specific acquisition
scope; an exact release also has one active request across manual/worker paths.
Deleted/explicitly failed downloads free the slot after accepted bookkeeping is complete. A completed import permits a
different upgrade release; replaying its same request remains idempotent.

Activity shows submitting/uncertain requests. **Check client** reads the original
client directly and matches its exact intent tag or torrent infohash, with bounded
5–300 second retry backoff. It never sends another add. Missing results and outages
remain unresolved. A changed client address must be restored before reconciliation.
**Attach existing download** accepts an operator-confirmed exact ID; use this for
SABnzbd after acknowledgement loss, since arbitrary intent tags do not round-trip.
**Allow new attempt** requires inspecting the client and acknowledging duplicate
risk. It releases the reservation without starting a download. Active submissions
cannot be released while their lease remains valid.

An **Accepted · recovery needed** row means the remote submission succeeded but
local persistence needs repair. **Finish recovery** reuses the saved receipt to
restore the download link, book state and a single grab-history entry. It offers
no attach/release/new-attempt controls. If a previous download row was removed,
receipt recovery preserves that removal. If its persistence write never existed,
recovery reconstructs it from the accepted snapshot without another client add.

A selected upgrade is not yet an installed release. Native import records its
installed release/score and import history only when all required files commit.
An import interrupted by a history failure retries through Imports like other
commit failures. Finish pending accepted-acquisition recovery in Activity before
retrying an import whose message asks for it. Failed upgrades retain imported book
status; replayed results do not send new import/grab notifications.

GET `/api/v1/acquisition-recovery` returns up to 200 unresolved requests, oldest
first. POST `/api/v1/acquisition-recovery/{id}` accepts `action: check`, or
`action: attach` with `downloadId` and `confirmed: true`, or `action: release`
with `confirmed: true`. These routes use normal application authentication.
No-database development mode does not provide durable acquisition guarantees.

## Resumable library scans

The UI starts a job promptly with POST `/api/v1/library/scans` (202 Accepted),
then polls saved progress while the scheduler performs all file work.

POST `/api/v1/library/scan` still accepts `root`, `format`, and `limit`. Its response
now includes `jobId`, `state`, `phase`, `hasMore`, and `missing`. `limit` controls
only the first batch (default 1,000, maximum 5,000); it no longer truncates the
entire scan. A persisted scheduler task advances user-created jobs in 500-entry
batches every five seconds. It does not initiate new scans on its own. Closing the
browser does not stop a saved job; use **Cancel scan** in Imports.

GET `/api/v1/library/scans` returns up to 100 jobs, with unfinished/failed jobs
first. POST `/api/v1/library/scans/{id}` accepts `action: cancel` or `action: retry`.
Retry resumes the saved path queue; cancelled jobs require a new scan. Files added
or deleted while walking are reconciled conservatively, and interruptions retain
progress. Missing observations are staged until successful completion and checked
again before publication. No failed/cancelled scan publishes partial missing-file
changes. Finished jobs retain summary/root evidence and discard their path queues.

Root directory device/inode identity is checked across batches and successful
scans. If a root has changed, first verify the intended library is actually mounted.
The custom-root form exposes a replacement-folder acknowledgement after that
error; the equivalent API field is `acceptRootChange: true`. A root changing during
a running job cannot be accepted in that job. Start a new scan after inspection.
Nested device mismatches block missing-file conclusions. Root identity is currently
implemented on Unix (qualified on macOS/Linux); unsupported platforms fail clearly.

File `presenceState` is `unknown`, `present`, or `missing`, separate from import
history. Older records begin unknown; a scan must first observe a file before a
later scan can mark it missing. Native verified imports mark their destinations
present. Imports displays missing local files explicitly. Native book evidence,
moved-file reconciliation and repair previews are described below; compatible-API
parity remains outstanding. Scans never fuzzy-reassign a file automatically.


### Preview legacy library repairs

In Imports, choose **Preview library repairs**, then **Continue report** until
all pages are checked. Clean pages still have to be continued. A failed page can
be retried without discarding earlier findings; **Start a fresh report** reruns
from the beginning. The report shows why a record needs review and recommends an
action. It makes no changes.

A duplicate recorded hash is not permission to delete a file. Verify fresh bytes,
intentional copies/hardlinks and book assignments first. A legacy audiobook marked
imported can still lack evidence that every chapter arrived; single-file books
can be valid. Compare the original client inventory to actual destinations.
Possible move candidates require fresh path/content verification before the old
file identity is reattached. Deliberate moves and replacements can explain a
historical import-manifest discrepancy; keep that original history intact.

The report inspects saved database evidence rather than current media or client
inventories. Related-record samples show up to 20 paths with full candidate
counts. Each page is a current observation; start a fresh report after other
library changes. The preview does not apply repairs. Completed scans can perform the narrowly
verified move reattachment described below.


### Reattach moved native files

Scan all involved roots after moving files outside Librarry. A successful complete
scan can reattach the original file ID when it confirms the old path is absent,
finds exactly one matching new location and the new scan-created record has no
manual changes, book/download associations or import references. Copies with
identical recorded content make the match ambiguous and remain in repair preview.
Both paths must belong to this scan's roots. Calibre-managed records and roots
stay under Calibre review.

**Library scans** shows the reattached count. **View reattached files** shows the
old/new paths and retained IDs, with **Load more reattached files** for longer
history. Reconciliation changes database paths, not media on disk. The original
manual names, associations and import history remain intact; an old cleanup
receipt cannot authorize deletion merely because the file was reattached.

A destination changed after it was hashed requires a fresh scan. Failed database
completion can be resumed; cancellation leaves original identities unchanged.
Records created before discovery evidence was introduced are retained for manual
review rather than guessed into an existing book. Native file presence still
needs broader unified book-state projection and live NAS qualification.


### Replace reviewed completed-download destinations

In a pending file-set review, choose **Replace reviewed files** under **Existing
destinations**, confirm the book assignments, then preview again. The preview
identifies existing files to replace and is bound to their current content.
Changing the choice invalidates the preview. Keep both is the default.

The old file bytes are retained at recorded recovery paths until the complete new
set commits. **Import recovery** separately reports **Replacement backups** and
source cleanup. A recycle-folder failure keeps backups and exposes **Retry
cleanup**. Cleaning a replacement backup does not remove the download's source.

A different book's assigned file or Calibre-managed file cannot be replaced by
this flow. An existing book directory with extra old chapters/unknown files outside
the new manifest requires review or Keep both; full old-layout retirement is not
yet implemented. Matching chapter/sidecar replacements preserve tracked IDs and
manual names/notes. Historical manifests continue describing their original bytes.


## Provider connection checks

Open System and use the named **Check** button to verify a configured metadata
provider. Reading provider status alone performs no metadata HTTP request. Cards
separate configuration from the last actual request and successful request;
connection observations reset on API restart. The native route is
`POST /api/v1/providers/{name}/check` (URL-encode spaces in provider names) and uses
normal authentication. Missing-key checks report missing credentials without IO;
HTTP 429 shows the next allowed retry time. See [provider setup](provider-setup.md)
for probe scope and credential qualification limits.


Google fallback contracts run offline with `go test ./backend/internal/metadata`.
They cover primary-first routing, ISBN checksums/equivalence, literal titles and
subtitles, Unicode, languages/formats, partial errors, and author/series exclusion.
No provider key is needed for these fixtures; they do not qualify a real key.
See [provider setup](provider-setup.md) for search behavior.


Metadata cache tests use an injected clock and controlled providers to verify
positive/empty expiry, query dimensions, LRU/byte limits, nested-value isolation,
concurrent misses, cancellation and failure invalidation without live quota.
`go test -race ./backend/internal/metadata` runs them. Cached reads keep the
provider's actual request timestamps; use System's explicit check for current
connection evidence. Caches reset on process restart.


Author monitoring now consumes complete stable-ID bibliographies; `searchLimit`
no longer limits its total results. Offline fixtures cover 205-book Open Library
and Hardcover traversals, failed pages/count drift/duplicate IDs, same-name
identities, repeated database sync and original-date/credit policy handling.
Optional live read-only Open Library qualification is:

```bash
LIBRARRY_TEST_LIVE_OPEN_LIBRARY=1 go test -v ./backend/internal/metadata -run '^TestLiveOpenLibraryBibliography$' -count=1
```

This probe uses a known public author and the application's shared one-second
pacing transport, without catalog mutations. It checks request spacing and is
skipped explicitly in the default suite. It does not qualify Hardcover credentials. Manual/name-hash
legacy author subscriptions need a stable provider selection before monitoring;
existing wanted items are retained when a bibliography fails.


Shared HTTP budget contracts run with `go test -race ./backend/internal/providerhttp ./backend/internal/metadata ./backend/internal/importlists` (one shell command).
They cover named/legacy quotas, backoff expiry, concurrent clients, independent
hosts, canceled waits, shared metadata/list quota and unchanged health timestamps.
The regular API process passes the same client to metadata and import lists;
replacing credentials in a future runtime-settings flow must replace that client
and its provider instances together.


Import-list fixtures cover 605 books over short provider pages, missing/hidden
lists, incomplete pages, count/timestamp changes, malformed identities, response
size/page limits, cancellation and nullable timestamps. Database tests add 205
books, honor more than 1,000 exclusions, preserve removed/manual legacy editions,
commit initial root/monitoring settings, retry failed additions, and converge
concurrent syncs. Run `go test -race ./backend/internal/importlists` with
`LIBRARRY_TEST_DATABASE_URL` set as above for the database checks. No real Hardcover
token is used and no release is grabbed.


Rich Hardcover fixtures cover batched title enrichment, distinct ebook/audio IDs,
exact ISBN-10/13 lookup, ISBN-979 without blank-identifier broadening, work versus
edition dates, narrators, languages, covers, duration, missing/physical defaults,
invalid relationships/ISBN conflicts, interrupted enrichment and cache isolation.
Database checks prove that concrete editions do not reuse a work/format
placeholder, retain normalized edition evidence, and preserve add-only monitoring.
Run the metadata and wanted packages with the disposable Postgres configuration;
no live provider token is used.


### Book status evidence

Native wanted/library/detail responses include `stateEvidence`. `files.state` is
present, missing, incomplete, unknown or unavailable. Download evidence is fresh,
partial, unavailable or notConfigured. An ebook needs recorded present media; an
audiobook needs a complete matching committed media manifest. A partial chapter
loss is Incomplete; legacy audiobook files without a manifest are Unknown, even
if their old lifecycle says imported. Run a scan to update observations; do not
change lifecycle rows to manufacture completeness. A rename with retained IDs and
matching content preserves presence. Restoring a missing chapter and successfully
scanning restores completeness if its manifest identity still matches.

Book details explain unavailable evidence. A client outage never establishes that
a book is missing or resurrects stale download rows; known positive files retain
their presence with a warning. A completed, already-imported source still seeding
in the client does not hide later library damage. These are recorded import/scan
observations, not a live mount health check. Scanning an unavailable root retains
prior presence. Sidecar damage continues to block cleanup verification separately.

Wanted includes explicit Incomplete/Unknown filters over its loaded list. Global
list caps/counts and author-file-policy/Readarr use of the new evidence remain unfinished.
The native status path may spend up to five seconds on live client requests;
local-only page latency is not yet a qualified product guarantee.


### Worker checks and recovery gaps

Monitor and upgrade endpoints now return `skippedReason` per checked book when
current evidence blocks a search. Known missing/incomplete imported books enter
normal recovery search; unverified legacy media and partial/unavailable client
responses remain skipped. Upgrade search only operates on complete present copies
below cutoff. A still-seeding already-imported source does not hide library loss.
Automatic grab rechecks monitoring/settings/media after provider IO. An explicit
manual grab remains a separate owner action.

Check/attempt timestamps in migration 0042 advance fairness independently of
`last_search_at`, `last_upgrade_search_at`, and `last_sync_at`. Failed/skipped
checks back off for at most 15 minutes; successful activity retains configured
intervals. `force: true` retries immediately while retaining fair ordering.
Restarting the API does not reset this progress. This is not a worker lease and
does not eliminate duplicate provider reads from overlapping runs.

Wanted's Incomplete and Unknown tabs are `/wanted/incomplete` and `/wanted/unknown`.
Check Next Batch and Check Upgrade Batch examine the next 50 eligible scheduling
candidates, which can include skipped books; they do not mean every matching book
or the visible page. Selected searches retain explicit selected-ID scope. Feed
sync traverses every eligible monitored book in 200-row batches, with an explicit
1,000-detail response cap and full evaluated-match counts. Global list paging and
all-matching bulk jobs remain separate work.

Feed release observations persist decisions without updating the full indexer
search timestamp, so repeated RSS matches cannot postpone due searches.

### Selected upgrade checks

Wanted → Cutoff Unmet → Upgrade Search Selected checks the entire current
selection, including selections larger than 50. Each request accepts at most 200
books. Skipped books contribute to the summary and carry an explanation; a book
that was deleted before validation asks the operator to refresh the selection.
The action does not automatically grab. Check Upgrade Batch remains a separate
50-book queue action. Neither action means every matching book in a large library
has been processed, and collection-wide bulk jobs remain unfinished.


### Full book collection browsing

Library and Wanted's Missing, Incomplete, Unknown and Cutoff Unmet tabs use native
server pages of up to 100 books. Filters/sorts apply to the full tracked collection;
counts no longer mean just the first loaded 200 books. Previous/Next navigate
pages; Refresh page refreshes the current page. Changes while browsing can move
books, so return to the first page to restart a traversal after edits.

Selections clear on page/filter/tab changes. Select shown, mass edits, selected
search and selected upgrade actions affect only the current selected page.
Scheduled batch buttons remain separate 50-book actions. Metadata Review now
uses its own paged collection; file lists, compatibility and other legacy readers
retain separate scaling work.


### Author subscription browsing

Library → Authors pages up to 100 subscriptions, with full name/provider search,
format and monitored/unmonitored filters. Counts include every tracked book of
that subscription's format linked to its recorded author identity. Same-name
people do not share counts. An unresolved or conflicting provider identity is
shown as unavailable, not guessed by display name. Manual author corrections
continue to take precedence. Client outages leave uncertain book counts Unknown.

Previous/Next and Refresh page work independently of metadata refresh. Check
Author Batch and Force Author Batch check at most 50 subscriptions; each row's
refresh targets that subscription only. These are subscription settings rows,
not a deduplicated catalog of every writer in the library. Author review candidates
use the separate paged queue described below.


### Metadata Review

Wanted → Review includes tracked imported and unmonitored books with conflicting
metadata; removed and ignored books stay excluded. Search and format filters
apply to the full queue, and Previous/Next pages up to 100 reviews. The Review
count and conflict summary cover the full library, not just the loaded page.
Provider edition formats remain visible evidence but do not conflict with the
owner's ebook/audiobook acquisition choice.

Select shown and Keep current operate on the current page only. Every selected
confirmation commits together or rolls back together. Changed displayed evidence,
concurrent owner corrections and concurrently cleared overrides require a fresh
review instead of overwriting the owner's choice. If a conflicting field has no
current value to keep, open the book details and choose a value; skipped books
are reported explicitly. Refresh page reloads evidence without broadening the
selection. Collection-wide resumable bulk jobs remain future work.


### Author review candidates

Library → Authors → Author review queue shows six candidates per page, with
search, format and Pending/Wanted/Ignored/All decisions filters across the full
queue. Counts cover all stored candidates; changing filters returns to page one.
Previous reviews and Next reviews reach older candidates. A failed load offers
Retry reviews rather than an empty-queue claim.

Mark wanted uses the destination, profile and tags captured with the candidate.
If the work is already tracked in that format, the decision links the existing
book and preserves its destination, profile, tags, monitoring and removed/ignored
state. Open tracked book to change those choices explicitly. Ignore saves the
review decision without changing a tracked book. New candidate evidence requires
a refresh before deciding. The book, decision and history commit together;
retrying the same saved action returns its receipt, while a conflicting action
returns 409. No provider lookup or download is triggered by review resolution.


### Browsing library files

Imports and each book detail show Tracked files with up to 100 rows per page.
Search covers paths, titles and authors across the full collection. Format,
recorded presence and path/title/recent-update sort filters reset to page one.
Previous files, Next files and Refresh files navigate the collection. Counters
cover the full collection, or all files linked to the current book; they do not
shrink to the loaded page. An unavailable collection offers Retry files and
retains no misleading empty-list claim or stale Imports totals.

Presence means the last recorded scan/import observation, not a fresh disk
check. Unknown is explicit. Import status is separate: an imported file can be
missing locally. Book completeness still uses the full native import/file evidence,
not whichever chapter page is displayed. Open book follows recorded relational
associations; stale JSON hints do not define membership. Unassigned files remain
browsable. Destinations belonging to unfinished imports stay hidden until commit.

Rename Files remains available when the book list is empty. Its preview uses the
same file collection, with search, format and previous/next controls. Selection
and Apply cover only shown files. Changed files are initially selected on each
page; review the paths and uncheck any to exclude before applying. Collection-wide
resumable rename/bulk jobs remain separate work. Concurrent changes can affect
subsequent pages; Refresh preview reloads the current page.


### Recover a standalone file rename

Rename Files shows the reviewed source/destination pair and binds Apply to that
preview. Refresh preview after a change warning. A failure after planning exposes
its saved operation in Imports → Import recovery as **Saved file rename**. Use
**Retry rename** for unfinished publication or **Retry cleanup** once the new path
is committed. Retrying uses the captured destination even if naming settings have
changed. The original remains until the verified copy and the existing file row
commit; a pending old source is excluded from scan discovery.

The file ID, owner names/notes, book/download links and original import provenance
are preserved. An old import retry can return the verified renamed location.
Missing files, changed bytes, a conflicting destination or an ownership change
remain errors requiring review; a retry does not overwrite unrelated bytes.

Chapter sets and companion files show **Retained** with a reason directing you
to the book's **Rename book folder** action. Calibre-managed files remain under
Calibre's control. Per-file selection applies only to the displayed page; there
is no unattended collection-wide rename job.


### Rename a complete recorded book folder

Open a book and choose **Rename book folder**. Preview checks its latest complete
committed import against every linked file, current bytes, companion references
and naming settings. It shows the source and destination folders and pages through
the entire captured set. Apply moves **every file in this plan**, including rows
on other preview pages. The target folder follows the book's current metadata and
selected library root; basenames and relative disc paths stay unchanged.

CUE, M3U/M3U8 and local OPF references must resolve to recorded members within the
book folder. Unsupported encodings, absolute/escaping or missing file references,
unrecorded files, shared ownership and incomplete/legacy sets retain the folder
for review. This action does not rewrite chapters or playlists to new names.
Calibre owns its own layout. A verified scan move within the recorded book folder
can be retained, provided the current companion references are still valid.

Apply requires the current preview revision. If interrupted, refresh and choose
**Resume complete book rename**, or find **Saved book folder rename** in Imports.
Recovery keeps the captured destination even after naming settings change. All
media paths become visible in one database commit; originals remain until then.
Partial source cleanup can resume after restart. Existing file IDs, manual
metadata, book/download associations and monitoring remain unchanged.


### Qualify the Calibre HTTP client against a real server

The client supports Calibre's default Digest authentication on HTTP and explicit
Basic authentication. Configure the root's username/password normally; the client
first reads the server's authentication challenge from library-info. Redirects
are rejected, so configure the final server URL and prefix. Credentials belong
in the root fields, not in the host URL or query string.

The following fixture creates a new authenticated Calibre library in a disposable
container, uploads the repository's legal EPUB, updates metadata, converts to TXT,
deletes the created book and verifies that the library is empty. It does not use
homelab paths, accounts or media. `DOCKER_CONTEXT` is optional and scopes this
script without changing the global Docker context.

```sh
docker build -f scripts/fixtures/calibre/Dockerfile -t librarry-calibre:fixture .
python3 scripts/test-calibre.py
LIBRARRY_CALIBRE_AUTH_MODE=basic python3 scripts/test-calibre.py
```

The fixture prints its actual Calibre version. September local qualification used
Debian's Calibre 8.5 on ARM64. Ordinary Go test runs skip the disposable-server
case; CI builds the fixture and tests both modes. The contract verifies protocol
identities and actual remote effects. Set `LIBRARRY_TEST_DATABASE_URL` to the
disposable Postgres instance to also run real handoff recovery through fresh
service instances after metadata failure and lost upload acknowledgement. CI
runs both client and handoff tests. Calibre-managed sources do not earn download
cleanup eligibility.


### Recover a Calibre handoff

Open **Imports → Calibre handoffs**. An accepted book ID stays attached to the
saved original root across retry and restart. **Retry Calibre handoff** resumes
metadata syncing, known conversion jobs and local bookkeeping. Running conversions
also resume through the configured conversion background task. Disabling that
task requires timely manual checks; expired server jobs need review.

For an uncertain upload, inspect the original Calibre library first. Confirm that
the earlier request has stopped, then enter the existing Calibre book ID and choose
**Attach existing book**. Librarry verifies the ID and source format; you must
verify that it is the right book. If you verified that the upload is absent,
**Confirmed absent: allow another upload** permits a new send. A similar decision
is required for uncertain/failed conversions: verify an existing output format or
allow another conversion. After saving a decision, retry the handoff.

Recovery never silently changes servers or deletes the retained source. Restore
the original root configuration if the target has changed; password rotation is
allowed. Changes to the wanted destination or file associations require resolving
those owner settings before retry. These controls apply to new journal-backed
handoffs; old IDs and external Calibre file moves are not automatically repaired.

### Qualify and inspect shared background workers

System → Tasks reads shared database status and the last recorded success.
Every built-in worker remains listed when disabled or missing dependencies.
**Disabled here** reflects this API instance's startup flags; **Unavailable here**
means required configuration/services are absent. Reasons explain what to restore.
Dependency availability does not prove provider reachability: actual failures remain
in run/health evidence. Blocked workers have no local scheduling loop or next-run
time. Manual System Tasks runs return 409 when disabled and 503 when unavailable;
history and review remain accessible. Enable/fix configuration and restart this
API instance to resume scheduling. A peer with different flags may still be running
that worker; shared running state and history remain visible. Stopping automation
across a deployment requires changing every instance.

The Readarr `/api/v1/system/task` routes use the same recorded state. Unknown
native start/finish/duration values are omitted. The Readarr fields retain their
non-null date/time types: unknown dates use `0001-01-01T00:00:00Z` and unknown
duration uses `00:00:00`, with the corresponding `librarryLastStartTimeKnown`,
`librarryLastExecutionKnown`, `librarryNextExecutionKnown` and
`librarryLastDurationKnown` flags false. These are placeholders, not run evidence;
polling never fabricates current-clock executions.
Schedules/dependency reasons describe the responding instance. Database read
failures return 503 instead of a synthetic schedule. Direct domain commands retain
their existing explicit operator scope outside the scheduler.

**History** pages through retained runs, including counts, available operation IDs,
measured duration, completion state and next action. A pass that reports individual
errors is **degraded** and does not advance last success. Warnings are separate. A stopped owner is
shown as interrupted; an old heartbeat does not permit stealing a still-held
worker lock. **Run now** works before the next due time but returns busy while any
API process owns that task. During a database outage, workers refuse new claims
and status reports an outage instead of an empty or healthy task list.

The standalone fixture below runs two API containers with one disposable
Postgres database and a third API without persistence. It blocks a harmless scan query, checks shared running status
and duplicate-trigger refusal, kills the owning API, checks interruption, and
recovers through the peer before restarting the original process. It also checks
disabled-peer history/manual refusal, the separate import-list flag and unavailable
workers without persistence. It never
contacts a live download client or metadata provider.

```sh
DOCKER_CONTEXT=your-test-context python3 scripts/test-worker-packaged.py librarry-api:your-candidate
```

Task ownership uses a session advisory lock. Configure a direct/session-pooled
Postgres connection; transaction-pooling proxies are not supported for workers.
History keeps 100 successful runs per task. Unreviewed failed, degraded and
interrupted runs are preserved until reviewed. **Mark reviewed** acknowledges
the diagnostic only; it does not retry or repair work. **Mark unreviewed** reopens
it. Reviews bind the current state and review timestamp; stale decisions return
409. Use the **Unreviewed failures** filter to find older failures beyond page one.
Reviewed failures become eligible for cleanup after 90 days. Each task completion
removes at most 500 eligible failures, retaining its current run. Hourly
**History Maintenance** also removes up to 500 eligible reviewed failures across
all persisted workers, including workers now disabled. Their current run remains
available for diagnostic readback. Import/acquisition receipts are not pruned. Historical
last success is backfilled from recorded completed runs; older per-item error
counts cannot be reconstructed. Interrupted owners with unknown finish time have
no fabricated duration. Native notifications use the durable outbox described below. These fixtures do
not establish a live multi-instance deployment.

### Review notification delivery

Settings → Connect → Notification delivery lists new queued messages and their
outcomes. **Accepted** records HTTP acceptance, not a read receipt. **Uncertain**
means a request may already have reached the receiver: inspect it before choosing
**Confirm acceptance**, **Review retry**, or **Cancel delivery**. Retry can create
a duplicate, uses the connection's current settings and requires confirmation.
Cancellation stops future attempts but cannot retract a sent request. A stale
review is rejected; close the dialog, refresh and inspect the current entry.
Changed/deleted/disabled connections stop pending messages. New connections do not
receive old events. HTTP 429 uses bounded retries; other ambiguous failures wait
for review. Explicit connection tests remain immediate.

Native and migrated Readarr-compatible webhook connections are covered. History
marks compatibility targets as **Readarr webhook**; their configuration remains
under `/api/v1/notification`. The legacy `onDownload` import flag is honored unless
`onReleaseImport` is explicitly set. Compatibility health messages require
`onHealthIssue: true`. New events retain book/file details from commit time. Use a disposable local receiver
for qualification; do not point test notifications at real people.

```sh
DOCKER_CONTEXT=your-test-context python3 scripts/test-notification-packaged.py librarry-api:your-candidate
```

This fixture creates its own Postgres, API and Python HTTP receiver containers,
verifies pending recovery after restart for both target kinds, kills the API after
the receiver records a request, and verifies uncertainty plus confirmation or
cancellation without a second send. Readarr fixtures also check PUT/Basic settings
and immutable book details after an intervening edit.
It removes the containers/network on exit. The packaged backup fixture also
compares event, delivery, attempt, action and health-state records after restore.


### Notification history retention

**History Maintenance** runs hourly (or through System → Tasks → Run now).
It examines up to 100 eligible notification events per pass. Detailed history can
expire only after the event is at least 90 days old and every delivery has been
accepted or explicitly cancelled for at least 90 days. Pending, retrying, sending,
failed and uncertain deliveries are preserved. A delivery automatically stopped
because its connection changed or disappeared is also preserved until reviewed.
Settings → Connect offers **Confirm cancellation** for those stopped deliveries.
A retry clears the resolution time; accepting/cancelling it again starts a fresh
90-day window. Legacy accepted messages inherit their saved acceptance update
time; legacy cancellations remain unreviewed until explicitly confirmed.

Compaction removes resolved delivery/attempt/action detail and native/Readarr
payload snapshots together in a transaction. One compact event record retains its
UUID, source key, occurrence/compaction timestamps and outcome counts. These
identities are retained indefinitely to prevent an old source event from creating
new deliveries; compact-record count still grows with distinct events. Maintenance
does not delete import/acquisition receipts, domain history, health-transition
state, settings or files, and does not contact receivers. Busy delivery/review
sessions are skipped and retried on a later pass. Maintenance has a 30-second
notification deadline, records actual committed counts, and rolls back any event
whose compaction cannot finish. A later batch resumes the remaining history.

### Support reports and probes

In System, select **Download support report** to save `librarry-support.json`.
The same authenticated read is `GET /api/v1/system/support`. It reports the real
API build, selected effective settings, database connectivity/version, startup
schema, anonymous directory checks and recorded provider/task evidence. It omits
credentials, URLs, private paths, book metadata and free-text logs. Review before
sharing. Partial sections and unknown values are explicit; no external checks or
notifications are triggered by the export.

Use `/healthz` for liveness and `/readyz` for database readiness. A process without
Postgres can answer metadata requests but returns 503 from readiness. A successful
readiness check does not guarantee provider availability, mounts, writable media,
complete imports or a successful backup. The existing setup checklist is separate
from this current-connectivity probe. Worker failures and last success are retained
in System Tasks; downloading support does not acknowledge or clear them.

### Checking acquisition integrations

System shows configured, verified, failed and stale observations separately for
Prowlarr, qBittorrent, Transmission and SABnzbd. Use **Check [integration]** to make
an explicit read-only connection check, or run the Health Check task. The worker
normally checks every five minutes. Refreshing status, readiness or a support
report does not contact those services or emit health notifications.

Checks show the last attempt, success and numeric version, preserve prior success
after failure, and expire to stale after ten minutes. Concurrent/repeated checks
reuse observations for 15 seconds and respect Retry-After. Restart or changing
integration settings clears these process-local observations. A configuration
change during a check requires a new check. A verified connection is evidence of
API read access, not a guarantee that every search, grab or import will work.


### Dashboard triage

Needs attention now includes unfinished native imports, pending Calibre handoffs
and unresolved file links as well as review queues. Import counts include work still
running and committed transfers awaiting local cleanup; inspect the saved plan and
lease evidence before retrying. The recovery link opens the unfinished filter.
Refresh attention retries failed count sources. An unavailable source or incomplete
client evidence prevents an all-clear; retained counts may be stale.

Acquisition totals cover all active tracked books, while the action strip previews
recent books only. Imported here means a saved acquisition/import record. Use
Library to inspect current file presence and completeness. Import review counts
cover the complete queue and the review screen pages through it. These are
unreleased stabilization changes.


### Browsing import reviews

Imports → Import reviews uses pages of 50 reviews with complete matching/total
counts. Search title, author, source path or reason; filter by Pending/Resolved/All,
format, and file matches versus download payloads. Resolved includes imported,
skipped and rejected records; skipped/rejected payload reviews retain their
existing Reopen action. Pending payloads use the same page controls as file reviews.

Select page selects pending file rows only. Page or filter changes clear selection
and temporary match choices. Bulk actions send the selected IDs, never the entire
matching queue. Payloads still require individual file assignments and a current
preview. Changing pages discards unsubmitted payload edits; return and preview again.
Newly created reviews appear on the first page; retries do not change creation order.
Counts are a database snapshot per response, not a frozen export across pages.
Failed reads show an error and retry; First review page remains available to recover
from an obsolete cursor.


### Choosing books for imports

The manual-import and payload-file book selectors show 50 choices per page. Open
**Search and browse books** under a selector to search title, author or saved book
ID and reach older pages. Choices use saved local identities and owner-edited
labels; there is no provider or download-client request. Imported and unmonitored
active books remain selectable for explicit replacement/repair, while removed and
ignored books are omitted.

The current selection stays pinned even when it is outside a search or page.
Search/paging does not change the intended book. Changing the manual format clears
its old selection. If a selected identity is no longer active or no longer matches
the file format, it is shown as unavailable, not silently replaced by another book.
Database errors offer Retry books and retain the selected ID. Payload assignments,
retained-download choices and current-preview requirements remain unchanged.


### Removed and ignored books

Open Library → Removed to find inactive records, including books older than the
former list limits. Search title, author or saved book ID, filter format/status,
and use the page controls. Counts cover every inactive record; pages are ordered
by creation time, so edits do not move records. Last updated is not a removal date.
Use the book link to inspect existing files and metadata evidence.

Restore opens a review of the saved record. Monitoring is unchecked by default;
enabling it permits scheduled acquisition under the existing automation settings.
Restore itself neither searches nor grabs. Root/profile/tags, metadata overrides,
author policy, file links and prior history stay unchanged. It does not recover
physically deleted files. A concurrent edit, another restore or a new removal
rejects the old review. Use Reload book, inspect the new settings and submit again.
A successful restore writes one history event in the same transaction. If the
response is interrupted, refresh the list/details before retrying.

Direct links to inactive books show their status and the same explicit Restore
flow. Their regular monitoring/edit/release controls return after restoration;
file and provenance inspection remain available.

### Search library identity checks

Search checks saved work/edition/provider identities and the selected format before
allowing Add. This includes older, imported, unmonitored, removed and ignored books.
Titles alone are not identity evidence. Saved owner-edited titles appear on the
book buttons; provider source keys distinguish multiple matches. Open a saved book
to inspect file evidence or use its explicit restore flow.

A failed or incomplete check keeps Add disabled and offers Retry library check.
If another session adds the book first, the server rejects the repeated add and
Search refreshes its saved matches. No settings are overwritten and removed books
are not restored by Add. An unknown edition format uses the selected search
format consistently for both lookup and insertion; a concrete edition retains its
own format. Search checks tracking identity, not live download/file presence.


### Compatibility book qualification

Readarr-compatible book reads now include all active records, with missing/cutoff
pages based on native file evidence. Select books by their emitted numeric ID or
`librarryId`; title matching is no longer accepted. Missing/inactive IDs reject a
whole monitor/editor/delete request, and ambiguous aliases return 409. Refresh
before retrying a conflicting selection. Delete removes tracking, not files.

Run `scripts/test-integration.sh` with `LIBRARRY_TEST_DATABASE_URL` for the
10,001-book traversal, lost/partial files, quality cutoff, numeric sorting,
ambiguous identities, atomic rollback and concurrent-edit fixtures.
`scripts/test-packaged.py` checks complete older identities across restart,
native/compatible state agreement and mutation readback using disposable data.
A client that ignores `librarryStateCounts`, `librarryUnknownBooks` and
`librarryDownloads` cannot interpret an empty missing page as proof of health.
Persistent collision-free numeric IDs, complete non-book compatibility and a
real Readarr migration remain unqualified.
