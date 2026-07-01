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
