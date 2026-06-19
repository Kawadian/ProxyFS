import { test, expect } from "@playwright/test";

const ADMIN_USER = "proto-admin";
const ADMIN_PASS = "proto-pass-12345";

async function ensureAdminSession(page: import("@playwright/test").Page) {
  await page.goto("/");
  const setupHeading = page.getByRole("heading", { name: /Initial Setup|初期セットアップ/i });
  if (await setupHeading.isVisible().catch(() => false)) {
    await page.locator("#setup-username").fill(ADMIN_USER);
    await page.locator("#setup-password").fill(ADMIN_PASS);
    await page.locator("#setup-confirm").fill(ADMIN_PASS);
    await page.getByRole("button", { name: /Complete setup|セットアップ完了/i }).click();
  } else {
    await page.goto("/login");
    await page.getByLabel(/^Username$|^ユーザー名$/i).fill(ADMIN_USER);
    await page.getByLabel(/^Password$|^パスワード$/i).fill(ADMIN_PASS);
    await page.getByRole("button", { name: /Sign in|ログイン/i }).click();
  }
  await expect(page.getByRole("heading", { name: /Dashboard|ダッシュボード/i })).toBeVisible({
    timeout: 15_000,
  });
}

test.describe("Protocols sidebar", () => {
  test.skip(!process.env.E2E_FULL && !process.env.PLAYWRIGHT_BASE_URL, "requires running hub backend");

  test("admin sees Protocols link in sidebar", async ({ page }) => {
    await ensureAdminSession(page);

    const protocolsLink = page.getByRole("link", { name: /Protocols|プロトコル/i });
    await expect(protocolsLink).toBeVisible();

    await protocolsLink.click();
    await expect(page.getByRole("heading", { name: /^Protocols$|^プロトコル$/i })).toBeVisible();
    await expect(page.getByText(/SFTP/i).first()).toBeVisible();
  });
});
