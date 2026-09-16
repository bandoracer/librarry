import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";

test("support report downloads only on request and errors remain retryable", async ({ page }, testInfo) => {
  let calls = 0;
  let fail = true;
  const report = { formatVersion: 1, generatedAt: "2026-09-16T12:00:00Z", build: { commit: "fixture-source", imageDigest: "unknown" }, database: { status: "unavailable" }, providers: [], tasks: [], roots: [] };
  await page.route("**/api/v1/system/support", route => {
    calls++;
    return route.fulfill(fail ? { status: 503, json: { error: "Support report is temporarily unavailable" } } : { json: report });
  });
  await page.goto("/providers");
  const button = page.getByRole("button", { name: "Download support report", exact: true });
  await expect(button).toBeVisible();
  expect(calls).toBe(0);
  await button.click();
  await expect(page.getByText("Support report is temporarily unavailable", { exact: false })).toBeVisible();
  await expect(button).toBeEnabled();
  fail = false;
  const pending = page.waitForEvent("download");
  await button.click();
  const download = await pending;
  expect(download.suggestedFilename()).toBe("librarry-support.json");
  expect(JSON.parse(await readFile((await download.path())!, "utf8"))).toEqual(report);
  expect(calls).toBe(2);
  await expect(page.getByText("Support report is temporarily unavailable", { exact: false })).toHaveCount(0);
  await expect(page.getByText("Report prepared.", { exact: false })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await button.scrollIntoViewIfNeeded();
  await page.screenshot({ path: `../output/playwright/support-diagnostics-${testInfo.project.name}.png` });
});

test("live support export has real build and no invented provider request evidence", async ({ request }) => {
  const response = await request.get("/api/v1/system/support");
  expect(response.status()).toBe(200);
  expect(response.headers()["cache-control"]).toBe("no-store");
  const report = await response.json();
  expect(report.formatVersion).toBe(1);
  expect(report.build.commit).toBeTruthy();
  expect(report.build.imageDigest).toBe("unknown");
  expect(report.database.status).toBe("ready");
  expect(report.database.serverVersionNumber).toBeGreaterThan(0);
  expect(report.tasks.length).toBeGreaterThan(0);
  for (const provider of report.providers) {
    if (!provider.lastCheckedAt) expect(provider.lastSuccessAt).toBeUndefined();
    expect(provider.message).toBeUndefined();
  }
  for (const root of report.roots) expect(root.path).toBeUndefined();
  for (const task of report.tasks) expect(task.lastError).toBeUndefined();
});
