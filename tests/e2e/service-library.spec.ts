import { test, expect } from "@playwright/test";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const account = { email: "owner@example.test", password: "shipper-browser-test-password" };

test.use({ viewport: { width: 1440, height: 900 } });

test("deploys a service straight from the library", async ({ page }) => {
  await page.goto("/");
  if (await page.getByLabel("Setup token").count()) {
    await page.getByLabel("Setup token").fill(token);
    await page.getByLabel("Email", { exact: true }).fill(account.email);
    await page.getByLabel("Password", { exact: true }).fill(account.password);
    await page.getByRole("button", { name: "Create administrator", exact: true }).click();
  } else {
    await page.getByLabel("Email", { exact: true }).fill(account.email);
    await page.getByLabel("Password", { exact: true }).fill(account.password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
  }
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();

  await page.goto("/new");
  await expect(page.getByRole("heading", { name: "Deploy an application", exact: true })).toBeVisible();
  await page.getByRole("button", { name: /Service Library/ }).first().click();
  const dialog = page.getByRole("dialog", { name: "Service Library", exact: true });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("365 services", { exact: true })).toBeVisible();
  await dialog.getByLabel("Search services", { exact: true }).fill("n8n");
  await dialog.locator(".library-tile").filter({ hasText: "n8n" }).first().click();
  await expect(dialog.locator(".library-service-title").getByText("n8n", { exact: true })).toBeVisible();
  await expect(dialog.getByText("2 services", { exact: true }).first()).toBeVisible();

  // The library form already covers name, domains and settings, so Deploy skips the second review.
  await expect(dialog.getByText("Advanced", { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Deploy", exact: true }).click();
  await expect(page).toHaveURL(/\/applications\/[^/?]+\?tab=deployments/);
  await expect(page.getByRole("heading", { name: "n8n", exact: true })).toBeVisible();

  await page.goto("/new");
  await page.getByRole("button", { name: "Configure manually", exact: true }).click();
  const source = page.getByRole("group", { name: "Source type", exact: true });
  await expect(source.getByText("Service Library", { exact: true })).toBeVisible();
  await source.getByRole("radio", { name: "Service Library", exact: true }).check({ force: true });
  await expect(page.getByRole("dialog", { name: "Service Library", exact: true })).toBeVisible();
});
