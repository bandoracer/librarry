# Expanded discovery cohort

34 additional diagnostic queries captured September 21, 2026: nonfiction,
programming, cookbooks, poetry, translated fiction, classics, and explicit Spanish,
French and Swedish preferences. Combined with the original 16, there are 50 query
intents (not 50 unrelated authors or statistically sampled production searches).

`cases.json` declares the query and intended author before retrieval. The same
researcher assigned positive work IDs after inspecting catalog responses; these
are **not independently judged or blind production-qualification labels**.
Author/publisher corroboration is included for selected identities and the newly
found failures. Other listed sources are direct Open Library work records.
The untranslated/transliterated work title can differ from the selected edition.
Additional valid duplicate records remain unjudged, not automatically incorrect.

Snapshots are read-only and not overwritten. Each has URL, UTC time and status.
`baseline.json` records the prior local Go implementation on this new cohort;
`improved.json` records author-agreement plus bounded structured rescue. These are
not measurements of the deployed NAS app.

| Version | Intended first | In first 10 | MRR@10 |
| --- | ---: | ---: | ---: |
| Prior local implementation | 31/34 | 33/34 | 0.9314 |
| Current implementation | 33/34 | 34/34 | 0.9755 |

Original cohort remains 16/16 first. Combined current result: **49/50 first,
50/50 in the first ten**. `The Employees` is still sixth; its known-gap label
permits the regression test to pass only while the result stays within the first
six. It is not counted as a Top-1 success. The independently judged, diverse
production-qualification gate remains outstanding despite reaching 50 queries.

An attempted exact-title promotion requiring a valid edition ISBN and author ID
is retained in `rejected-title-promotion.json` (new-cohort output). It regressed
five original cases: Percy Jackson, Earthsea, Pride and Prejudice, Watership Down,
and Pern. It also disrupted translated/classic editions in the expansion. That
rule was removed; record completeness alone does not establish intended identity.

The current rescue is only used for explicit `title by author` queries when broad
results contain no matching title/author record. `parable-rescue.json` captures
that second request. There is at most one rescue per search, under the existing
provider rate limits; no hardcoded benchmark query/author/ID is in runtime code.
A failed rescue retains usable primary results with a visible provider error and
is not cached as a complete response. Plain queries still use one request.

Reproduce the current application assertions:

```sh
go test ./backend/internal/metadata -run 'TestDiscovery(Expanded|Captured)Benchmark' -count=1 -v
```

To record a fresh report from the frozen fixtures, set
`LIBRARRY_DISCOVERY_REPORT_PATH` to a new output file. `LIBRARRY_REPORT_DISCOVERY=1`
permits exploratory top-result failures while still recording actual target ranks;
do not use it to claim a passing acceptance check. The captured baseline/rejected
reports are historical outputs, not automatically regenerable from the final code.
`manifest.json` fixes the corpus/labels/report bytes for audit.
