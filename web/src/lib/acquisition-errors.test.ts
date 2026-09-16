import { afterEach, describe, expect, it, vi } from "vitest";
import { grabRelease, grabWanted, searchReleases, searchWantedReleases, type Release } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("release search failures", () => {
  const searches = [() => searchReleases("Fixture", "ebook"), () => searchWantedReleases("fixture")];

  it.each(searches)("directs an unconfigured indexer to its settings", async (search) => {
    vi.stubGlobal("fetch", async () => new Response('{"error":"integration is not configured"}', { status: 502 }));
    await expect(search()).rejects.toThrow("Open Settings → Indexers");
  });

  it.each(searches)("preserves an actual provider failure rather than claiming no results", async (search) => {
    vi.stubGlobal("fetch", async () => new Response('{"error":"prowlarr search returned 503 Service Unavailable"}', { status: 502 }));
    await expect(search()).rejects.toThrow("prowlarr search returned 503 Service Unavailable");
  });

  it.each(searches)("retains HTTP status when the server has no error detail", async (search) => {
    vi.stubGlobal("fetch", async () => new Response(null, { status: 502 }));
    await expect(search()).rejects.toThrow("502");
  });
});

describe("grab failures", () => {
  it.each([
    () => grabWanted("fixture"),
    () => grabRelease({ title: "Fixture", downloadUrl: "https://example.invalid/fixture.torrent" } as Release, "ebook"),
  ])("preserves the recovery reason without resubmitting a grab", async (grab) => {
    const request = vi.fn(async () => new Response('{"error":"acceptance is uncertain; reconcile the existing attempt before retrying"}', { status: 409 }));
    vi.stubGlobal("fetch", request);
    await expect(grab()).rejects.toThrow("acceptance is uncertain; reconcile the existing attempt");
    expect(request).toHaveBeenCalledTimes(1);
  });
});
