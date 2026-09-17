# Controlled NAS rollout

September 16, 2026 (September 17, 00:19–00:25 UTC).

PR #52 merged into main at `e742210d5c4b6a422ce60b5dc86f08e2fd93e99c` after
all six checks passed in [run 35164850694](https://github.com/bandoracer/librarry/actions/runs/35164850694).
Application source remains `6e209ffd8d3724879e52187e27790029e66af986`;
subsequent candidate commits changed documentation only.

## Deployed artifacts

TrueNAS app `librarry` now pins both immutable index references from the
[qualification report](2026-09-16-release-qualification.md#published-artifact-identity).
The NAS runs their native AMD64 manifests. Images were pulled by digest and OCI
source labels verified before the change; the tested images were not rebuilt.

- API: `ghcr.io/bandoracer/librarry-api@sha256:5b42f0e33c465026e4105a740baba72202df606389e614d150ac1e8da2334cd2`
- Web: `ghcr.io/bandoracer/librarry-web@sha256:a8c9ea2acae6fa155c73715aa4be2e7b3ce8cb42dc7458795f16be9ce2cdf4db`

The source and image manifest are distributed as the
[September stabilization prerelease](https://github.com/bandoracer/librarry/releases/tag/stabilization-2026-09-16). Historical
`latest` aliases remain unchanged pending observation and stable promotion.

## Backup and upgrade

API/web writers were stopped before a fresh custom-format PostgreSQL dump,
config archive, complete ebook/audiobook-root archive, media hash manifest and
export of the original image bytes. The dump's archive directory was checked
with `pg_restore --list`. Private artifacts and the original TrueNAS Compose
configuration are retained with restricted permissions under:

`/mnt/HDD_pool/vault/app-config/librarry/releases/2026-09-17-stabilization/`

The private `rollout.py rollback` helper restores the original database snapshot,
media/config archives and original app configuration. Rollback is a snapshot
restore, not a reverse migration; stop writers and evaluate post-upgrade changes
before using it. A separate production-copy rehearsal already exercised the
snapshot/original-image rollback path. Production itself was not rolled back.

TrueNAS `app.update` replaced only the application images and temporarily paused
acquisition/import automation for migration verification. The database migrated
from 0029 to 0058. Before automation resumed, every pre-existing column value
matched across nine tables: wanted items (6), files (3), manual overrides (0),
provider records (23), works (6), editions (6), downloads (130), roots (2) and
compatibility resources (1). All three media sizes/hashes and exact file/book
links matched. The API user could write and hard-link in both library roots and
its config mount. Original environment settings were then restored. TrueNAS recreated the Postgres
container too; its image and persistent mount remained unchanged.

## Live readback

- API reports source `6e209ff` and migration 0058; `/healthz` and `/readyz` return 200.
- Prowlarr, qBittorrent and Open Library passed explicit read-only connection checks.
- A scan of the ebook root completed with three files and zero missing files.
  Library now reports three Downloaded, zero Unknown and three recorded files.
  Scanning updated presence evidence; it did not modify media bytes.
- The browser Library view loaded the existing records after restart. All 13
  workers remained visible; initial acquisition/import passes reported zero
  errors, zero grabs and zero removals. This is initial evidence, not sustained
  unattended-operation qualification. The scheduled backup worker also created
  a new dump successfully, and the next health pass reported zero warnings/errors.
- The existing Cosmos upstream was malformed. Its target was corrected to the
  LAN portal and existing Cosmos authentication enabled for this route only.
  The hostname now returns 302 to Cosmos sign-in; the LAN application retains
  its existing authentication mode. Other Cosmos configuration was unchanged.
  The repair used Cosmos's documented [configuration-file recovery procedure](https://cosmos-cloud.io/docs/issues-and-troubleshooting/).
  Its original configuration is retained privately on the proxy host.

## Remaining gate

The 72-hour/20-controlled-case observation window begins September 17 at
00:21 UTC. No stable alias was promoted. Passing time is insufficient without
the operator case log and no unresolved integrity/recovery errors. Rich-provider
credentials and real Readarr migration/consumer qualification remain open.
See [current status](../status.md) and the [release checklist](../release-checklist.md).
