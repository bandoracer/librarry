# Unraid Install

Librarry is a multi-container app: Postgres, API/worker, and web/nginx. The
supported Unraid packaging is therefore a Docker Compose Manager stack, not a
single Community Applications XML template.

## Install

1. Install the Unraid Docker Compose Manager plugin if it is not already present.
2. Create a new stack named `librarry` and copy both
   [docker-compose.yml](docker-compose.yml) and
   [.env.example](.env.example) into the stack directory.
3. Rename `.env.example` to `.env` and replace every `replace-with-...` value.
   Use a URL-safe Postgres password, for example `openssl rand -hex 24`, and
   use that same value in the database URL.
4. Set `LIBRARRY_WEB_ORIGIN` to the exact URL opened in a browser. If you
   change `LIBRARRY_WEB_PORT`, update this value too.
5. Keep `LIBRARRY_AUTH_METHOD=forms` and set a unique browser username and
   password. This is the supported UI protection for a remote or
   reverse-proxied install. `LIBRARRY_API_KEY` is optional and is for Arr API
   clients and calendar feeds, not interactive browser sign-in.
6. Validate and start the stack from its directory:

   ```bash
   docker compose --env-file .env config -q
   docker compose pull
   docker compose up -d
   docker compose ps
   ```

Default paths:

- App data: `/mnt/user/appdata/librarry`
- Backups/config: `/mnt/user/appdata/librarry/config`
- Media/download mount: `/mnt/user/media-stack`
- Web UI: `http://tower.local:30200`
- Container media mount: `/data`
- API run user: `PUID=99`, `PGID=100`

Set `LIBRARRY_WEB_ORIGIN` to the hostname you actually use for the web UI, such
as `http://tower.local:30200` or a reverse-proxy URL. `LIBRARRY_WEB_BIND`
defaults to `0.0.0.0`; set it to the Unraid LAN address when a reverse proxy is
the only intended entry point.

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
`$LIBRARRY_APPDATA_PATH/config/backups`. Keep the `appdata` share on a local
cache pool; do not place Postgres on a network-mounted share.

To update:

```bash
docker compose pull
docker compose up -d
```

The defaults pull public multi-architecture GHCR images. To pin a release,
replace `:latest` in `.env` with the same published version tag for both API
and web images.

Smoke test after start:

```bash
curl -fsS http://tower.local:30200/healthz
```

Back up Postgres before upgrades, Readarr migration tests, or large library
imports:

```bash
docker exec librarry-postgres pg_dump -U librarry librarry > librarry.sql
```

Both `librarry-api` and `librarry-web` GHCR packages are public, so Unraid can
pull them without a GitHub login. The stack supports the normal Unraid x86_64
servers and ARM64 Docker hosts.

## Candidate selection and automation

Installer defaults select the promoted `latest` image pair. Select both qualified
index digests explicitly when a deployment must remain pinned. [Current status](../../docs/status.md)
distinguishes candidate publication, production-copy rollback qualification and
actual deployment. The maintainer NAS runs this same pair. The owner authorized
promotion before the longer observation period completed.

Scheduled auto-grab and removal defaults are enabled. Review the
[automation and import controls](../../docs/deployment.md#stabilization-candidate-configuration)
before connecting real clients. Back up database, configuration and media; follow
[upgrade and rollback instructions](../../docs/deployment.md#upgrade) for your target.



Hourly History Maintenance compacts notification detail only after all recipients
have been resolved for 90 days. Unresolved deliveries and compact event identities
remain in Postgres; include both in backups. Restore with notification egress
isolated until later receiver acceptance is reconciled. Compaction cannot protect
against acceptance that happened after the backup. See the
[retention policy](../../docs/guides/operations.md#notification-history-retention).


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
