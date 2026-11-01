import { expect, test } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

test("Socket fixture import, live progress, durable completion and viewer inventory", async ({
  page,
  browser
}) => {
  test.skip(
    process.env.SOCKET_BROWSER_FIXTURE !== "1",
    "Requires the disposable Socket browser fixture API."
  );
  test.setTimeout(120_000);
  const screenshots = resolve(process.cwd(), "../../docs/pr/screenshots");
  await mkdir(screenshots, { recursive: true });
  await page.setViewportSize({ width: 1440, height: 1100 });
  await page.goto("/login");
  await page.getByLabel("Email").fill("socket-admin@example.test");
  await page.getByLabel("Password", { exact: true }).fill("fixture-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/app/);
  const reset = await page.request.delete("/api/v1/integrations/socket", {
    headers: { "Content-Type": "application/json" }
  });
  expect(reset.ok()).toBe(true);
  await page.getByRole("link", { name: "Socket integration" }).click();
  await expect(page.getByRole("heading", { name: "Socket integration" })).toBeVisible();
  await page.getByLabel("Socket organization").fill("fixture");
  await page.getByLabel("API token", { exact: true }).fill("disposable-fixture-token");
  await page.getByLabel("Synchronization schedule").selectOption("360");
  await page.getByRole("button", { name: "Save configuration" }).click();
  await expect(page.getByText("Configuration saved.")).toBeVisible();
  await expect(page.getByLabel("API token", { exact: true })).toHaveValue("");
  await page.getByRole("button", { name: "Test connection" }).click();
  await expect(page.getByText(/Connection verified/)).toBeVisible();
  await page.screenshot({ path: resolve(screenshots, "socket-configuration.png"), fullPage: true });
  await page.getByRole("button", { name: "Sync now" }).click();
  await expect(page.getByRole("button", { name: "Sync in progress…" })).toBeDisabled();
  await expect(page.getByText(/4 of 12 repositories processed/)).toBeVisible({ timeout: 15_000 });
  await page.screenshot({ path: resolve(screenshots, "socket-progress.png"), fullPage: true });
  await page.reload();
  await expect(page.getByText("completed", { exact: true })).toBeVisible({ timeout: 60_000 });
  await expect(page.getByText("12 of 12 repositories processed · 0 failed")).toBeVisible();
  await page.screenshot({ path: resolve(screenshots, "socket-completed.png"), fullPage: true });
  await page.getByRole("link", { name: "Repositories", exact: true }).click();
  await page.getByLabel("Search repositories").fill("Fixture");
  await page.getByRole("button", { name: "Search", exact: true }).click();
  // Socket imports are beyond the first 100 alphabetically sorted CLI repositories.
  await page.goto("/app/repositories?q=Fixture+Socket");
  const row = page.getByRole("row").filter({ hasText: "Fixture Socket project 12" });
  await expect(row).toBeVisible();
  await expect(row.getByText("Socket", { exact: true })).toBeVisible();
  await expect(row.getByRole("link", { name: "Open Socket dashboard" })).toHaveAttribute(
    "href",
    /^https:\/\/socket.dev\//
  );
  await page.screenshot({ path: resolve(screenshots, "socket-repositories.png"), fullPage: true });
  await row.getByRole("link", { name: "Manifest files" }).click();
  await expect(page.getByText("Fixture Socket project 12", { exact: true })).toBeVisible();
  await expect(page.getByText("Socket inventory", { exact: true })).toBeVisible();
  await expect(page.getByText("empty/package.json", { exact: true })).toBeVisible();
  await expect(page.getByText("@fixture/unreferenced@1.0.0", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Repository dependency · No source file reference provided by Socket")
  ).toBeVisible();
  await page.screenshot({ path: resolve(screenshots, "socket-inventory.png"), fullPage: true });
  const inventoryURL = page.url();
  await page.goto("/app/repositories?q=CLI+plus");
  const linkedRow = page.getByRole("row").filter({ hasText: "Fixture CLI plus Socket" });
  await expect(linkedRow.getByText("CLI", { exact: true })).toBeVisible();
  await expect(linkedRow.getByText("Socket", { exact: true })).toBeVisible();
  await page.screenshot({ path: resolve(screenshots, "socket-both-sources.png"), fullPage: true });
  await page.goto("/app/dependencies");
  await expect(page.getByText("react@19.1.0")).toBeVisible();
  await expect(page.getByText("fastapi@0.115.0")).toBeVisible();
  await page.screenshot({ path: resolve(screenshots, "socket-dependencies.png"), fullPage: true });
  const viewerContext = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
  const viewer = await viewerContext.newPage();
  await viewer.goto("/login");
  await viewer.getByLabel("Email").fill("socket-viewer@example.test");
  await viewer.getByLabel("Password", { exact: true }).fill("fixture-password");
  await viewer.getByRole("button", { name: "Sign in" }).click();
  await expect(viewer).toHaveURL(/\/app/);
  await expect(viewer.getByRole("link", { name: "Socket integration" })).toHaveCount(0);
  const denied = await viewer.request.get("/api/v1/integrations/socket");
  expect(denied.status()).toBe(403);
  await viewer.goto(inventoryURL);
  await expect(viewer.getByText("Socket inventory", { exact: true })).toBeVisible();
  await expect(viewer.getByText("Manage CLI repository mapping")).toHaveCount(0);
  await viewer.screenshot({ path: resolve(screenshots, "socket-viewer.png"), fullPage: true });
  await viewerContext.close();
});
