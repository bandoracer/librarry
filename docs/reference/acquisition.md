# Acquisition contracts

Implementation contracts for contributors. Operator instructions live in the [guides](../README.md#operate). These describe candidate source behavior, not complete compatibility or live qualification.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Acquisition](#acquisition)
- [Durable acquisition submission](#durable-acquisition-submission)
- [Scheduled evidence and fair checks](#scheduled-evidence-and-fair-checks)
- [Explicit upgrade selection](#explicit-upgrade-selection)
- [Acquisition integration health evidence](#acquisition-integration-health-evidence)

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
grabs the best approved replacement. Scheduled recovery defaults to auto-grab and failed-download removal. Set
`LIBRARRY_FAILED_DOWNLOAD_AUTO_GRAB=false` and
`LIBRARRY_FAILED_DOWNLOAD_REMOVE=false` for review-first behavior.

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
cutoff and the candidate improves by the configured minimum delta. Scheduled upgrade
search defaults to auto-grab; set `LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB=false`
for search-only behavior. Manual requests retain their explicit `autoGrab` scope.

Feed sync can be triggered manually through `POST /api/v1/wanted/feed-sync` and
can also run on an interval in the API process. Prowlarr does not provide a
single aggregate RSS endpoint, so Librarry lists RSS-enabled Prowlarr indexers,
pulls each Prowlarr-compatible Torznab/Newznab feed, stores seen releases, and
matches feed entries against wanted items with the same release evaluator used by
manual search. Scheduled feed sync defaults to auto-grab and sends only approved releases
to the matching download client. Set `LIBRARRY_FEED_SYNC_AUTO_GRAB=false` for
search-only behavior. Manual requests retain their explicit `autoGrab` scope.

## Durable acquisition submission

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
notifications are captured by the committing transaction through the
[durable outbox](operations.md#transactional-native-notification-delivery).
Compatibility webhooks now share that durable delivery path. No exactly-once
remote-delivery guarantee is made.
Broader worker/live-client and legacy qualification remain S10/S21 work.

## Scheduled evidence and fair checks

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

## Explicit upgrade selection

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

## Acquisition integration health evidence

`GET /api/v1/integrations/health`, native/compatible health and readiness reads,
and support export read process-local observations. They do not initiate remote
checks. `POST /api/v1/integrations/{name}/check` uses the normal auth boundary and
accepts the exact built-in names (Prowlarr, qBittorrent, Transmission, SABnzbd).
The five-minute Health Check task also checks configured integrations. Reading
System health no longer dispatches health notifications; the task owns checks
and notification transitions.

Each immutable configuration generation owns four observations. Concurrent checks
for the same integration coalesce; finished checks are reused for 15 seconds.
Each probe has a 15-second deadline. HTTP 429 honors Retry-After, capped at 24 hours
with a one-minute fallback. Cancellation from the caller does not create an outage
observation. A check finishing after configuration replacement returns 409 instead
of showing success against different credentials. Checks for different clients
can run concurrently. Observations disappear on restart/reconfiguration, retain
last success and last known numeric version after failure, and become stale after
ten minutes. Stale/never-checked states are warnings, not invented outages or
current readiness. Timestamps describe check evidence, not every acquisition call.

Probes validate the actual protocol shape: Prowlarr app name/version, qBittorrent
application version after the exact login acknowledgement when credentials are
configured, successful Transmission session-get with version/RPC version, and a
bounded SABnzbd queue read with version/status/slots. SABnzbd's version route is
public and cannot establish API-key access. qBittorrent injected clients now get
a cookie jar without mutating their caller's HTTP client; ordinary acquisition
requests retain the authenticated session too. Transmission probes keep their
session challenge local, independent of command state. When qBittorrent or
Transmission is used without credentials, a successful read establishes access
but does not invent an authenticated identity.

Health messages never echo request URLs, response bodies or raw network errors.
Redirects are refused and bodies are limited to 1 MiB. Version fields expose only
the numeric components; private build suffixes are omitted. This does not certify
remote mutations, indexer search coverage, download success or completed imports.

Contracts checked against primary documentation:
- [qBittorrent WebUI API](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-%28qBittorrent-5.0%29)
- [Transmission 4.0.6 RPC](https://github.com/transmission/transmission/blob/4.0.6/docs/rpc-spec.md)
- [SABnzbd API authentication and queue](https://sabnzbd.org/wiki/configuration/5.1/api)
