import { test, expect, type Page } from "@playwright/test";

const result = (id: string, title = "Fixture title", format = "ebook") => ({ provider: "Hardcover", kind: "book", work: { id: `hardcover:${id}`, title, authors: [{ id: "hardcover-author:1", name: "Writer" }] }, edition: { id: `hardcover-edition:${id}`, title, format }, score: 1, confidence: "high", matchedOn: ["isbn"] });
const book = (id: string, status = "wanted") => ({ id, title: "Owner corrected title", authorName: "Owner author", format: "ebook", status, monitored: false, qualityProfile: "premium", tags: ["saved"], updatedAt: "2026-09-16T00:00:00Z" });
const openResult = async (page: Page, title: string) => page.locator(".search-result-row").filter({ hasText: title }).click();

test("search checks saved provider identities, inactive records and ambiguity without capped wanted lists", async ({ page }, testInfo) => {
  const results = [result("1", "Older tracked candidate"), result("2", "Removed candidate"), result("3", "Ambiguous candidate"), result("4", "Same title adaptation")];
  let legacyReads = 0;
  await page.route("**/api/v1/wanted", route => { legacyReads++; return route.fulfill({ json: { wanted: [] } }); });
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results } }));
  await page.route("**/api/v1/library/book-matches", route => {
    const { candidates } = route.request().postDataJSON();
    expect(candidates).toHaveLength(4);
    expect(candidates[0]).toMatchObject({ provider: "Hardcover", workIds: ["hardcover:1"], editionIds: ["hardcover-edition:1"], sourceKey: "hardcover-edition:1", format: "ebook" });
    return route.fulfill({ json: { matches: candidates.map((c: { key: string }, i: number) => ({ key: c.key, total: i === 2 ? 12 : i === 3 ? 0 : 1, books: i === 2 ? Array.from({ length: 10 }, (_, n) => book(`ambiguous-${n}`)) : i === 3 ? [] : [book(i === 0 ? "old-book" : "removed-book", i === 0 ? "wanted" : "removed")] })) } });
  });
  await page.goto("/search?query=fixture");
  await openResult(page, "Older tracked candidate");
  await expect(page.getByLabel("Saved book matches")).toContainText("Owner corrected title");
  await expect(page.getByRole("button", { name: "Open book", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Add Book", exact: true })).toHaveCount(0);
  if (testInfo.project.name === "mobile") await page.keyboard.press("Escape");
  await openResult(page, "Removed candidate");
  await expect(page.getByLabel("Saved book matches")).toContainText("Removed");
  await page.getByRole("button", { name: "Open book", exact: true }).click();
  await expect(page).toHaveURL(/\/library\/book\/removed-book/);
  expect(legacyReads).toBe(0);
  await page.goto("/search?query=fixture");
  await openResult(page, "Ambiguous candidate");
  await expect(page.getByLabel("Saved book matches")).toContainText("Showing 10 of 12 saved matches");
  await expect(page.getByRole("button", { name: "Add Book", exact: true })).toHaveCount(0);
  await page.screenshot({ path: `../output/playwright/search-identity-${testInfo.project.name}.png`, fullPage: true });
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  if (testInfo.project.name === "mobile") await page.keyboard.press("Escape");
  await openResult(page, "Same title adaptation");
  await expect(page.getByRole("button", { name: "Add Book", exact: true })).toBeEnabled();
});

test("search fails closed during incomplete or failed library checks and recovers after retry", async ({ page }) => {
  let failure: "outage" | "incomplete" | "none" = "outage";
  let adds = 0;
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [result("5")] } }));
  await page.route("**/api/v1/wanted", route => { adds++; return route.fulfill({ status: 500, json: {} }); });
  await page.route("**/api/v1/library/book-matches", route => {
    if (failure === "outage") return route.fulfill({ status: 503, json: { error: "Fixture persistence outage" } });
    const { candidates } = route.request().postDataJSON();
    return route.fulfill({ json: { matches: failure === "incomplete" ? [] : candidates.map((c: { key: string }) => ({ key: c.key, total: 0, books: [] })) } });
  });
  await page.goto("/search?query=fixture");
  await openResult(page, "Fixture title");
  const add = page.getByRole("button", { name: "Add Book", exact: true });
  await expect(add).toBeDisabled();
  await expect(page.getByRole("alert")).toContainText("Fixture persistence outage");
  failure = "incomplete";
  await page.getByRole("button", { name: "Retry library check" }).click();
  await expect(page.getByRole("alert")).toContainText("Library identity check was incomplete");
  await expect(add).toBeDisabled();
  failure = "none";
  await page.getByRole("button", { name: "Retry library check" }).click();
  await expect(add).toBeEnabled();
  expect(adds).toBe(0);
});

