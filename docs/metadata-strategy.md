# Metadata Strategy

Readarr's failure mode was metadata centralization. Librarry avoids that by
normalizing multiple providers into a local canonical model with provenance.

## Provider Order

1. Hardcover is the primary rich provider for modern book, edition, series,
   ebook, and audiobook metadata. It requires a backend-only token.
2. Open Library is the open-data backbone for works, authors, editions, ISBNs,
   and covers. Author identity lookup uses the Open Library author search API,
   while monitoring verifies the author and traverses every works page. Hardcover
   monitoring likewise follows stable author IDs and paginated book contributions.
   Limited/name-only search results cannot establish complete author coverage.
3. Google Books runs after primary providers only when no suitable exact ISBN or
   full-title match exists. Returned matches are validated, including ISBN
   checksums/equivalence, title/subtitle, known format and language. It never runs
   for author, bibliography or series queries. See [provider setup](provider-setup.md)
   for exact lookup rules and limits.
4. Local OPF sidecars, embedded EPUB package metadata, MP3 ID3 tags, and
   M4B/MP4 metadata atoms are high-confidence import evidence.
5. Manual overrides always win.

## Matching Policy

Exact identifiers beat fuzzy matching:

- ISBN, ASIN, and provider IDs are high-confidence matches.
- Search results from different providers are clustered before ranking when
  they share an exact edition ISBN or verified work/edition identity.
  The primary result keeps the best provider identity while merged provider IDs,
  identifiers, dates, publisher fields, covers, and match evidence are retained
  as provenance.
- Title plus author identity can be medium-confidence.
- Author subscriptions require stable provider identities and match those IDs or
  explicit provider aliases. A matching name cannot override a different ID.
  Same-name author search results remain separate without common identity evidence.
- Original publication dates are separate from edition dates and drive whole-work
  first/latest policies when available. Non-writing or unknown Hardcover credits
  enter review before policies select eligible works.
- Ambiguous title-only associations in import and metadata correction go to manual review. Explicit user selection in Add New follows the separate discovery contract below.
- Format selection is explicit: ebook and audiobook editions must not silently
  collapse into the same file target.

## Sources Not In Core

Goodreads, Amazon, and Audible scraping are intentionally excluded from core.
They can be future optional plugins only if the legal and operational risk is
made explicit.


Native metadata Review uses the same field evidence as direct book provenance,
including imported and unmonitored tracked books. Text comparison retains Unicode
letters, marks and numbers and normalizes canonical composition. The wanted
media format is the owner's acquisition choice; a provider describing an ebook
edition does not by itself create a conflict for an audiobook target. Its format
remains visible as provider evidence. Keep current confirms a displayed evidence
revision and protects newer owner corrections or cleared overrides. Fields with
no current value still require an explicit choice in the book's metadata editor.

## Search ranking research

The [September 21 ranking design](metadata-ranking-design.md) separates candidate
retrieval, coherent edition selection, relevance, and metadata quality. Its frozen
16-query benchmark found that retrieval/language normalization fixes account for
the measured gains; extra ranking weights did not outperform provider relevance.
The initial retrieval/normalization baseline is now implemented in source, with
identity tiers and evidence labels. Provider relevance determines discovery order;
legacy scores remain in the API for compatibility and are not probabilities.
The Add New UI omits confidence badges and does not use those scores to gate an
explicit user selection. Only concrete edition conflicts or a missing author
require add confirmation. Unknown source formats remain unknown; the visible
acquisition format controls the wanted item. Provider evidence and identifiers
remain available in an expandable details section.
Search records merge only with shared edition ISBN or identical work/edition
identity, preserving source IDs. Text-only enrichment of persisted records follows
its existing separate review contract. The custom weighted ranker remains future qualification work. Explicit Series
mode uses stable Hardcover series IDs and numbered memberships, without deriving
order from publication years. Credentialed live qualification is recorded in the
[metadata rollout](reviews/2026-09-21-metadata-rollout.md).
A bounded explicit-author rescue and chooser for returned editions are implemented;
see the [expanded iteration](reviews/2026-09-21-metadata-discovery-iteration.md). See the
[implementation checks](reviews/2026-09-21-metadata-discovery.md). That retrieval
baseline is deployed; the UI simplification and cleanup below remain source-only.

## Candidate discovery cleanup

The follow-up to the credentialed rollout adds primary, related and incomplete
presentation sections. A bare exact-title stub with no author cannot outrank a
coherent matching digital edition. Verified title focus accepts subtitle variants;
a keyword inside an unrelated title does not qualify. When that strong evidence
exists, records by unrelated authors move to the expandable related section.
Otherwise provider relevance remains the baseline, with zero-overlap unrelated
hits demoted only when another result supplies a credible title anchor.

Summaries, workbooks and clearly labeled companion/collection material move to
related results unless explicitly requested. Exact ISBN results remain primary,
including conflicts. Series retains source order; unnumbered supplements, other
matching series and work-only records are separated from numbered usable editions.
These sections do not modify provider facts or acquisition policy. Verified work
aliases may share one UI row, but text-only duplicates remain separate.

See the [live follow-up](reviews/2026-09-21-discovery-cleanup.md). This follow-up is
source-only until a separate deployment; the preceding retrieval/edition rollout
is already live as recorded in [status](status.md).
