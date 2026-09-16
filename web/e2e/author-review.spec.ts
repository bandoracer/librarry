import { expect, test } from "@playwright/test";

const candidate = (id: string, status = "pending") => ({ id, revision: "a".repeat(64), title: `Candidate ${id}`, authorName: "Fixture author", provider: "Hardcover", format: "ebook", qualityProfile: "premium", reason: "Publication evidence needs review", status, wantedId: status === "wanted" ? "saved-book" : undefined, result: { work: { title: `Candidate ${id}`, authors: [] }, edition: {} } });

test("Author review traverses older candidates, sends revisions and shows retained settings", async ({ page }, testInfo) => {
 let resolved = false;
 await page.route("**/api/v1/authors/metadata/review?**", route => {
  const params = new URL(route.request().url()).searchParams;
  expect(params.get("limit")).toBe("6");
  const filtered = Boolean(params.get("q")) || params.get("status") !== "pending" || params.get("format") !== "all";
  if (filtered) expect(params.has("cursor")).toBe(false);
  const older = params.get("cursor") === "older";
  return route.fulfill({ json: { reviews: [candidate(older || filtered ? "older" : "first", resolved || params.get("status") === "wanted" ? "wanted" : "pending")], total: 10001, filtered: filtered ? 1 : 10001, counts: { pending: 10000 }, nextCursor: older || filtered ? undefined : "older" } });
 });
 await page.route("**/api/v1/authors/metadata/review/older/resolve", route => {
  expect(route.request().postDataJSON()).toEqual({ action: "wanted", revision: "a".repeat(64) });
  resolved = true;
  return route.fulfill({ json: { review: candidate("older", "wanted"), wantedItem: { id: "saved-book", title: "Owner title" }, alreadyTracked: true } });
 });
 await page.goto("/library/authors");
 await expect(page.getByText("1 shown · 10001 matching · 10000 pending", { exact: true })).toBeVisible();
 await page.getByRole("button", { name: "Next reviews", exact: true }).click();
 await expect(page.getByText("Candidate older", { exact: true })).toBeVisible();
 await page.getByRole("button", { name: "Mark wanted", exact: true }).click();
 await expect(page.getByText("Already tracked; existing book settings retained", { exact: true })).toBeVisible();
 await expect(page.getByRole("link", { name: "Open tracked book" })).toHaveAttribute("href", "/library/book/saved-book");
 await expect(page.getByRole("button", { name: "Ignore", exact: true })).toHaveCount(0);
 await page.getByRole("textbox", { name: "Search author reviews" }).fill("older");
 await expect(page.getByText("1 shown · 1 matching · 10000 pending", { exact: true })).toBeVisible();
 await expect(page.getByRole("button", { name: "Previous reviews", exact: true })).toBeDisabled();
 await page.getByRole("combobox", { name: "Author review status" }).selectOption("wanted");
 await page.getByRole("combobox", { name: "Author review format" }).selectOption("ebook");
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.getByText("Author review queue", { exact: true }).scrollIntoViewIfNeeded();
 await page.screenshot({ path: `../output/playwright/author-review-${testInfo.project.name}.png`, fullPage: true });
});

test("Author review failure offers retry and stale decision remains actionable", async ({ page }) => {
 let failed = true;
 await page.route("**/api/v1/authors/metadata/review?**", route => failed ? route.fulfill({ status: 503, json: { error: "Controlled author review outage" } }) : route.fulfill({ json: { reviews: [candidate("stale")], total: 1, filtered: 1, counts: { pending: 1 } } }));
 await page.route("**/api/v1/authors/metadata/review/stale/resolve", route => route.fulfill({ status: 409, json: { error: "Review changed; refresh before deciding" } }));
 await page.goto("/library/authors");
 await expect(page.getByText("Author reviews could not be loaded", { exact: true })).toBeVisible();
 await expect(page.getByText("No matching author reviews", { exact: true })).toHaveCount(0);
 failed = false;
 await page.getByRole("button", { name: "Retry reviews" }).click();
 await page.getByRole("button", { name: "Ignore", exact: true }).click();
 await expect(page.getByText(/Review changed; refresh before deciding/)).toBeVisible();
 await expect(page.getByRole("button", { name: "Ignore", exact: true })).toBeEnabled();
});
