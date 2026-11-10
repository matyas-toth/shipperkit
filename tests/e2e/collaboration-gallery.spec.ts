import { test, expect } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { join } from "node:path";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const account = { email: "owner@example.test", password: "shipper-browser-test-password" };
const out = join(process.cwd(), ".shipper", "collaboration-gallery", process.env.SHIPPER_GALLERY_ROUND || "round-01");
test.use({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 });

test("capture collaboration settings", async ({ page }) => {
  await mkdir(out, { recursive: true });
  await page.goto("/");
  await expect(page.getByLabel("Setup token")).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(300);
  await page.screenshot({ path: join(out, "00-admin-setup.png"), animations: "disabled" });
  await page.getByLabel("Setup token").fill(token);
  await page.getByLabel("Email", { exact: true }).fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByRole("button", { name: "Create administrator", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();

  const session = await (await page.request.get("/api/auth/session")).json();
  const action = (path: string, data: unknown) => page.request.post(path, {
    headers: { "X-CSRF-Token": session.csrfToken }, data,
  });
  await action("/api/users", { email: "mara@example.test", role: "developer", password: "temporary-password-123" });
  await action("/api/users", { email: "jonas@example.test", role: "viewer", password: "temporary-password-456" });
  await action("/api/users", { email: "noah@example.test", role: "viewer", password: "" });
  const users = await (await page.request.get("/api/users")).json();
  const jonas = users.items.find((item: { email: string }) => item.email === "jonas@example.test");
  await action("/api/users/" + jonas.id + "/disable", {});
  for (const provider of [
    { id: "google", clientId: "634982157204-example.apps.googleusercontent.com", secret: "google-client-secret" },
    { id: "github", clientId: "Iv1.8f4c2e91a7bd3c60", secret: "github-client-secret" },
    { id: "discord", clientId: "1284569012345678901", secret: "discord-client-secret" },
  ]) {
    await action("/api/auth/providers/" + provider.id, { clientId: provider.clientId, clientSecret: provider.secret, enabled: provider.id !== "discord" });
  }

  await page.goto("/settings#users");
  await expect(page.getByRole("heading", { name: "Users", exact: true })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(out, "01-users.png"), animations: "disabled" });

  await page.getByRole("button", { name: "Add user", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Add user", exact: true })).toBeVisible();
  await page.getByLabel("Email", { exact: true }).fill("sara@example.test");
  await page.waitForTimeout(180);
  await page.screenshot({ path: join(out, "02-add-user.png"), animations: "disabled" });
  const drawer = page.getByRole("dialog");
  await drawer.getByRole("button", { name: "Add user", exact: true }).click();
  await expect(page.getByRole("heading", { name: "User created", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Done", exact: true }).click();
  await expect(page.getByText("sara@example.test", { exact: true })).toBeVisible();

  await page.locator('summary[aria-label="Actions for mara@example.test"]').click();
  await page.getByRole("button", { name: "View activity", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Activity", exact: true })).toBeVisible();
  await page.waitForTimeout(220);
  await page.screenshot({ path: join(out, "03-user-activity.png"), animations: "disabled" });
  await page.getByRole("button", { name: "Close", exact: true }).click();

  await page.setViewportSize({ width: 1440, height: 1500 });
  await page.goto("/settings#authentication");
  await expect(page.getByRole("heading", { name: "Sign-in methods", exact: true })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(out, "04-authentication.png"), animations: "disabled" });
  await page.locator(".provider-card").first().getByText("Setup steps", { exact: true }).click();
  await page.waitForTimeout(180);
  await page.screenshot({ path: join(out, "05-provider-steps.png"), animations: "disabled" });
  const saved = page.waitForResponse(response => response.url().endsWith("/api/auth/providers/google") && response.request().method() === "POST");
  await page.locator(".provider-card").first().getByRole("button", { name: "Save", exact: true }).click();
  expect((await saved).status()).toBe(200);
  await expect(page.locator("#authentication .error-notice")).toHaveCount(0);

  await page.setViewportSize({ width: 1440, height: 1000 });
  await action("/api/auth/logout", {});
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Welcome back aboard.", exact: true })).toBeVisible();
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(out, "06-sign-in.png"), animations: "disabled" });

  await page.getByLabel("Email", { exact: true }).fill("mara@example.test");
  await page.getByLabel("Password", { exact: true }).fill("temporary-password-123");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Make it yours.", exact: true })).toBeVisible();
  await page.waitForTimeout(220);
  await page.screenshot({ path: join(out, "07-password-gate.png"), animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(180);
  await page.screenshot({ path: join(out, "08-password-gate-mobile.png"), animations: "disabled" });
});
