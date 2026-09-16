import { expect, test } from "@playwright/test";

test("integration checks expose evidence and ordinary refreshes do not probe", async ({ page }, testInfo) => {
 let checks = 0;
 let fail = false;
 let integration: Record<string, unknown> = { name: "qBittorrent", configured: true, status: "configured", freshness: "never_checked", message: "Configured; connection has not been checked." };
 await page.route("**/api/v1/integrations/health", route => route.fulfill({ json: { integrations: [integration, { name: "SABnzbd", configured: false, status: "missing_credentials", message: "Configure the integration." }] } }));
 await page.route("**/api/v1/integrations/qBittorrent/check", route => {
  checks++;
  if (fail) return route.fulfill({ status: 409, json: { error: "Integration configuration changed; refresh and check again" } });
  integration = { ...integration, status: "ready", freshness: "fresh", lastCheckedAt: "2026-09-16T17:00:00Z", lastSuccessAt: "2026-09-16T17:00:00Z", version: "5.0.4", lastVersionAt: "2026-09-16T17:00:00Z", message: "Connection verified by a read-only API request." };
  return route.fulfill({ json: integration });
 });
 await page.goto("/providers");
 const button = page.getByRole("button", { name: "Check qBittorrent", exact: true });
 await expect(button).toBeVisible();
 await expect(page.getByRole("button", { name: "Check SABnzbd", exact: true })).toBeDisabled();
 expect(checks).toBe(0);
 await page.getByRole("button", { name: "Refresh", exact: true }).click();
 expect(checks).toBe(0);
 await button.click();
 await expect(page.getByText("Last known version 5.0.4", { exact: false })).toBeVisible();
 expect(checks).toBe(1);
 fail = true;
 await button.click();
 await expect(page.getByText("Integration configuration changed", { exact: false })).toBeVisible();
 await expect(button).toBeEnabled();
 integration = { ...integration, status: "stale", observedStatus: "ready", freshness: "stale", message: "The last connection check is older than 10 minutes. Check again for current evidence." };
 await page.getByRole("button", { name: "Refresh", exact: true }).click();
 await expect(page.getByText("Previous result: ready", { exact: true })).toBeVisible();
 await expect(page.getByText("0/2 acquisition integrations ready.", { exact: true })).toBeVisible();
 await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
 await button.scrollIntoViewIfNeeded();
 await page.screenshot({ path: `../output/playwright/integration-health-${testInfo.project.name}.png` });
 integration = { ...integration, status: "rate_limited", freshness: "fresh", retryAfter: "2099-01-01T00:00:00Z" };
 await page.getByRole("button", { name: "Refresh", exact: true }).click();
 await expect(button).toBeDisabled();
});
