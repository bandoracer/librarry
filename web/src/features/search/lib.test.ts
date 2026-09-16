import { describe, expect, it } from "vitest";
import type { SearchResult, WantedItem } from "../../lib/api";
import { searchResultExistingWanted, searchResultWantedFormat } from "./lib";

describe("edition format while changing search filters", () => {
  it("keeps a displayed ebook separate from tracked audio while the next search is pending", () => {
    const result: SearchResult = {
      provider: "Hardcover", kind: "book", work: { id: "hardcover:1", title: "Book" },
      edition: { id: "hardcover-edition:101", title: "Book", format: "ebook" },
      confidence: "high", score: 0.99, matchedOn: ["isbn"]
    };
    const trackedAudio = { id: "audio", format: "audiobook", title: "Book", sourceProvider: "Hardcover", sourceKey: "hardcover-edition:102" } as WantedItem;
    expect(searchResultWantedFormat(result, "audiobook")).toBe("ebook");
    expect(searchResultExistingWanted(result, [trackedAudio], "audiobook")).toBeUndefined();
    const unknown = { ...result, edition: { ...result.edition!, id: "", format: "any" as const } };
    expect(searchResultWantedFormat(unknown, "audiobook")).toBe("audiobook");
  });
});

it("never treats a title or unknown author as provider identity", () => {
  const result: SearchResult = { provider: "Hardcover", kind: "book", work: { id: "hardcover:1", title: "The Book" }, confidence: "high", score: 1, matchedOn: [] };
  const unrelated = { id: "other", format: "ebook", title: "Book", authorName: "", sourceProvider: "Hardcover", sourceKey: "hardcover:2" } as WantedItem;
  expect(searchResultExistingWanted(result, [unrelated], "ebook")).toBeUndefined();
  expect(searchResultExistingWanted(result, [{ ...unrelated, sourceKey: "hardcover:1", title: "Owner title" }], "ebook")?.id).toBe("other");
});
