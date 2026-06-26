# Architecture

Librarry is split into a Go backend, a React frontend, and Postgres.

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
setup clients can round-trip Calibre configuration. When a manual or
completed-download import lands under a Calibre-enabled root, Librarry posts the
file bytes to the Calibre Content Server add-book endpoint and records the
returned Calibre book ID on the imported file. Librarry then calls the
Content Server set-fields endpoint with the imported title, author, and common
identifiers available on the file record. If the root folder has output formats
configured, Librarry fetches Calibre conversion book data and starts conversion
jobs for target formats that are not already present, then captures an immediate
status snapshot from Calibre's conversion status endpoint. Stored conversion
jobs can be refreshed later through the native refresh endpoint or
`RefreshCalibreConversions` command, and the API process can poll those jobs on
an interval through the `RefreshCalibreConversions` background task. When a
Calibre-backed file is physically deleted, Librarry calls the Content Server
delete-books endpoint before removing the local file record. Richer edition
metadata, embedded metadata writes, path refresh after Calibre renames, and
rollback for failed Calibre imports are still future work.

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
compatibility layer; tasks are derived from Librarry scheduler intervals for
feed sync, missing-book monitoring, author refresh, failed-download recovery,
and upgrade search.

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
