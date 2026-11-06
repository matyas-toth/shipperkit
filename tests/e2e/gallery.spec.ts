import { test, expect } from "@playwright/test";
import { mkdir, copyFile } from "node:fs/promises";
import { join } from "node:path";
import { installFixtures } from "./gallery-fixtures";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const account = { email: "owner@example.test", password: "shipper-browser-test-password" };
const root = join(process.cwd(), ".shipper", "ui-gallery");
const out = join(root, "round-01");
const references = join(root, "references");
test.use({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });

let index = 0;
async function shot(page: import("@playwright/test").Page, name: string, options: { fullPage?: boolean; scroll?: boolean } = {}) {
  index += 1;
  if (options.scroll !== false) {
    await page.evaluate(() => {
      document.querySelector<HTMLElement>(".page-scroll")?.scrollTo(0, 0);
      window.scrollTo(0, 0);
    });
  }
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(140);
  await page.screenshot({ path: join(out, String(index).padStart(2, "0") + "-" + name + ".png"), fullPage: options.fullPage ?? false, animations: "disabled", caret: "hide" });
}

test("capture product surfaces", async ({ page }) => {
  test.setTimeout(300_000);
  await mkdir(out, { recursive: true });
  await mkdir(references, { recursive: true });
  await page.route("**/api/events", route => route.continue());

  await page.goto("/");
  await expect(page.getByLabel("Setup token")).toBeVisible();
  await shot(page, "auth-setup");

  await page.getByLabel("Setup token").fill(token);
  await page.getByLabel("Email", { exact: true }).fill(account.email);
  await page.getByLabel("Password", { exact: true }).fill(account.password);
  await page.getByRole("button", { name: "Create administrator", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();
  await page.request.post("http://127.0.0.1:9083/seed");
  await installFixtures(page);
  await page.reload();
  await expect(page.getByRole("button", { name: "View Website", exact: true })).toBeVisible();
  await page.mouse.move(0, 0);
  await shot(page, "applications-list");

  await page.getByRole("textbox", { name: "Search applications", exact: true }).fill("at");
  await expect(page.getByRole("button", { name: "View Atlas API", exact: true })).toBeVisible();
  await shot(page, "applications-search");
  await page.getByRole("textbox", { name: "Search applications", exact: true }).fill("zzz");
  await expect(page.getByText("No matching applications", { exact: true })).toBeVisible();
  await shot(page, "applications-empty");
  await page.getByRole("textbox", { name: "Search applications", exact: true }).fill("");

  await page.getByRole("button", { name: "View Website", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Website", exact: true })).toBeVisible();
  await page.waitForTimeout(600);
  await shot(page, "website-overview");
  await page.getByText("Evidence", { exact: true }).click();
  await page.locator(".diagnostics-section").scrollIntoViewIfNeeded();
  await page.waitForTimeout(250);
  await shot(page, "website-diagnostics-expanded", { scroll: false });

  await page.goto("/applications/application-atlas");
  await expect(page.getByRole("heading", { name: "Atlas API", exact: true })).toBeVisible();
  await page.waitForTimeout(600);
  await shot(page, "atlas-overview");
  await page.getByRole("button", { name: "Open service api", exact: true }).click();
  await expect(page.getByRole("heading", { name: "api", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.waitForTimeout(400);
  await shot(page, "atlas-service");

  await page.goto("/applications/application-api");
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: "Inspect api", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await shot(page, "container-drawer");
  await page.keyboard.press("Escape");

  await page.goto("/applications/application-checkout?tab=deployments");
  await expect(page.getByRole("heading", { name: "Checkout", exact: true })).toBeVisible();
  await page.waitForTimeout(600);
  await shot(page, "checkout-failed-deployments");

  await page.goto("/applications/application-web?tab=deployments");
  await page.waitForTimeout(500);
  await shot(page, "website-deployments");

  await page.goto("/applications/application-web?tab=deployments&deployment=deployment-web-3");
  await page.waitForTimeout(700);
  await shot(page, "deployment-failed");

  await page.goto("/applications/application-web?tab=logs");
  await page.waitForTimeout(1800);
  await shot(page, "website-logs");

  await page.goto("/applications/application-web?tab=domains");
  await page.waitForTimeout(500);
  await shot(page, "website-domains");
  await page.getByRole("button", { name: "Edit website.localhost", exact: true }).click();
  await page.waitForTimeout(300);
  await shot(page, "domain-editor");

  await page.goto("/applications/application-web?tab=env");
  await page.waitForTimeout(500);
  await shot(page, "website-env", { fullPage: true });
  await page.getByRole("button", { name: "Options for DATABASE_URL", exact: true }).click();
  await page.waitForTimeout(250);
  await shot(page, "env-options");
  await page.keyboard.press("Escape");

  await page.goto("/applications/application-web?tab=tasks");
  await page.waitForTimeout(700);
  await shot(page, "website-tasks");
  await page.getByRole("button", { name: "New task", exact: true }).click();
  await page.waitForTimeout(300);
  await shot(page, "task-editor", { fullPage: true });
  await page.getByRole("button", { name: "Close task", exact: true }).click();

  await page.goto("/applications/application-web?tab=backups");
  await page.waitForTimeout(800);
  await shot(page, "website-backups");
  await page.setViewportSize({ width: 1440, height: 1120 });
  await page.getByRole("button", { name: "Inspect backup run-4", exact: true }).click();
  await page.locator(".backup-detail").scrollIntoViewIfNeeded();
  await page.waitForTimeout(400);
  await shot(page, "backup-detail", { scroll: false });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByRole("button", { name: "Close backup details", exact: true }).click();
  await page.getByRole("button", { name: "New backup", exact: true }).click();
  await page.waitForTimeout(400);
  await shot(page, "backup-editor");

  await page.goto("/applications/application-web?tab=settings");
  await page.waitForTimeout(500);
  await shot(page, "app-settings-general");
  await page.getByRole("button", { name: "Source and build", exact: true }).click();
  await page.waitForTimeout(300);
  await shot(page, "app-settings-source", { fullPage: true });
  await page.getByRole("button", { name: "Runtime", exact: true }).click();
  await page.waitForTimeout(300);
  await shot(page, "app-settings-runtime");
  await page.getByRole("button", { name: "Storage", exact: true }).click();
  await page.waitForTimeout(300);
  await shot(page, "app-settings-storage");

  await page.goto("/applications/application-web?tab=env");
  await page.waitForTimeout(500);
  await page.getByLabel("Value for PUBLIC_ORIGIN", { exact: true }).fill("https://website.localhost/hungarian");
  await page.getByRole("button", { name: "Review changes", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "Review changes" })).toBeVisible();
  await shot(page, "change-review");
  await page.keyboard.press("Escape");

  await page.goto("/applications/application-web?tab=settings");
  await page.waitForTimeout(300);
  await page.getByText("Danger zone", { exact: true }).click();
  await page.getByRole("button", { name: "Delete application", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByLabel("Application name", { exact: true }).fill("Website");
  await page.waitForTimeout(200);
  await shot(page, "delete-dialog");
  await page.keyboard.press("Escape");

  await page.goto("/new");
  await expect(page.getByRole("heading", { name: "New application", exact: true })).toBeVisible();
  await shot(page, "new-source");
  await page.getByLabel("Name", { exact: true }).fill("Portfolio");
  await page.getByRole("textbox", { name: "Repository URL", exact: true }).fill("https://github.com/acme/portfolio.git");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(page.getByText("Railpack", { exact: false }).first()).toBeVisible();
  await page.waitForTimeout(500);
  await shot(page, "new-methods");
  await page.getByText("Configuration", { exact: true }).click();
  await page.waitForTimeout(300);
  await shot(page, "new-config", { fullPage: true });
  await page.getByText("Configuration", { exact: true }).click();
  await page.locator("details.configuration").filter({ hasText: ".env" }).last().evaluate(element => { (element as HTMLDetailsElement).open = true; });
  const detectedVariables = page.getByRole("button", { name: "Add detected variables", exact: true });
  if (await detectedVariables.count()) await detectedVariables.click();
  await page.locator("details.configuration").filter({ hasText: ".env" }).last().evaluate(element => element.scrollIntoView({ block: "start" }));
  await page.waitForTimeout(300);
  await shot(page, "new-variables", { scroll: false });
  await page.getByRole("button", { name: "Review deployment", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Review deployment", exact: true })).toBeVisible();
  await page.waitForTimeout(300);
  await shot(page, "new-review", { fullPage: true });

  await page.goto("/system");
  await page.waitForTimeout(900);
  await shot(page, "system-overview");
  await page.goto("/system#maintenance");
  await page.waitForTimeout(700);
  await shot(page, "system-maintenance");
  await page.goto("/system#notifications");
  await page.waitForTimeout(700);
  await shot(page, "system-notifications");
  await page.goto("/system#tokens");
  await page.waitForTimeout(700);
  await shot(page, "system-tokens");

  await page.goto("/settings");
  await page.waitForTimeout(900);
  await shot(page, "settings-general", { fullPage: true });
  await page.goto("/settings#sources");
  await page.waitForTimeout(600);
  await shot(page, "settings-connections");
  await page.goto("/settings#recovery");
  await page.waitForTimeout(800);
  await shot(page, "settings-recovery", { fullPage: true });
  await page.goto("/settings#account");
  await page.waitForTimeout(500);
  await shot(page, "settings-account", { fullPage: true });

  await page.getByRole("button", { name: "Account", exact: true }).click();
  await page.waitForTimeout(200);
  await shot(page, "account-menu");
  await page.keyboard.press("Escape");

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.waitForTimeout(500);
  await shot(page, "mobile-applications");
  await page.getByRole("button", { name: "View Website", exact: true }).click();
  await page.waitForTimeout(600);
  await shot(page, "mobile-overview");
  await page.getByRole("button", { name: "Deployments", exact: true }).click();
  await page.waitForTimeout(700);
  await shot(page, "mobile-deployments");

  await copyFile(join(process.cwd(), ".shipper", "ui-gallery", "references-src", "t3code.png"), join(references, "t3code.png")).catch(() => {});
});
