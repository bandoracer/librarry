# Series ordering and discovery edge cases

Implemented locally on `codex/metadata-ranking-design`; **not packaged, published
or deployed**. Continues the [previous iteration](2026-09-21-metadata-discovery-iteration.md).

## Implemented

Add New has an explicit **Series** mode. It requests up to three matching
Hardcover series, with at most 25 non-compilation book memberships each. Results
retain a stable series ID, series name and the source's numeric position. Each
series stays together; zero/fractional positions are preserved and missing
positions sort last. Publication dates and title digits never invent positions.
Matching exact names precede partial names within the bounded returned set.
This is discovery of a limited window, not complete bibliography enumeration.
The API displays at most 50 edition records; selecting an ebook/audio record keeps
its edition identity intact. The same work in two series stays in both groups.

A missing Hardcover token produces an actionable error. Series queries do not
silently fall back to Open Library title searches or contact Google. The complete
GraphQL query validates against Hardcover's published schema; live permission and
catalog checks still require credentials. No public website is scraped by runtime.

Explicit `title by author` rescue now uses Open Library general `q` for the title
with a separate author constraint. Work-title-only lookup missed translated titles.
`The Employees by Olga Ravn` now retrieves the English edition of `De ansatte` and
ranks it first. The previous Parable rescue still works. Generic title searches
retain their previous ordering; `The Employees` alone is still sixth.

## Evidence and limitations

The previous two cohorts remain **49/50 intended works first, 50/50 within ten**.
The [ten new edge cases](../../research/metadata-ranking/edge-cases/README.md) contain
eight positive intents (six first-place matches, two typo retrieval misses) and
two intentionally empty queries (both empty). These exploratory cases are not a
separate independently judged benchmark. Audiobook format remains unknown when
Open Library does not provide format evidence.

Validation: full Go race suite with a disposable loopback Postgres database;
29 frontend unit tests; TypeScript/Vite build; 18 desktop/mobile search browser
tests, including the two new series tests; and official GraphQL schema validation.
No acquisition, live library mutation, provider configuration or deployment occurred.

The live provider health endpoint was rechecked September 21 (September 22 UTC):
Hardcover and Google Books both report `missing_credentials`. Configure the
server-side tokens described in [provider setup](../provider-setup.md) before live
multi-provider qualification. Series search is locally tested, not live-qualified.

Remaining: unconstrained ambiguous-title ranking, typo rescue, full edition/series
pagination, independent relevance judgments, and credentialed rich-provider tests.
Packaging/main promotion and NAS deployment remain separate release work.
