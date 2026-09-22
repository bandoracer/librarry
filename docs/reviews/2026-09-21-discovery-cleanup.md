# Discovery cleanup after the live review

Source candidate only; neither this change nor the simplified download UI has
been deployed. The NAS remains on the preceding metadata rollout.

## Verified problems

Sixteen live searches found the intended book/series first in all but the DDIA
case. The DDIA top record had no author, cover or coherent edition; its real ebook
was third. The typo `project hail marry` put King Lear second. Summaries, workbooks
and empty catalog records competed with the normal acquisition choices. A bad
Hardcover default-edition relationship discarded its entire contribution to
The Way of Kings search.

## Changes

- Empty author records move behind usable candidates. Coherent digital editions
  with a directly matching title or subtitle variant get priority. Arbitrary
  keyword overlap is not enough to promote an edition.
- A coherent title/author result can anchor the main list. Related authors,
  summaries, workbooks and labeled companion material remain in an expandable
  section. Topic searches without a credible anchor retain provider relevance.
- Series keeps provider positions. Unnumbered supplements and other matching
  series are secondary; sparse records stay accessible separately.
- UI grouping accepts verified work aliases as well as the same typed work ID.
  It never infers shared identity from matching titles/authors; selected edition
  records and overlapping series memberships remain intact.
- Interactive Hardcover search excludes malformed editions while retaining
  independent healthy records, sibling editions, or validated work-only evidence.
  Partial responses report the problem and are not cached as complete. Exact ISBN
  lookup and automated bibliography validation remain strict.

The additional searches exposed an interim regression: a verified ebook of
The Amazing Maurice and His Educated Rodents outranked Educated. Tightening title
focus and retaining the healthy edition of the intended Hardcover work corrected
it. That case is no longer treated as an untouched holdout.

## Validation

- All 24 fresh live searches returned the intended book/series first. The eight
  additional queries were Mistborn, Foundation, Sapiens, Educated, Piranesi,
  Pride and Prejudice, The Stand and Dune Messiah. All used English; the explicit
  audiobook and ISBN checks retained their requested identity/format behavior.
- DDIA's complete English ebook moved from third to first. King Lear remained
  secondary for `project hail marry`. Percy Jackson's primary series list contains
  seven unique numbered novels (ebook/audio choices), in source order 1 through 7.
- The Way of Kings and Educated now return their intended Hardcover editions while
  reporting partial invalid-edition warnings. Those malformed records were not
  silently accepted. All other final probes had no provider errors.
- Existing 50-query captured-provider benchmark and the new 16-case normalized
  live cohort regressions passed. The historical unconstrained Open Library-only
  Employees gap is still explicitly allowed in that old corpus; the credentialed
  combined-provider live query returned the intended book first.
- Full Go suite against disposable PostgreSQL, metadata race tests, 33 frontend
  unit tests, production web build, 42 desktop/mobile browser checks, deployment
  configuration checks and `git diff --check` passed.
- An initial unpaced diagnostic run hit Hardcover quota limits. Final live probes
  used the production request-budget transport plus spacing and completed without
  quota errors. No library items or downloads were created.
Captured inputs and candidate responses are in
[the live review cohort](../../research/metadata-ranking/live-review/README.md).

## Remaining limits

These are diagnostic searches, not a catalog-wide accuracy claim. Cross-provider
records without a verified shared identity still appear separately. Adaptations
or bundles with no recognizable title marker/source evidence can remain in the
main list. Short ambiguous queries and topic searches still rely heavily on
provider relevance. Incomplete records are separated, not repaired or assigned
an invented language/format. Series remains a bounded provider view.
