# File integrity and recovery contracts

Implementation contracts for contributors. Operator instructions live in the [guides](../README.md#operate). These describe candidate source behavior, not complete compatibility or live qualification.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Library Import](#library-import)
- [Durable completed-import operations (unreleased)](#durable-completed-import-operations-unreleased)
- [Journaled native import staging](#journaled-native-import-staging)
- [Manual import and replacement recovery](#manual-import-and-replacement-recovery)
- [Persisted scan execution and local presence](#persisted-scan-execution-and-local-presence)
- [Library repair evidence](#library-repair-evidence)
- [Scan move identity boundary](#scan-move-identity-boundary)
- [Completed destination replacement](#completed-destination-replacement)
- [Durable standalone file renames](#durable-standalone-file-renames)
- [Complete recorded book folder renames](#complete-recorded-book-folder-renames)
- [Calibre protocol identities and authentication](#calibre-protocol-identities-and-authentication)

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
provided folder for supported ebook/audiobook candidates, and `POST /api/v1/manualimport` imports selected files through the regular library import
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

Readarr-compatible book, missing and cutoff-unmet reads use native book
membership and recorded presence evidence. An ebook needs present linked media;
an audiobook needs a complete committed manifest. Legacy metadata hints alone do
not establish complete media. See [compatibility book contracts](collections.md#complete-compatibility-book-reads-and-atomic-selected-edits).
The web UI uses `/api/v1/librarry/history`; external Arr clients use the
Readarr-shaped `/api/v1/history` response.

Completed-download import uses the client's exact finalized file inventory and
plans the complete media/sidecar set. Unknown inventory or uncertain book
identity requires review. Explicitly reviewed multi-book mappings bind each file
to its intended book. Saved plans, hashes, leases and transactional publication
make interrupted native imports recoverable, as detailed below.

Manual imports support `copy`, `move`, `hardlink` and `hardlinkOrCopy`.
Completed-download imports support `copy`, `hardlink` and `hardlinkOrCopy`,
preserving the client source until separately verified cleanup. Conflict policy
supports keep-both, replacement, skip and fail. Native file publication and
Calibre-server handoff have separate recovery contracts; a Calibre response does
not establish native filesystem rollback.

Default client categories are `books-ebook` and `books-audiobook`. Default roots
are `/data/media/books/ebooks`, `/data/media/books/audiobooks` and
`/data/torrents/books`. Keep download-client and Librarry paths aligned.

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

`GET /api/v1/library/import-recovery` returns independently paged operation,
Calibre handoff and legacy-issue collections with matching counts; see
[recovery collection contracts](collections.md#import-recovery-collection-observations). `POST /api/v1/library/import-operations/{id}/retry` resumes
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

## Journaled native import staging

Migration 0033 records each file's lease-specific staging path before transfer.
Native copies write exclusively to that path, check manifest size/hash, sync, and
renew the operation lease before exclusive publication. Recovery reloads the
journal after claiming the operation and reclaims only its recorded regular file
inside the destination parent. It never sweeps directories by prefix. Missing
stages are safe to retry; symlinks or forged journal paths require review.
Directory synchronization precedes clearing the journal. Imports displays pending
temporary paths alongside their manifest files. Unrecorded stages from older
versions and Calibre operations require separate operator investigation.

## Manual import and replacement recovery

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

## Persisted scan execution and local presence

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

## Library repair evidence

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

## Scan move identity boundary

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

## Completed destination replacement

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

## Durable standalone file renames

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

## Complete recorded book folder renames

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

## Calibre protocol identities and authentication

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
