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

test("task failures paginate, reject stale reviews, and refresh task counts", async ({ page }, testInfo) => {
  const id = "12345678-1234-1234-1234-123456789abc";
  const task = { id: "wanted-monitor", name: "Wanted Monitor", interval: "30m", running: false, runState: "degraded", lastRunAt: "2026-09-16T12:00:00Z", lastSuccessAt: "2026-09-16T11:00:00Z", unreviewedFailures: 1 };
  let reviewedAt: string | undefined;
  let stale = true;
  await page.route("**/api/v1/system/tasks", route => route.fulfill({ json: { tasks: [{ ...task, unreviewedFailures: reviewedAt ? 0 : 1 }] } }));
  await page.route("**/api/v1/system/tasks/wanted-monitor/runs**", async route => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/review")) {
      const body = route.request().postDataJSON();
      expect(body.expectedState).toBe("degraded");
      expect(body.expectedReviewedAt ?? undefined).toBe(reviewedAt);
      if (stale) { stale = false; await route.fulfill({ status: 409, json: { error: "Task run changed; refresh before reviewing it" } }); return; }
      reviewedAt = body.reviewed ? "2026-09-16T13:00:00Z" : undefined;
      await route.fulfill({ json: { ok: true } }); return;
    }
    const unreviewed = url.searchParams.get("view") === "unreviewed";
    const offset = Number(url.searchParams.get("offset") ?? 0);
    const failure = { id, taskId: task.id, state: "degraded", trigger: "scheduled", startedAt: task.lastRunAt, durationMs: 1200, reviewedAt, details: { counts: { checked: 25, errors: 2 }, errors: 2, operationIds: [id], nextAction: "Review wanted monitoring history." } };
    const runs = unreviewed ? (reviewedAt ? [] : [failure]) : offset === 0 ? Array.from({ length: 100 }, (_, i) => ({ id: `success-${i}`, state: "completed", trigger: "scheduled", startedAt: task.lastRunAt, outcome: "Routine successful run" })) : [failure];
    await route.fulfill({ json: { runs, total: unreviewed ? runs.length : 101, offset, limit: 100 } });
  });
  await page.goto("/providers/tasks");
  await expect(page.getByRole("columnheader", { name: "Last Success", exact: true })).toBeVisible();
  await expect(page.getByText("1 unreviewed", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "View Wanted Monitor run history" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Next runs" }).click();
  await expect(dialog.getByText("Review wanted monitoring history.", { exact: true })).toBeVisible();
  await expect(dialog.getByText("checked: 25 · errors: 2", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Next runs" })).toBeDisabled();
  await dialog.getByRole("button", { name: "Mark reviewed", exact: true }).click();
  await expect(dialog.getByText("Task run changed; refresh before reviewing it", { exact: false })).toBeVisible();
  await dialog.getByRole("button", { name: "Mark reviewed", exact: true }).click();
  await expect(dialog.getByRole("button", { name: "Mark unreviewed", exact: true })).toBeVisible();
  await expect(page.getByText("1 unreviewed", { exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "Mark unreviewed", exact: true }).click();
  await dialog.getByLabel("Task history view").selectOption("unreviewed");
  await expect(dialog.getByText("1 retained run", { exact: true })).toBeVisible();
  await expect.poll(() => dialog.locator(".modal-body").evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/task-diagnostics-${testInfo.project.name}.png`, fullPage: true });
  await dialog.getByRole("button", { name: "Mark reviewed", exact: true }).click();
  await expect(dialog.getByText("No matching runs on this page.", { exact: true })).toBeVisible();
});
