import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { libraryQueryOptions } from "./queries";
import { fetchWanted } from "./api";
import { withDemoFallback } from "./demo";

afterEach(() => vi.unstubAllGlobals());

describe("library query contract", () => {
  it("does not serialize TanStack context and retains imported books", async () => {
    const fetch = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => new Response(JSON.stringify({ wanted: [{ id: "one", status: "imported" }] })));
    vi.stubGlobal("fetch", fetch);
    const client = new QueryClient();
    try {
      const rows = await client.fetchQuery(libraryQueryOptions());
      expect(fetch.mock.calls[0]?.[0]).toBe("/api/v1/wanted?view=library");
      expect(rows.map(row => row.id)).toEqual(["one"]);
    } finally { client.clear(); }
  });
  it("normalizes an empty persisted collection", async () => {
    vi.stubGlobal("fetch", async () => new Response('{"wanted":null}'));
    expect(await fetchWanted("library")).toEqual([]);
  });
  it("never passes callback context to optional fetcher arguments", async () => {
    const fetcher = vi.fn(async () => []);
    const client = new QueryClient();
    try {
      await client.fetchQuery({ queryKey: ["fixture"], queryFn: withDemoFallback(fetcher, () => []) });
      expect(fetcher).toHaveBeenCalledWith();
    } finally { client.clear(); }
  });
  it("propagates real failures instead of demo data", async () => {
    await expect(withDemoFallback(async () => { throw new Error("offline"); }, () => ["fake"])()).rejects.toThrow("offline");
  });
});

describe("direct book lookup", () => {
  it("returns null only for a missing book and propagates outages", async () => {
    const { fetchWantedItem } = await import("./api");
    vi.stubGlobal("fetch", async () => new Response('{"error":"missing"}', { status: 404 }));
    expect(await fetchWantedItem("book")).toBeNull();
    vi.stubGlobal("fetch", async () => new Response('{"error":"offline"}', { status: 503 }));
    await expect(fetchWantedItem("book")).rejects.toThrow("offline");
  });
  it("filters files by the book before applying the collection limit", async () => {
    const { fetchLibraryFiles } = await import("./api");
    const fetch = vi.fn(async () => new Response('{"files":[]}'));
    vi.stubGlobal("fetch", fetch);
    await fetchLibraryFiles("any", 100, "target-book");
    expect(fetch).toHaveBeenCalledWith("/api/v1/library/files?limit=100&wantedId=target-book", expect.any(Object));
  });
});
