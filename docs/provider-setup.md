# Provider Setup

## Hardcover

Set `LIBRARRY_HARDCOVER_TOKEN` on the backend. The token is server-side only and
enables book search. **Configured** means the token is present; it does not prove
access. In System, **Check Hardcover** sends the documented read-only `me { id }`
query. A valid user response establishes successful authentication; the response's
user details are not returned to the browser or saved. A copied `Bearer ` prefix
is normalized so it is not sent twice. Regular successful searches also establish
request evidence. GraphQL/HTTP errors are shown as degraded, rejected credentials,
forbidden access, rate limiting or an unavailable provider as appropriate. Typesense book/author results are decoded. Author search retains numeric provider
IDs, and monitoring traverses the selected author's book contributions with
pagination. These Hardcover paths have contract-fixture coverage. On September 21, 2026,
the maintainer NAS passed authentication and a title search with covers. A local
credentialed title probe returned coherent work/ebook metadata; series discovery initially failed on a forbidden substring operator. The corrected
search-by-ID path now returns numbered Percy Jackson memberships. Exact ebook
ISBN lookup succeeds; one paperback ISBN returned no edition results.
Broader edition/series/list qualification remains pending; see the
[credential setup record](reviews/2026-09-21-hardcover-credentials.md). Open Library
continues to work independently.

Series mode discovers up to three identities with Hardcover's dedicated Series
search, then queries their explicit memberships and numbered positions.
It returns a bounded discovery window: up to three matching series, 25
non-compilation books each, and at most 50 displayed edition records. Exact names
lead within the returned set. Retrieval prioritizes provider-featured memberships
within its bound; this flag is not proof of a primary novel. Verified ebook/audio
editions lead work-only records, with numbered order retained within each group
and missing positions last. It does not enumerate a
complete series. A missing token is surfaced instead of using unrelated title
results. The corrected query passed a live credentialed Percy Jackson check on September 21;
this is bounded discovery evidence, not proof of complete series coverage.

## Open Library

Open Library requires no token. Librarry uses it as the open-data backbone and
cover fallback. Author lookup uses Open Library's author search endpoint to get
stable author IDs. Author monitoring verifies that identity, then pages through the works-by-author
endpoint and checks the declared count. A name-only or synthetic legacy identity
cannot establish a complete bibliography: select the correct author from search
and replace the old subscription, preserving its desired settings. Existing
wanted books are retained.

Book discovery uses one general `q` search, up to 25 work candidates, with the
configured language preference passed to edition selection (`lang=en` for English).
The displayed page remains ten results. Explicit `title by author` queries with no
matching candidate can make one additional general `q` title search constrained by
`author` (including translated edition titles), also
capped at 25. A failed rescue preserves primary results and reports the error.
ISBN and author searches use their specific
routes; connection checks remain one-result ISBN requests. Only `editions.docs`
supplies edition ISBN, language, cover, publisher and publication date. A work's
language array is kept separately. The search endpoint does not verify media
format, so Open Library results show **Format unknown** and require add review.
General searches exclude known nonpreferred edition languages; an exact ISBN
edition is retained with an explicit conflict so a different edition is not
silently substituted. These are discovery rules, not acquisition authorization.

## Google Books

Set `LIBRARRY_GOOGLE_BOOKS_API_KEY` to enable exact fallback lookups. Book search
contacts primary providers first. Google runs only when they have no exact ISBN or
full-title result compatible with the selected language and known format. Author,
author bibliography and series queries never contact Google. A primary outage
can trigger fallback; its error stays visible alongside any successful results.

Use a checksum-valid ISBN-10/ISBN-13 (optional `ISBN:` prefix), or the full title,
including a subtitle when present. Equivalent ISBN-10/978 ISBN-13 values match.
Malformed explicit ISBNs do not become title searches. Title matching ignores
case, punctuation and Unicode composition, while preserving accents and all
words. Related titles and study guides are excluded unless their full title was
requested. Exact returned identifiers/titles are checked locally, not assumed
from Google's search ranking.

Results retain provider IDs, original identifiers, contributors, language and
source keys. Known incompatible languages/formats are excluded; unknown evidence
stays unknown. Google results are marked ebook only when the provider says so;
requesting audiobook never invents audiobook metadata. A title match identifies
a candidate, not proof of the right author or edition. The maintainer NAS passed
a credentialed Google Books health lookup on September 21, 2026. Broader fallback
and ranking qualification remains pending; see the [setup record](reviews/2026-09-21-google-books-credentials.md).

