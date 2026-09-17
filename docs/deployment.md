# Deployment

Librarry is distributed as a three-service self-hosted stack:

- `ghcr.io/bandoracer/librarry-api`: Go API, migrations, background workers,
  metadata providers, download-client integrations, scans, and imports.
- `ghcr.io/bandoracer/librarry-web`: nginx serving the React app and proxying
  `/api/` plus `/healthz` and `/readyz` to the API service.
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
Pushes to `main`, version tags, pull requests, and ordinary manual runs only
validate builds. To publish images for qualification, manually dispatch the
workflow at the reviewed candidate ref and explicitly enable `publish_candidate`.
It publishes only `candidate-<full commit>-<run ID>-<attempt>` tags and records
each image digest in a `candidate-librarry-api` or `candidate-librarry-web` artifact.
Rebuilding a commit gets a different tag. Candidate tags are not release approval;
use the recorded digest for qualification and deployment.

This build workflow cannot update `latest`, branch aliases, or version aliases.
The separate **Promote tested images** workflow is manual-only on `main`: supply
the qualified API and web index digests. It checks both architectures, records
the previous pair, copies the indexes to `latest`, verifies both tags, and
attempts to restore the previous pair if promotion fails. It never rebuilds.
Merging a fix alone does not update package tags or installed containers.
See the [release checklist](release-checklist.md) for qualification evidence
and the owner decision to promote this release before observation completed.

The installer defaults use the current qualified `:latest` pair:

```dotenv
LIBRARRY_API_IMAGE=ghcr.io/bandoracer/librarry-api:latest
LIBRARRY_WEB_IMAGE=ghcr.io/bandoracer/librarry-web:latest
```

For candidate testing, override both variables with the corresponding
`ghcr.io/bandoracer/librarry-api@sha256:<digest>` and
`ghcr.io/bandoracer/librarry-web@sha256:<digest>` values recorded by the same run.
Wait for both image jobs to succeed; a partially successful run is not a usable
release pair. Verify both GHCR packages allow anonymous pulls for public installs.
Package visibility is independent of a successful authenticated publication.

## Generic Docker Compose

```bash
git clone https://github.com/bandoracer/librarry.git
cd librarry/deploy
cp .env.example .env
```

Edit `.env` in the current `deploy` directory before first start. At minimum:

1. Replace the `POSTGRES_PASSWORD` placeholder.
2. Update `LIBRARRY_DATABASE_URL` with the same password.
3. Set absolute host paths for `LIBRARRY_POSTGRES_DATA` and
   `LIBRARRY_MEDIA_STACK_PATH`.
4. Set `LIBRARRY_WEB_ORIGIN` to the URL users will open.
5. Keep `LIBRARRY_AUTH_METHOD=forms` and replace the browser username/password.
   Configure `LIBRARRY_API_KEY` separately for compatible API clients and feeds.

Then validate and start the stack:

```bash
docker compose config -q
docker compose pull
docker compose up -d
docker compose ps
```

Open `http://127.0.0.1:30200` or the host/port configured in `.env`.

For source builds, stay in `deploy` and use the explicit build file:

