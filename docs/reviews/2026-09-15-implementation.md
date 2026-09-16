# Stabilization implementation ledger

Execution started September 15, 2026 on `codex/stabilize-librarry`. This is a
progress record, not a claim that the full stabilization plan is complete.

## Implemented in the branch

- Community PR #2 is cherry-picked with Zak Strassberg's authorship preserved.
  Prowlarr JSON searches send repeated categories.
- Acquisition configuration swaps use immutable snapshots. Store mutations scope
  client identity; ambiguous legacy external IDs fail instead of updating several
  clients. Remote acceptance/deletion followed by failed persistence reports an
  explicit error.
- Completed imports inspect exact client payload inventories. Traversal, symlinks,
  unknown membership and incomplete files cannot silently satisfy a book. Chapter
  sets and explicitly mapped packs now use the durable importer described below.
  Completed move mode is rejected to preserve seeding sources.
- Scan observations preserve authoritative names, source paths, JSON associations,
  manual metadata, and Calibre evidence. A real Postgres regression verifies two
  rescans preserve an imported record's identity.
- Copies stage and sync data before atomic, non-overwriting publication. Failed
  replacement transfer keeps the original. Replacement originals remain at a
  recovery path until persistence succeeds; manual moves remove sources only
  after record persistence. Native completed-import recovery is described below; native manual/replacement recovery is described in its continuation below.
- New completed imports store a client/ID/content-hash receipt. Cleanup rechecks
  exact client inventory, file count/progress/size, source and destination hashes,
  destination separation, and seed-goal evidence. Old imports do not inherit
  eligibility. Calibre-only remote files cannot satisfy local verification yet.
- Forms/basic startup fails without persistence or a usable user; invalid auth
  modes and unreadable persisted auth settings fail. Failed settings persistence
  cannot silently switch the method. Method and credential updates now commit together, with rollback coverage for
  a deliberately failed second write. Changed credentials revoke sessions; stale
  concurrent logins cannot recreate them. Env-owned fields are locked in Settings.
- Invalid typed environment settings now fail startup before database/worker work. Legacy integer-minute durations remain accepted; invalid completed import modes are rejected.
- Every advertised runtime env key reaches every installer. Defaults agree with
  the owner's auto-grab/removal decision; nondefault completed settings are tested
  through actual Compose rendering.
- System status has stable build identity, actual auth mode and migration version.
  Go is 1.26.8 with updated pgx/crypto/text. Frontend/tooling dependencies are
  patched, including a security-motivated React Router 7 update. nginx moves from
  1.27 to the current stable 1.30 line.
- Hardcover HTTP-200 GraphQL errors and malformed data no longer masquerade as
  successful empty searches. Typesense documents are decoded; health reports
  configured without inventing token verification or rate limiting.
- Direct book detail fetches and book-filtered file queries bypass collection caps. A 10,001-book/file database fixture verifies access to an old imported book; removed and unknown books return 404, outages remain distinct and retryable.
- Library has an explicit server view, preserves imported books, hides removed
  entries, and rejects invalid views. Query callback arguments cannot become
  filters. Empty lists normalize to arrays.
- Empty Library leads with adding a book; book totals include unmonitored books.
  Dialogs trap/restore focus and close on Escape. Closed mobile navigation is
  hidden from keyboard/screen-reader navigation; the open drawer traps focus.
  Route errors retain navigation, and auth probe failures offer retry.
- Added disposable Postgres tests, frontend tests, desktop/mobile browser smoke
  tests, and deployment checks. Image builds depend on the verification workflow.

## Qualification evidence so far

- Full Go suite passed with `-race` and disposable Postgres on Go 1.26.8.
- Go vet and deployment render checks passed. Frontend unit tests and production
  build passed; the expanding browser suite is rerun after UI changes.
- Latest local browser run: 15 passed, one desktop-only inapplicable mobile test
  skipped, at 1440 and 390 pixels. Covered fresh persisted lists, eight core
  routes, no console/page errors, no page overflow, outage recovery, dialog focus
  containment/restoration, environment-owned auth controls, direct-book outage recovery, mobile navigation accessibility, and import-recovery failures/plans at both sizes.
- `govulncheck` on Go 1.26.8 reported zero reachable vulnerabilities, zero affected
  imported packages. The remaining module-only advisory, GO-2026-5932, concerns
  unmaintained `golang.org/x/crypto/openpgp`, which this project does not import.
  npm audit after the router
  update reported zero vulnerabilities. Initial OS scans found fixable OpenSSL/libuuid findings. After runtime package refresh, Trivy 0.74.0 reported zero OS findings in both ARM64 images; the API retains only the unused module advisory above.
- An ARM64 API candidate image built and started. It reported Go 1.26.8, schema
  29, the injected commit/build timestamp, and auth `none`. Configured forms with
  no database exited with code 1 before listening.
- The API+web image pair passed exact imports, sibling/multipart rejection, source retention, repeat-import skipping and two scans through nginx. A subsequent isolated dump/restore preserved wanted/download/file counts and the verification receipt (89,577-byte dump).
- An isolated Postgres custom-format dump restored 29 migrations and a file's
  fixture provenance (88,021-byte dump). This is fixture evidence, not a restore
  of the live September backup or a complete media rollback drill.
- Exploratory Chromium review confirmed the empty-state improvement and hidden
  mobile drawer. Before/after screenshots are local `output/playwright/` artifacts.

## Plan accounting

