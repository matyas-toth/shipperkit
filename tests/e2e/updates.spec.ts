import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";

const account = { email: "owner@example.test", password: "shipper-browser-test-password" };
const image = "ghcr.io/matyas-toth/shipperkit@sha256:" + "a".repeat(64);
const release = "https://github.com/matyas-toth/shipperkit/releases/tag/v1.0.0-rc.9";

async function login(page: import("@playwright/test").Page) {
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();
}

test("update badge, channels, settings and progress dialog", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  const state: Record<string, any> = {
    enabled: true, currentVersion: "1.0.0-rc.9", channel: "main", auto: false,
    available: { version: "1.0.0-rc.10", image, releaseUrl: release, channel: "main", checkedAt: 1794000000 },
    checkedAt: 1794000000, checking: false, error: "",
    status: { state: "idle", message: "", fromVersion: "", toVersion: "", startedAt: 0, finishedAt: 0, requestId: "", log: "" },
  };
  const settings: Record<string, any>[] = [];
  let checks = 0, applies = 0, statusReads = 0;
  await page.route("**/api/updates", route => route.fulfill({ json: state }));
  await page.route("**/api/updates/check", async route => {
    checks++;
    state.checkedAt = 1794000060;
    await route.fulfill({ json: state });
  });
  await page.route("**/api/updates/settings", async route => {
    const body = JSON.parse(route.request().postData() || "{}");
    settings.push(body);
    state.channel = body.channel;
    state.auto = body.auto;
    state.available = { ...state.available, channel: body.channel };
    await route.fulfill({ json: state });
  });
  await page.route("**/api/updates/apply", async route => {
    applies++;
    await route.fulfill({ status: 202, json: { state: "queued", message: "Waiting for the update helper", fromVersion: "1.0.0-rc.9", toVersion: "1.0.0-rc.10", startedAt: 1794000070, finishedAt: 0, requestId: "request-1", log: "" } });
  });
  await page.route("**/api/updates/status", async route => {
    statusReads++;
    if (statusReads === 1) {
      const stale = {
        state: "succeeded", message: "Shipper 1.0.0-rc.9 is ready",
        fromVersion: "1.0.0-rc.8", toVersion: "1.0.0-rc.9", startedAt: 1794000000, finishedAt: 1794000010,
        requestId: "previous-request", log: "",
      };
      state.status = stale;
      await route.fulfill({ json: stale });
      return;
    }
    const done = statusReads > 2;
    const status = {
      state: done ? "succeeded" : "updating", message: done ? "Shipper 1.0.0-rc.10 is ready" : "Taking a checkpoint",
      fromVersion: "1.0.0-rc.9", toVersion: "1.0.0-rc.10", startedAt: 1794000070, finishedAt: done ? 1794000090 : 0,
      requestId: "request-1", log: done ? "service: Shipper 1.0.0-rc.10 is ready" : "update: checking operations and taking a checkpoint",
    };
    state.status = status;
    if (done) { state.currentVersion = "1.0.0-rc.10"; state.available = null; }
    await route.fulfill({ json: status });
  });

  await login(page);
  await expect(page.locator(".nav-update-badge")).toHaveText("Update");
  await page.locator(".primary-navigation").getByRole("link", { name: /Settings/ }).click();
  await expect(page.getByRole("heading", { name: "Account settings", exact: true }).locator("svg")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Server settings", exact: true }).locator("svg")).toHaveCount(0);
  await page.getByRole("navigation", { name: "Settings sections" }).getByRole("link", { name: "Updates", exact: true }).click();

  await expect(page.getByText("Update available", { exact: true })).toBeVisible();
  await expect(page.getByText("Shipper 1.0.0-rc.10 is ready to install. Applications stay online.", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "See what changed", exact: true })).toHaveAttribute("href", release);
  await expect(page.getByRole("radio", { name: /Main/ })).toBeChecked();
  await expect(page.getByText("Can break things or ship bugs. Use it to test upcoming changes.", { exact: true })).toBeVisible();
  await page.screenshot({ path: join(tmpdir(), "shipper-update-available.png") });

  await page.getByRole("radio", { name: /Nightly/ }).check();
  await expect.poll(() => settings.at(-1)?.channel).toBe("nightly");
  await expect(page.getByRole("radio", { name: /Nightly/ })).toBeChecked();
  await page.getByRole("radio", { name: /Main/ }).check();
  await expect.poll(() => settings.at(-1)?.channel).toBe("main");
  await page.getByRole("switch", { name: "Auto update" }).check();
  await expect.poll(() => settings.at(-1)?.auto).toBe(true);
  await expect(page.getByText("Auto update is on. Shipper will install 1.0.0-rc.10 automatically.", { exact: true })).toBeVisible();
  await page.getByRole("switch", { name: "Auto update" }).uncheck();
  await expect.poll(() => settings.at(-1)?.auto).toBe(false);
  await page.getByRole("button", { name: "Check for updates", exact: true }).click();
  await expect.poll(() => checks).toBe(1);
  await expect(page.getByText(/Checked /)).toBeVisible();

  await page.getByRole("button", { name: "Update", exact: true }).click();
  await expect.poll(() => applies).toBe(1);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Updating Shipper");
  await expect(dialog).toContainText("Taking a checkpoint");
  await expect(dialog).toContainText("Shipper is up to date", { timeout: 10_000 });
  await expect(dialog).toContainText("Shipper 1.0.0-rc.10 is ready. Reload to use the new release.");
  await page.screenshot({ path: join(tmpdir(), "shipper-update-success.png") });
  await dialog.getByRole("button", { name: "Close", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.locator(".update-success-banner")).toContainText("Shipper 1.0.0-rc.10 is ready");

  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await page.locator('.mobile-nav-link[href="/settings"]').click();
  await page.getByRole("navigation", { name: "Settings sections" }).getByRole("link", { name: "Updates", exact: true }).click();
  await expect(page.locator(".update-success-banner")).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: join(tmpdir(), "shipper-update-mobile.png"), fullPage: true });
  expect(errors).toEqual([]);
});
