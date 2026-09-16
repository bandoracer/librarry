import { expect, test } from "@playwright/test";

test("scan progress survives navigation and exposes resume and cancellation", async ({ page }, testInfo) => {
  const id = "00000000-0000-0000-0000-000000000011";
  const job = { id, format: "any", roots: ["/library/fixture"], state: "failed", phase: "discover", scanned: 10_001, upserted: 10_001, skipped: 2, missing: 0, cancelRequested: false, lastError: "Scan root unavailable; previous presence retained." };
  await page.route("**/api/v1/library/scans", route => route.fulfill({ json: { scans: [job], limit: 100 } }));
  await page.route(`**/api/v1/library/scans/${id}`, route => {
    const { action } = route.request().postDataJSON();
    if (action === "retry") { job.state = "queued"; job.lastError = ""; }
    else { expect(action).toBe("cancel"); job.state = "cancelled"; job.cancelRequested = true; }
    return route.fulfill({ json: job });
  });
  await page.goto("/imports");
  await page.getByText("All books", { exact: true }).click();
  await expect(page.getByText("10,001 checked", { exact: true })).toBeVisible();
  await expect(page.getByText("Scan root unavailable; previous presence retained.", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Resume scan", exact: true }).click();
  await expect(page.getByText("queued", { exact: true })).toBeVisible();
  await page.goto("/library");
  await page.goto("/imports");
  await page.getByText("All books", { exact: true }).click();
  await expect(page.getByText("10,001 checked", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Cancel scan", exact: true }).click();
  await expect(page.getByText("cancelled", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Resume scan", exact: true })).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/scan-recovery-${testInfo.project.name}.png`, fullPage: true });
});

test("a replaced custom root requires a fresh acknowledgement", async ({ page }) => {
  let attempts = 0;
  await page.route("**/api/v1/library/scans", route => {
    if (route.request().method() !== "POST") return route.fallback();
    const request = route.request().postDataJSON();
    attempts++;
    if (attempts === 1) return route.fulfill({ status: 502, json: { error: "scan root identity changed: /library/replaced; verify the mounted library" } });
    expect(request).toMatchObject({ root: "/library/replaced", acceptRootChange: true });
    return route.fulfill({ json: { id: "fixture", roots: [request.root], state: "completed", phase: "complete", hasMore: false, scanned: 0, upserted: 0, skipped: 0, missing: 0, files: [] } });
  });
  await page.goto("/imports");
  const confirmation = page.getByRole("checkbox", { name: "I verified the library is mounted correctly; use this replacement folder." });
  await expect(confirmation).toHaveCount(0);
  await page.getByPlaceholder("/data/media/books/ebooks").fill("/library/replaced");
  const scan = page.getByRole("group", { name: "Scan custom library root" }).getByRole("button", { name: "All", exact: true });
  await scan.click();
  await expect(confirmation).toBeVisible();
  await expect(confirmation).not.toBeChecked();
  await confirmation.check();
  await scan.click();
  await expect.poll(() => attempts).toBe(2);
  await expect(confirmation).toHaveCount(0);
});
