# Librarry Deployment Files

This directory contains the public deployment surfaces for Librarry.

| Target | File | Notes |
| --- | --- | --- |
| Generic Docker Compose | [docker-compose.yml](docker-compose.yml) + [.env.example](.env.example) | Pulls published GHCR images and starts Postgres, API, and web. |
| Source-build Compose | [docker-compose.build.yml](docker-compose.build.yml) | Builds API and web images from the current checkout. |
| TrueNAS SCALE Custom App | [truenas/install.yaml](truenas/install.yaml) | Paste into the Custom App YAML editor and replace placeholders. |
| Unraid Docker Compose Manager | [unraid/docker-compose.yml](unraid/docker-compose.yml) + [unraid/.env.example](unraid/.env.example) | Three-service stack with Unraid WebUI labels. |

Librarry is intentionally a stack, not a single-container app:

- `librarry-api`: Go API, background workers, migrations, integrations, imports.
- `librarry-web`: nginx-served React UI and reverse proxy for `/api/`.
- `postgres`: persistent Librarry database.

The API container needs read/write access to the same book download and library
paths used by your media stack. Mount that shared root into the API container as
`/data`, then keep these paths aligned:

```dotenv
LIBRARRY_BOOK_TORRENT_ROOT=/data/torrents/books
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
```

See [../docs/deployment.md](../docs/deployment.md) for the full install guide,
upgrade commands, backups, reverse proxy guidance, and NAS-specific notes.

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


`LIBRARRY_IMPORT_LIST_SYNC_ENABLED` defaults to `true`. Set it to `false` and
recreate the API container to pause scheduled list sync; explicit list-sync
commands remain available. It is independent of feed sync. System → Tasks keeps
disabled/unavailable workers visible with reasons and retained shared history.
Flags apply to each API instance; update every instance to stop scheduled work
across a deployment.
