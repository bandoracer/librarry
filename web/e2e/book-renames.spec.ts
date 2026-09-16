import { expect, test } from "@playwright/test";

const book = "00000000-0000-0000-0000-000000000090";
const files = Array.from({ length: 1501 }, (_, index) => ({ relativePath: `Disc ${Math.floor(index / 100) + 1}/${String(index + 1).padStart(4, "0")}.mp3`, sourcePath: `/old/${index + 1}.mp3`, destinationPath: `/new/${index + 1}.mp3`, sizeBytes: 12345, format: "audiobook" }));
files.push({ relativePath: "cover.jpg", sourcePath: "/old/cover.jpg", destinationPath: "/new/cover.jpg", sizeBytes: 4000, format: "sidecar" });
const plan = { wantedId: book, title: "Long chapter book", revision: "complete-set-proof", sourceFolder: "/books/Recorded author/Old book folder", destinationFolder: "/books/Recorded author/Corrected book folder", mediaFiles: 1501, companionFiles: 1, noop: false, files };
async function mockBook(page: import("@playwright/test").Page) {
 await page.route(`**/api/v1/wanted/${book}`, route => route.fulfill({ json: { id: book, title: plan.title, authorName: "Recorded author", format: "audiobook", status: "imported", monitored: false, qualityProfile: "standard", derivedState: "downloaded" } }));
}

test("Book folder preview covers every chapter and applies the complete reviewed set", async ({ page }, testInfo) => {
 await mockBook(page);
 const applied: unknown[] = []; let moved = false;
 await page.route(`**/api/v1/library/books/${book}/rename/preview`, route => route.fulfill({ json: { ...plan, noop: moved } }));
 await page.route(`**/api/v1/library/books/${book}/rename`, route => { applied.push(route.request().postDataJSON()); moved = true; return route.fulfill({ json: { imported: true, moved: true, operationId: "book-rename" } }); });
 await page.goto(`/library/book/${book}`);
 await page.getByRole("button", { name: "Rename book folder", exact: true }).click();
 const dialog = page.getByRole("dialog", { name: "Rename book folder", exact: true });
 await expect(dialog.getByText(/1501 book files · 1 companion file/)).toBeVisible();
 await expect(dialog.getByText(/including files on other preview pages/)).toBeVisible();
 await expect(dialog.getByText(plan.sourceFolder, { exact: true })).toBeVisible();
 await expect(dialog.getByText(plan.destinationFolder, { exact: true })).toBeVisible();
 for (let i = 0; i < 15; i++) await dialog.getByRole("button", { name: "Next files", exact: true }).click();
 await expect(dialog.getByText("Disc 16/1501.mp3", { exact: true })).toBeVisible();
 await expect(dialog.getByText("cover.jpg", { exact: true })).toBeVisible();
 await expect(dialog.getByRole("button", { name: "Next files", exact: true })).toBeDisabled();
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 expect(await dialog.evaluate(element => { const box = element.getBoundingClientRect(); return box.left >= 0 && box.right <= innerWidth && box.top >= 0 && box.bottom <= innerHeight; })).toBe(true);
 expect(await dialog.locator(".modal-foot").evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
 await page.screenshot({ path: `../output/playwright/book-rename-${testInfo.project.name}.png`, fullPage: true });
 await dialog.getByRole("button", { name: "Rename complete book (1502 files)", exact: true }).click();
 await expect(dialog).toHaveCount(0);
 expect(applied).toEqual([{ revision: "complete-set-proof" }]);
});

test("A failed book rename exposes its saved plan and resumes the same destination", async ({ page }) => {
 await mockBook(page);
 let attempts = 0; const revisions: string[] = [];
 await page.route(`**/api/v1/library/books/${book}/rename/preview`, route => route.fulfill({ json: { ...plan, revision: attempts ? "saved-set-proof" : plan.revision, operationId: attempts ? "saved-set" : undefined } }));
 await page.route(`**/api/v1/library/books/${book}/rename`, route => {
  attempts++; revisions.push(route.request().postDataJSON().revision);
  return attempts === 1 ? route.fulfill({ status: 409, json: { error: "Controlled commit failure; originals retained" } }) : route.fulfill({ json: { imported: true, moved: true, operationId: "saved-set" } });
 });
 await page.goto(`/library/book/${book}`);
 await page.getByRole("button", { name: "Rename book folder", exact: true }).click();
 const dialog = page.getByRole("dialog");
 await dialog.getByRole("button", { name: "Rename complete book (1502 files)", exact: true }).click();
 await expect(dialog.getByText(/Controlled commit failure/)).toBeVisible();
 await expect(dialog.getByRole("link", { name: "Open recovery", exact: true })).toHaveAttribute("href", "/imports");
 await expect(dialog.getByText(/Resume the previously saved folder plan/)).toBeVisible();
 await expect(dialog.getByText(plan.destinationFolder, { exact: true })).toBeVisible();
 await dialog.getByRole("button", { name: "Resume complete book rename", exact: true }).click();
 await expect(dialog).toHaveCount(0);
 expect(revisions).toEqual([plan.revision, "saved-set-proof"]);
});

test("Unproven book folders explain the blocker without offering Apply", async ({ page }) => {
 await mockBook(page);
 await page.route(`**/api/v1/library/books/${book}/rename/preview`, route => route.fulfill({ status: 409, json: { error: "Book folder contains an unrecorded file: private-notes.txt" } }));
 await page.goto(`/library/book/${book}`);
 await page.getByRole("button", { name: "Rename book folder", exact: true }).click();
 const dialog = page.getByRole("dialog");
 await expect(dialog.getByText(/unrecorded file: private-notes.txt/)).toBeVisible();
 await expect(dialog.getByRole("button", { name: "Rename complete book (0 files)", exact: true })).toBeDisabled();
 await expect(dialog.getByRole("button", { name: "Refresh folder preview", exact: true })).toBeEnabled();
});
