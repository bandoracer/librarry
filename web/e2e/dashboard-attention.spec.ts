import { expect, test } from "@playwright/test";

test("dashboard shows complete counts, recovery links and never clears failed sources", async ({ page }, testInfo) => {
  let populated = true, fail = false, malformed = false, evidence = "fresh";
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/api/v1/wanted/metadata/review?**", route => route.fulfill({ json: { items: [], total: populated ? 10001 : 0, filtered: populated ? 10001 : 0, conflictCount: 0, generatedAt: "2026-09-16T17:00:00Z" } }));
  await page.route("**/api/v1/authors/metadata/review?**", route => route.fulfill({ json: { reviews: [], total: 1001, filtered: populated ? 501 : 0, counts: { pending: populated ? 501 : 0 } } }));
  await page.route("**/api/v1/system/attention", route => fail
    ? route.fulfill({ status: 503, json: { error: "Fixture database outage" } })
    : malformed ? route.fulfill({ json: {} }) : route.fulfill({ json: { observedAt: "2026-09-16T17:00:00Z", importReviews: populated ? 301 : 0, importOperations: populated ? 201 : 0, calibreHandoffs: populated ? 101 : 0, legacyLinks: populated ? 1001 : 0 } }));
  await page.route("**/api/v1/acquisition/queue?**", route => {
    expect(new URL(route.request().url()).searchParams.get("limit")).toBe("8");
    return route.fulfill({ json: { items: [], previewLimit: 8, generatedAt: "2026-09-16T17:00:00Z", downloads: evidence, summary: { total: 10001, needsSearch: evidence === "fresh" ? 8001 : 0, readyToGrab: 0, queued: 0, importReady: 0, imported: 1000, blocked: populated ? 1000 : 0, unknown: evidence === "fresh" ? 0 : 8001 } } });
  });
  await page.route("**/api/v1/downloads?**", route => route.fulfill({ json: { downloads: [] } }));
  await page.goto("/dashboard");
  const row = (label: string) => page.locator("tr").filter({ has: page.getByText(label, { exact: true }) });
  for (const [label, count] of [["Metadata reviews", "10001"], ["Author candidates", "501"], ["Import reviews", "301"], ["Unfinished imports", "201"], ["Calibre handoffs", "101"], ["Unresolved file links", "1001"], ["Blocked acquisitions", "1000"]]) {
    await expect(row(label).getByText(count, { exact: true })).toBeVisible();
  }
  await expect(page.getByText("10001 active books across the acquisition ledger", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Open metadata reviews" })).toHaveAttribute("href", "/wanted/review");
  await expect(page.getByRole("link", { name: "Open author candidates" })).toHaveAttribute("href", "/library/authors");
  await expect(page.getByText("All caught up", { exact: true })).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/dashboard-attention-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("link", { name: "Open unfinished imports" }).click();
  await expect(page).toHaveURL(/\/imports\?unfinishedOnly=true#recovery$/);
  await expect(page.getByRole("checkbox", { name: "Show only unfinished imports and Calibre handoffs" })).toBeChecked();
  await page.goto("/dashboard");
  populated = false; fail = true;
  await page.getByRole("button", { name: "Refresh attention", exact: true }).click();
  await expect(page.getByText(/Some queues could not be loaded/)).toBeVisible();
  await expect(page.getByText("All caught up", { exact: true })).toHaveCount(0);
  fail = false; evidence = "unavailable";
  await page.getByRole("button", { name: "Refresh attention", exact: true }).click();
  await expect(page.getByText(/Download-client evidence is incomplete/)).toBeVisible();
  await expect(page.getByRole("link", { name: "Check integrations", exact: true })).toHaveAttribute("href", "/providers");
  await expect(page.getByText("Attention status unavailable", { exact: true })).toBeVisible();
  evidence = "fresh";
  await page.getByRole("button", { name: "Refresh attention", exact: true }).click();
  await expect(page.getByText("All caught up", { exact: true })).toBeVisible();
  malformed = true;
  await page.getByRole("button", { name: "Refresh attention", exact: true }).click();
  await expect(page.getByText("Attention status unavailable", { exact: true })).toBeVisible();
  await expect(page.getByText("All caught up", { exact: true })).toHaveCount(0);
  expect(errors).toEqual([]);
});
