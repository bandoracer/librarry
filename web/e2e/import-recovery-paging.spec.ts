import { expect, test } from "@playwright/test";

test("recovery pages each collection independently and describes transfer ownership accurately", async ({ page }, testInfo) => {
  const seen: URLSearchParams[] = [];
  let retries = 0;
  const operation = (id: string, leaseState: string, state = "transferring") => ({
    id, sourceKind: "manual", state, cleanupState: "blocked", client: "", downloadId: "", wantedId: "", attempts: 1,
    metadata: { title: id }, files: [],
    recovery: { leasePurpose: state === "committed" ? "cleanup" : "transfer", leaseState, observedAt: "2026-09-16T12:00:00Z", recordedAt: "2026-09-16T11:00:00Z", leaseExpiresAt: "2026-09-16T12:02:00Z", verifiedFiles: 2, totalFiles: 3 }
  });
  await page.route("**/api/v1/library/import-recovery*", route => {
    const q = new URL(route.request().url()).searchParams;
    seen.push(q);
    const older = !!q.get("operationsCursor"), calibreOlder = !!q.get("calibreCursor"), issuesOlder = !!q.get("issuesCursor");
    return route.fulfill({ json: {
      operations: older ? [operation("Older import", "none")] : [operation("Leased import", "held"), operation("Expired import", "expired"), operation("Cleanup import", "none", "committed"), operation("Leased cleanup", "held", "committed")],
      operationsPage: { total: 104, nextCursor: older ? undefined : "older-imports" },
      calibreHandoffs: [{ id: "handoff", sourcePath: calibreOlder ? "/fixture/older-calibre.epub" : "/fixture/newer-calibre.epub", phase: "planned", attempts: 0, conversions: [] }],
      calibrePage: { total: 101, nextCursor: calibreOlder ? undefined : "older-calibre" },
      issues: [{ fileId: "issue", kind: "wanted", path: issuesOlder ? "/fixture/older-link.epub" : "/fixture/newer-link.epub", reason: "Unresolved book association" }],
      issuesPage: { total: 101, nextCursor: issuesOlder ? undefined : "older-issues" },
      unfinished: 104, calibreUnfinished: 101, unresolved: 101, limit: 100
    } });
  });
  await page.route("**/api/v1/library/import-operations/*/retry", route => { retries++; return route.fulfill({ json: { imported: true } }); });
  await page.goto("/imports");
  const leased = page.locator("details").filter({ has: page.locator("summary", { hasText: "Leased import" }) });
  await leased.locator("summary").click();
  await expect(leased.getByText(/Transfer lease held until/)).toBeVisible();
  await expect(leased.getByText(/2 of 3 manifest files verified/)).toBeVisible();
  await expect(leased.getByRole("button", { name: "Retry import", exact: true })).toBeDisabled();
  const expired = page.locator("details").filter({ has: page.locator("summary", { hasText: "Expired import" }) });
  await expired.locator("summary").click();
  await expect(expired.getByText(/does not prove the previous process has stopped/)).toBeVisible();
  await expired.getByRole("button", { name: "Retry import", exact: true }).click();
  await expect.poll(() => retries).toBe(1);
  const cleanup = page.locator("details").filter({ has: page.locator("summary", { hasText: "Cleanup import" }) });
  await cleanup.locator("summary").click();
  await expect(cleanup.getByText(/No cleanup lease recorded/)).toBeVisible();
  await expect(cleanup.getByRole("button", { name: "Retry cleanup" })).toBeEnabled();

  const leasedCleanup = page.locator("details").filter({ has: page.locator("summary", { hasText: "Leased cleanup" }) });
  await leasedCleanup.locator("summary").click();
  await expect(leasedCleanup.getByText(/Cleanup lease held until/)).toBeVisible();
  await expect(leasedCleanup.getByRole("button", { name: "Retry cleanup" })).toBeDisabled();
  await page.screenshot({ path: `../output/playwright/import-recovery-leases-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "Next imports", exact: true }).click();
  await expect(page.getByText("Older import", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Next imports", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "Next Calibre handoffs", exact: true }).click();
  await expect(page.getByText("/fixture/older-calibre.epub", { exact: true })).toBeVisible();
  await page.getByText(/Legacy links needing review/).click();
  await page.getByRole("button", { name: "Next legacy links", exact: true }).click();
  await expect(page.getByText("/fixture/older-link.epub", { exact: true })).toBeVisible();
  expect(seen[seen.length - 1].get("operationsCursor")).toBe("older-imports");
  expect(seen[seen.length - 1].get("calibreCursor")).toBe("older-calibre");
  await page.getByRole("button", { name: "Previous imports", exact: true }).click();
  await expect(page.getByText("Leased import", { exact: true })).toBeVisible();
  await expect(page.getByText("/fixture/older-link.epub", { exact: true })).toBeVisible();
  await page.getByRole("checkbox", { name: "Show only unfinished imports and Calibre handoffs" }).check();
  await expect(page.getByText("/fixture/newer-calibre.epub", { exact: true })).toBeVisible();
  expect(seen[seen.length - 1].get("unfinishedOnly")).toBe("true");
  expect(seen[seen.length - 1].has("issuesCursor")).toBe(false);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/import-recovery-paging-${testInfo.project.name}.png`, fullPage: true });
});
