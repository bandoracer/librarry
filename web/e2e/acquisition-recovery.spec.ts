import { expect, test } from "@playwright/test";

test("uncertain acquisitions retain errors and require an explicit release decision", async ({ page }, testInfo) => {
  const id = "00000000-0000-0000-0000-000000000010";
  const intents = [{ id, title: "Uncertain fixture", client: "qBittorrent", state: "uncertain", attempts: 1, lastError: "The client may have accepted the request." }];
  await page.route("**/api/v1/acquisition-recovery", route => route.fulfill({ json: { intents, limit: 200 } }));
  let released = false;
  await page.route(`**/api/v1/acquisition-recovery/${id}`, route => {
    const body = route.request().postDataJSON();
    if (body.action === "check") return route.fulfill({ status: 409, json: { error: "No unique matching download was found; original retained." } });
    expect(body).toMatchObject({ action: "release", confirmed: true });
    released = true; intents.splice(0);
    return route.fulfill({ json: { resolved: true } });
  });
  await page.goto("/downloads");
  await expect(page.getByText("Uncertain fixture", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Check client", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("No unique matching download");
  await expect(page.getByText("Uncertain fixture", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Allow new attempt…", exact: true }).click();
  const dialog = page.getByRole("dialog");
  const allow = dialog.getByRole("button", { name: "Allow new attempt", exact: true });
  await expect(allow).toBeDisabled();
  await expect(dialog).toContainText("it does not submit a download immediately");
  await dialog.getByRole("checkbox").check();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/acquisition-recovery-${testInfo.project.name}.png`, fullPage: true });
  await allow.click();
  await expect.poll(() => released).toBe(true);
  await expect(dialog).toBeHidden();
  await expect(page.getByText("Uncertain fixture", { exact: true })).toHaveCount(0);
});
