# Contributing to Librarry

Librarry is an early alpha book manager. Metadata edge cases, recoverable import
workflows, deployment feedback and clear operator controls are useful areas for
contributions. Read [current status](docs/status.md) before assuming a feature
is released or production-qualified.

## Before changing code

- Check existing issues and pull requests. For a substantial feature, describe
  the concrete operator problem before starting a broad implementation.
- Keep download-client controls scoped to book acquisition and import handoff.
- Preserve provider provenance and manual overrides. Do not add Goodreads,
  Amazon or Audible scraping to core.
- Keep database migrations append-only. Preserve existing file identities and
  associations; design failure/restart behavior alongside the successful path.
- Never commit credentials, database dumps, private library inventories or
  unredacted support logs. See [security reporting](SECURITY.md) for sensitive bugs.

## Set up and verify

Follow [local development](docs/local-dev.md) for Go 1.26.8, Node.js 22, Docker
and disposable PostgreSQL 16 setup. The default Compose file pulls images;
the explicitly named source-build file builds your checkout.

Choose checks relevant to the change:

| Change | Verification |
| --- | --- |
| Go/backend behavior | `go vet ./...` and `scripts/test-integration.sh` |
| Web behavior | From `web`: `npm ci`, `npm test`, `npm run build`; relevant browser checks |
| Deployment/environment | `scripts/check-deployment.sh`; render the affected Compose/template files |
| Recovery or packaging | Isolated [packaged checks](docs/local-dev.md#packaged-checks) |
| Documentation | Check relative links/anchors and documented commands; `git diff --check` |

Plain Go tests without `LIBRARRY_TEST_DATABASE_URL` skip database cases. State
what ran and what was skipped. Use legal fixtures and isolated client/receiver
namespaces; never grab arbitrary indexer results or send test notifications to
real people.

## Pull requests

Describe the problem and resulting behavior, including a concrete before/after
example when useful. Keep the change focused. Include relevant validation,
unresolved limitations and any migration or recovery implications.

Update the matching operator guide when behavior changes. Keep current release
claims in [status](docs/status.md) and the [release checklist](docs/release-checklist.md).
The README should remain a short introduction and setup entry point. Record
user-visible changes in [CHANGELOG.md](CHANGELOG.md) under Unreleased.

Image builds and publication are separate. Ordinary PR/main/tag builds cannot
publish; candidate publication requires explicit dispatch. Do not reintroduce
automatic stable aliases or treat a successful build as deployment approval.

The [agent guide](AGENTS.md) contains repository-specific implementation rules.
