import { describe, expect, it } from "vitest";
import type { WantedItem } from "../../lib/api";
import { libraryWantedAuthorPath } from "./lib";

describe("author identity links", () => {
  const book = { id: "book", title: "Book", authorName: "Primary Writer", format: "ebook", status: "wanted", monitored: true, qualityProfile: "standard" } as WantedItem;
  it("links the displayed writer when coauthors sort earlier", () => {
    expect(libraryWantedAuthorPath({ ...book, authors: [{ id: "second", name: "Another Writer" }, { id: "primary", name: "Primary Writer" }] })).toBe("/library/author/primary");
  });
  it("uses a legacy name lookup when an override removed identity evidence", () => {
    expect(libraryWantedAuthorPath({ ...book, authorName: "Corrected Name", authors: [] })).toBe("/library/author/corrected%20name");
  });
});
