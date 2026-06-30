# TrueNAS Install

Librarry is distributed as two application images plus Postgres:

- `ghcr.io/bandoracer/librarry-api`
- `ghcr.io/bandoracer/librarry-web`
- `postgres:16-alpine`

Use [install.yaml](install.yaml) as a TrueNAS Custom App compose template. Before
installing, replace the generic placeholders with values for your NAS:

- `truenas.local`: your TrueNAS hostname or LAN IP.
- `/mnt/tank/apps/librarry/postgres`: a persistent app dataset for Postgres.
- `/mnt/tank/media-stack`: the host media/download dataset mounted as `/data`.
- `change-me`: a real Postgres password, and the matching password inside
  `LIBRARRY_DATABASE_URL`.
- provider, Prowlarr, qBittorrent, Transmission, or SABnzbd credentials when you
  want those integrations enabled at startup.

The default web port is `30200`. Put the app behind TrueNAS local networking,
Cosmos, Cloudflare Access, or another trusted access boundary before exposing it
outside your LAN. Set `LIBRARRY_API_KEY` when the API is reachable by anything
other than trusted local users.

To pin a release, replace `:latest` with a published version tag in both Librarry
image references.

After the first GHCR publish, verify the `librarry-api` and `librarry-web`
packages are public in GitHub's package settings before installing on a NAS that
does not authenticate to GHCR.
