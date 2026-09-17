# Metadata and monitoring

Configure profiles, author monitoring, feeds and import lists, and review provider evidence. Scheduled auto-grab defaults are explicit below.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Quality Profiles](#quality-profiles)
- [Wanted Monitor](#wanted-monitor)
- [Author Monitor](#author-monitor)
- [Author Add-Filters](#author-add-filters)
- [Metadata Profiles](#metadata-profiles)
- [Feed Sync](#feed-sync)
- [Import Lists](#import-lists)
- [Native Import Lists (Hardcover)](#native-import-lists-hardcover)
- [Provider connection checks](#provider-connection-checks)
- [Author subscription browsing](#author-subscription-browsing)
- [Metadata Review](#metadata-review)
- [Author review candidates](#author-review-candidates)
- [Search library identity checks](#search-library-identity-checks)

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
[native book status evidence](library.md#book-status-evidence); adoption by
author file-based policies remains open.

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

## Author Add-Filters

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

## Metadata Profiles

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

## Native Import Lists (Hardcover)

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

## Provider connection checks

Open System and use the named **Check** button to verify a configured metadata
provider. Reading provider status alone performs no metadata HTTP request. Cards
separate configuration from the last actual request and successful request;
connection observations reset on API restart. The native route is
`POST /api/v1/providers/{name}/check` (URL-encode spaces in provider names) and uses
normal authentication. Missing-key checks report missing credentials without IO;
HTTP 429 shows the next allowed retry time. See [provider setup](../provider-setup.md)
for probe scope and credential qualification limits.


Google fallback contracts run offline with `go test ./backend/internal/metadata`.
They cover primary-first routing, ISBN checksums/equivalence, literal titles and
subtitles, Unicode, languages/formats, partial errors, and author/series exclusion.
No provider key is needed for these fixtures; they do not qualify a real key.
See [provider setup](../provider-setup.md) for search behavior.


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
`LIBRARRY_TEST_DATABASE_URL` pointing to a disposable database as described in
[local development](../local-dev.md#reproducible-verification). No real Hardcover
token is used and no release is grabbed.


Rich Hardcover fixtures cover batched title enrichment, distinct ebook/audio IDs,
exact ISBN-10/13 lookup, ISBN-979 without blank-identifier broadening, work versus
edition dates, narrators, languages, covers, duration, missing/physical defaults,
invalid relationships/ISBN conflicts, interrupted enrichment and cache isolation.
Database checks prove that concrete editions do not reuse a work/format
placeholder, retain normalized edition evidence, and preserve add-only monitoring.
Run the metadata and wanted packages with the disposable Postgres configuration;
no live provider token is used.

## Author subscription browsing

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

## Metadata Review

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

## Author review candidates

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

## Search library identity checks

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
