# Metadata discovery release — September 21, 2026

## Candidate

Improved provider retrieval, coherent edition normalization, identity-safe merging,
explainable search evidence, edition selection, and explicit bounded series search.
No database migration is included. Prior benchmark and implementation records are
linked from [status](../status.md).

Credentialed qualification uncovered a live API restriction absent from schema
validation: Hardcover blocks `_ilike`. The corrected adapter uses its dedicated
Series search (three candidates), followed by explicit-ID membership lookup (25
non-compilation memberships each). Provider-featured records lead retrieval so
sparse duplicates do not exhaust the bounded window; the flag is not treated as
proof of a primary novel. Verified ebook/audio editions precede work-only records,
with provider positions preserved within each group. Catalog omissions and bad
membership flags remain visible evidence limitations.

## Read-only provider checks

- Hardcover authentication succeeds with the configured personal API key.
- Percy Jackson series search returns The Lightning Thief first, coherent
  ebook/audio editions, and source positions for subsequent volumes.
- Project Hail Mary title search returns Andy Weir's ebook/audio editions first.
- Ebook ISBN 9780593135211 returns the matching Project Hail Mary edition.
- Rick Riordan author search returns stable identity `hardcover-author:87224` first.
- Both Hardcover and Google Books pass the deployed application's credential checks.

## Release gates

Local validation and GitHub packaging are in progress. This record does not yet
assert deployment. The rollout must preserve the existing provider credentials,
mounts, database and media; only the paired API/web image references change.