## Local Metadata

Local OPF sidecars and embedded EPUB package metadata are extracted during
library scan/import and used as high-confidence local evidence for title,
author, identifiers, language, publisher, and series metadata. MP3 ID3 tags and
M4B/MP4 metadata atoms are also extracted for audiobook imports. The provider is
present as a health/diagnostic source and does not return remote search results.


## Observed provider health

System reads cached observations; opening or refreshing the page does not contact
metadata providers. **Check Open Library** and **Check Google Books** perform a
one-result ISBN lookup. Missing credentials skip remote checks entirely. Configured
remote providers start unverified after process restart. Local OPF is local import
evidence and cannot establish readiness for remote book/author lookups.

Cards show the last actual request and last successful request separately. An
outage retains the historical success time but does not claim current
reachability/authentication. An invalid token clears successful authentication;
forbidden/malformed responses do not prove it. A valid empty result is a successful
request, while a missing expected result structure is an error. Provider response
bodies and credential-bearing request URLs are excluded from returned errors.

Concurrent requests run one at a time per provider, with a bounded wait. Explicit
checks reuse a recent observation for 15 seconds. HTTP 429 prevents another request
until Retry-After (one minute when absent/invalid, capped at 24 hours). This is
request backoff, not a scheduled background retry. Observations are in memory;
restarting or constructing providers with different credentials resets them.

