# Deployment

Librarry is distributed as a small self-hosted stack:

- `ghcr.io/bandoracer/librarry-api`: Go API, migrations, and background workers.
- `ghcr.io/bandoracer/librarry-web`: nginx serving the React app and proxying
  `/api/` plus `/healthz` to the API service.
- `postgres:16-alpine`: persistent application database.

The default web port is `30200`. The API is intended to stay private on the
compose network; browser traffic reaches it through the web container.

## Images

Images are built for `linux/amd64` and `linux/arm64` by
[.github/workflows/container-images.yml](../.github/workflows/container-images.yml).
The workflow publishes on pushes to `main`, version tags, and manual dispatches.
After the first publish, verify both GHCR packages are public in GitHub's
package settings; anonymous Docker and NAS pulls fail against private packages.

Use `:latest` for alpha testing. Use a version tag once releases are cut:

```dotenv
LIBRARRY_API_IMAGE=ghcr.io/bandoracer/librarry-api:latest
LIBRARRY_WEB_IMAGE=ghcr.io/bandoracer/librarry-web:latest
```

## Generic Docker Compose

```bash
git clone https://github.com/bandoracer/librarry.git
cd librarry/deploy
cp .env.example .env
docker compose pull
docker compose up -d
```

Then open `http://127.0.0.1:30200`.

For source builds instead of published images:

```bash
cd deploy
docker compose -f docker-compose.build.yml up --build
```

## Persistent Paths

The public Compose file uses:

- `LIBRARRY_POSTGRES_DATA=postgres-data` as a Docker named volume by default.
- `LIBRARRY_MEDIA_STACK_PATH=./data/media-stack` mounted into the API as `/data`.

Production installs should usually replace those with absolute host paths:

```dotenv
LIBRARRY_POSTGRES_DATA=/srv/librarry/postgres
LIBRARRY_MEDIA_STACK_PATH=/srv/media-stack
```

Inside the API container, the expected book paths are:

```dotenv
LIBRARRY_BOOK_TORRENT_ROOT=/data/torrents/books
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
```

## Required Security Settings

Set `LIBRARRY_API_KEY` before exposing Librarry outside trusted local access.
When configured, `/api/` accepts `X-Api-Key`, `apikey`, `apiKey`, or
`Authorization: Bearer ...`; `/healthz` and `/ping` stay open for probes.

The example Postgres password is not safe for public or shared environments.
If you change `POSTGRES_PASSWORD`, update `LIBRARRY_DATABASE_URL` to match.
Use URL-safe characters or percent-encode special characters in the database URL.

## Integrations

Optional integrations can be set in `.env` before first start or saved later from
Settings:

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

Open Library works without credentials. Hardcover and Google Books need their
own provider credentials. Prowlarr handles indexer search. qBittorrent,
Transmission, and SABnzbd remain external download clients.

## TrueNAS

Use [deploy/truenas/install.yaml](../deploy/truenas/install.yaml) as a TrueNAS
Custom App compose template. Before installing:

1. Replace `truenas.local` with your TrueNAS hostname or LAN IP.
2. Replace `/mnt/tank/apps/librarry/postgres` with a persistent app dataset.
3. Replace `/mnt/tank/media-stack` with the dataset mounted into Librarry as
   `/data`.
4. Replace `change-me` in the Postgres password and `LIBRARRY_DATABASE_URL`.
5. Set provider, Prowlarr, and download-client credentials as needed.

The TrueNAS template uses the published GHCR images and binds the web UI to
`0.0.0.0:30200`.

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
```

Librarry does not ship a single Community Applications XML template because the
app is a three-service stack. Installing only one container would leave either
the API or database missing.

## Reverse Proxy

Proxy the web container, not the API container. The web image serves the SPA and
proxies API calls internally:

```text
client -> reverse proxy -> librarry-web:80 -> librarry-api:8080
```

For a public hostname, set:

```dotenv
LIBRARRY_WEB_ORIGIN=https://librarry.example.com
LIBRARRY_API_KEY=<random long value>
```

## Upgrade

```bash
cd deploy
docker compose pull
docker compose up -d
```

Back up Postgres before major upgrades or migration testing:

```bash
docker compose exec postgres pg_dump -U librarry librarry > librarry.sql
```
