# Stabilization release boundary review

September 16, 2026. Reviewed baseline: `main` at
`6c1f32ed1c916e8077040f9ded3a44d1bd07868e`; original safety PR #3 at
`7771862d493696fe1c4e7ccaecb52192ad73a426`; stack tip PR #50 at
`30f7ddefe7ec3e30e2d0c2e0ded61f97d2e7f013`.

## Decision

Do not ship PR #3 unchanged. Its maintained Go/Postgres/race suite passes, but
later recovery fixes address reachable paths in that release boundary. Extracting
a smaller patch remains possible, but would require deliberate backports or
disabling unsupported mutation paths. It cannot be achieved just by merging the
first draft and assigning a patch version.

Prefer qualifying the existing stack with feature expansion frozen. This does
not approve the stack wholesale: review the existing PR units, integrate the
publication guard, and qualify the exact resulting candidate using the
[release checklist](../release-checklist.md). Version and release claims follow
the qualified scope, not the originally proposed 0.4.2 label.

## Safety dependencies found

| PR #3 behavior | Consequence | Later repair to review |
| --- | --- | --- |
| Completed imports use anonymous `.librarry-import-*` / `.librarry-copy-*` temporary files and an in-memory defer for cleanup | Process death can leave unowned stages; the durable import plan cannot identify and reclaim them | PR #5 (`27223cd`): per-file staging journal and lease-aware publication |
| Native manual import/replacement publishes bytes before file/wanted writes without a durable operation | Persistence failure or restart can leave a new destination or previous copy without a recoverable saved plan | PR #6 (`8dcd0c1`): durable manual/replacement plans and guarded cleanup |
| `applyRename` moves the original before `UpdateFile` | A failed DB write leaves the recorded path missing while bytes exist at an unrecorded new path | PR #32 (`e3ce314`): durable rename; **fault reproduced below** |
| Grab calls the remote client before durable acceptance/intent | An acknowledgement or DB failure can leave an accepted download uncertain; a retry has no durable submission barrier in this boundary | PR #7 (`7443a90`) and #12 (`ea7587f`): intent/reconciliation and bookkeeping recovery |
| Calibre handoff follows the legacy direct-upload path | Native local receipts cannot certify the remote operation or make its retry safe | PR #34 (`b900575`) and #35 (`8a0f8f6`): real server contract and durable handoff; otherwise exclude this workflow from the release |

This is a targeted release-boundary review, not an exhaustive security review or
proof that every remaining draft is required. Chapter-set imports, scans,
author policies, compatibility APIs and UI behavior still need the combined
review and journey gate.

## Reproduced rename fault

In an isolated checkout of PR #3, create a temporary `Before.epub`, persist its
file record, and add a disposable DB constraint rejecting a new `After.epub`
path. Invoke `applyRename` with the original record and the new destination.
The call returns the injected persistence error. Observed:

```text
source missing=true, destination present=true, database still points to source=true
FAIL: rename removed the recorded source before its new path was persisted
```

The fixture uses a unique database created/dropped by `testdb.Open` and a temporary
media directory; no production data is involved. Its source is preserved in
[fixtures/pr3-rename-review_test.go.txt](fixtures/pr3-rename-review_test.go.txt).
To reproduce, copy it to `backend/internal/library/release_review_test.go` in
an isolated PR #3 worktree, set `LIBRARRY_TEST_DATABASE_URL` to disposable
Postgres, and run:

```bash
go test ./backend/internal/library -run TestReleaseReviewRenameKeepsSourceWhenPersistenceFails -count=1 -v
```

Failure is expected at PR #3. Remove the copied test afterward. The stack tip's
maintained `TestRenameCommitFailureRetainsSourceAndResumes` passes and covers
both retaining the original after failed commit and successful retry.

## Fresh evidence from this review

- PR #3: full `go test -race -timeout 10m ./...` passed against disposable
  Postgres. This does not include the newly injected rename regression above.
- Stack tip: five focused library recovery tests passed with the race detector:
  manual move after DB failure, manual replacement retention, staged-copy restart
  recovery, stale-worker publication fencing, and rename failure/retry.
- Publication guard: the directly executed policy covers 18 event/input
  combinations. Only an explicit candidate dispatch can publish. Full source
  identity, distinct rebuild tags and malformed-input rejection are checked.
  `actionlint` validates the workflow.

The local tests do not qualify real providers, clients, NAS mounts, ARM64/AMD64
runtime parity, a production-copy upgrade, or a live rollback. No application
deployment, release tag or image publication occurred during this review.

## Publication isolation

The existing main workflow publishes on main pushes, version tags and manual
runs; its metadata rules can move `latest`. An independent main-based guard
changes automatic runs to build validation and allows only explicit candidate
publication. Candidate tags contain full source SHA, run ID and attempt, and
the build exports image digests for subsequent qualification.

The guard does not add stable promotion. In particular, native images tested
before a later multi-platform rebuild are not the same qualified artifacts.
Stable promotion must use the exact digests actually exercised by the release
qualification. Installer defaults remain on their historical images until that
promotion; users can opt into digest-pinned candidate testing separately.

When integrating older draft branches, preserve this policy and rerun its tests.
A stack merge must not restore the old publication conditions or metadata aliases.
