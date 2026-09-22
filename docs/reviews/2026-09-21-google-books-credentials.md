# Google Books credential setup — September 21, 2026

Created project `borchetta-librarry` in the owner's personal Google account and
enabled `books.googleapis.com` after explicit approval of the API/Books terms.
Created **Librarry Google Books backend**, restricted to **Books API**. It is a
public-metadata API key, with no OAuth client or service-account binding. No
application/IP restriction was added; the key is stored and used server-side.

[Manage the credential](https://console.cloud.google.com/apis/credentials/key/efd873aa-9741-4b3b-8c6a-1e1db06f404f?project=borchetta-librarry).

The local copy is outside the repository in an owner-only file under
`~/.config/librarry/`. The key was never printed in tool output or committed.
TrueNAS `app.update` saved it as `LIBRARRY_GOOGLE_BOOKS_API_KEY` in Librarry's
persistent custom app configuration. The previous configuration was backed up
under `/root/librarry-credential-backups/` with owner-only permissions. The API
restarted; saved image references were verified unchanged. This did not publish
or deploy the unreleased ranking/series changes.

Verified:

- Direct credentialed Google Books ISBN lookup for `9780593135204`: HTTP 200,
  one result, *Project Hail Mary*.
- TrueNAS configuration readback: credential persisted; app state `RUNNING`.
- Live `/healthz`: HTTP 200, `ok: true`.
- Live `POST /api/v1/providers/Google%20Books/check` at
  `2026-09-22T00:51:58Z` (September 21 locally): `ready`, `configured: true`,
  `reachable: true`, `authenticated: true`, successful provider lookup.

This establishes credential and basic lookup readiness. It is not full Google
fallback/ranking qualification. Hardcover still requires its own token.
