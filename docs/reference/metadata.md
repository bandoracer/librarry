# Metadata contracts

Implementation contracts for contributors. Operator instructions live in the [guides](../README.md#operate). These describe candidate source behavior, not complete compatibility or live qualification.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Metadata Model](#metadata-model)
- [Provider request observations](#provider-request-observations)
- [Exact metadata fallback](#exact-metadata-fallback)
- [Provider result cache](#provider-result-cache)
- [Complete author bibliographies](#complete-author-bibliographies)
- [Shared provider HTTP budget](#shared-provider-http-budget)
- [Author identity and destinations](#author-identity-and-destinations)

## Metadata Model

The data model separates conceptual works from concrete editions. This is the
main durability choice: a single book can have many ebook, audiobook, print, and
translated editions.

Manual overrides are stored separately and should always win over provider data.
Provider records keep raw provenance so future reconciliation can explain where
data came from and why a match was accepted or sent to review.

## Provider request observations

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

## Exact metadata fallback

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

## Provider result cache

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

## Complete author bibliographies

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

## Shared provider HTTP budget

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

## Author identity and destinations

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
