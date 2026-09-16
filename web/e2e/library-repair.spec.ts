import { expect, test } from "@playwright/test";

test("repair preview continues past clean pages and explains evidence without mutations", async ({ page }, testInfo) => {
  let requests = 0;
  let failNext = true;
  await page.route("**/api/v1/library/repair-preview*", route => {
    expect(route.request().method()).toBe("GET");
    requests++;
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    const common = { readOnly: true, generatedAt: "2026-09-16T00:00:00Z", section: "files" };
    if (!cursor) return route.fulfill({ json: { ...common, checked: 100, findings: [], nextCursor: "page-two" } });
    if (failNext) { failNext = false; return route.fulfill({ status: 502, json: { error: "Database temporarily unavailable" } }); }
    return route.fulfill({ json: { ...common, checked: 1, findings: [{
      id: "duplicate_content:fixture", kind: "duplicate_content", subjectId: "fixture",
      path: "/library/author-with-a-very-long-name/a-very-long-title-with-different-editions/record-one.epub",
      reason: "Multiple file records have the same recorded SHA-256, size and format.",
      proposedAction: "Compare current bytes and book assignments. Keep intentional copies or hardlinks.",
      evidence: { sha256: "a".repeat(64), recordCount: 2, sampleLimit: 20, records: [
        { id: "file-one", path: "/library/one.epub", presence: "present" },
        { id: "file-two", path: "/library/two.epub", presence: "unknown" }
      ] }
    }] } });
  });
  await page.goto("/imports");
  await expect(page.getByRole("button", { name: "Preview library repairs" })).toBeVisible();
  expect(requests).toBe(0);
  await page.getByRole("button", { name: "Preview library repairs" }).click();
  await expect(page.getByText("100 records checked · 0 findings · More records to check", { exact: true })).toBeVisible();
  await expect(page.getByText("No repair findings in the checked records.", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Continue report" }).click();
  await expect(page.getByText("Database temporarily unavailable", { exact: false })).toBeVisible();
  await page.getByRole("button", { name: "Retry report" }).click();
  await expect(page.getByText("101 records checked · 1 finding · Report complete", { exact: true })).toBeVisible();
  await page.getByText("Duplicate content records", { exact: true }).click();
  await expect(page.getByText("/library/two.epub", { exact: true })).toBeVisible();
  await expect(page.getByText("Recorded presence: unknown", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /Apply repairs|Delete duplicates/ })).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/library-repair-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "Start a fresh report" }).click();
  await expect(page.getByText("100 records checked · 0 findings · More records to check", { exact: true })).toBeVisible();
  await expect(page.getByText("Duplicate content records", { exact: true })).toHaveCount(0);
});
