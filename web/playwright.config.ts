import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { outputFolder: "../output/playwright/report", open: "never" }]],
  outputDir: "../output/playwright/results",
  use: { baseURL: "http://127.0.0.1:15173", trace: "retain-on-failure" },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 1000 } } },
    { name: "mobile", use: { ...devices["Desktop Chrome"], viewport: { width: 390, height: 844 } } }
  ],
  webServer: [
    { command: "../scripts/start-test-api.sh", url: "http://127.0.0.1:18182/healthz", reuseExistingServer: false, timeout: 120000 },
    { command: "npm run dev -- --port 15173 --strictPort", url: "http://127.0.0.1:15173", reuseExistingServer: false, env: { LIBRARRY_DEV_API: "http://127.0.0.1:18182" } }
  ]
});
