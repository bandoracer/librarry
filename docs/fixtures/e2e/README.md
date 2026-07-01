# E2E Download Fixture

This directory contains a tiny EPUB and matching torrent used for Librarry
end-to-end acquisition tests.

- `librarry-public-domain-e2e-book.epub` is a generated EPUB fixture dedicated
  to the public domain under CC0.
- `librarry-public-domain-e2e-book.torrent` points at the EPUB through a GitHub
  raw web seed so qBittorrent can complete the download without depending on a
  public swarm.

The fixture exists only to validate the legal download flow:

1. Add the torrent through Librarry.
2. Start it in qBittorrent from Librarry's queue.
3. Verify qBittorrent reaches a completed state.
4. Verify the EPUB exists on the configured download path.
