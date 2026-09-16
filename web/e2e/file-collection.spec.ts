import { expect, test } from "@playwright/test";

const bookID = "00000000-0000-0000-0000-000000000007";
const file = (id: string) => ({ id, path: `/books/Very long audiobook directory/Disc 12/${id}.mp3`, title: `Chapter ${id}`, authorName: "Recorded writer", mediaFormat: "audiobook", importStatus: "imported", presenceState: "unknown", sizeBytes: 1000, wantedIds: [bookID], metadata: { wantedId: "stale-json-book" } });

test("Book files page all chapters using recorded book links and preserve native completeness", async ({ page }, testInfo) => {
 let legacyReads = 0;
 await page.route("**/api/v1/library/files?**", route => { legacyReads++; return route.fulfill({ json: { files: [] } }); });
 await page.route(`**/api/v1/wanted/${bookID}`, route => route.fulfill({ json: { id: bookID, title: "Large audiobook", authorName: "Recorded writer", format: "audiobook", qualityProfile: "standard", status: "imported", monitored: true, derivedState: "downloaded", stateEvidence: { files: { state: "present", presentFiles: 1501, requiredFiles: 1501, reason: "All 1501 required chapters are recorded present" }, downloads: "notConfigured", quality: "available" } } }));
 await page.route("**/api/v1/library/files/collection?**", route => {
  const q = new URL(route.request().url()).searchParams;
  expect(q.get("wantedId")).toBe(bookID); expect(q.get("limit")).toBe("100");
  const filtered = q.get("q") || q.get("presence") !== "all" || q.get("sort") !== "path";
  if (filtered) expect(q.has("cursor")).toBe(false);
  const older = q.get("cursor") === "older" || filtered;
  return route.fulfill({ json: { files: [file(older ? "1501" : "0001")], total: 1501, filtered: filtered ? 1 : 1501, counts: { audiobook: 1501 }, nextCursor: older ? undefined : "older" } });
 });
 await page.goto(`/library/book/${bookID}`);
 await expect(page.getByText("1 shown · 1501 matching · 1501 total files", { exact: true })).toBeVisible();
 await expect(page.getByText(/All 1501 required chapters are recorded present/)).toBeVisible();
 await page.getByRole("button", { name: "Next files", exact: true }).click();
 await expect(page.getByText("Chapter 1501", { exact: true })).toBeVisible();
 await expect(page.getByText("1501.mp3", { exact: true })).toBeVisible();
 await expect(page.getByRole("link", { name: "Open book", exact: true })).toHaveAttribute("href", `/library/book/${bookID}`);
 await page.getByRole("textbox", { name: "Search tracked files" }).fill("1501");
 await expect(page.getByText("1 shown · 1 matching · 1501 total files", { exact: true })).toBeVisible();
 await expect(page.getByRole("button", { name: "Previous files", exact: true })).toBeDisabled();
 await page.getByRole("combobox", { name: "Recorded file presence" }).selectOption("unknown");
 await page.getByRole("combobox", { name: "File sort", exact: true }).selectOption("updated");
 expect(legacyReads).toBe(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.getByText("Tracked files", { exact: true }).scrollIntoViewIfNeeded();
 await page.screenshot({ path: `../output/playwright/file-collection-${testInfo.project.name}.png`, fullPage: true });
});

test("Imports uses global counts and distinguishes file outages from empty lists", async ({ page }) => {
 let failed = true;
 await page.route("**/api/v1/library/files/collection?**", route => failed ? route.fulfill({ status: 503, json: { error: "Controlled file outage" } }) : route.fulfill({ json: { files: [file("1501")], total: 10001, filtered: 10001, counts: { imported: 9500, ebook: 8500, audiobook: 1501 }, nextCursor: "older" } }));
 await page.goto("/imports");
 await expect(page.getByText("Files could not be loaded", { exact: true })).toBeVisible();
 await expect(page.getByText("No matching files", { exact: true })).toHaveCount(0);
 failed = false;
 await page.getByRole("button", { name: "Retry files", exact: true }).click();
 await expect(page.getByText("1 shown · 10001 matching · 10001 total files", { exact: true })).toBeVisible();
 for (const value of ["10001", "9500", "8500", "1501"]) await expect(page.locator(".imports-statbar-wrap").getByText(value, { exact: true })).toBeVisible();
 failed = true;
 await page.getByRole("button", { name: "Refresh files", exact: true }).click();
 await expect(page.getByText("Files could not be loaded", { exact: true })).toBeVisible();
 await expect(page.locator(".imports-statbar-wrap").getByText("10001", { exact: true })).toHaveCount(0);
 await expect(page.getByText("No matching files", { exact: true })).toHaveCount(0);
});

test("Rename preview reaches older files and applies only the selected page", async ({ page }) => {
 const previews: string[][] = []; const applied: string[][] = [];
 await page.route("**/api/v1/library/files/collection?**", route => {
  const q = new URL(route.request().url()).searchParams;
  expect(q.get("limit")).toBe("100");
  const next = q.get("cursor") === "older";
  return route.fulfill({ json: { files: [file(next ? "1501" : "0001")], total: 1501, filtered: 1501, counts: {}, nextCursor: next ? undefined : "older" } });
 });
 await page.route("**/api/v1/library/files/rename/preview", route => {
  const ids = route.request().postDataJSON().ids; previews.push(ids);
  return route.fulfill({ json: { requested: ids.length, renamed: 0, skipped: 0, errored: 0, results: [], previews: ids.map((id: string) => ({ revision: `revision-${id}`, file: file(id), sourcePath: file(id).path, destinationPath: `/renamed/${id}.mp3`, noop: false })) } });
 });
 await page.route("**/api/v1/library/files/rename", route => {
  const body = route.request().postDataJSON(); const ids = body.ids; applied.push(ids);
  expect(body.revisions).toEqual(Object.fromEntries(ids.map((id: string) => [id, `revision-${id}`])));
  return route.fulfill({ json: { requested: ids.length, renamed: ids.length, skipped: 0, errored: 0, previews: [], results: [] } });
 });
 await page.goto("/library");
 await page.getByRole("button", { name: "Rename Files", exact: true }).click();
 const dialog = page.getByRole("dialog");
 await expect(dialog.getByText("1 shown · 1501 matching · 1501 total files", { exact: true })).toBeVisible();
 await dialog.getByRole("button", { name: "Next preview", exact: true }).click();
 await expect(dialog.getByRole("checkbox", { name: `Rename ${file("1501").path}`, exact: true })).toBeChecked();
 await expect(dialog.getByRole("checkbox", { name: `Rename ${file("0001").path}`, exact: true })).toHaveCount(0);
 await dialog.getByRole("button", { name: "Apply 1 Rename", exact: true }).click();
 await expect.poll(() => applied).toEqual([["1501"]]);
 expect(previews).toContainEqual(["0001"]); expect(previews).toContainEqual(["1501"]);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test("Rename explains retained chapter sets and preview failures", async ({ page }) => {
 const reason = "Keep this file with its chapter set or companion files. Renaming a complete book set is not supported yet.";
 await page.route("**/api/v1/library/files/collection?**", route => route.fulfill({ json: { files: [file("0001"), file("0002")], total: 2, filtered: 2, counts: {} } }));
 await page.route("**/api/v1/library/files/rename/preview", route => route.fulfill({ json: { requested: 2, renamed: 0, skipped: 1, errored: 1, previews: [{ file: file("0001"), sourcePath: file("0001").path, destinationPath: file("0001").path, noop: true, reason, revision: "retained" }], results: [{ preview: { file: file("0002"), sourcePath: file("0002").path }, status: "error", message: "Source is unavailable; restore the mount before renaming" }] } }));
 await page.goto("/library");
 await page.getByRole("button", { name: "Rename Files", exact: true }).click();
 const dialog = page.getByRole("dialog");
 await expect(dialog.getByText(reason, { exact: true })).toBeVisible();
 await expect(dialog.getByText(/Source is unavailable/)).toBeVisible();
 await expect(dialog.getByRole("checkbox", { name: `Rename ${file("0001").path}`, exact: true })).toBeDisabled();
 await expect(dialog.getByRole("button", { name: "Apply 0 Renames", exact: true })).toBeDisabled();
});

test("Imports identifies saved renames and retries their original plan", async ({ page }) => {
 let recovered = false;
 const requests: string[] = [];
 await page.route("**/api/v1/library/import-recovery", route => route.fulfill({ json: { unfinished: recovered ? 0 : 1, unresolved: 0, limit: 100, issues: [], operations: [{ id: "saved-rename", sourceKind: "manual", state: recovered ? "committed" : "failed", cleanupState: recovered ? "cleaned" : "blocked", attempts: recovered ? 2 : 1, metadata: { title: "Fixture rename", renameFileId: "preserved-id" }, files: [{ id: "manifest", sourcePath: "/old/Fixture.epub", destinationPath: "/saved/Fixture.epub", sizeBytes: 1234, state: recovered ? "committed" : "verified", sourceRemoved: recovered }], lastError: recovered ? "" : "Controlled commit failure" }] } }));
 await page.route("**/api/v1/library/import-operations/saved-rename/retry", route => { requests.push(route.request().url()); recovered = true; return route.fulfill({ json: { imported: true, operationId: "saved-rename", file: { id: "preserved-id" } } }); });
 await page.goto("/imports");
 const operation = page.locator(".imports-recovery-item").filter({ hasText: "Fixture rename" });
 await operation.locator("summary").click();
 await expect(operation.getByText(/Saved file rename/)).toBeVisible();
 await expect(operation.getByText("Existing book links are preserved", { exact: true })).toBeVisible();
 await expect(operation.getByText("Controlled commit failure", { exact: true })).toBeVisible();
 await operation.getByRole("button", { name: "Retry rename", exact: true }).click();
 await expect(page.getByText("Recovery completed.", { exact: true })).toBeVisible();
 await expect(operation.getByRole("button", { name: "Retry rename", exact: true })).toHaveCount(0);
 await expect(operation.getByText(/move completed/)).toBeVisible();
 expect(requests).toHaveLength(1);
});
