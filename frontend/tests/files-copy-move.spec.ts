import { test, expect } from "@playwright/test";

const NODE_ID = "test-node-1";
const files = [
  {
    name: "readme.txt",
    path: "readme.txt",
    is_dir: false,
    size: 42,
    mode: "-rw-r--r--",
    mod_time: "2026-06-19T00:00:00Z",
  },
  {
    name: "docs",
    path: "docs",
    is_dir: true,
    size: 0,
    mode: "drwxr-xr-x",
    mod_time: "2026-06-19T00:00:00Z",
  },
];

test.describe("Files copy/move clipboard", () => {
  test.beforeEach(async ({ page }) => {
    await page.route("**/api/v1/nodes", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: NODE_ID,
            name: "Test Node",
            slug: "test-node",
            host: "localhost",
            port: 22,
            provider: "local",
            enabled: true,
            host_key_status: "unknown",
            created_at: "2026-06-19T00:00:00Z",
            updated_at: "2026-06-19T00:00:00Z",
          },
        ]),
      });
    });

    await page.route("**/api/v1/fs/list**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(files),
      });
    });

    await page.route("**/api/v1/auth/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "u1",
            username: "admin",
            display_name: "Admin",
            role: "admin",
            enabled: true,
            created_at: "2026-06-19T00:00:00Z",
            updated_at: "2026-06-19T00:00:00Z",
          },
          csrf_token: "test-csrf",
        }),
      });
    });

    await page.route("**/api/v1/auth/setup", async (route) => {
      await route.fulfill({ status: 409, contentType: "application/json", body: "{}" });
    });
  });

  async function openFile(page: import("@playwright/test").Page) {
    await page.goto("/files");
    await page.getByText("Test Node").click();
    await page.getByText("readme.txt").click();
  }

  test("toolbar copy button shows clipboard banner at top", async ({ page }) => {
    await openFile(page);
    await page.getByRole("button", { name: "Copy", exact: true }).first().click();
    const banner = page.locator(".clipboard-banner");
    await expect(banner).toBeVisible();
    await expect(banner).toContainText("Copy: readme.txt");
    await expect(banner.getByRole("button", { name: /paste here|ここに貼り付け/i })).toBeVisible();
  });

  test("toolbar move button shows clipboard banner", async ({ page }) => {
    await openFile(page);
    await page.getByRole("button", { name: "Move", exact: true }).first().click();
    const banner = page.locator(".clipboard-banner");
    await expect(banner).toBeVisible();
    await expect(banner).toContainText("Move: readme.txt");
  });

  test("row copy icon shows clipboard banner", async ({ page }) => {
    await openFile(page);
    await page.getByRole("button", { name: "Copy", exact: true }).last().click();
    await expect(page.locator(".clipboard-banner")).toContainText("Copy: readme.txt");
  });
});