Connection checks do not certify rich metadata coverage or author/list traversal.
Hardcover and Google authentication checks have passed on the maintainer NAS;
that does not qualify all search paths or catalog coverage. The probe
contracts follow [Hardcover's getting-started example](https://docs.hardcover.app/api/getting-started/)
and [Google Books volumes search](https://developers.google.com/books/docs/v1/using#PerformingSearch).


## Search result caching

Repeated identical queries reuse successful provider results for five minutes;
valid empty results expire after 30 seconds. Hits do not extend those lifetimes
or change provider request/success timestamps. Query text, type, format, language,
limit and provider identity are separate cache keys, including author IDs. Errors
are never cached. A failed search or explicit health check clears the affected
provider's cached results, so known outages or rejected credentials remain visible.
Quota backoff and waits canceled before sending retain unexpired successful data.
Other providers keep their independent cache entries. Concurrent identical misses
share a successful fetch; canceled requests do not clear earlier successes.

Caches are process-local and tied to one provider configuration. Restarting or
replacing the metadata service clears them. Each provider retains at most 128
entries and 2 MiB of serialized results/query keys; responses over 256 KiB and
query keys over 4 KiB are returned normally without caching. Least-recently-used
entries are evicted. This is short-lived search reuse, not a persistent raw record
or bibliography store, and does not establish complete author coverage.


## Complete author monitoring

Monitoring uses a separate bibliography operation, scoped to the selected provider
and stable author ID. It does not use ranked search results or the legacy
`searchLimit` as a total-book cap. Open Library checks count consistency, duplicate
IDs and premature endings. Hardcover walks book IDs in ascending order until an
empty page, validating the selected author's contributions. Missing identity,
malformed pages, changed counts, request failures, a two-minute traversal timeout
or the 10,000-record safety limit fail explicitly; no partial bibliography is
applied and the subscription stays unsynced. Provider records can change during
traversal; this is not an atomic snapshot of an external database.

Same-name authors remain separate unless stable identities match. Hardcover
illustrator/editor/unknown credits enter metadata review unless the selected
person also has an explicit Author/Writer credit. Eligible works alone determine
first/latest policies. Original publication dates are distinct from edition dates;
work-only evidence does not invent an ebook/audiobook edition. Rich edition
selection, all-policy qualification and legacy identity repair remain open.


Monitoring adds newly discovered works and reuses existing work/format tracking,
including older synthetic edition identities. Existing selected editions, manual
corrections, unmonitored state and removed entries survive; this pass does not
refresh or re-add existing books. Legacy/Readarr identities can still be stored
for migration, but require provider identity repair before automated traversal.


## Shared request pacing and quotas

The application shares one HTTP request budget between metadata lookups, author
pages, explicit connection checks and Hardcover import lists. Requests start at
most once per second per provider host. Open Library uses its conservative
unidentified-client limit; no operator contact address is assumed. Hardcover and
Google use the same conservative spacing, while actual response quotas can stop
requests for longer.

HTTP 429 respects Retry-After. Hardcover's documented named `RateLimit` buckets
also stop requests proactively when remaining quota reaches zero, even after a
successful response. Legacy `X-RateLimit-*` headers are fallback evidence when
structured buckets are unavailable. Daily and burst waits use the longest valid
reset, capped at 24 hours. A blocked request fails promptly with the retry time
visible in System. It does not advance actual request timestamps or invent
reachability/authentication. Successful cached searches remain usable until their
ordinary expiry, and another provider's budget stays independent.

This budget is process-local. Other applications or replicas can consume the same
account quota independently; provider responses remain authoritative. Restarting
or replacing the shared client resets its local observations. No credentials are
stored in budget keys. Import-list errors omit response messages and request URLs;
missing list data is an error, while a valid empty list remains valid.

References: [Open Library rate limits](https://openlibrary.org/developers/api#rate-limits)
and [Hardcover's current quota/header contract](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/Getting-Started.mdx#rate-limits).


Hardcover import lists now traverse by increasing membership ID until a terminal
empty page, rather than stopping after 200 books. Each page verifies list identity,
declared membership count and available modification timestamp; changed counts,
changed timestamps, repeated/invalid identities, hidden books and early endings
fail the complete fetch. A nullable provider timestamp is accepted; these checks
cannot provide a transaction snapshot across a concurrently edited remote list.
No entries from a failed fetch are added. Traversal is bounded to 10,000 membership
rows, 101 requests, two minutes and 4 MiB per response; exceeding a bound returns
an explicit error instead of partial success. Repeated membership of the same
book adds it once.

Sync honors every stored exclusion and reuses existing work/format tracking,
including older edition identities. It preserves removed status, manual metadata,
monitoring and existing root choices. New monitoring/root defaults commit with
creation. Failed persistence leaves the list's prior success timestamp unchanged;
the next run can reuse successful additions and retry missing ones. Optional
search-on-add is still best effort and does not have durable retry delivery.
These behaviors are fixture-qualified against the documented
[Hardcover list schema](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/GraphQL/Schemas/Lists.mdx);
a real token/private-list check remains pending.


Hardcover book discovery uses the Typesense work IDs to request a bounded batch
of work details and default ebook/audio editions. A requested format selects its
matching default; an Any-format search can return both distinct editions. Missing,
physical or ambiguous defaults retain work-only evidence with unknown format.
This does not enumerate every edition or find every alternative language; Open
Library remains the backbone and exact ISBN is the route to a specific edition.

ISBN queries bypass fuzzy work discovery and query edition identifiers directly,
including equivalent ISBN-10/978 ISBN-13 values. ISBN-979 queries never broaden to
blank ISBN-10 records. Returned IDs, parent work and ISBN evidence are validated;
known physical editions are not offered as ebook acquisitions. A failed detail
request returns a provider error rather than incomplete rich results. Successful
searches use the shared bounded cache and request budget.

Work IDs, stable contributor IDs/roles and original publication dates remain
separate from edition IDs, languages, ISBNs, publishers, release dates, covers,
pages, ASINs and audiobook duration/narrators. ASINs come from the provider API;
no retail scraping is added. Search merges reject known format/language conflicts,
different edition IDs within one provider and disjoint nonempty ISBN evidence.
Unknown work evidence cannot bridge otherwise incompatible edition candidates.

The implementation follows the documented
[book](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/GraphQL/Schemas/Books.mdx)
and [edition](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/GraphQL/Schemas/Editions.mdx)
fields. Contract tests cover these shapes; a live Hardcover token is still needed
before claiming provider qualification.

## Partial Hardcover catalog data

The source candidate preserves independently validated results when an interactive
book search encounters malformed default-edition data. A healthy sibling edition
can still be used; if only the work validates, its media format remains unknown.
Invalid work identities and conflicting edition identities are never accepted.
The response still reports a provider warning and is not cached as complete.
Exact ISBN lookup and automated author bibliography traversal retain strict
validation. Series membership validation also remains strict.

Book queries also retrieve Hardcover's `book_category_id`, `cached_tags(path: "Genre")` and edition-level
`edition_information`, plus Open Library's work-level `subject` projection.
Hardcover category 4 was verified as Graphic Novel against its live
`book_categories` catalog. Open Library subjects are aggregated work evidence,
not proof that a selected edition is a comic. See the
[adaptation policy](metadata-strategy.md#adaptation-evidence).
