import { test, expect, type Page } from "@playwright/test";

// The live result that exposed the legacy 75%/medium confidence friction.
const result = {
  provider: "Hardcover", kind: "book",
  work: { id: "hardcover:465017", title: "A Brief History of Time", authors: [{ id: "hardcover-author:214466", name: "Stephen Hawking" }] },
  edition: { id: "hardcover-edition:32171966", workId: "hardcover:465017", title: "A Brief History of Time", format: "ebook", language: "English" },
  score: 0.75, confidence: "medium", matchedOn: ["hardcover work and edition records"],
  evidence: ["Provider relevance", "english edition", "ebook verified"]
};
const saved = { id: "time-book", title: result.work.title, authorName: "Stephen Hawking", format: "ebook", status: "wanted", monitored: true, qualityProfile: "standard", tags: [] };

async function setup(page: Page, conflicts: string[] = []) {
  const adds: any[] = [], searches: any[] = [], grabs: any[] = [];
  await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [{ ...result, conflicts }] } }));
  await page.route("**/api/v1/library/book-matches", route => route.fulfill({ json: { matches: route.request().postDataJSON().candidates.map((candidate: { key: string }) => ({ key: candidate.key, total: 0, books: [] })) } }));
  await page.route("**/api/v1/library/root-folders", route => route.fulfill({ json: { rootFolders: [{ id: "ebook-root", name: "Ebooks", path: "/ebooks", mediaFormat: "ebook", isDefault: true }] } }));
  await page.route("**/api/v1/wanted", route => { adds.push(route.request().postDataJSON()); return route.fulfill({ json: saved }); });
  await page.route("**/api/v1/wanted/time-book/search", route => {
    searches.push(route.request().postDataJSON());
    return route.fulfill({ json: { item: saved, releases: [
      { id: "rejected", approved: false, score: 200, title: "Wrong language" },
      { id: "best", approved: true, score: 100, title: "Approved EPUB" },
      { id: "other", approved: true, score: 80, title: "Approved PDF" }
    ] } });
  });
  await page.route("**/api/v1/wanted/time-book/grab", route => { grabs.push(route.request().postDataJSON()); return route.fulfill({ json: { id: "fixture-download", name: saved.title } }); });
  await page.goto("/search?query=A+Brief+History+of+Time");
  await page.locator(".search-result-row").click();
  await expect(page.getByRole("button", { name: "Download ebook", exact: true })).toBeEnabled();
  return { adds, searches, grabs };
}

