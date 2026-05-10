import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:3000"
  },
  webServer: process.env.PLAYWRIGHT_WEB_SERVER
    ? {
        command: process.env.PLAYWRIGHT_WEB_SERVER,
        url: process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:3000",
        reuseExistingServer: true
      }
    : undefined
});
