import { expect, test } from "@playwright/test";

test("author additions keep selected format, destination and profile; refresh uses saved settings", async ({ page }, testInfo) => {
  const authors: Record<string, unknown>[] = [];
  const additions: Record<string, unknown>[] = [];
  const refreshes: Record<string, unknown>[] = [];
  const roots = [
    { id: "ebook-root", name: "Ebooks", path: "/ebooks", mediaFormat: "ebook", isDefault: true },
    { id: "audio-root", name: "Audio", path: "/audio", mediaFormat: "audiobook", isDefault: true },
    { id: "chosen-root", name: "Chosen audio", path: "/chosen-audio", mediaFormat: "audiobook" }
  ];
  await page.route("**/api/v1/library/root-folders", route => route.fulfill({ json: { rootFolders: roots } }));
  await page.route("**/api/v1/quality-profiles", route => route.fulfill({ json: { profiles: [
    { name: "standard", mediaFormat: "any" }, { name: "audio-premium", mediaFormat: "audiobook" }
  ] } }));
  await page.route("**/api/v1/authors?**", route => route.fulfill({ json: { authors } }));
  await page.route("**/api/v1/authors", route => {
    const body = route.request().postDataJSON();
    additions.push(body);
    const subscription = { ...body, id: "fixture-author", provider: "Hardcover", providerKey: "hardcover-author:7", authorName: "Fixture author", status: "monitored" };
    authors.push(subscription);
    return route.fulfill({ json: subscription });
  });
  await page.route("**/api/v1/authors/monitor", route => {
    refreshes.push(route.request().postDataJSON());
    if (refreshes.length === 1) return route.fulfill({ status: 503, json: { error: "fixture outage" } });
    return route.fulfill({ json: { status: "completed", authorsChecked: 1, itemsFound: 0, wantedCreated: 0, errorCount: 0, items: [] } });
  });
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [{
    provider: "Hardcover", kind: "author", confidence: "high", score: 1, matchedOn: ["author"],
    work: { id: "author-work:7", title: "Fixture author", authors: [{ id: "hardcover-author:7", name: "Fixture author" }] },
    edition: { id: "", format: "any" }
  }] } }));
  await page.goto("/search");
  await page.getByRole("tab", { name: "Author", exact: true }).click();
  await page.getByLabel("Target format", { exact: true }).selectOption("audiobook");
  await page.getByLabel("Author query").fill("Fixture author");
  await page.getByRole("button", { name: "Find author", exact: true }).click();
  await page.locator(".search-result-row").filter({ hasText: "Fixture author" }).click();
  const surface = testInfo.project.name === "mobile" ? page.getByRole("dialog") : page.locator(".search-detail");
  // Mobile displays the same form in a dialog; desktop keeps it beside results.
  await page.getByLabel("Root folder", { exact: true }).filter({ visible: true }).selectOption("chosen-root");
  await page.getByLabel("Quality profile", { exact: true }).filter({ visible: true }).selectOption("audio-premium");
  await page.getByLabel("Tags", { exact: true }).filter({ visible: true }).fill("fixture-tag");
  await page.screenshot({ path: `../output/playwright/author-destination-form-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "Monitor Author", exact: true }).filter({ visible: true }).click();
  await expect(page.getByText("Fixture author is saved, but the refresh failed:", { exact: false })).toBeVisible();
  expect(additions).toHaveLength(1);
  expect(additions[0]).toMatchObject({ format: "audiobook", rootFolderId: "chosen-root", qualityProfile: "audio-premium", tags: ["fixture-tag"] });
  await page.getByRole("button", { name: "Refresh Author", exact: true }).filter({ visible: true }).click();
  await expect.poll(() => refreshes.length).toBe(2);
  expect(additions).toHaveLength(1);
  expect(refreshes[1]).toMatchObject({ authorIds: ["fixture-author"], force: true });
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  if (testInfo.project.name === "mobile") await expect(surface).toBeVisible();
  await page.screenshot({ path: `../output/playwright/author-destination-${testInfo.project.name}.png`, fullPage: true });
});

test("author settings can change and clear the destination without rewriting the quality profile", async ({ page }) => {
  let author = { id: "fixture-author", provider: "Hardcover", providerKey: "hardcover-author:7", authorName: "Fixture author", format: "ebook", status: "monitored", missingBookPolicy: "all", monitorNewItems: true, qualityProfile: "custom", rootFolderId: "root-a", tags: [] };
  const updates: Record<string, unknown>[] = [];
  await page.route("**/api/v1/authors?**", route => route.fulfill({ json: { authors: [author] } }));
  await page.route("**/api/v1/authors/fixture-author", route => {
    const body = route.request().postDataJSON();
    updates.push(body);
    author = { ...author, ...body };
    return route.fulfill({ json: author });
  });
  await page.route("**/api/v1/library/root-folders", route => route.fulfill({ json: { rootFolders: [
    { id: "root-a", name: "A", path: "/a", mediaFormat: "ebook" },
    { id: "root-b", name: "B", path: "/b", mediaFormat: "ebook" },
    { id: "audio", name: "Audio", path: "/audio", mediaFormat: "audiobook" }
  ] } }));
  await page.goto("/library/authors");
  for (const root of ["root-b", ""]) {
    await page.getByRole("button", { name: "Edit author settings for Fixture author", exact: true }).click();
    const select = page.getByLabel("Fixture author root folder", { exact: true });
    await expect(select.locator('option[value="audio"]')).toHaveCount(0);
    await select.selectOption(root);
    await page.getByRole("button", { name: "Save author settings", exact: true }).click();
    await expect(select).toHaveCount(0);
    expect(updates.at(-1)).toMatchObject({ rootFolderId: root, qualityProfile: "custom" });
  }
});
