# Metadata search ranking: evidence and proposed design

Research date: September 21, 2026. The original study was design/offline only.
The initial retrieval/normalization baseline has since been implemented and tested
in source; see [implementation evidence](reviews/2026-09-21-metadata-discovery.md).
The [expanded iteration](reviews/2026-09-21-metadata-discovery-iteration.md) adds
50-query diagnostic coverage, bounded author rescue and returned-edition selection.
**No deployment changed.** The wider proposal below is not fully implemented. The reproducible experiment is in
[`research/metadata-ranking`](../research/metadata-ranking/README.md).

## Recommendation

Fix candidate retrieval and edition normalization before changing ranking weights.
Preserve useful provider relevance, give explicit identifiers and author constraints
priority, and use record quality only as bounded supporting evidence. Show why a
result matches instead of presenting an uncalibrated percentage as confidence.

The initial implementation should use the simpler retrieval-only baseline plus
identity/edition safeguards. The experimental weighted ranker ties that baseline;
this benchmark does **not** justify shipping its extra weights as an improvement.

## What direct searches found

The live app has Open Library configured; Hardcover and Google Books credentials
are absent. I queried Open Library's APIs directly and the live Librarry API,
checked ambiguous work/edition records, and used author/publisher bibliographies to
label the intended works independently of ranking scores.

Hardcover's public pages were also checked: its Andy Weir author page identifies
Project Hail Mary, but book/search pages supplied incomplete page shells to the
research client. That is not an API benchmark. A direct Google Books exact-ISBN
request returned HTTP 429; it was recorded and not retried. Neither provider gets
fabricated benchmark coverage. Their absence is a limitation, not evidence that
Open Library is the best provider overall.

Representative catalog identities:

