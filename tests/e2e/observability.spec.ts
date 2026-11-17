import { test, expect } from "@playwright/test";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const account = { email: "owner@example.test", password: "shipper-browser-test-password" };

async function enter(page: import("@playwright/test").Page) {
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
}

test("system capacity and deployment diagnosis", async ({ page, request }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await enter(page);
  await request.post("http://127.0.0.1:9083/metrics");
  await request.post("http://127.0.0.1:9083/failure");
  await page.waitForTimeout(1200);
  await page.goto("/");
  await expect(page.locator(".system-pulse")).toHaveCount(0);
  await page.goto("/system");
  await expect(page.getByRole("heading", { name: "Capacity", exact: true })).toBeVisible();
  await expect(page.locator(".capacity-charts .metric-chart")).toHaveCount(3);
  await page.goto("/applications/application-web?tab=deployments&deployment=deployment-web-failed");
  await expect(page.getByText("Deployment failed", { exact: true })).toBeVisible();
  await expect(page.locator(".deployment-result")).toContainText("A service started, but its health check never passed.");
  await page.getByRole("button", { name: "Diagnose", exact: true }).click();
  await expect(page.locator(".diagnosis-drawer")).toBeVisible();
  await expect(page.locator(".diagnosis-cause h3")).toHaveText("A service started, but its health check never passed.");
  await expect(page.locator(".diagnosis-checks-table tr").filter({ hasText: "Runtime" })).toContainText("Failed");
  await expect(page.locator(".diagnosis-log-title").filter({ hasText: "garage" })).toBeVisible();
  await expect(page.locator(".diagnosis-log").first()).toContainText("ERROR failed to read /etc/garage.toml");
  await expect(page.locator(".diagnosis-service").filter({ hasText: "garage" })).toContainText("Unhealthy");
  await expect(page.getByRole("button", { name: "Copy for agent", exact: true })).toBeVisible();
});
