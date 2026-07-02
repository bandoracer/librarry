# Deployment

Librarry is distributed as a three-service self-hosted stack:

- `ghcr.io/bandoracer/librarry-api`: Go API, migrations, background workers,
  metadata providers, download-client integrations, scans, and imports.
- `ghcr.io/bandoracer/librarry-web`: nginx serving the React app and proxying
  `/api/` plus `/healthz` to the API service.
- `postgres:16-alpine`: persistent Librarry database.

The default web port is `30200`. Keep the API private on the Compose network and
send browser or reverse-proxy traffic to the web container.

## Install Matrix

| Target | Files | Use when |
| --- | --- | --- |
| Generic Docker Compose | [deploy/docker-compose.yml](../deploy/docker-compose.yml), [deploy/.env.example](../deploy/.env.example) | Linux server, VM, mini PC, or any Docker host. |
| Source-build Compose | [deploy/docker-compose.build.yml](../deploy/docker-compose.build.yml) | Testing a checkout before images are published. |
| TrueNAS SCALE Custom App | [deploy/truenas/install.yaml](../deploy/truenas/install.yaml) | Installing through the TrueNAS Custom App YAML editor. |
| Unraid Docker Compose Manager | [deploy/unraid/docker-compose.yml](../deploy/unraid/docker-compose.yml), [deploy/unraid/.env.example](../deploy/unraid/.env.example) | Installing as a multi-container Unraid stack. |

## Images

Images are built for `linux/amd64` and `linux/arm64` by
[.github/workflows/container-images.yml](../.github/workflows/container-images.yml).
The workflow publishes on pushes to `main`, version tags, and manual dispatches.

Use `:latest` only for alpha testing:

```dotenv
LIBRARRY_API_IMAGE=ghcr.io/bandoracer/librarry-api:latest
LIBRARRY_WEB_IMAGE=ghcr.io/bandoracer/librarry-web:latest
```

For production-like installs, pin both images to the same version tag once
tagged releases exist. After the first publish, verify both GHCR packages are
public in GitHub package settings; anonymous Docker, TrueNAS, and Unraid pulls
fail against private packages.

## Generic Docker Compose

```bash
git clone https://github.com/bandoracer/librarry.git
cd librarry/deploy
cp .env.example .env
```

Edit `deploy/.env` before first start. At minimum:

1. Replace `POSTGRES_PASSWORD=change-me`.
2. Update `LIBRARRY_DATABASE_URL` with the same password.
3. Set absolute host paths for `LIBRARRY_POSTGRES_DATA` and
   `LIBRARRY_MEDIA_STACK_PATH`.
4. Set `LIBRARRY_WEB_ORIGIN` to the URL users will open.
5. Set `LIBRARRY_API_KEY` before exposing the app outside trusted local access.

Then start the stack:

```bash
docker compose pull
docker compose up -d
docker compose ps
```

Open `http://127.0.0.1:30200` or the host/port configured in `.env`.

For source builds instead of published images:

```bash
cd deploy
docker compose -f docker-compose.build.yml up --build
```

## Path Contract

Librarry needs the same completed-download and library paths that your download
client, Prowlarr, and media stack use. The examples mount one shared host root
into the API container as `/data`:

```dotenv
LIBRARRY_MEDIA_STACK_PATH=/srv/media-stack
LIBRARRY_CONFIG_PATH=/srv/librarry/config
LIBRARRY_BOOK_TORRENT_ROOT=/data/torrents/books
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
LIBRARRY_BACKUP_DIR=/config/backups
```

If qBittorrent saves a book to `/srv/media-stack/torrents/books` on the host,
Librarry should see that same file at `/data/torrents/books` inside the API
container. If these paths do not line up, grabs may succeed but completed import
will not find the payload.

The API container also mounts persistent app config at `/config`. Scheduled
database backups default to `/config/backups`; keep that path on durable storage
or disable backups with `LIBRARRY_BACKUP_ENABLED=false`.

The API image includes a built-in `librarry` user with UID/GID `1000:1000`.
Change `LIBRARRY_RUN_USER` in generic Compose, `PUID`/`PGID` in Unraid, or the
`user:` line in the TrueNAS template to match the owner of your media dataset.
The API user needs read access for scans and read/write access for imports,
renames, moves, hardlinks, and deletes.

## Security

Set `LIBRARRY_API_KEY` before using a public hostname, Cloudflare Access, Cosmos,
Tailscale Funnel, or any reverse proxy reachable by more than trusted local
users. When configured, `/api/` accepts `X-Api-Key`, `apikey`, `apiKey`, or
`Authorization: Bearer ...`; `/healthz` and `/ping` stay open for probes.

The Postgres password in examples is a placeholder. Use URL-safe characters or
percent-encode special characters in `LIBRARRY_DATABASE_URL`.

Do not publish provider tokens, Prowlarr API keys, download-client passwords, or
database passwords in Compose files, screenshots, logs, issues, or support
requests.

## Integrations

Optional integrations can be set in `.env` before first start or saved later
from Settings:

