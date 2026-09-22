import { test, expect } from "@playwright/test";

test("search shows edition evidence and reviews conflicts without percentage scores", async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  const result = {
    provider: "Open Library", kind: "book", work: { id: "openlibrary:OL1W", title: "The Lightning Thief", authors: [{ id: "openlibrary:OL1A", name: "Rick Riordan" }], coverUrl: "/work-cover.jpg" },
    edition: { id: "openlibrary:OL2M", workId: "openlibrary:OL1W", title: "Percy Jackson and the Lightning Thief", format: "any", language: "eng", isbns: ["9781423103349"], coverUrl: "/edition-cover.jpg" },
    score: 0.99, confidence: "review", matchedOn: ["isbn"], evidence: ["Exact ISBN", "english edition", "Edition format unknown"], conflicts: ["Fixture edition conflict"]
  };
  let editionRequests = 0, workRequests = 0, adds = 0;
  await page.route("**/edition-cover.jpg", route => { editionRequests++; return route.fulfill({ status: 404 }); });
  await page.route("**/work-cover.jpg", route => { workRequests++; return route.fulfill({ status: 404 }); });
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [result], providerErrors: [] } }));
  await page.route("**/api/v1/library/book-matches", route => route.fulfill({ json: { matches: route.request().postDataJSON().candidates.map((c: { key: string }) => ({ key: c.key, total: 0, books: [] })) } }));
  await page.route("**/api/v1/wanted", route => { adds++; return route.fulfill({ status: 500, json: {} }); });
  await page.goto("/search?query=9781423103349");
  const row = page.locator(".search-result-row");
  await expect(row).toContainText("Exact ISBN");
  await expect(row).toContainText("Edition format unknown");
  await expect(row).not.toContainText("99%");
  await expect.poll(() => editionRequests).toBeGreaterThan(0);
  await expect(row.locator("img")).toHaveCount(0);
  expect(workRequests).toBe(0);
  await row.click();
  await expect(page.getByLabel("Selected metadata evidence")).toContainText("Fixture edition conflict");
  await page.getByRole("button", { name: "Review & Add Book", exact: true }).click();
  const review = page.getByRole("dialog", { name: "Review before adding" });
  await expect(review).toContainText("Fixture edition conflict");
  await expect(review).toContainText("format is unknown");
  expect(adds).toBe(0);
  await review.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/search-evidence-${testInfo.project.name}.png`, fullPage: true });
  expect(errors).toEqual([]);
});

test("partial provider failure retains usable search results and explains the missing source", async ({ page }) => {
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: {
    results: [{ provider: "Open Library", kind: "book", work: { id: "openlibrary:OL1W", title: "Available novel", authors: [] }, score: 0.5, confidence: "review", matchedOn: [] }],
    providerErrors: [{ provider: "Hardcover", message: "Provider rate limit is active" }]
  } }));
  await page.route("**/api/v1/library/book-matches", route => route.fulfill({ json: { matches: route.request().postDataJSON().candidates.map((c: { key: string }) => ({ key: c.key, total: 0, books: [] })) } }));
  await page.goto("/search?query=novel");
  await expect(page.locator(".search-result-row")).toContainText("Available novel");
  await expect(page.getByRole("status")).toContainText("Hardcover: Provider rate limit is active");
});

test("edition chooser keeps grouped work identities and selected add payload intact", async ({ page }, testInfo) => {
  const base = { provider: "Hardcover", kind: "book", work: { id: "hardcover:71", title: "One Novel", authors: [{ id: "hardcover-author:4", name: "Fixture Writer" }] }, score: 0.99, confidence: "high", matchedOn: ["isbn"] };
  const ebook = { ...base, edition: { id: "hardcover-edition:101", workId: "hardcover:71", title: "One Novel", format: "ebook", language: "English", isbns: ["9780142437247"], publisher: "Ebook Press", coverUrl: "/ebook.svg" } };
  const audio = { ...base, edition: { id: "hardcover-edition:102", workId: "hardcover:71", title: "One Novel", format: "audiobook", language: "English", isbns: ["9781423103349"], publisher: "Audio Press", coverUrl: "/audio.svg" } };
  const adaptation = { ...ebook, work: { ...base.work, id: "hardcover:72", title: "Graphic adaptation" }, edition: { ...ebook.edition, id: "hardcover-edition:103", workId: "hardcover:72", title: "Graphic adaptation" } };
  let created: any;
  await page.route("**/*.svg", route => route.fulfill({ contentType: "image/svg+xml", body: '<svg xmlns="http://www.w3.org/2000/svg" width="60" height="90"><rect width="60" height="90" fill="blue"/></svg>' }));
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [ebook, audio, adaptation] } }));
  await page.route("**/api/v1/library/book-matches", route => {
    const { candidates } = route.request().postDataJSON();
    expect(candidates).toHaveLength(3);
    return route.fulfill({ json: { matches: candidates.map((c: { key: string }) => ({ key: c.key, total: 0, books: [] })) } });
  });
  await page.route("**/api/v1/wanted", route => { created = route.request().postDataJSON(); return route.fulfill({ status: 503, json: { error: "Fixture prevents persistence" } }); });
  await page.goto("/search?query=One Novel");
  await expect(page.locator(".search-result-row")).toHaveCount(2);
  const row = page.locator(".search-result-row").filter({ hasText: "One Novel" });
  await expect(row).toContainText("2 editions");
  await row.click();
  const selector = page.getByLabel("Edition", { exact: true });
  await expect(selector.locator("option")).toHaveCount(2);
  await selector.selectOption({ label: "audiobook · English · Audio Press · hardcover-edition:102" });
  await expect(page.locator(".search-detail-cover img")).toHaveAttribute("src", "/audio.svg");
  await expect(page.getByLabel("Selected metadata evidence")).toContainText("hardcover-edition:102");
  await expect(page.getByLabel("Selected metadata evidence")).toContainText("Audio Press");
  await page.screenshot({ path: `../output/playwright/edition-chooser-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "Add Book", exact: true }).click();
  await expect.poll(() => created?.format).toBe("audiobook");
  expect(created.result.edition).toEqual(audio.edition);
  expect(created.result.work.id).toBe("hardcover:71");
  expect(created.preserveExisting).toBe(true);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
});

