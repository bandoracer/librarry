# Current status

Last verified: September 16, 2026. This is the authoritative readiness summary.
Feature guides describe candidate behavior; historical records are evidence of
past checkpoints, not current deployment claims.

Librarry is early alpha. The stabilization candidate is qualified for a
**controlled rollout**. It is not yet qualified as an unattended replacement for
an existing Readarr installation.

## Source, images and deployment

| State | Verified position |
| --- | --- |
| Application candidate | `6e209ffd8d3724879e52187e27790029e66af986`, consolidated in [PR #52](https://github.com/bandoracer/librarry/pull/52). Later documentation-only commits do not change its runtime qualification. |
| Published candidate | Paired API/web images from [run 35157133068](https://github.com/bandoracer/librarry/actions/runs/35157133068), with immutable index and AMD64/ARM64 digests in the [qualification report](reviews/2026-09-16-release-qualification.md#published-artifact-identity). |
| Stable aliases | Both historical `latest` digests were unchanged by candidate publication. Main/PR/tag builds cannot publish; candidate publication requires explicit dispatch. |
| Live homelab | Original local API/web images, reported version 0.4.0, database migration 0029. The candidate has not been deployed there. |
| Endpoints at last check | LAN `/healthz`: 200. Cosmos hostname: 502; proxy repair remains open. |

Do not infer deployment from a merged commit or successful image build. Installer
defaults still select historical `latest`; candidate testing requires selecting
both recorded candidate image references.

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

1. Controlled live rollout: current backup/rollback record, actual mount and
   permission checks, provider/client readback, and Cosmos proxy repair.
2. The planned 72-hour/20-controlled-case observation period before stable
   promotion. Qualified digests must be promoted without rebuilding.
3. Live Hardcover and Google Books credentials and rich-provider qualification.
   Their adapters have fixture coverage; Open Library has real-search evidence.
4. A real Readarr migration and consumer qualification. Compatibility remains
   partial, including persistent collision-free numeric identity mapping and
   non-book resource contracts.
5. Broader product work in the [stabilization backlog](stabilization-plan.md),
   including richer Calibre behavior, settings-source consistency and durable
   collection-wide operations. Implemented endpoints do not prove full parity.

Use the [release checklist](release-checklist.md) for gate evidence and the next
rollout steps. Feature expansion remains frozen during this release effort.

## Current live deployment notes

These are maintainer homelab values, not install defaults:

- TrueNAS app: `librarry`; portal: `http://192.168.1.221:30200/`.
- Cosmos hostname: `https://librarry.borchetta.xyz/` (502 at last verification).
- Original images: `librarry-api:local` and `librarry-web:local`.
- Postgres: `/mnt/HDD_pool/vault/app-config/librarry/postgres`.
- Config mount: `/mnt/HDD_pool/vault/app-config/librarry/config:/config`.
- Media mount: `/mnt/HDD_pool/vault/media-stack:/data`.
- Roots: `/data/media/books/ebooks`, `/data/media/books/audiobooks`;
  downloads: `/data/torrents/books`; standard search language: English.

Prowlarr search and qBittorrent handoff have prior live evidence. That evidence is
separate from the newer candidate's isolated qualification. Do not publish live
credentials, database dumps or private library inventories in issues or commits.

## Historical evidence

- [September qualification](reviews/2026-09-16-release-qualification.md)
- [Initial audit](reviews/2026-09-15-audit.md) and [implementation ledger](reviews/2026-09-15-implementation.md)
- [Preserved status and milestone ledger](history/2026-09-16-status-ledger.md)
- [Historical Readarr parity audit](readarr-parity.md) and [UI audit](ui-backlog.md)

Historical claims can predate later fixes or stricter qualification. This page
and the release checklist take precedence for current readiness.
