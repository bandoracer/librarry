# Discovery edge cases — September 21, 2026

Ten additional diagnostic queries were declared before fetching responses. They
cover translated titles with author constraints, three misspellings, punctuation,
two intentionally nonexistent titles and two audiobook requests. This is not an
independently judged accuracy benchmark. The Hobbit's initially mistyped target ID
was corrected to `OL27482W`, matching the original frozen cohort's existing label.
No query or observed miss was removed.

`capture.py` retains raw responses, exact URLs, status codes and retrieval times.
The snapshots with `-rescue` show the previous work-title/author request; those
with `-author-rescue` use general `q` plus the author constraint. The production
adapter makes at most two requests, not all research branches. The additional
Parable snapshot requalifies the changed rescue against the expanded cohort.

Replay: `go test ./backend/internal/metadata -run 'TestDiscovery(CapturedEdgeCases|ExpandedBenchmark|CapturedBenchmark)' -v`

`results.json` comes from the actual Go adapter, normalization, filtering, merge,
ranking and cache path. Set `LIBRARRY_EDGE_REPORT_PATH` to a new output path to
write a fresh report. Assertions also check the complete request parameters.

- Six of eight positive-intent cases return the intended work first.
- `percy jakson` and `project hail maryy` return no candidates: **known retrieval
  misses**, retained explicitly in the report, not counted as successes.
- Both intentionally nonexistent queries return zero records without errors.
- `The Employees by Olga Ravn` previously returned no records: the work-title
  rescue could not match the translated edition of `De ansatte`. General `q`
  constrained by `author` retrieves the correct work. The author-suffix version
  already worked. The unconstrained `The Employees` query remains sixth in the
  previous cohort; this iteration does not claim to solve that ambiguity.
- Audiobook requests find the intended works but every Open Library edition stays
  **format unknown**. Retrieval success does not establish audiobook availability.

## Series source contract

The new Hardcover series query was validated with graphql-core against the
[official schema at commit 2eee2f8](https://github.com/hardcoverapp/hardcover-docs/blob/2eee2f8e6916a9157f384b3b7aba9648fdc85ae6/schema.graphql).
The schema defines `series`, `book_series`, `position`, `compilation`, and the
nested book/default-edition relationships. This proves query/schema compatibility,
not token permissions or real catalog quality. The provider's `featured` flag is
not assumed to mean primary reading order; only explicit compilation membership
is excluded. Numbered side stories can remain.

Local fixture tests cover numeric order (including zero and fractional positions),
null positions last, overlapping series, invalid membership, empty responses,
literal wildcard handling, missing credentials and cache replay. Browser tests
cover series mode and errors at desktop/mobile widths. Live provider-health readback
still reports Hardcover and Google Books as missing credentials. No authenticated
series, rich edition, or Google fallback claims are made.
