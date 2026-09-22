# Adaptation evidence and search release

Deployed and promoted to `latest` with the preceding UI/discovery changes. See
the [release record](2026-09-21-discovery-rollout.md) for rollout evidence.

## Evidence and behavior

Direct source checks found that Hardcover category 4 is Graphic Novel, while
some graphic volumes are incorrectly categorized as Book but carry comic genres.
Open Library subjects and Hardcover genres can also include comics on original
prose works. The original Sapiens and Hobbit demonstrate why tags alone are not
sufficient to classify a particular edition.

- Preserve Hardcover category, genre and edition-information evidence and Open
  Library subjects in normalized results, without adding provider requests.
- Label explicit category/title evidence **Graphic novel**, and edition-level
  abridgment evidence **Abridged**. `unabridged` does not match.
- Corroborate comic subjects/genres with a shared original author and a distinct
  additional credited author before showing **Possible graphic adaptation**.
  Same-work editions and verified aliases are excluded from this inference.
- Move alternatives to related results when a coherent original is available.
  Explicit comic/manga/graphic or abridgment requests keep those choices visible;
  explicit comic searches prefer matching titles with comic evidence over unrelated
  provider hits. Exact ISBNs always remain primary. Standalone original graphic
  works remain primary.
- Show the concise label in result rows and selected-book details. No extra
  confirmation or acquisition step was introduced.

Source references: [Hardcover schema](https://github.com/hardcoverapp/hardcover-docs/blob/main/schema.graphql),
[original Hobbit](https://openlibrary.org/works/OL27482W),
[Hobbit adaptation](https://openlibrary.org/works/OL219602W),
[original Sapiens](https://openlibrary.org/works/OL17075811W).

## Verified checks

All 28 fresh credentialed searches put the intended work/series first: the
preceding 24 retained their first result, while Watchmen, Maus, The Hobbit graphic
novel and The Stand comics verified graphic intent. The two known partial
Hardcover warnings (Way of Kings and Educated) remain visible; no rate-limit errors
occurred. Results are in [the captured readback](../../research/metadata-ranking/live-review/adaptation-live.json).

The Sapiens work's default ebook is actually titled *Sapiens: A Graphic History,
Volume 2*, while its default audiobook is the original. Explicit edition-title
evidence correctly labels that ebook, even though the provider assigns it to the
original work. An extra contributor alone never labels a same-work edition.

Metadata/race tests, the full Go suite, 33 frontend tests, production web build,
42 desktop/mobile checks and deployment configuration checks passed locally.

## Validation boundaries

The regression suite covers originals with comic tags, corroborated adaptations,
known categories, same-work edition contributors, verified aliases, missing tags,
explicit comic intent, exact ISBNs and unabridged wording. Provider tests verify
normalization and cached evidence isolation. Desktop/mobile tests verify labels
and that selecting secondary results preserves the full edition payload.

The existing captured search cohorts predate the subject projection; their tests
ignore only that additive field when comparing historical requests. They remain
regression evidence for title/edition behavior, not new subject coverage.

Incomplete or incorrect provider data still prevents reliable classification.
For example, the mislabeled Sapiens: Das Spiel der Welten record has neither a
Graphic Novel category nor comic genres. It remains unclassified; no hardcoded
book or author exceptions were added. Unverified duplicate records and unlabeled
bundles remain. No live acquisitions are used in these checks.
