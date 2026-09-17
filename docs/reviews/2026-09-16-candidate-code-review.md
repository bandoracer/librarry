# Candidate code review

Reviewed September 16, 2026. Application commit:
`e2f99c0b35cde4766e5adf9dca0a7f7013b4ae82`, on
`codex/stabilization-candidate`. Comparison base: main at
`4d5bc4ca159d62b711b9c818941fb96e2fb311a8` (publication isolation).

## Verdict and scope

No new blocking defect was found in the paths reviewed below. The candidate is
suitable to push and retain as the frozen qualification baseline. This is a
focused review of the highest-risk boundaries and the latest fixes, not a
line-by-line approval of all 408 changed files or authorization to deploy.
The earlier stacked drafts remain available as review units.

- Authentication: startup rejects failed persisted configuration reads;
  requested authentication does not fall back to open access on persistence
  failure. Configuration, credentials and session revocation commit atomically.
  Session creation checks the credential hash under the same configuration lock.
  Reviewed the rollback and stale-login regression tests with these paths.
- Native file recovery: imports journal staging paths before creating them;
  publication checks lease ownership under a database lock. Rename commits keep
  original media until the destination and database identity have committed.
  Cleanup rechecks destination bytes and ownership. The database-failure rename
  regression verifies retained source bytes, restart recovery and stable identity.
- Acquisition: an uncertain client response retains the intent. Subsequent
  attempts reconcile the original client instead of sending a second add;
  accepted receipts are persisted before downstream download bookkeeping.
- Compatibility collections: selections resolve exact identities and reject
  ambiguous numeric aliases; bulk writes validate the complete selection under
  transaction locks. Paging shares the native evidence projection.
- Migrations: all 29 migrations from 0030 through 0058 are additions; migrations
  already on main are unchanged. The 0058 replacement function preserves 0043's
  evidence rules while bounding the linked-file lookup before the format filter.
  The cold-statistics regression checks work performed as well as page contents.
- Latest UI fixes: release and grab errors retain server explanations, including
  uncertain-acceptance instructions. No automatic retry was added. The collapsed
  theme control has an explicit accessible name.
- Publication: the combined workflow retains PR #51's default build-only policy.
  Main pushes, tags and ordinary dispatches cannot publish images or move stable
  aliases. Explicit candidate publication still requires all qualification jobs.

## Verified CI evidence

[Run 35150888617](https://github.com/bandoracer/librarry/actions/runs/35150888617)
completed successfully on the exact application commit above. All six jobs passed:

1. Publication policy: four tests covering the event/input matrix.
2. Go vet, Postgres race tests, frontend build, 23 frontend unit tests,
   143 browser tests and deployment configuration contracts.
3. Disposable Calibre contract, including Basic and Digest authentication.
4. Packaged import, isolated restore, worker and notification fixtures, followed
   by the configured image vulnerability scan.
5. API image build for linux/amd64 and linux/arm64.
6. Web image build for linux/amd64 and linux/arm64.

The GHCR login, published-digest recording and candidate identity upload steps
were skipped in both image jobs, as expected for this build-only run. There was
no image publication. Later documentation-only commits do not change the tested
application, migrations, tests or workflows.

## Outstanding release evidence

This section records the earlier `e2f99c0` checkpoint. The subsequent
[release qualification report](2026-09-16-release-qualification.md) records the
broader review, continued operator journey, published candidate identities and
production-copy upgrade/rollback for `6e209ff`.

The [release checklist](../release-checklist.md) remains in force. Combined review
of the remaining stack, a complete browser-driven acquisition/import/recovery
journey, first-run usability, a production-copy upgrade and rollback rehearsal,
exact published-image qualification on supported architectures, and a controlled
rollout with observation remain open. Green builds do not demonstrate those
outcomes. No live NAS deployment, stable promotion or Readarr replacement claim
is made by this review.
