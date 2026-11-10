import { test, expect } from "@playwright/test";

const account = { email: "owner@example.test", password: "shipper-browser-test-password" };

test("user provisioning and provider settings send session CSRF tokens", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();

  await page.goto("/settings#users");
  await page.getByRole("button", { name: "Add user", exact: true }).click();
  await page.getByLabel("Email", { exact: true }).fill("csrf-check@example.test");
  const drawer = page.getByRole("dialog");
  await drawer.getByRole("button", { name: "Add user", exact: true }).click();
  await expect(page.getByRole("heading", { name: "User created", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Done", exact: true }).click();
  await expect(page.getByText("csrf-check@example.test", { exact: true })).toBeVisible();

  await page.goto("/settings#authentication");
  const provider = page.locator(".provider-card").first();
  await expect(provider).toBeVisible();
  const saved = page.waitForResponse(response => response.url().endsWith("/api/auth/providers/google") && response.request().method() === "POST");
  await provider.getByRole("button", { name: "Save", exact: true }).click();
  expect((await saved).status()).toBe(200);
  await expect(page.locator("#authentication .error-notice")).toHaveCount(0);
});