test("series mode preserves source order, overlapping memberships and missing-credential errors", async ({ page }) => {
  let available = true;
  await page.route("**/api/v1/search?**", route => {
    expect(new URL(route.request().url()).searchParams.get("type")).toBe("series");
    return route.fulfill({ json: available ? { results: [
      { provider: "Hardcover", kind: "book", work: { id: "hardcover:1", title: "First book", authors: [], seriesId: "hardcover-series:3", series: "Test", seriesPosition: "0" }, score: 0.5, confidence: "review", matchedOn: ["series"] },
      { provider: "Hardcover", kind: "book", work: { id: "hardcover:2", title: "Later book", authors: [], seriesId: "hardcover-series:3", series: "Test", seriesPosition: "1.5" }, score: 0.5, confidence: "review", matchedOn: ["series"] },
      { provider: "Hardcover", kind: "book", work: { id: "hardcover:1", title: "First book", authors: [], seriesId: "hardcover-series:9", series: "Test extended", seriesPosition: "2" }, score: 0.5, confidence: "review", matchedOn: ["series"] }
    ] } : { results: [], providerErrors: [{ provider: "Hardcover", message: "Series search requires a configured Hardcover token" }] } });
  });
  await page.route("**/api/v1/library/book-matches", route => route.fulfill({ json: { matches: route.request().postDataJSON().candidates.map((c: { key: string }) => ({ key: c.key, total: 0, books: [] })) } }));
  await page.goto("/search?mode=series&query=Test");
  await expect(page.getByLabel("Series query")).toHaveValue("Test");
  const rows = page.locator(".search-result-row");
  await expect(rows).toHaveCount(3);
  await expect(rows.nth(0)).toContainText("Test #0");
  await expect(rows.nth(1)).toContainText("Test #1.5");
  await expect(rows.nth(2)).toContainText("Test extended #2");
  await expect(page.getByText("Hardcover series order.", { exact: false })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  available = false;
  await page.getByRole("button", { name: "Search", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("requires a configured Hardcover token");
  await expect(rows).toHaveCount(0);
});