```bash
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

Use `LIBRARRY_AUTH_METHOD=forms` with a unique browser username/password for
remote or reverse-proxied access. Forms/Basic methods require usable persisted
credentials. Configure `LIBRARRY_API_KEY` separately for compatible API clients
and calendar feeds; an API key is not the interactive browser sign-in flow.

API-key clients can send `X-Api-Key`, `apikey`, `apiKey`, or
`Authorization: Bearer ...`. `/healthz`, `/readyz` and `/ping` remain probes;
nginx serves static UI assets separately from authenticated API data/actions.
Keep API and PostgreSQL ports private. See [authentication](guides/operations.md#authentication)
and [security reporting](../SECURITY.md).

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
stack directory and edit it before starting. The stack fails early when the
database password or database URL has not been configured.

Default Unraid paths:

```dotenv
LIBRARRY_APPDATA_PATH=/mnt/user/appdata/librarry
LIBRARRY_MEDIA_STACK_PATH=/mnt/user/media-stack
LIBRARRY_WEB_BIND=0.0.0.0
LIBRARRY_WEB_ORIGIN=http://tower.local:30200
PUID=99
PGID=100
```

The Unraid stack stores Postgres under
`$LIBRARRY_APPDATA_PATH/postgres` and backups/config under
`$LIBRARRY_APPDATA_PATH/config`.

Set `PUID` and `PGID` to the same user/group used by your download client and
book library paths. Unraid's common `nobody:users` mapping is `99:100`. Keep
the appdata share on a local cache pool; Postgres should not run on a
network-mounted share.

For a remotely accessible installation, use forms authentication:

```dotenv
LIBRARRY_AUTH_METHOD=forms
LIBRARRY_AUTH_USERNAME=admin
LIBRARRY_AUTH_PASSWORD=<unique-password>
```

This gives the web UI a normal browser sign-in. `LIBRARRY_API_KEY` is optional
and intended for Readarr-compatible clients and calendar feeds; it is not a
replacement for the browser sign-in configuration.

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

Check [current status](status.md) and the [release checklist](release-checklist.md)
before selecting an image pair. Updating the checkout or pulling historical
`latest` does not install the candidate. Pin both API and web to the paired
immutable references from the same qualified run.

Before changing a running installation:

1. Record the old image references/digests, schema version, Compose/template and
   effective configuration. Keep credential-bearing configuration private.
2. Stop application workers and other writers to the library/download paths for
   a consistent database/media checkpoint. Keep PostgreSQL available for the dump.
3. Back up the database, app configuration, library and relevant download data.
   Keep originals untouched while rehearsing the restore on isolated copies.
4. Update both image references and validate paths, permissions and auth settings.
5. Recreate the API/web services, inspect startup/migration logs, then verify
   readiness, browser sign-in, file evidence and read-only integration checks.

For Compose, from the configured stack directory after the backup:

```bash
docker compose config -q
docker compose pull
docker compose up -d
docker compose ps
```

For TrueNAS, edit the Custom App image references and redeploy through its UI.
For Unraid, use the configured Compose Manager stack directory. Verify both
`/healthz` and `/readyz`; neither alone proves working media mounts or imports.

If rollback is required, stop the new application and restore the matching
pre-upgrade database/media/config checkpoint before starting the recorded old
image pair. Do not point older binaries at a newer schema or attempt an
unreviewed in-place reverse migration. The documented production-copy rehearsal
proved this approach for the recorded candidate; a later installation still
needs its own current backup and target checks.

## Backup And Restore

From the configured Compose stack directory, create a PostgreSQL custom-format
dump without a terminal:

```bash
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > librarry.dump
```

Check the command's exit status and retain the database dump alongside a matching
media/config checkpoint and the original image digests. Scheduled application
backups also use custom format and require `pg_restore`, not `psql`.

Rehearse restoration in a **separate isolated stack**, with copied media,
separate database storage, external acquisition disabled and notification egress
blocked. From that restore stack's directory, create an empty test database and
load the dump:

```bash
docker compose exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" librarry_restore'
docker compose exec -T postgres sh -c 'pg_restore -U "$POSTGRES_USER" --no-owner --no-acl -d librarry_restore' < librarry.dump
```

Configure only the isolated API to use `librarry_restore`. Verify books,
overrides, file/book links, paths and media hashes, not just successful startup.
A database dump does not contain library bytes or environment configuration.
Do not run these restore steps against the live database. Review
[restored notification state](#notification-state-and-restored-databases) before
allowing the restored application to contact receivers.

## Release Checklist

Use the [bounded release checklist](release-checklist.md). Successful image
builds are one prerequisite, not evidence that imports, upgrades, restores, or
the target NAS work correctly. Do not publish stable aliases before completing
the applicable release gates.

## Stabilization candidate configuration

All deployment variants now pass completed import/removal controls, import mode,
rename/recycle/extra-file settings, and import-list sync interval to the API.
Automatic grabbing and removal remain enabled by default. To retain all completed
downloads, set `LIBRARRY_COMPLETED_REMOVE_ENABLED=false`; source Compose and image
Compose now honor it. Use `hardlinkOrCopy`, `hardlink`, or `copy` for
`LIBRARRY_COMPLETED_IMPORT_MODE`; completed-download move mode is rejected.

Removal is individually gated by verified imported content and actual seeding
eligibility. Legacy imports and incomplete/ambiguous payloads stay in the client.
Back up both Postgres and library/download data before upgrading. The recorded candidate passed an isolated
production-copy upgrade and rollback, followed by a controlled NAS rollout.
The [rollout record](reviews/2026-09-16-live-rollout.md) preserves target-specific
backup, migration and readback evidence.
See [current status](status.md) and the [qualification report](reviews/2026-09-16-release-qualification.md)
for the exact scope and remaining observation work.

### Candidate filesystem qualification

The stabilization candidate stages data in the destination directory, then uses
an atomic, non-overwriting hard link to publish the staged file. This requires
hard-link support within the destination filesystem even in copy mode; source
and destination may still be on different filesystems. Unsupported filesystems
return an import error and retain the original. Interrupted multi-file recovery
has controlled fixture coverage; SMB/NFS mount behavior is not qualified. Run
the packaged fixture tests on the intended storage before promoting this candidate.

### Database sessions for background workers

The stabilization candidate coordinates registered workers using Postgres session
advisory locks and persisted schedule/run records (migration 0047). Use a direct
Postgres connection or a session-pooling proxy. Transaction pooling cannot
preserve worker ownership and is not supported. A database outage prevents new
worker claims; a lost owner session is shown as interrupted in System Tasks.

Two disposable API processes have verified shared ownership and process-kill
recovery. This does not certify running multiple production API instances against
a shared NAS. Acquisition/import journals remain the authority for side effects,
and live platform/soak qualification is still required.

### Notification state and restored databases

Migrations 0048–0049 include native and compatibility events, immutable domain
snapshots, target deliveries, attempts, operator decisions and health transitions
in ordinary Postgres backups. New history is
captured transactionally; applying the migration does not resend old history.
The outbox worker starts automatically with database persistence. It uses the same
direct/session-pooled connection requirement as other shared workers.

A database backup cannot roll back a remote receiver. Restore into an isolated
network, inspect pending/uncertain deliveries and verify their receiver state
before allowing notification egress. An older backup may predate acceptance
that happened after the backup. Confirm acceptance or cancel such entries instead
of blindly replaying them. Resolved delivery/attempt/action details can be compacted after 90 days by hourly
History Maintenance. Unresolved deliveries and permanent compact source-event
records remain; backups must retain both. Compact records prevent replay of an
archived event, but cannot record acceptances that happened after a restored backup.
Include this ledger when estimating backup/storage size. The disposable restore
fixture verifies row preservation, not live recipient or homelab restoration.


### Worker scheduling flags

System → Tasks distinguishes instance configuration from shared execution history.
Disabled workers remain inspectable and cannot be manually run through that
instance's System Tasks endpoint. Unavailable dependencies appear with reasons.
Changing environment flags requires recreating/restarting the API; configure every
API instance when pausing automation across a shared database.
`LIBRARRY_IMPORT_LIST_SYNC_ENABLED=true` is now explicitly forwarded by generic,
source-build, TrueNAS and Unraid templates. Set it false to pause list scheduling
without changing feed sync or removing explicit per-list/compatibility commands.

### Liveness, readiness and support

The API and web nginx expose `/healthz` for process liveness and `/readyz` for a
current database-connectivity check. Readiness returns 503 if persistence is
unconfigured or unavailable and recovers when the database responds. Its response
contains only status and check time. It does not probe external integrations or
claim that mounts, imports or backups are healthy. Keep liveness restart policies
separate from readiness routing so a database outage does not cause restart loops.

System's **Download support report** uses the usual API/session authentication.
The report includes build identity, schema at startup, selected effective settings,
Postgres version when readable, anonymous roots and recorded provider/worker
observations. It never includes credentials, URLs, private paths, book metadata or
free-text logs. Unknown remote versions and image digest remain unknown: use your
container runtime to obtain the actual deployed manifest digest. Provider times
are process-local request evidence and are not refreshed by downloading. Root
presence is not NAS/mount/write-permission certification. No production restore or
unattended soak is implied by a successful export.