```dotenv
LIBRARRY_HARDCOVER_TOKEN=
LIBRARRY_GOOGLE_BOOKS_API_KEY=
LIBRARRY_PROWLARR_URL=
LIBRARRY_PROWLARR_API_KEY=
LIBRARRY_QBITTORRENT_URL=
LIBRARRY_QBITTORRENT_USERNAME=
LIBRARRY_QBITTORRENT_PASSWORD=
LIBRARRY_TRANSMISSION_URL=
LIBRARRY_TRANSMISSION_USERNAME=
LIBRARRY_TRANSMISSION_PASSWORD=
LIBRARRY_SABNZBD_URL=
LIBRARRY_SABNZBD_API_KEY=
LIBRARRY_SABNZBD_USERNAME=
LIBRARRY_SABNZBD_PASSWORD=
```

Open Library works without credentials. Hardcover provides richer metadata when
`LIBRARRY_HARDCOVER_TOKEN` is set. Google Books is an exact-match fallback and
requires `LIBRARRY_GOOGLE_BOOKS_API_KEY`. Prowlarr handles indexer search.
qBittorrent, Transmission, and SABnzbd remain external download clients.

## TrueNAS SCALE

Use [deploy/truenas/install.yaml](../deploy/truenas/install.yaml) as a Custom
App compose template.

Before installing:

1. Create persistent datasets for app data, backups/config, and the shared
   media/download root.
2. Grant the API run user access to the media dataset. The template defaults to
   TrueNAS SCALE's common apps UID/GID `568:568`; replace the `user:` value if
   your dataset ACL uses a different owner.
3. Replace `truenas.local` with your TrueNAS hostname or LAN IP.
4. Replace `/mnt/tank/apps/librarry/postgres` with your Postgres app dataset.
5. Replace `/mnt/tank/apps/librarry/config` with your persistent app config
   dataset mounted into Librarry as `/config`.
6. Replace `/mnt/tank/media-stack` with the dataset mounted into Librarry as
   `/data`.
7. Replace `change-me` in both `POSTGRES_PASSWORD` and
   `LIBRARRY_DATABASE_URL`.
8. Add provider, Prowlarr, and download-client credentials as needed.

The template uses published GHCR images and binds the web UI to
`0.0.0.0:30200`. If you expose it through Cosmos, Cloudflare, or another reverse
proxy, proxy the web service and set `LIBRARRY_WEB_ORIGIN` to the public URL.

## Unraid

Use [deploy/unraid/docker-compose.yml](../deploy/unraid/docker-compose.yml) with
the Unraid Docker Compose Manager plugin. Copy
[deploy/unraid/.env.example](../deploy/unraid/.env.example) to `.env` in the
stack directory and edit it before starting.

Default Unraid paths:

```dotenv
LIBRARRY_APPDATA_PATH=/mnt/user/appdata/librarry
LIBRARRY_MEDIA_STACK_PATH=/mnt/user/media-stack
LIBRARRY_WEB_ORIGIN=http://tower.local:30200
PUID=99
PGID=100
```

The Unraid stack stores Postgres under
`$LIBRARRY_APPDATA_PATH/postgres` and backups/config under
`$LIBRARRY_APPDATA_PATH/config`.

Set `PUID` and `PGID` to the same user/group used by your download client and
book library paths. Unraid's common `nobody:users` mapping is `99:100`.

Librarry does not ship a single Community Applications XML template because the
app is a three-service stack. Installing only one container would leave either
the API or database missing. The Compose stack adds Unraid WebUI labels to the
web container for easier access from the Docker page.

## Reverse Proxy

Proxy the web container, not the API container:

```text
client -> reverse proxy -> librarry-web:80 -> librarry-api:8080
```

For a public hostname:

```dotenv
LIBRARRY_WEB_ORIGIN=https://librarry.example.com
LIBRARRY_API_KEY=<random long value>
```

The web image handles React direct-route fallback with nginx `try_files`, so
direct loads such as `/settings` and `/downloads` should return the SPA.

## Upgrade

Generic Compose and Unraid:

```bash
cd deploy
docker compose pull
docker compose up -d
docker compose ps
```

TrueNAS:

1. Back up Postgres.
2. Edit the Custom App if changing image tags or paths.
3. Redeploy/recreate the app from the TrueNAS UI.
4. Open `/healthz` and the web UI.

Pin both API and web to the same version tag when moving past alpha testing.

## Backup And Restore

Back up Postgres before major upgrades, Readarr migration testing, or large
library imports:

```bash
docker compose exec postgres pg_dump -U librarry librarry > librarry.sql
```

Restore into an empty database:

```bash
docker compose exec -T postgres psql -U librarry librarry < librarry.sql
```

Also back up any host library paths that Librarry imports into, especially when
using move, rename, hardlink, or delete actions.

## Release Checklist

Before calling a build publicly installable:

1. `go test ./...`
2. `cd web && npm run build`
3. `docker compose -f deploy/docker-compose.build.yml config`
4. `docker compose -f deploy/docker-compose.yml config`
5. Build/push both GHCR images for `linux/amd64` and `linux/arm64`.
6. Verify the GHCR packages are public.
7. Smoke test `/healthz`, web direct routes, provider health, and at least one
   safe book search/grab path on the target platform.
