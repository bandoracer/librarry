# Metadata discovery implementation checks — September 21, 2026

Historical implementation checkpoint. Subsequent deployment and live provider
qualification are recorded in the [rollout](2026-09-21-metadata-rollout.md).

**Historical first iteration:** see the [expanded iteration](2026-09-21-metadata-discovery-iteration.md) for subsequent changes and current checks.

Status: implemented and tested in `codex/metadata-ranking-design`, based on
`09bba592a37ca0a38461396f06ce9c6bcf905194`. **Not deployed or published.**
No live library/configuration/acquisition changes were made.

## Implemented

- Open Library uses general `q` retrieval and explicit matching-edition fields.
  Ordinary discovery fetches at most 25 works in one request; ISBN queries retain
  their requested bound (including one-result health probes). Language selection
  receives the configured preference. There are no extra detail/rescue requests.
- Work languages, subtitle and all returned authors remain work evidence. Edition
  ID, title, language, ISBNs, cover, publisher and date come from one nested record.
  Work-wide ISBN/language/edition arrays never form a synthetic edition. Author
  work lists also stop inventing edition IDs, dates and requested media formats.
- Book ordering uses verified exact ISBN first, explicit title/author agreement
  and exact Google fallback next, then native provider position. Equal positions
  use the existing provider preference and stable IDs. No cover/popularity bonus
  or experimental Python weights enter the application.
- Cache hits rebuild the same ordering. Search merges require shared valid ISBN
  evidence or identical work/edition IDs; title/author resemblance is insufficient.
  Existing persistence/manual-override paths are unchanged. Legacy synthetic
  Google author IDs stay stable while textual comparisons gain Unicode support.
- Exact ISBN requests verify the selected edition (including ISBN-10/13
  equivalence). Known conflicting exact editions remain visible with review
  reasons. Malformed ISBN input returns a validation error before contacting
  providers. Edition keys are not accepted as work identities.
- Add New displays evidence rather than score percentages; requested format is
  never presented as observed format. Unknown formats/conflicts use the existing
  review-before-add flow. Edition artwork takes precedence, missing edition
  artwork falls back to the work image, and failed images get a placeholder.
  Partial provider failures remain visible alongside successful results.

## Checks

- `go test ./...`: passed.
- `go test -race ./...` with a new, disposable loopback Postgres 17 cluster:
  passed, including database integration tests. Metadata/API race tests were
  repeated after the final validation changes.
- `go vet ./...`: passed.
- Frontend Vitest: 28 tests passed. TypeScript/Vite production build: passed.
- Search browser suites: desktop 1440×1000 and mobile 390×844; edition evidence,
  image failure, unknown-format review, partial-source failure, saved identity,
  concurrent add protection and format preservation. Final result: 14 passed.
- `TestDiscoveryCapturedBenchmark`: **16/16 intended top results** through the
  real Go adapter, cache, normalization, filter, merge and sort pipeline, both
  cold and cached. Fixture transport enforces the actual 25-candidate request
  cap (10 for author/ISBN cases); it does not return the original 40 unbounded.
  The live historical baseline was 2/16. Python research weights are unused.
- Focused regressions cover work/edition isolation, unknown language/format,
  ISBN edition conflicts, explicit authors, Unicode, unchanged synthetic IDs,
  malformed IDs/ISBNs, and an exact coverless edition beating a wrong covered one.
- Fresh, read-only Open Library smoke checks through the changed Go code passed:

| Query | First work | Selected edition | Language | Observed media format |
| --- | --- | --- | --- | --- |
| percy jackson | The Lightning Thief / OL492658W | OL27284483M | eng | unknown |
| project hail mary | Project Hail Mary / OL21745884W | OL29597011M | eng | unknown |
| 9781423103349 | The Sea of Monsters / OL492646W | OL42335418M | eng | unknown |

The three upstream requests took approximately 0.51s, 0.63s and 0.06s during this
check, excluding intentional pacing. These are smoke observations, not a latency
SLO. The opt-in test is reproducible:

```sh
go test ./backend/internal/metadata -run TestDiscoveryCapturedBenchmark -v
LIBRARRY_LIVE_DISCOVERY_TEST=1 go test ./backend/internal/metadata \
  -run TestLiveOpenLibraryDiscovery -v -count=1
```

The first run of the new partial-provider browser assertion expected `alert`;
the warning correctly rendered with `status`. The assertion was corrected to
match the existing accessible warning component and the suite rerun. Browser
screenshots/traces live under ignored `output/playwright/` locally.

## Remaining qualification and scope

This implements the simpler baseline recommended by the
[ranking design](../metadata-ranking-design.md). It does not implement the entire
longer-term pipeline: no generic provider-ID query parser, automatic rescue query,
custom weighted quality/derivative classifier, series reading-order UI, or full
work-grouped edition chooser. Current known media formats remain separate results;
no series positions are inferred from titles or publication years.

The 16 diagnostic queries mostly cover popular English fiction. They are not a
production-accuracy estimate; the 50-query diverse/independently judged cohort is
still outstanding. Same-title records remain separate without verified shared
identity. Missing media format remains unknown and requires review. Hardcover
and Google live credential qualification and multi-provider ranking evaluation
remain pending. No claim of general cross-provider ranking quality is made from
an Open Library-only benchmark. Image packaging, main promotion and NAS rollout
were not part of this implementation/test step.
