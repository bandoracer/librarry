# First stabilization release

Prepared September 16, 2026. Feature expansion is frozen. This is the delivery
checklist; S01–S25 remains the longer-term backlog, not a reason to add more
features before shipping. No release version or stable image is approved yet.

## Release boundary

The original safety patch at PR #3 (`7771862`) has confirmed recovery gaps.
The [boundary review](reviews/2026-09-16-release-boundary.md) identifies later
fixes. The recommended qualification baseline is the existing stack at
`30f7ddefe7ec3e30e2d0c2e0ded61f97d2e7f013`, with publication isolation applied.
That is a candidate for review, not a declaration that all 48 drafts are ready
to merge. A smaller patch requires explicit backport selection and a narrower
supported surface; PR count is not a safety argument.

Use the existing PR boundaries as review units. Avoid unrelated refactors,
new provider features, broader compatibility work, or new download-client controls.
Only release blockers, workflow polish, regression fixes, and qualification work
belong in the release candidate. Every change invalidates affected evidence.

## Gates

| Gate | Required evidence | Current state |
| --- | --- | --- |
| 1. Publication isolation | PR/main/tag/default-dispatch builds cannot publish; explicit candidate runs produce paired source/digest records; stable aliases unchanged | PR #51 merged as `4d5bc4c` after publication policy and both image builds passed; candidate integration preserves the guard; explicit candidate publication has not been exercised |
| 2. Reviewed candidate | Exact source SHA and migration range; safety/auth/import review; known limitations; all candidate CI checks green | Frozen application baseline `e2f99c0`, migrations 0030–0058; all six CI jobs green. [Focused safety and latest-fix review](reviews/2026-09-16-candidate-code-review.md) found no new blockers; review of the remaining stack stays open |
| 3. Complete operator journey | Fresh database, setup, English search, add, controlled ebook and chapter acquisition/import, deliberate failure and restart recovery; inspect desktop, mobile and tablet | Fresh browser/Open Library search/add and setup checks completed; two feedback/accessibility defects fixed. Controlled full lifecycle and complete setup/UX review remain open |
| 4. Safe upgrade and rollback | Restore a sanitized pre-upgrade database/media/config copy; compare books, overrides, links and hashes; rehearse rollback using the original images and pre-upgrade snapshot | Prior disposable restores exist; production-copy rehearsal and rollback open |
| 5. Exact artifact qualification | Publish candidates explicitly; record paired manifest/architecture digests; pull and test those digests on supported architectures; scan those images | Not started for a release candidate; build success alone does not prove ARM64 runtime or exact-artifact qualification |
| 6. Controlled target rollout | Named target, backup and rollback record, real integration readback, media permissions/mount checks, health and reverse-proxy checks | Open; no September live deployment claimed |
| 7. Observation and stable promotion | Timed operator log, no unresolved integrity/recovery errors, reproducible release notes; promote qualified digests without rebuilding | Open; expanded stabilized release retains the planned 72-hour/20-controlled-case gate |

## One operator journey

Use a disposable namespace with workers unable to grab external releases until
the controlled case explicitly enables them. Do not reuse the live database or
download directory. Keep fixtures legal and record the exact source of each.

1. Start with an empty database. Visit every main route, confirm zero-row lists,
   configure authentication and roots, restart, and verify saved settings and
   environment-owned fields. Invalid credentials and settings must fail clearly.
2. Search in English, distinguish an empty result from a provider failure, inspect
   provenance, and add a selected edition. Adding it again must retain identity,
   overrides, monitoring and destination rather than create or reset a book.
3. Submit a controlled single EPUB/M4B and a chapter set through a test client.
   Verify the exact inventory, bytes, wanted state, and complete library file set.
   Foreign siblings and incomplete chapters must stay out of successful imports.
4. Interrupt import, inject persistence failure, and restart. Review and resume
   the saved operation through the UI. Preserve originals until verified commit;
   uncertain acquisition must not cause a second submission. Verify cleanup
   eligibility and failure visibility separately from a successful import.
5. Rename, remove tracking, and restore tracking. Confirm expected media hashes,
   overrides and associations; changes to tracking must not silently delete bytes.
6. Exercise keyboard navigation, narrow phone, tablet, desktop, deep links,
   loading, empty, error and recovery states. Record screenshots and route sizes.
   Mocked browser responses and real API observations must be labeled separately.

## External qualification inputs

Authenticated NAS access and a restorable pre-upgrade snapshot are needed for
the target rollout. Rich Hardcover/Google claims need actual provider credentials;
Open Library plus controlled provider fixtures can cover the initial local
journey. Readarr migration claims require an appropriate source copy and a real
consumer. Missing evidence narrows release claims; it is not a passed test.

Before merging a stabilization PR, preserve the publication policy from gate 1.
PR #3 and descendants currently carry older workflow changes; resolve their
workflow merge conflicts explicitly and rerun publication tests and actionlint.
Do not silently restore automatic `latest` publication while integrating the stack.
