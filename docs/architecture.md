# Architecture

Native author subscriptions and author metadata reviews carry an optional
`rootFolderId` (migration 0041). Subscription roots are checked against the wanted
format. Automatic additions copy root/profile/tags into a new wanted item in its
creation transaction; add-only refresh preserves existing tracking. Reviews use
the destination from their latest evaluation when explicitly marked wanted.
Changing author defaults does not migrate existing books. Root deletion clears
these references in the same way as wanted-book destinations.

`GET /api/v1/library/authors/{key}` resolves a subscription UUID, canonical author
UUID, or historical normalized name key. It returns an author, subscriptions,
book page, total count, next cursor and explicit identity choices for ambiguous
name links. Book pages are bounded to 100 rows and ordered by lowercase title
with UUID as the tie-breaker. Membership, counts, manual overrides and author
links share a read snapshot. Cursors bind to the resolved lookup key; paging
guarantees apply to stable data, not a frozen snapshot across multiple requests.

Wanted payloads expose recorded writer identities in `authors`. A manual
author-name override suppresses those links and moves that book into the
explicitly unidentified name group until the override is cleared. New writes
persist all supplied work contributors with their roles; author pages include
author/writer relationships and do not treat narrator credits as authorship.
Author alias locks prevent concurrent additions from splitting one stored
identity. Existing legacy roles/omitted coauthors are not silently backfilled.
Manual overrides and author links load in batches rather than one query per book.

Librarry is split into a Go backend, a React frontend, and Postgres.

The frontend is a modular Vite + React + TanStack Query SPA; its structure,
navigation contract, demo mode, and conventions are documented in
[frontend.md](frontend.md). UI surfaces that still need backend support are
tracked in [ui-backlog.md](ui-backlog.md).

## Backend

The backend owns provider credentials, metadata normalization, provider health,
matching policy, acquisition workers, and Postgres persistence. Provider tokens
never need to be exposed to the browser.

When `LIBRARRY_API_KEY` is configured, the router enforces Readarr-compatible
API-key auth on `/api/` routes before dispatching to native or compatibility
handlers. It accepts `X-Api-Key`, `apikey`, `apiKey`, and bearer auth while
leaving `/healthz` and `/ping` open for service probes.

Initial API surface:

- Readarr-compatible endpoints:
  - `GET /ping`
  - `HEAD /ping`
  - `GET /api/v1/system/status`
  - `GET /api/v1/system/routes`
  - `GET /api/v1/system/routes/duplicate`
  - `GET /api/v1/health`
  - `GET /api/v1/system/backup`
  - `GET /api/v1/update`
  - `GET /api/v1/diskspace`
  - `GET /api/v1/filesystem`
  - `GET /api/v1/language`
  - `GET /api/v1/localization`
  - `GET /api/v1/localization/options`
  - `GET /api/v1/log`
  - `GET /api/v1/log/file`
  - `GET /api/v1/log/file/{filename}`
  - `GET /api/v1/config/naming`
  - `GET /api/v1/config/naming/{id}`
  - `PUT /api/v1/config/naming/{id}`
  - `GET /api/v1/config/naming/examples`
  - `GET /api/v1/config/mediamanagement`
  - `GET /api/v1/config/mediamanagement/{id}`
  - `PUT /api/v1/config/mediamanagement/{id}`
  - `GET /api/v1/config/host`
  - `GET /api/v1/config/host/{id}`
  - `PUT /api/v1/config/host/{id}`
  - `GET /api/v1/config/ui`
  - `GET /api/v1/config/ui/{id}`
  - `PUT /api/v1/config/ui/{id}`
  - `GET /api/v1/config/downloadclient`
  - `GET /api/v1/config/downloadclient/{id}`
  - `PUT /api/v1/config/downloadclient/{id}`
  - `GET /api/v1/config/indexer`
  - `GET /api/v1/config/indexer/{id}`
  - `PUT /api/v1/config/indexer/{id}`
  - `GET /api/v1/calendar`
  - `GET /api/v1/history`
  - `GET /api/v1/history/since`
  - `GET /api/v1/history/author`
  - `GET /api/v1/history/book`
  - `GET /api/v1/parse`
  - `GET /api/v1/rootfolder`
  - `GET /api/v1/rootfolder/{id}`
  - `POST /api/v1/rootfolder`
  - `PUT /api/v1/rootfolder/{id}`
  - `DELETE /api/v1/rootfolder/{id}`
  - `GET /api/v1/queue`
  - `GET /api/v1/queue/details`
  - `GET /api/v1/queue/status`
  - `POST /api/v1/queue/grab/{id}`
  - `POST /api/v1/queue/grab/bulk`
  - `DELETE /api/v1/queue/{id}`
  - `DELETE /api/v1/queue/bulk`
  - `GET /api/v1/blocklist`
  - `DELETE /api/v1/blocklist/{id}`
  - `DELETE /api/v1/blocklist/bulk`
  - `GET /api/v1/blacklist`
  - `DELETE /api/v1/blacklist/{id}`
  - `DELETE /api/v1/blacklist/bulk`
  - `GET /api/v1/author`
  - `POST /api/v1/author`
  - `GET /api/v1/author/lookup`
  - `GET /api/v1/author/{id}`
  - `PUT /api/v1/author/{id}`
  - `DELETE /api/v1/author/{id}`
  - `PUT /api/v1/author/editor`
  - `DELETE /api/v1/author/editor`
  - `GET /api/v1/book`
  - `POST /api/v1/book`
  - `GET /api/v1/book/lookup`
  - `GET /api/v1/book/{id}`
  - `GET /api/v1/book/{id}/overview`
  - `PUT /api/v1/book/{id}`
  - `PUT /api/v1/book/monitor`
  - `DELETE /api/v1/book/{id}`
  - `PUT /api/v1/book/editor`
  - `DELETE /api/v1/book/editor`
  - `GET /api/v1/bookfile`
  - `GET /api/v1/bookfile/{id}`
  - `PUT /api/v1/bookfile/{id}`
  - `DELETE /api/v1/bookfile/{id}`
  - `DELETE /api/v1/bookfile/bulk`
  - `GET /api/v1/rename`
  - `GET /api/v1/retag`
  - `POST /api/v1/retag`
  - `GET /api/v1/wanted/missing`
  - `GET /api/v1/wanted/missing/{id}`
  - `GET /api/v1/wanted/cutoff`
  - `GET /api/v1/wanted/cutoff/{id}`
  - `GET /api/v1/qualityprofile`
  - `POST /api/v1/qualityprofile`
  - `GET /api/v1/qualityprofile/{id}`
  - `PUT /api/v1/qualityprofile/{id}`
  - `DELETE /api/v1/qualityprofile/{id}`
  - `GET /api/v1/delayprofile`
  - `POST /api/v1/delayprofile`
  - `GET /api/v1/delayprofile/{id}`
  - `PUT /api/v1/delayprofile/{id}`
  - `DELETE /api/v1/delayprofile/{id}`
  - `GET /api/v1/qualitydefinition`
  - `PUT /api/v1/qualitydefinition/{id}`
  - `GET /api/v1/languageprofile`
  - `POST /api/v1/languageprofile`
  - `GET /api/v1/languageprofile/{id}`
  - `PUT /api/v1/languageprofile/{id}`
  - `DELETE /api/v1/languageprofile/{id}`
  - `GET /api/v1/metadataprofile`
  - `POST /api/v1/metadataprofile`
  - `GET /api/v1/metadataprofile/{id}`
  - `PUT /api/v1/metadataprofile/{id}`
  - `DELETE /api/v1/metadataprofile/{id}`
  - `GET /api/v1/metadata`
  - `GET /api/v1/metadata/schema`
  - `POST /api/v1/metadata/test`
  - `POST /api/v1/metadata/testall`
  - `POST /api/v1/metadata/action/{name}`
  - `PUT /api/v1/metadata/bulk`
  - `DELETE /api/v1/metadata/bulk`
  - `POST /api/v1/metadata`
  - `GET /api/v1/metadata/{id}`
  - `PUT /api/v1/metadata/{id}`
  - `DELETE /api/v1/metadata/{id}`
  - `GET /api/v1/customformat`
  - `POST /api/v1/customformat`
  - `GET /api/v1/customformat/{id}`
  - `PUT /api/v1/customformat/{id}`
  - `DELETE /api/v1/customformat/{id}`
  - `GET /api/v1/tag`
  - `POST /api/v1/tag`
  - `GET /api/v1/tag/{id}`
  - `PUT /api/v1/tag/{id}`
  - `DELETE /api/v1/tag/{id}`
  - `GET /api/v1/restriction`
  - `POST /api/v1/restriction`
  - `GET /api/v1/restriction/{id}`
  - `PUT /api/v1/restriction/{id}`
  - `DELETE /api/v1/restriction/{id}`
  - `GET /api/v1/notification`
  - `GET /api/v1/notification/schema`
  - `POST /api/v1/notification/test`
  - `POST /api/v1/notification/testall`
  - `POST /api/v1/notification/action/{name}`
  - `PUT /api/v1/notification/bulk`
  - `DELETE /api/v1/notification/bulk`
  - `POST /api/v1/notification`
  - `GET /api/v1/notification/{id}`
  - `PUT /api/v1/notification/{id}`
  - `DELETE /api/v1/notification/{id}`
  - `GET /api/v1/importlist`
  - `GET /api/v1/importlist/schema`
  - `POST /api/v1/importlist/test`
  - `POST /api/v1/importlist/testall`
  - `POST /api/v1/importlist/action/{name}`
  - `PUT /api/v1/importlist/bulk`
  - `DELETE /api/v1/importlist/bulk`
  - `POST /api/v1/importlist`
  - `GET /api/v1/importlist/{id}`
  - `PUT /api/v1/importlist/{id}`
  - `DELETE /api/v1/importlist/{id}`
  - `GET /api/v1/importlistexclusion`
  - `POST /api/v1/importlistexclusion`
  - `GET /api/v1/importlistexclusion/{id}`
  - `PUT /api/v1/importlistexclusion/{id}`
  - `DELETE /api/v1/importlistexclusion/{id}`
  - `GET /api/v1/remotepathmapping`
  - `POST /api/v1/remotepathmapping`
  - `GET /api/v1/remotepathmapping/{id}`
  - `PUT /api/v1/remotepathmapping/{id}`
  - `DELETE /api/v1/remotepathmapping/{id}`
  - `GET /api/v1/downloadclient`
  - `GET /api/v1/downloadclient/schema`
  - `POST /api/v1/downloadclient/test`
  - `POST /api/v1/downloadclient/testall`
  - `POST /api/v1/downloadclient/action/{name}`
  - `PUT /api/v1/downloadclient/bulk`
  - `DELETE /api/v1/downloadclient/bulk`
  - `POST /api/v1/downloadclient`
  - `GET /api/v1/downloadclient/{id}`
  - `PUT /api/v1/downloadclient/{id}`
  - `DELETE /api/v1/downloadclient/{id}`
  - `GET /api/v1/indexer`
  - `GET /api/v1/indexer/schema`
  - `POST /api/v1/indexer/test`
  - `POST /api/v1/indexer/testall`
  - `POST /api/v1/indexer/action/{name}`
  - `PUT /api/v1/indexer/bulk`
  - `DELETE /api/v1/indexer/bulk`
  - `POST /api/v1/indexer`
  - `GET /api/v1/indexer/{id}`
  - `PUT /api/v1/indexer/{id}`
  - `DELETE /api/v1/indexer/{id}`
  - `GET /api/v1/release`
  - `POST /api/v1/release`
  - `GET /api/v1/manualimport`
  - `POST /api/v1/manualimport`
  - `GET /api/v1/command`
  - `POST /api/v1/command`
  - `GET /api/v1/command/{id}`
  - `DELETE /api/v1/command/{id}`
  - `GET /api/v1/system/task`
  - `GET /api/v1/system/task/{id}`
