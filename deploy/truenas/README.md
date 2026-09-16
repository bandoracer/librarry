# TrueNAS SCALE Install

Librarry is distributed as two application images plus Postgres:

- `ghcr.io/bandoracer/librarry-api`
- `ghcr.io/bandoracer/librarry-web`
- `postgres:16-alpine`

Use [install.yaml](install.yaml) as a TrueNAS Custom App compose template.

## Before Install

Create or choose:

- a persistent app dataset for Postgres, for example
  `/mnt/tank/apps/librarry/postgres`;
- a persistent app config dataset for backups, for example
  `/mnt/tank/apps/librarry/config`;
- a shared media/download dataset that Librarry can see as `/data`, for example
  `/mnt/tank/media-stack`;
- ACLs that let the API container read scans and read/write completed imports.

The template runs the API as `568:568`, the common TrueNAS SCALE apps user/group.
Replace the `user:` value if your media dataset is owned by a different UID/GID.

## Template Values

Before installing, replace the generic placeholders with values for your NAS:

- `truenas.local`: your TrueNAS hostname or LAN IP.
- `/mnt/tank/apps/librarry/postgres`: a persistent app dataset for Postgres.
- `/mnt/tank/apps/librarry/config`: a persistent app config dataset mounted as
  `/config`, including scheduled pg_dump backups under `/config/backups`.
- `/mnt/tank/media-stack`: the host media/download dataset mounted as `/data`.
- `change-me`: a real Postgres password, and the matching password inside
  `LIBRARRY_DATABASE_URL`.
- provider, Prowlarr, qBittorrent, Transmission, or SABnzbd credentials when you
  want those integrations enabled at startup.

Inside the API container, the default book paths are:

```dotenv
LIBRARRY_BOOK_TORRENT_ROOT=/data/torrents/books
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
```

Make sure those paths correspond to the same host files your download client and
library paths use. A mismatched mount can make a download succeed while import
cannot find the completed file.

The default web port is `30200`. Put the app behind TrueNAS local networking,
Cosmos, Cloudflare Access, or another trusted access boundary before exposing it
outside your LAN. Set `LIBRARRY_API_KEY` when the API is reachable by anything
other than trusted local users.

To pin a release, replace `:latest` with a published version tag in both Librarry
image references.

## Smoke Test

After the app starts:

```bash
curl -fsS http://<truenas-host>:30200/healthz
```

Then open `http://<truenas-host>:30200/`, visit Settings, and verify provider
health plus Prowlarr/download-client health after credentials are saved.

## Backups

Back up Postgres before upgrades, Readarr migration tests, or large library
imports. From a shell that can run Docker commands against the app, replace the
container name with the Postgres container name shown by TrueNAS:

```bash
docker exec <postgres-container> pg_dump -U librarry librarry > librarry.sql
```

After the first GHCR publish, verify the `librarry-api` and `librarry-web`
packages are public in GitHub's package settings before installing on a NAS that
does not authenticate to GHCR.

## Stabilization candidate configuration

All deployment variants now pass completed import/removal controls, import mode,
rename/recycle/extra-file settings, and import-list sync enable/interval settings to the API.
Automatic grabbing and removal remain enabled by default. To retain all completed
downloads, set `LIBRARRY_COMPLETED_REMOVE_ENABLED=false`; source Compose and image
Compose now honor it. Use `hardlinkOrCopy`, `hardlink`, or `copy` for
`LIBRARRY_COMPLETED_IMPORT_MODE`; completed-download move mode is rejected.

Removal is individually gated by verified imported content and actual seeding
eligibility. Legacy imports and incomplete/ambiguous payloads stay in the client.
Back up both Postgres and library/download data before upgrading. The local
fixture restore check does not qualify restoration of the live homelab backup.
No September candidate release or production rollback rehearsal is complete yet.


Hourly History Maintenance compacts notification detail only after all recipients
have been resolved for 90 days. Unresolved deliveries and compact event identities
remain in Postgres; include both in backups. Restore with notification egress
isolated until later receiver acceptance is reconciled. Compaction cannot protect
against acceptance that happened after the backup. See the
[retention policy](../../docs/local-dev.md#notification-history-retention).


`LIBRARRY_IMPORT_LIST_SYNC_ENABLED` defaults to `true`. Set it to `false` and
recreate the API container to pause scheduled list sync; explicit list-sync
commands remain available. It is independent of feed sync. System → Tasks keeps
disabled/unavailable workers visible with reasons and retained shared history.
Flags apply to each API instance; update every instance to stop scheduled work
across a deployment.

The same portal exposes `/readyz` for current database connectivity (200 ready,
503 without usable persistence). `/healthz` remains process liveness; neither
certifies external clients, mounts or completed imports. System can download a
redacted support report without contacting providers. See
[probe and support semantics](../../docs/deployment.md#liveness-readiness-and-support).