| Search | Best verified target(s) | Problem in current results |
| --- | --- | --- |
| Percy Jackson | [The Lightning Thief, OL492658W](https://openlibrary.org/works/OL492658W), plus primary novels verified against [Riordan's series page](https://rickriordan.com/series/percy-jackson-and-the-olympians/) | Bare `Percy jackson` stubs lead; primary novels are absent. |
| Project Hail Mary | [OL21745884W](https://openlibrary.org/works/OL21745884W), corroborated by [the publisher](https://global.penguinrandomhouse.com/announcements/new-andy-weir-novel-project-hail-mary-to-be-published-in-may-2021/) | Omnibus and summaries survive while the original is filtered out. |
| The Hobbit | [OL27482W](https://openlibrary.org/works/OL27482W), corroborated by [Tolkien's estate](https://www.tolkienestate.com/writing/john-d-rateliff-the-hobbit/) | Jude Fisher's companion appears before Tolkien's missing original. |
| The Left Hand of Darkness | [OL59800W](https://openlibrary.org/works/OL59800W), corroborated by [Le Guin's bibliography](https://www.ursulakleguin.com/bibliography) | Criticism and collections survive while the original is removed. |
| Murderbot | [All Systems Red, OL17914663W](https://openlibrary.org/works/OL17914663W), plus [Wells's primary installments](https://www.marthawells.com/murderbot.htm) | Title-only search returns collections and misses individually titled novels. |
| Percy Jackson the Ultimate Guide | [OL17685821W](https://openlibrary.org/works/OL17685821W), corroborated by [Disney's guide page](https://books.disney.com/book/percy-jackson-and-the-olympians-the-ultimate-guide/) | Search returns nothing; the requested words are in a subtitle. |
| 9781423103349 | [The Sea of Monsters, OL42335418M](https://openlibrary.org/books/OL42335418M) | Empty live result despite an English edition with this exact ISBN. |

The guide also exposes attribution disagreement: Open Library credits Mary-Jane
Knight; Disney's page credits Rick Riordan. Preserve both sources and flag the
conflict instead of converting an apparent agreement into false certainty.

An extra audit found that the live first result for `Dragonriders of Pern`,
[OL8339123M](https://openlibrary.org/books/OL8339123M), has the subtitle **The Book
Game** and publisher **Nova Game Designs**. The top-level title and author alone
are insufficient to select it as an original novel.

### Root causes in the current code

1. `OpenLibraryProvider.searchBooks` sends `title=...` for ordinary book queries.
   That loses matches through subtitles, series, and author terms.
2. It maps `language[0]` from a **work-wide language array** into one edition's
   language. `resultFitsQuery` then rejects the entire result for English search.
   For Project Hail Mary, the captured work starts with `fin` but includes `eng`;
   its selected matching edition is explicitly English. Array order is not a
   preferred-language rule.
3. It chooses `edition_key[0]` and attaches work-wide ISBNs to that edition.
   Those independent arrays do not describe a coherent edition. Even when a work
   matches an ISBN, the displayed edition can be unrelated to that ISBN.
4. The current score is a base 0.20 plus 0.55 for exact normalized title, or 0.35
   for substring match. Thus an almost empty exact-title stub beats a real novel
   whose title contains series context. Ties mostly preserve provider ordering.
5. Search normalizes text to ASCII and infers requested format as actual format.
   Neither is reliable evidence about a translated title or an ebook edition.

[Open Library's Search API documentation](https://openlibrary.org/dev/docs/api/search)
explicitly distinguishes work records from matching `editions.docs`, and documents
`q`, `lang`, and edition selection. Its best edition may favor readable/covers:
that still does not guarantee the ebook/audiobook format the user requested.

## Benchmark and interpretation

There are 16 query intents and 513 deduplicated candidate observations across
queries. Seven queries informed development, five were inspected validation cases,
and four additional queries were fetched only after freezing prototype weights.
Queries cover broad series, exact titles, an explicit guide, a named author,
title-plus-author text, and a valid ISBN. Labels and URLs are explicit in
[`labels.json`](../research/metadata-ranking/labels.json).

| Method | Intended target first | Target in first 10 | MRR@10 |
| --- | ---: | ---: | ---: |
| Actual live API | 2/16 | 3/16 | 0.146 |
| Re-rank only surviving live candidates, with enriched metadata | 3/16 | 3/16 | 0.188 |
| Title retrieval + correct edition language, legacy lexical scoring | 11/16 | 12/16 | 0.708 |
| General retrieval + correct edition language, native provider order | 16/16 | 16/16 | 1.000 |
| Same retrieval + experimental bounded-quality ranker | 16/16 | 16/16 | 1.000 |
| Experimental ranker with cover bonus removed | 16/16 | 16/16 | 1.000 |

The four-query blind extension passes 4/4 for both retrieval-only and weighted
ranking. **The measured improvement belongs primarily to retrieval/normalization.**
The cover bonus contributes no top-result gain here. Simply swapping ranking
weights cannot recover missing candidates.

These are manually selected diagnostic queries, mostly well-known English fiction,
not random production traffic. Success means a labeled intended work/author appears;
it is not precision, nDCG, a probability, or proof every edition is correct. Other
results are unjudged, not automatically irrelevant. The two Austen work records are
both accepted after checking that the second represents the original text with
critical apparatus. Identity grouping remains a separate issue.

For broad series queries, any labeled primary installment counts as relevant.
Starting at book one is a separate metric: the prototype returns All Systems Red
at **#5**, versus **#6** in native order. It finds Murderbot books but does not solve
reading order. Production needs verified series membership/position for that.

## Proposed search pipeline

### 1. Parse intent without inventing certainty

- Valid ISBN-10/13: verify checksum, canonicalize equivalent ISBNs, and use an
  identifier lookup. Malformed explicit ISBNs remain validation errors.
- Explicit provider ID: retain its entity kind (work, edition, author, series).
  Never treat an edition ID ending in `M` as a work merely because it appears in
  a search response's key field.
- Author tab: author-identity search, separate from book search. Same-name authors
  remain separate unless stable identity evidence connects them.
- Title + explicit author (e.g. `... by Ursula K. Le Guin`): search structured
  title/author fields plus a bounded broad fallback. Preserve the original query;
  only parse unambiguous separators and never split arbitrary titles on `by`.
- General words: broad book discovery, including provider-supported subtitles,
  aliases and series context. Do not assume every exact phrase names one work.
- Explicit modifiers such as `graphic novel`, `study guide`, `boxed set`, language
  and format change intent; they must not trigger default derivative penalties.

### 2. Retrieve enough candidates, within a budget

- Hardcover remains the intended rich primary source: search work IDs, then obtain
  bounded work/edition/series evidence through its supported API.
- Open Library uses `q=<escaped user text>&lang=en` with explicit required fields,
  including subtitle, author IDs, edition count, and `editions.docs` fields. Keep
  provider result rank and retrieval route. Do not request `fields=*` routinely.
- Start with 25 candidates per configured primary provider, up to 50 only for
  sparse/ambiguous results. The research used 40; benchmark 25 versus 40 before
  selecting the production cap. The first displayed page is not the fetch limit.
- Use at most one title/structured-author rescue query per provider when the broad
  branch lacks a convincing match. Deduplicate same-provider IDs; appearing in two
  query branches is **not independent corroboration**.
- Cap detail hydration to the leading five candidates, share existing provider
  request budgets/cache, and return partial results with explicit provider errors.
  Respect Retry-After; never let enrichment block an otherwise useful search.
- Google Books stays exact-match fallback only, under the existing policy. Local
  OPF/embedded metadata remains import evidence, not a remote discovery source.
- Author and publisher pages used in this research are benchmark references, not
  a proposal to scrape websites in production. No Amazon/Goodreads/Audible scraping.

### 3. Normalize a coherent work and edition

Keep `workLanguages` separate from `selectedEdition.language`. A known English
edition makes a work eligible for English discovery regardless of array ordering.
Missing edition language is labeled unknown; it does not become English.

For general discovery, choose a matching edition using language, verified media
format, completeness, and the provider's edition relevance. For an explicit ISBN,
resolve an edition that actually contains that ISBN; a work-wide ISBN hit alone
is insufficient. If an exact edition conflicts with a language/format preference,
show the exact identity and explicit conflict instead of silently substituting
another edition. Acquisition eligibility remains governed by the constraints.

Edition ID, title, ISBNs, publisher, release date, format, contributors and cover
must come from that same edition. Work attributes remain work attributes. Do not
copy the user's requested format into the edition's observed format. When format
is unknown, say so and require edition review before claiming a concrete match.

### 4. Rank identity first, relevance next, quality last

Use explicit tiers, then an internal ranking score:

1. **Verified requested identity:** exact provider entity or exact edition ISBN.
2. **Strong constrained match:** title/alias and explicitly supplied author agree,
   without a known edition/type conflict.
3. **Discovery matches:** provider-relevant titles, subtitles and supported series
   relationships. An exact title by itself does not escape this tier.
4. **Related/uncertain:** adaptations, companion works, thin records, or explicit
   conflicts that need review. They remain discoverable and explainable.

Inside a tier, retain provider relevance as the dominant input. Never compare
raw provider scores as though their scales were equivalent. Multi-provider rank
fusion can use weighted reciprocal rank, with equal initial weights and only
verified cross-provider identities grouped. Hardcover preference resolves equally
supported metadata fields; it must not override an exact identifier elsewhere.
Fusion weights require a real Hardcover-enabled evaluation before deployment.

Small, bounded supporting signals:

- Stable author identity and title/author agreement.
- Valid identifiers and a coherent selected edition.
- Confirmed requested language/format; unknown is distinct from conflict.
- Independent provider corroboration, counted once per provider.
- Log-capped catalog breadth, never raw ISBN/edition totals or average star ratings.
- Cover presence only as a tie-level convenience; zero cover must not disqualify a
  real obscure book or exact ISBN.

Down-rank **unrequested** summaries, workbooks, trivia, adaptations and bundles
using typed metadata where possible. Text markers are fallible supporting clues,
not hard deletion rules. Do not punish a novel titled *The Book of Lost Names*
for containing “book,” or an explicitly requested guide for being a guide.

The research prototype uses an exact-ISBN work-hit tier and the following
transparent score within a tier; it is deliberately not a calibrated percentage:

| Component | Experimental value |
| --- | --- |
| Primary general-search rank `r` | `60 / (1 + 0.08 × (r − 1))` |
| Title-only rescue rank | `20 / (1 + 0.08 × (r − 1))` |
| Query-token coverage in title/subtitle/selected-edition title | 0…3 |
| Author present / absent | +4 / −10 |
| At least one valid work ISBN | +3 |
| Selected edition known English | +3 |
| Catalog breadth | `min(6, log2(1 + edition_count))` |
| Cover present | +1 |
| Sparse one-edition record with no ISBN/language | −12 |
| Unrequested derivative / bundle marker | −18 / −10 |

Those weights were not optimized to improve a headline number. The simpler
baseline already wins all 16 work-level cases. The prototype also omits production
constraints such as explicit-author conflict detection, true edition identity
gating, multi-provider fusion and verified content types. Do not wire it directly
into the API.

### 5. Separate discovery UI from acquisition confidence

Replace `score 75%` with evidence labels, for example:

- `Exact ISBN · English edition`
- `Title and author match · EPUB verified`
- `Series match · Book 1`
- `Limited metadata · Author unverified · Cover unavailable`

Expose an expandable explanation with matched fields, provenance, missing fields,
and conflicts. Internal rank values are not user-facing likelihoods and never
become auto-grab thresholds. Preserve existing operator overrides and acquisition
review rules.

Group editions of the same verified work into a single discovery card with an
edition chooser. Keep adaptations, companion guides, bundles, and same-title books
by different authors separate. For verified series, show a series section with
primary installments in provider-confirmed order and companions separately. Never
invent series order from publication year alone.

### 6. Cover selection must follow identity

Use selected-edition cover first. A canonical work cover can be a labeled fallback
when no exact-edition image is available. Cross-provider covers require a verified
shared edition ISBN or work identity; fuzzy title similarity is insufficient.
Resolve unavailable/broken images to a stable placeholder. Record image source and
scope, and let manual cover overrides win. A blank cover is better than artwork
from an unrelated adaptation or volume.

## Implementation sequence and acceptance gates

1. **Normalization fix:** separate work language sets from edition fields, map
   matching editions coherently, and preserve unknowns. Add regressions using the
   frozen PHM/Lightning/Hobbit/ISBN responses. Verify the exact edition, not just its
   enclosing work. Keep English the standard preference.
2. **Retrieval fix:** general search with subtitle/author coverage, preserved
   provider order, bounded fallback and explicit partial-failure states. Re-run
   the current suite and benchmark candidate caps/latency before expanding limits.
3. **Ranking/explanations:** implement intent tiers and bounded signals behind a
   feature flag; compare with the retrieval-only baseline on a larger judged set.
   Require meaningful improvement before enabling additional weights.
4. **Work/edition and series UI:** grouped editions, true series order, related-work
   separation, evidence labels and correct cover provenance. Test search-to-add
   using the selected edition/format; do not let presentation silently change IDs.
5. **Qualification:** at least 50 independently labeled query intents covering
   obscure/new books, nonfiction, public-domain books, diacritics, transliteration,
   misspellings, author collisions, adaptations, bundles, known-missing covers,
   audiobook narrators, explicit-language requests and invalid/no-match ISBNs.
   Split by author/series family, not minor variants of the same query.

Production gates: no exact-ID regressions, zero edition-field leakage, no false
format claims, no foreign-language filtering of eligible English works, deterministic
results on captured fixtures, and preserved manual/provenance overrides. Evaluate
MRR, graded nDCG and precision only after pooled top results are independently
judged; measure recall before and after ranking separately. Evaluate first-book
placement separately from series relevance. Add provider-outage/rate-limit cases,
latency/request counts, and a Hardcover-enabled cohort. Do not claim that 16/16
on this diagnostic set proves production readiness.

Relevant implementation files: `backend/internal/metadata/providers.go`,
`types.go`, `service.go`, `match.go`, `merge.go`, `exact_fallback.go`,
`hardcover_editions.go`, and `web/src/features/search/SearchPage.tsx`.