test("search add sends the preservation guard and reconciles a concurrent add without another mutation", async ({ page }, testInfo) => {
  let exists = false;
  const adds: unknown[] = [];
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [result("6")] } }));
  await page.route("**/api/v1/library/book-matches", route => route.fulfill({ json: { matches: route.request().postDataJSON().candidates.map((c: { key: string }) => ({ key: c.key, total: exists ? 1 : 0, books: exists ? [book("concurrent-book")] : [] })) } }));
  await page.route("**/api/v1/wanted", route => {
    adds.push(route.request().postDataJSON()); exists = true;
    return route.fulfill({ status: 409, json: { error: "This provider identity is already saved" } });
  });
  await page.goto("/search?query=fixture");
  await openResult(page, "Fixture title");
  await page.getByRole("button", { name: "Add Book", exact: true }).click();
  if (testInfo.project.name === "mobile") await openResult(page, "Fixture title");
  await expect(page.getByRole("button", { name: "Open book", exact: true })).toBeVisible();
  expect(adds).toHaveLength(1);
  expect(adds[0]).toMatchObject({ format: "ebook", preserveExisting: true });
  await expect(page.getByRole("button", { name: "Add Book", exact: true })).toHaveCount(0);
});

test("changing search format uses the displayed edition identity and format", async ({ page }) => {
  const formats: string[] = [];
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [result("7")] } }));
  await page.route("**/api/v1/library/book-matches", route => {
    const { candidates } = route.request().postDataJSON();
    formats.push(candidates[0].format);
    return route.fulfill({ json: { matches: candidates.map((c: { key: string }) => ({ key: c.key, total: 0, books: [] })) } });
  });
  await page.goto("/search?query=fixture");
  await expect.poll(() => formats.length).toBeGreaterThan(0);
  await page.getByLabel("Format", { exact: true }).selectOption("audiobook");
  await openResult(page, "Fixture title");
  await expect(page.getByRole("button", { name: "Add Book", exact: true })).toBeEnabled();
  expect(formats.every(format => format === "ebook")).toBe(true);
});

test("unknown edition format uses the same chosen format for identity checks and add", async ({ page }) => {
  let format = "";
  let created: { format: string; preserveExisting: boolean } | undefined;
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [result("8", "Unknown edition", "any")] } }));
  await page.route("**/api/v1/library/book-matches", route => {
    const { candidates } = route.request().postDataJSON(); format = candidates[0].format;
    return route.fulfill({ json: { matches: candidates.map((c: { key: string }) => ({ key: c.key, total: 0, books: [] })) } });
  });
  await page.route("**/api/v1/wanted", route => {
    created = route.request().postDataJSON();
    return route.fulfill({ json: { ...book("new-audio"), format: "audiobook" } });
  });
  await page.goto("/search?query=fixture");
  await expect.poll(() => format).toBe("ebook");
  await page.getByLabel("Format", { exact: true }).selectOption("audiobook");
  await expect.poll(() => format).toBe("audiobook");
  await openResult(page, "Unknown edition");
  await page.getByRole("button", { name: "Add Book", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Review before adding" })).toHaveCount(0);
  await expect.poll(() => created?.format).toBe("audiobook");
  expect(created?.preserveExisting).toBe(true);
});
