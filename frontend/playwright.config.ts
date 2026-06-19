import { defineConfig, devices } from "@playwright/test";

const fullStack = !!process.env.E2E_FULL;
const hubURL = process.env.PLAYWRIGHT_BASE_URL ?? (fullStack ? "http://127.0.0.1:18080" : "http://127.0.0.1:4173");

export default defineConfig({
  testDir: "./tests",
  fullyParallel: !fullStack,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: fullStack ? 1 : process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: hubURL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: fullStack
    ? [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }]
    : [
        { name: "chromium", use: { ...devices["Desktop Chrome"] } },
        { name: "mobile", use: { ...devices["iPhone 13"] } },
      ],
  webServer: fullStack
    ? {
        command: "bash ../scripts/e2e-hub.sh",
        url: `${hubURL}/health/live`,
        reuseExistingServer: false,
        timeout: 120_000,
        stdout: "pipe",
        stderr: "pipe",
      }
    : {
        command: "npm run preview -- --host 127.0.0.1 --port 4173",
        url: "http://127.0.0.1:4173",
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
});
