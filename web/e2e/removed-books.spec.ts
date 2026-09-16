import { test, expect } from "@playwright/test";

const saved = { id: "00000000-0000-0000-0000-000000000008", title: "Older removed book", authorName: "Owner author", format: "ebook", qualityProfile: "premium", status: "removed", monitored: false, tags: ["saved-tag"], updatedAt: "2026-09-16T00:00:00Z", createdAt: "2026-01-01T00:00:00Z" };

test("removed books paginate and restore only after reviewing current settings", async ({ page }, testInfo) => {
  let restored = false;
  let latest = { ...saved };
  let attempts = 0;
  await page.route("**/api/v1/library/removed-books?*", route => {
    const q = new URL(route.request().url()).searchParams;
    return route.fulfill({ json: { books: restored ? [] : [{ ...saved, title: q.has("cursor") ? saved.title : "Newest removed book" }], total: restored ? 10000 : 10001, filtered: restored ? 9999 : 10000, counts: { removed: restored ? 9999 : 10000, ignored: 1 }, nextCursor: q.has("cursor") ? undefined : "older", observedAt: saved.updatedAt } });
  });
  await page.route(`**/api/v1/wanted/${saved.id}`, route => route.fulfill({ json: latest }));
  await page.route(`**/api/v1/wanted/${saved.id}/restore`, route => {
    const body = route.request().postDataJSON(); attempts++;
    if (attempts === 1) {
      expect(body).toEqual({ updatedAt: saved.updatedAt, monitored: false });
      latest = { ...saved, title: "Owner changed title", qualityProfile: "standard", updatedAt: "2026-09-16T01:00:00Z" };
      return route.fulfill({ status: 409, json: { error: "Book changed; refresh before restoring" } });
    }
    expect(body).toEqual({ updatedAt: latest.updatedAt, monitored: true });
    restored = true;
    return route.fulfill({ json: { ...latest, status: "wanted", monitored: true } });
  });
  await page.goto("/library");
  await page.getByRole("link", { name: "Removed", exact: true }).click();
  const top = page.getByRole("navigation", { name: "Top removed book pages" });
  await expect(top).toContainText("10000 matching · 10001 inactive books");
  await top.getByRole("button", { name: "Next books" }).click();
  await expect(page.getByRole("link", { name: saved.title, exact: true })).toBeVisible();
  const trigger = page.getByRole("button", { name: "Restore…", exact: true });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "Restore book", exact: true });
  await expect(dialog.getByRole("checkbox", { name: "Enable monitoring after restore", exact: true })).not.toBeChecked();
  await expect(dialog).toContainText("premium");
  await expect(dialog).toContainText("saved-tag");
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  await trigger.click();
  await dialog.getByRole("button", { name: "Restore to Library", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("Book changed");
  await dialog.getByRole("button", { name: "Reload book", exact: true }).click();
  await expect(dialog).toContainText("Owner changed title");
  await expect(dialog).toContainText("standard");
  await dialog.getByRole("checkbox", { name: "Enable monitoring after restore", exact: true }).check();
  await dialog.screenshot({ path: `../output/playwright/restore-book-${testInfo.project.name}.png` });
  await dialog.getByRole("button", { name: "Restore to Library", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect.poll(() => attempts).toBe(2);
  await expect(top).toContainText("10000 inactive books");
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
});

test("removed book filters reset pages and errors stay distinct from empty results", async ({ page }, testInfo) => {
  let last = new URLSearchParams();
  let fail = false;
  await page.route("**/api/v1/library/removed-books?*", route => {
    last = new URL(route.request().url()).searchParams;
    if (fail) return route.fulfill({ status: 503, json: { error: "Fixture outage" } });
    return route.fulfill({ json: { books: [{ ...saved, status: last.get("status") === "ignored" ? "ignored" : "removed" }], total: 10001, filtered: last.get("q") ? 1 : 10001, counts: { removed: 10000, ignored: 1 }, nextCursor: last.has("cursor") ? undefined : "older" } });
  });
  await page.goto("/library/removed");
  const top = page.getByRole("navigation", { name: "Top removed book pages" });
  await top.getByRole("button", { name: "Next books" }).click();
  await expect(top).toContainText("Page 2");
  await page.getByLabel("Removed book status", { exact: true }).selectOption("ignored");
  await page.getByLabel("Removed book format", { exact: true }).selectOption("ebook");
  await page.getByLabel("Search removed books", { exact: true }).fill("Owner");
  await expect.poll(() => last.get("q")).toBe("Owner");
  expect(last.get("cursor")).toBeNull();
  expect(last.get("status")).toBe("ignored");
  await expect(top).toContainText("Page 1");
  await page.screenshot({ path: `../output/playwright/removed-books-${testInfo.project.name}.png`, fullPage: true });
  fail = true;
  await page.getByRole("button", { name: "Refresh removed books", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Removed books could not be loaded");
  await expect(page.getByText("No removed books on this page", { exact: true })).toHaveCount(0);
  await expect(top).toContainText("— matching · — inactive books");
  fail = false;
  await page.getByRole("button", { name: "Retry removed books", exact: true }).click();
  await expect(page.getByRole("link", { name: saved.title, exact: true })).toBeVisible();
});

test("removed book deep links offer explicit restore without implicit monitoring", async ({ page }) => {
  let book = { ...saved };
  await page.route(`**/api/v1/wanted/${saved.id}`, route => route.fulfill({ json: book }));
  await page.route(`**/api/v1/wanted/${saved.id}/restore`, route => {
    expect(route.request().postDataJSON()).toEqual({ updatedAt: saved.updatedAt, monitored: false });
    book = { ...book, status: "wanted" };
    return route.fulfill({ json: book });
  });
  await page.goto(`/library/book/${saved.id}`);
  await expect(page.getByRole("link", { name: "Removed books", exact: true })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: `${saved.title} monitored`, exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Restore…", exact: true }).click();
  await page.getByRole("button", { name: "Restore to Library", exact: true }).click();
  await expect(page.getByRole("link", { name: "Removed books", exact: true })).toHaveCount(0);
  await expect(page.getByRole("checkbox", { name: `${saved.title} monitored`, exact: true })).not.toBeChecked();
});

test("an uncertain restore is reconciled without sending another restore", async ({ page }) => {
  let attempts = 0;
  await page.route("**/api/v1/library/removed-books?*", route => route.fulfill({ json: { books: [saved], total: 1, filtered: 1, counts: { removed: 1, ignored: 0 } } }));
  await page.route(`**/api/v1/wanted/${saved.id}`, route => route.fulfill({ json: { ...saved, status: "wanted", updatedAt: "2026-09-16T01:00:00Z" } }));
  await page.route(`**/api/v1/wanted/${saved.id}/restore`, route => {
    attempts++;
    return route.fulfill({ status: 503, json: { error: "Response interrupted; refresh to check the current state" } });
  });
  await page.goto("/library/removed");
  await page.getByRole("button", { name: "Restore…", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Restore book", exact: true });
  await dialog.getByRole("button", { name: "Restore to Library", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("Response interrupted");
  await dialog.getByRole("button", { name: "Reload book", exact: true }).click();
  await expect(dialog.getByRole("button", { name: "Restore to Library", exact: true })).toBeDisabled();
  await expect(dialog.getByRole("link", { name: "Open book", exact: true })).toHaveAttribute("href", `/library/book/${saved.id}`);
  expect(attempts).toBe(1);
});
