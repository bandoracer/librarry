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
- a shared media/download dataset that Librarry can see as `/data`, for example
  `/mnt/tank/media-stack`;
- ACLs that let the API container read scans and read/write completed imports.

The template runs the API as `568:568`, the common TrueNAS SCALE apps user/group.
Replace the `user:` value if your media dataset is owned by a different UID/GID.

## Template Values

Before installing, replace the generic placeholders with values for your NAS:

- `truenas.local`: your TrueNAS hostname or LAN IP.
- `/mnt/tank/apps/librarry/postgres`: a persistent app dataset for Postgres.
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
