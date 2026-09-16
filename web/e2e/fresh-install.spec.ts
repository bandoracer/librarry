import { test, expect } from "@playwright/test";

test("fresh database supports core navigation without a crash", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
  for (const path of ["/dashboard", "/library", "/library/authors", "/wanted", "/downloads", "/imports", "/search", "/settings", "/providers/tasks"]) {
    await page.goto(path);
    await expect(page.locator("h1")).toBeVisible();
    await expect(page.locator(".page-loading")).toHaveCount(0);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
    expect(errors, path).toEqual([]);
  }
});

test("empty persisted lists use arrays and invalid views are rejected", async ({ request }) => {
  for (const [path, key] of [["/api/v1/wanted?view=library", "wanted"], ["/api/v1/authors", "authors"], ["/api/v1/library/files", "files"]]) {
    const response = await request.get(path);
    expect(response.ok()).toBe(true);
    expect((await response.json())[key]).toEqual([]);
  }
  expect((await request.get("/api/v1/wanted?view=%5Bobject%20Object%5D")).status()).toBe(400);
});

test("an unavailable authentication service offers recovery", async ({ page }) => {
  await page.route("**/api/v1/auth/status", route => route.fulfill({ status: 503, contentType: "application/json", body: '{"error":"fixture outage"}' }));
  await page.goto("/library");
  await expect(page.getByRole("heading", { name: "Can’t connect to Librarry" })).toBeVisible();
  await page.unroute("**/api/v1/auth/status");
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByRole("heading", { name: "Library", exact: true })).toBeVisible();
});

test("dialogs trap keyboard focus and restore their trigger", async ({ page }) => {
  let finishBooks: () => void = () => {};
  const booksReady = new Promise<void>(resolve => { finishBooks = resolve; });
  await page.route("**/api/v1/library/books?**", async route => { await booksReady; return route.fulfill({ json: { total: 1, filtered: 1, counts: { missing: 1 }, recordedFiles: 0, downloads: "notConfigured", books: [{ id: "fixture", title: "Fixture book", authorName: "Fixture author", format: "ebook", qualityProfile: "Default", status: "wanted", derivedState: "missing", monitored: true }] } }); });
  await page.goto("/library");
  const trigger = page.getByRole("button", { name: "Rename Files" });
  await expect(trigger).toBeVisible();
  const originalTrigger = await trigger.elementHandle();
  finishBooks();
  await expect(page.getByRole("button", { name: "RSS Sync" })).toBeVisible();
  expect(await originalTrigger!.evaluate(element => element.isConnected)).toBe(true);
  await trigger.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press("Tab");
    await expect.poll(() => dialog.evaluate(element => element.contains(document.activeElement))).toBe(true);
  }
  await page.keyboard.press("Shift+Tab");
  await expect.poll(() => dialog.evaluate(element => element.contains(document.activeElement))).toBe(true);
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
});


test("mobile navigation is hidden from keyboard users until opened", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile");
  await page.goto("/library");
  const menu = page.getByRole("navigation", { name: "Primary" });
  await expect(menu).toBeHidden();
  const trigger = page.getByRole("button", { name: "Open navigation" });
  await trigger.click();
  await expect(menu).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(menu).toBeHidden();
  await expect(trigger).toBeFocused();
});

