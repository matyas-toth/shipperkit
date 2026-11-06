import { test, expect } from "@playwright/test";
import { installFixtures } from "./gallery-fixtures";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const account = { email: "owner@example.test", password: "shipper-browser-test-password" };

test("debug drawer", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Setup token").fill(token);
  await page.getByLabel("Email", { exact: true }).fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByRole("button", { name: "Create administrator", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();
  await page.request.post("http://127.0.0.1:9083/seed");
  await installFixtures(page);
  await page.goto("/applications/application-api");
  await page.getByRole("button", { name: "Inspect api", exact: true }).click();
  await page.waitForTimeout(500);
  console.log("STYLE", await page.locator(".drawer-scrim").evaluate(el => { const s = getComputedStyle(el); return { backdropFilter: s.backdropFilter, background: s.backgroundColor, zIndex: s.zIndex }; }));
  console.log("ANY", await page.evaluate(() => [...document.querySelectorAll("*")].filter(el => getComputedStyle(el).backdropFilter !== "none").map(el => el.className).slice(0, 10)));
});
