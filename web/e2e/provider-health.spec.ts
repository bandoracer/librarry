import { expect, test } from "@playwright/test";

test("provider health separates configured credentials from observed success and retains check failures", async ({ page }, testInfo) => {
  const providers: Record<string, unknown>[] = [
    { name: "Hardcover", status: "configured", configured: true, message: "Token configured; connection has not been checked." },
    { name: "Google Books", status: "missing_credentials", configured: false, message: "Configure an API key." },
    { name: "Local OPF", status: "ready", configured: true, message: "Local import evidence." }
  ];
  let checks = 0;
  await page.route("**/api/v1/providers/health", route => route.fulfill({ json: { providers } }));
  await page.route("**/api/v1/providers/Hardcover/check", route => {
    expect(route.request().method()).toBe("POST");
    if (++checks === 1) return route.fulfill({ status: 502, json: { error: "Connection check interrupted" } });
    providers[0] = { ...providers[0], status: checks === 2 ? "invalid_credentials" : "ready", reachable: true, authenticated: checks > 2,
      lastCheckedAt: new Date().toISOString(), lastSuccessAt: checks > 2 ? new Date().toISOString() : undefined,
      message: checks === 2 ? "Hardcover rejected the token. Check the configured token." : "Credentials accepted by the last successful request." };
    return route.fulfill({ json: providers[0] });
  });
  await page.goto("/providers");
  const card = page.locator("article").filter({ has: page.getByRole("button", { name: "Check Hardcover", exact: true }) });
  await expect(card).toContainText("Connection not checked");
  expect(checks).toBe(0);
  await expect(page.getByRole("button", { name: "Check Google Books", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Check Local OPF" })).toHaveCount(0);
  const check = card.getByRole("button", { name: "Check Hardcover", exact: true });
  await check.click();
  await expect(card.getByRole("alert")).toContainText("Connection check interrupted");
  await expect(card).toContainText("Connection not checked");
  await check.click();
  await expect(card).toContainText("invalid credentials");
  await expect(card).toContainText("Last request");
  await expect(card).not.toContainText("Last success");
  await check.click();
  await expect(card).toContainText("Credentials accepted");
  await expect(card).toContainText("Last success");
  await expect(card.getByRole("alert")).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  await card.screenshot({ path: `../output/playwright/provider-health-${testInfo.project.name}.png` });
  expect(checks).toBe(3);
});
