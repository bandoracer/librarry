import { expect, test } from "@playwright/test";

test("task history explains interrupted runs and survives navigation", async ({ page }, testInfo) => {
  const task = { id: "wanted-monitor", name: "Wanted Monitor", interval: "30m", running: false, runState: "interrupted", lastRunAt: "2026-09-16T12:00:00Z", lastError: "Worker connection ended; completion is unverified" };
  await page.route("**/api/v1/system/tasks", route => route.fulfill({ json: { tasks: [task] } }));
  let unavailable = true;
  await page.route("**/api/v1/system/tasks/wanted-monitor/runs", route => unavailable ? route.fulfill({ status: 503, json: { error: "Task history is temporarily unavailable" } }) : route.fulfill({ json: { runs: [{ id: "run-1", taskId: task.id, trigger: "scheduled", state: "interrupted", startedAt: task.lastRunAt, heartbeatAt: task.lastRunAt, error: task.lastError }, { id: "run-2", taskId: task.id, trigger: "manual", state: "completed", startedAt: task.lastRunAt, heartbeatAt: task.lastRunAt, outcome: "20 wanted books checked" }], limit: 100 } }));
  await page.goto("/providers/tasks");
  const history = page.getByRole("button", { name: "View Wanted Monitor run history" });
  await history.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Task history is temporarily unavailable", { exact: false })).toBeVisible();
  unavailable = false;
  await dialog.getByRole("button", { name: "Try again" }).click();
  await expect(dialog.getByText("20 wanted books checked", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Worker connection ended; completion is unverified", { exact: true })).toBeVisible();
  await expect.poll(() => dialog.locator(".modal-body").evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
  const bounds = await dialog.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(page.viewportSize()!.width + 1);
  await page.screenshot({ path: `../output/playwright/task-history-${testInfo.project.name}.png`, fullPage: true });
  await page.keyboard.press("Escape");
  await expect(history).toBeFocused();
  await page.goto("/library");
  await page.goto("/providers/tasks");
  await history.click();
  await expect(dialog.getByText("20 wanted books checked", { exact: true })).toBeVisible();
});
