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

## Continuation: direct author details and identity paging (S14/S15)

Branch: `codex/direct-author-details`, based on destination PR #21. Author pages
previously resolved from capped subscription/book lists and grouped by display
name. They now request a direct subscription/canonical identity or resolve a
legacy name key into explicit choices. Identified people sharing a name are never
combined by this lookup. Manual author-name overrides suppress inherited identity
links and enter a clearly labeled name-only group. Unknown/non-Latin legacy name
keys retain their historical normalization behavior.

Book membership includes imported/unmonitored rows and excludes removed/ignored
rows. Pages contain at most 100 books with lowercase-title/UUID ordering and an
author-bound cursor. Count, page membership, manual overrides and recorded writer
links share a database snapshot. Subsequent pages reflect later edits; there is
no claim of a frozen multi-request snapshot. A 10,001-book fixture traverses 101
pages with title ties/non-ASCII titles and no stable-data gaps/duplicates. A
subscription beyond the former 500-row cap resolves directly. Legacy name choices
are bounded and explicitly report truncation instead of silently picking an ID.

New wanted writes persist all supplied contributors/roles and serialize author
alias resolution across concurrent books. Writer/coauthor relations contribute to
author pages; narrator-only roles do not. Existing omitted coauthors, conflated
identities and incorrect legacy roles remain explicit repair work. No schema
migration or guessed relationship backfill is included.

Wanted payloads now carry recorded writer IDs, which book/subscription links use.
Detail pages distinguish outage from missing records, show total books separately
from page status counts, and scope Search Page to the displayed books. Override
and writer-link hydration uses batch reads rather than one query per book.
Global library counts/caps, complete verified-presence semantics, live NAS checks
and broader S15 performance qualification remain open. Derived-state annotation
still uses the existing service and may query the configured download client.

