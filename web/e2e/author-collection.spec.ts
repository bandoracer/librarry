import { expect, test } from "@playwright/test";

const author = (id: string, name: string, linked = true) => ({ id, authorName: name, provider: "Hardcover", providerKey: id, format: "ebook", status: "monitored", qualityProfile: "standard", monitorNewItems: true, missingBookPolicy: "all", counts: linked ? { missing: 10001 } : {}, totalBooks: linked ? 10001 : 0, identityLinked: linked });

test("Authors pages subscriptions and uses identity counts without legacy book scans", async ({ page }, testInfo) => {
 let legacyReads = 0;
 for (const path of ["wanted?", "authors?", "library/files?"]) await page.route(`**/api/v1/${path}**`, route => { legacyReads++; return route.fulfill({ json: {} }); });
 await page.route("**/api/v1/library/authors?**", route => {
  const q = new URL(route.request().url()).searchParams;
  expect(q.get("limit")).toBe("100");
  const searched = q.get("q") === "Older";
  if (searched || q.get("status") === "unmonitored") expect(q.has("cursor")).toBe(false);
  const second = q.get("cursor") === "older";
  return route.fulfill({ json: { authors: [{ ...author(second || searched ? "older-author" : "first-author", second || searched ? "Older Author" : "First Author"), status: q.get("status") }, { ...author("unlinked", "Unlinked Author", false), status: q.get("status") }], total: 1002, filtered: searched ? 2 : 1002, nextCursor: second || searched ? undefined : "older", downloads: "partial" } });
 });
 let refreshed: string[] = [];
 await page.route("**/api/v1/authors/monitor", route => {
  refreshed = route.request().postDataJSON().authorIds;
  return route.fulfill({ json: { status: "completed", authorsChecked: 1, itemsFound: 0, wantedCreated: 0, errorCount: 0, items: [] } });
 });
 await page.goto("/library/authors");
 await expect(page.getByText("2 shown · 1002 matching · 1002 total subscriptions", { exact: true })).toBeVisible();
 await expect(page.getByText("10001 tracked · 10001 missing", { exact: true })).toBeVisible();
 await expect(page.getByText("Provider identity is not linked. Book counts are unavailable.", { exact: true })).toBeVisible();
 await expect(page.getByText("Download-client evidence is incomplete. Some book counts are unknown.", { exact: true })).toBeVisible();
 await page.getByRole("button", { name: "Next", exact: true }).click();
 await expect(page.getByRole("link", { name: "Older Author", exact: true })).toHaveAttribute("href", /older-author/);
 await page.getByRole("button", { name: "Refresh Older Author", exact: true }).click();
 await expect.poll(() => refreshed).toEqual(["older-author"]);
 await page.getByRole("textbox", { name: "Filter author subscriptions" }).fill("Older");
 await expect(page.getByText("2 shown · 2 matching · 1002 total subscriptions", { exact: true })).toBeVisible();
 await expect(page.getByRole("button", { name: "Previous", exact: true })).toBeDisabled();
 await expect(page.getByRole("button", { name: "Next", exact: true })).toBeDisabled();
 await page.getByRole("combobox", { name: "Author subscription status" }).selectOption("unmonitored");
 await expect(page.getByText("Unmonitored", { exact: true }).filter({ visible: true })).toHaveCount(2);
 expect(legacyReads).toBe(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.evaluate(() => window.scrollTo(0, 0));
 await page.screenshot({ path: `../output/playwright/author-collection-${testInfo.project.name}.png`, fullPage: true });
});

test("Authors distinguishes a collection failure from an empty collection and retries", async ({ page }) => {
 let failed = true;
 await page.route("**/api/v1/library/authors?**", route => failed ? route.fulfill({ status: 503, json: { error: "Controlled author outage" } }) : route.fulfill({ json: { authors: [], total: 0, filtered: 0, downloads: "notConfigured" } }));
 await page.goto("/library/authors");
 await expect(page.getByText("Author subscriptions could not be loaded", { exact: true })).toBeVisible();
 await expect(page.getByText("No author subscriptions", { exact: true })).toHaveCount(0);
 failed = false;
 await page.getByRole("button", { name: "Retry", exact: true }).click();
 await expect(page.getByText("No author subscriptions", { exact: true })).toBeVisible();
});
