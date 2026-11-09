import { test, expect } from "@playwright/test";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const account = { email: "owner@example.test", password: "shipper-browser-test-password" };

test("mobile bottom navigation replaces the top tabs and keeps search", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  const setupToken = page.getByLabel("Setup token");
  const email = page.getByLabel("Email", { exact: true });
  await expect(setupToken.or(email)).toBeVisible();
  if (await setupToken.isVisible()) {
    await setupToken.fill(token);
    await email.fill(account.email);
    await page.getByLabel("Password", { exact: true }).fill(account.password);
    await page.getByRole("button", { name: "Create administrator", exact: true }).click();
  } else {
    await email.fill(account.email);
    await page.getByLabel("Password", { exact: true }).fill(account.password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
  }
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();

  const bottom = page.locator(".mobile-bottom-nav");
  const top = page.locator(".topbar .primary-navigation");
  await expect(bottom).toBeVisible();
  await expect(top).toBeHidden();
  await expect(page.locator(".topbar-search")).toBeHidden();
  await expect(bottom.locator(".mobile-nav-link")).toHaveCount(3);
  await expect(page.getByRole("link", { name: "Applications", exact: true })).toHaveCount(1);
  await expect(bottom.locator('.mobile-nav-link[href="/"]')).toHaveAttribute("aria-current", "page");
  expect(await page.locator("body").evaluate(element => element.scrollWidth <= innerWidth)).toBe(true);

  await bottom.locator('.mobile-nav-link[href="/system"]').click();
  await expect(page.getByRole("heading", { name: "System", exact: true })).toBeVisible();
  await expect(bottom.locator('.mobile-nav-link[href="/system"]')).toHaveAttribute("aria-current", "page");
  await expect(bottom.locator('.mobile-nav-link[href="/"]')).not.toHaveAttribute("aria-current", "page");

  await bottom.locator('.mobile-nav-link[href="/settings"]').click();
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeVisible();
  await expect(bottom.locator('.mobile-nav-link[href="/settings"]')).toHaveAttribute("aria-current", "page");

  await bottom.locator(".mobile-nav-search").click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.locator("#application-search")).toBeFocused();

  await page.setViewportSize({ width: 320, height: 844 });
  expect(await page.locator("body").evaluate(element => element.scrollWidth <= innerWidth)).toBe(true);

  await page.setViewportSize({ width: 1024, height: 900 });
  await expect(bottom).toBeHidden();
  await expect(top).toBeVisible();
  await expect(page.locator(".topbar-search")).toBeVisible();
});
