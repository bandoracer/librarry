import { expect, test } from "@playwright/test";

test("direct author pages distinguish outage, ambiguity and missing records", async ({ page }, testInfo) => {
  let state = "outage";
  await page.route("**/api/v1/library/authors/same%20name?**", route => {
    if (state === "outage") return route.fulfill({ status: 503, json: { error: "Fixture outage" } });
    if (state === "missing") return route.fulfill({ status: 404, json: { error: "Author not found" } });
    return route.fulfill({ json: { author: { id: "same name", name: "Same Name" }, books: [], subscriptions: [], totalBooks: 0, choices: [
      { id: "first-author", name: "Same Name", provider: "Hardcover" }, { id: "second-author", name: "Same Name", provider: "Open Library" }
    ] } });
  });
  await page.route("**/api/v1/library/authors/second-author?**", route => route.fulfill({ json: {
    author: { id: "second-author", name: "Same Name" }, subscriptions: [], totalBooks: 1, choices: [],
    books: [{ id: "book-two", title: "Second person's book", authorName: "Same Name", format: "ebook", qualityProfile: "standard", status: "imported", derivedState: "downloaded", monitored: false }]
  } }));
  await page.goto("/library/author/same%20name");
  await expect(page.getByRole("heading", { name: "Author unavailable", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Author not found", exact: true })).toHaveCount(0);
  state = "choices";
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Choose an author", exact: true })).toBeVisible();
  await page.screenshot({ path: `../output/playwright/author-choices-${testInfo.project.name}.png`, fullPage: true });
  await page.getByRole("link", { name: "Same Name · Open Library", exact: true }).click();
  await expect(page.getByRole("link", { name: "Second person's book", exact: true })).toBeVisible();
  await expect(page.getByText("Downloaded", { exact: true })).toBeVisible();
  state = "missing";
  await page.goto("/library/author/same%20name");
  await expect(page.getByRole("heading", { name: "Author not found", exact: true })).toBeVisible();
});

test("author pages use server pagination and scope bulk search to the visible page", async ({ page }, testInfo) => {
  let collectionReads = 0;
  const searched: string[] = [];
  await page.route("**/api/v1/wanted?**", route => { collectionReads++; return route.fulfill({ json: { wanted: [] } }); });
  await page.route("**/api/v1/authors?**", route => { collectionReads++; return route.fulfill({ json: { authors: [] } }); });
  await page.route("**/api/v1/library/authors/paged-author?**", route => {
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    const offset = cursor === "page-3" ? 200 : cursor === "page-2" ? 100 : 0;
    return route.fulfill({ json: { author: { id: "paged-author", name: "Paged Author" }, choices: [], subscriptions: [], totalBooks: 201,
      nextCursor: offset === 200 ? undefined : offset === 100 ? "page-3" : "page-2",
      books: Array.from({ length: offset === 200 ? 1 : 100 }, (_, index) => ({ id: offset === 200 ? "older-book" : `book-${offset + index}`, title: offset === 200 ? "Older imported book" : `Book ${offset + index}`, authorName: "Paged Author", format: "ebook", qualityProfile: "standard", status: "imported", monitored: true, derivedState: "downloaded" }))
    } });
  });
  await page.route("**/api/v1/wanted/*/search", route => { searched.push(route.request().url().split("/").at(-2)!); return route.fulfill({ json: { releases: [] } }); });
  await page.goto("/library/author/paged-author");
  await expect(page.getByText("100 of 201 tracked books · page 1", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Next page", exact: true }).click();
  await expect(page.getByText("100 of 201 tracked books · page 2", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Next page", exact: true }).click();
  await expect(page.getByRole("link", { name: "Older imported book", exact: true })).toBeVisible();
  await page.screenshot({ path: `../output/playwright/author-page-${testInfo.project.name}.png`, fullPage: true });
  await expect(page.getByRole("button", { name: "Next page", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "Search Page", exact: true }).click();
  await expect.poll(() => searched).toEqual(["older-book"]);
  await page.getByRole("button", { name: "Previous page", exact: true }).click();
  await expect(page.getByRole("link", { name: "Book 100", exact: true })).toBeVisible();
  expect(collectionReads).toBe(0);
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
});
