# Current status

Last verified: September 16, 2026. This is the authoritative readiness summary.
Feature guides describe candidate behavior; historical records are evidence of
past checkpoints, not current deployment claims.

Librarry is early alpha. The stabilization candidate is qualified for a
**controlled rollout**, now deployed on the maintainer NAS. It is not yet
qualified as an unattended replacement for
an existing Readarr installation.

## Kindle implementation (not deployed)

The Kindle feature branch adds manual native EPUB/PDF sends, SMTP settings,
a test document, and durable per-book delivery history. See the [Kindle guide](guides/kindle.md).
SMTP acceptance is distinguished from Kindle arrival; uncertain attempts are
never automatically retried. [Verification](reviews/2026-09-17-kindle-implementation.md)
includes Postgres race tests and desktop/mobile browser checks against a local
TLS mail receiver. No physical Kindle receipt was tested. This does not change
the live image identities below.

## Source, images and deployment

| State | Verified position |
| --- | --- |
| Application candidate | `6e209ffd8d3724879e52187e27790029e66af986`, merged through [PR #52](https://github.com/bandoracer/librarry/pull/52) at main commit `e742210d5c4b6a422ce60b5dc86f08e2fd93e99c`. Later documentation-only commits do not change its runtime qualification. |
| Published candidate | Paired API/web images from [run 35157133068](https://github.com/bandoracer/librarry/actions/runs/35157133068), with immutable index and AMD64/ARM64 digests in the [qualification report](reviews/2026-09-16-release-qualification.md#published-artifact-identity). |
| Latest aliases | Owner-authorized promotion points both `latest` tags at the exact qualified index digests above, without rebuilding. Automatic main/PR/tag builds still cannot publish; promotion is a separate manual workflow. |
| Live homelab | Qualified candidate API/web index digests, running their native AMD64 manifests; application source `6e209ff`, database migration 0058. [Rollout evidence](reviews/2026-09-16-live-rollout.md). |
| Endpoints at last check | LAN `/healthz` and `/readyz`: 200. Cosmos target corrected; hostname returns 302 to existing Cosmos sign-in. |

Do not infer deployment from a merged commit or successful image build. Installer
defaults now select the promoted `latest` pair. Use both recorded immutable image
references when a deployment must remain pinned.

## Completed qualification

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
rollout steps. Feature expansion remains frozen during this release effort.

## Current live deployment notes

These are maintainer homelab values, not install defaults:

- TrueNAS app: `librarry`; portal: `http://192.168.1.221:30200/`.
- Cosmos hostname: `https://librarry.borchetta.xyz/` (Cosmos sign-in required).
- Live images: immutable GHCR candidate pair from the qualification report.
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

- [Live rollout and rollback checkpoint](reviews/2026-09-16-live-rollout.md)
- [September qualification](reviews/2026-09-16-release-qualification.md)
- [Initial audit](reviews/2026-09-15-audit.md) and [implementation ledger](reviews/2026-09-15-implementation.md)
- [Preserved status and milestone ledger](history/2026-09-16-status-ledger.md)
- [Historical Readarr parity audit](readarr-parity.md) and [UI audit](ui-backlog.md)

Historical claims can predate later fixes or stricter qualification. This page
and the release checklist take precedence for current readiness.
