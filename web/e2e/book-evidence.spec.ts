import { expect, test } from "@playwright/test";

test("book status explains partial audiobook loss and client outages", async ({ page }, testInfo) => {
 const id = "00000000-0000-0000-0000-000000000023";
 let state = "incomplete";
 await page.route(`**/api/v1/wanted/${id}`, route => route.fulfill({ json: {
  id, title: "Evidence fixture", authorName: "Fixture Author", format: "audiobook", status: "imported", monitored: true, qualityProfile: "standard", derivedState: state,
  stateEvidence: { files: { state: state === "unknown" ? "missing" : state, reason: "Required audiobook media is missing.", presentFiles: state === "unknown" ? 0 : 1, requiredFiles: 2 }, downloads: state === "unknown" ? "unavailable" : "fresh", quality: "available", message: state === "unknown" ? "Download-client evidence is unavailable or incomplete." : "Required audiobook media is missing." }
 } }));
 await page.goto(`/library/book/${id}`);
 await expect(page.getByText("Incomplete", { exact: true })).toBeVisible();
 await expect(page.getByRole("status").filter({ hasText: "Required audiobook media is missing." })).toBeVisible();
 await expect(page.getByText("Downloaded", { exact: true })).toHaveCount(0);
 state = "unknown";
 await page.reload();
 await expect(page.getByText("Unknown", { exact: true })).toBeVisible();
 await expect(page.getByRole("status").filter({ hasText: "Download-client evidence is unavailable or incomplete." })).toBeVisible();
 await expect(page.getByText("Missing", { exact: true })).toHaveCount(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 await page.screenshot({ path: `../output/playwright/book-evidence-${testInfo.project.name}.png`, fullPage: true });
});