- Librarry-native endpoints:
  - `GET /healthz`
  - `GET /api/v1/providers/health`
  - `GET /api/v1/providers/diagnostics`
  - `GET /api/v1/readiness`
  - `GET /api/v1/search?query=&type=book&format=any`
  - `GET /api/v1/integrations/health`
  - `GET /api/v1/integrations/config`
  - `PUT /api/v1/integrations/config`
  - `POST /api/v1/integrations/bootstrap`
  - `POST /api/v1/releases/search`
  - `POST /api/v1/grabs`
  - `GET /api/v1/downloads`
  - `GET /api/v1/downloads/{id}`
  - `GET /api/v1/downloads/resources`
  - `GET /api/v1/downloads/preferences`
  - `PUT /api/v1/downloads/preferences`
  - `POST /api/v1/downloads/categories/actions`
  - `POST /api/v1/downloads/tags/actions`
  - `POST /api/v1/downloads/actions`
  - `POST /api/v1/downloads/{id}/files/actions`
  - `POST /api/v1/downloads/{id}/trackers/actions`
  - `POST /api/v1/downloads/rebalance`
  - `POST /api/v1/downloads/recover-failed`
  - `GET /api/v1/quality-profiles`
  - `POST /api/v1/quality-profiles`
  - `GET /api/v1/authors`
  - `POST /api/v1/authors`
  - `POST /api/v1/authors/monitor`
  - `GET /api/v1/wanted`
  - `POST /api/v1/wanted`
  - `PUT /api/v1/wanted/{id}`
  - `PATCH /api/v1/wanted/{id}`
  - `DELETE /api/v1/wanted/{id}`
  - `GET /api/v1/wanted/metadata/review`
  - `GET /api/v1/wanted/metadata/{id}`
  - `POST /api/v1/wanted/metadata/{id}/apply`
  - `POST /api/v1/wanted/metadata/{id}/apply-bulk`
  - `DELETE /api/v1/wanted/{id}/overrides/{field}`
  - `POST /api/v1/wanted/{id}/search`
  - `GET /api/v1/wanted/releases/{id}`
  - `POST /api/v1/wanted/{id}/grab`
  - `POST /api/v1/wanted/monitor`
  - `POST /api/v1/wanted/feed-sync`
  - `POST /api/v1/wanted/upgrades`
  - `GET /api/v1/librarry/history`
  - `GET /api/v1/library/files`
  - `DELETE /api/v1/library/files/{id}`
  - `POST /api/v1/library/files/delete`
  - `POST /api/v1/library/books/{id}/rename/preview`
  - `POST /api/v1/library/books/{id}/rename`
  - `POST /api/v1/library/files/rename/preview`
  - `POST /api/v1/library/files/rename`
  - `POST /api/v1/library/calibre/conversions/refresh`
  - `GET /api/v1/library/import-reviews`
  - `POST /api/v1/library/import-reviews/resolve-bulk`
  - `POST /api/v1/library/scan`
  - `POST /api/v1/library/import`
  - `POST /api/v1/library/import-completed`
  - `POST /api/v1/library/import-reviews/{id}/resolve`
  - `POST /api/v1/settings/validate`

## Metadata Model

The data model separates conceptual works from concrete editions. This is the
main durability choice: a single book can have many ebook, audiobook, print, and
translated editions.

Manual overrides are stored separately and should always win over provider data.
Provider records keep raw provenance so future reconciliation can explain where
data came from and why a match was accepted or sent to review.

## Acquisition

Librarry integrates directly with Prowlarr, qBittorrent, Transmission, and
SABnzbd. Prowlarr is queried for book releases through
`/api/v1/releases/search`; the
Readarr-compatible `/api/v1/release` endpoint maps the same acquisition flow
into interactive release-search and grab payloads. When a Readarr-compatible
release search is tied to a known book or wanted item, Librarry routes it
through the wanted-release evaluator, persists the scored release decisions,
returns Readarr-style integer release IDs, and resolves those IDs back to the
stored Librarry release decision on grab. Torrent releases are sent to
qBittorrent by default, or Transmission when qBittorrent is absent or the grab
payload requests it. Usenet/NZB releases are sent to SABnzbd through the same
`/api/v1/grabs` API. Startup and `/api/v1/integrations/bootstrap` ensure the
book categories exist in qBittorrent when qBittorrent is configured; Transmission
does not have native categories, so Librarry maps book categories to labels.
The Downloads page can also send a manual magnet link, `.torrent` file,
torrent URL, or NZB URL through the same grab path, paused by default, with
ebook/audiobook category selection.

Native integration settings are exposed through `GET` and `PUT`
`/api/v1/integrations/config`. The Settings page writes Prowlarr,
qBittorrent, Transmission, and SABnzbd connection details into the same
Postgres-backed compatibility resource records used by the Readarr-compatible
indexer/download-client endpoints. The running acquisition service is
reconfigured immediately after a save, and startup reloads those persisted
records before constructing the Prowlarr and download-client clients.

Download state is reconciled from qBittorrent, Transmission, and SABnzbd through
`/api/v1/downloads` and stored in Postgres when database persistence is
configured. Torrent actions are exposed through `/api/v1/downloads/actions` for
start, stop, delete, recheck, priority changes, force-start, sequential-download
toggle, first/last-piece priority toggle, rename, tag add/remove, category
changes, and location changes, plus delete-with-data removal and per-torrent
download and upload speed limits. qBittorrent and Transmission global
preferences are exposed through `/api/v1/downloads/preferences` for save paths,
temp or incomplete path usage, paused-add behavior, global and alternate speed
limits, scheduler toggles, and queue caps. qBittorrent category/tag namespaces,
Transmission label-derived category/tag resources, and SABnzbd category
resources are exposed through
`/api/v1/downloads/resources`,
`/api/v1/downloads/categories/actions`, and
`/api/v1/downloads/tags/actions`, with tag actions limited to clients that have
tag or label primitives. The Downloads page filters by client, state, category,
and text, and uses the same native actions for name, tags, category, save-path,
bandwidth, preferences, tracker, file, namespace, and bulk queue controls.
qBittorrent and Transmission details are exposed through
`/api/v1/downloads/{id}` with properties, peer state, tracker state, and torrent
file lists. Per-file priority changes are exposed through
`/api/v1/downloads/{id}/files/actions` for qBittorrent and Transmission
skip/normal/high/max file selection. Tracker add/edit/remove actions are exposed
through `/api/v1/downloads/{id}/trackers/actions` for qBittorrent and
Transmission torrents.
`/api/v1/downloads/rebalance` adds a simple active-download limiter that can
preview or apply start/stop operations against a filtered queue. Transmission
actions support start, stop, delete, recheck, queue movement, force-start, set
location, label-backed category/tag changes, label resource listing and
rename/delete, tracker add/edit/remove, per-torrent speed limits, detail
inspection, and file-priority changes. SABnzbd supports
queue/history detail lookup, queued-file inspection through `get_files`,
category namespace administration, and start, stop, delete, rename, category,
and priority actions. The API accepts
multiple download IDs for these actions, routes
each ID back to its owning client when possible, and the web UI exposes
selected-row bulk controls for the common queue operations. The current manager
is intended for Librarry acquisition operations, not as a full replacement for a
dedicated torrent-client interface.

Failed-download recovery can be triggered manually through
`POST /api/v1/downloads/recover-failed` and can also run on an interval in the
API process. Recovery detects qBittorrent error/missing-file states and stale
stalled downloads with no seeders, reopens the linked wanted item, records the
failure on the download row, searches for replacement releases, and optionally
grabs the best approved replacement. Scheduled recovery defaults to search-only;
auto-grab and failed-torrent removal are explicit settings.

Readarr-compatible `/api/v1/blocklist` and legacy `/api/v1/blacklist` endpoints
are populated from failed active downloads plus failed Librarry history events.
Single and bulk delete clear persisted download failure metadata when the record
maps to an active download, and store compatibility tombstones so cleared
download or history records stay suppressed from future blocklist responses.

Readarr-compatible root folders are persisted in Postgres through
`compat_root_folders`. The compatibility API keeps environment-configured ebook
and audiobook roots as defaults, overlays persisted folders by path, and maps
stored UUIDs to stable integer IDs for Arr-style clients.
Root folder records also preserve Readarr's Calibre Content Server fields
(`isCalibreLibrary`, host, port, URL base, credentials, library, output format,
output profile, SSL, default profiles, monitor options, and tags) as metadata so
setup clients can round-trip Calibre configuration.

New native-root Calibre imports use a separate `calibre_handoffs` journal. The
source path and exact optional download UUID reserve one plan with the original
root ID, a credential-free target fingerprint, source hash and captured conversion
targets. A dedicated Postgres session advisory lock serializes remote work;
per-run tokens fence writes from a previous connection. No database transaction
is held across network calls.

The journal progresses through planned, uploading, accepted, converting, ready
and committed. Uploading without a saved book ID means uncertain acceptance and
cannot automatically send again. Per-format conversion states similarly record
starting/submitted/polling/done or review-needed outcomes. Each destructive
terminal status response is saved before the next poll. Conversion job zero is
valid. The conversion worker resumes known accepted work in oldest-updated order;
legacy file polling cannot consume a journal-owned terminal status again.

Metadata syncing reads current owner data. Commit locks and checks the current
book/file revision, original target and relational ownership, then records the
file, book/download associations, imported status, installed release and history
in one transaction. Root password rotation does not change the saved target;
server/library/identity changes do. The source remains the tracked path because
Calibre does not return its managed file path. No native cleanup receipt is earned.

