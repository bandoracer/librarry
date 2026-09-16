import { test, expect } from "@playwright/test";

test("payload mapping requires a current preview and preserves review errors", async ({ page }, testInfo) => {
  const id = "00000000-0000-0000-0000-000000000003";
  const book = { id, title: "Fixture Book", authorName: "Fixture Author", format: "audiobook", status: "wanted", monitored: true };
  const files = ["Disc 1/01.mp3", "Disc 2/01.mp3", "cover.jpg"].map(relativePath => ({ relativePath, sourcePath: `/downloads/Fixture/${relativePath}`, format: relativePath.endsWith("mp3") ? "audiobook" : "sidecar", included: true, selected: true, progress: 1, sizeBytes: 1024 }));
  const review = { id, title: "Uncertain audiobook", authorName: "", status: "pending", reason: "Chapter grouping needs review", mediaFormat: "audiobook", sourcePath: "/downloads/Fixture", sizeBytes: 3072, metadata: { payloadReview: true, downloadClient: "qBittorrent", payload: { files } }, createdAt: "2026-09-15T00:00:00Z", updatedAt: "2026-09-15T00:00:00Z" };
  await page.route("**/api/v1/wanted?view=library", route => route.fulfill({ json: { wanted: [book] } }));
  await page.route("**/api/v1/library/import-reviews?*", route => route.fulfill({ json: { reviews: [review] } }));
  let previews = 0;
  await page.route(`**/api/v1/library/import-reviews/${id}/preview`, async route => {
    const body = route.request().postDataJSON();
    expect(body.confirmIdentity).toBe(true);
    expect(body.mapping[0].wantedId).toBe(id);
    previews++;
    await route.fulfill({ json: { fingerprint: `preview-${previews}`, operation: { files: files.filter(file => !body.mapping.find((mapping: { relativePath: string; exclude?: boolean }) => mapping.relativePath === file.relativePath)?.exclude).map(file => ({ ...file, destinationPath: `/library/Fixture/${file.relativePath}` })) } } });
  });
  await page.route(`**/api/v1/library/import-reviews/${id}/resolve`, async route => {
    const body = route.request().postDataJSON();
    expect(body.previewToken).toBe("preview-2");
    await route.fulfill({ status: 409, json: { error: "Payload changed; refresh the preview" } });
  });
  await page.goto("/imports");
  await expect(page.getByText("Review files: Uncertain audiobook", { exact: true })).toBeVisible();
  const preview = page.getByRole("button", { name: "Preview destinations", exact: true });
  await expect(preview).toBeDisabled();
  await page.getByLabel("Book for all media files", { exact: true }).selectOption(id);
  await page.getByRole("button", { name: "Apply book to all", exact: true }).click();
  const confirm = page.getByRole("checkbox", { name: "I checked these book assignments", exact: false });
  await confirm.check();
  await preview.click();
  await expect(page.getByText("→ /library/Fixture/Disc 2/01.mp3", { exact: true })).toBeVisible();
  await page.getByLabel("Book for cover.jpg", { exact: true }).selectOption("__retain");
  await expect(page.getByRole("button", { name: "Import this file set", exact: true })).toHaveCount(0);
  await expect(confirm).not.toBeChecked();
  await confirm.check();
  await preview.click();
  await page.getByRole("button", { name: "Import this file set", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Payload changed; refresh the preview");
  await expect(page.getByRole("button", { name: "Skip this download", exact: true })).toBeEnabled();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/payload-review-${testInfo.project.name}.png`, fullPage: true });
});

test("replacement changes require a fresh preview and expose existing destinations", async ({ page }, testInfo) => {
  const id = "00000000-0000-0000-0000-000000000039";
  const file = { relativePath: "Book.epub", sourcePath: "/downloads/Book.epub", format: "ebook", included: true, selected: true, progress: 1, sizeBytes: 2048 };
  await page.route("**/api/v1/wanted?view=library", route => route.fulfill({ json: { wanted: [{ id, title: "Replacement Book", format: "ebook" }] } }));
  await page.route("**/api/v1/library/import-reviews?*", route => route.fulfill({ json: { reviews: [{ id, wantedId: id, title: "Replacement Book", status: "pending", reason: "Review replacement", metadata: { payloadReview: true, payload: { files: [file] } } }] } }));
  let previews = 0;
  await page.route(`**/api/v1/library/import-reviews/${id}/preview`, route => {
    const request = route.request().postDataJSON(); previews++;
    return route.fulfill({ json: { fingerprint: `preview-${previews}`, operation: { files: [{ ...file, destinationPath: "/library/Book.epub", previousPath: request.conflictAction === "replace" ? "/library/.saved-previous" : undefined, previousSizeBytes: 1024 }] } } });
  });
  await page.route(`**/api/v1/library/import-reviews/${id}/resolve`, route => {
    expect(route.request().postDataJSON()).toMatchObject({ conflictAction: "replace", previewToken: "preview-2", confirmIdentity: true });
    return route.fulfill({ status: 409, json: { error: "Destination changed; refresh the replacement preview" } });
  });
  await page.goto("/imports");
  const confirmation = page.getByRole("checkbox", { name: "I checked these book assignments", exact: false });
  await confirmation.check();
  await page.getByRole("button", { name: "Preview destinations", exact: true }).click();
  await expect(page.getByRole("button", { name: "Import this file set", exact: true })).toBeVisible();
  await page.getByLabel("Existing destinations", { exact: true }).selectOption("replace");
  await expect(confirmation).not.toBeChecked();
  await expect(page.getByRole("button", { name: "Import this file set", exact: true })).toHaveCount(0);
  await confirmation.check();
  await page.getByRole("button", { name: "Preview destinations", exact: true }).click();
  await expect(page.getByText(/Replaces an existing .*file. The preview is bound to its current content./)).toBeVisible();
  await page.screenshot({ path: `../output/playwright/completed-replacement-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "Import this file set", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Destination changed; refresh the replacement preview");
});

test("completed replacement backup cleanup remains separate from download removal", async ({ page }) => {
  const id = "00000000-0000-0000-0000-000000000039";
  const operation = { id, sourceKind: "completed", state: "committed", cleanupState: "blocked", replacementCleanupState: "pending", replacementCleanupError: "Recycle folder unavailable; previous file retained", metadata: { title: "Replacement Book" }, files: [{ id: "file", sourcePath: "/downloads/Book.epub", destinationPath: "/library/Book.epub", previousPath: "/library/.previous", sizeBytes: 2048, state: "committed" }] };
  await page.route("**/api/v1/library/import-recovery", route => route.fulfill({ json: { operations: [operation], issues: [], unfinished: operation.replacementCleanupState === "pending" ? 1 : 0, unresolved: 0, limit: 100 } }));
  await page.route(`**/api/v1/library/import-operations/${id}/retry`, route => {
    operation.replacementCleanupState = "cleaned"; operation.replacementCleanupError = "";
    return route.fulfill({ json: { imported: true, skipped: true } });
  });
  await page.goto("/imports");
  await page.getByText("Replacement Book", { exact: true }).click();
  await expect(page.getByText("Replacement backups: cleanup pending", { exact: true })).toBeVisible();
  await expect(page.getByText("Recycle folder unavailable; previous file retained", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Retry cleanup", exact: true }).click();
  await expect(page.getByText("Replacement backups: cleanup complete", { exact: true })).toBeVisible();
  await expect(page.getByText(/Cleanup: source retained/)).toBeVisible();
  await expect(page.getByText("Previous file recovery path: /library/.previous", { exact: true })).toHaveCount(0);
});
