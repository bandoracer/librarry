# Current status

Last verified: September 17, 2026. This is the authoritative readiness summary.
Feature guides describe candidate behavior; historical records are evidence of
past checkpoints, not current deployment claims.

Librarry is early alpha. The stabilization candidate is qualified for a
**controlled rollout**, now deployed on the maintainer NAS. It is not yet
qualified as an unattended replacement for
an existing Readarr installation.

## Kindle delivery

Manual native EPUB/PDF delivery is deployed on the maintainer NAS. Settings,
a test document, and durable per-book history are available; see the
[Kindle guide](guides/kindle.md). The exact published API image passed isolated
Kindle checks on native AMD64 and ARM64. The configured Resend server accepted
one setup test on September 17 at 08:29 UTC. Physical Kindle receipt remains
unconfirmed. Uncertain attempts are never automatically retried.
[Implementation checks](reviews/2026-09-17-kindle-implementation.md) and
[rollout evidence](reviews/2026-09-17-kindle-rollout.md) record the boundaries.

## Source, images and deployment

| State | Verified position |
| --- | --- |
| Application candidate | `3d89a6358da7ad5064718c2e3f0c65d48021922b`, merged through [PR #53](https://github.com/bandoracer/librarry/pull/53) at main commit `173b0de7b9bd3616d0c4ae935d5e43f921700a98`. Later documentation-only commits do not change the runtime. |
| Published candidate | Paired API/web images from [run 35197792820](https://github.com/bandoracer/librarry/actions/runs/35197792820); immutable indexes and architecture manifests are recorded in the [Kindle rollout](reviews/2026-09-17-kindle-rollout.md). |
| Latest aliases | Still the previously promoted stabilization pair at source `6e209ff`, recorded in the [qualification report](reviews/2026-09-16-release-qualification.md#published-artifact-identity). The Kindle rollout did not move `latest`; use its recorded image pair to install this feature. |
| Live homelab | Kindle candidate API/web index digests, native AMD64 manifests; source `3d89a63`, database migration 0059. All three media files and existing file/book links were preserved. [Rollout evidence](reviews/2026-09-17-kindle-rollout.md). |
| Endpoints at last check | LAN `/healthz` and `/readyz`: 200. Cosmos target corrected; hostname returns 302 to existing Cosmos sign-in. |

Do not infer deployment from a merged commit or successful image build. Installer
defaults now select the promoted `latest` pair. Use both recorded immutable image
references when a deployment must remain pinned.

## Stabilization qualification (preceding Kindle rollout)

- Broader risk review of metadata, acquisition, imports, identity, workers,
  notifications, authentication and operator controls. Three UI defects found
  during the continued journey were fixed; no additional backend blocker was
  found within the documented review scope.
- All six candidate CI jobs passed: Go vet/Postgres race tests, 25 frontend tests,
  143 browser tests, build/deployment checks, disposable real Calibre, packaged
  recovery/restore, scans and both multiarchitecture builds.
- A fresh browser journey covered Forms login/restart, real English Open Library
  search/add, uncertain acquisition reconciliation without resubmission, EPUB and
  chapter imports, missing/foreign-file review, failure/restart recovery,
  rename/remove/restore and desktop/tablet/phone layouts.
- The exact published images passed the application, worker and notification
  suites on native ARM64 and native AMD64. Four exact-image scans found zero
  fixable HIGH/CRITICAL findings under the configured `--ignore-unfixed` policy.
- A sanitized production copy migrated 0029→0058, preserving all existing fields
  across nine tables, three exact file/book associations and three media hashes.
  Restoring the pre-upgrade snapshot and original image bytes passed rollback
  and reverted a deliberate copy-only mutation.

The [qualification report](reviews/2026-09-16-release-qualification.md) records
methods, digests and boundaries. Chapter bytes were synthetic, roots were
preconfigured, and client failure cases used controlled fixtures. These checks
do not establish audio playback, unattended operation or a real Readarr migration.

## Known gaps

1. The planned 72-hour/20-controlled-case observation period remains incomplete.
   It started September 17 at 00:21 UTC (September 16 locally). The owner
   explicitly authorized immediate `latest` promotion to replace the broken old
   build; observation is no longer a publication prerequisite. This decision
   does not turn unfinished observation into passed evidence.
2. Live Hardcover and Google Books credentials and rich-provider qualification.
   Their adapters have fixture coverage; Open Library has real-search evidence.
3. A real Readarr migration and consumer qualification. Compatibility remains
   partial, including persistent collision-free numeric identity mapping and
   non-book resource contracts.
4. Broader product work in the [stabilization backlog](stabilization-plan.md),
   including richer Calibre behavior, settings-source consistency and durable
   collection-wide operations. Implemented endpoints do not prove full parity.

Use the [release checklist](release-checklist.md) for gate evidence and the next
rollout steps. The owner requested the Kindle addition after stabilization;
other expansion remains subject to the documented release priorities.

## Current live deployment notes

These are maintainer homelab values, not install defaults:

- TrueNAS app: `librarry`; portal: `http://192.168.1.221:30200/`.
- Cosmos hostname: `https://librarry.borchetta.xyz/` (Cosmos sign-in required).
- Live images: immutable GHCR pair from the Kindle rollout report.
- A private pre-Kindle database/config/media checkpoint and the preceding
  stabilization image pair are retained for immediate rollback.
- Original `librarry-api:local` and `librarry-web:local` images and a fresh
  pre-upgrade database/config/media checkpoint are retained for rollback.
- Postgres: `/mnt/HDD_pool/vault/app-config/librarry/postgres`.
- Config mount: `/mnt/HDD_pool/vault/app-config/librarry/config:/config`.
- Media mount: `/mnt/HDD_pool/vault/media-stack:/data`.
- Roots: `/data/media/books/ebooks`, `/data/media/books/audiobooks`;
  downloads: `/data/torrents/books`; standard search language: English.

Prowlarr, qBittorrent and Open Library passed read-only checks after rollout.
All three existing books have present-file evidence after a successful scan;
media hashes and associations are intact. Automation settings were restored and
initial worker passes reported no errors or new grabs. Browser Library and
System checks passed. Do not publish live
credentials, database dumps or private library inventories in issues or commits.

## Historical evidence

- [Kindle rollout and SMTP acceptance](reviews/2026-09-17-kindle-rollout.md)
- [Live rollout and rollback checkpoint](reviews/2026-09-16-live-rollout.md)
- [September qualification](reviews/2026-09-16-release-qualification.md)
- [Initial audit](reviews/2026-09-15-audit.md) and [implementation ledger](reviews/2026-09-15-implementation.md)
- [Preserved status and milestone ledger](history/2026-09-16-status-ledger.md)
- [Historical Readarr parity audit](readarr-parity.md) and [UI audit](ui-backlog.md)

Historical claims can predate later fixes or stricter qualification. This page
and the release checklist take precedence for current readiness.
