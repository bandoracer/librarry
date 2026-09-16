import { describe, expect, it } from "vitest";
import type { WantedItem } from "../../lib/api";
import { libraryWantedAuthorPath, wantedItemBookState, summarizeWantedItems, presenceLabel } from "./lib";

describe("author identity links", () => {
  const book = { id: "book", title: "Book", authorName: "Primary Writer", format: "ebook", status: "wanted", monitored: true, qualityProfile: "standard" } as WantedItem;
  it("links the displayed writer when coauthors sort earlier", () => {
    expect(libraryWantedAuthorPath({ ...book, authors: [{ id: "second", name: "Another Writer" }, { id: "primary", name: "Primary Writer" }] })).toBe("/library/author/primary");
  });
  it("uses a legacy name lookup when an override removed identity evidence", () => {
    expect(libraryWantedAuthorPath({ ...book, authorName: "Corrected Name", authors: [] })).toBe("/library/author/corrected%20name");
  });
});


describe("file evidence presentation", () => {
 const book = { id: "book", format: "audiobook", status: "imported", monitored: true } as WantedItem;
 it("keeps partial and unavailable states instead of falling back to imported status", () => {
  for (const state of ["unknown", "incomplete"] as const) {
   const item = { ...book, derivedState: state };
   expect(wantedItemBookState(item, [])).toBe(state);
   expect(presenceLabel(state)).toBe(state === "unknown" ? "Unknown" : "Incomplete");
   const summary = summarizeWantedItems([item], new Map([[item.id, state]]));
   expect(summary[state]).toBe(1);
   expect(summary.missing + summary.downloaded).toBe(0);
  }
 });
});
