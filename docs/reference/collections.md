# Collection and identity contracts

Implementation contracts for contributors. Operator instructions live in the [guides](../README.md#operate). These describe candidate source behavior, not complete compatibility or live qualification.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Direct native book lookup](#direct-native-book-lookup)
- [Native book presence evidence](#native-book-presence-evidence)
- [Shared recorded file evidence](#shared-recorded-file-evidence)
- [Native paginated book collection](#native-paginated-book-collection)
- [Native author subscription collection](#native-author-subscription-collection)
- [Native metadata Review collection and confirmation](#native-metadata-review-collection-and-confirmation)
- [Author candidate review collection and decisions](#author-candidate-review-collection-and-decisions)
- [Native file collection](#native-file-collection)
- [Import recovery collection observations](#import-recovery-collection-observations)
- [Dashboard count and evidence contracts](#dashboard-count-and-evidence-contracts)
- [Import review collection contract](#import-review-collection-contract)
- [Local book identity choices](#local-book-identity-choices)
- [Removed book collection and restore](#removed-book-collection-and-restore)
- [Search identity lookup and preserved adds](#search-identity-lookup-and-preserved-adds)
- [Complete compatibility book reads and atomic selected edits](#complete-compatibility-book-reads-and-atomic-selected-edits)
- [Cold-statistics file evidence](#cold-statistics-file-evidence)

## Direct native book lookup

`GET /api/v1/wanted/{uuid}` retrieves one tracked book independently of collection
limits. Removed and ignored rows remain readable for explicit recovery; only
missing records return 404. Malformed IDs return 400 and storage failure returns
503. Reading an inactive record does not restore or monitor it. `GET /api/v1/library/files?wantedId={uuid}` filters file
associations before the result limit. Book routes use these queries and preserve
the distinction between missing data and a retryable service failure. The paged
collection and compatibility book contracts below describe full book browsing;
other compatibility resources retain their individual limits.

## Native book presence evidence

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

## Shared recorded file evidence

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

## Native paginated book collection

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

## Native author subscription collection

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

## Native metadata Review collection and confirmation

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

## Author candidate review collection and decisions

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

## Native file collection

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

## Import recovery collection observations

`GET /api/v1/library/import-recovery` returns independently paged operations,
Calibre handoffs and unresolved link issues. `limit` is 1–100; each collection has
`total` and optional `nextCursor`. Cursors bind creation time/identity to their
collection and the `unfinishedOnly` filter. Creation time, UUID and issue kind
provide stable tie-breakers. Migration 0052 adds supporting full/partial indexes.
A read-only repeatable-read transaction keeps each response's exact totals,
manifest rows and observations consistent; pages across requests remain live.
No filesystem or remote client calls occur during this read.

Native `recovery` evidence includes database observation time, greatest recorded
operation/file update time, verified/committed file count and lease purpose/expiry.
`held`, `expired`, `none` and `not_applicable` describe the journal evidence; they
never convert a slow operation to failed or assert that an expired owner is dead.
Pending committed cleanup exposes the lease as cleanup ownership, not transfer. The existing retry endpoint keeps
all ownership, immutable-plan and byte-verification checks authoritative.

## Dashboard count and evidence contracts

`GET /api/v1/system/attention` requires normal API authentication and returns
`observedAt`, `importReviews`, `importOperations`, `calibreHandoffs` and `legacyLinks`.
One SQL statement counts pending reviews, unfinished transfer/local-cleanup work,
uncommitted Calibre handoffs and unresolved legacy links at one database snapshot.
It returns no paths or manifest payloads and makes no external requests. Missing
persistence or failed reads return 503 rather than zero counts. It uses a five-second
read deadline and `Cache-Control: no-store`.

The dashboard requests one row from the existing metadata and author-review
collections and uses their complete counts. `GET /api/v1/acquisition/queue` now
returns a full active-ledger summary independently of the bounded `items` preview.
A batched release-count query and a single client observation supply classification;
only preview rows need detailed release hydration. Removed/ignored books are
excluded before selecting the preview. `previewLimit` reports that bound, and
`downloads` reports fresh/notConfigured/partial/unavailable evidence. Missing
client observations produce `unknown` instead of suggesting a new grab; positive
observations and saved import history remain usable. The summary's imported count
is acquisition history, distinct from native library file-presence evidence.

Counts from different dashboard sources have independent snapshots and refreshes.
A failed or malformed source prevents the all-clear and retains an explicit warning;
old cached counts are not presented as a fresh successful refresh. Recovery routes
use `/imports?unfinishedOnly=true#recovery`. No summary read initiates acquisition.

## Import review collection contract

`GET /api/v1/library/import-reviews?view=collection` preserves the legacy list
response for callers omitting `view`. The collection returns `reviews` (always an
array), `total`, `filtered`, global `counts.pending`/`counts.resolved`, `nextCursor`
and `observedAt`. Query parameters are `status=pending|resolved|all` (default
pending), `format=all|ebook|audiobook|unknown`, `kind=all|file|payload`, literal
case-insensitive `q` (title, author, source path and reason), `limit=1..100`
(default 50) and `cursor`. Resolved means every non-pending saved decision,
including imported, skipped and rejected; the legacy list retains exact statuses.

A read-only repeatable-read transaction keeps counts and rows consistent within a
response. Creation time descending plus UUID descending is the stable keyset;
updates/retries do not move records, and a deleted anchor does not invalidate the
next page. Cursors bind status, format, kind and search; limit may change. Migration
0053 adds full and pending creation/identity indexes. Pages are live observations
across requests. No filesystem or client calls run during collection reads.

Normal API authentication applies. Duplicate/unknown query keys, invalid filters,
limits and incompatible cursors return 400. Unavailable persistence returns 503,
with a five-second request deadline and no private database error text. Successful
responses use no-store. File bulk actions retain explicit selected IDs, while
pending payloads retain their individual preview and resolve contracts.

## Local book identity choices

`GET /api/v1/library/book-choices` returns `books` (array), `selected` (optional
identity), `total`, `filtered`, `nextCursor` and `observedAt`. Each identity includes
only its ID, saved title/author and format. `q` is a literal case-insensitive title,
author or saved-ID search; `format=all|ebook|audiobook`, `limit=1..100` (default 50),
`cursor` and `selectedId` complete the query. Total counts every active identity;
filtered applies search/format. Removed/ignored books are excluded. Owner edits
persisted in wanted_items take precedence over the works fallback for blank titles.

A read-only repeatable-read transaction serves counts, choices and the selected
identity. Selection is independent of search/page but must remain active and match
the requested format. Cursors bind search and format; changing selectedId or page
size does not invalidate a cursor. Creation-time/UUID descending ordering remains
stable when labels change. Migration 0054 indexes active creation/identity order.
Each page is a live snapshot, not an immutable multi-request export.

No file-presence projection, download client or provider is consulted. Normal API
authentication, strict query validation, a five-second deadline, no-store and generic
503 errors apply. This replaces capped identity selectors without changing the
legacy wanted list, import execution, or payload preview contracts.

## Removed book collection and restore

`GET /api/v1/library/removed-books` returns `books` (array), total/filtered counts,
global `counts.removed`/`counts.ignored`, `nextCursor` and `observedAt`. It accepts
literal case-insensitive `q` (title, author or saved ID), `format=all|ebook|audiobook`,
`status=removed|ignored|all` (default removed), `limit=1..100` (default 50) and
`cursor`. A read-only repeatable-read transaction provides one response snapshot;
creation-time/UUID descending keys bind search, format and status. Migration 0055
indexes inactive ordering. Labels/overrides are read in batches. It performs no
provider/client/filesystem call and makes no current file-presence claim.

`POST /api/v1/wanted/{id}/restore` accepts the reviewed `updatedAt` and an optional
boolean `monitored` (default false). A row lock checks the exact timestamp and
removed/ignored status before changing only status to wanted, monitoring and update
time. The existing identity, file links, metadata, root, profile, tags and author
policy are retained. `wanted_restored` history commits atomically with the change;
a history failure rolls back the restore. Repeated/stale decisions return 409,
missing IDs 404, malformed bodies 400 and persistence failures 503. A response lost
after commit should be reconciled by reading the current record, not blind replay.

Both routes require normal API authentication and use no-store on success. Reads
have a five-second deadline and restore ten seconds, including lock waits. No
acquisition or file mutation is performed by restore. Monitoring can allow later
scheduled work only when explicitly selected. The legacy update API remains
available for existing integrations.

## Search identity lookup and preserved adds

`POST /api/v1/library/book-matches` accepts `candidates` with a unique `key`,
`provider`, `workIds`, `editionIds`, `sourceKey` and ebook/audiobook `format`.
The authenticated, no-store route bounds bodies to 1 MiB, batches to 100 candidates,
combined work/edition IDs to 64 per candidate, keys to 2048 bytes and identities to
512 bytes. Unknown fields, duplicate candidate keys, trailing JSON and invalid
formats fail with 400. Storage failures return 503, not empty matches. Reads have a
five-second deadline and a repeatable-read snapshot.

Matching joins exact provider/source identities and typed work/edition provenance
against all wanted rows in the requested format, including removed/ignored rows.
Legacy work/format source placeholders remain recognized. Titles and author-name
similarity are not identity evidence. Each candidate returns its exact `total`
and up to ten saved books, ordered active before inactive, then creation time/UUID.
Owner overrides are hydrated in a batch. No provider, client or filesystem probe
runs. Schema 0056 indexes wanted work/format and edition/format joins. The UI shows
ambiguity explicitly and validates the complete response before permitting Add.
These are tracking badges, not current file-presence claims.

Search sends `preserveExisting: true` on native wanted creation. Ordered identity
locks and existing local work locks coordinate concurrent preserved adds, including
merged and previously saved provider aliases. The transaction rechecks identities
before changing metadata; any existing tracking returns 409. An insert conflict
also returns 409 and rolls back instead of updating the owner record. Creation has
a ten-second HTTP deadline. Existing callers that omit the opt-in flag retain the
legacy upsert contract; automated author additions retain OnlyIfUntracked behavior.
No duplicate is selected for mutation and no inactive record is silently restored.

The shared collection projection materializes the wanted-ID/file-evidence join
before work/profile enrichment. The new work index otherwise changes the planner's
join order, rescanning wanted rows for every evidence row. This boundary retains
indexed identity lookups without quadratic collection reads.

## Complete compatibility book reads and atomic selected edits

Readarr-compatible `GET /api/v1/book` returns the complete active book array;
removed and ignored records are excluded. Details, release targets and selected
book/upgrade search commands resolve against the same complete collection. The native UUID
(`librarryId`) takes precedence, followed by the emitted numeric ID and then a
unique raw work/edition/source alias. Titles and hashes of aliases are not book
identities. Missing selections fail; ambiguous aliases or numeric collisions
return 409 instead of choosing a record. Numeric IDs still use the historical
31-bit hash; a persistent collision-free mapping remains open under S16.

Missing and cutoff pages share native book/file/download/quality evidence in a
read-only repeatable-read transaction with one download-client observation.
An imported lifecycle string alone does not establish presence. Partial chapter
sets remain incomplete; uncertain files or unavailable client evidence remain
unknown. The response adds `librarryStateCounts`, `librarryUnknownBooks` and
`librarryDownloads`, and records carry their derived state/evidence. Quality
profiles and owner metadata are hydrated in the same database snapshot.

`page` is 1–1,000,000 and `pageSize` 1–1,000 (defaults 1 and 100). Sort keys are
`title`, `authorTitle`, `releaseDate` and `id`, with `ascending` or `descending`
and a deterministic UUID tiebreaker. Invalid, empty or repeated paging/sort values
return 400. Counts cover the entire filtered collection; only the selected page
is hydrated. Migration 0057 supplies the SQL equivalent of the existing numeric
ID function for honest numeric sorting. Unknown release dates are not invented
from creation timestamps. Separate page requests do not freeze ongoing edits.

Book monitor/editor/delete accepts at most 500 selected identities. Every target
must resolve before writing. A transaction locks targets in UUID order, verifies
that each is still active with its reviewed update timestamp, and commits all
book changes/overrides/tag creation together. Stale targets return 409; an unknown
or inactive target returns 404; storage failures return 503. Delete retires
tracking and disables monitoring; it does not delete media. Lost responses should
be reconciled by rereading the records. This transaction guarantee is specific to
these book operations, not author editors or multi-file manual imports.

Full-array reads have a ten-second deadline, bounded wanted pages five seconds,
and book mutations fifteen seconds including selection and lock waits. Single-ID
selection currently hydrates the full compatibility collection, which is complete
but more expensive than the native local selector. Other legacy collections,
unsupported payload fields and real Readarr-client qualification remain open.

## Cold-statistics file evidence

Migration 0058 preserves the `librarry_book_file_evidence` contract while fencing
its linked-file lookup with a lateral subquery. The file is looked up by the
recorded link ID before the media-format condition is applied. Without that
boundary, missing column statistics can cause Postgres to choose a media-format
index scan for every book, including books with no linked files. The regression
fixture disables auto-analysis, includes many unrelated files and one recorded
link, and bounds examined file rows rather than relying on a machine-specific
latency assertion or a particular join type. Existing compatibility page
validation, five-second deadline and evidence rules remain unchanged.
