# Unraid Install

Librarry is a multi-container app: Postgres, API/worker, and web/nginx. The
supported Unraid packaging is therefore a Docker Compose Manager stack, not a
single Community Applications XML template.

## Install

1. Install the Unraid Docker Compose Manager plugin if it is not already present.
2. Create a new stack named `librarry`.
3. Copy [docker-compose.yml](docker-compose.yml) into the stack.
4. Copy [.env.example](.env.example) to `.env` beside the stack and edit it.
5. Start the stack.

Default paths:

- App data: `/mnt/user/appdata/librarry`
- Backups/config: `/mnt/user/appdata/librarry/config`
- Media/download mount: `/mnt/user/media-stack`
- Web UI: `http://tower.local:30200`
- Container media mount: `/data`
- API run user: `PUID=99`, `PGID=100`

Set `LIBRARRY_WEB_ORIGIN` to the hostname you actually use for the web UI, such
as `http://tower.local:30200` or a reverse-proxy URL. Set `LIBRARRY_API_KEY`
before exposing Librarry outside trusted LAN access.

Set `PUID` and `PGID` to the same user/group used by qBittorrent, Transmission,
SABnzbd, and the book library share. The API container needs read access for
scans and read/write access for imports, renames, moves, hardlinks, and deletes.

Inside the API container, the default book paths are:

```dotenv
LIBRARRY_BOOK_TORRENT_ROOT=/data/torrents/books
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
```

If your download client saves completed books somewhere else, change either its
book category path or these Librarry settings so they describe the same files.

The web container includes Unraid labels for icon, WebUI URL, and shell access.
Scheduled pg_dump backups are written to `/config/backups`, which maps to
`$LIBRARRY_APPDATA_PATH/config/backups`.

To update:

```bash
docker compose pull
docker compose up -d
```

To pin a release, replace `:latest` in `.env` with a published version tag.

Smoke test after start:

```bash
curl -fsS http://tower.local:30200/healthz
```

Back up Postgres before upgrades, Readarr migration tests, or large library
imports:

```bash
docker exec librarry-postgres pg_dump -U librarry librarry > librarry.sql
```

After the first GHCR publish, verify the `librarry-api` and `librarry-web`
packages are public in GitHub's package settings before installing on an Unraid
host that does not authenticate to GHCR.