`GET /api/v1/library/import-recovery` includes bounded `calibreHandoffs` and a
separate total `calibreUnfinished` count. `POST
/api/v1/library/calibre-handoffs/{id}/retry` resumes the saved plan. The sibling
`/resolve` endpoint requires explicit confirmation for attaching an existing book
ID, allowing another upload, accepting an existing format or allowing another
conversion. Attach/format decisions read the original server before accepting;
the owner identifies the book, without automatic fuzzy matching. Decisions and
journal updates commit together before further remote work. Credentials and
remote diagnostic bodies are excluded from recovery output.

Physical deletion of a journal-backed book resolves its saved root and positive
Calibre book ID, rejecting a changed target. Legacy file conversions retain the
older refresh route. Richer edition metadata, embedded metadata writes, remote
path refresh and legacy identity repair remain future work.

Readarr-compatible operational support endpoints expose filesystem browsing,
languages, localization strings, logs, update records, and backup records. The
current log, backup, and update endpoints are intentionally conservative
compatibility surfaces: they satisfy normal client probes without pretending to
run an in-app updater or backup manager.

Common Arr resource endpoints are exposed for quality definitions, language
profiles, metadata profiles, metadata consumers, tags, custom formats,
restrictions, notifications, import lists, import-list exclusions, and remote
path mappings. These endpoints render useful defaults and persist create,
update, and delete operations in `compat_resources` using the Readarr-style
integer ID exposed to API clients. Resource payloads are kept as JSON so
Librarry can preserve fields it does not natively interpret yet.
Readarr-compatible restriction resources are interpreted by the wanted release
evaluator as additional required, ignored, and preferred terms. Untagged
restrictions apply globally; tagged restrictions apply when the wanted item has
at least one matching tag ID. Readarr-compatible import-list resources are
interpreted by `ImportListSync`: enabled lists can carry inline `books`,
`items`, `entries`, `titles`, `queries`, `isbns`, or `fields[].value` entries
that resolve through metadata search or deterministic fallback records before
creating monitored wanted items. Import-list exclusion resources are checked
before creation. Author-monitor-created wanted items inherit their
author subscription tags. Readarr-compatible author and book editor payloads
honor `applyTags` modes for adding, removing, replacing, or leaving tags
unchanged.
Download-client, indexer, notification, and import-list resources also expose
Readarr-style schema, test, test-all, action, and bulk mutation endpoints so
Arr clients that validate resource implementations before saving can complete
their normal probe flow.
Webhook notification resources have native side effects: enabled Webhook
notifications are delivered for grab, release import, upgrade, failed-download,
and test events using the stored URL/method fields. Other notification
implementations remain compatibility records until they are backed by native
delivery code.

Compatibility config endpoints for naming, media-management, host, UI,
download-client, and indexer settings render current Librarry defaults and
persist compatible PUT updates in `compat_resources` as singleton records.
These records preserve Readarr-shaped fields that Librarry does not natively
interpret yet, while the active scheduler/task behavior remains derived from
native Librarry config. Delay profiles and system tasks are exposed in the same
compatibility layer; tasks use the real scheduler definition and persisted run
evidence for feed sync, missing-book monitoring, author refresh, failed-download
recovery, upgrade search, import-list sync and Calibre refresh.

Wanted items are stored in Postgres from normalized metadata results. Native
`PUT`/`PATCH /api/v1/wanted/{id}` updates title, author, cover URL,
quality profile, monitoring state, and tags. Title, author, cover URL, and
quality-profile edits create `manual_overrides` rows for the wanted item. The
metadata correction API also supports protected bibliographic overrides for
language, publisher, published date, series, series position, and ISBN, so
provider evidence can be promoted without changing the provider record itself.
Single-field corrections use `POST /api/v1/wanted/metadata/{id}/apply`; provider
record promotion uses `POST /api/v1/wanted/metadata/{id}/apply-bulk` to protect
all usable values from the selected provider record in one transaction. Provider
refreshes preserve manually corrected fields during upsert. Wanted item payloads
include the current manual override list. Readarr-compatible book records project
protected language, publisher, published date, series, series position, and ISBN
values into their `editions` payloads so corrected bibliographic metadata is
available to Arr clients, and
`DELETE /api/v1/wanted/{id}/overrides/{field}` clears one override. Title,
author, and cover URL resets immediately restore canonical work/author values
when those rows are available; quality-profile reset returns to `standard`, and
bibliographic override resets remove the protected value so provider evidence is
shown normally again.
`GET /api/v1/wanted/metadata/review` is conflict-driven: it lists wanted items
with unresolved provider disagreements, not every item with a protected override.
Protected-only corrections remain visible in item provenance and override chips
without keeping the item in the operator's review queue. When an operator keeps
the current canonical value during conflict review, Librarry stores the same
manual override value with a `metadata review canonical accepted` reason so the
field remains protected without reappearing as unresolved review work. Operators
can apply that same canonical-accepted decision to selected review items through
`POST /api/v1/wanted/metadata/review/confirm-canonical`.
`DELETE /api/v1/wanted/{id}` soft-removes the wanted item without deleting
library files or download-client data. Monitoring state is stored separately
from acquisition/import status so Readarr-compatible monitor/unmonitor calls do
not erase grabbed or imported state. A wanted item can search releases through
Prowlarr, using protected ISBN corrections before broad title/author terms,
persist scored release decisions, enforce protected language corrections during
scoring, and record explicit rejection reasons before a candidate is sent to the
matching download client.
`GET /api/v1/wanted/releases/{id}` reloads the stored decision set for manual
review. `POST /api/v1/wanted/{id}/grab` only accepts approved decisions unless
the request includes `force: true`; forced grabs are recorded in history while
scheduled monitor, feed, failed-download, and upgrade flows continue to select
approved decisions only.
Readarr-compatible book updates persist monitored state, quality profile
changes, title/author display overrides, and soft removal.
The book editor endpoints apply the same durable mutations in bulk for common
Arr mass-editor clients.
The native Wanted queue uses tracked library files to segment active items into
missing, grabbed, present, and all views, so operators can focus on books that
still need acquisition or import without losing visibility into grabbed items.

Quality profiles are stored in Postgres and applied anywhere a release is
evaluated: manual wanted search, scheduled monitoring, feed sync, failed
download replacement search, and upgrade search. Profiles currently control
minimum approval score, upgrade cutoff score, minimum torrent seeders, maximum
size, preferred terms, required terms, rejected terms, and whether upgrade
search is allowed.

The wanted monitor can be triggered manually through
`POST /api/v1/wanted/monitor` and can also run on an interval in the API process.
It selects due wanted items, reuses the same release evaluator as manual search,
records monitor run summaries, writes history events, and optionally sends the
best approved release to the matching download client when
`LIBRARRY_MONITOR_AUTO_GRAB=true`.

Author subscriptions are stored separately from wanted items. A subscription
captures provider provenance, author identity, target format, and quality
profile. Readarr-compatible author updates persist monitored/unmonitored state,
quality profile changes, monitor-new-items settings, and soft removal. The
author editor endpoints apply the same durable mutations in bulk. The author
monitor can be triggered manually through
`POST /api/v1/authors/monitor` and can also run on an interval in the API
process. It searches metadata providers for due authors, creates or refreshes
wanted items for matching books, records sync history, and does not grab
releases directly. Metadata queries distinguish author identity lookup from
author bibliography lookup: `/api/v1/author/lookup` asks providers for author
records, while the author monitor asks for works by the subscribed author and
passes the stored provider key when one is available.

Upgrade search can be triggered manually through `POST /api/v1/wanted/upgrades`
and can also run on an interval in the API process. It searches grabbed/imported
wanted items, compares approved releases against the current grabbed release
score, and records upgrade candidates only when the item is below its profile
cutoff and the candidate improves by the configured minimum delta. Upgrade
search defaults to search-only; auto-grab is an explicit setting.

Feed sync can be triggered manually through `POST /api/v1/wanted/feed-sync` and
can also run on an interval in the API process. Prowlarr does not provide a
single aggregate RSS endpoint, so Librarry lists RSS-enabled Prowlarr indexers,
pulls each Prowlarr-compatible Torznab/Newznab feed, stores seen releases, and
matches feed entries against wanted items with the same release evaluator used by
manual search. Feed sync defaults to search-only and only sends approved
releases to the matching download client when
`LIBRARRY_FEED_SYNC_AUTO_GRAB=true` or a manual request sets `autoGrab`.

## Library Import

Library scanning walks the configured ebook and audiobook roots, classifies
supported file extensions, extracts OPF sidecar and embedded EPUB package
metadata plus MP3 ID3 and M4B/MP4 audio tags when present, stores file records
in Postgres, and keeps a fingerprint in file metadata for later reconciliation.
Manual import accepts a source file path, optionally ties it to a wanted item,
and copies or moves the file into a sanitized path below the format root. The
default naming policy is
`Author/Title/Title.ext`; deployments can change the author folder, book folder,
file name, and space replacement templates. Embedded metadata title/author
evidence is preferred over filename parsing unless a wanted item supplies an
explicit title or author.

