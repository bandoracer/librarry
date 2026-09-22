# Book acquisition

Configure download clients, evaluate releases, and recover uncertain or failed acquisitions. These instructions describe the candidate; installed historical images may differ.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Acquisition](#acquisition)
- [Upgrades](#upgrades)
- [Failed Downloads](#failed-downloads)
- [Blocklist](#blocklist)
- [Downloads](#downloads)
- [Acquisition recovery](#acquisition-recovery)
- [Worker checks and recovery gaps](#worker-checks-and-recovery-gaps)
- [Selected upgrade checks](#selected-upgrade-checks)
- [Checking acquisition integrations](#checking-acquisition-integrations)

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

## Download from search

In **Add New**, select the book, then **Download ebook** or **Download audiobook**.
Librarry saves that edition, searches releases using the standard search language,
and starts the highest-ranked approved result. This uses your quality profile and
format-specific root folder; change them under **Options**. **Add Book** saves
without an immediate search or download; normal monitoring still applies.

Source scores such as “medium” are no longer displayed as a verdict on a book.
A selected title does not require another confirmation just because its ISBN is
missing or the query omitted the author. Explicit edition conflicts and missing
authors still prompt for review. Unknown source formats use a visible **Download
format** selector and remain unknown in stored provider evidence.

If nothing qualifies, the book stays saved for monitoring. Search or client errors
open the saved book for recovery; an uncertain download request is never retried
automatically. **Options → Search Releases** retains manual release selection.
Provider identifiers and evidence remain under **Book details and sources**.

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

## Worker checks and recovery gaps

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
1,000-detail response cap and full evaluated-match counts. Library/Wanted browsing
is paged independently; durable all-matching bulk jobs remain separate work.

Feed release observations persist decisions without updating the full indexer
search timestamp, so repeated RSS matches cannot postpone due searches.

## Selected upgrade checks

Wanted → Cutoff Unmet → Upgrade Search Selected checks the entire current
selection, including selections larger than 50. Each request accepts at most 200
books. Skipped books contribute to the summary and carry an explanation; a book
that was deleted before validation asks the operator to refresh the selection.
The action does not automatically grab. Check Upgrade Batch remains a separate
50-book queue action. Neither action means every matching book in a large library
has been processed, and collection-wide bulk jobs remain unfinished.

## Checking acquisition integrations

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
