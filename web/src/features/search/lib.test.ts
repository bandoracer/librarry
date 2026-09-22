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

import { searchResultMatchLabel, searchResultMatchChips, searchResultEvidenceSummary, searchResultCover, searchResultWantedReviewReasons } from "./lib";

describe("discovery evidence", () => {
  const result: SearchResult = { provider: "Open Library", kind: "book", work: { id: "openlibrary:OL1W", title: "Novel", coverUrl: "work.jpg" }, edition: { id: "openlibrary:OL2M", title: "Novel", format: "any", coverUrl: "edition.jpg" }, score: 0.99, confidence: "high", matchedOn: ["isbn"], evidence: ["Exact ISBN", "Edition format unknown"] };
  it("does not present a ranking score as a probability", () => {
    expect(searchResultMatchLabel(result)).toBe("Exact ISBN");
    const labels = JSON.stringify([searchResultMatchChips(result), searchResultEvidenceSummary(result, "ebook")]);
    expect(labels).not.toContain("99%");
    expect(labels).not.toContain("without review");
    expect(labels).toContain("Edition format unknown");
  });
  it("prefers the selected edition cover with a work fallback", () => {
    expect(searchResultCover(result)).toBe("edition.jpg");
    expect(searchResultCover({ ...result, edition: { ...result.edition!, coverUrl: "" } })).toBe("work.jpg");
  });
  it("requires review for unknown format or explicit conflicts even with exact ISBN", () => {
    const reasons = searchResultWantedReviewReasons({ ...result, conflicts: ["Edition language differs from your preference"] });
    expect(reasons).toContain("Edition language differs from your preference");
    expect(reasons.some(reason => reason.includes("format is unknown"))).toBe(true);
  });
});

import { groupSearchEditions } from "./lib";

it("groups exact works without merging edition or adaptation evidence", () => {
  const base: SearchResult = { provider: "Hardcover", kind: "book", work: { id: "hardcover:1", title: "Novel" }, edition: { id: "hardcover-edition:1", title: "Novel", format: "ebook" }, confidence: "high", score: 1, matchedOn: [] };
  const audio = { ...base, edition: { id: "hardcover-edition:2", title: "Novel", format: "audiobook" as const } };
  const adaptation = { ...base, work: { ...base.work, id: "hardcover:2" } };
  const editionOnly = { ...base, work: { ...base.work, id: "openlibrary:OL123M" } };
  const groups = groupSearchEditions([base, audio, adaptation, editionOnly, { ...editionOnly }]);
  expect(groups.map(group => group.length)).toEqual([2, 1, 1, 1]);
  expect(groups[0][0]).toBe(base);
  expect(groups[0][1]).toBe(audio);
  expect(base.edition?.format).toBe("ebook");
});
