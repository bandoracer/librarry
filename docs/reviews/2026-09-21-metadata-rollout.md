# Metadata discovery release — September 21, 2026

## Candidate

Improved provider retrieval, coherent edition normalization, identity-safe merging,
explainable search evidence, edition selection, and explicit bounded series search.
No database migration is included. Prior benchmark and implementation records are
linked from [status](../status.md).

Credentialed qualification uncovered a live API restriction absent from schema
validation: Hardcover blocks `_ilike`. The corrected adapter uses its dedicated
Series search (three candidates), followed by explicit-ID membership lookup (25
non-compilation memberships each). Provider-featured records lead retrieval so
sparse duplicates do not exhaust the bounded window; the flag is not treated as
proof of a primary novel. Verified ebook/audio editions precede work-only records,
with provider positions preserved within each group. Catalog omissions and bad
membership flags remain visible evidence limitations.

## Read-only provider checks

- Hardcover authentication succeeds with the configured personal API key.
- Percy Jackson series search returns The Lightning Thief first, coherent
  ebook/audio editions, and source positions for subsequent volumes.
- Project Hail Mary title search returns Andy Weir's ebook/audio editions first.
- Ebook ISBN 9780593135211 returns the matching Project Hail Mary edition.
- Rick Riordan author search returns stable identity `hardcover-author:87224` first.
- Both Hardcover and Google Books pass the deployed application's credential checks.

## Qualification

All six jobs passed in [candidate run 35675737504](https://github.com/bandoracer/librarry/actions/runs/35675737504):

- Go vet and full PostgreSQL race suite.
- 29 frontend tests and production build.
- 151 browser tests passed, one skipped.
- Deployment configuration contracts and disposable real Calibre checks.
- Packaged imports, isolated restore, worker and notification checks.
- Candidate image scans under the repository's fixable HIGH/CRITICAL policy.
- AMD64 and ARM64 API/web publication.

Local verification also passed the full PostgreSQL race suite, 18 focused
search desktop/mobile checks, the original 16-case benchmark, six Python tests,
and diff/credential-leak checks. The first local full run exposed an existing
notification test race: PostgreSQL termination returned before lock release and
failure cleanup waited on the blocked receiver. The test now waits for termination
and always releases its receiver; the repeated suite and GitHub run both pass.

## Published and deployed identity

Application source: `ca6c8432689b69545448d3cb30879d9bc7b08e0a`.
[PR #54](https://github.com/bandoracer/librarry/pull/54) merged to main at
`f23c674e99d4923a14705e882631f28c161fb2fa`. The merge tree matches the qualified
source tree. Redundant PR/main verification runs were canceled; the explicit
candidate publication run above completed all gates on the deployed source.

| Component | Multiarchitecture index | Live AMD64 image config |
| --- | --- | --- |
| API | `sha256:f20994d2133eb9f40de6ee3314fe2951d746aa3ae9662cd33ad56a409181b701` | `sha256:984f7aea798180556c45ab944501ad6babeef26fbfeb235adf956470cda1a8e6` |
| Web | `sha256:85204d79a5dfadc3f9ea78774e6a575569be29105ebb898f3efa964761eabf45` | `sha256:5ee5b4423fd470f67dbb84ba1ef489a9c09bfa871bf0e8c4920399d5c8df8928` |

Use `ghcr.io/bandoracer/librarry-api@<API index>` and
`ghcr.io/bandoracer/librarry-web@<Web index>` for this immutable pair. Both runtime
image labels identify the application source above. The
[promotion run](https://github.com/bandoracer/librarry/actions/runs/35677049125)
verified both `latest` aliases against these indexes without rebuilding.

## NAS rollout and preservation

A private checkpoint at
`/mnt/HDD_pool/vault/app-config/librarry/releases/2026-09-21-metadata-012257/`
contains the previous configuration, a custom-format database dump validated with
`pg_restore --list`, file/link/override/migration snapshots, and three media hashes.
Credential-bearing files stay private on the NAS.

TrueNAS `app.update` replaced only API/web image references and exact configuration
readback matched the requested configuration. Provider credentials, mounts and
other settings were preserved. The app returned `RUNNING` and readiness passed.
The checkpoint retains the prior Kindle image pair for rollback; no schema change
was included or required.

At September 22, 01:46 UTC (September 21 local), live checks confirmed:

- Health and readiness HTTP 200; API reports source `ca6c843` and migration 0059.
- Hardcover and Google Books ready, reachable and authenticated after restart.
- `percy jackson` returns The Lightning Thief first with edition evidence and cover.
- Percy Jackson Series mode returns explicit positions with the first five novels
  in order and selectable English ebook/audio editions; no provider errors.
- Ebook ISBN 9780593135211 returns Project Hail Mary with Exact ISBN evidence.
- Browser-rendered cover artwork loads; changing The Lightning Thief from ebook
  to audiobook switches the complete edition ID, identifiers, publisher, format
  and default root folder. No book was added or release grabbed.
- All three stored file records and file/book links, manual overrides, migration
  records and media hashes exactly match the pre-deployment checkpoint.

Series results remain a bounded provider window. Unknown-language/format records
and misclassified collections can remain after verified editions. Missing positions
sort last within each verified/work-only group, not globally. The short UI helper
text could describe these qualifications more precisely. Broader catalog judging,
full enumeration and generic typo rescue remain future work.
