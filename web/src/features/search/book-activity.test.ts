import { describe, expect, it } from "vitest";
import { activityMatches, bookActivityLabel, type BookActivity } from "./book-activity";
import type { SearchResult, WantedItem } from "../../lib/api";
const result = { provider: "Hardcover", kind: "book", work: { id: "hardcover:1", title: "Book", providerIds: ["openlibrary:OL1W"] }, edition: { id: "hardcover-edition:1", title: "Book", format: "ebook" }, matchedOn: [], score: 1, confidence: "high" } as SearchResult;
const action: BookActivity = { result, format: "ebook", phase: "searching" };

describe("book download state", () => {
  it("shares duplicate protection across editions and proven aliases, keeping formats and namesakes independent", () => {
    expect(activityMatches(action, { ...result, edition: { ...result.edition!, id: "hardcover-edition:2" } }, "ebook")).toBe(true);
    expect(activityMatches(action, { ...result, work: { id: "openlibrary:OL1W", title: "Book" }, edition: undefined }, "ebook")).toBe(true);
    expect(activityMatches(action, result, "audiobook")).toBe(false);
    expect(activityMatches(action, { ...result, work: { id: "hardcover:99", title: "Book" }, edition: undefined }, "ebook")).toBe(false);
  });
  it("keeps local progress and failure visible while saved-state queries catch up", () => {
    const match = { key: "book", total: 1, books: [{ status: "wanted" } as WantedItem] };
    expect(bookActivityLabel(action, match)).toBe("Finding download");
    expect(bookActivityLabel({ ...action, phase: "queued" }, match)).toBe("Queued");
    expect(bookActivityLabel({ ...action, phase: "error" }, match)).toBe("Needs attention");
    expect(bookActivityLabel({ ...action, phase: "queued" }, { ...match, books: [{ status: "grabbed", derivedState: "downloaded" } as WantedItem] })).toBe("In library");
    expect(bookActivityLabel(undefined, { ...match, books: [{ status: "removed" } as WantedItem] })).toBe("Removed");
  });
});

it("shows durable import review, idle transfers, and outage state instead of queued", () => {
  for (const [downloadState, label] of [["stalled", "Stalled"], ["waiting_metadata", "Waiting for metadata"], ["import_ready", "Waiting for import"], ["paused", "Paused"]]) {
    const book = { status: "grabbed", derivedState: "downloading", downloadState } as WantedItem;
    expect(bookActivityLabel({ ...action, phase: "queued" }, { key: "book", total: 1, books: [book] })).toBe(label);
    expect(bookActivityLabel(undefined, { key: "book", total: 1, books: [{ ...book, importReviewId: "review" }] })).toBe("Needs import review");
    expect(bookActivityLabel(undefined, { key: "book", total: 1, books: [{ ...book, derivedState: "downloaded" }] })).toBe("In library");
  }
  expect(bookActivityLabel(undefined, { key: "book", total: 1, books: [{ status: "grabbed", derivedState: "unknown" } as WantedItem] })).toBe("Status unavailable");
});