test("environment-owned authentication is visible and cannot be edited", async ({ page }) => {
  await page.route("**/api/v1/auth/status", route => route.fulfill({ contentType: "application/json", body: JSON.stringify({ method: "forms", authenticated: true, username: "fixture", methodLocked: true, credentialsLocked: true }) }));
  await page.goto("/settings");
  await expect(page.getByLabel("Authentication method", { exact: true })).toBeDisabled();
  await expect(page.getByText("Authentication method is set by the server environment.", { exact: false })).toBeVisible();
  await expect(page.getByLabel("Authentication username", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Save authentication", exact: true })).toBeDisabled();
});

test("direct book links distinguish an outage from a missing book", async ({ page }) => {
  const id = "00000000-0000-0000-0000-000000000001";
  let unavailable = true;
  await page.route(`**/api/v1/wanted/${id}`, route => route.fulfill({ status: unavailable ? 503 : 200, contentType: "application/json", body: JSON.stringify(unavailable ? { error: "fixture outage" } : { id, title: "Older imported book", authorName: "Fixture author", format: "ebook", status: "imported", monitored: true, qualityProfile: "standard" }) }));
  await page.goto(`/library/book/${id}`);
  await expect(page.getByRole("heading", { name: "Book unavailable", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Book not found", exact: true })).toHaveCount(0);
  unavailable = false;
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Older imported book", exact: true })).toBeVisible();
  await page.unroute(`**/api/v1/wanted/${id}`);
  await page.goto(`/library/book/${id}`);
  await expect(page.getByRole("heading", { name: "Book not found", exact: true })).toBeVisible();
});

test("import recovery shows the saved plan and retains failures", async ({ page }, testInfo) => {
  const id = "00000000-0000-0000-0000-000000000002";
  const report = { unfinished: 1, unresolved: 1, limit: 100, operations: [{ id, wantedId: id, client: "qBittorrent", downloadId: "fixture", state: "failed", cleanupState: "blocked", attempts: 1, metadata: { title: "Interrupted fixture" }, lastError: "Database unavailable after publication", files: [{ id: "file", sourcePath: "/downloads/fixture.epub", destinationPath: "/library/Fixture/fixture.epub", sizeBytes: 1024, state: "verified", sha256: "a".repeat(64) }] }], issues: [{ fileId: "legacy", path: "/library/legacy.epub", kind: "download", reason: "download identifier is ambiguous across clients" }] };
  await page.route("**/api/v1/library/import-recovery", route => route.fulfill({ contentType: "application/json", body: JSON.stringify(report) }));
  await page.route(`**/api/v1/library/import-operations/${id}/retry`, route => route.fulfill({ status: 409, contentType: "application/json", body: '{"error":"Source checksum changed; review required"}' }));
  await page.goto("/imports");
  await page.getByText("Interrupted fixture", { exact: true }).click();
  await expect(page.getByText("/downloads/fixture.epub", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "View book", exact: true })).toHaveAttribute("href", `/library/book/${id}`);
  await page.getByRole("button", { name: "Retry import", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Source checksum changed; review required");
  await expect(page.getByText("Import verified and committed.", { exact: true })).toHaveCount(0);
  await page.getByText("Legacy links needing review (1)", { exact: true }).click();
  await expect(page.getByText("download identifier is ambiguous across clients", { exact: true })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/import-recovery-${testInfo.project.name}.png`, fullPage: true });
});

test("manual recovery distinguishes committed files from pending cleanup", async ({ page }, testInfo) => {
  const id = "00000000-0000-0000-0000-000000000004";
  const report = { unfinished: 1, unresolved: 0, limit: 100, issues: [], operations: [{ id, sourceKind: "manual", wantedId: "", client: "", downloadId: "", mode: "copy", state: "committed", cleanupState: "blocked", cleanupError: "Recycle bin unavailable; previous file retained", attempts: 1, metadata: { title: "Manual replacement" }, files: [{ id: "file", sourcePath: "/incoming/book.epub", destinationPath: "/library/book.epub", previousPath: "/library/.librarry-previous-fixture", sizeBytes: 1024, state: "committed", sha256: "a".repeat(64) }] }] };
  await page.route("**/api/v1/library/import-recovery", route => route.fulfill({ json: report }));
  await page.route(`**/api/v1/library/import-operations/${id}/retry`, route => {
    report.unfinished = 0;
    report.operations[0].cleanupState = "cleaned";
    report.operations[0].cleanupError = "";
    return route.fulfill({ json: { imported: true, skipped: true, operationId: id } });
  });
  await page.goto("/imports");
  await expect(page.getByText("cleanup pending", { exact: true })).toBeVisible();
  await page.getByText("Manual replacement", { exact: true }).click();
  await expect(page.getByText("No book assigned", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "View book", exact: true })).toHaveCount(0);
  await expect(page.getByText("Previous file recovery path:", { exact: false })).toBeVisible();
  await page.getByRole("button", { name: "Retry cleanup", exact: true }).click();
  await expect(page.getByText("Cleanup: complete; source retained", { exact: false })).toBeVisible();
  await expect(page.getByText("cleanup pending", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Retry cleanup", exact: true })).toHaveCount(0);
  await expect(page.getByText("Previous file recovery path:", { exact: false })).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/manual-recovery-${testInfo.project.name}.png`, fullPage: true });
});

test("manual book assignments and move options stay separate from completed imports", async ({ page }) => {
  const id = "00000000-0000-0000-0000-000000000005";
  await page.route("**/api/v1/library/book-choices?*", route => route.fulfill({ json: { books: [{ id, title: "Manual book", authorName: "Author", format: "ebook" }], total: 1, filtered: 1 } }));
  let imported = false;
  await page.route("**/api/v1/library/import", route => {
    expect(route.request().postDataJSON()).toMatchObject({ sourcePath: "/incoming/book.epub", wantedId: id, importMode: "move" });
    imported = true;
    return route.fulfill({ json: { imported: true, destinationPath: "/library/book.epub", file: { title: "Manual book" } } });
  });
  let completed = false;
  await page.route("**/api/v1/library/import-completed", route => {
    const body = route.request().postDataJSON();
    expect(body.importMode).toBe("hardlinkOrCopy");
    expect(body.move).toBe(false);
    expect(body.conflictAction).toBe("rename");
    completed = true;
    return route.fulfill({ json: { checked: 0, imported: 0, autoMatched: 0, reviewQueued: 0, skipped: 0, errored: 0, results: [] } });
  });
  await page.goto("/imports");
  await page.getByPlaceholder("Source file path to import into the library").fill("/incoming/book.epub");
  await page.getByLabel("Book for manual import", { exact: true }).selectOption(id);
  await page.getByLabel("Manual import mode", { exact: true }).selectOption("move");
  await page.getByRole("button", { name: "Import", exact: true }).click();
  await expect.poll(() => imported).toBe(true);
  await page.getByRole("button", { name: "Import Completed Downloads", exact: true }).click();
  await expect.poll(() => completed).toBe(true);
});
