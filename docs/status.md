# Current Status

Last updated: 2026-09-16.

Librarry is an early alpha Readarr replacement. It is useful for validating the
metadata-first workflow and exercising acquisition integrations, but it is not
yet ready to replace a production Readarr instance unattended.

## September stabilization work

Bounded release review and qualification have resumed; feature expansion stays
frozen. The [pause-point assessment](reviews/2026-09-16-pause-point.md) preserves
the prior checkpoint. The [release-boundary review](reviews/2026-09-16-release-boundary.md)
rejects PR #3 as a standalone release because later fixes address reachable
recovery failures. The [short release checklist](release-checklist.md) now governs
delivery, while S01–S25 remains the longer-term backlog.

This candidate combines the existing PR #50 stack with publication isolation.
Automatic builds cannot advance `latest` or version aliases; explicit candidate
publication records unique tags and digests. No release, deployment or live
qualification is implied by these repository changes.

The [audit](reviews/2026-09-15-audit.md) and [execution plan](stabilization-plan.md)
separate current risks from the older milestone record below. The implementation
branch adds exact client payload selection, protected scan observations,
verified cleanup, relational file/book/download links, durable complete-file-set import
plans and recovery controls, scoped download mutations, authentication failure handling,
configuration snapshots, repeated Prowlarr categories, CI tests, and UI recovery.
See the [implementation ledger](reviews/2026-09-15-implementation.md) for evidence.

