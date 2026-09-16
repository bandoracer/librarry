import { test, expect } from "@playwright/test";

const row = (id: string, title: string) => ({ id, title, sourcePath: `/fixture/${title}.epub`, status: "pending", mediaFormat: "ebook", reason: "Choose the correct book", metadata: {}, createdAt: "2026-09-16T00:00:00Z", updatedAt: "2026-09-16T00:00:00Z" });

test("import reviews traverse pages and bulk decisions use only the visible selection", async ({ page }, testInfo) => {
  let resolved = false;
  await page.route("**/api/v1/library/import-reviews?*", route => {
    const query = new URL(route.request().url()).searchParams;
    expect(query.get("view")).toBe("collection");
    expect(query.get("limit")).toBe("50");
    const second = query.get("cursor") === "older-page";
    return route.fulfill({ json: { reviews: second ? resolved ? [] : [row("older", "Older review")] : [row("newest", "Newest review")], total: 10001, filtered: 7000, counts: { pending: 7000, resolved: 3001 }, nextCursor: second ? undefined : "older-page", observedAt: "2026-09-16T00:00:00Z" } });
  });
  let ids: string[] = [];
  await page.route("**/api/v1/library/import-reviews/resolve-bulk", route => {
    const body = route.request().postDataJSON();
    expect(body.action).toBe("skip");
    ids = body.ids;
    resolved = true;
    return route.fulfill({ json: { requested: 1, resolved: 1, imported: 0, skipped: 1, rejected: 0, errored: 0, results: [{ id: "older", status: "skipped" }] } });
  });
  await page.goto("/imports#reviews");
  const top = page.getByRole("navigation", { name: "Top import review pages" });
  await expect(top).toContainText("7000 matching · 10001 total · Page 1");
  await page.getByRole("button", { name: "Select page", exact: true }).click();
  await expect(page.getByLabel("Select Newest review", { exact: true })).toBeChecked();
  await top.getByRole("button", { name: "Next reviews" }).click();
  await expect(page.getByLabel("Select Older review", { exact: true })).not.toBeChecked();
  await expect(page.getByRole("button", { name: "Resolve selected…" })).toBeDisabled();
  await expect(top).toContainText("Page 2");
  await expect(top.getByRole("button", { name: "Next reviews" })).toBeDisabled();
  await page.getByRole("button", { name: "Select page", exact: true }).click();
  await page.getByRole("button", { name: "Resolve selected…" }).click();
  await page.getByRole("button", { name: "Skip 1 review", exact: true }).click();
  await expect.poll(() => ids).toEqual(["older"]);
  await expect(page.getByText("No reviews on this page", { exact: true })).toBeVisible();
  await top.getByRole("button", { name: "Previous reviews" }).click();
  await expect(page.getByLabel("Select Newest review", { exact: true })).not.toBeChecked();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/import-review-pages-${testInfo.project.name}.png`, fullPage: true });
});

test("review filters reset paging and expose resolved payload history", async ({ page }) => {
  let last: URLSearchParams;
  await page.route("**/api/v1/library/import-reviews?*", route => {
    last = new URL(route.request().url()).searchParams;
    const payload = last.get("kind") === "payload";
    const resolved = last.get("status") === "resolved";
    return route.fulfill({ json: { reviews: payload ? [{ ...row("payload", "Payload review"), status: resolved ? "skipped" : "pending", metadata: { payloadReview: true, payload: { files: [] } } }] : [row("file", "File review")], total: 10001, filtered: payload ? 1 : 200, counts: { pending: 7000, resolved: 3001 }, nextCursor: last.get("cursor") || payload ? undefined : "next", observedAt: "2026-09-16T00:00:00Z" } });
  });
  await page.goto("/imports#reviews");
  const top = page.getByRole("navigation", { name: "Top import review pages" });
  await top.getByRole("button", { name: "Next reviews" }).click();
  await expect(top).toContainText("Page 2");
  await page.getByLabel("Import review type", { exact: true }).selectOption("payload");
  await expect(top).toContainText("Page 1");
  await expect(page.getByText("Review files: Payload review", { exact: true })).toBeVisible();
  await expect.poll(() => last.get("cursor")).toBeNull();
  await page.getByRole("tablist", { name: "Import review status filter" }).getByRole("tab", { name: "Resolved", exact: true }).click();
  await expect(page.getByRole("button", { name: "Reopen review", exact: true })).toBeVisible();
  await page.getByLabel("Import review format", { exact: true }).selectOption("ebook");
  await page.getByLabel("Search import reviews", { exact: true }).fill("old title");
  await expect.poll(() => last.get("q")).toBe("old title");
  expect(last!.get("status")).toBe("resolved");
  expect(last!.get("format")).toBe("ebook");
  expect(last!.get("kind")).toBe("payload");
  expect(last!.get("cursor")).toBeNull();
});

test("a failed review page shows an error and can return to the first page", async ({ page }) => {
  let fail = true;
  await page.route("**/api/v1/library/import-reviews?*", route => {
    if (new URL(route.request().url()).searchParams.has("cursor") && fail) return route.fulfill({ status: 503, json: { error: "Fixture review database unavailable" } });
    return route.fulfill({ json: { reviews: [row("file", "Available review")], total: 10001, filtered: 10001, counts: { pending: 10001, resolved: 0 }, nextCursor: "next", observedAt: "2026-09-16T00:00:00Z" } });
  });
  await page.goto("/imports#reviews");
  const top = page.getByRole("navigation", { name: "Top import review pages" });
  await top.getByRole("button", { name: "Next reviews" }).click();
  await expect(page.getByText("Review records are unavailable.", { exact: true })).toBeVisible();
  await expect(page.getByText("No reviews on this page", { exact: true })).toHaveCount(0);
  await expect(top).toContainText("— matching · — total");
  fail = false;
  await page.getByRole("button", { name: "Retry reviews", exact: true }).click();
  await expect(page.getByLabel("Select Available review", { exact: true })).toBeVisible();
  await top.getByRole("button", { name: "First review page" }).click();
  await expect(top).toContainText("Page 1");
});
