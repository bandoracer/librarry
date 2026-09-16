import { expect, test } from "@playwright/test";

const fixture = {
  targetKind: "compat", id: "delivery-1", eventId: "event-1", event: { type: "import", title: "Book imported: Walden", message: "/library/Walden.epub", fields: {} },
  targetId: "target-1", targetName: "Fixture receiver", targetType: "webhook", targetRevision: "2026-09-16T12:00:00Z", currentTargetRevision: "2026-09-16T13:00:00Z", targetAvailable: true,
  state: "uncertain", attempts: 1, statusCode: null, message: "Sender stopped before acceptance was recorded; inspect the receiver before retrying", nextAttemptAt: "2026-09-16T12:30:00Z", createdAt: "2026-09-16T12:30:00Z", updatedAt: "2026-09-16T12:31:00Z"
};

test("notification recovery requires a reviewed decision and exposes stale state", async ({ page }, testInfo) => {
  await page.route("**/api/v1/notifications", route => route.fulfill({ json: { targets: [] } }));
  let unavailable = true, resolved = false;
  const decisions: unknown[] = [];
  await page.route("**/api/v1/notification-deliveries?*", route => unavailable ? route.fulfill({ status: 503, json: { error: "Notification history is unavailable" } }) : route.fulfill({ json: { items: [{ ...fixture, state: resolved ? "pending" : "uncertain" }], total: 1, limit: 25, offset: 0 } }));
  await page.route("**/api/v1/notification-deliveries/delivery-1/resolve", async route => {
    decisions.push(route.request().postDataJSON());
    if (decisions.length === 1) return route.fulfill({ status: 409, json: { error: "Delivery changed; refresh before resolving it" } });
    resolved = true; return route.fulfill({ json: { ok: true } });
  });
  await page.goto("/settings/connect");
  await expect(page.getByText("Notification history is unavailable", { exact: false })).toBeVisible();
  unavailable = false; await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByText(fixture.event.title, { exact: true })).toBeVisible();
  await expect(page.getByText(/Fixture receiver \(Readarr webhook\)/)).toBeVisible();
  await page.getByRole("button", { name: /Review retry for/ }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("retrying can create a duplicate");
  await expect(dialog.getByRole("button", { name: "Save decision" })).toBeDisabled();
  await dialog.getByRole("checkbox").check();
  await dialog.getByRole("button", { name: "Save decision" }).click();
  await expect(dialog).toContainText("Delivery changed");
  expect(decisions[0]).toEqual({ action: "retry", confirm: true, expectedUpdatedAt: fixture.updatedAt, expectedTargetRevision: fixture.currentTargetRevision });
  await expect.poll(() => dialog.locator(".modal-body").evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/notification-recovery-${testInfo.project.name}.png`, fullPage: true });
  await dialog.getByRole("button", { name: "Back", exact: true }).click();
  await page.getByRole("button", { name: "Confirm acceptance", exact: true }).click();
  await expect(dialog).toContainText("without sending another request");
  await dialog.getByRole("checkbox").check();
  await dialog.getByRole("button", { name: "Save decision" }).click();
  await expect(dialog).toHaveCount(0);
  expect((decisions[1] as { action: string }).action).toBe("accepted");
});

test("notification delivery history traverses all pages", async ({ page }) => {
  await page.route("**/api/v1/notifications", route => route.fulfill({ json: { targets: [] } }));
  await page.route("**/api/v1/notification-deliveries?*", route => {
    const offset = Number(new URL(route.request().url()).searchParams.get("offset"));
    return route.fulfill({ json: { items: Array.from({ length: offset === 0 ? 25 : 1 }, (_, i) => ({ ...fixture, id: `delivery-${offset + i}`, event: { ...fixture.event, title: `Fixture message ${offset + i}` }, state: "accepted" })), total: 26, limit: 25, offset } });
  });
  await page.goto("/settings/connect");
  await expect(page.getByText("Fixture message 24", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Next deliveries" }).click();
  await expect(page.getByText("Fixture message 25", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Next deliveries" })).toBeDisabled();
  await page.getByRole("button", { name: "Previous deliveries" }).click();
  await expect(page.getByText("Fixture message 0", { exact: true })).toBeVisible();
});
