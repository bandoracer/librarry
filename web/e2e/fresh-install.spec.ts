import { test, expect } from "@playwright/test";

test("fresh database supports core navigation without a crash", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
  for (const path of ["/library", "/library/authors", "/wanted", "/downloads", "/imports", "/search", "/settings", "/providers/tasks"]) {
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
  await page.route("**/api/v1/wanted?view=library", route => route.fulfill({ contentType: "application/json", body: JSON.stringify({ wanted: [{ id: "fixture", title: "Fixture book", authorName: "Fixture author", format: "ebook", qualityProfile: "Default", status: "wanted", monitored: true }] }) }));
  await page.goto("/library");
  const trigger = page.getByRole("button", { name: "Rename Files" });
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