| Work | State | Remaining gate or scope |
|---|---|---|
| S01 | Substantially implemented | Live predeployment file/database inventory and backup-copy restore |
| S02 | Implemented locally | Verification, packaged safety/auth/restore tests and scan gates implemented; latest run linked on PR #3 |
| S03 | Partial | Unified effective configuration/source/precedence view for settings beyond auth |
| S04 | Implemented safety guard | Packaged safety regressions and multipart review passed; controlled live qualification remains |
| S05 | Implemented | Read-only live audiobook search against the candidate |
| S06 | Partial | Latest multi-platform checks linked on PR #3; upgrade/rollback and candidate deployment remain |
| S07 | Implemented with fixture qualification | Relational links, manifest/operation records and reconciliation report; live database-copy migration still pending |
| S08 | Implemented with fixture qualification | Exact adapter inventories, complete chapter sets, sidecars and reviewed per-book mapping; real-client qualification remains S21/S24 |
| S09 | Partial | Durable native/manual plans, staging journals, move and reviewed destination replacement recovery; changed chapter-set retirement, Calibre and broader disk fault qualification remain |
| S10 | Partial | Durable submission/reconciliation, accepted bookkeeping, native installed-release/history commit and replay notification guards implemented with fixtures; full worker, legacy repair and live-client qualification remain |
| S11 | Implemented with fixture qualification | Persisted scans, guarded presence, unambiguous native move reattachment and legacy repair previews; controlled live NAS/root qualification remains S21/S24 |
| S12 | Partial | Observed provider health, explicit checks, error sanitization, request backoff, exact Google fallback and bounded result caching implemented; stable-ID author traversal implemented with fixtures and read-only Open Library proof; shared request pacing/quota backoff and complete import-list traversal implemented; default edition enrichment/exact ISBN lookup implemented; broader edition/series discovery, persistent raw records and live Hardcover/Google credentials remain |
| S13 | Partial | Add-only bibliography/list sync preserves existing tracking; complete list traversal, atomic list defaults and full exclusions implemented; format/language/edition merge checks and concrete edition aliases implemented; matching corpus and full author monitoring policy qualification remain |
| S14 | Partial | Library view and direct book/file queries fixed; author detail lookup and full verified lifecycle semantics remain |
| S15–S16 | Not complete | Pagination/counts/scale and full compatibility/migration contracts |
| S17–S19 | Not complete | Setup, search/add, activity/import-review redesign and workflow qualification |
| S20 | Partial | Empty state, recovery and keyboard fixes; populated/degraded/tablet/full accessibility review remains |
| S21–S25 | Not complete | Remaining release/security/integration/documentation gates and timed soak |

No production deployment, tag, release, live backup restore, arbitrary grab, or
72-hour soak has been completed by this work. The approved full plan remains the
scope for subsequent implementation; this ledger must not be used to mark it done.

## Primary references checked during implementation

