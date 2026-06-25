# Local Development

## Backend

```bash
go test ./...
LIBRARRY_DATABASE_URL=postgres://librarry:librarry@127.0.0.1:5432/librarry?sslmode=disable \
  go run ./backend/cmd/librarry
```

If `LIBRARRY_DATABASE_URL` is omitted, the API starts without persistence so
provider search and health endpoints can be developed independently.

## Frontend

```bash
cd web
npm install
npm run dev
```

The Vite dev server proxies `/api` to `http://127.0.0.1:8080`.

## Compose

```bash
cd deploy
cp .env.example .env
docker compose up --build
```

## TrueNAS Custom App

`deploy/truenas/install.yaml` is a paste-ready shape for TrueNAS custom apps,
but it intentionally contains placeholder secrets. Build `librarry-api:local`
and `librarry-web:local` on the TrueNAS host or replace the images with registry
tags before installing the custom app. Do not commit a real Prowlarr API key.

The default TrueNAS port is `192.168.1.221:30200`, with `/mnt/HDD_pool/vault/media-stack`
mounted into the API container as `/data`.
