import { test, expect } from "@playwright/test";

const book = (id: number, title: string, discoverySection?: string) => ({
  provider: "Hardcover", kind: "book", discoverySection,
  work: { id: `hardcover:${id}`, title, authors: [{ id: "hardcover-author:1", name: "Writer" }] },
  edition: { id: `hardcover-edition:${id}`, title, format: "ebook", language: "English" },
  score: 0.75, confidence: "medium", matchedOn: []
});

test("secondary results are collapsed and retain their own add identity", async ({ page }, testInfo) => {
  let payload: any;
  const summary = { ...book(2, "Summary of Novel", "related"), contentLabel: "Possible graphic adaptation" };
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [book(1, "Novel"), summary, book(3, "Incomplete Novel", "incomplete")] } }));
  await page.route("**/api/v1/library/book-matches", route => route.fulfill({ json: { matches: route.request().postDataJSON().candidates.map((candidate: { key: string }) => ({ key: candidate.key, total: 0, books: [] })) } }));
  await page.route("**/api/v1/wanted", route => { payload = route.request().postDataJSON(); return route.fulfill({ status: 503, json: { error: "Fixture prevents persistence" } }); });
  await page.goto("/search?query=Novel");
  const rows = page.locator(".search-result-row");
  await expect(rows.filter({ visible: true })).toHaveCount(1);
  await expect(page.getByText("Related books and companion material (1)", { exact: true })).toBeVisible();
  await expect(page.getByText("Incomplete catalog records (1)", { exact: true })).toBeVisible();
  await page.screenshot({ path: `../output/playwright/search-sections-${testInfo.project.name}.png`, fullPage: true });
  await page.getByText("Related books and companion material (1)", { exact: true }).click();
  await expect(rows.filter({ hasText: "Summary of Novel" })).toContainText("Possible graphic adaptation");
  await rows.filter({ hasText: "Summary of Novel" }).click();
  await expect(page.locator(".search-detail-badges").getByText("Possible graphic adaptation")).toBeVisible();
  await page.getByRole("button", { name: "Add Book", exact: true }).click();
  await expect.poll(() => payload?.result.work.id).toBe("hardcover:2");
  expect(payload.result.edition).toEqual(summary.edition);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
});

test("a search with only incomplete records expands them automatically", async ({ page }) => {
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [book(3, "Rare catalog record", "incomplete")] } }));
  await page.route("**/api/v1/library/book-matches", route => route.fulfill({ json: { matches: route.request().postDataJSON().candidates.map((candidate: { key: string }) => ({ key: candidate.key, total: 0, books: [] })) } }));
  await page.goto("/search?query=rare");
  await expect(page.locator(".search-result-row")).toBeVisible();
});
