import { test, expect } from "@playwright/test";

const fullStack = !!process.env.E2E_FULL;

test.describe.configure({ mode: fullStack ? "serial" : "parallel" });

test.describe("Initial setup flow", () => {
  test.skip(!fullStack, "requires E2E_FULL=1 with running hub backend");

  test("completes setup and lands on dashboard", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /Initial Setup|初期セットアップ/i })).toBeVisible({
      timeout: 15_000,
    });

    await page.locator("#setup-username").fill("e2e-admin");
    await page.locator("#setup-password").fill("e2e-pass-12345");
    await page.locator("#setup-confirm").fill("e2e-pass-12345");
    await page.getByPlaceholder("LXC File Hub").fill("E2E Hub");
    await page.getByRole("button", { name: /Complete setup|セットアップ完了/i }).click();

    await expect(page.getByRole("heading", { name: /Dashboard|ダッシュボード/i })).toBeVisible({
      timeout: 15_000,
    });
    await expect(page.getByText(/E2E Hub|e2e-admin/i).first()).toBeVisible();
  });

  test("login after setup", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: /Sign in|ログイン/i })).toBeVisible();
    await page.getByLabel(/^Username$|^ユーザー名$/i).fill("e2e-admin");
    await page.getByLabel(/^Password$|^パスワード$/i).fill("e2e-pass-12345");
    await page.getByRole("button", { name: /Sign in|ログイン/i }).click();
    await expect(page.getByRole("heading", { name: /Dashboard|ダッシュボード/i })).toBeVisible({
      timeout: 15_000,
    });
  });

  test("navigate to nodes page", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/^Username$|^ユーザー名$/i).fill("e2e-admin");
    await page.getByLabel(/^Password$|^パスワード$/i).fill("e2e-pass-12345");
    await page.getByRole("button", { name: /Sign in|ログイン/i }).click();
    await expect(page.getByRole("heading", { name: /Dashboard|ダッシュボード/i })).toBeVisible();

    await page.getByRole("link", { name: /Nodes|接続先/i }).click();
    await expect(page.getByRole("heading", { name: /Nodes|接続先/i })).toBeVisible();
  });
});