These changes are **unreleased work**: [safety/recovery PR #3](https://github.com/bandoracer/librarry/pull/3),
[multipart PR #4](https://github.com/bandoracer/librarry/pull/4),
[staging recovery PR #5](https://github.com/bandoracer/librarry/pull/5),
[manual recovery PR #6](https://github.com/bandoracer/librarry/pull/6),
[acquisition recovery PR #7](https://github.com/bandoracer/librarry/pull/7),
[persisted scan PR #8](https://github.com/bandoracer/librarry/pull/8),
[repair preview PR #9](https://github.com/bandoracer/librarry/pull/9),
[move reconciliation PR #10](https://github.com/bandoracer/librarry/pull/10),
[reviewed replacement PR #11](https://github.com/bandoracer/librarry/pull/11),
[acquisition bookkeeping PR #12](https://github.com/bandoracer/librarry/pull/12),
[provider health PR #13](https://github.com/bandoracer/librarry/pull/13),
[exact fallback PR #14](https://github.com/bandoracer/librarry/pull/14),
[metadata cache PR #15](https://github.com/bandoracer/librarry/pull/15),
[author bibliography PR #16](https://github.com/bandoracer/librarry/pull/16),
[provider request budget PR #17](https://github.com/bandoracer/librarry/pull/17),
[complete list PR #18](https://github.com/bandoracer/librarry/pull/18),
[Hardcover edition PR #19](https://github.com/bandoracer/librarry/pull/19),
[author policy PR #20](https://github.com/bandoracer/librarry/pull/20),
[author destination PR #21](https://github.com/bandoracer/librarry/pull/21),
[direct author detail PR #22](https://github.com/bandoracer/librarry/pull/22),
[book presence PR #23](https://github.com/bandoracer/librarry/pull/23),
[worker fairness PR #24](https://github.com/bandoracer/librarry/pull/24),
[explicit upgrade selection PR #25](https://github.com/bandoracer/librarry/pull/25),
[collection projection PR #26](https://github.com/bandoracer/librarry/pull/26),
[book paging PR #27](https://github.com/bandoracer/librarry/pull/27),
[author paging PR #28](https://github.com/bandoracer/librarry/pull/28),
[metadata review PR #29](https://github.com/bandoracer/librarry/pull/29),
[author review PR #30](https://github.com/bandoracer/librarry/pull/30),
[file paging PR #31](https://github.com/bandoracer/librarry/pull/31),
[durable rename PR #32](https://github.com/bandoracer/librarry/pull/32),
[book folder PR #33](https://github.com/bandoracer/librarry/pull/33),
[Calibre client PR #34](https://github.com/bandoracer/librarry/pull/34),
[Calibre recovery PR #35](https://github.com/bandoracer/librarry/pull/35),
[worker coordination PR #36](https://github.com/bandoracer/librarry/pull/36),
[native notification PR #37](https://github.com/bandoracer/librarry/pull/37),
[compatibility notification PR #38](https://github.com/bandoracer/librarry/pull/38),
[worker diagnostics PR #39](https://github.com/bandoracer/librarry/pull/39),
[notification retention PR #40](https://github.com/bandoracer/librarry/pull/40),
[worker availability PR #41](https://github.com/bandoracer/librarry/pull/41),
[support diagnostics PR #42](https://github.com/bandoracer/librarry/pull/42),
[integration health PR #43](https://github.com/bandoracer/librarry/pull/43),
[import recovery PR #44](https://github.com/bandoracer/librarry/pull/44),
[dashboard counts PR #45](https://github.com/bandoracer/librarry/pull/45),
[import reviews PR #46](https://github.com/bandoracer/librarry/pull/46),
[book choices PR #47](https://github.com/bandoracer/librarry/pull/47),
[removed book recovery PR #48](https://github.com/bandoracer/librarry/pull/48),
[search identity PR #49](https://github.com/bandoracer/librarry/pull/49), and
the `codex/compat-book-collections` continuation. They do not certify the current homelab. The September audit found the LAN portal reachable and reporting 0.4.0;
the Cosmos hostname returned 502. Earlier successful Cosmos checks below are
historical. No September production rollout or unattended soak is complete.


Compatibility book reads now reach all active records. Missing/cutoff pages use
the native evidence rules, complete counts and bounded SQL pages. Selected book
monitor/edit/delete validates every identity and commits the batch atomically;
missing, inactive, ambiguous or concurrently changed targets fail explicitly.
Partial chapter sets remain incomplete, and imported status cannot hide lost
media. Numeric IDs retain their legacy hash; collisions are rejected on lookup,
but collision-free resource mapping and real Readarr migration are still open.
These changes do not qualify other compatibility resources or unattended use.

Worker history now preserves unreviewed failures beyond the 100-success retention
window. Task history has pagination, reversible review, counts, available operation
IDs and measured duration. Per-item errors, cleanup failures and backup-pruning
failures produce a degraded run and do not advance the separate last-success
record. Reviewed failures become eligible after 90 days, in bounded batches on
the task's next completion. Unknown interrupted finish times stay unknown.
Historical successes cannot recover missing older per-item error counts. This
is local qualification. Hourly History Maintenance now compacts notification
details only after every recipient has been resolved for 90 days, preserving
unreviewed stopped deliveries and permanent compact source-event records against
replay. The same task prunes old reviewed failures for disabled workers. Support
diagnostics are now available as a redacted, on-demand export; broader live
readiness/freshness qualification remains open under S23.
All built-in workers now stay visible when disabled or unavailable, with reasons
and retained history. Local configuration is separated from shared peer activity;
blocked workers have no local next run and refuse manual task execution. Readarr
task times come from persisted evidence rather than the current clock. Import-list
scheduling has its own enable flag, default true, across all installers. Small
screens keep each task name, reason, timing and controls in one readable row.

System now downloads a versioned support report containing build identity,
Postgres connectivity/version, startup schema, selected effective settings,
anonymous root observations, recorded provider evidence and worker status.
Credentials, URLs, paths, book metadata and free-text errors are omitted by an
allowlist. The report makes no provider/client requests and preserves unknown
versions, digest and timestamps. `/healthz` is liveness; `/readyz` checks database
connectivity through both API and nginx, returning 503 without usable persistence.
Neither probe claims media, provider or end-to-end workflow health. Directory
checks are time-bounded and do not certify mount identity or write permission.
No September live readiness or NAS qualification is implied.

Acquisition integration health now separates configured state from observed checks.
Status/readiness/support reads do not contact Prowlarr or download clients; explicit
System checks and the five-minute Health Check worker do. Checks retain actual
attempt/success/version times, coalesce for 15 seconds, honor bounded retry delays,
and become stale after ten minutes. Restart/reconfiguration clears process-local
evidence; old in-flight results cannot attach to new settings. Malformed responses
cannot count as success. SABnzbd checks a bounded authenticated queue read instead
of its public version route. qBittorrent now retains login cookies when the service
injects its HTTP client and accepts only the documented login acknowledgement.
Support exports include recorded numeric client versions without private suffixes,
URLs or response bodies. These are contract/container fixtures, not new live-client
qualification or proof that a remote mutation/import will succeed.

Dashboard attention now uses full metadata/author-review counts and an authenticated
count-only recovery endpoint for pending import reviews, unfinished native imports,
Calibre handoffs and unresolved legacy links. Recovery links open the unfinished
filter. Missing counts, failed refreshes and incomplete client evidence cannot
produce “All caught up”. Acquisition totals inspect all active tracked books;
the separate action preview remains bounded and explicitly labelled. Import totals
are historical acquisition records, not fresh file-presence claims. This does not
complete paging for the older import-review list or other legacy readers.

Automatic native import now accepts single books and identifiable audiobook
chapter sets from complete client inventories. All required chapters and relevant
sidecars enter one durable manifest, preserving disc directories and natural
chapter order. Conflicting metadata, incomplete/unselected files and multi-book
packs enter review. Review shows all files and exclusions, supports assigning
files to different wanted books, and requires a current destination preview.
Skip/Reject retain sources and prevent worker import until the review is reopened.

Automatic removal requires a verified import receipt, fresh client inventory,
current matching source/destination hashes for every required file, a destination
outside download storage, and positive seed-goal evidence. Retaining any supported
file in downloads during explicit mapping blocks automatic cleanup. Old `imported`
rows alone are not eligible. Completed import rejects move mode; use
hardlinkOrCopy, hardlink, or copy.

Native completed imports save immutable manifests and use expiring worker leases.
A failed database commit can resume without duplicating published chapters;
uncommitted destinations remain hidden from scans. New native staging files are
journaled and reclaimed on retry; unrecorded older stages remain untouched.
Imports now pages native operations, Calibre handoffs and unresolved legacy links
independently, with exact matching totals and an unfinished filter. Migration 0052
adds indexes for creation-time/identity ordering; retries do not reorder pages.
Native recovery shows recorded activity, verified-file count and transfer or cleanup lease
observations. Expired leases are not diagnosed as dead workers; lease purpose
follows the saved operation phase. These are database observations,
not fresh byte verification, throughput or automatic stall detection. Native
manual imports now use the same ledger, including configured same-basename
sidecars, moves after commit and recoverable replacement of an existing file.
Committed manual imports with unfinished cleanup remain visible and retryable.
A configured recycle bin failure retains the previous file and reports an error.

Native import review browsing now reaches older pending and resolved records with
complete counts, search, format/type filters and cursor pages. Resolved includes
actual imported/skipped/rejected decisions. Payload reviews share the collection;
file bulk actions select only the current page and clear selection on navigation.
Failed reads show an error rather than an empty queue. Migration 0053 adds paging
indexes. Manual and payload book-choice selectors now search/page all active saved
books through a database-only endpoint. Selected identities remain visible outside
the current search/page; owner-edited labels win. Migration 0054 adds a supporting
creation/identity index. Removed/ignored records are excluded from new choices.

Search now checks the complete saved collection using typed provider identities
and format rather than the first 200 wanted rows or title similarity. Removed,
ignored and ambiguous matches remain explicit. Lookup failures disable Add until
retry succeeds. Search adds request transactional preservation: existing tracking
returns a conflict without changing settings or restoring a removed record. This
local check does not claim current file presence, and legacy callers that omit the
preservation flag retain their existing behavior.

Removed and ignored books now have a complete searchable Library view with
creation-time pages and exact counts. Explicit restore preserves saved file links,
history, metadata overrides, roots, profiles and tags. Monitoring defaults off;
a changed record requires a fresh review. Restore and its history event commit
atomically. Removed book deep links explain their inactive state and offer Restore
instead of an implicit monitoring toggle. This restores tracking, not deleted files.

Completed-download review now supports replacing matching destinations. The
preview binds both new and old content; old bytes are journaled before publication
and kept until the complete replacement commits. Replacement-backup cleanup has
its own retryable state and cannot authorize download-source deletion. Existing
file IDs, book associations and manual names/notes survive. Keep both remains the
default. Extra old chapters or unrelated files outside the new manifest require
review; whole-book retirement across different chapter layouts and live upgrades remain
open. New single-book Calibre handoffs now have durable recovery; multipart
Calibre roots still require review.

Multipart qualification uses generated local fixtures, adapter contract tests and
disposable API/web/Postgres containers. It is not a live client or homelab
certification. SABnzbd import trusts only a successful completed history record's
final output directory, never a queue/archive inventory or a shared-folder search.

With Postgres persistence, manual and scheduled grabs now reserve the wanted
book and exact release before sending an add. Duplicate requests replay the saved
receipt without resetting current download progress. Interrupted/ambiguous sends
stay visible in Activity; a lease expiry, empty queue or client outage cannot
silently authorize another add. Check client reconciles exact tags/infohashes on
the original configured client. SABnzbd acknowledgement loss needs an explicit
client-ID selection. Attach/release decisions require operator confirmation.
This is contract-fixture and packaged restart qualification, not live-client proof.
New acquisition receipts retain the selected release and score. Accepted downloads
whose local bookkeeping failed remain visible and retryable, with one grab-history
entry after recovery. Native import commits installed-release state and import
history with the complete file set. Failed upgrades preserve imported status;
blocklisting uses the failed download's identity. API/worker notifications skip
replayed results. Native and Readarr-compatible webhook notifications now use a transactional
outbox for newly committed events. Legacy history/current-release repair and
broader scheduled-worker qualification remain open under S10/S21.

Library scans now persist their path queue and progress, resume through the
scheduler after restart, and support cancellation/retry in Imports. The old file
limit is a batch size, not a total scan cap. A 10,001-file fixture completes and
rescans without duplicate records. Files expose separate local presence evidence;
missing observations apply only after all roots complete successfully and are
rechecked before commit. Unavailable/changed roots and lost nested-device evidence
retain previous presence. Legacy records are not declared missing until a scan
has positively observed them. Imports now offers a paginated, read-only repair preview for broken legacy
associations, duplicate recorded content, possible moves, unverified audiobook
completeness and discrepancies against committed manifests. Every finding explains
the evidence and a recommended action; this preview does not verify current bytes
or apply repairs. Completed scans now reattach a missing file to one unambiguous,
untouched and unassigned scan discovery with the same SHA-256, size and format.
The current scan's hash is bound to the same inode, size and nanosecond mtime;
root/device checks and final ownership fences protect completion. The original
ID, manual metadata, associations and historical import manifest survive.
Ambiguous copies, changed/assigned discoveries and Calibre ownership remain for
review. Imports shows a paginated path-change history. Unified book/presence
projections remain S14 work; live NAS/mount qualification remains outstanding.

Remote provider health now distinguishes configured credentials from actual request
evidence. System's explicit checks verify Hardcover authentication with a read-only
query and Open Library/Google access with an ISBN lookup. Polling status does not
spend provider quota or refresh request timestamps. Errors retain the prior success
time while showing rejected credentials, denied access, rate limiting or outage;
HTTP 429 applies retry backoff. Credential-bearing URLs and provider error bodies
are not returned. These behaviors are qualified with contract fixtures, including
concurrency and malformed/empty-response cases. Google fallback now runs only when
primary providers have no suitable exact ISBN/full-title match. Returned matches
are checked for ISBN checksum/equivalence or literal title/subtitle and known
language/format conflicts. Author and series queries never use Google; unknown
format stays unknown. Bounded process-local provider caches reuse successful
searches for five minutes and empty results for 30 seconds. Cache reads do not
invent fresh request evidence; credential, malformed-response and outage failures
invalidate affected entries, while rate limiting preserves unexpired results.
Author monitoring now verifies a stable provider ID and traverses a complete
bibliography before applying policy. Failed pages or unproven identities retain
existing data and leave the author unsynced. Existing tracked editions, manual
corrections and removed entries survive add-only monitoring. Same-name identities stay separate;
unknown/non-writing Hardcover credits enter review. Original work dates remain
separate from editions. Open Library returned 418 works across six paced read-only
requests in September qualification. Hardcover traversal is fixture-qualified only.
Metadata and Hardcover lists now share a per-host request budget with one-second
spacing and header-driven daily/burst backoff. Unsent requests do not invent health
evidence; valid cached data survives rate limiting. The paced Open Library probe
was rerun through this production transport.

Hardcover title searches now enrich work identities with separate default ebook
and audiobook editions. Exact ISBN queries look up actual editions directly.
Responses retain stable contributor roles, edition ISBN/language/publisher/date,
cover and duration evidence; original work dates stay separate. Unsupported or
missing edition formats remain unknown. Conflicting formats/languages/edition
identities no longer collapse during merging, and concrete editions no longer
share the generic work/format placeholder in persistence. This is fixture and
database qualification. Real tokens, broader edition discovery, series and
persistent raw provider snapshots remain outstanding under S12.
Hardcover list sync now verifies a visible list and traverses all pages before
adding entries. Failed/inconsistent traversal leaves existing tracking and the
success timestamp unchanged. Existing tracked books survive add-only list sync;
new root/monitor settings commit with creation. All stored exclusions are honored.
This list behavior is contract/database-qualified; real-token validation remains
pending. Search-on-add remains a best-effort search with logged errors.

Author policy qualification now covers all seven modes and repeat refreshes.
Latest selection keeps the latest published book when a future title exists;
missing/overlapping dates require review instead of provider-order guesses.
Original work dates/years precede edition dates, and partial dates cannot invent
cutoff precision. Ignored reviews follow stable work identity within the author
and format; saved book exclusions apply before policy selection. Language filters
use canonical equality instead of prefixes. Settings and exclusions are re-read
after provider IO, and changed subscriptions cannot receive a stale success
timestamp. This does not yet serialize every concurrent owner edit with candidate
insertion. The complete verified-file projection and the broader curated matching
corpus remain S13/S14 work.

Native author subscriptions now persist a format-compatible destination root.
New books inherit that root, quality profile and tags; existing books retain their
settings. Pending reviews retain the destination from their latest evaluation.
Add New passes the chosen defaults, runs a targeted refresh after saving, and
reports partial save/refresh failures. Refresh Author uses the saved subscription
without resetting it. Author settings can change or clear the root for future
additions. Migration 0041 adds the references without guessing destinations for
legacy subscriptions/reviews. Readarr root mapping remains S16 work; this does
not qualify live storage paths or provider access.

Author details now use direct server lookup and bounded title/ID pagination,
including imported and unmonitored books beyond old collection caps. Same-name
identities are kept separate; old name links offer explicit choices. Manual
author-name overrides take precedence over stored associations. New wanted
writes retain coauthors and contributor roles with concurrent alias locking.
Existing omitted coauthors, incorrect historical roles and conflated legacy
identities require repair rather than a guessed backfill. The detail page
distinguishes outage from absence and scopes bulk search to its visible page.
Native book/author/list status now reads page-scoped file evidence. Ebooks need a
recorded present media file; audiobooks need one complete committed per-book media
manifest whose linked file identities, sizes and hashes still match. Lost chapters
are incomplete; legacy audio without a manifest remains unknown. A successful
alternate complete import can satisfy presence. Sidecars are outside playback
presence; their full verification remains required for cleanup. Renamed files keep
presence through identity/content, without rewriting historical manifests.

Download state uses successful live responses only, with explicit partial/outage
signals and a five-second request budget. Already-imported download sources do
not hide later chapter loss. File/database uncertainty becomes unknown. Presence
is the last recorded scan/import observation, not a filesystem probe on page load;
unavailable mounts still retain previous observations. Quality profiles load once
per page, and older books are no longer annotated through the global cutoff cap.
The cutoff list excludes missing/unverified media but retains its existing cap.

Global collection counts/caps, author-file-policy integration of this evidence,
Readarr parity, legacy manifest repair and broader S15 latency qualification remain
open. General download queues retain their separate historical fallback behavior.
No new migration, production deployment or live NAS qualification is included.

Scheduled book monitoring and upgrade checks now use the native file/client
evidence. Known missing/incomplete imported books can enter recovery search;
unknown/outage evidence is skipped with a reason. Upgrades require complete
present media below cutoff. Automatic monitor/feed/upgrade/failed-replacement
grabs recheck ownership/settings/evidence after provider IO; the acquisition
reservation locks the wanted row and rejects unmonitored automatic requests.
Explicit manual grabs remain available.

Migration 0042 separates check/attempt timestamps from successful search/sync
history. Skipped or failed batches advance fairly, with stable ID tie-breakers
and a retry backoff capped at 15 minutes. Successful searches/syncs still obey
their configured intervals; Force permits an immediate operator retry. Checks
do not change owner revisions or invent successful search timestamps. This does
not introduce renewable worker leases; overlapping workers can still perform
redundant provider reads, while acquisition reservations prevent duplicate adds.

Feed matching now traverses eligible monitored books in 200-row ID batches rather
than only the newest 200 wanted rows, including imported books with known file
loss. Response details cap at 1,000 with an explicit truncation flag/message;
counts cover all evaluated matches. Wanted exposes actual Incomplete/Unknown
routes, and its batch buttons state their 50-book scope. Global list counts,
server-defined all-matching bulk jobs and durable per-item skip history remain
unfinished. Author file policies still use the older association lookup.

Feed release observations persist decisions without updating the full indexer
search timestamp, so repeated RSS matches cannot postpone due searches.

## Historical verified milestones

- The Go API, React web UI, Postgres migrations, and Docker/TrueNAS custom-app
  shape build and run together.
- Public distribution files are present for generic Docker Compose, TrueNAS
  Custom Apps, and Unraid Docker Compose Manager, including example env files,
  path/permission guidance, backup notes, and upgrade instructions.
- The live TrueNAS deployment was verified at `http://192.168.1.221:30200/`.
- The Cosmos route `https://librarry.borchetta.xyz/` was verified after the
  route was added, and the app rendered through the proxy.
- The Cosmos route was re-tested on 2026-06-29 after proper React routing was
  added. Direct loads for `/dashboard`, `/library`, `/search`, `/wanted`,
  `/downloads`, `/imports`, `/providers`, and `/settings` all returned the live
  `index-BYM1I52w.js` bundle.
- Open Library metadata search works and returns normalized book results with
  provider provenance.
- Metadata search, manual release search, and wanted-item release search now
  default to English through the library settings preference.
- Local OPF/embedded metadata support is wired for import evidence.
- Metadata result merging deduplicates equivalent provider candidates by ISBN
  or compatible title/author evidence while retaining provider aliases.
- Manual override persistence exists for wanted-item bibliographic corrections.
- Prowlarr integration is configured in the live deployment and returned real
  ebook release results through Librarry.
- Prowlarr release grabs now resolve the Prowlarr download URL inside Librarry
  before handing off to qBittorrent or Transmission. Torrent files are uploaded
  to the client, magnet redirects are handed off as magnets, and qBittorrent
  infohash IDs are normalized to lowercase so live queue rows reconcile with
  stored rows. This was verified against the live TrueNAS deployment on
  2026-07-01 with a public-domain Moby Dick EPUB candidate:
  `740b73b9bd31325f178b216cc43b4e735a6dca47` appeared in qBittorrent and was
  then removed through Librarry cleanup.
- A deployed browser E2E through `https://librarry.borchetta.xyz/` successfully
  searched ISBN `9781250313195`, created a temporary wanted item for Gideon the
  Ninth, searched Prowlarr releases, persisted four approved release decisions,
  rendered the wanted queue without browser errors or 4xx/5xx API calls, and
  removed the temporary wanted item afterward.
- qBittorrent integration is configured in the live deployment. A paused legal
  Ubuntu torrent smoke test was added, listed through Librarry with
  `librarry-smoke`, deleted with files, and verified absent afterward.
- A deployed browser E2E on 2026-07-01 searched for Moby-Dick, selected a
  normalized Open Library result, searched Prowlarr releases, and successfully
  queued a qBittorrent torrent through the Librarry UI. A public-domain
  Internet Archive Pride and Prejudice torrent was then added through the
  deployed manual-add UI, refreshed to the real qBittorrent hash, started from
  the Librarry queue, and observed through the API as `downloading` at roughly
  98% before qBittorrent reported `stalledDL` from peer availability.
- A deterministic CC0 EPUB fixture in `docs/fixtures/e2e/` was added through the
  LAN-deployed manual-add UI, started from the Librarry queue, completed in
  qBittorrent as hash `25fa3db023fd85445bfc49adca0387991839d562`, and rendered
  in Librarry with `progress: 1` and `importStatus: ready`. The completed file
  was verified on the `media-stack` filesystem at
  `/mnt/HDD_pool/vault/media-stack/torrents/books/librarry-public-domain-e2e-book.epub`
  with SHA-256
  `3ae24570da25451931c1b2cb62c4cd205eb387801a8dd5099045c2f15ec07eb2`.
- Queue actions no longer merge stored placeholder download rows back into the
  UI after an ID-specific live client lookup returns no matching hash. The UI
  also disables Start/Stop/details and qBittorrent manager actions for
  persisted `pending` placeholder rows until the external client exposes a real
  download ID.
- The public Cosmos hostname was re-tested after the deployment:
  `/api/v1/downloads?tag=librarry` returned live qBittorrent rows from both curl
  and Chromium, and one valid browser POST to `/api/v1/grabs` reached the API
  and returned 200. A later Chromium POST to `/api/v1/grabs` intermittently
  reproduced `net::ERR_ECH_FALLBACK_CERTIFICATE_INVALID`; the LAN URL remains
  the reliable deployed E2E path until the Cosmos/Cloudflare route is fixed.
- Removed qBittorrent downloads are hidden from active download listings.
- The UI tolerates nullable list payloads from the API, including empty
  `files`, `authors`, `downloads`, `releases`, `profiles`, `reviews`, and
  `events` arrays.
- On 2026-07-01 the web UI was migrated from a single-file React app
  (7,445-line `App.tsx`) to a modular feature architecture with TanStack Query
  and lazy-loaded pages, and redesigned toward classic arr conventions (dark
  sidebar always reachable: full rail / icon rail / mobile drawer; page
  toolbars; queue-first Activity page; confirm-before-delete). Route paths are
  unchanged (`/dashboard`, `/library`, `/search`, `/wanted`, `/downloads`,
  `/imports`, `/providers`, `/settings`); nav labels changed (Search→Add New,
  Queue→Activity, Providers→System) and the default route is now `/library`.
  Verified in demo mode at 375/768/1280 px with zero console errors; `tsc -b`
  and `vite build` pass. See docs/frontend.md and docs/ui-backlog.md. The
  redesigned UI has not yet been re-verified against the live TrueNAS
  deployment.
- On 2026-07-01 the redesigned UI was verified read-only against the live
  TrueNAS API (dev server proxied to `http://192.168.1.221:30200`): library,
  wanted, activity queue, system checklist, and settings all rendered live
  data with zero console errors, and saved secrets stayed masked. Follow-up
  changes from that pass: the downloads API now annotates `wanted:<id>`-tagged
  rows with the wanted item's id/title/author and the Activity queue links
  them; the queue and dashboard failed-download triage default to the
  Librarry-tagged scope; and Settings shows a standing warning when saves are
  runtime-only because no database is configured. The redesigned UI was then
  deployed to the live TrueNAS app from local images
  `librarry-api:local` (`6d4f29274da4`) and `librarry-web:local`
  (`8e7ca39b82c6`) and verified through both
  `http://192.168.1.221:30200/library` and
  `https://librarry.borchetta.xyz/library`.
- A real-user E2E was run through the deployed redesigned UI on 2026-07-01
  (bundle `index-Cuji0PMa.js`, API build 2026-07-01T22:30:20Z): searched
  "Moby Dick" on Add New (9 normalized Open Library results), added it through
  the review-confirmation modal (medium-confidence gate fired with reasons),
  deep-linked to the wanted item via `/wanted?item=<id>`, ran a Prowlarr
  release search (18 found · 6 approved · 12 rejected, rejection reasons
  rendered), grabbed the top approved public-domain EPUB (score 88.5) paused,
  started it from the Activity queue, and watched it complete: progress 1,
  `stalledUP`, `importStatus: ready` at `/data/torrents/books`. The wanted
  item flipped to `grabbed` with release score 88.5, and the queue row showed
  the new "Moby Dick · Herman Melville" wanted-item link from the downloads
  annotation. Zero browser console errors. The test book and its wanted item
  were intentionally left in place as proof.
- Completed download handling now runs automatically (arr parity): a
  background worker imports completed librarry-tagged downloads every minute
  by default (`LIBRARRY_COMPLETED_IMPORT_*`, mode `hardlinkOrCopy`), matching
  wanted-linked downloads directly and routing unmatched files to the import
  review queue with store-level dedupe by source path. Covered by unit tests;
  needs live verification after the next API image deploy — the Moby Dick
  E2E download sitting at `importStatus: ready` should import on its own
  within ~a minute of startup.

- v0.2.0 (2026-07-01) implemented the full Readarr parity plan
  (docs/parity-plan.md, all six milestones; delay profiles skipped by owner
  decision): blocklist with evaluation rejection and auto-blocklisting,
  remove-after-seeding, cutoff-unmet view, search-on-add, seven monitor
  modes, author/book detail pages, poster/overview library views, mass
  editor, multiple root folders, remote path mappings, recycle bin,
  series/year naming tokens, rename UI, task scheduler with run-now, native
  notifications (webhook/ntfy/Discord/Telegram) dispatched from workers,
  continuous health checks, disk space, calendar + iCal feed, Hardcover
  import lists with exclusions, author add-filters, native tags,
  none/basic/forms authentication, and scheduled pg_dump backups. Auto-grab
  defaults flipped to arr parity. Verified locally against a disposable
  Postgres 16 (all 26 migrations, endpoint smoke across the new surface,
  forms-auth login through the real UI, task run-now with tracked outcomes);
  `go test ./...`, `go vet`, and the web build pass. The homelab was deployed
  to v0.2.0 from local images `librarry-api:local` (`22e241134431`) and
  `librarry-web:local` (`18ba4ec169e7`) on 2026-07-01. LAN health, the
  `/calendar` route, qBittorrent download listing, database migrations,
  completed-download import, and a manual pg_dump backup were verified live.
  Not yet verified live: Hardcover list sync against the real GraphQL schema
  (no token locally), ICS consumption by a calendar app, and backup restore.
- The first v0.2.0 live deploy exposed missing deployment directories. The
  `media-stack` ebook/audiobook roots were created for the API user, allowing
  Moby Dick to import to
  `/mnt/HDD_pool/vault/media-stack/media/books/ebooks/Herman Melville/Moby Dick/Moby Dick.epub`
  with SHA-256
  `db359a71d3f57af793cf1906a56da1309fce32e908a846229b1146cd2c32fde5`.
  The live custom app also now mounts
  `/mnt/HDD_pool/vault/app-config/librarry/config:/config`; a manual backup
  created `librarry-20260702-013251.dump`.
- `https://librarry.borchetta.xyz/healthz` and direct HTML requests return the
  v0.2.0 deployment through Cosmos, but Chromium/Playwright still reproduced
  `ERR_ECH_FALLBACK_CERTIFICATE_INVALID` on
  `https://librarry.borchetta.xyz/calendar`. The LAN URL remains the reliable
  browser verification path until the Cosmos/Cloudflare certificate path is
  fixed.

- The deployed v0.2.0 instance passed a live E2E of the parity surface on
  2026-07-01/02: completed-download handling ran autonomously on deploy —
  Moby Dick auto-imported to
  `/data/media/books/ebooks/Herman Melville/Moby Dick/Moby Dick.epub`
  (wanted status `imported`) and its seeded torrent was auto-removed from
  qBittorrent; the unmatched CC0 fixture correctly queued an import review;
  failed-download recovery auto-grabbed a paused replacement for the old
  failed download (auto-grab defaults working live); search-on-add ran a
  release search during a real add (11 found · 3 approved · 8 rejected);
  queue-remove with "Blocklist release" produced a blocklist entry (infohash,
  reason, source) in the Blocklist tab; all 8 health checks passed; the task
  registry showed real last/next runs; disk space read the NAS filesystem;
  and Backup Now produced a real pg_dump in the deployed container alongside
  the scheduled dumps. Test artifacts were cleaned up (temporary wanted item
  deleted before the monitor tick, blocklist cleared, fixture removed with
  data). Wide-breakpoint layout fixes shipped as v0.2.1.
- Known deployment bug found during the E2E, fixed in-repo: nginx did not
  proxy `/feed/`, so the deployed iCal URL returned the SPA shell; the
  `/feed/` proxy block is now in deploy/nginx.conf and ships with the next
  web image. The live calendar is empty because existing wanted rows have
  year-only publish dates (full-date items will populate it).

- v0.3.0 adopts Readarr's derived book-state semantics: presence is computed
  at the API boundary (`derivedState`: missing / downloading / downloaded /
  cutoffUnmet / unmonitored) from monitored + tracked file + profile cutoff +
  live librarry downloads, instead of the stored lifecycle status. The UI
  shows only derived states ("Grabbed"/"Present" labels are gone), and the
  wanted monitor now treats a grabbed book with no live download and no file
  as missing again — a vanished download can no longer strand a book in
  "grabbed" (the client-unreachable case conservatively skips grabbed items
  to avoid double grabs). The stored status remains in payloads for
  Readarr-compat clients. Live proof pending next deploy: A Brief History of
  Time (stranded grabbed, score-100 stored release) should re-enter the
  monitor and re-grab automatically.

- v0.4.0 closes the remaining Readarr inconsistencies (all ten from the
  2026-07-02 audit): quality profiles are now ordered quality ladders with a
  cutoff quality (plus separate release profiles and a quality-definitions
  size table; release scores became rank×1000+preferred composites and the UI
  shows parsed quality badges); named metadata profiles selectable per author
  (per-author filters remain as overrides); Wanted is a pure Missing /
  Cutoff Unmet / Review gap view with author management moved to Library →
  Authors and the acquisition strip to the Dashboard; root folders support
  Calibre-managed mode (per-root content-server settings, import handoff,
  rename skip); Settings splits into Readarr-shaped tabs (Quality, Indexers,
  Download Clients; /settings/connections redirects); System gains
  Status/Tasks/Backups sub-tabs; Library toolbar says Refresh Monitored /
  Update All / RSS Sync ("Feed Sync" renamed everywhere); a Rename Books
  toggle (default on — Readarr defaults off — set LIBRARRY_RENAME_BOOKS);
  the add dialog gains root-folder and tags pickers; the blocklist shows the
  linked book. Migrations 0027–0029 verified against scratch Postgres 16 on
  fresh and upgrade paths; full test suite and builds pass; all restructured
  pages verified at 1920px with zero console errors. Legacy links
  (?filter=…, ?item=…, ?tab=authors, /settings/connections) redirect.

- The deployed v0.4.0 instance was verified on 2026-07-02: bundle and API at
  0.4.0 on LAN and Cosmos, the /feed/ proxy serves text/calendar, all three
  new endpoint families answer, quality profiles migrated to ladders with the
  "Migrated terms" release profile created from legacy terms, and — the
  v0.3.0 proof — A Brief History of Time un-stranded itself: automatically
  re-searched, re-grabbed, downloaded, and imported with zero clicks
  (history events 08:07–08:08Z). Two issues found and fixed in v0.4.1:
  feed-sync/recovery/upgrade worker grabs were hardcoded paused (only the
  monitor grabbed active) — all automated grabs now start immediately per
  arr parity; and legacy-scale release scores (pre-0027, e.g. Moby Dick at
  88.5) were misclassified as cutoff-unmet and could trigger false upgrade
  grabs — legacy-scale items now count as at-cutoff until a re-search
  rescales them.

## Available But Still Needs Real-World Proving

- Readarr migration preview/import has broad API and UI coverage, but needs a
  full dry run against a real Readarr instance and a row-by-row comparison.
- Library scanning and import can process ebook/audiobook roots, but the real
  `media-stack` book roots still need a controlled full scan and import review.
- The end-to-end book loop has proof through metadata search, release search,
  manual torrent add, qBittorrent grab/start, completed download, and filesystem
  verification. It still needs more proof on real data for completed download ->
  import -> tracked file -> missing state clears.
- Author monitoring exists, including all/future/none policies, but needs longer
  observation with real monitored authors before it should auto-grab.
- Calibre Content Server handoff exists for add-book, basic metadata fields,
  conversion starts, status refresh, and deletes, but richer edition sync,
  embedded metadata writes, path refresh after Calibre renames, and rollback are
  still future work.
- Readarr-compatible APIs cover many common routes, but this is compatibility
  surface area, not a promise of complete OpenAPI parity or identical side
  effects.

## Known Gaps

- Hardcover is the intended primary rich metadata provider, but the live
  deployment still needs `LIBRARRY_HARDCOVER_TOKEN`.
- Google Books exact-match fallback is implemented, but the live deployment
  still needs `LIBRARRY_GOOGLE_BOOKS_API_KEY`.
- Goodreads, Amazon, and Audible scraping are intentionally not in core.
- The GHCR image workflow is in-repo, but the first tagged release still needs
  to be cut and verified after the workflow runs from `main`.
- The default local, TrueNAS, and Unraid examples are operator-facing and should
  be put behind `LIBRARRY_API_KEY`, Cosmos auth, Cloudflare Access, or another
  trusted boundary before WAN exposure.
- Public Docker/NAS packaging is ready for trial installs, but the first
  versioned GHCR release still needs to be cut and smoke tested after the image
  workflow publishes from `main`.
- Librarry intentionally does not replace qBittorrent, Transmission, or SABnzbd
  as full download-client administration UIs.

## Current Live Deployment Notes

The local homelab deployment, when present, uses:

- TrueNAS custom app name: `librarry`
- Web portal: `http://192.168.1.221:30200/`
- Cosmos hostname: `https://librarry.borchetta.xyz/`
- API and web images: currently deployed from local images; public templates use
  `ghcr.io/bandoracer/librarry-api:latest` and
  `ghcr.io/bandoracer/librarry-web:latest`
- Postgres data: `/mnt/HDD_pool/vault/app-config/librarry/postgres`
- App config/backups: `/mnt/HDD_pool/vault/app-config/librarry/config:/config`
- Media mount: `/mnt/HDD_pool/vault/media-stack:/data`
- Ebook root: `/data/media/books/ebooks`
- Audiobook root: `/data/media/books/audiobooks`
- Book torrent root: `/data/torrents/books`
- Standard search language: `English`

Do not commit real API keys, database passwords, Prowlarr keys, download-client
passwords, or provider tokens.

## Readiness Assessment

Librarry is roughly alpha quality:

- Good enough to test metadata search, wanted items, release decisions,
  Prowlarr search, qBittorrent handoff, and UI review workflows.
- Not yet good enough to retire Readarr without a controlled migration/import
  test and several successful full acquisition/import loops.

Explicit upgrade selections now process up to 200 selected books independently of
the queue batch limit, report ineligible selections as skips, and reject invalid
or missing IDs before starting work. Malformed request bodies can no longer
fall through to a default queue action. A 200-selected-plus-one-unselected
database regression and a 75-row desktop/mobile fixture exercise the boundary.
This is source/fixture qualification; global collection caps, durable bulk jobs
and live qualification remain open.

Native recorded file evidence now uses the shared schema-43 SQL projection. It
preserves incomplete-audiobook, alternate-import and pending-publication behavior
while enabling filtering/counting before collection pagination. A 10,001-ebook
fixture verifies complete counts and stable filtered traversal. Native Library
books and Wanted gap tabs now use global filters/counts and stable server pages
through `/api/v1/library/books`, with current-page bulk selection. A 10,001-book /
10,003-file fixture traverses all four sorts and preserves imported/unmonitored
membership while excluding removed/ignored books. Client/quality/file evidence
matches book details; outages remain explicit. This is source and isolated
fixture qualification. Native author subscriptions now also page across the full
collection with provider-identity/format book counts. Ambiguous or unresolved
identities remain explicit; manual author overrides are excluded from recorded
author membership. Native metadata Review now includes older/imported/unmonitored
books, full counts, paging and selections bound to the displayed evidence.
Confirmations are atomic and protect concurrent corrections and clears. The separate
author candidate review queue now pages all stored candidates and resolves each
decision atomically with book creation and history. Existing books retain owner
settings, and retries replay the saved decision. A 10,001-candidate fixture
traversed 101 pages without gaps or duplicates (local p95 approximately 7 ms).
Legacy readers, search badges, compatibility, removed-book browsing and
durable all-matching jobs remain unfinished under S14/S15.


Native file browsing now pages the full recorded collection in Imports and book
details. The new file table shows recorded presence separately from import status,
uses relational book links, preserves unassigned files, and hides destinations
belonging to unfinished imports. Imports counters cover the full collection.
Rename preview pages across files and applies only selected rows on the current
page; it remains accessible even when no tracked books are listed. A fixture with
10,001 linked files traverses 303 pages across path/title/updated sorts without
gaps, and a 1,500-chapter book is fully reachable. This is local fixture
qualification; legacy/compatible list endpoints and Calibre batch processing
retain separate scaling work. Durable collection-wide jobs remain unfinished.


Standalone native file renames now use durable import staging and cleanup. A
failed database commit retains the original, and a restart resumes the saved
target. File IDs, source provenance, owner metadata and relational links survive;
renames do not change wanted status or monitoring. Imports identifies saved
renames and offers retry. Original import receipts follow verified rename history
when checking their current destination, so a rename does not strand cleanup.
Migration 0044 reserves each active file rename and its old path until cleanup.

The file-by-file action retains known chapter sets and companion files with an
explicit explanation. The separate book action previews and moves a complete
recorded folder, preserving current basenames, disc paths and verified relative
CUE/M3U/OPF references. Migration 0045 reserves every media identity and retains
sidecar provenance. Atomic path commits and retryable cleanup preserve file IDs,
associations and monitoring through restart. Original receipts also follow
verified scan moves of the same bytes. Unproven sets, unknown files, unsafe
references and Calibre layouts stay for review. General chapter/reference
rewriting, whole-book replacement/retirement, Calibre recovery, broader disk
faults and live NAS qualification remain open under S09. These are unreleased
source/fixture changes, not a production rollout.


### Calibre client contract qualification

Source review and a real disposable Calibre 8.5 Content Server exposed two false
assumptions in the old HTTP fixtures: add-book returns its actual identity in
`book_id`, while `id` echoes the upload job; conversion job `0` is valid. The
client now uses the library ID and preserves job zero through metadata storage.
Malformed identities and incomplete conversion outcomes remain errors. Uploads
stream from the open source file, with bounded acknowledgement parsing.

The client negotiates default Digest or explicit Basic authentication through a
read-only library-info probe before transmitting a mutation. It sends the payload
once, does not replay an ambiguous write and does not follow redirects with the
integration credentials. Local ARM64 qualification covers both authentication
modes, metadata readback, EPUB-to-TXT conversion, deletion and an empty-library
readback; the original legal fixture stays unchanged. CI now runs the same
isolated server contract. This is a real disposable integration, not a homelab
or current-latest-Calibre certification.

Accepted uploads can still be repeated after later metadata/conversion or local
persistence failure. A controlled fixture reproduced two adds after one metadata
failure. Durable handoff receipts, uncertain-outcome reconciliation, terminal
conversion polling across failures/restarts, source-path/root resolution and
fair background conversion traversal remain open. Existing Calibre IDs are not
guessed or repaired automatically. S09 and the overall release gates remain open.


### Calibre handoff recovery qualification

New native-root Calibre imports use migration 0046's separate handoff journal.
Upload intent is saved before sending, and the acknowledged book ID is saved
before metadata or conversion work. Accepted handoffs retry without another
upload. Interrupted sends and lost terminal conversion responses remain for
operator review. Imports can attach an explicitly selected existing book, accept
a verified existing format, or authorize another upload/conversion after the
operator confirms inspection. These decisions are recorded in history.

Conversions are started and polled one at a time, preserving job zero and each
terminal acknowledgement before proceeding. The conversion background task
resumes accepted handoffs in oldest-updated order; uncertain sends are excluded.
File registration, wanted/download state, relational associations, installed
release and import history commit together. Owner changes during metadata sync
reject the commit and are read again on retry. Saved roots cannot silently change
servers; password rotation is allowed. Sources remain in place and no native
cleanup receipt is created. Existing Calibre IDs are not retroactively repaired.

Disposable Calibre 8.5 tests passed for Digest and Basic: a real accepted upload
survives metadata interruption or lost acknowledgement and a fresh service
instance, converts to TXT, commits once and is explicitly deleted from the test
library. Final server readback confirms zero remaining fixture books. Database
fault tests cover acknowledgement persistence, terminal-status persistence,
atomic bookkeeping, concurrent retry, target changes, owner edits, cross-client
identity, changed source bytes and background recovery. These are local fixtures;
Calibre-side path refresh, richer editions, legacy refresh fairness, live NAS
qualification and the unattended soak remain open.

### Shared worker coordination qualification

Registered background jobs and manual System Tasks runs now coordinate through
Postgres session ownership and a shared due time. Another API process cannot
claim a currently owned job or repeat a not-yet-due scheduled pass. Run state
survives restart. Lost sessions appear as interrupted; heartbeats cancel the old
worker context on coordination failure. Expired heartbeat age alone never steals
a live session lock. Manual jobs participate in shutdown. System → Tasks → History
retains 100 successes plus unreviewed failures, with paginated review and a separate
last-success record. Reviewed failures are eligible for cleanup after 90 days.

Two disposable packaged API processes were qualified against the same database:
the peer observed the active run, refused a concurrent trigger, detected SIGKILL,
recovered the task, and served the same history after the former owner restarted.
Postgres tests cover schedule persistence, stale heartbeats, lost connections,
stale completion, panic cleanup, bounded history, outage refusal and shutdown.
Desktop/mobile history testing includes errors, retry, focus restoration,
navigation and visual inspection. Direct business API operations still rely on
their acquisition/import journals. Notification outbox delivery, broader worker
side-effect qualification, full S23 operational diagnostics and live soak remain
open. No production or homelab rollout is implied by these container fixtures.

## Notification recovery (unreleased)

Migration 0048 captures new grab/upgrade and import history, download-failure
transitions, and persisted health transitions together with their native target
fan-out. Rollback discards the notification; recovery preserves one committed
event. Unassigned manual imports also receive committed history. Existing history
is not replayed and later-added connections do not receive older events.

The shared Notification Delivery task processes at most 25 entries per pass.
Each attempt has session ownership and a saved send intent. HTTP 2xx records
receiver acceptance. A lost response, HTTP 408/5xx, interrupted sender or failed
acceptance save stays uncertain without automatic resend. HTTP 429 retries with
backoff up to five total attempts; a requested wait above 24 hours needs review.
Redirects are refused. Deleted, disabled or changed connections stop queued sends;
confirmed retry binds the current settings revision. Credentials and receiver
response bodies are not copied into the delivery ledger or returned by its API.

Settings → Connect shows paginated status with confirmed retry, acceptance and
cancellation decisions. A stable delivery header lets a cooperating receiver
deduplicate, but does not establish exactly-once third-party delivery. Explicit
connection tests remain synchronous. Migration 0049 extends capture to enabled
Readarr-compatible webhook resources, including scheduled work and recovery.
Book, download, release and complete imported-file details are saved at commit,
so later edits/deletion do not rewrite the message. Methods, Basic/Bearer settings
and trigger aliases are retained; raw provider URLs and arbitrary file metadata
are excluded. Health messages require explicit opt-in for compatibility targets.
Terminal delivery/attempt records currently remain until backup/maintenance policy
is defined; bounded retention, support diagnostics and live qualification
remain open. No production notifications were sent during qualification.
