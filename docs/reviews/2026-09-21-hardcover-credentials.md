# Hardcover account and credential setup — September 21, 2026

## Account and configuration

- Created and email-confirmed the free Hardcover account `@ryanborchetta`.
- The owner created the `Librarry metadata` personal API key. Its actual settings
  are no expiration and 11 read/write scopes, broader than the prepared catalog
  and public-profile read-only recommendation. Librarry uses metadata read queries.
- Stored the key outside the repository in an owner-only local credential file.
  No credential value is included in this record.
- Updated `LIBRARRY_HARDCOVER_TOKEN` through TrueNAS `app.update`, preserving
  the other configuration and API/web image references. The previous configuration
  was backed up in a root-only directory on the NAS. App state returned `RUNNING`.
- The ranking/series changes remain local and unreleased.

## Live verification

At September 22, 2026, 01:13 UTC (September 21 local):

- LAN `/healthz`: HTTP 200, `ok: true`.
- `POST /api/v1/providers/Hardcover/check`: HTTP 200, ready, configured,
  reachable and authenticated.
- Google Books check after the service restart: HTTP 200, ready, reachable and
  authenticated, confirming its credential remains functional.
- Production title search for `Project Hail Mary`: 10 results, no provider errors;
  first result from Hardcover with a cover. Production still includes derivative
  summary results; this is connection evidence, not ranking qualification.

## Candidate-code probes

A temporary Go harness called the current worktree's Hardcover adapter directly,
using the credential from the local protected file and making only read requests:

- `Project Hail Mary`: six normalized results. First: Andy Weir's work
  `hardcover:427578`, ebook edition `hardcover-edition:30405492`, English,
  ISBN 9780593135211, publisher Ballantine Books, with work and edition covers.
- ISBN `9780593135204`: valid successful response with zero normalized results.
  This probe does not establish exact-ISBN coverage.
- Series `Percy Jackson and the Olympians`: zero results, provider denied access.
  Official-schema validation was insufficient to establish runtime query support;
  investigate provider restrictions before shipping series mode.

The temporary harness was removed. No library additions, downloads, or purchases
were performed. Broader credentialed ranking, author, series and list qualification
remain open.

## Deployment preparation follow-up

The denied series query used `_ilike`, which the live API explicitly forbids.
Series discovery now uses the provider's `query_type: "Series"` search and queries
memberships by returned IDs. A repeated credentialed probe succeeded, with
The Lightning Thief first and explicit numbered memberships. Ebook ISBN
9780593135211 returned the correct Project Hail Mary edition, and author search
returned Rick Riordan with stable ID `hardcover-author:87224` first. See the
[rollout record](2026-09-21-metadata-rollout.md) for release state.
