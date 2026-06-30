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
- Media/download mount: `/mnt/user/media-stack`
- Web UI: `http://tower.local:30200`
- Container media mount: `/data`

Set `LIBRARRY_WEB_ORIGIN` to the hostname you actually use for the web UI, such
as `http://tower.local:30200` or a reverse-proxy URL. Set `LIBRARRY_API_KEY`
before exposing Librarry outside trusted LAN access.

To update:

```bash
docker compose pull
docker compose up -d
```

To pin a release, replace `:latest` in `.env` with a published version tag.

After the first GHCR publish, verify the `librarry-api` and `librarry-web`
packages are public in GitHub's package settings before installing on an Unraid
host that does not authenticate to GHCR.