The Readarr-compatible `/api/v1/manualimport` surface maps to the same import
engine. `GET /api/v1/manualimport` lists pending import reviews, can scan a
provided folder for supported ebook/audiobook candidates, and `POST
`/api/v1/manualimport` imports selected files through the regular library import
path or resolves a pending review when the payload includes `librarryReviewId`
or matches one pending source path. The naming and media-management
compatibility config endpoints reflect the active Librarry roots and naming
templates and persist compatible overrides through `compat_resources`; root
folder writes are persisted in
`compat_root_folders`.

The Readarr-compatible `/api/v1/bookfile` surface maps native Librarry file
records back into Arr-style bookfile records with stable numeric IDs, nested
book/author records, quality, size, path, and date metadata. `GET
/api/v1/bookfile` supports book and author ID filtering, while `GET
/api/v1/bookfile/{id}` returns a single mapped file. `DELETE` removes the native
file record and honors `deleteFiles=true` for physical file removal. `PUT`
persists Readarr-style quality, language, scene/release-group, and Arr ID
metadata on the tracked file record without moving or rewriting the physical
file.

The Readarr-compatible `/api/v1/retag` surface computes title, author,
language, and quality tag differences for tracked files and persists applied
retag state back onto the native file record. `RetagFiles`, `RetagBookFiles`,
and `RetagBooks` commands run the same path. This is database-backed retag state
for compatibility and auditability; embedded EPUB/MP3/M4B metadata writes remain
separate future work.

The native library rename endpoints preview or apply moves for selected tracked
files using the active naming templates. The Readarr-compatible `/api/v1/rename`
endpoint maps tracked files into Arr-style rename previews, and `RenameFiles`,
`RenameBookFiles`, or `RenameBooks` commands apply the same native rename path
after translating Readarr-style numeric IDs back to Librarry file IDs.

Readarr-compatible commands complete synchronously when Librarry can execute the
matching native operation. The command collection exposes stable completed task
records, `POST /api/v1/command` returns a pollable command ID, and
`GET /api/v1/command/{id}` plus `DELETE /api/v1/command/{id}` support clients
that follow the normal Arr command polling and cancel flow. `RssSync`,
`MissingBookSearch`, `BookSearch`, `RefreshAuthor`, `AuthorSearch`,
`ImportListSync`, `FailedDownloadCheck`, `UpgradeSearch`,
`CutoffUnmetBookSearch`, `RenameFiles`, `RefreshCalibreConversions`, and
`RescanFolders` map to native Librarry work.

Readarr-compatible calendar, history, parse, missing, and cutoff-unmet endpoints
are derived from wanted items, tracked library files, Librarry history events,
title parsing, and quality-profile cutoffs. The missing endpoints include
wanted/grabbed monitored books that do not have a matching present file; a file
can satisfy a wanted item through explicit `wantedId` metadata, exact ISBN
evidence from provider records or protected overrides, or normalized title,
author, and format matching. The web UI keeps using the native
`/api/v1/librarry/history` event feed so external Arr clients can use the
Readarr-shaped `/api/v1/history` response.

Completed-download import refreshes Librarry-tagged download-client items, filters
for completed torrents, locates the best supported ebook or audiobook file below
the torrent save path, imports linked wanted downloads, and records
imported/error/skipped state on the download row. Completed downloads without a
wanted tag use the same local-metadata matcher before review creation. A single
high-confidence ISBN match from provider records or protected overrides, or a
title/author/format match, is auto-imported into that wanted item by default;
clients can pass `autoMatch: false` to force manual review. Ambiguous or
low-confidence matches are persisted as pending import reviews. The review
builder compares local metadata against current wanted items and stores ranked
wanted-item candidates with matched fields. Ambiguous review decisions require
an explicit wanted-item selection before import, or can be skipped or rejected;
the API rejects candidate-bearing import reviews that are imported without a
resolved `wantedId`. Each review stores explainable evidence in JSON metadata:
source-file facts, filename parsing, local OPF/embedded/audio metadata,
download-client context, wanted-item candidates, a confidence label, and the
policy reason that forced manual review.

Manual imports, completed-download imports, and individual or bulk import-review
decisions share the same organization policy. `importMode` supports `copy`, `move`,
`hardlink`, and `hardlinkOrCopy`; `conflictAction` supports keep-both renaming,
replacement, skip, and fail behavior. Successful imports record the chosen mode,
conflict action, replacement path, and hardlink status in file metadata, and
wanted items are marked imported only after the destination file is persisted.
Future work should add richer profile-aware organization rules and stronger
rollback for failed downstream metadata sync.

- `books-ebook`
- `books-audiobook`

Default roots are:

- `/data/media/books/ebooks`
- `/data/media/books/audiobooks`
- `/data/torrents/books`

## Stabilization boundaries

Acquisition operations hold one immutable client/configuration generation. Settings
updates atomically publish a new generation; in-flight operations keep their
original clients. Download mutations bind external IDs to a client; legacy callers
without a client may mutate only an unambiguous external ID.

Library scans call an observation-specific store method. The scan updates physical
file evidence and keeps fresh local metadata under `scanEvidence`; it preserves
existing names, wanted/download links, source paths, and Calibre metadata.

Verified completed imports retain `verifiedDownload` evidence (client, external
ID, SHA-256) in file metadata as a compatibility projection of the durable
operation ledger below. Cleanup compares fresh client inventory and all required
filesystem hashes before requesting deletion.

`GET /api/v1/wanted?view=library` includes tracked imported/unmonitored books and
excludes removed/ignored entries. `cutoff-unmet` retains its separate membership.
Unknown views return 400. Direct book/file lookup bypasses collection caps; collection pagination remains
planned. System status reports build version/commit/time, active authentication,
and the applied migration filename/number. Local builds without an injected build
timestamp report `unknown` rather than the request time.

### Authentication configuration transactions

The auth store commits the single-user credentials, session revocation and
`auth-config` resource together. In-memory enforcement changes only after commit.
A database advisory lock serializes configuration and session creation; a session
must still match the credential hash verified at login. Credential updates revoke
existing sessions, and session validation verifies the stored user identity.
Environment-owned method/credentials are exposed as lock flags in auth status;
the web settings form disables those controls and the API rejects overwrite attempts.

### Direct native book lookup

`GET /api/v1/wanted/{uuid}` retrieves one tracked book independently of collection
limits. Removed/ignored/missing rows return 404; malformed IDs return 400; storage
failure returns 503. `GET /api/v1/library/files?wantedId={uuid}` filters file
associations before the result limit. Book routes use these queries and preserve
the distinction between missing data and a retryable service failure. General
collection pagination and compatibility reconciliation remain separate stabilization
work; direct author and native presence contracts are described below.

## Durable completed-import operations (unreleased)

Migrations 0030–0031 add `file_wanted_links`, `file_download_links`,
`import_operations`, `import_operation_files`, and reconciliation reports.
Legacy JSON identifiers backfill only unambiguous relationships. Invalid or
ambiguous identifiers remain in `import_reconciliation_issues`; migration does
not manufacture verified receipts. Existing JSON and `downloads.imported_file_id`
remain compatibility projections. Book presence and book-scoped file queries
read the relational links.

Native completed imports persist source/destination paths, sizes, SHA-256 hashes,
mode, and book/download identity before file transfer. A per-download operation
and expiring, renewable lease fence concurrent workers. Retries use the original
plan and verify already-published bytes. Publication is exclusive and never
truncates a concurrent file. Files, relationships, wanted status, download status,
and committed operation state become visible in one Postgres transaction. A
trigger prevents scanners and compatibility writers from registering unfinished
manifest destinations. This is recoverable coordination across the filesystem
and database, not a shared filesystem/database transaction.

`GET /api/v1/library/import-recovery` returns up to 100 recent operations
(unfinished first), up to 100 unresolved legacy link issues, and total unfinished
and unresolved counts. `POST /api/v1/library/import-operations/{id}/retry` resumes
only that saved plan; it accepts no path or identity overrides. These routes use
the same authentication boundary as other library APIs. The Imports page exposes
plans, failures, attempts, cleanup state, and retry.

Cleanup separately records blocked/eligible/cleaned state. It rechecks current
client inventory, all saved file hashes (including manifested sidecars), relational
links, and source/destination separation. Remote deletion failure leaves the
committed import intact and records its error. The singular imported-file field
continues serving older clients.

Completed imports use exact qBittorrent/Transmission file inventories, including
explicit selection state. SABnzbd's successfully completed history record supplies
the finalized extracted directory. Unknown inventories fail closed. Required
media and sidecars retain relative disc paths in the manifest. Migration 0032 adds
per-file wanted identity so an explicitly reviewed pack can map to several books.
Automatic grouping checks local title/author/ISBN evidence, audio album identity,
and numbered disc/chapter layout; conflicting or uncertain sets require review.

`POST /api/v1/library/import-reviews/{id}/preview` accepts per-file
`mapping: [{relativePath, wantedId, exclude}]`, transfer mode, conflict policy and
`confirmIdentity`. It returns the complete operation preview and a fingerprint
without creating directories or records. Resolving the review requires that
fingerprint as `previewToken`; changed source content or destinations invalidate
it. Explicitly retained book/sidecar files prevent cleanup. Skip/reject dispositions
are scoped to the client/download and persist across polling. Resolving with
`action: "reopen"` returns a skipped/rejected payload review to pending.

This engine covers native completed imports, reviewed completed payloads and
native manual imports. Calibre handoff and completed-download replacement remain
S09 work. Existing single-book Calibre imports retain their previous behavior
and cannot claim a native verified-cleanup receipt.


### Journaled native import staging

Migration 0033 records each file's lease-specific staging path before transfer.
Native copies write exclusively to that path, check manifest size/hash, sync, and
renew the operation lease before exclusive publication. Recovery reloads the
journal after claiming the operation and reclaims only its recorded regular file
inside the destination parent. It never sweeps directories by prefix. Missing
stages are safe to retry; symlinks or forged journal paths require review.
Directory synchronization precedes clearing the journal. Imports displays pending
temporary paths alongside their manifest files. Unrecorded stages from older
versions and Calibre operations require separate operator investigation.


### Manual import and replacement recovery

Migration 0034 allows `source_kind=manual` operations without invented download or
wanted associations. Request scope and source-manifest hashes identify retries;
an unfinished request resumes its saved destinations even after settings change.
A moved source can be absent when retrying a committed operation. An unassociated
manual file remains valid, and adopting/replacing an existing file without a new
book assignment preserves its established canonical identity.

Manual source files and configured same-basename extras enter the manifest. Move
copies/verifies/commits first, then records source removal per file. In-place
adoption never removes the file. A download-linked manual request cannot move a
seeding source. Explicit format mismatches are rejected before transfer.

Replacement records the previous size/hash and a recovery path in the plan. Only
a verified new stage permits retirement of the previous name; an exclusive hard
link retains the old inode first. Both the new content and previous copy are
checked before database commit. Pending replacement destinations are hidden from
native file list/detail queries. The previous copy is discarded or recycled only
after commit. A configured recycle-bin failure retains it; it never degrades to
permanent deletion. Completed manual cleanup has its own recorded state and can
be retried without repeating the import.

Publication and destructive cleanup hold the import ownership row lock across
the filesystem mutation, closing the check/act lease-takeover window. This
coordinates Librarry workers; it is not an atomic filesystem/Postgres transaction.
The recovery API includes committed manual operations with pending cleanup in
its unfinished count; the UI offers Retry cleanup and shows retained backup paths.

### Durable acquisition submission

Migrations 0035/0036 add acquisition intents with unique active book/release
reservations. Short Postgres claims precede remote submission; no transaction stays
open during client HTTP calls. Intent identity includes the selected client endpoint
hash, requested book, and content hash or hashed release URL/payload. Source URLs,
provider credentials and uploaded payload bytes are not retained in the intent.

A request moves from submitting to accepted or uncertain. Expired submitting leases
become reconciliation work, never another send. Exact accepted receipts are saved
before the ordinary download row, so retry can repair a failed download write.
Receipt replay preserves current progress/import state. Authoritative intent book
links survive tagless client observations. A raw manual grab and a book-associated
worker cannot reserve the same exact release concurrently.

Reconciliation reads only the original client, bypassing aggregate cached/partial
lists. Exact intent tags/infohashes establish identity; title similarity does not.
SABnzbd uncertainty requires explicit operator attachment when no ID was received.
Unsuccessful checks retain state and exponentially back off to five minutes.
Operator-confirmed attachment/release is available from Activity and native recovery
routes; release itself sends no remote mutation. Changing the configured endpoint
prevents automatic reconciliation against a different server. Existing accepted
imports can reserve a different upgrade, while replay of the original is a no-op.

The ledger coordinates all production paths through `acquisition.Service.Grab`.
Migration 0040 stores a sanitized release-selection snapshot before submission
and records independent bookkeeping completion. After saving the accepted receipt,
a short transaction links the exact client/download to that receipt and release,
updates eligible wanted status, and inserts one `release_grabbed` history event.
Receipt locking serializes retries. A database failure leaves acceptance durable
and exposes **Finish recovery** in Activity; it cannot authorize a second send.
Reconstruction of a missing download projection restores the association without
repeating history. Receipt replay includes removed rows, so it cannot resurrect a
removed row by confusing it with a failed persistence write. Existing migrated
intents have no manufactured selection or new historical event.

Acquisition does not update `wanted_items.current_release_id/score`. Native import
commits that projection and one `book_imported` event per mapped book in the same
transaction as all file/link/status records. The installed score comes from the
saved acquisition decision, not a later search. Unfinished acceptance bookkeeping
blocks import commit until recovered. A pack cannot lend one book's release score
to another mapped book; unknown completed imports and manual replacements clear
unproven installed-release identity. Failed upgrades preserve imported status, and
failed-download blocklisting uses the download's saved release ID or exact hash,
never an unrelated installed release. Legacy installed projections remain for
separate review rather than being guessed from historical searches.

Worker grab counts and compatibility producers exclude receipt replays. Native
notifications are now captured by the committing transaction, as described below.
Compatibility webhooks now share that durable delivery path. No exactly-once
remote-delivery guarantee is made.
Broader worker/live-client and legacy qualification remain S10/S21 work.

### Persisted scan execution and local presence

Migration 0037 adds scan jobs, per-job root identities, a durable directory/file
queue, staged absence evidence, and file presence/root/device/last-seen fields.
Directory enumeration streams pages into the queue with idempotent inserts.
Per-entry observation and progress acknowledgement share a short ownership-fenced
transaction. Hashing and metadata extraction occur outside it, with lease renewal
and mutation checks. The scanner shares import destination locks and excludes
unfinished publications. Existing canonical/original-root aliases preserve proven
file identity and manual metadata instead of creating another record.

The scheduler advances queued/expired jobs. A cancelled active worker stops at its
next acknowledgement boundary; expired cancellation is finalized after restart.
Failed jobs retain the queue for explicit retry. After full discovery, keyset
reconciliation checks previously observed files. Root identity and per-file device
evidence distinguish missing files from unavailable roots/nested filesystems.
Absences remain staged until completion, are checked again after any batch pause,
and apply atomically only if file identity/version and import visibility still
match. A failed final write rolls back presence changes and remains retryable.
Successful jobs discard queues while retaining summaries and root evidence.

`presence_state` is local filesystem evidence, separate from `import_status`.
New verified imports are present; historical records are unknown until observed.
Native files expose these observations. The move/repair and native book evidence
sections below describe subsequent work; Readarr parity remains unqualified.


### Library repair evidence

`GET /api/v1/library/repair-preview?cursor=...` produces a read-only page of repair
findings. An opaque validated keyset cursor traverses files, imported downloads
and committed import operations. Each request uses one repeatable-read read-only
transaction, checks at most 100 entities and returns `checked`, `section`,
`generatedAt`, `findings` and an optional `nextCursor`. Empty findings do not mean
completion while a cursor remains. Concurrent changes can affect later pages;
this is not a persistent frozen audit artifact.

Findings cover unresolved/missing exact associations, same-content record groups,
possible move candidates from completed scans, unverified legacy audiobook
completeness and discrepancies against historical import manifests. Samples expose
up to 20 related records and total counts. Evidence uses selected fields, never
raw file metadata. This endpoint does not touch media or clients, mutate records,
create verification receipts, or apply proposed actions. Existing authentication
middleware protects it. The scan completion path separately performs verified
native move reconciliation as described below.


### Scan move identity boundary

Migration 0038 records `library_scan_discoveries` with the original authoritative
projection of a new scan-created file. Observations can update scan evidence,
but manual identity/provenance changes make the discovery ineligible for folding
into an older row. Files retain a `scan_file_stamp` (device, inode, size and
nanosecond mtime) bound to the SHA-256 computed in a discovery batch.

After discovery and absence reconciliation, completion selects exact, globally
unambiguous SHA-256/size/format pairs: an absent original and an unassigned,
unchanged discovery positively observed in this job. It rechecks filesystem
stamps and roots, then takes path advisory locks and file-row locks. A second
eligibility check observes assignments/import references and row changes before
removing the discovery record and updating the original path. The original
metadata is not rewritten, and book/download links continue pointing at the same
ID. Calibre-owned records/paths remain excluded. Completion, missing publication
and `library_scan_moves` history commit atomically under the scan lease fence.

`GET /api/v1/library/scans/{id}/moves?cursor=...` returns up to 100 historical
old/new paths and retained IDs; `nextCursor` continues by file ID. Job/outcome
`moved` counts describe reattached identities, not filesystem mutations. The
immutable import manifest still names its original destinations. Reattachment
cannot by itself establish a new cleanup receipt, current chapter completeness,
or a unified wanted/book presence state.


### Completed destination replacement

Payload review accepts `conflictAction: replace` after explicit identity
confirmation and a current preview token. Existing destination hashes and saved
recovery paths participate in the plan fingerprint. Matching files and sidecars
are staged and verified before their previous inodes are journaled at recovery
paths. All library projections remain hidden until the complete operation commits.
Publication and commit recheck book ownership; current file IDs and non-operational
manual metadata are preserved while current import provenance records the new
source. Different existing chapter layouts remain operator review work.

Migration 0039 adds `replacement_cleanup_state` (`none`, `pending`, `cleaned`) and
`replacement_cleanup_error`. This state tracks previous-file disposal, separately
from `cleanup_state`, which governs remote download-source removal. Committed
operations with pending replacement cleanup count as unfinished recovery work.
Retry verifies the complete destination set before disposing of recorded backups.
Download cleanup rejects pending replacement cleanup and still requires its
independent whole-set/client/seed evidence. Historical import manifests are not
rewritten to pretend the older version's receipt verifies newer bytes.


### Provider request observations

`GET /api/v1/providers/health` returns configuration and cached request evidence;
`POST /api/v1/providers/{name}/check` explicitly verifies a named provider using
normal application authentication. Hardcover checks its read-only `me { id }`
query; Open Library/Google check one ISBN result. `lastCheckedAt`, `lastSuccessAt`,
nullable `reachable`/`authenticated`, and `retryAfter` describe actual evidence.
The older `checkedAt` remains a snapshot timestamp for compatibility and is not a
remote probe time. System renders the explicit evidence fields.

Each provider has a synchronized observation and a single request slot. Queue waits
are bounded at 15 seconds; clients without a bounded timeout receive one. Explicit
checks reuse observations for 15 seconds. HTTP 429 backoff applies to both checks
and searches. Snapshot reads and unavailable credentials perform no remote IO;
caller cancellation does not become an outage. Error classifications exclude raw
response bodies and credential-bearing URLs. Missing response lists/counts are
errors, while valid empty lists remain valid responses. This state is process-local
and resets with provider instances. Richer traversal remains separate S12 work.


### Exact metadata fallback

The metadata service collects primary results before considering Google Books,
independent of provider registration order. Known language/format conflicts are
filtered before merging. A checksum-valid ISBN or normalized full-title match
suppresses fallback; otherwise the Google adapter sends `isbn:` or a quoted
`intitle:` search and validates each returned record. Only book queries qualify.
The service repeats exact-result validation at its provider boundary. Canonical
ISBN-13 queries reach all adapters while the response retains the user's query.
Exact matches rank ahead of fuzzy primary results; partial provider errors remain
visible. Existing merge logic retains provider aliases.

Google requests full projection with explicit fields because the lite projection
omits industry identifiers. Subtitle, all authors, language, original identifiers
and volume/source IDs are retained; `saleInfo.isEbook` is the only concrete format
evidence used. Unknown format is not assigned from the requested format. Empty
results remain arrays. Broader work/edition matching and persisted raw records
remain separate S12/S13 work.


### Provider result cache

Each immutable metadata service owns an independent bounded cache per provider.
Keys include every query field after service normalization; equivalent validated
ISBNs use the canonical provider query. Values are serialized snapshots, so
merging a result or mutating a returned nested slice cannot corrupt later hits.
Successful nonempty/empty responses expire five minutes/30 seconds after fetch,
without sliding expiration. LRU eviction caps each provider at 128 entries and
2 MiB of serialized result/key data (plus bounded map/object overhead); oversized
responses/keys bypass storage. The complete response still reaches the caller.

A cancellable provider slot rechecks the cache after waiting, preventing duplicate
successful fetches for concurrent identical misses. Waits are bounded at 15 seconds;
provider request timeouts/backoff still apply. No detached fetch outlives its
caller. Errors invalidate only that provider's entries and are not cached; caller
cancellation, unsent waits and quota backoff preserve unexpired successful keys. A generation fence prevents an
in-flight success from repopulating entries invalidated by an explicit failed
health check. Cached reads never advance observed request/success timestamps.
Credentials are not cache keys or values; replacing the service resets all caches.


### Complete author bibliographies

`metadata.Service.AuthorBibliography` is separate from ranked/limited search and
its caches. It dispatches only to the provider identified by a verified author key.
The author monitor uses this operation before any wanted/review mutations. The
legacy request `searchLimit` remains accepted for compatibility but cannot truncate
a bibliography. The operation has a two-minute deadline and a 10,000-record guard;
either limit returns an explicit failure instead of applying partial results.

Open Library verifies `/authors/{id}.json`, then follows explicit limit/offset
pages with a stable declared size and unique valid work IDs. Hardcover verifies
`authors(where: id)` and queries books by contribution author ID, using ascending
book-ID cursors until an empty page. Per-page request observation/backoff includes
shape and identity validation. Neither adapter uses an external next-link URL.
Missing/changed/duplicate data is an error, not an empty successful bibliography.
External mutations do not have transactional snapshot guarantees.

Author search clustering and subscription matching use stable IDs/aliases rather
than matching names. `Author.role` retains Hardcover contribution evidence;
non-writing/unknown credits enter review. Only candidates passing metadata/credit
filters participate in first/latest policy selection. `Work.firstPublishDate`
retains original dates independently of `Edition.publishedDate`; work-only records
have unknown format and no fabricated edition ID. Failed bibliography, filter,
policy or wanted/review persistence keeps the subscription unsynced for retry.
Existing success-history insertion remains best-effort and is not an outbox.


Author additions use an internal `OnlyIfUntracked` flag that cannot be set through
JSON. The store serializes same-provider work/format creation, resolves existing
work aliases, and serializes known canonical work identities before checking for
any existing tracking. Existing rows—including removed/unmonitored and legacy
synthetic edition rows—return unchanged. This prevents automatic resurrection and
losing a selected edition; explicit user adds keep their existing behavior.


### Shared provider HTTP budget

The API process constructs one `providerhttp` client for metadata and Hardcover
import lists. Its transport serializes requests per known provider host, spaces
starts by one second, and records retry deadlines from 429/Retry-After and exhausted
named RateLimit buckets. Legacy quota headers are fallback evidence. Different
hosts have separate slots/deadlines; budget keys contain no token or query text.
Cloned bounded clients retain the shared transport instance. Explicitly injected
custom clients remain useful for isolated adapter fixtures.

`NotSentError` distinguishes quota refusal/canceled budget waits from a failed
network attempt. Metadata observations keep their actual checked/success times;
health combines existing request evidence with the shared quota deadline. Cached
successful data remains valid through ordinary TTL during rate limiting. Credential,
shape and connection failures still invalidate cached queries. No background fetch
or retry is detached from its caller. This is per-process coordination, not a
cross-replica quota service or persistent account budget.


Import-list fetchers return a complete validated traversal or an error before
wanted mutations begin. Hardcover uses stable membership-ID pagination and checks
list identity, declared count and available modification timestamps on each page.
Remote edits are detected when these signals change; the API provides no snapshot
transaction. Bounds fail explicitly rather than returning truncated success.
Native list additions use the same work/format add-only lock as bibliography
monitoring. Root and initial monitoring settings join the creation transaction;
existing tracking is returned unchanged. A list success timestamp advances only
after all required additions succeed, and timestamp failures are returned in the
outcome. The scheduled task reports errored outcomes as task failures. Per-book
commits make a retry idempotent without holding one transaction over network IO.
Search-on-add retains its existing best-effort behavior pending durable follow-up
work. Manual runs remain synchronous; long lists may exceed an upstream proxy's
request timeout, while scheduled runs have a ten-minute job budget.


Hardcover search now resolves work search hits into a bounded batch of work and
default ebook/audio edition records. Exact ISBN requests query edition records
directly with locally verified ISBN equivalence. Bibliographies retain one
preferred edition for the requested format per work; Any-format ordinary search
may expose both ebook and audiobook results. Provider errors/invalid relationships
fail the lookup, while missing supported defaults preserve unknown work evidence.
Each GraphQL response has a 4 MiB body bound.

Native edition evidence includes cover, duration and contributor roles in the
normalized response and stored provider-record JSON. Those stored records still
contain normalized search results; durable original response snapshots remain
separate S12 work. Concrete edition IDs use their own provider aliases and never
acquire the work/format placeholder alias. Previously conflated legacy aliases
are retained for review, not silently rewritten. Add-only monitoring continues to
reuse the existing tracked work/format regardless of a newly selected default.
Merge clusters require compatibility with every member so a work-only candidate
cannot join incompatible ebook/audio editions indirectly.


### Native book presence evidence

`WantedItem.stateEvidence` contains file state/reason/counts, download availability,
quality-profile availability and an explanatory message. `derivedState` adds
`incomplete` and `unknown`; stored lifecycle values remain unchanged. Page-scoped
file/link/manifest reads share one repeatable-read transaction. Ebooks require at
least one present matching-format media record with available/imported status and
no unfinished publication at that path. Audiobooks require all required media in
one committed per-book manifest, matching current IDs, links, sizes, hashes and
presence. Partial known loss is incomplete; unverified legacy audio or content
changes are unknown. Complete alternate imports win over incomplete old sets.
Sidecar presence does not determine playback availability or relax cleanup rules.

File observations are persisted evidence, not live filesystem access. Proven
renames retain file identity; manifest paths remain immutable. An unavailable
mount retains prior observations until a successful scan can reconcile them.

`LiveDownloadEvidence` retains successful client lists and labels their overall
availability fresh/partial/unavailable/notConfigured. Saved bookkeeping may
annotate those exact client/ID matches as already imported, but never supplies
rows absent from live results. Native annotation bounds this remote read to five
seconds. A current new download can establish downloading during file recovery;
already-imported sources and failed/deleted client states cannot. Missing without
complete client evidence becomes unknown; positive file evidence survives client
outages with a warning. File database failure explicitly marks evidence unavailable.
Quality profiles are loaded once per page; cutoff membership also requires
positive file evidence. Global cutoff paging and author-policy/Readarr adoption
remain separate work. The general Activity/download endpoint is unchanged.


### Scheduled evidence and fair checks

Migration 0042 adds `wanted_items.last_monitor_checked_at`,
`last_upgrade_checked_at`, and `author_subscriptions.last_sync_attempt_at`.
Scheduling orders by the check/attempt timestamp, falling back to previous
successful activity, then deterministic creation/name and UUID tie-breakers.
A checked row advances before remote work, including skips and failures. Check
backoff is the smaller of 15 minutes and the requested interval; successful
search/sync timestamps retain their independent interval. Force bypasses waiting.
Check writes leave `updated_at` unchanged because it is an owner revision fence.
No success timestamp is written for a skipped or failed search/sync.

Monitor candidates include monitored imported books. Page evidence distinguishes
known gaps from complete copies and uncertainty. Upgrades require a complete
present copy that actually needs the configured cutoff. Provider IO is followed
by an automatic-grab preflight that rechecks settings, monitoring, current media
and live download evidence. Acquisition reservation also takes the wanted row
lock and rejects stopped/removed automatic requests. The reservation is the
linearization point: a stop after submission cannot recall the external request.
Manual grabs retain their explicit override scope. Overlapping worker/provider
reads are not serialized by these scheduling timestamps; durable acquisition
reservations continue to prevent duplicate adds.

Feeds page all eligible monitored books by UUID in batches of 200. Owner changes
between pages take effect on later reads; there is no frozen whole-run snapshot.
Book-specific evaluation settings load only after a release-title candidate
matches. Run responses retain at most 1,000 match details, set `matchesTruncated`,
and explain that evaluation counters cover the full run. Release decisions remain
stored per book. Per-item skipped-run history is not yet durable; returned skip
reasons and saved run summaries must not be described as a complete event log.

Feed release observations persist decisions without updating the full indexer
search timestamp, so repeated RSS matches cannot postpone due searches.

Full-search decision persistence also locks/checks the book revision captured
before provider IO. A changed revision rejects stale decisions and does not
advance the successful-search timestamp.

### Explicit upgrade selection

`POST /api/v1/wanted/upgrades` treats nonempty `wantedIds` as the complete
selection, independently of the scheduled `limit`. Up to 200 UUIDs are accepted;
duplicates are normalized, input order is retained, and every selected book is
checked or returned with a skip reason. Unmonitored/removed books and checks not
yet due are reported rather than silently omitted. Force bypasses timing only;
file evidence, owner monitoring and acquisition safeguards still apply.

Invalid JSON, unknown request fields, invalid IDs, oversized selections and
nonexistent selected books fail with HTTP 400 before starting work. A selection
snapshot validates membership before the run; owner settings are re-read during
processing. An empty selection retains the queue batch contract (50 by default,
200 maximum). This endpoint is synchronous and does not provide durable resume
or an all-matching collection job. Global paging and bulk jobs remain S15 work.


### Shared recorded file evidence

Migration 0043 introduces `librarry_book_file_evidence(uuid[])`, the shared SQL
projection behind native per-book file evidence. A nonempty UUID array limits the
scope, an empty array returns no records, and an explicit SQL NULL evaluates the
recorded collection. Links, current file observations, pending publication and
committed required-media manifests are evaluated in one statement snapshot.
The existing ebook/audio evidence ranking and explanatory reasons are retained;
complete alternate imports outrank older incomplete manifests. This is recorded
evidence, not a live filesystem probe.

The projection is joined before native book collection filters, counts and
pagination. Quality cutoffs and live download evidence remain separate inputs to
derived book state; the collection contract below combines them.


### Native paginated book collection

`GET /api/v1/library/books` returns `books`, `total`, `filtered`, global state
`counts`, `recordedFiles`, `downloads`, `observedAt` and an optional `nextCursor`.
The tracked library excludes removed/ignored rows and retains imported and
unmonitored books. `recordedFiles` counts file records, including retained files
from removed books; it is not a count of bytes verified present now.

Parameters are `q` (literal case-insensitive substring, maximum 256 UTF-8 bytes),
`format` (all/ebook/audiobook), `monitor` (all/monitored/unmonitored), `state`
(all/missing/incomplete/unknown/downloading/cutoffUnmet/downloaded/unmonitored),
`sort` (status/title/author/added), `limit` (1–100, default 100), and `cursor`.
Unknown/duplicate parameters, invalid values and cursors bound to another filter
or sort return 400. Failure to read persistence returns 503, not an empty library.

Totals and state counts describe the entire tracked collection; `filtered` applies
all requested filters. Profiles, file/manifests, counts, page membership and
manual overrides/author links share one repeatable-read database snapshot.
One live-client observation precedes that snapshot and has its existing five-second
budget. No per-book client request occurs. Client outage/partial evidence remains
explicit; present verified media still wins and unverified gaps become Unknown.
Quality uses the same normalized profile cutoffs and saved installed score as
book details, including the legacy-score exemption.

Sorts use PostgreSQL lowercase plus C collation and a UUID tie-breaker. Added sorts
newest first; status sorts missing, incomplete, unknown, downloading, cutoff unmet,
downloaded, then unmonitored. Opaque keyset cursors carry the previous sort values
and bind the filters/sort. They survive process restart without server memory.
Each request is internally consistent; the whole traversal is not a frozen
snapshot. Concurrent edits or changing client evidence can move books between
pages; refresh from the first page when that matters.

Derived states are materialized before cursor filtering and the bounded page is
materialized before detail hydration. Measured plans otherwise compared 100 million
rows on a late page. These read transactions disable JIT and force custom plans
locally; they do not change database-wide or pooled-session settings. The 10,001-book,
10,003-file fixture measured 61.400 ms p95 for local service reads, including
counts/hydration and excluding external client IO, on Apple M5 Max/ARM64 with
Colima Postgres 16.15. Author/review collections, compatibility surfaces and durable
all-matching bulk jobs are not covered by this endpoint.


### Native author subscription collection

`GET /api/v1/library/authors` returns `authors`, `total`, `filtered`, `downloads`,
`observedAt` and optional `nextCursor`. Each row embeds the subscription settings
plus `identityLinked`, `totalBooks` and state `counts`. It accepts `q` (literal
case-insensitive name/provider/key substring, maximum 256 UTF-8 bytes), `format`
(all/ebook/audiobook), `status` (monitored default, unmonitored or all), `limit`
(1–100, default 100) and `cursor`. Removed subscriptions are excluded even from
all. Unknown/duplicate/invalid filters return 400; read failures return 503.

Subscriptions sort by lowercase name, format and UUID with C collation. Cursors
bind filters, have a distinct namespace from book cursors, and survive restart.
A traversal is not frozen across requests. Counts and page settings share the
book collection's repeatable-read snapshot, quality projection and one bounded
client observation. Book counts are computed for identities on the current page
against all active tracked books; no capped wanted/file arrays or per-author
client requests are used. Duplicate writer roles cannot double-count a book.

Membership requires a unique canonical provider-record author identity and a
recorded author/writer link, with the subscription's format. Narrator-only,
removed/ignored and manually overridden author assignments are excluded.
Ambiguous legacy provider mappings remain unresolved. Aliases and ebook/audio
subscriptions retain independent settings rows; these are not unique-person
counts or an all-authors catalog. Author metadata review and compatibility
collection readers remain separate work.


### Native metadata Review collection and confirmation

`GET /api/v1/wanted/metadata/review` accepts `q` (literal case-insensitive
substring across title/author/source provider/profile, maximum 256 UTF-8 bytes),
`format` (all/ebook/audiobook), `limit` (1–100, default 100) and `cursor`.
The response preserves `items` and `generatedAt` and adds global `total` reviews,
`conflictCount`, filtered review count `filtered`, and optional `nextCursor`.
Invalid/unknown/duplicate parameters return 400; unavailable persistence returns
503, not an empty review queue. Imported and unmonitored tracked books participate;
removed/ignored books do not.

Reviews and direct metadata detail now use the same batched provenance reader in
repeatable-read transactions. Field comparison preserves Unicode letters/marks/
numbers and canonical Unicode composition. Provider format is informative, because
the wanted format is the owner's acquisition target. Conflicts without a current
canonical value still need an explicit metadata choice in book details.

The collection projects all active book metadata per request to obtain exact
counts, rather than maintaining an eventually consistent review index. There are
no per-book database or provider round trips. Processing and memory still grow
with the library; only the response is bounded. Sort keys are PostgreSQL lowercase
title/author with C collation and UUID ties. Review cursors have a distinct
namespace, bind filters and survive process restart; multi-request traversal is
not a frozen snapshot. Evidence revisions are calculated for the returned page,
not every counted book.

`POST /api/v1/wanted/metadata/review/confirm-canonical` accepts 1–200 UUID entries
in `wantedIds`, deduplicates them, and validates every requested identity before
mutation. Optional `revisions` maps exactly those IDs to their displayed evidence
hashes; the native UI supplies it. A stale hash or concurrent owner change returns
409 and rolls back every confirmation. Missing/invalid selections, mixed `all`
and IDs, malformed/unknown JSON fields or multiple JSON objects return 400.
Resolved/removed/ignored books and books without confirmable canonical values
are reported as skipped.

The transaction locks selected books in UUID order, updates acceptance reasons
on existing override rows, and inserts only when the observed override was absent.
It never replaces an existing value. Updating existing rows rather than upserting
also prevents resurrecting a concurrently cleared override; snapshot serialization
failures become a retryable 409. Native corrections and clears take the book lock
first as well, so their lock order cannot invert confirmation. Those owner actions
serialize behind confirmation and remain the final value/clear. Direct metadata
correction remains available.
Older API clients without revisions explicitly confirm the current server snapshot.
Legacy `all: true` now processes the entire active review selection atomically,
rather than the first 200 rows. It is synchronous and can be expensive; it is not
a resumable all-matching bulk job and is not the native UI's selected-page action.


### Author candidate review collection and decisions

`GET /api/v1/authors/metadata/review` accepts `q`, `format` (all/ebook/audiobook),
`status` (pending default, wanted, ignored, all), `limit` (1–100, default 100) and
`cursor`. It retains the `reviews` array and adds `total`, `filtered`, global
status `counts`, `nextCursor` and `observedAt`. Unknown/duplicate filters or invalid
limits/cursors return 400; unavailable persistence returns 503. Older callers
requesting more than 100 must page. Read-only repeatable-read transactions bind
counts and page membership within each response. Newest-first creation time and
UUID provide a stable tie-breaker. Cursors bind normalized filters, and remain
valid after process restart; separate page requests are not a frozen snapshot.

Each review carries a revision of its complete saved evidence/settings.
`POST /api/v1/authors/metadata/review/{id}/resolve` accepts one strict JSON object
with `action` and optional `revision`. A row lock serializes competing actions.
Pending decisions require matching revisions when supplied; stale evidence or a
differently resolved action returns 409. Legacy requests without revisions use
the current saved candidate. Retrying the same resolved action returns
`replayed: true` without a second mutation/history entry.

Wanted creation/reuse, review resolution and history share one transaction.
Creation uses the candidate's captured root/profile/tags; add-only reuse returns
`alreadyTracked: true` and retains all existing book settings/status/monitoring.
Ignore changes no book. The existing create helper now supports a caller-owned
transaction and hydrates the response before commit. No schema migration or
external provider/acquisition request is required. Legacy dashboard summaries
still use a bounded reader; they do not determine mutation membership.


### Native file collection

`GET /api/v1/library/files/collection` accepts `q` (up to 256 bytes), optional
`wantedId` UUID, `format` (all/any/ebook/audiobook), `presence`
(all/present/missing/unknown), `sort` (path default, title, updated), `cursor` and
`limit` (1–100, default 100). It returns `files`, `total`, `filtered`, `counts`,
`nextCursor` and `observedAt`. Counts are unfiltered within the optional book
scope; the filtered count applies search/format/presence. Each file includes
`wantedIds` from `file_wanted_links`, including multiple associations, independently
of legacy metadata hints. Empty arrays and maps are normalized at the boundary.
Invalid/duplicate filters return 400; unavailable persistence returns 503.

Read-only repeatable-read transactions bind counts, page membership and file
records within each response. Cursors bind normalized filters and sort, with
UUID tie-breakers and C collation for text keys; later page requests are not a
frozen collection. Scope queries exclude any path claimed by an uncommitted
import operation. Sorting/counting omit file metadata until the bounded page has
been selected. Reads perform no filesystem probes, provider calls or mutations.
No new schema migration is needed. The old `/api/v1/library/files` and compatible
readers retain their existing bounded contracts; native browsing and rename
preview now use the complete collection. Calibre refresh batching, broader
compatibility and durable all-matching jobs remain open.


### Durable standalone file renames

Native rename apply and compatible rename commands reuse manual import operations
with `metadata.renameFileId`, one verified media file, and move-after-commit
cleanup. Migration 0044 permits one unfinished rename per file identity. Source
and destination reservations prevent another import or scan from claiming the old
source while cleanup is pending. The visibility transaction updates the existing
file's location and byte evidence and inserts `file_renamed` history. It does not
rewrite file metadata, replay legacy JSON associations, or update wanted rows.
The rename manifest deliberately has no wanted ID: it cannot manufacture a new
complete-book manifest from one selected chapter.

Preview includes `revision` and, for recovery, `operationId`. Apply accepts an
optional `revisions` map keyed by every selected file ID; the native UI always
sends it. Changed paths, metadata, destination collisions or naming output require
refresh before mutation. Legacy callers may omit revisions. A saved operation
retains its original destination across settings changes. JSON request decoding
rejects unknown fields, multiple values, empty selections and oversized bodies.
Existing destination bytes are never overwritten, including legacy overwrite
requests.

Original import manifests remain immutable. Receipt replay and completed-download
cleanup resolve their destination through later committed rename records for the
same file ID, hash and size, then check the current file row and actual bytes.
Unjournaled path edits or changed associations cannot authorize download deletion.
Ordinary download identity, inventory and seeding gates still apply.

Known multipart/companion layouts are retained by the per-file rename action.
It detects persisted sets and linked audio chapters, chapter/disc naming and
nearby companion/audio files. Complete-set folder moves use the separate book
action below. Rewriting individual chapter names and sidecar references remains
unsupported.


### Complete recorded book folder renames

`POST /api/v1/library/books/{id}/rename/preview` captures every member of a complete
current non-rename import for that wanted book. Apply at the matching `/rename`
route requires its revision. The group must account for all linked media with
exclusive ownership, matching format/hash/size, and no pending original cleanup.
The target is the book directory from current naming and root settings. Every
current basename and relative disc path is preserved; CUE/M3U/OPF references and
both folder inventories are checked before publication and visibility commit.
Unrecorded files, symlinks, unsupported references and overlapping roots fail.

Migration 0045 adds `file_rename_claims` to reserve every media identity, backfills
active standalone renames, and releases claims only when cleanup finishes. It
adds `rename_origin_file_id` for sidecars, which have no tracked file row. Group
metadata retains `renameWantedId`, `renameOriginOperationId` and the destination
folder; per-member wanted IDs remain unset so rename operations do not replace
original completeness evidence. Commit locks media rows and current book links,
checks exact membership, updates existing paths/history in one transaction, and
then performs lease-fenced source cleanup. Saved plans resume without reinterpreting
naming settings. A per-file request cannot silently expand to a saved book plan.

Immutable original receipts follow ordered committed rename and verified scan-move
edges for the same file identity and bytes. Sidecars follow their original manifest
identity through committed folder moves. Arbitrary path edits remain insufficient.
This supports original receipt replay and download cleanup without rewriting
historical manifests. Actual destination hashes and ordinary client/source gates
remain required. Book plans outside proven current folder layouts, changed chapter
sets and Calibre-managed roots still need operator review or later recovery work.


### Calibre protocol identities and authentication

The Content Server add-book response uses `book_id` for subsequent metadata,
conversion and deletion calls. Its `id` field is only an echoed upload identifier
and may be a string. Librarry rejects responses without a positive `book_id`.
Conversion job identities are non-negative: zero is preserved through JSON
metadata and status URLs, while missing/malformed/negative IDs remain invalid.
Conversion book-data must name the requested book; terminal responses must
explicitly include an outcome.

Authentication is negotiated against the read-only `/ajax/library-info` route
using the configured URL prefix. The actual request receives Basic credentials
only after a Basic challenge, or Digest credentials calculated for its own method
and URI using `github.com/icholy/digest`. The client has no shared challenge cache
that could mix roots or retain rotated credentials. Uploads use section readers
over the same open file for streaming and Digest body hashing. Authentication
probes never call mutation routes, writes are not automatically replayed, and
redirects are not followed. Host URLs cannot embed credentials/query/fragment.

These contracts are derived from upstream
[add-book source](https://github.com/kovidgoyal/calibre/blob/master/src/calibre/srv/cdb.py),
[conversion source](https://github.com/kovidgoyal/calibre/blob/master/src/calibre/srv/convert.py)
and [server authentication documentation](https://manual.calibre-ebook.com/generated/en/calibre-server.html),
and exercised against a disposable real Calibre 8.5 server. Durable upload
acknowledgement, one-time terminal status consumption and local commit recovery
remain a separate required handoff state machine.

### Shared scheduled-worker ownership and history

With database persistence, the scheduler claims a task-specific Postgres session
advisory lock before invoking a registered worker. Manual System Tasks requests
claim synchronously, so another API process returns 409 while the owner runs.
Scheduled claims additionally check `worker_tasks.next_run_at`; starting another
API or restarting one cannot immediately repeat a recently completed scheduled
pass. Manual triggers can override the due time but not active ownership.

Migration 0047 stores the current run/due time in `worker_tasks` and diagnostic
runs in `worker_task_runs`. An active lock, not heartbeat age, establishes running
status. The owner refreshes its heartbeat every ten seconds and cancels its worker
context when coordination fails. A lost session is shown as interrupted, and a
successor marks the abandoned run interrupted before claiming. Completion is
bound to the same run ID and connection. Jobs must honor cancellation; remote
requests already accepted cannot be rolled back by scheduler coordination. The
acquisition/import journals remain responsible for individual side-effect safety.
This is shared scheduler ownership, not an exactly-once external delivery claim.
Use a direct or session-pooled Postgres connection, not transaction pooling.

Manual registry runs join application shutdown alongside scheduled runs. Panics
are recorded as unverified failures without exposing panic payloads. Migration
0050 adds structured counts, bounded operation UUIDs, next action, review timestamp,
and a separate last-success identity/time. Reports with errors become `degraded`
even if the worker returns no top-level error. Last success advances only on a
clean completion. Import cleanup, backup pruning and notification delivery outcomes
contribute to the report. Historic completed states remain historical evidence,
not reconstructed per-item qualification.

Retention keeps 100 successes, protecting current/last-success identities even
with skewed timestamps. Unreviewed failures are never routine deletion candidates.
Up to 500 failures reviewed more than 90 days ago are pruned during each completion;
the current run and domain recovery journals remain. Hourly History Maintenance
also prunes up to 500 eligible reviewed failures across all persisted workers,
including disabled workers, while preserving their current run. Lock-based interruption requires the current owner identity as well
as its PostgreSQL session lock; a recycled PID cannot make an old row active.
An abandoned run has no invented completion timestamp/duration.
`GET /api/v1/system/tasks` reads shared state and returns an unavailable response
when that state cannot be read. `GET /api/v1/system/tasks/{id}/runs` exposes the
retained runs for a registered task with `view=all|unreviewed`, `limit` (1–100)
and `offset`. Count and page use one materialized effective-state snapshot.
`POST /api/v1/system/tasks/{id}/runs/{runId}/review` accepts `reviewed`,
`expectedState` and `expectedReviewedAt`; only inactive failures can be reviewed,
and stale state/review timestamps return 409. System Tasks offers paginated
history and review in an accessible dialog. Direct business
API/compatibility operations still use their domain-level coordination rather
than becoming scheduler jobs.

### Transactional native notification delivery

Migration 0048 adds immutable `notification_events`, per-target
`notification_deliveries`, attempt/action history, and persisted health states.
An insert trigger captures `release_grabbed` and `book_imported` history. A
separate trigger captures the transition into `downloads.failed_at`, avoiding
reliance on the recovery worker's later best-effort history write. Both execute
inside the domain transaction. Health observations serialize by check ID and
atomically capture only ok/unknown-to-unhealthy transitions. No historical
backfill occurs. Domain callback sends have been removed for native targets.

Fan-out records only targets enabled for that event at capture time, their IDs,
names/types and exact `updated_at` revisions. Event payloads allow-list basic
book/release labels and identifiers; raw release URLs, target settings and
credentials are not copied. The sender fetches current settings under a short
shared row lock, refuses changed/deleted/disabled targets and commits `sending`
plus an attempt token before HTTP. Settings may change after a send starts; those
changes cannot retract an in-flight request. No DB transaction spans remote I/O.

A per-delivery advisory session lock serializes send and operator resolution.
Completion writes through the original connection/token. Acquiring an abandoned
`sending` entry marks it uncertain rather than issuing another request. Network
errors, 408/5xx and a lost result save are uncertain. 2xx is acceptance; other
3xx/4xx fail. Redirects are not followed. 429 alone gets automatic backoff
(minimum exponential minutes, respecting Retry-After up to 24 hours) with at most
five total sends. Longer requested waits require operator review. Explicit retry
uses a current target revision and a current delivery revision, and records an
audit action. `X-Librarry-Delivery-ID` and `X-Librarry-Event-ID` remain stable across
retry; webhook event timestamps describe the original committed event.

The shared `notification-delivery` task advances at most 25 entries per pass at a
15-second interval. `GET /api/v1/notification-deliveries?limit=25&offset=0` returns
one repeatable-read page/count without settings; `POST
/api/v1/notification-deliveries/{id}/resolve` accepts retry/accepted/cancel plus
confirmation and expected revisions. The latest send state is shown in Settings
→ Connect; attempt and resolution records remain in Postgres and backups. Explicit
connection tests do not enter the queue. Migration 0049 extends this mechanism to Readarr-compatible webhooks, as described
below.

### Readarr-compatible webhook delivery

Migration 0049 namespaces delivery targets as `native` or `compat`; identical UUIDs
in the two resource tables cannot collide. New notification events capture enabled,
matching `compat_resources` webhook targets. The migration never adds recipients
to older events. `onReleaseImport` takes precedence over the legacy `onDownload`
alias, including settings readback; health notifications are opt-in through
`onHealthIssue`. Unsupported implementations are not treated as webhooks.

Before an event is inserted, a trigger saves an allow-listed `compat_context`
snapshot of the relevant wanted book, exact client/download, selected release and
ordered imported file set. UUID lookups retain index use and tolerate missing or
non-UUID legacy identifiers. Unknown import release identity never borrows another
release from the download. Provider download/info URLs, arbitrary file metadata
and target settings are excluded. Upgrade acquisition receipts now persist their
original current/cutoff scores so post-crash history repair retains those values.

The API installs the payload adapter before starting workers. It reconstructs the
existing book/author/download/release/import/bookFile shapes from the saved
snapshot, includes every imported file in `bookFiles`, and keeps the original
event ID/time. Source describes the persisted trigger, not the API request that
happened to finish recovery. Compatibility settings are loaded from the current
resource under revision/enable/trigger checks; its existing field aliases, method,
Authorization and Basic authentication remain supported. Bodies cannot be replayed
implicitly by Go's transport, including custom GET/PUT methods. Redirects and
secret-bearing network errors are handled the same way as native delivery.

API callback sends are removed. Manual API actions, scheduled work and domain
recovery now reach the same commit-time capture. Compatibility test/test-all remain
explicit synchronous requests. Delivery history identifies Readarr webhooks;
configuration remains under `/api/v1/notification`. Attempt and resolution state
uses the same session ownership, review controls and backup guarantees as native
connections. This qualifies generated fixtures, not live third-party consumers.


### Resolved notification history compaction

Migration 0051 introduces `notification_deliveries.resolved_at` and event
`archived_at`/`retention_summary`. HTTP acceptance and confirmed acceptance/cancel
start the resolution window; retry clears it. Automated connection-change
cancellations are unresolved. Legacy accepted rows inherit `updated_at`; legacy
cancellations are conservatively left unresolved.

Hourly `history-maintenance` selects at most 100 events older than 90 days whose
entire recipient set has been resolved for 90 days. Empty-recipient events use
creation time. Each event has its own transaction: lock the event with SKIP LOCKED,
try the existing delivery advisory keys with transaction locks, recheck resolution,
delete delivery/attempt/action detail, and replace both payload snapshots with a
bounded count summary. Holding the event row prevents new foreign-key references;
nonblocking delivery locks protect concurrent send/review/retry. A failure rolls
back that event; prior committed counts remain in the task report. The notification
pass has a 30-second deadline.

The event UUID, unique source key and timestamps survive forever. Compaction never
removes this replay barrier, recreates recipients, or changes health episode state.
Re-enqueuing an archived source key still does nothing. Detailed history is bounded
by the resolved retention window; unresolved records and compact event identities
are intentionally retained. Import/acquisition journals and domain history are
outside this policy. Restore must include compact records as well as active outbox
rows; deleting them manually can permit replay of the same source event.


### Worker availability and compatibility status

All 13 built-in workers register their definitions even when disabled or missing
configured dependencies. `Task.DisabledReason` and `UnavailableReason` separately
describe this instance's startup policy and prerequisites. Blank reasons preserve
the previous enabled/available default for registry callers. Blocked definitions
may omit a body; active definitions still require one. Startup skips their loops,
and manual/internal claim paths refuse them. Native manual requests return 409 for
disabled tasks and 503 for unavailable dependencies. History/review continue to use
the registered identity and shared database records.

`TaskStatus.enabled`/`available` and their reasons describe the responding instance.
Shared running/outcome/history/last-success evidence remains visible even when a
peer runs a locally disabled task. `nextRunAt` is omitted for local blocked tasks;
`lastFinishedAt` is only present when completion is recorded. A missing database
configuration produces an unavailable inventory; loss of configured persistence
returns 503 instead of invented status. Dependency availability means configured
prerequisites, not successful provider checks.

Readarr task names/IDs remain stable. The compatibility routes now map the registry
statuses and saved start/finish/duration/due times instead of deriving fake runs
from `now`. Native unknown times are omitted. Readarr's
[TaskResource](https://github.com/Readarr/Readarr/blob/develop/src/Readarr.Api.V1/System/Tasks/TaskResource.cs)
uses non-null DateTime/TimeSpan fields, so compatibility dates use the year-1 zero
value and duration uses zero when unknown. Four `librarry*Known` booleans explicitly
separate these placeholders from recorded start/finish/due/duration evidence.
Database failures propagate as 503.
ImportListSync uses its own actual interval and enable flag rather than feed-sync
settings. `LIBRARRY_IMPORT_LIST_SYNC_ENABLED` defaults true and is forwarded by
all deployment variants. Explicit native/compatibility sync commands remain
separate from scheduled execution. No schema migration is needed for this change.
