# Provider Setup

## Hardcover

Set `LIBRARRY_HARDCOVER_TOKEN` on the backend. The token is server-side only and
enables book search. **Configured** means the token is present; it does not prove
access. In System, **Check Hardcover** sends the documented read-only `me { id }`
query. A valid user response establishes successful authentication; the response's
user details are not returned to the browser or saved. A copied `Bearer ` prefix
is normalized so it is not sent twice. Regular successful searches also establish
request evidence. GraphQL/HTTP errors are shown as degraded, rejected credentials,
forbidden access, rate limiting or an unavailable provider as appropriate. Typesense result documents are decoded, but author
bibliography traversal and full rich edition/series enrichment are still pending
qualification. Open Library continues to work independently.

## Open Library

Open Library requires no token. Librarry uses it as the open-data backbone and
cover fallback. Author lookup uses Open Library's author search endpoint to get
stable author IDs. Author monitoring then uses those IDs with the works-by-author
endpoint when available, falling back to an author-name book search for manually
entered authors or non-Open Library identities.

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
a candidate, not proof of the right author or edition. Real-key qualification is
still pending.

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
Real Hardcover and Google credential qualification remains pending. The probe
contracts follow [Hardcover's getting-started example](https://docs.hardcover.app/api/getting-started/)
and [Google Books volumes search](https://developers.google.com/books/docs/v1/using#PerformingSearch).
