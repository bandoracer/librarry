# Concurrent search downloads — September 21, 2026

Search downloads now run independently per verified book identity and target
format. Completion keeps search open, while each row reports progress or outcome.
Alternate editions and verified aliases share duplicate protection. Mobile details
close on submission so another row is immediately accessible. Failed saved actions
recover through **Open book** without replaying an uncertain request.

## Release

- Source: `d66e29f81326a746ffe487d479d4caa1e879a7f4`.
- [PR #56](https://github.com/bandoracer/librarry/pull/56), merged at
  `7416e65cf0891984c139aff9047a7429b25ec89a`.
- [Candidate qualification](https://github.com/bandoracer/librarry/actions/runs/35689933247): all six jobs passed.
- [Paired latest promotion](https://github.com/bandoracer/librarry/actions/runs/35691323606): passed.

| Component | Published index | Running AMD64 image |
| --- | --- | --- |
| Api | `ghcr.io/bandoracer/librarry-api@sha256:80975e8da3ddadd0c172b0c1d10d7b577beb67db4b2658da314ab4f7ebaf3c7e` | `sha256:f719970e252dfa51e84f50b8a21cc932cad8d37c59641fc0c56ff6c52d253d05` |
| Web | `ghcr.io/bandoracer/librarry-web@sha256:741181d9729e136d85e0eefd2e329a4faf07b2465226a21cf66069cb6936e8b8` | `sha256:876946b86dc411a744911ffbaee5046b46c16e1f8667257eb76c023a5d85759f` |

The NAS is pinned to these immutable image references. Both runtime image labels
and the system-status endpoint report the qualified source. Health and readiness
return 200. The schema remains `0059_kindle_delivery.sql`; this change adds no
backend behavior or database migration.

## Verification

- Full local Go suite against disposable PostgreSQL, 35 frontend tests,
  production build and 46 targeted desktop/mobile browser checks passed.
- Concurrency tests hold saves and release searches open, finish different books
  out of order, and fail one while another succeeds. They verify independent
  format/destination snapshots, correct release IDs, approval enforcement,
  same-work duplicate protection and recovery with stale identity responses.
- CI passed Go vet, PostgreSQL race tests, 35 frontend tests, the production
  build, 175 browser tests with one existing skip, and deployment contracts.
  Calibre, packaged imports/recovery/restore, workers, notifications and the
  configured fixable HIGH/CRITICAL vulnerability gate passed. Images were built
  for AMD64 and ARM64; promotion verified both aliases and index membership.
- Live browser readback shows queued indicators on three previously started
  Percy Jackson books. An unqueued book still offers **Download ebook**, while a
  queued book offers **Open book**. The live list and local desktop/mobile
  concurrency screenshots were visually inspected.
- All acquisition requests in concurrency tests were intercepted. Live checks
  were read-only and did not start additional downloads.

## Preservation

A private configuration/database checkpoint and media hashes were recorded before
rollout; the database dump validated. Only API/web image references changed in the
saved NAS configuration. All three file records, three file/wanted links, manual
overrides, 59 migration records and three media hashes matched after rollout.
The prior image pair and checkpoint remain available for rollback.

In-flight UI phases are local to the mounted search page. After reloading, rows
use saved library state, refreshed every 30 seconds. The broader operational
qualification limits in [status](../status.md) remain unchanged.
