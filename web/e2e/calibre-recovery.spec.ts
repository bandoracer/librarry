import { expect, test } from "@playwright/test";

test("uncertain Calibre upload requires inspection and retains its saved identity", async ({ page }, testInfo) => {
  const handoff = { id: "00000000-0000-0000-0000-000000000046", sourcePath: "/downloads/a-long-book-folder/public-domain-fixture.epub", rootFolderId: "root", phase: "uploading", bookId: 0, lastError: "Upload acknowledgement is uncertain", attempts: 1, conversions: [{ format: "TXT", state: "planned", jobId: 0 }] };
  await page.route("**/api/v1/library/import-recovery", route => route.fulfill({ json: { operations: [], issues: [], unfinished: 0, unresolved: 0, limit: 100, calibreUnfinished: handoff.phase === "committed" ? 0 : 1, calibreHandoffs: [handoff] } }));
  let resolved = 0, retried = 0;
  await page.route(`**/api/v1/library/calibre-handoffs/${handoff.id}/resolve`, route => {
    expect(route.request().postDataJSON()).toEqual({ action: "attach-book", confirm: true, bookId: 42 });
    resolved++; handoff.phase = "accepted"; handoff.bookId = 42; handoff.lastError = "";
    return route.fulfill({ json: { skipped: true, message: "Decision saved" } });
  });
  await page.route(`**/api/v1/library/calibre-handoffs/${handoff.id}/retry`, route => {
    retried++; handoff.phase = "committed"; handoff.conversions[0].state = "done";
    return route.fulfill({ json: { imported: true, message: "Calibre handoff committed; source retained" } });
  });
  await page.goto("/imports");
  const section = page.getByRole("region", { name: "Calibre recovery" });
  await section.locator("summary").click();
  await expect(section.getByRole("button", { name: "Retry Calibre handoff", exact: true })).toBeDisabled();
  const attach = section.getByRole("button", { name: "Attach existing book", exact: true });
  await section.getByRole("spinbutton", { name: "Existing Calibre book ID" }).fill("42");
  await expect(attach).toBeDisabled();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/calibre-uncertain-${testInfo.project.name}.png`, fullPage: true });
  await section.getByRole("checkbox").check();
  await attach.click();
  await expect.poll(() => resolved).toBe(1);
  await expect(section.getByText(/Saved book ID 42/)).toBeVisible();
  await section.getByRole("button", { name: "Retry Calibre handoff", exact: true }).click();
  await expect.poll(() => retried).toBe(1);
  await expect(section.getByText("Calibre handoff committed; source retained", { exact: true })).toBeVisible();
  await expect(section.getByText("TXT: done · Job 0", { exact: true })).toBeVisible();
  await expect(section.getByRole("button", { name: "Retry Calibre handoff", exact: true })).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/calibre-recovery-${testInfo.project.name}.png`, fullPage: true });
});
