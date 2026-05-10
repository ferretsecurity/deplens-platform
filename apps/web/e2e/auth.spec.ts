import { expect, test } from "@playwright/test";

test("login page renders and anonymous app routes redirect", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: /sign in to deplens/i })).toBeVisible();

  await page.goto("/app/repositories");
  await expect(page).toHaveURL(/\/login$/);
});
