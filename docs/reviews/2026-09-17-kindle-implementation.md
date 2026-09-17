# Kindle delivery implementation verification

Source branch: `codex/send-to-kindle`, based on main `36f19ae`.
This is implementation qualification; no production deployment or delivery to a
physical Kindle was performed during these implementation checks. Subsequent
publication, deployment and SMTP acceptance are recorded in the
[rollout report](2026-09-17-kindle-rollout.md).

## Behavior

Settings → Kindle supports authenticated SMTP with implicit TLS or mandatory
STARTTLS, redacted credentials, a Resend preset, a saved Kindle destination and
an explicit test document. Active ebook detail pages offer present native EPUB
and PDF files. Files must be under a registered ebook root (or the legacy root
when none are registered), be associated only with the selected book, have no
unfinished import, and pass bounded size, checksum and format checks.

Attempt identities and SMTP outcomes survive restart. Replaying an existing
request does not resubmit it, including uncertain outcomes. A database uniqueness
constraint also prevents concurrent attempts for the same file and recipient.
No automatic delivery retry or send-on-import is enabled. SMTP acceptance is
shown separately from device arrival.

## Completed checks

- Full `go test -race -timeout 20m ./...` passed against disposable Postgres 16.
  Focused Kindle/API race tests were rerun after final service changes and passed.
- Tests cover settings validation, redaction, password preservation/clearing,
  environment defaults, API authentication, malformed requests, empty history,
  cross-book selection, missing/removed files, root escape, changed hashes/sizes,
  invalid EPUBs, oversized files, MIME attachment byte preservation, and both TLS
  modes with successful, explicitly rejected and uncertain SMTP outcomes.
- Durable database tests cover duplicate and concurrent requests, process-restart
  replay, interrupted sends, manual retries and preservation of unknown history.
- `go vet ./...`, frontend build, 25 frontend unit tests, Compose validation for
  all four templates, and `git diff --check` passed.
- Interactive browser checks used a local fixture library and a loopback-only
  SMTP receiver. Settings saved while retaining the password. An untrusted SMTP
  certificate produced a visible failed attempt and no received document. After
  a fixture-only launcher trusted that certificate, the test document and EPUB
  were accepted and received. The fixture trust hook was removed after building
  the temporary launcher; production code continues to verify certificates.
- Desktop and 390×844 phone-width views were checked. The saved recipient,
  document selector, submission result and durable history were usable. Invalid
  recipient validation was exercised, and unsaved changes prevented test sends.
  Reload restored the saved values. Browser request IDs use `getRandomValues`,
  so they do not require a secure context on a plain-HTTP LAN address.

## Remaining qualification at this checkpoint

Build/publish/deploy this branch before using the feature on the live NAS. Add
`books@mail.borchetta.xyz` to Amazon's approved senders, configure the owner's
Kindle address and explicitly send a test document to confirm Amazon processing
and physical-device receipt. The existing Resend key and verified domain were
not used by this fixture run. Calibre conversion/export remains separate.
