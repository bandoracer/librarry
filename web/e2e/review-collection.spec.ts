import { expect, test } from "@playwright/test";

const review = (id: string, title: string) => ({ revision: "a".repeat(64), wantedItem: { id, title, authorName: "Review Author", format: "ebook", status: "imported", monitored: true, qualityProfile: "standard" }, fields: [{ fieldName: "title", label: "Title", canonicalValue: title, conflict: true, candidates: [] }], conflictCount: 1, protectedCount: 0, recordCount: 2, candidateCount: 2 });

test("Review pages imported conflicts, filters globally and confirms only the selected page", async ({ page }, testInfo) => {
 const first = review("00000000-0000-0000-0000-000000000001", "First review");
 const older = review("00000000-0000-0000-0000-000000000501", "Older imported review");
 await page.route("**/api/v1/wanted/metadata/review?**", route => {
  const q = new URL(route.request().url()).searchParams;
  expect(q.get("limit")).toBe("100");
  const searched = q.get("q") === "Older";
  if (searched) expect(q.has("cursor")).toBe(false);
  const second = q.get("cursor") === "second-review";
  return route.fulfill({ json: { items: [second || searched ? older : first], total: 501, filtered: searched ? 1 : 501, conflictCount: 501, nextCursor: second || searched ? undefined : "second-review" } });
 });
 let selected: string[] = [];
 let failing = true;
 await page.route("**/api/v1/wanted/metadata/review/confirm-canonical", route => {
  selected = route.request().postDataJSON().wantedIds;
  expect(route.request().postDataJSON().revisions).toEqual({ [older.wantedItem.id]: "a".repeat(64) });
  return failing ? route.fulfill({ status: 409, json: { error: "Metadata changed during confirmation; refresh and review again" } }) : route.fulfill({ json: { status: "ok", itemsReviewed: selected.length, fieldsConfirmed: selected.length, skippedItems: 0, items: [] } });
 });
 await page.goto("/wanted/review");
 await expect(page.getByRole("link", { name: "Review", exact: true })).toBeInViewport();
 await expect(page.getByText("501 unresolved metadata conflicts in the library", { exact: true })).toBeVisible();
 await page.getByRole("button", { name: "Select shown", exact: true }).click();
 await page.getByRole("button", { name: "Next", exact: true }).click();
 await expect(page.getByRole("link", { name: older.wantedItem.title, exact: true })).toBeVisible();
 await expect(page.getByRole("button", { name: "Keep current", exact: true })).toBeDisabled();
 await page.getByRole("button", { name: "Select shown", exact: true }).click();
 await page.getByRole("button", { name: "Keep current", exact: true }).click();
 await expect(page.getByText("Metadata changed during confirmation; refresh and review again", { exact: false })).toBeVisible();
 expect(selected).toEqual([older.wantedItem.id]);
 await expect(page.getByRole("checkbox", { name: `Select ${older.wantedItem.title}`, exact: true })).toBeChecked();
 failing = false;
 await page.getByRole("button", { name: "Keep current", exact: true }).click();
 await expect(page.getByRole("button", { name: "Keep current", exact: true })).toBeDisabled();
 await page.getByRole("textbox", { name: "Filter metadata reviews" }).fill("Older");
 await expect(page.getByText("1 shown · 1 matching · 0 selected on this page", { exact: true })).toBeVisible();
 await expect(page.getByRole("button", { name: "Previous", exact: true })).toBeDisabled();
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.evaluate(() => window.scrollTo(0, 0));
 await page.screenshot({ path: `../output/playwright/review-collection-${testInfo.project.name}.png`, fullPage: true });
});

test("Review distinguishes unavailable data from an empty queue and retries", async ({ page }) => {
 let failed = true;
 await page.route("**/api/v1/wanted/metadata/review?**", route => failed ? route.fulfill({ status: 503, json: { error: "Controlled review outage" } }) : route.fulfill({ json: { items: [], total: 0, filtered: 0, conflictCount: 0 } }));
 await page.goto("/wanted/review");
 await expect(page.getByText("Books could not be loaded", { exact: true })).toBeVisible();
 await expect(page.getByText("No metadata reviews pending", { exact: true })).toHaveCount(0);
 failed = false;
 await page.getByRole("button", { name: "Retry", exact: true }).click();
 await expect(page.getByText("No metadata reviews pending", { exact: true })).toBeVisible();
});
