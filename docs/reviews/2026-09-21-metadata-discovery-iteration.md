# Discovery iteration — expanded benchmark and edition selection

Historical implementation checkpoint. Subsequent deployment and live provider
qualification are recorded in the [rollout](2026-09-21-metadata-rollout.md).

Implemented locally on `codex/metadata-ranking-design`. **Not published or
deployed.** Builds on the [initial implementation](2026-09-21-metadata-discovery.md).

Follow-up: [series and edge cases](2026-09-21-series-and-edge-cases.md) adds explicit
series mode and replaces the work-title rescue with translated-title-aware retrieval.
The results below describe the preceding iteration.

## Changes

- Recognize `title author` using the actual returned author identity and an exact
  remaining title, rather than guessing the last words of every query are names.
- For explicit `title by author` searches with no matching result, send one
  structured Open Library `title` + `author` rescue request. Each branch is capped
  at 25 candidates. Generic searches remain one request; exact ISBN/health probes
  retain their existing smaller limits. Preserve useful primary results and
  surface a secondary failure without caching it as complete success.
- Group returned ebook/audiobook editions under a verified work ID in Add New.
  An edition selector changes the complete selected record, artwork, evidence,
  saved-library identity check and add payload together. Adaptations and unknown
  identities stay separate. This chooses among returned records; it does not
  fetch a complete remote edition catalog or merge metadata between options.

## Evidence

The [expanded corpus](../../research/metadata-ranking/expansion/README.md) adds 34
queries with nonfiction, technical books, cookbooks, poetry, translations and
explicit non-English preferences. Previous local implementation: 47/50 intended
first results across both cohorts. Current: **49/50 first, 50/50 within ten**.
This is diagnostic coverage, not a 98% production-accuracy claim. The labels are
not independently judged; two intents also refer to the same translated work.

Newly recovered cases were independently corroborated against the author's
[Parable bibliography](https://www.octaviabutler.com/parableseries/) and the
publisher's [Devotions record](https://penguinrandomhousesecondaryeducation.com/book/?isbn=9780399563249).
Live Open Library checks through the changed Go implementation placed both works
first, along with the original Percy Jackson, Project Hail Mary and ISBN checks.

An exact-title boost requiring author/ISBN completeness was tested and rejected.
It brought back five bad results in the original cohort, including an adaptation
and a game. No generic exact-title promotion or custom Python score was shipped.
The remaining miss, [The Employees](https://www.ndbooks.com/book/the-employees/),
is still sixth behind HR books; it is explicitly retained as a known ranking gap.

Verification: all checks below passed.

- Go metadata regressions cover both cohorts, cache replay, bounded rescue,
  structured parameters, partial failure and non-caching of incomplete results.
- `go test -race ./...` against a disposable loopback Postgres cluster.
- 29 frontend unit tests and TypeScript/Vite production build.
- 16 desktop/mobile browser tests, including complete selected-edition add payload,
  separate adaptations, image changes, existing saved identities and error paths.
- Five fresh upstream smoke queries passed. The Parable request used its bounded
  rescue; no book was acquired or added during these upstream checks.

## Remaining

- Better unconstrained ambiguous-title ranking, especially The Employees, without
  regressing series and adaptations.
- A separately judged cohort, misspellings/no-match cases, richer audiobook and
  credentialed Hardcover/Google qualification. Fifty exploratory queries do not
  satisfy all production gates in the ranking design.
- Complete edition enumeration and provider-confirmed series reading order.
  There is still no trustworthy series-position source configured for the live
  Open Library-only deployment; no publication-year or title-based order is invented.
- Image packaging, PR/main promotion and deployment are separate from this local
  iteration. The NAS app is unchanged.
