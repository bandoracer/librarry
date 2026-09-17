import { test, expect } from "@playwright/test";

const newest = { id: "00000000-0000-0000-0000-000000000001", title: "Newest book", authorName: "Author", format: "ebook" };
const older = { id: "00000000-0000-0000-0000-000000000002", title: "Older book beyond 200", authorName: "Owner edited author", format: "ebook" };

test("manual import finds older books and preserves its selection while browsing", async ({ page }, testInfo) => {
  let latest = new URLSearchParams();
  await page.route("**/api/v1/library/book-choices?*", route => {
    latest = new URL(route.request().url()).searchParams;
    const search = latest.get("q") ?? "";
    const second = latest.has("cursor");
    const selected = [newest, older].find(book => book.id === latest.get("selectedId") && book.format === latest.get("format"));
    return route.fulfill({ json: { books: latest.get("format") === "audiobook" || search === "absent" ? [] : second || search ? [older] : [newest], selected, total: 10001, filtered: search === "absent" ? 0 : search ? 1 : 10001, nextCursor: second || search ? undefined : "older-page", observedAt: "2026-09-16T00:00:00Z" } });
  });
  let importID = "";
  await page.route("**/api/v1/library/import", route => {
    importID = route.request().postDataJSON().wantedId;
    return route.fulfill({ json: { imported: true, destinationPath: "/library/fixture.epub", file: { title: older.title } } });
  });
  await page.goto("/imports");
  const select = page.getByLabel("Book for manual import", { exact: true });
  const picker = page.locator(".imports-book-choice").filter({ has: select });
  await picker.getByText("Search and browse books", { exact: true }).click();
  const nav = page.getByRole("navigation", { name: "Book pages: Book for manual import", exact: true });
  await expect(nav).toContainText("10001 matching · Page 1");
  await nav.getByRole("button", { name: "Next books" }).click();
  await select.selectOption(older.id);
  await nav.getByRole("button", { name: "Previous books" }).click();
  await expect(select).toHaveValue(older.id);
  await expect(select.locator("option:checked")).toContainText("Owner edited author");
  await page.getByLabel("Search books: Book for manual import", { exact: true }).fill("absent");
  await expect.poll(() => latest.get("q")).toBe("absent");
  await expect(select).toHaveValue(older.id);
  await page.getByLabel("Manual import source path", { exact: true }).fill("/fixture/older.epub");
  await page.locator(".imports-block").filter({ has: select }).screenshot({ path: `../output/playwright/import-book-choices-${testInfo.project.name}.png` });
  await page.getByRole("button", { name: "Import", exact: true }).click();
  await expect.poll(() => importID).toBe(older.id);
  await page.getByLabel("Manual import format", { exact: true }).selectOption("audiobook");
  await expect(select).toHaveValue("");
  await expect.poll(() => latest.get("format")).toBe("audiobook");
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
});

test("payload file assignments reach older books and changes invalidate a preview", async ({ page }) => {
  const reviewID = "00000000-0000-0000-0000-000000000003";
  await page.route("**/api/v1/library/book-choices?*", route => {
    const query = new URL(route.request().url()).searchParams;
    return route.fulfill({ json: { books: query.has("cursor") ? [older] : [newest], selected: [newest, older].find(book => book.id === query.get("selectedId")), total: 10001, filtered: 10001, nextCursor: query.has("cursor") ? undefined : "older" } });
  });
  await page.route("**/api/v1/library/import-reviews?*", route => route.fulfill({ json: { reviews: [{ id: reviewID, status: "pending", title: "Book pack", wantedId: newest.id, metadata: { payloadReview: true, payload: { files: [{ relativePath: "Book.epub", format: "ebook", progress: 1, sizeBytes: 1024 }] } } }], total: 1, filtered: 1, counts: { pending: 1, resolved: 0 } } }));
  let previews = 0;
  await page.route(`**/api/v1/library/import-reviews/${reviewID}/preview`, route => {
    expect(route.request().postDataJSON().mapping).toEqual([{ relativePath: "Book.epub", wantedId: older.id, exclude: false }]);
    previews++;
    return route.fulfill({ json: { fingerprint: "older-book-preview", operation: { files: [] } } });
  });
  await page.goto("/imports#reviews");
  const select = page.getByLabel("Book for Book.epub", { exact: true });
  const picker = page.locator(".imports-book-choice").filter({ has: select });
  await picker.getByText("Search and browse books", { exact: true }).click();
  await page.getByRole("navigation", { name: "Book pages: Book for Book.epub", exact: true }).getByRole("button", { name: "Next books" }).click();
  await select.selectOption(older.id);
  const confirm = page.getByRole("checkbox", { name: "I checked these book assignments", exact: false });
  await confirm.check();
  await page.getByRole("button", { name: "Preview destinations", exact: true }).click();
  await expect.poll(() => previews).toBe(1);
  await expect(page.getByRole("button", { name: "Import this file set", exact: true })).toBeVisible();
  await select.selectOption("__retain");
  await expect(confirm).not.toBeChecked();
  await expect(page.getByRole("button", { name: "Import this file set", exact: true })).toHaveCount(0);
});

test("choice failures can be retried without clearing the intended book", async ({ page }) => {
  let fail = false;
  await page.route("**/api/v1/library/book-choices?*", route => {
    const query = new URL(route.request().url()).searchParams;
    if (fail) return route.fulfill({ status: 503, json: { error: "Database unavailable" } });
    return route.fulfill({ json: { books: [older], selected: query.get("selectedId") ? older : undefined, total: 1, filtered: 1 } });
  });
  await page.goto("/imports");
  const select = page.getByLabel("Book for manual import", { exact: true });
  await select.selectOption(older.id);
  await expect(select).toBeEnabled();
  const picker = page.locator(".imports-book-choice").filter({ has: select });
  await picker.getByText("Search and browse books", { exact: true }).click();
  fail = true;
  await page.getByLabel("Search books: Book for manual import", { exact: true }).fill("other");
  await expect(picker.getByRole("alert")).toContainText("Book choices could not be loaded");
  await expect(select).toHaveValue(older.id);
  await expect(select).toBeDisabled();
  fail = false;
  await picker.getByRole("button", { name: "Retry books", exact: true }).click();
  await expect(select).toBeEnabled();
  await expect(select).toHaveValue(older.id);
});

test("a removed selection is explicit and never replaced by the first choice", async ({ page }) => {
  await page.route("**/api/v1/library/book-choices?*", route => {
    const query = new URL(route.request().url()).searchParams;
    const removed = query.get("q") === "removed";
    return route.fulfill({ json: { books: removed ? [newest] : [older], selected: removed ? undefined : query.get("selectedId") ? older : undefined, total: 1, filtered: 1 } });
  });
  await page.goto("/imports");
  const select = page.getByLabel("Book for manual import", { exact: true });
  await select.selectOption(older.id);
  const picker = page.locator(".imports-book-choice").filter({ has: select });
  await picker.getByText("Search and browse books", { exact: true }).click();
  await page.getByLabel("Search books: Book for manual import", { exact: true }).fill("removed");
  await expect(picker.getByText("This book is no longer available in this format. Choose another book.", { exact: true })).toBeVisible();
  await expect(select).toHaveValue(older.id);
  await expect(select.locator("option:checked")).toContainText("Selected book unavailable");
  await select.selectOption(newest.id);
  await expect(select).toHaveValue(newest.id);
});