Destination PR #21 at `3710690e57e82c7cfda707d28eed1cd645e69038` passed
[CI run 35078169189](https://github.com/bandoracer/librarry/actions/runs/35078169189),
including source/race/browser, packaged restart/restore and AMD64/ARM64 builds.
No images were published or deployed.

Final author-detail qualification: the full Go race/Postgres suite, vet, nine web
unit checks and the production web build passed. All 47 applicable desktop/mobile
browser checks passed (one desktop-only case skipped on mobile). The strengthened
author fixture reaches the imported book on page three of 201 books without
reading capped global collections. Mobile paging and desktop identity-choice
screenshots were inspected. Newly built local ARM64 API and web images passed
packaged restart/import/acquisition/scan/replacement/authentication qualification;
direct author links retained imported books after restart. The 406,610-byte dump
restored successfully at schema 41. No real provider or acquisition request,
production deployment, release or image publication occurred.


## Continuation: native book presence evidence (S14/S15)

Branch: `codex/verified-book-presence`, based on direct-author PR #22. The previous
annotation counted any linked file, including scan-confirmed missing media. It
also queried the capped global cutoff list per page and used a download method
that could silently substitute stored rows after empty/failed client responses.

Native annotation now batches only the page's file/link/manifest evidence in one
snapshot and loads quality profiles once. Ebooks require positive observations;
audiobooks require a complete committed per-book media set with matching IDs,
links, format, hashes and sizes. Partial chapter loss is incomplete; legacy or
conflicting evidence is unknown. A complete alternate import may satisfy the
book. Proven renames preserve identity/content and historical manifests. Pending
publication prevents certainty. Sidecars remain part of cleanup verification,
separate from playable media presence. No schema change or guessed backfill.

A separate live-only acquisition read labels fresh/partial/unavailable/notConfigured
responses and never returns stored-only downloads. Saved bookkeeping can exclude
an already-imported source from downloading. Native annotation uses a five-second
remote budget; failures stay explicit. File-database outages become unknown
instead of invoking the UI's legacy inference. The native cutoff list now requires
positive file evidence, while retaining its existing collection cap. Library,
author and book UI retain incomplete/unknown states; book details explain evidence.

Known boundaries: file evidence is the last recorded observation, not a page-load
filesystem probe or live mount-health guarantee. Worker/author policies, global
collection paging/counts, Readarr parity, historical manifest repair and broader
latency qualification remain open. General Activity download fallback is unchanged.

Direct-author PR #22 at `3876da31ca6dd764947fe9ccea173f207dc7b208` passed
[CI run 35080699203](https://github.com/bandoracer/librarry/actions/runs/35080699203),
including source/race/browser, packaged restart/restore and AMD64/ARM64 builds.

Presence qualification: the full Go race/Postgres suite and vet passed; the
additional already-imported-source regression passed under race instrumentation.
Ten web unit checks and the production build passed. All 49 applicable desktop/
mobile browser checks passed (one desktop-only case skipped on mobile); status
screenshots were inspected at both widths. Newly built local ARM64 images passed
packaged qualification at schema 41. A real fixture audiobook moved from complete
to incomplete after chapter deletion and scan, then returned to complete after
restoration and scan. The 406,904-byte backup restored successfully. All client
requests targeted isolated fixtures; no real acquisition/provider request,
production deployment, release, live NAS qualification or image publication.


Hosted presence qualification initially failed on Linux because the runner user
could not unlink an API-owned chapter directory. The chapter-loss fixture now
removes/restores its controlled file inside the disposable container, preserving
normal application ownership. The corrected local package run passed, including
a 406,432-byte restore; hosted qualification is pending on the follow-up commit.
The first local retry from `/tmp` failed because that directory is not shared with
Colima; moving the isolated checkout under the shared home directory resolved the
fixture mount. Neither failure changed production state.


## Continuation: scheduled evidence and fair checks (S14/S15)

Branch: `codex/worker-evidence-fairness`, based on presence PR #23 and its Linux
fixture ownership correction. Monitoring previously excluded every imported book
and every linked file, leaving known file loss unrecoverable. Skips and provider
failures could repeatedly occupy the first batch. Feed matching selected only the
latest 200 wanted rows and could include unmonitored entries.

Migration 0042 adds independent check/attempt clocks and ordering indexes. Monitor,
upgrade and author retries advance after skips/errors without claiming successful
search/sync or changing owner revisions. Stable UUID tie-breakers support fair
ordering. Backoff is capped at 15 minutes; successful activity keeps the requested
interval, and Force permits a deliberate immediate retry. The existing author
persistence-failure regression was expanded to assert no false sync, scheduled
backoff, and an idempotent forced retry after fixing the injected failure.

Monitor/upgrade checks use native media and live-client evidence. Known imported
file loss can be searched; partial audio belongs to recovery, not upgrade search.
Unknown/client outage evidence blocks automatic work with a reason. Automatic
acquisition rechecks after provider IO; owner field changes invalidate stale
search evaluation. The acquisition transaction locks the wanted row and rejects
unmonitored automatic requests. Manual owner grabs remain supported. Existing
acquisition reservations protect duplicate client adds; this is not a renewable
worker lease and does not eliminate overlapping provider reads.

Feeds traverse all monitored candidates in bounded UUID pages, load per-book
settings after a title candidate matches, and cap returned detail at 1,000 with
explicit truncation and full counters. Book decisions still persist. Native
Wanted now wires the Incomplete/Unknown helpers to actual routes; the prior helper
addition alone did not expose those tabs. Batch buttons now state their limited
scope and toasts explain skips. Global list counts/caps, all-matching bulk jobs,
author-file-policy adoption, compatibility and durable per-item skip history
remain open. No complete S14/S15 claim.

Feed release observations persist decisions without updating the full indexer
search timestamp, so repeated RSS matches cannot postpone due searches.

Full-search decision persistence also locks/checks the book revision captured
before provider IO. A changed revision rejects stale decisions and does not
advance the successful-search timestamp.


Presence PR #23 at `15abd047f80336f742972dae78120fb5d67d3ba9` passed
[CI run 35083013552](https://github.com/bandoracer/librarry/actions/runs/35083013552),
including source/race/browser, corrected Linux packaged restart/restore and
AMD64/ARM64 builds. This supersedes the initial fixture-permission failure.
No images were published or deployed.


Final worker qualification: full Go race/Postgres suite and vet passed. The tests
traverse 10,001 tied records through monitor and upgrade ordering without repeat
batches, reach older imported books during paged feed matching, advance past 205
failed authors, recover incomplete audio instead of upgrading it, preserve manual
grab behavior, and reject stale book revisions before saving search success. The
owner-edit fixture now uses the actual settings write path so it advances the
same revision as the app. Ten web unit checks, production build and 51 applicable
desktop/mobile browser cases passed (one desktop-only mobile skip). Wanted status
and skip-toast screenshots were inspected at both widths.

Latest local ARM64 API/web images passed packaged qualification at schema 42.
Scheduling progress survived restart and the next monitor batch advanced; the
408,901-byte backup restored with file/book/receipt evidence intact. No real
indexer/provider/acquisition request, production deployment, release or image
publication occurred. The broader plan remains active.

Next collection-contract work should also fix the selected-upgrade UI's inherited
50-item default: explicit IDs currently constrain membership but the request does
not raise its limit to cover a larger selection. Global profile/restriction edits
do not share the per-book revision fence and need broader decision/configuration
validation. Neither limitation is certified complete by these worker tests.


Worker fairness PR #24 at `b5533c8725888adb9538a4b6dda6590f56d281cc` passed
[CI run 35084318036](https://github.com/bandoracer/librarry/actions/runs/35084318036),
including source/race/browser, packaged restart/restore and AMD64/ARM64 builds.
No images were published or deployed.

## Continuation: exact upgrade selections (S15)

Branch: `codex/explicit-upgrade-selection`, based on PR #24. Explicit selections
previously inherited the 50-book scheduled batch limit, and malformed JSON was
silently decoded as a default batch. Nonempty wantedIds now select the entire
request (maximum 200), independently of the queue limit. UUIDs normalize and
deduplicate while preserving order. Unmonitored, removed, ignored and not-yet-due
books return per-item skip reasons. Force bypasses timing only. Missing IDs reject
the whole selection before a run starts; later owner changes retain existing
per-book checks. Invalid JSON, unknown fields, invalid IDs, oversized requests and
invalid limits return 400 without starting work. An empty selection retains the
bounded queue action. The web client sends the selection size and displays the
server's explanation when validation fails.

Qualification: the full Go race/Postgres suite and vet passed; the 200-selected
plus one unselected regression proves complete processing and exact scope. Other
cases cover duplicates, stopped/recent books, Force, deleted/invalid IDs, bad JSON
and unchanged scheduling clocks on rejected work. Twelve web unit checks and the
production build passed. All 53 applicable browser cases passed, with one expected
mobile skip; the 75-row upgrade selection passed at 1440x1000 and 390x844, and the
mobile screenshot was inspected. Global collection paging/counts, durable
all-matching jobs and live qualification remain open. No complete S15 claim.

The local ARM64 API/web images also passed packaged restart, scan, evidence,
recovery and authentication qualification at schema 42. A 408,967-byte database
backup restored with book/file/download/receipt evidence intact. This is isolated
fixture qualification, with no production deployment, publication or real grab.


## Continuation: shared collection file projection (S14/S15)

Branch: `codex/collection-file-projection`, based on explicit selection PR #25.
Migration 0043 moves the existing file-evidence contract into a shared, stable SQL
function. Native per-book, detail and worker reads now use that function. SQL
collection readers can apply the same evidence before filtering/counting/paging,
without a second implementation of audiobook completeness. NULL is an explicit
whole-collection scope; empty arrays remain empty. No stored file, link or
manifest data is rewritten. This does not yet expose global paginated browsing.

Focused qualification passed the existing file-evidence cases and a populated
schema-42 upgrade retaining a partial audiobook's manifest/file identities. A
10,001-ebook fixture reports 3,333 present, 3,334 missing and 3,334 unknown entries
and traverses all present entries in stable UUID pages without duplicates/gaps.
The initial scale fixture used undashed IDs in legacy JSON and therefore did not
create relational links; correcting the fixture to use canonical UUIDs fixed that
setup error without changing migration 0043. Database-only page p95 was 46.028 ms
on Apple M5 Max/ARM64, Colima Postgres 16.15. This is not full API or provider
latency, nor broad mixed-media performance certification. Global derived-state
filters/counts/sorting, native collection routes and UI adoption remain next.

Final local projection qualification: full Go race/Postgres suite and vet passed.
All 53 applicable browser cases passed against the new backend (one expected
mobile skip). The local ARM64 API with the unchanged current web image passed
packaged scan, presence, recovery, restart, authentication and restore checks at
schema 43; a 413,352-byte backup restored with file/book/download/receipt evidence
intact. No production rollout, real grab, image publication or release occurred.


Explicit upgrade selection PR #25 at `4cfa394b399c4cc63e21d8fbf6e24923b3109dd2`
passed [CI 35085446775](https://github.com/bandoracer/librarry/actions/runs/35085446775).
Shared projection PR #26 at `5c410b69f1cfb0a5b83ed4405b65a5aaa111f3fc` passed
[CI 35086016041](https://github.com/bandoracer/librarry/actions/runs/35086016041).
Both include source/race/browser, packaged restart/restore and AMD64/ARM64 builds.
No images were published or deployed.

## Continuation: native book collection paging (S14/S15)

Branch: `codex/paged-book-collection`, based on PR #26. The new
`GET /api/v1/library/books` evaluates tracked membership, global state counts,
filtered totals and bounded pages in a repeatable-read snapshot. Profiles and
manual overrides/author links use the same snapshot. It captures one live-client
observation per request, retains uncertainty, and normalizes installed quality
using the detail contract. Removed/ignored books are excluded; imported and
unmonitored books remain reachable. Four sorts use unique ID ties and cursors
bound to filters/sort. Each request is consistent, not a frozen multi-page session.

Library books and Wanted Missing/Incomplete/Unknown/Cutoff Unmet now consume this
endpoint with actual Previous/Next controls and global counters. Text filters,
format/monitor filters and sorting run on the server. Current-page selections
clear on navigation/filter changes; errors offer retry rather than claiming empty
success. Demo builds retain explicit seeded fallback. Metadata review's count is
labelled loaded, because that collection remains separate work. The inherited
Library Update All label was corrected to Check Author Batch for its 50-author
request. No all-matching bulk job is implied.

The initial scale run exposed real planner costs. JIT compilation took about
525 ms before useful work, and a late-page plan compared 100,030,002 rejected join
pairs (~5.95 s). Limiting before hydration still left 6,931,386 rejected pairs
(~398 ms). Materializing derived states before cursor filtering reduced that plan
to ~35.6 ms. Read transactions disable JIT and force custom plans locally; pooled
sessions/database settings are unchanged. Diagnostic runs using obsolete slow
plans were explicitly canceled after those plans were measured; the final scale
run passed 404 pages across all four sorts. Its 10,001 active books / 10,003 file
records returned exact counts with no duplicates/gaps at 61.400 ms local service
p95 on Apple M5 Max/ARM64, Colima Postgres 16.15, excluding external client IO.

Validation also covers full filtered subsets, invalid parameters/cursor reuse,
empty arrays, manual corrections, partial/unknown media, client outages, custom
profiles and saved/legacy scores. Full Go race/Postgres checks and vet passed;
the additional installed-quality regression passed with races. Fourteen web unit
checks and the production build passed. All 59 applicable desktop/mobile browser
cases passed (one expected mobile skip); both new views were inspected at 390x844,
with selection/paging/error recovery also exercised at 1440x1000.

The final local ARM64 API/web pair passed packaged qualification at schema 43:
complete cursor traversal, exact counts, incomplete-audio filtering and cursor
continuation after API restart. The 413,062-byte backup restored with native file
projection, records and receipts matching the source. No real provider/indexer
grab, production rollout, publication, tag or release occurred. Author collections,
metadata review, file/legacy readers, compatibility and durable all-matching bulk
jobs remain open; S14/S15 and the overall plan are not complete.

The final `go test ./...` run also passed after the installed-quality fixture was
added. The next audited gap is AuthorsTab: its 500-subscription response and
counts derived from capped wanted/file/review lists still need native collection
membership, accurate statistics and pagination.


Book paging PR #27 at `e468d4bb9ee9cced97e4cc21e01e2bda0a6837be` passed
[CI 35088661629](https://github.com/bandoracer/librarry/actions/runs/35088661629),
including source/race/browser, packaged restore and platform image gates.

## Continuation: native author subscription collection (S14/S15)

Branch `codex/paged-author-subscriptions` follows PR #27. AuthorsTab previously
stopped at 500 subscriptions and calculated book counts from capped wanted/file
arrays by matching names. The new `/api/v1/library/authors` pages subscriptions
with name/provider, format and status filters. Full book counts use unique
recorded provider identity, writer membership, subscription format and the native
book evidence projection. Same-name people stay separate; manual author overrides
win; duplicate writer roles count once, narrator-only and removed books count
zero. Unresolved or conflicting identities remain explicit. Settings and counts
share a database snapshot and one client observation. Cursors bind filters and
survive restart without claiming a frozen traversal.

The UI replaces the capped source arrays, adds Previous/Next and retry states,
keeps row actions targeted, and labels the default 50-author monitor work as a
batch. Desktop/mobile coverage verifies older pages, global filters, exact refresh
IDs, settings persistence and errors distinct from empty results. Mobile toolbar
spacing and card padding were also corrected. Subscription rows retain separate
provider/format settings; this does not introduce an all-writers catalog or
complete author metadata-review pagination.

The scale fixture traversed 1,002 subscriptions with 10,001 tracked books and no
gaps/duplicates. The first non-race run's slowest local page was 343.047 ms; the
final focused race run measured 412.793 ms (Apple M5 Max/ARM64, Colima Postgres
16.15; no external client IO). The added contracts cover same-name identities,
manual overrides, duplicate writer roles, narrator-only links, format separation,
removed books, ambiguous provider records and live-client outage states.


Full Go race/Postgres checks and vet passed, as did the focused race run after
adding narrator/duplicate-role/removed-book coverage. Fourteen web unit checks,
the production build and all 63 applicable desktop/mobile browser cases passed
(one expected mobile skip). The eight author paging/settings cases also passed
after the mobile spacing correction; the 390x844 capture was visually inspected.


The local ARM64 API/web pair (`librarry-api:paged-authors` and
`librarry-web:paged-authors`) passed schema-43 packaged qualification. The new
checks verify linked/unlinked identity counts, complete subscription traversal,
and cursor continuation after process restart. The 413,054-byte database backup
restored with author settings, book/file/download records, native evidence and
receipts intact. No real provider request, acquisition, deployment, image
publication, tag or release occurred. Author review, all-writers catalog discovery,
file/legacy collections, compatibility and durable all-matching bulk jobs remain
open. S14/S15 and the full stabilization goal remain in progress.

The final `go test ./...` run also passed. Next audited gap: the metadata Review
queue calls the 200-row `ListWanted` reader, performs per-book provenance reads,
and explicitly skips imported books. Selected canonical-confirmation also depends
on that truncated queue. These remain the next S14/S15 continuation.


Author subscription PR #28 at `57a12a28e500b413b530e303abae089b47a66fb0` passed
[CI 35090148378](https://github.com/bandoracer/librarry/actions/runs/35090148378),
including source/race/browser, packaged restore and platform image gates.


## Continuation: native metadata Review and atomic confirmation (S14/S15)

Branch `codex/paged-metadata-review` follows PR #28. Review previously inspected
only 200 wanted rows, skipped imported books and made per-book provenance queries.
Selected confirmation used that truncated queue and could partially commit a
multi-book request. The native endpoint now counts and pages every active tracked
book with unresolved metadata, including imported/unmonitored books. Removed and
ignored books stay excluded. Full text/format filtering, stable title/author/UUID
keys, cursor binding, global totals and explicit read errors replace loaded-list
inference. Direct provenance and Review share a repeatable-read batched reader.

The reader projects all active metadata per request. It has no per-book database
or provider round trips, but processing/memory still grow with library size; only
the returned page is bounded. The final local scale fixture has 10,001 active books
with distinct work/provider records, and traverses 101 pages without gaps or
duplicates at 208.342 ms p95 (Apple M5 Max/ARM64, Colima Postgres 16.15; no external
provider IO). Revisions are computed only for the page/lookahead rather than for
every counted book. Unicode comparison preserves letters/marks/numbers and NFC
composition; differing CJK titles no longer collapse to the same empty ASCII key.
Provider edition format remains visible evidence without contradicting an owner's
explicit ebook/audiobook acquisition target.

Keep current validates every selected UUID independently of the collection page,
locks books in UUID order, and commits all accepted fields together. Native UI
requests bind the displayed field evidence via revisions; stale evidence returns
409. Existing overrides change only their acceptance reason. An initial upsert
implementation reproduced a race that resurrected a concurrently cleared override;
separate UPDATE/INSERT paths fixed it. Native corrections and clears now take the
book lock before override changes to avoid inverse lock ordering. Tests prove both
native serialized owner actions and noncooperating older update/delete writers:
new owner values or clears survive, while stale concurrent confirmation aborts.
A controlled late write failure rolls back all earlier confirmations. Missing IDs
reject the entire selection. A 200-book older selection and a 501-book all request
prove the old listing cap no longer limits mutation membership.

The native UI adds Review filtering/paging, global counters, current-page scope,
retryable failure states and explicit skipped-book feedback. Selection clears on
page/filter/tab changes, evidence revisions travel with confirmation, and mobile
navigation keeps the active Review tab in horizontal view without moving the
whole page vertically. The separate author metadata review queue remains capped;
file/legacy readers, search badges, compatibility, removed-item browsing and
resumable all-matching bulk jobs remain open. Legacy API `all: true` is synchronous
and atomic, not a durable bulk job.

Validation before final image qualification: full Go race/Postgres suite passed;
final revision/confirmation/owner-lock contracts passed with races. The 14 web unit
checks, production build and all 67 applicable desktop/mobile browser cases passed
(one expected mobile skip); the ten Review/book paging cases passed again after
mobile navigation/busy-state fixes. The initial browser failures exposed an
initial debounce reset and an over-exact test message expectation; both were
corrected. The initial stale-revision fixture changed zero confidence to zero and
was corrected to actually change the record; the final stale-revision regression
passes. Mobile Review was visually inspected at 390x844.


The final `go test ./...` run with Postgres and vet passed after the owner lock
ordering change, as did focused race checks for native and noncooperating
update/clear races. The final local ARM64 API/web pair (`librarry-api:paged-review`
and `librarry-web:paged-review`) passed schema-43 packaged qualification: imported
conflicts, exact counts, complete cursor traversal, cursor continuation after
restart, selected confirmation preserving other overrides, authentication, and a
413,546-byte isolated database restore with settings/evidence/receipts intact.
Earlier package runs and the pre-fix clear-race reproduction are superseded by
these final checks. No real provider request, acquisition, production deployment,
image publication, tag or release occurred. The full stabilization goal remains
active; S14/S15 are not complete. Next audited collection gap: AuthorsTab's author
review panel fetches the capped candidate list and displays only six entries.


## Author candidate review paging and atomic decisions (2026-09-16)

Continued S14/S15 on `codex/paged-author-review`, stacked on metadata Review PR
#29. That preceding PR's CI completed successfully in run 35093172787.
The separate AuthorsTab queue fetched a capped list and displayed only six rows
with no continuation. It now uses a native filtered collection with six visible
rows per page and every stored candidate reachable. Pending/wanted/ignored/all
status, format and literal search apply globally; response counts and bounded
page records share a repeatable-read snapshot. Timestamp/UUID cursors bind filters
and survive process restart. API limits are 1–100; invalid or duplicate query
parameters return 400, unavailable persistence 503, and empty arrays stay arrays.

The previous resolution path created a book and saved the decision in separate
transactions, followed by best-effort history. A row lock now serializes the
whole operation. Wanted creation/reuse, resolution and history commit together;
a controlled history failure proves rollback of the book and decision. Displayed
candidate revisions protect against changed settings/evidence. Same-action
retries replay the saved receipt; competing different actions return 409. Existing
tracked work/format records preserve destination, profile, tags, status and
monitoring, including removed books; UI feedback explicitly says settings were
retained. New books inherit the candidate's saved settings. No provider lookup,
indexer search or acquisition is part of resolution.

A 10,001-candidate fixture traversed 101 pages with no gaps/duplicates and local
p95 6.934 ms (Apple M5 Max ARM64, Colima Postgres 16.15). Additional database tests
cover filter binding, stale revisions, late history rollback, captured defaults,
existing-owner choices, opposing concurrent decisions and same-action retries.
Full Go race/Postgres checks passed. API tests cover strict JSON, unknown fields,
oversized bodies and paging input errors. Fourteen web unit checks and production
build passed; the complete browser suite passed 71 cases with one expected skip.
A focused rerun exposed an initial empty-search debounce resetting a just-opened
second page; subscription and candidate search effects now schedule resets only
when the search actually changes. All eight focused desktop/mobile author tests passed after
that correction. Mobile presentation was inspected at 390x844.

Local ARM64 `librarry-api:paged-author-review` and
`librarry-web:paged-author-review` passed schema-43 packaged qualification,
including seven candidates across two pages, cursor continuation after restart,
captured destination/profile/tags, same-action replay after another restart,
conflicting Ignore rejection and exactly one history event. The final 414,797-byte backup
restored with author review decisions and the prior file/book/import receipts
intact; auth/restart checks also passed. This final packaged run used the web
image rebuilt after the debounce correction; the earlier 414,916-byte restore
is superseded. The final `go test ./...` run with Postgres also passed. No migration, published image, tag, release or production
rollout occurred.

S14/S15 and the full stabilization goal remain active. Native file/legacy readers,
search badges and bounded dashboard summaries, compatibility, removed-book
browsing and resumable collection-wide jobs remain open. Broader S03/S09/S10/S12/
S13 and S16–S25 gates remain in the plan; live credentials/platform/soak work is
still separate from fixture qualification.

Next concrete collection gap: `BookPage` calls `useLibraryFiles`, which asks for
only 100 files; the store has a hard 500-row maximum and no cursor. Large chapter
sets can therefore be truncated even though native completeness evidence counts
the full manifest. Next work should add a bounded paged file contract and update
book/file readers without weakening pending-publication exclusion or book links.


## Full file collection and paged rename preview (2026-09-16)

Continued S14/S15 on `codex/paged-library-files`, stacked on author review PR #30.
PR #30 completed all CI jobs in run 35095085699. Imports previously read 100
files for its table and whole-library counters; book details fetched the same
bounded list for fallback state but showed no file table. Rename preview fetched
at most 500 files and had no continuation. These native consumers now use
`GET /api/v1/library/files/collection`.

The collection returns bounded files plus full scoped total/filtered counts and
presence/format/import counters. Search, format, recorded presence and three
stable sorts bind the cursor. Repeatable-read snapshots keep counts and rows
consistent per response. Metadata and relational book links load only after page
selection. Stale JSON hints do not replace `file_wanted_links`; multiple links
remain explicit. Unassigned files remain visible; every path claimed by an
uncommitted import operation stays excluded. Read failures are explicit, empty
arrays remain arrays, and listing never probes bytes or calls providers.

Book details now show every chapter through paging while retaining the native
whole-book completeness result. Imports uses full counts and suppresses stale
counts after refresh failures. The shared table shows the filename first,
retains recorded title/path, and places presence near the filename for mobile.
Rename preview uses exact current-page file IDs, resets selection with page/filter
changes and explains that Apply affects only shown selections. Rename Files is
also available when no books are listed, so unassigned files remain accessible.
This changes browsing/selection, not the underlying rename durability contract.

A 10,001-file fixture with relational links traversed 303 pages across path,
title and updated sorts without gaps/duplicates; local p95 was 27.977 ms while
browser qualification also ran (Apple M5 Max/ARM64, Colima Postgres 16.15).
A single 1,500-chapter book traversed every scoped page. Additional tests cover
literal search, global counts, invalid/filter-mismatched cursors, stale JSON vs
relational membership, multiple links and every uncommitted operation state.
Full Go race/Postgres tests, vet, final `go test ./...`, 14 web unit checks and
production build passed. The complete browser suite passed 77 cases with one
expected skip. Six focused desktop/mobile file tests passed again after the final
presentation and stale-count changes. Initial browser failures exposed the hidden
rename action on an empty book list and unscoped status assertions matching new
filter options; the UI and targeted assertions were corrected. Mobile file
presentation was visually inspected at 390x844.

The local ARM64 API/web pair (`librarry-api:paged-files`,
`librarry-web:paged-files`) passed schema-43 packaged traversal, totals, relational
chapter membership, restart-cursor and authentication checks, plus isolated
backup/restore. The final rebuilt web image passed the full packaged script and
a 414,543-byte backup restored successfully, superseding the earlier 414,260-byte
run. No migration,
published image, release, real acquisition or production deployment occurred.
Legacy/compatible lists, Calibre batch fairness, search badges, dashboard counts,
removed-book browsing and durable all-matching jobs remain open. The stabilization
goal and S14/S15 are not complete.

Next safety gap found during source review: `applyRename` moves bytes before
`Store.UpdateFile` saves the new path. A DB failure between those steps has no
rename journal. Chapter rename destinations also use the general book template
and need file-set qualification. Next work should reproduce interrupted rename,
then provide durable recovery and preserve chapter/disc layout and associations;
current paging qualification does not certify those mutation guarantees.

## Durable standalone file renames (2026-09-16)

Continued S09 on `codex/durable-file-renames`, stacked on file paging PR #31.
The original implementation removed the source before persisting the new path.
A controlled `files` update failure reproduced the resulting missing original
and stale record. Renames now reuse the import staging/lease/cleanup machinery,
with migration 0044 reserving each active file identity and source path.

The visibility transaction changes the existing row's location and verified byte
evidence and inserts one history event. File IDs, names/notes, source/import
provenance, relational book/download links, monitoring and wanted lifecycle remain
intact. Sources stay until commit, then lease-fenced cleanup removes the old name.
Scans skip both sides of an unfinished rename; old sources remain reserved until
cleanup completes. New imports/direct writers cannot claim those reserved names.
Naming changes cannot redirect a saved recovery plan. Repeated A → B → A → B
renames get distinct operation identities without duplicating a retry.

Original immutable import receipts resolve their current destination only through
committed rename history for the same file ID, hash and size. Replay and download
cleanup still verify bytes, associations, source separation and client inventory.
An arbitrary path edit cannot pass as a verified relocation. A rename has no wanted
manifest assignment, so one selected chapter cannot manufacture complete-book
import evidence. Pending original import/replacement cleanup must finish first.

Native previews now carry revisions. Apply sends every selected revision and
rejects changed evidence; malformed/unknown/oversized JSON and multiple request
values are rejected. Legacy callers can still omit revision maps. Existing bytes
are never overwritten. Templates cannot change the file extension. Imports labels
saved renames and offers retry; preview failures and reasons for retained files
are visible. Known chapter/disc, linked audio and companion layouts are retained
by this per-file action. **Complete-set renaming and CUE/playlist reference
handling remain unfinished**, rather than flattening or splitting those layouts.

The prior file-paging PR's CI caught a loading race: replacing the empty-library
Rename toolbar with the populated toolbar could drop a click. Commit `f194cf8`
keeps the trigger mounted. A controlled delayed book response checks DOM identity,
focus trapping and focus restoration in six desktop/mobile runs. PR #31 was
updated with that fix; the first failing run was not rerun unchanged.

Database qualification covers failed path/history commits, cleanup failure after
commit, restart, scan suppression before/after commit, corrected relational links
alongside stale JSON, concurrent owner corrections and retries, changed bytes,
extension changes, collisions, repeated rename history, receipt replay and unsafe
path-edit rejection. Chapter/disc/companion fixtures retain their original paths.
The full Go/Postgres race suite and normal suite passed, as did vet, 14 web units
and the production build. The existing browser suite passed 77 cases with one
expected skip; four added desktop/mobile cases passed for retained layouts,
preview errors and saved-rename recovery. Final focused race reruns qualified
source reservation and indexed identity queries after review.

Local ARM64 images `librarry-api:durable-renames` and
`librarry-web:durable-renames` pass schema-44 packaged restart qualification:
a controlled rename commit failure retains the old path, a scan skips both names,
and API restart/retry preserves file identity and original import provenance.
Original import replay returns the verified new path. The same packaged suite
covers the earlier import, scan, acquisition, collection, authentication and
isolated restore contracts. Final rebuilt-image results are recorded below.

This is unreleased source and isolated-fixture qualification. No image publication,
tag, release, production deployment, real acquisition or live NAS restore occurred.
S09 and the full stabilization goal remain active: complete-set renames, changed
chapter-layout retirement, Calibre handoff recovery, broader disk-fault coverage
and live qualification are still required.

PR #31 CI run `35098515531` passed verification, packaged qualification and both
API/web builds on `f194cf87391c17fd028dfe2cb95f62c9ca5cac13`.

Final local qualification passed after the source-reservation and extension
checks: `go test ./...`, focused rename/import race tests, vet and diff checks.
The rebuilt schema-44 API/web pair passed the full packaged suite and restored a
417,058-byte backup with file/book/download counts and prior receipts preserved,
superseding the earlier 416,899- and 417,103-byte runs. Source and destination
scan suppression during a failed rename is included in this final package test.

## Complete recorded book folder renames (2026-09-16)

Continued S09 on `codex/book-set-renames`, stacked on durable rename PR #32.
PR #32 CI run `35100257329` passed all four jobs on
`e3ce314ae4361a5ea12d2a9b89aa128ba64ec23d`.

Book details now offer a separate **Rename book folder** preview and apply action.
The plan must account for every linked media file through a complete committed
native import, with matching current bytes and exclusive ownership. Naming and
root settings choose the destination folder; current basenames, chapter order,
disc directories and companion paths remain unchanged. This avoids splitting a
chapter set through per-file selection. Preview pages are presentation only:
Apply authorizes the complete captured set and requires its current revision.

Migration 0045 reserves every media identity and retains original sidecar manifest
IDs. A single visibility transaction updates paths/history on the existing rows,
then verified cleanup removes sources. Failed commits retain originals; saved
plans survive restart and settings changes. Interrupted cleanup resumes even when
some originals were already removed. Claims release only after all cleanup
finishes. Exact membership fences reject changed/foreign book links; no wanted
lifecycle or monitoring state is rewritten.

CUE/M3U/M3U8/local OPF references must remain inside the complete recorded set.
Absolute/escaping/missing references, unsupported encoding/size, unrecorded files,
symlinks, collisions, shared ownership and Calibre-managed files retain the folder
for review. General chapter renaming or companion rewriting is not implemented.
Original receipts now follow ordered verified scan moves as well as explicit
renames of the same file ID/hash/size. Arbitrary path edits or inconsistent move
history still fail; sidecars follow their original manifest identity. Historical
manifests stay immutable.

Fault fixtures cover full commit rollback, changed membership, partial cleanup
bookkeeping failure, repeated A/B folder moves, stale preview, scan reconciliation,
source/target inventory changes and ownership conflicts. A real 151-chapter import
and move preserves all chapter IDs/order and both companions beyond a display
page. Companion fixtures cover CUE directives, namespaced OPF, BOM/CRLF playlists,
Windows separators, traversal, missing members, malformed XML and invalid/large
text. Browser fixtures preview 1,501 chapters plus a cover over 16 pages, submit
the whole revision, recover a failed apply and retain unproven folders.

Visual inspection found the wide modal could extend offscreen on mobile despite
passing the document-width check. Shared grid sizing and wrapping footer controls
now constrain the dialog itself; browser checks assert its visible bounds. The
corrected 390px mobile preview was inspected. All 87 desktop/mobile cases passed
with one expected skip, along with 14 web units, production build and vet.

The schema-45 local ARM64 images `librarry-api:book-renames` and
`librarry-web:book-renames` passed the full packaged restart/recovery suite and
restored a 422,439-byte isolated backup, including sidecar identities and claims.
The new packaged case keeps a scan-renamed chapter, forces a whole-book commit
failure, confirms scan suppression, restarts, resumes the captured plan, preserves
monitoring and replays the original replacement receipt. An initial later fairness
fixture failed because the new case intentionally unmonitored its book; restoring
that fixture's prior setting after the rename assertions fixed the test isolation.
The older scan test's blanket rejection of moved-file receipts was updated to the
new verified-chain contract, adding mismatched-hash and arbitrary-path rejection.

No production deployment, image publication, real acquisition, live NAS restore
or unattended soak occurred. S09 and the full goal remain active: changed chapter
layout retirement, Calibre handoff recovery, broader disk faults and live
qualification remain outstanding. Final Go suite results follow below.

Final Postgres `go test -race ./...` and `go test ./...` passed after the verified
scan-receipt regression update. The added ownership/layout and 151-chapter checks
also passed a focused race run. Final vet and diff checks passed.
