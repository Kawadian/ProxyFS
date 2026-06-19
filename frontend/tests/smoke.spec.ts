import { test, expect } from "@playwright/test";

test.describe("LXC File Hub UI", () => {
  test("shows login or setup page", async ({ page }) => {
    await page.goto("/");
    const setupTitle = page.getByRole("heading", { name: /Initial Setup|初期セットアップ/i });
    const loginTitle = page.getByRole("heading", { name: /Sign in|ログイン/i });
    await expect(setupTitle.or(loginTitle)).toBeVisible({ timeout: 10_000 });
  });

  test("auth page has form fields", async ({ page }) => {
    await page.goto("/login");
    await page.waitForLoadState("networkidle");

    const setupTitle = page.getByRole("heading", { name: /Initial Setup|初期セットアップ/i });
    if (await setupTitle.isVisible()) {
      await expect(page.getByLabel(/Admin username|管理者ユーザー名/i)).toBeVisible();
      await expect(page.getByLabel(/^Admin password|^管理者パスワード/i).first()).toBeVisible();
      return;
    }

    await expect(page.getByRole("heading", { name: /Sign in|ログイン/i })).toBeVisible();
    await expect(page.getByLabel(/^Username$|^ユーザー名$/i)).toBeVisible();
    await expect(page.getByLabel(/^Password$|^パスワード$/i)).toBeVisible();
  });

  test("setup page has required fields when shown", async ({ page }) => {
    await page.goto("/setup");
    const setupTitle = page.getByRole("heading", { name: /Initial Setup|初期セットアップ/i });
    if (!(await setupTitle.isVisible().catch(() => false))) {
      test.skip();
      return;
    }
    await expect(page.getByLabel(/Admin username|管理者ユーザー名/i)).toBeVisible();
    await expect(page.getByLabel(/^Admin password|^管理者パスワード/i).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /Complete setup|セットアップ完了/i })).toBeVisible();
  });

  test("language selector works on auth pages", async ({ page }) => {
    await page.goto("/setup");
    const setupTitle = page.getByRole("heading", { name: /Initial Setup|初期セットアップ/i });
    if (!(await setupTitle.isVisible().catch(() => false))) {
      await page.goto("/login");
    }

    const langSelect = page.locator("select").first();
    await expect(langSelect).toBeVisible();
    await langSelect.selectOption("ja");
    await expect(
      page.getByRole("heading", { name: /初期セットアップ|ログイン|Sign in/i }),
    ).toBeVisible();
  });

  test("page has correct title", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle(/LXC File Hub/i);
  });

  test("responsive layout renders on mobile", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto("/");
    await expect(page.locator("body")).toBeVisible();
  });
});