- [qBittorrent 5.0 serializer](https://github.com/qbittorrent/qBittorrent/blob/release-5.0.0/src/webui/api/serialize/serialize_torrent.cpp)
  and [torrent implementation](https://github.com/qbittorrent/qBittorrent/blob/release-5.0.0/src/base/bittorrent/torrentimpl.cpp):
  effective maximum seeding time is in minutes, despite the API wiki's seconds
  label; elapsed seeding time is seconds. The adapter test covers this discrepancy.
- [Hardcover search guide](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/guides/Searching.mdx):
  Typesense search results and supported document fields.
- [Go downloads](https://go.dev/dl/) and [nginx stable release](https://nginx.org/en/download.html).
- [React Router navigation advisory](https://github.com/advisories/GHSA-wrjc-x8rr-h8h6)
  motivated upgrading the router rather than leaving its version finding unresolved.

## Candidate identity and CI follow-up

- Packaged fixture qualification also passed on API source
  `6130a6a6edbf2246051abfb5edea88b9a050a58d` with the refreshed web image.
- The first expanded Linux CI qualification passed every application/restore
  assertion, then failed fixture cleanup because the runner uid differed from
  container uid 1000. The harness now returns only its unique fixture tree to
  the runner owner before cleanup. A later CI TypeScript check also caught an
  obsolete error branch after the new early-return error state; it was removed.
  Final verification is recorded by the checks on [PR #3](https://github.com/bandoracer/librarry/pull/3).
- `scripts/test-packaged.py` is the maintained reproduction; it does not connect
  to production clients or use provider credentials.
- Trivy usage follows its [container image documentation](https://trivy.dev/docs/dev/guide/target/container_image/).
  CI pins the scanner image digest and rejects fixable high/critical findings.

The packaged auth matrix passes forms enforcement, login cookie persistence,
restart with persisted configuration/session, Basic credentials, and restoring
explicit none. This uses fixture credentials only. All real-provider and live
homelab gates remain separate from this local/CI evidence.


## Continuation: relational links and durable import recovery

- Append-only migrations 0030–0031 preserve unambiguous legacy relationships,
  report unresolved links, and add import/file manifests. Re-running migration is
  idempotent and does not grant old rows cleanup eligibility.
- Native completed imports persist immutable paths, size/hash evidence, source
  identity, and mode before copying. Expiring renewable leases fence stale workers;
  retries reuse verified publications rather than create a renamed duplicate.
- One transaction commits all file records/links, wanted status, download
  projection, and operation state. A database trigger blocks premature visibility
  from scans and compatibility writes. Scans preserve historical path aliases.
- Configured same-basename sidecars in a dedicated payload directory are required
  manifest members. Removing one blocks retry. This initial restriction is superseded by the S08 continuation below.
- Recovery API and Imports controls show saved plans, attempts, failure reason,
  cleanup status and unresolved legacy links. Retry accepts only an operation ID.
- Cleanup verifies relational ownership and every manifested file again, and stores
  client deletion failures separately from the committed import. Manual and remote
  Calibre imports remain outside the native durable engine; automatic replacement
  requires review and keeps the existing file intact.
- Fault tests inject a failure in the final download write after publication and
  preceding file/book writes. The transaction rolls back, a scan cannot expose the
  copied file, and a new service resumes the original destination. Other tests cover
  expired/stale leases, changed bytes, missing sidecars, migration ambiguity,
  missing ownership, and cleanup client failure.

Continuation verification: the full Go race suite passed with disposable Postgres,
Go vet passed, six frontend tests passed, and the production web build passed.
Desktop/mobile browser checks: 15 passed, one inapplicable desktop test skipped.

The locally built c62268f API/web pair passed the expanded packaged harness on
schema 31: a forced final database-write failure rolled back all library/book
records after filesystem publication; a real API container restart resumed the
same plan. Rescans, committed retry, source retention, multipart rejection and
authentication checks passed. A 111,639-byte isolated dump restored matching
operation IDs/states, manifests/hashes/file links, relational links, legacy
receipts and entity counts. This remains fixture evidence, not a live restore.
Inventory lookup and unapplied remote-deletion failures are also persisted and
reported by the cleanup worker instead of appearing as successful task runs.


## Continuation: exact file sets and operator mapping (S08)

Branch: `codex/multipart-imports`, stacked on the safety/recovery PR.

- qBittorrent and Transmission preserve explicit per-file selection separately
  from priority. Missing selection evidence remains unknown. SABnzbd accepts only
  successful completed history with a final output directory; queue/archive file
  names cannot stand in for extracted files.
- Client inventories resolve boundary-checked relative paths. Traversal, aliases
  resolving to duplicate sources, symlinks (including excluded entries), unknown
  inventories and shared-folder search are rejected. Complete media/sidecars must
  match client byte counts and completion state.
- Single EPUB/M4B and multi-disc MP3 imports preserve all required files. Natural
  chapter order remains stable on committed retries. Sidecar sets retain their
  names/relative paths in a fresh destination folder. Local ISBN, title, author,
  album and chapter evidence gates automatic grouping.
- Migration 0032 adds per-manifest-file wanted identity and client-scoped pending
  review uniqueness. Multi-book packs require explicit file assignments. Each
  destination book receives its own relational file links in the final transaction.
- Review lists every file and exclusion, provides per-file book/retain choices,
  and requires an acknowledged destination preview. A hash of the complete plan
  rejects changed bytes, destinations or assignments. Preview writes no library
  directories or operation records. Skip/reject persist across worker runs;
  reopening restores manual review rather than silently importing.
- Whole-set recovery validates current inventories before and after transfer,
  then commits all required files together. Cleanup verifies all chapters/sidecars
  and refuses any required member retained in downloads by an explicit mapping.
- Existing single-book Calibre handoff remains available without native verified
  cleanup eligibility. Multipart Calibre, manual-path recovery, replacement and
  temporary-stage reclamation remain open S09 work.

Qualification on September 15 (local Pacific time): full Go race/Postgres suite,
Go vet, six frontend tests, production build and 17 browser checks passed (one
mobile-only case is inapplicable on desktop). Browser fixtures exercised mapping,
preview invalidation and retained server errors at 1440/390 pixels. Screenshot
inspection caught and fixed an incorrect empty-review message beneath payload
reviews. Adapter fixtures cover selected/unselected/unknown qBittorrent and
Transmission files and successful/failed/in-progress SAB history.

Disposable ARM64 API/web images with schema 32 passed the packaged harness:
missing payload enters review; a forced database failure hides unfinished files;
a real process restart resumes the saved plan; three chapters across two discs
and a cover import completely; source bytes remain intact; rescans preserve links.
A subsequent packaged HTTP review test passed read-only preview, 409 stale-token
rejection, explicit mapping import and source retention. A 115,911-byte database
dump restored matching operation/manifest/link state.
Forms, persisted sessions across restart, Basic and explicit none checks passed.
This test used a read-only qBittorrent HTTP contract fixture, not a live client.
Image CI for the eventual commit is tracked on the stacked PR.

S08 references: [SABnzbd API](https://sabnzbd.org/wiki/configuration/5.0/api)
(history storage versus get_files archive inventory) and
[Transmission RPC specification](https://github.com/transmission/transmission/blob/4.0.6/docs/rpc-spec.md)
(per-file wanted/progress metadata). These contract fixtures do not replace
version-specific real-client qualification in S21.


## Continuation: journaled temporary copies (S09, in progress)

Branch: `codex/import-stage-recovery`, stacked on multipart imports.
Migration 0033 journals native temporary paths with their file and worker-lease
identity before creation. Copies verify expected manifest bytes before publication;
lease renewal after copying prevents an expired worker from publishing. Restart
reloads the journal and reclaims only a validated recorded stage, synchronizes the
directory, and clears the journal. Unrelated temporary files and older unrecorded
stages remain untouched. The recovery panel exposes pending stage paths.

Fault tests cover interrupted copies, interruption after publication, stale caller
snapshots across worker takeover, expired-worker staging, changed source bytes,
forged journal paths, symlink stages and preservation of unrelated files. These
checks extend native completed-import recovery; S09 still includes manual/Calibre
operations, recoverable replacement and broader filesystem fault qualification.


Multipart PR #4 at `96d4f8444430118d450345a79c314a90a1ec8209` passed
[Linux CI run 35054048047](https://github.com/bandoracer/librarry/actions/runs/35054048047):
source/race/browser verification, packaged API/restore qualification, image scans,
and API/web AMD64/ARM64 builds. No PR images were published.

Local staging recovery qualification passed the full Go race/Postgres suite,
Go vet, frontend build, and 17 desktop/mobile browser checks (one inapplicable
desktop case skipped). The schema-33 ARM64 API container restarted with a seeded
journaled interrupted copy, reclaimed that exact file, preserved an unrelated
file, resumed its original destination and completed the broader import/auth
suite. A 116,148-byte isolated dump restored matching staging/manifest/link state.


## Continuation: native manual imports and replacements

Branch: `codex/manual-import-recovery`, based on the staging recovery branch.
Migration 0034 adds manual request identity, optional book/download association,
previous-file manifests and per-file move cleanup. Manual copy, hardlink, move,
in-place adoption and file replacement now use the durable engine. Configured
sidecars participate in verification rather than failing silently. Repeated
requests reuse the operation, including after a moved source disappears.

Replacement preserves the previous inode at a saved, hash-checked recovery path
until the new content and database transaction commit. Pending replacement paths
are excluded from native file queries. Filesystem publication and destructive
cleanup hold a database ownership-row fence, preventing a lease takeover during
the mutation. Committed manual operations with pending cleanup remain counted,
visible and retryable. The manual form supports an explicit book assignment and
keeps its move/conflict choices separate from completed-download imports. A configured recycle bin no longer falls back to permanent
deletion when unavailable.

New tests exercise all transfer modes, optional wanted identity, in-place import,
failed final database commit, replacement rollback/retry, unavailable recycle-bin
recovery, unacknowledged move deletion and takeover blocking during filesystem IO.
The full Go race/Postgres suite, vet, frontend build and 21 desktop/mobile browser
checks passed (one mobile-only check is inapplicable on desktop).

The schema-34 ARM64 packaged API passed forced manual-commit failure followed by
actual process restart, retry of the same saved destination, post-commit move
cleanup and duplicate-request suppression. The broader payload/staging/review/auth
suite passed; a 118,533-byte dump restored matching identities/manifests/cleanup.
This remains fixture qualification. Calibre recovery and completed-download
replacement remain open; the full S01–S25 plan is not complete.

Staging PR #5 at `15e7743d5b872b20c007a53229963c89b1c1aba8` passed
[CI run 35054590253](https://github.com/bandoracer/librarry/actions/runs/35054590253),
including packaged qualification, image scans and AMD64/ARM64 image builds.


## Continuation: acquisition submission recovery (S10)

Branch: `codex/acquisition-recovery`, based on manual recovery. Migrations 0035 and
0036 add unique active book/release reservations, expiring submission claims,
accepted receipts and unresolved outcomes. All production grab entry points share
the acquisition service. External submission never holds a database transaction.
A retry/expired claim reconciles exact client identity instead of resending.
A changed endpoint, unavailable client, empty response or matching title alone
cannot authorize a duplicate. A known acceptance is persisted before download
bookkeeping; replay repairs a missing row without resetting existing progress.

Activity exposes unresolved attempts, backoff, direct checks and explicit
operator-confirmed attach/release actions. SABnzbd lost acknowledgements require
an exact selected client ID. Authoritative book associations survive tagless client
observations. Replayed receipts suppress duplicate grab notifications/history;
more extensive history/current-release repair remains to be implemented.

Qualification: full Go race/Postgres suite, vet, production build and 23 applicable
desktop/mobile browser checks passed (one desktop case is inapplicable). Tests
cover concurrent service instances, accepted-then-error, persistence failure,
restart, original-client identity, legacy active downloads, manual/worker release
collisions, explicit release and imported-book upgrades. API tests reject missing
operator confirmations. The mobile recovery confirmation was inspected visually.

The schema-36 ARM64 image passed a simulated client acceptance followed by HTTP
502, actual API-container restart, exact reconciliation and duplicate suppression
(one total add). The same harness passed prior import/staging/review/auth checks;
a 123,275-byte dump restored matching intent and import records. The isolated
client fixture performs no actual network downloads. Current adapter references:
[SABnzbd add/queue API](https://sabnzbd.org/wiki/configuration/5.0/api) and
[Transmission 4.0.6 RPC](https://github.com/transmission/transmission/blob/4.0.6/docs/rpc-spec.md).
Live client/worker qualification and remaining S10 reconciliation are still open.

Manual recovery PR #6 at `8dcd0c1042811161ba343f9a74c70cee4f1a9fb2` passed
[CI run 35056853220](https://github.com/bandoracer/librarry/actions/runs/35056853220),
including source/browser/race verification, packaged restart/restore/auth checks,
image scans and AMD64/ARM64 builds. No PR images were published.


## Continuation: persisted scans and local presence (S11)

Branch: `codex/resumable-scans`, based on acquisition recovery. Migration 0037
replaces the scan's total-file cap with a persisted directory/file queue and
batch progress. A scheduler task resumes jobs, including after process restart;
Imports shows progress, failures, cancellation and retry. Root device/inode and
per-file device evidence guard against unavailable/replaced/nested mounts.

Missing observations are staged across batches and rechecked before atomic final
publication. Cancelling or failing leaves prior presence intact. File changes
between batches, source alias paths, stale claims, and final database failure are
covered. Manual names, original source paths and relational/import provenance
survive observations. Native files now expose separate local presence evidence.
Automatic moved-file reattachment, legacy repair previews, and shared book-state
projections are still outstanding; S11 is not complete.

The full Go race/Postgres suite passed, including 10,001 physical fixture files
continued by a new service instance and rescanned without duplicates. Focused
fault tests cover staged absence cancellation, reappearing files, unavailable
roots, changed root identity, lost nested-device evidence, failed final database
writes, and expired worker cancellation. Frontend build and 27 applicable browser
checks passed (one desktop case is inapplicable); mobile progress was inspected.

A schema-37 ARM64 packaged API resumed 1,201 files through the scheduler after
SIGKILL/restart (the test advances the expired lease instead of idling two minutes).
A completed second scan confirmed one deleted file; making the root unavailable
preserved the remaining presence. Existing import/acquisition/auth checks passed;
a 295,892-byte dump restored scan, presence, intent and import records. This is
isolated fixture evidence, not a live media-root or NAS-mount qualification.

Acquisition PR #7 at `7443a908f16005806016ad36094866b6521a3d95` passed
[CI run 35058040266](https://github.com/bandoracer/librarry/actions/runs/35058040266),
including packaged qualification, image scans and AMD64/ARM64 builds.


## Continuation: library repair preview (S11)

Branch: `codex/library-repair-preview`, based on persisted scans. Imports can now
inspect saved evidence for broken book/download associations, duplicate recorded
SHA-256/size/format groups, possible moved files, legacy audiobook imports without
complete manifests, and committed-manifest discrepancies. Findings show exact
identifiers, sample paths, total candidate counts, reasons and recommended actions.
Single-file audiobooks are explicitly not assumed incomplete. Intentional copies,
hardlinks, deliberate moves and replacements remain review decisions.

The API reads each page in a read-only repeatable-read transaction and inspects at
most 100 entities. Keyset cursors traverse files, imported downloads and committed
operations, including pages with no findings. Candidate samples are explicitly
limited to 20 while totals remain visible. Pages are live observations, not one
frozen cross-page snapshot. Raw file metadata/provider credentials are not returned.
Generating the report never rewrites links, manifests, presence or cleanup state,
and cannot authorize deletion or fuzzy reassignment. Automatic unambiguous move
reattachment is still outstanding under S11.

Database tests cover malformed/ambiguous legacy IDs, missing exact links, recorded
content duplicates, one-versus-many move candidates, incomplete-scan exclusion,
healthy imports and later missing/deleted tracked files. A before/after record
snapshot verifies that previewing makes no writes. A 205-file fixture verifies
pagination across clean pages. API tests cover empty lists, invalid cursors,
unavailable persistence and absence of a mutation route. Desktop/mobile browser
coverage exercises clean-page continuation, request failure/retry, evidence paths,
long content hashes, fresh reports and no horizontal overflow.

Persisted-scan PR #8 at `161efe2233fd33c0c0526bbe7a42d30cb3918e14` passed
[CI run 35060952968](https://github.com/bandoracer/librarry/actions/runs/35060952968),
including source/race/browser checks, packaged restart/restore qualification,
image scans and AMD64/ARM64 builds. No images were published or deployed.


Qualification for the repair preview: full Go race/Postgres suite and vet passed;
production frontend build and 29 applicable desktop/mobile browser checks passed
(one desktop case is inapplicable). The local ARM64 packaged API inspected all
report pages over 1,200 records, exposed controlled legacy/duplicate/audiobook
findings, rejected unauthenticated access, and left file rows byte-for-byte
unchanged. The existing recovery/scan/auth harness also passed, including a
295,860-byte database restore. These are disposable fixture results, not a live
library audit. The packaged local web image is the earlier multipart snapshot;
new report UI behavior is covered by the current-source browser/build checks.


## Continuation: unambiguous moved-file reconciliation (S11)

Branch: `codex/moved-file-reconciliation`, based on repair preview. Migration 0038
records the authoritative baseline of scan-created discoveries, the exact
filesystem observation associated with a scan hash, and durable move history.
A completed scan can retain an absent original ID at a new observed location
when exactly two records share its SHA-256, size and format, and the new record
is still an untouched, unassigned scan discovery. Both locations must participate
in the completed scan; interrupted/cancelled discovery evidence can be reused by
a later complete scan. Legacy records without discovery evidence remain review.

A final move check validates roots/devices, source absence, the destination's
inode/size/nanosecond mtime, saved row versions and current associations. File/path
locks prevent concurrent assignments or imports from being silently discarded.
The scan's already-computed hash avoids rehashing an entire moved library in one
unbounded final pass. Any database failure rolls back ID/path reconciliation,
missing-state publication and history together. The original file row preserves
manual metadata, source path, book/download relationships and import history.
Only the redundant unassigned discovery row is removed; no media is renamed or
deleted. Old import manifests and cleanup receipts do not silently follow moves.
Calibre-owned paths/records and ambiguous same-content groups remain in review.

Imports displays reattached counts and paginated previous/current path history.
The synchronous scan result returns retained IDs after reconciliation. Move
history preserves historical IDs even after later deliberate file removal.

Qualification includes original import/link/override preservation, failed-commit
rollback and service restart, cancellation, changed content/inodes, manual edits,
Calibre ownership, reappeared sources, concurrent manual assignment under an
observed database lock, and 205 files moved across two configured roots with
restarted batches. History pagination covers 202 rows. Full Go race/Postgres
suite, vet, frontend build and 31 applicable desktop/mobile browser checks passed
(one desktop case is inapplicable). Mobile history was inspected visually.

Repair preview PR #9 at `7f8104f607f33e827eb305273e1b9db00aae4ef9` passed
[CI run 35061686219](https://github.com/bandoracer/librarry/actions/runs/35061686219),
including packaged qualification, source/race/browser checks, image scans and
AMD64/ARM64 builds. No production deployment or image publication occurred.


The schema-38 local ARM64 packaged API passed an imported audiobook chapter move,
forced reconciliation-commit failure, actual container restart and retry. The
original chapter ID, wanted/download links and historical manifest were retained;
source files remained present. A 399,910-byte dump restored matching scan moves,
discovery baselines, filesystem stamps, imports and acquisition records. Restore
comparisons now use deterministic ID ordering (the first run exposed an unordered
comparison). Existing packaged import/review/acquisition/scan/auth checks passed.
The local web image remains the earlier multipart snapshot; new history UI is
qualified by current-source browser/build checks. Live NAS/mount proof remains
outside this fixture result.


## Continuation: reviewed completed-destination replacement (S09)

Branch: `codex/completed-replacement`, based on moved-file reconciliation.
Completed-download payload review now offers Keep both (default) or Replace
reviewed files. Replacement requires a current preview and explicit book mapping.
Its deterministic recovery paths bind the exact client/download, destination and
old/new hashes, so a stale destination invalidates the preview without making
preview generation mutate files. Ebook and matching multi-disc/sidecar sets use
the existing staged-publication/recovery engine. Extra old files outside an
incoming book-directory manifest are retained for review rather than mixed into
a new chapter set or deleted implicitly. Complete retirement of a different old
chapter layout remains S09 work; this is not full automatic upgrade qualification.

Migration 0039 separates completed replacement-backup cleanup from download-source
cleanup. A committed import with a blocked recycle path stays visible and
retryable in Import recovery. Source removal cannot become eligible while backup
cleanup is pending. New content verifies before each replacement; all saved old
bytes remain available until the complete operation commits. Ownership is checked
at preview, publication and commit, and current manual file names/notes survive.
Retained file IDs keep historical download links; old import manifests remain
unchanged. Retrying an already committed replacement resumes backup cleanup.

Tests exercise ebook and multi-disc replacement, stable previews, stale old bytes,
unknown extra chapters, changed book ownership, forced final database failure,
service restart, retained source and previous hashes, file-ID preservation,
manual metadata and blocked recycle cleanup. Frontend production build and 35
applicable desktop/mobile browser checks passed (one desktop case inapplicable),
including replacement-choice invalidation and independent backup/source status.

Move reconciliation PR #10 at `c1f8f5f98787e6ccb16708bf6f93621d8daace20` passed
[CI run 35063006004](https://github.com/bandoracer/librarry/actions/runs/35063006004),
including source/race/browser checks, packaged qualification, image scans and
AMD64/ARM64 builds. No images were published or deployed.


The schema-39 ARM64 packaged API passed reviewed chapter/sidecar replacement with
a forced final database failure, actual container restart and retry of the saved
operation. It retained old bytes through failure, preserved original file IDs,
cleaned recovery backups after commit and retained all download sources. A
401,097-byte dump restored matching replacement-cleanup state and the existing
import/acquisition/scan records. Existing packaged recovery and authentication
checks passed. The local web image is the earlier multipart snapshot; the current
UI passed its production build and 35 applicable browser checks, plus a final
six-case desktop/mobile review run with mobile preview inspected visually.

Full Go race/Postgres and vet checks passed. Final focused race tests additionally
cover restoring a physically missing destination under its existing ID/manual
name, and rejecting an extra old file that appears after planning. The directory
manifest is rechecked before transfer and before commit. These are disposable
fixture results; no release, live upgrade or homelab qualification is implied.


## Continuation: acquisition and import bookkeeping (S10)

Branch: `codex/acquisition-bookkeeping`, based on reviewed replacement. The prior
code changed a book's current release at grab time, before the selected upgrade
was installed; it could also lose grab history after remote acceptance or send
notifications again when retrying committed imports.

Migration 0040 saves the selected release's sanitized ID/score/provenance before
client submission, plus bookkeeping completion and an explicit download-to-intent
link. The accepted client receipt commits first. A separate short transaction
repairs download/release links, eligible wanted state and one grab-history entry.
Failures remain visible as Accepted / Finish recovery in Activity, with no new-add
controls. Concurrent receipt replays serialize, preserve current download progress
and user removals, and do not duplicate history. Migrated intents do not gain
invented release selections or retrospective history.

Native import commits installed-release selection and one import event per mapped
book alongside files, associations and status. A failed history write rolls back
all database projections while retaining recoverable published bytes. Retry uses
the same operation. Scores come from the saved acquisition, including a valid zero;
a later search or a different book in a mapped pack cannot supply installed quality.
Pending acquisition bookkeeping prevents import commit until recovered. Failed
upgrades retain imported book state, and opaque-client failures use their own
saved release identity instead of blocklisting the installed release. Native/API
notification producers and worker grab counts suppress replayed outcomes. Remote
notification delivery remains best effort, not a durable outbox.

Qualification: full Go race/Postgres suite and vet passed, plus a focused real
wanted-service → acquisition-service → local qBittorrent contract test. Tests cover
lost acknowledgements, history-write failure, concurrent restart/replay, immutable
score snapshots, removal preservation, missing projection repair, foreign release
rejection, import-history rollback and mapped-book isolation. Production frontend
build and 37 applicable desktop/mobile checks passed (one desktop case inapplicable);
the accepted-recovery mobile state was inspected visually. The local ARM64 schema-40
API passed actual restarts after acknowledgement loss and a forced history failure,
repaired one history event without a second client add, and passed all existing
packaged import/scan/replacement/auth checks. A 404,213-byte database dump restored
matching history, selection, installed-release and bookkeeping state. Final focused
acquisition/wanted race tests also verify recovery returns the repaired release link.
Packaged web
remains the earlier multipart snapshot; changed UI is qualified from current source.

Reviewed replacement PR #11 at `8b91a2d54b7869568da1f67c1ae709d5ce3681a4` passed
[CI run 35064804252](https://github.com/bandoracer/librarry/actions/runs/35064804252),
including source/race/browser, packaged restart/restore, image scans and AMD64/ARM64
API/web builds. No images were published or deployed. S09/S10 remain partial:
changed-layout retirement, Calibre recovery, legacy repair, broader worker/fault
qualification and the live release gates are still open.


## Continuation: observed provider health (S12)

Branch: `codex/provider-health`, based on acquisition bookkeeping. Remote providers
previously returned configured/ready state and fresh checked timestamps without
observing the network. System now separates configuration from actual last request
and last success, with nullable reachability/authentication and a retry time.
Snapshot reads spend no provider quota. Explicit checks use Hardcover's documented
read-only account query or a one-result Open Library/Google ISBN lookup; local OPF
cannot prove remote readiness. No account details are exposed or persisted.

A provider serializes requests, bounds queued waits, coalesces recent explicit
checks for 15 seconds and honors HTTP 429 backoff. Cancellation does not invent an
outage. HTTP/GraphQL failures distinguish rejected credentials, forbidden access,
rate limiting, degraded responses and unavailability. Malformed/missing lists do
not become healthy empty searches; valid empty responses remain valid. Request
errors exclude provider bodies and credential-bearing URLs, including Google keys.
Observations are process-local and reset on restart/reconfiguration.

Full Go race/Postgres suite, vet and frontend production build passed. The 37 prior
browser checks passed; the new desktop/mobile cases passed after correcting a test
expectation for the existing lowercase status labels. Mobile success was inspected
visually. Contract tests cover successful/rejected credentials, malformed responses,
concurrent check coalescing, cancellation, missing keys, rate limits and redacted
network errors. The local ARM64 schema-40 packaged API passed startup observations,
missing-token checks, authenticated route enforcement and all existing
import/acquisition/scan/replacement checks. A 404,543-byte database dump restored.
Packaged web remains the older multipart snapshot; changed UI is qualified from
current source. Real provider credentials have not been supplied or tested.

Acquisition bookkeeping PR #12 at `ea7587fd18575db4ba313380a6ac9205893b3a6f` passed
[CI run 35066114133](https://github.com/bandoracer/librarry/actions/runs/35066114133),
including source/race/browser, packaged restart/restore, image scans and AMD64/ARM64
API/web builds. No images were published or deployed. Rich provider traversal,
Google's exact-fallback enforcement, result caching and live qualification remain
S12 work; this is not completion of the stabilization plan.


## Continuation: exact Google fallback (S12)

Branch: `codex/exact-metadata-fallback`, based on provider health. Primary providers
now run before Google regardless of registration order. Google is skipped for
exact suitable primary matches and for all author/bibliography/series queries.
ISBN-10/13 checksums and equivalent 978 identifiers are validated; explicit invalid
ISBNs cannot become text searches. Title fallback compares every word, including
subtitles, with Unicode composition/case/punctuation normalization and retained
accents. Exact results rank before fuzzy primary results. Known conflicting
languages/formats are filtered before merging; explicit Any language is preserved.
Primary/fallback errors retain successful partial results.

The Google adapter requests full projection and explicit identifier fields, checks
returned matches, preserves contributors/language/source IDs and derives ebook
format only from provider evidence. Unknown format remains unknown even for an
audiobook query. Existing provider-alias merge coverage remains, now separate from
fallback orchestration tests. A title-only match remains a candidate rather than
proof of author/edition identity. No real Google key has been qualified.

Provider health PR #13 at `83e78f187baa45a2e0a7c95efd0eaa813e4f8a45` passed
[CI run 35067198012](https://github.com/bandoracer/librarry/actions/runs/35067198012),
including source/race/browser, packaged restore and AMD64/ARM64 image builds.
No images were published or deployed.

Contract references: [Google volumes search](https://developers.google.com/books/docs/v1/using#PerformingSearch)
and [volume fields/projection](https://developers.google.com/books/docs/v1/reference/volumes).

Validation: full Go race/Postgres suite, vet, production web build and all 39
applicable desktop/mobile browser checks passed (one desktop-only case skipped on
mobile). The local ARM64 schema-40 API passed packaged import/restart/replacement,
acquisition bookkeeping, scan/move/repair and authentication regressions; an
isolated 403,593-byte database dump restored with counts and receipt intact. The
packaged web is the older multipart snapshot; current UI was tested from source.


## Continuation: bounded metadata search caching (S12)

Branch: `codex/metadata-cache`, based on exact fallback PR #14. Repeated successful
provider queries now reuse immutable serialized snapshots for five minutes; valid
empty searches expire after 30 seconds. Cache keys include all query dimensions.
Entries are bounded by count, result/key bytes and per-entry size, with LRU eviction.
Large results still return in full without storage. One provider slot rechecks
after waiting so simultaneous identical misses share a successful request. Caller
cancellation cannot leave detached work or clear earlier successes.

Errors are never cached, and failed requests/checks clear only the affected
provider's entries. A generation fence rejects repopulation by an older in-flight
fetch. A cache hit cannot advance actual request/health evidence. Service/provider
configuration changes reset caches and the constructor copies the provider list.
No persistent schema or deployment settings changed. Rich traversal, durable raw
records and real credentials remain open under S12.

Validation: cache contracts passed under the race detector, including expiration,
all query dimensions, nested response isolation, count/byte eviction, concurrent
misses, canceled callers, provider-scoped failures, health timestamp preservation
and in-flight invalidation. Full Go race/Postgres suite and vet passed. The local
ARM64 schema-40 packaged API passed import/acquisition/scan/restart/replacement,
repair, move and authentication regressions; a 404,789-byte database dump restored
with counts and receipt intact. Packaged web remains the older multipart snapshot;
this continuation changes no UI. No image was published or deployed.


## Continuation: complete author bibliographies (S12/S13)

Branch: `codex/author-bibliographies`, based on metadata cache PR #15. The monitor
previously treated capped search results (normally 20 books) as a whole author
bibliography. It now verifies a stable provider author ID, follows every page and
applies policies only after successful traversal. Open Library validates declared
counts/identities; Hardcover walks ordered book IDs and verifies contributions.
Timeout/size bounds, duplicate/nonadvancing IDs, changed counts, malformed data
and page failures return errors without applying a partial list. Name-hash legacy
subscriptions fail visibly instead of guessing; existing wanted items remain.

Hardcover author search now retains numeric identities and images. Same-name
identities no longer merge or satisfy another subscription. Contributor roles are
preserved; unknown/non-writing roles enter review. Filtered candidates cannot
supplant an eligible book in first/latest policy. Original publication dates have
a separate field; unknown edition format stays unknown. Failed filter/policy or
wanted/review persistence does not mark an author synced.

Contract and database tests cover 205-book traversals, failures, empty-vs-missing
author identity, no partial mutation, repeated sync without duplicate rows,
original dates, and writing-credit review. A live read-only Open Library probe
retrieved 418 works across six conservatively paced requests, checking declared
counts and IDs. No catalog mutations or download grabs occurred. This is not
Hardcover token, production pacing or rich edition qualification.

Exact fallback PR #14 at `58bee619797e805a6512ed3c74afcb59b896d1c2` passed
[CI run 35068283820](https://github.com/bandoracer/librarry/actions/runs/35068283820).
Metadata cache PR #15 at `d658fa0e88a4ffb68dfdfd38f469f98d4720c56d` passed
[CI run 35068912844](https://github.com/bandoracer/librarry/actions/runs/35068912844).
Both include source/race/browser, packaged restart/restore and AMD64/ARM64 API/web
builds. Nothing was published or deployed.

Contracts: [Open Library authors](https://openlibrary.org/dev/docs/api/authors),
[Hardcover search](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/guides/Searching.mdx),
[authors](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/GraphQL/Schemas/Authors.mdx)
and [book contributions](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/GraphQL/Schemas/Contributions.mdx).


Automatic bibliography additions now reuse existing work/format tracking across
legacy synthetic and real edition identities. Existing removed/unmonitored/manual
state is not updated or resurrected. Per-work creation locks and canonical-work
locks serialize these decisions; repeat sync reports existing items as skipped,
not newly created. Database regressions cover a retained removed/manual record
and recovery from a failed second insertion without duplicating the first. Rich
metadata refresh remains separate from this add-only monitoring pass.

Final qualification: full Go race/Postgres suite and vet passed, including
concurrent automatic adds converging to one tracked work. The web production
build and 39 applicable desktop/mobile browser checks passed (one desktop-only
case skipped on mobile). The final local ARM64 schema-40 API passed the packaged
import/acquisition/scan/replacement/restart/authentication regressions; a
405,045-byte dump restored with counts and receipt intact. Packaged web remains
the older multipart snapshot; current web code was built/tested from source.
The opt-in Open Library probe was actually run; default tests explicitly skip
that live probe. No production deployment or release occurred.


## Continuation: shared provider request budget (S12)

Branch: `codex/provider-request-budget`, based on author bibliography PR #16. A
shared transport now coordinates metadata lookups, author pages, checks and
Hardcover import lists, with one-second per-host request spacing. It honors 429
Retry-After, exhausted named RateLimit buckets, and legacy quota headers as
fallback. Daily/burst backoff cannot be bypassed through the other client. Budgets
are bounded to known hosts, contain no credentials, and reset with the process.

Requests refused before IO cannot advance provider checked/success timestamps or
invent reachability/authentication. Health reports the shared retry time. Unexpired
cached searches remain usable during quota backoff, while credential/shape/outage
failures still invalidate them. Hardcover list credentials normalize a Bearer
prefix; list errors omit raw provider messages/URLs and missing list data fails
explicitly. Rich list pagination remains separate work.

Transport/adapter fixtures cover concurrency, spacing, host isolation, daily and
burst headers, malformed/legacy headers, expiry, canceled waits, shared clients,
cached results, and actual-vs-unsent health evidence. The live read-only Open
Library probe again retrieved 418 works in six requests, now through the exact
production pacing transport with measured spacing and no catalog mutations.

Author bibliography PR #16 at `9936547adc1eee0cf8aa15cd2ac5e9842fd7959e` passed
[CI run 35071061527](https://github.com/bandoracer/librarry/actions/runs/35071061527),
including source/race/browser, packaged restart/restore and AMD64/ARM64 builds.
No images were published or deployed.

Final request-budget qualification: full Go race/Postgres suite, vet and web
production build passed. Browser checks passed 39 applicable cases (one
desktop-only case skipped on mobile). The local schema-40 API image passed all
packaged restart/import/acquisition/scan/replacement/authentication checks; its
404,409-byte dump restored with file/download/wanted counts and receipt intact.
The packaged web image remains the older multipart snapshot; the current web
source was built and browser-tested separately. No production changes occurred.


## Continuation: complete Hardcover list traversal (S12/S13)

Branch: `codex/complete-import-lists`, based on request-budget PR #17. Hardcover
lists formerly returned only the first 200 rows and silently skipped malformed
books. The new fetch verifies list visibility and traverses stable membership IDs
through an empty terminal page, checking declared counts and modification evidence.
Incomplete/changed/invalid/oversized traversals fail before wanted mutations.
Repeated membership of one book deduplicates after membership completeness checks.
This is bounded traversal, not an atomic provider snapshot; live-token proof is
still unavailable.

List sync now preserves existing work/format tracking across legacy edition IDs,
including removed/manual state. New root/monitoring defaults commit with creation,
closing the second-write window. All exclusions are loaded (the former 1,000-row
cap could ignore older exclusions). Failed additions or timestamp persistence are
visible and cannot advance a success timestamp. Scheduled errored outcomes fail
the task; retries reuse completed additions. Search-on-add remains best effort
and durable follow-up delivery is still outstanding.

Request-budget PR #17 at `4cd05bb2fbbf4746ef4f0d47427a4c71ecb3788b` passed
[CI run 35072580313](https://github.com/bandoracer/librarry/actions/runs/35072580313),
including source/race/browser checks, packaged qualification and both architecture
builds. No images were published or deployed.

Final list qualification: full Go race/Postgres suite and vet passed, including
complete traversal, database failure/retry, concurrent adds and scheduler failure
reporting. Web production build and 39 applicable desktop/mobile browser cases
passed (one desktop-only case skipped on mobile). The schema-40 local ARM64 API
passed the packaged restart/import/acquisition/scan/replacement/authentication
checks; a 404,518-byte dump restored with counts and receipt intact. The packaged
web remains the older multipart snapshot, with current web source separately
built and browser-tested. No live Hardcover request, production change or release
occurred.


## Continuation: Hardcover edition evidence (S12/S13)

Branch: `codex/hardcover-edition-metadata`, based on complete-list PR #18. Book
searches previously exposed thin work hits with synthetic name-based authors and
no real edition identity. Search now batches work details and separate default
ebook/audio editions; ISBN lookup queries exact editions, checks returned ISBNs
and parent-work identity, and supports equivalent ISBN-10/13. Original publication
and edition release dates remain separate. Stable author/credit IDs, publisher,
language, cover, pages, audio duration and narrator evidence survive normalization.
A default relationship cannot invent an ebook/audio format; unsupported/missing
defaults return work-level unknown evidence. Broader edition/language enumeration,
series discovery and original raw provider snapshots remain open.

Merge checks reject conflicting formats/languages, distinct edition IDs within a
provider and nonoverlapping ISBN evidence. Every cluster member must agree,
preventing an unknown work from bridging incompatible editions. Contributor roles
and distinct same-name provider identities survive. Concrete edition persistence
no longer aliases every edition to one work/format placeholder; existing legacy
aliases are not rewritten. Add-only monitoring still preserves prior tracking.

Contract and database tests cover the new evidence, failures and identity rules.
Hardcover queries remain fixture-qualified because no live token is available.

Complete-list PR #18 at `85ebf93923dc7e4ea4dcd50fcc4474b001755c91` passed
[CI run 35073516756](https://github.com/bandoracer/librarry/actions/runs/35073516756),
including source/race/browser, packaged restart/restore and AMD64/ARM64 builds.
No images were published or deployed.

The Add New flow now also keeps a displayed ebook's known format when the user
changes the filter while another search is pending; a tracked audiobook cannot
masquerade as that ebook. Unknown-format results still use the requested target.
A regression covers this transition.

Final edition qualification: full Go race/Postgres suite and vet passed. All seven
web unit checks, web production build and 39 applicable desktop/mobile browser
cases passed (one desktop-only case skipped on mobile). The final schema-40 local
ARM64 API passed packaged restart/import/acquisition/scan/replacement/authentication
checks; its 405,178-byte dump restored with counts and receipt intact. Packaged web
remains the older multipart snapshot; current web source was separately built and
browser-tested. No live Hardcover request, production deployment or release.

## Continuation: author monitoring policies and exclusions (S13)

Branch: `codex/author-monitor-policies`, based on edition-evidence PR #19.
Latest monitoring formerly chose an unreleased title as the absolute newest
work, omitting the most recent published book. It now selects the latest
released work separately. First/latest require provable publication ordering;
missing or overlapping dates enter review rather than using provider order.
Date-only evidence retains day/month/year precision. Original work dates/years
win over reprint dates, and a coarse date spanning the subscription cutoff
cannot invent a future-publication classification.

Ignored reviews follow stable work aliases within the author subscription and
format, even if policy, preferred edition or title changes. Global saved book
exclusions are loaded without the former list limit and apply before first/latest
selection. Language allowlists compare canonical names/codes, never arbitrary
prefixes. Existing tracking, manual corrections and removed books remain intact
under repeated add-only passes.

Author settings, referenced metadata-profile filters and exclusions are read after
provider IO. Stops/removals during the fetch skip additions without marking the
author synced; changed policies and creation defaults take effect. Sync success
requires the same still-active subscription revision and a successful database
write. This closes the long remote-IO window, but does not serialize all owner
edits with every insertion in the later candidate loop. Complete root/profile
inheritance, verified file-presence semantics and broader matching corpus remain
open S13/S14 work. No new database migration is required.

Edition-evidence PR #19 at `8a7db4b631e9bf620085133e0c94cf853f7f8724` passed
[CI run 35075050971](https://github.com/bandoracer/librarry/actions/runs/35075050971),
including source/race/browser, packaged restart/restore and AMD64/ARM64 builds.
No images were published or deployed.

Final policy qualification: full Go race/Postgres suite, vet, web production build
and whitespace checks passed. Contract/database tests cover all seven policies,
repeated refreshes, 1,100 exclusions, changed edition/work aliases, language
normalization, imprecise dates, stops/removals/policy/filter/default changes during
provider fetch and stale sync timestamps. The schema-40 local ARM64 API passed
packaged restart/import/acquisition/scan/replacement/authentication checks; a
404,854-byte dump restored with counts and receipt intact. Packaged web remains
the older multipart snapshot; this change has no web source edits. No live provider
request, production deployment or release occurred.

## Continuation: native author destination inheritance (S13)

Branch: `codex/author-destination-inheritance`, based on author-policy PR #20.
Native author subscriptions previously had no root-folder field, and Add New
submitted the standard quality profile regardless of its form. Migration 0041
adds optional roots to subscriptions and review candidates. Roots must exist and
match the format; validation and subscription writes share a transaction/row
lock. New wanted additions inherit author root/profile/tags together. Existing
tracking retains its settings. Pending review decisions use the root from their
latest evaluation, which is displayed alongside the quality profile.

Root updates distinguish omission from explicit clearing. Root deletion clears
references consistently with existing wanted-book behavior. Existing records are
left unassigned rather than guessing historical destinations. No earlier
migration was modified. Readarr root-path/ID mapping remains S16 work.

Add New now passes selected creation defaults and respects audiobook targets
when provider author records have unknown edition format. Its former Refresh
Author button merely re-saved form values; it now runs a targeted refresh using
the saved subscription. Save success followed by refresh failure retains visible
tracking and reports the failure. Stable IDs take precedence over same-name
selection. Library author settings can change/clear root and quality defaults for
future additions; metadata-profile precedence copy now matches actual behavior.

Author-policy PR #20 at `d984f4b7e9d1256f38ca8d5ddbec259a4c48c5ad` passed
[CI run 35076757236](https://github.com/bandoracer/librarry/actions/runs/35076757236),
including source/race/browser, packaged restart/restore and AMD64/ARM64 builds.
No images were published or deployed.

Final destination qualification: full Go race/Postgres suite, vet, seven web unit
checks and the production web build passed. All 43 applicable desktop/mobile
browser checks passed (one desktop-only case skipped on mobile), including four
new author destination/save-failure/refresh cases. Desktop and mobile form
screenshots were inspected. Newly built local ARM64 API and web images passed
schema-41 packaged restart/import/acquisition/scan/replacement/authentication
qualification. Selected author root/profile/tags survived an API restart; the
406,472-byte backup restored with author destinations and existing receipt/file
evidence intact. Author monitoring was disabled in that smoke fixture, so no real
provider or acquisition request occurred. No production deployment or release.
