# Librarry Deployment Files

This directory contains the public deployment surfaces for Librarry.

Image builds do not automatically publish on merge or version tags. Candidate
publication requires an explicit manual workflow input and produces a unique
candidate tag plus recorded digests. Existing `latest` images are unchanged.
See the [publication policy](../docs/deployment.md#images) and
[release gates](../docs/release-checklist.md) before installing a candidate.

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

## Candidate selection and automation

Installer defaults select the promoted `latest` image pair. Select both qualified
index digests explicitly when a deployment must remain pinned. [Current status](../docs/status.md)
distinguishes candidate publication, production-copy rollback qualification and
actual deployment. The maintainer NAS runs this same pair. The owner authorized
promotion before the longer observation period completed.

Scheduled auto-grab and removal defaults are enabled. Review the
[automation and import controls](../docs/deployment.md#stabilization-candidate-configuration)
before connecting real clients. Back up database, configuration and media; follow
[upgrade and rollback instructions](../docs/deployment.md#upgrade) for your target.



`LIBRARRY_IMPORT_LIST_SYNC_ENABLED` defaults to `true`. Set it to `false` and
recreate the API container to pause scheduled list sync; explicit list-sync
commands remain available. It is independent of feed sync. System → Tasks keeps
disabled/unavailable workers visible with reasons and retained shared history.
Flags apply to each API instance; update every instance to stop scheduled work
across a deployment.

## Kindle email delivery

Manual EPUB/PDF delivery, SMTP configuration, credential precedence, and recovery
are described in the [Kindle guide](../docs/guides/kindle.md). Configure it under
Settings → Kindle. Delivery is disabled by default; it requires Postgres and an
authenticated TLS SMTP sender. New source builds include migration 0059; older
published images do not include this feature.