test("medium title match downloads with one action and keeps advanced options out of the way", async ({ page }, testInfo) => {
  const calls = await setup(page);
  await expect(page.locator(".search-result-row")).not.toContainText("medium");
  await expect(page.getByLabel("Selected metadata evidence")).not.toBeVisible();
  await expect(page.getByLabel("Quality profile", { exact: true })).not.toBeVisible();
  await expect(page.getByRole("button", { name: "Monitor Author", exact: true })).not.toBeVisible();
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  await page.screenshot({ path: `../output/playwright/simple-download-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("button", { name: "Download ebook", exact: true }).click();
  await expect(page).toHaveURL(/\/search\?/);
  await expect(page.locator(".search-result-row")).toContainText("Queued");
  expect(calls.adds).toHaveLength(1);
  expect(calls.adds[0]).toMatchObject({ result: { edition: result.edition }, format: "ebook", qualityProfile: "standard", rootFolderId: "ebook-root", preserveExisting: true });
  expect(calls.searches).toEqual([{ limit: 20, language: "English" }]);
  expect(calls.grabs).toEqual([{ releaseId: "best", paused: false, force: false }]);
});

test("add-only never searches or grabs", async ({ page }) => {
  const calls = await setup(page);
  await page.getByRole("button", { name: "Add Book", exact: true }).click();
  await expect(page).toHaveURL(/\/search\?/);
  await expect(page.locator(".search-result-row")).toContainText("Saved");
  expect(calls.adds).toHaveLength(1);
  expect(calls.searches).toHaveLength(0);
  expect(calls.grabs).toHaveLength(0);
});

for (const outcome of ["empty", "rejected", "search-failure", "grab-failure"] as const) {
  test(`${outcome} keeps the saved book and never forces or retries a download`, async ({ page }) => {
    const calls = await setup(page);
    if (outcome === "grab-failure") {
      await page.route("**/api/v1/wanted/time-book/grab", route => { calls.grabs.push(route.request().postDataJSON()); return route.fulfill({ status: 502, json: { error: "Fixture client unavailable" } }); });
    } else {
      await page.route("**/api/v1/wanted/time-book/search", route => route.fulfill(outcome === "search-failure"
        ? { status: 502, json: { error: "Fixture provider unavailable" } }
        : { json: { item: saved, releases: outcome === "empty" ? [] : [{ id: "rejected", approved: false, title: "Rejected" }] } }));
    }
    await page.getByRole("button", { name: "Download ebook", exact: true }).click();
    await expect(page).toHaveURL(/\/search\?/);
    await expect(page.locator(".search-result-row")).toContainText(outcome.endsWith("failure") ? "Needs attention" : "No download found");
    await page.locator(".search-result-row").click();
    await page.getByRole("button", { name: "Open book", exact: true }).click();
    await expect(page).toHaveURL(/\/library\/book\/time-book$/);
    expect(calls.adds).toHaveLength(1);
    expect(calls.grabs).toHaveLength(outcome === "grab-failure" ? 1 : 0);
    if (outcome.endsWith("failure")) await expect(page.getByText(/couldn't start the download:/)).toBeVisible();
    else await expect(page.getByText(/No suitable download found yet/)).toBeVisible();
  });
}

test("real conflicts require confirmation and preserve the download intent", async ({ page }) => {
  const calls = await setup(page, ["Edition language differs from your preference"]);
  await page.getByRole("button", { name: "Download ebook", exact: true }).click();
  const review = page.getByRole("dialog", { name: "Review before adding" });
  await expect(review).toContainText("Edition language differs from your preference");
  expect(calls.adds).toHaveLength(0);
  await review.getByRole("button", { name: "Download anyway", exact: true }).click();
  await expect(page).toHaveURL(/\/search\?/);
  await expect(page.locator(".search-result-row")).toContainText("Queued");
  expect(calls.adds).toHaveLength(1);
  expect(calls.grabs).toEqual([{ releaseId: "best", paused: false, force: false }]);
});

test("in-flight search disables duplicate actions across the whole sequence", async ({ page }) => {
  const calls = await setup(page);
  let finishSearch!: () => void;
  const waiting = new Promise<void>(resolve => { finishSearch = resolve; });
  await page.route("**/api/v1/wanted/time-book/search", async route => {
    calls.searches.push(route.request().postDataJSON());
    await waiting;
    await route.fulfill({ json: { item: saved, releases: [{ id: "best", approved: true }] } });
  });
  try {
    await page.getByRole("button", { name: "Download ebook", exact: true }).click();
    await expect(page.locator(".search-result-row")).toContainText("Finding download");
    await page.locator(".search-result-row").click();
    await expect(page.getByRole("button", { name: "Finding download", exact: true })).toBeDisabled();
    await expect(page.getByRole("button", { name: "Add Book", exact: true })).toHaveCount(0);
    expect(calls.adds).toHaveLength(1);
    expect(calls.searches).toHaveLength(1);
  } finally { finishSearch(); }
  await expect(page).toHaveURL(/\/search\?/);
  await expect(page.locator(".search-result-row")).toContainText("Queued");
  expect(calls.grabs).toHaveLength(1);
});

for (const secondFails of [false, true]) {
  test(`two books run independently and retain row status (${secondFails ? "one fails" : "both queue"})`, async ({ page }, testInfo) => {
    const calls = await setup(page);
    const second = { ...result, work: { ...result.work, id: "hardcover:2", title: "Another Book" }, edition: { ...result.edition, id: "hardcover-edition:2", workId: "hardcover:2", title: "Another Book", format: "audiobook" } };
    const alternate = { ...result, edition: { ...result.edition, id: "hardcover-edition:3", title: "Alternate edition" } };
    const secondSaved = { ...saved, id: "another-book", title: "Another Book", format: "audiobook" };
    await page.route("**/api/v1/search?**", route => route.fulfill({ json: { results: [result, alternate, second] } }));
    await page.route("**/api/v1/library/root-folders", route => route.fulfill({ json: { rootFolders: [
      { id: "ebook-root", name: "Ebooks", path: "/ebooks", mediaFormat: "ebook", isDefault: true },
      { id: "audio-root", name: "Audio", path: "/audio", mediaFormat: "audiobook", isDefault: true }
    ] } }));
    let finishSaveFirst!: () => void;
    const saveGate = new Promise<void>(resolve => { finishSaveFirst = resolve; });
    await page.route("**/api/v1/wanted", async route => {
      const payload = route.request().postDataJSON(); calls.adds.push(payload);
      if (payload.result.work.id === result.work.id) await saveGate;
      return route.fulfill({ json: payload.result.work.id === result.work.id ? saved : secondSaved });
    });
    let finishFirst!: () => void, finishSecond!: () => void;
    const firstGate = new Promise<void>(resolve => { finishFirst = resolve; });
    const secondGate = new Promise<void>(resolve => { finishSecond = resolve; });
    await page.route("**/api/v1/wanted/time-book/search", async route => {
      calls.searches.push("time-book"); await firstGate;
      await route.fulfill({ json: { item: saved, releases: [{ id: "first-release", approved: true }] } });
    });
    await page.route("**/api/v1/wanted/another-book/search", async route => {
      calls.searches.push("another-book"); await secondGate;
      await route.fulfill(secondFails ? { status: 502, json: { error: "Fixture second search failed" } } : { json: { item: secondSaved, releases: [{ id: "second-release", approved: true }] } });
    });
    await page.route("**/api/v1/wanted/*/grab", route => {
      calls.grabs.push({ url: route.request().url(), ...route.request().postDataJSON() });
      return route.fulfill({ json: { id: "fixture-download" } });
    });
    await page.goto("/search?query=books");
    const firstRow = page.locator(".search-result-row").filter({ hasText: "A Brief History of Time" });
    const secondRow = page.locator(".search-result-row").filter({ hasText: "Another Book" });
    try {
      await firstRow.click();
      await page.getByRole("button", { name: "Download ebook", exact: true }).click();
      await expect(firstRow).toContainText("Adding");
      // The mobile detail closes on submission, so the next row is immediately usable.
      await secondRow.click();
      await expect(page.getByRole("button", { name: "Download audiobook", exact: true })).toBeEnabled();
      await page.getByRole("button", { name: "Download audiobook", exact: true }).click();
      await expect(secondRow).toContainText("Finding download");
      await expect(firstRow).toContainText("Adding");
      finishSaveFirst();
      await expect(firstRow).toContainText("Finding download");
      expect(calls.searches).toEqual(["another-book", "time-book"]);
      expect(calls.adds).toHaveLength(2);
      expect(calls.adds[0]).toMatchObject({ format: "ebook", rootFolderId: "ebook-root", result: { edition: result.edition } });
      expect(calls.adds[1]).toMatchObject({ format: "audiobook", rootFolderId: "audio-root", result: { edition: second.edition } });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.screenshot({ path: `../output/playwright/concurrent-downloads-${testInfo.project.name}.png`, fullPage: true });
      await firstRow.click();
      await page.getByLabel("Edition", { exact: true }).selectOption({ index: 1 });
      await expect(page.getByRole("button", { name: "Finding download", exact: true })).toBeDisabled();
      expect(calls.adds).toHaveLength(2);
      finishSecond();
      await expect(secondRow).toContainText(secondFails ? "Needs attention" : "Queued");
      await expect(page.getByRole("button", { name: "Finding download", exact: true })).toBeDisabled();
      await expect(page).toHaveURL(/\/search\?/);
      finishFirst();
      await expect(firstRow).toContainText("Queued");
      await expect(page.getByRole("button", { name: "Open book", exact: true })).toBeVisible();
      expect(calls.adds).toHaveLength(2); // stale empty book-matches cannot re-enable Add/Download
      expect(calls.grabs.map(g => [g.url.split("/").at(-2), g.releaseId, g.paused, g.force])).toEqual(secondFails
        ? [["time-book", "first-release", false, false]]
        : [["another-book", "second-release", false, false], ["time-book", "first-release", false, false]]);
    } finally { finishSaveFirst(); finishFirst(); finishSecond(); }
  });
}
