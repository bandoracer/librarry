import { expect, test } from "@playwright/test";

const book = (id: string, title: string) => ({ id, title, authorName: "Fixture Author", format: "ebook", status: "imported", monitored: true, qualityProfile: "standard", derivedState: "downloaded" });

test("Library pages older books, filters globally and keeps edits on the chosen page", async ({ page }, testInfo) => {
 const first = book("00000000-0000-0000-0000-000000000001", "First page fixture");
 const older = book("00000000-0000-0000-0000-000000000501", "Older page fixture");
 let legacyReads = 0;
 await page.route("**/api/v1/wanted?**", route => { legacyReads++; return route.fulfill({ json: { wanted: [] } }); });
 await page.route("**/api/v1/library/books?**", route => {
  const q = new URL(route.request().url()).searchParams;
  expect(q.get("limit")).toBe("100");
  const filtered = q.get("q") === "Older";
  if (filtered) expect(q.has("cursor")).toBe(false);
  const second = q.get("cursor") === "second-page";
  return route.fulfill({ json: { books: [filtered || second ? older : first], total: 501, filtered: filtered ? 1 : 501, counts: { downloaded: 501 }, recordedFiles: 501, downloads: "fresh", nextCursor: filtered || second ? undefined : "second-page" } });
 });
 let edited: string[] = [];
 await page.route("**/api/v1/wanted/bulk", async route => {
  const body = route.request().postDataJSON();
  edited = body.ids;
  await route.fulfill({ json: { results: body.ids.map((id: string) => ({ id, status: "updated" })) } });
 });
 await page.goto("/library");
 await expect(page.getByText("1 shown · 501 matching · 501 total", { exact: true })).toBeVisible();
 await page.getByRole("button", { name: "Edit Mode", exact: true }).click();
 await page.getByRole("button", { name: "Select shown", exact: true }).click();
 await expect(page.getByText("1 selected on this page · 1 shown", { exact: true })).toBeVisible();
 await page.getByRole("button", { name: "Next", exact: true }).click();
 await expect(page.getByRole("link", { name: "Older page fixture", exact: true })).toBeVisible();
 await expect(page.getByText("0 selected on this page · 1 shown", { exact: true })).toBeVisible();
 await page.getByRole("button", { name: "Select shown", exact: true }).click();
 await page.getByRole("button", { name: "Unmonitor", exact: true }).click();
 await expect.poll(() => edited).toEqual([older.id]);
 await page.getByRole("button", { name: "Previous", exact: true }).click();
 await expect(page.getByRole("link", { name: "First page fixture", exact: true })).toBeVisible();
 await page.getByRole("textbox", { name: "Filter authors or books" }).fill("Older");
 await expect(page.getByText("1 shown · 1 matching · 501 total", { exact: true })).toBeVisible();
 await expect(page.getByRole("link", { name: "Older page fixture", exact: true })).toBeVisible();
 await expect(page.getByRole("button", { name: "Next", exact: true })).toBeDisabled();
 expect(legacyReads).toBe(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.evaluate(() => window.scrollTo(0, 0));
 await page.screenshot({ path: `../output/playwright/book-collection-${testInfo.project.name}.png`, fullPage: true });
});

test("Library distinguishes collection outage from empty data and can retry", async ({ page }) => {
 let failed = true;
 await page.route("**/api/v1/library/books?**", route => failed
  ? route.fulfill({ status: 503, json: { error: "Controlled collection outage" } })
  : route.fulfill({ json: { books: [], total: 0, filtered: 0, counts: {}, recordedFiles: 0, downloads: "notConfigured" } }));
 await page.goto("/library");
 await expect(page.getByText("Books could not be loaded", { exact: true })).toBeVisible();
 await expect(page.getByText("Your library is empty", { exact: true })).toHaveCount(0);
 failed = false;
 await page.getByRole("button", { name: "Retry", exact: true }).click();
 await expect(page.getByText("Your library is empty", { exact: true })).toBeVisible();
});

test("Wanted pages gaps globally and clears selections between pages and tabs", async ({ page }, testInfo) => {
 const first = { ...book("00000000-0000-0000-0000-000000000001", "First missing fixture"), derivedState: "missing" };
 const older = { ...book("00000000-0000-0000-0000-000000000501", "Older missing fixture"), derivedState: "missing" };
 const cutoff = { ...book("00000000-0000-0000-0000-000000000502", "Cutoff fixture"), derivedState: "cutoffUnmet" };
 await page.route("**/api/v1/library/books?**", route => {
  const q = new URL(route.request().url()).searchParams;
  const isCutoff = q.get("state") === "cutoffUnmet";
  const second = q.get("cursor") === "older-gap";
  if (isCutoff) expect(q.has("cursor")).toBe(false);
  return route.fulfill({ json: { books: [isCutoff ? cutoff : second ? older : first], total: 502, filtered: isCutoff ? 1 : 501, counts: { missing: 501, cutoffUnmet: 1 }, recordedFiles: 1, downloads: "fresh", nextCursor: !isCutoff && !second ? "older-gap" : undefined } });
 });
 let edited: string[] = [];
 await page.route("**/api/v1/wanted/bulk", async route => {
  edited = route.request().postDataJSON().ids;
  await route.fulfill({ json: { results: edited.map(id => ({ id, status: "updated" })) } });
 });
 await page.goto("/wanted");
 await page.getByRole("checkbox", { name: "Select all rows", exact: true }).check();
 await page.getByRole("button", { name: "Next", exact: true }).click();
 await expect(page.getByRole("link", { name: older.title, exact: true })).toBeVisible();
 await expect(page.getByRole("checkbox", { name: `Select ${older.title}`, exact: true })).not.toBeChecked();
 await page.getByRole("checkbox", { name: "Select all rows", exact: true }).check();
 await page.getByRole("button", { name: "Unmonitor Selected", exact: true }).click();
 await expect.poll(() => edited).toEqual([older.id]);
 await page.getByRole("link", { name: "Cutoff Unmet", exact: true }).click();
 await expect(page.getByRole("link", { name: cutoff.title, exact: true })).toBeVisible();
 await expect(page.getByRole("button", { name: "Upgrade Search Selected", exact: true })).toBeDisabled();
 await expect(page.getByRole("button", { name: "Previous", exact: true })).toBeDisabled();
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.evaluate(() => window.scrollTo(0, 0));
 await page.screenshot({ path: `../output/playwright/wanted-collection-${testInfo.project.name}.png`, fullPage: true });
});
