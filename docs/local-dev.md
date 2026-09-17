# Local development

Run commands from the repository root unless a step says otherwise. Operator
configuration and recovery instructions are in the [guides](README.md#operate).

## Requirements

- Go 1.26.8 and Node.js 22, pinned in [`.mise.toml`](../.mise.toml).
- Docker with Compose and a context permitted to run disposable containers.
- PostgreSQL 16 for persisted application and integration-test behavior.

If you use mise, run `mise install`. Install frontend dependencies once:

```bash
(cd web && npm ci)
```

## Backend

Use a dedicated development database. For an existing local PostgreSQL service:

```bash
LIBRARRY_DATABASE_URL=postgres://librarry:librarry@127.0.0.1:5432/librarry?sslmode=disable \
  go run ./backend/cmd/librarry
```

The URL above is an example, not a database provisioning command. Without a
URL the API can serve provider search without persistence, but library and
worker flows require PostgreSQL. Do not use production credentials or mounts.

## Frontend

In another terminal:

```bash
cd web
npm run dev
```

Open `http://127.0.0.1:5173`. Vite proxies `/api` to `http://127.0.0.1:8080`;
set `LIBRARRY_DEV_API` when using another development API address.

## Source-Build Compose

From the repository root:

```bash
cd deploy
cp .env.example .env
```

Edit `.env` with development-only credentials and writable disposable config/media
paths. Replace every credential placeholder; align the database password and URL.
Then, from `deploy`:

```bash
docker compose -f docker-compose.build.yml config -q
docker compose -f docker-compose.build.yml up --build
```

Open `http://127.0.0.1:30200` unless you changed `LIBRARRY_WEB_PORT`. The
source-build stack also exposes the API on port 8080 for development.

The default `docker-compose.yml` pulls published images. Adding `--build` to
that file does not build the checkout. Use the explicit build file above.
For published installs and persistent path setup, use [deployment](deployment.md).

## Reproducible verification

From the repository root:

```bash
go vet ./...
scripts/test-integration.sh
scripts/check-deployment.sh
(cd web && npm test && npm run build)
git diff --check
```

The integration script creates its own disposable PostgreSQL container; each Go
integration test creates and drops a unique database. To use an existing test
server, set `LIBRARRY_TEST_DATABASE_URL`. Production `LIBRARRY_DATABASE_URL` is
not used by this harness. Plain `go test ./...` without a test URL skips database
cases and does not establish integration qualification.

### Browser checks

Supply a dedicated, running PostgreSQL 16 test database, then run:

```bash
cd web
npx playwright install chromium
LIBRARRY_TEST_DATABASE_URL=postgres://postgres:librarry-test@127.0.0.1:15432/librarry_test?sslmode=disable npm run test:browser
```

The URL is an example; the integration script removes its own container when it
finishes. Browser tests start API/Vite processes on 18182/15173 without inherited
provider/client credentials. They apply migrations to the supplied database.
Artifacts go under `output/playwright/`.

### Packaged checks

For an already built or pulled API/web pair:

```bash
DOCKER_CONTEXT=your-test-context EXPECTED_COMMIT=your-source-sha \
  python3 scripts/test-packaged.py your-api-image your-web-image
DOCKER_CONTEXT=your-test-context \
  python3 scripts/test-worker-packaged.py your-api-image
DOCKER_CONTEXT=your-test-context \
  python3 scripts/test-notification-packaged.py your-api-image
```

These create isolated databases, media and client/receiver fixtures. They cover
imports, restart recovery, identity, authentication, restore, worker ownership
and notification replay. They do not contact real receivers or grab indexer
results. The [Calibre fixture](guides/imports.md#qualify-the-calibre-http-client-against-a-real-server)
uses a disposable real Content Server.

CI runs Go vet/race/Postgres, frontend, browser, deployment and Calibre checks,
then packaged qualification and image scanning before multiarchitecture builds.
The configured scan gate rejects fixable HIGH/CRITICAL findings; lower severity
and unfixed findings are outside that gate. See the [release checklist](release-checklist.md)
for the additional runtime, rollback and live observation requirements.

## Configuration and contributors

Malformed boolean, numeric and duration settings fail startup. Durations accept
unit strings or legacy integer minutes and must be positive. Completed-download
import mode accepts `hardlinkOrCopy`, `hardlink` or `copy`.

Forms/Basic authentication requires PostgreSQL and usable credentials. Unknown
methods and unreadable persisted auth settings are errors. Explicit environment
credentials/methods are owned by the environment and cannot be overwritten in
the UI. See [authentication](guides/operations.md#authentication) and
[contribution guidelines](../CONTRIBUTING.md).
