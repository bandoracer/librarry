import { expect, test } from "@playwright/test";

test("book status explains partial audiobook loss and client outages", async ({ page }, testInfo) => {
 const id = "00000000-0000-0000-0000-000000000023";
 let state = "incomplete";
 await page.route(`**/api/v1/wanted/${id}`, route => route.fulfill({ json: {
  id, title: "Evidence fixture", authorName: "Fixture Author", format: "audiobook", status: "imported", monitored: true, qualityProfile: "standard", derivedState: state,
  stateEvidence: { files: { state: state === "unknown" ? "missing" : state, reason: "Required audiobook media is missing.", presentFiles: state === "unknown" ? 0 : 1, requiredFiles: 2 }, downloads: state === "unknown" ? "unavailable" : "fresh", quality: "available", message: state === "unknown" ? "Download-client evidence is unavailable or incomplete." : "Required audiobook media is missing." }
 } }));
 await page.goto(`/library/book/${id}`);
 await expect(page.getByText("Incomplete", { exact: true })).toBeVisible();
 await expect(page.getByRole("status").filter({ hasText: "Required audiobook media is missing." })).toBeVisible();
 await expect(page.getByText("Downloaded", { exact: true })).toHaveCount(0);
 state = "unknown";
 await page.reload();
 await expect(page.getByText("Unknown", { exact: true })).toBeVisible();
 await expect(page.getByRole("status").filter({ hasText: "Download-client evidence is unavailable or incomplete." })).toBeVisible();
 await expect(page.getByText("Missing", { exact: true })).toHaveCount(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.screenshot({ path: `../output/playwright/book-evidence-${testInfo.project.name}.png`, fullPage: true });
});

test("Wanted exposes incomplete and unknown books and explains skipped batches", async ({ page }, testInfo) => {
 const books = ["missing", "incomplete", "unknown"].map((state, index) => ({ id: `evidence-${index}`, title: `${state} fixture book`, authorName: "Fixture Author", format: "ebook", status: "imported", monitored: true, qualityProfile: "standard", derivedState: state }));
 await page.route("**/api/v1/wanted?**", route => route.fulfill({ json: { wanted: new URL(route.request().url()).searchParams.get("view") === "cutoff-unmet" ? [] : books } }));
 await page.route("**/api/v1/wanted/monitor", async route => {
  expect(route.request().postDataJSON()).toMatchObject({ force: true, autoGrab: false });
  await route.fulfill({ json: { wantedChecked: 1, grabbedCount: 0, errorCount: 0, items: [{ wantedItem: books[2], skippedReason: "download-client evidence is unavailable or incomplete" }] } });
 });
 await page.goto("/wanted/unknown");
 await expect(page.getByRole("link", { name: "unknown fixture book", exact: true })).toBeVisible();
 await expect(page.getByRole("link", { name: "missing fixture book", exact: true })).toHaveCount(0);
 await page.getByRole("button", { name: "Check Next Batch", exact: true }).click();
 await expect(page.getByText(/1 skipped \(download-client evidence is unavailable or incomplete\)/)).toBeVisible();
 await page.getByRole("link", { name: "Incomplete", exact: true }).click();
 await expect(page.getByRole("link", { name: "incomplete fixture book", exact: true })).toBeVisible();
 await expect(page.getByRole("link", { name: "unknown fixture book", exact: true })).toHaveCount(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.screenshot({ path: `../output/playwright/wanted-evidence-${testInfo.project.name}.png`, fullPage: true });
});

test("Wanted upgrade selection sends every selected row beyond the default batch", async ({ page }, testInfo) => {
 const books = Array.from({ length: 75 }, (_, index) => ({ id: `00000000-0000-0000-0000-${String(index + 1).padStart(12, "0")}`, title: `Upgrade fixture ${String(index + 1).padStart(3, "0")}`, authorName: "Fixture Author", format: "ebook", status: "imported", monitored: true, qualityProfile: "standard", derivedState: "cutoffUnmet" }));
 await page.route("**/api/v1/wanted?**", route => route.fulfill({ json: { wanted: books } }));
 let submitted = 0;
 await page.route("**/api/v1/wanted/upgrades", async route => {
  const request = route.request().postDataJSON();
  expect(request).toMatchObject({ limit: 75, force: true, autoGrab: false });
  expect([...request.wantedIds].sort()).toEqual(books.map(book => book.id).sort());
  submitted++;
  await route.fulfill({ json: { wantedChecked: 75, upgradeCount: 0, grabbedCount: 0, errorCount: 0, items: books.map(wantedItem => ({ wantedItem })) } });
 });
 await page.goto("/wanted/cutoff-unmet");
 await page.getByRole("checkbox", { name: "Select all rows", exact: true }).check();
 await page.getByRole("button", { name: "Upgrade Search Selected", exact: true }).click();
 await expect(page.getByText("Upgrade search: 75 checked, 0 upgrades, 0 grabbed, 0 errors", { exact: true })).toBeVisible();
 expect(submitted).toBe(1);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.screenshot({ path: `../output/playwright/upgrade-selection-${testInfo.project.name}.png`, fullPage: true });
});
