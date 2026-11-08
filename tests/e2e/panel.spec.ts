import { test, expect, type Page } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";

async function login(page: Page) {
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
  await page.getByLabel("Password", { exact: true }).fill("shipper-browser-test-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();
}

test("source inspection proposes Compose and inspection failure keeps the source editable", async ({ page }) => {
  await login(page);
  await page.route("**/api/connections", r => r.fulfill({ json: { items: [] } }));
  let fail = true;
  await page.route("**/api/repositories/inspect", r => fail ? r.fulfill({ status: 422, json: { error: { message: "Repository is unavailable" } } }) : r.fulfill({ json: { method: r.request().postDataJSON().config.repository.includes("plain-html") ? "static" : "dockerfile", composeFile: r.request().postDataJSON().config.repository.includes("plain-html") ? undefined : "docker-compose.yml", commit: "a".repeat(40), variables: [] } }));
  await page.route("**/api/compose/preview", r => {
    expect(r.request().postDataJSON().config.composeFile).toBe("docker-compose.yml");
    return r.fulfill({ json: { services: [{ name: "web", image: "nginx:alpine", port: 80, volumes: [] }], composeFile: "docker-compose.yml", commit: "a".repeat(40) } });
  });
  await page.route("**/api/plan", r => { const input = r.request().postDataJSON(); return r.fulfill({ json: { ...input, applicationId: "new-application", revision: "proposal" } }); });
  await page.goto("/new");
  await expect(page.locator(".source-alternatives > :first-child")).toHaveText("Connect GitHub");
  await page.getByRole("link", { name: "Connect GitHub", exact: true }).click();
  await expect(page).toHaveURL(/settings#sources$/);
  await expect(page.getByRole("heading", { name: "Connections", exact: true })).toBeVisible();
  await page.goto("/new");
  await expect(page.getByLabel("Name", { exact: true })).toHaveCount(0);
  await expect(page.getByLabel("Build method", { exact: true })).toHaveCount(0);
  await page.getByLabel("Repository URL", { exact: true }).fill("https://github.com/example/store.git");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Repository is unavailable");
  await expect(page.getByLabel("Repository URL", { exact: true })).toBeEnabled();
  fail = false;
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Ready to deploy", exact: true })).toBeFocused();
  await expect(page.getByRole("region", { name: "Proposed deployment" })).toContainText("Docker Compose");
  await expect(page.getByRole("region", { name: "Proposed deployment" })).toContainText("store");
  await page.screenshot({ path: join(tmpdir(), "shipper-panel-proposal.png") });
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: join(tmpdir(), "shipper-panel-source-mobile.png") });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.getByLabel("Repository URL", { exact: true }).fill("https://github.com/example/plain-html.git");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(page.getByRole("region", { name: "Proposed deployment" })).toContainText("Static site");
  await page.getByRole("button", { name: "Configure manually", exact: true }).click();
  const steps = page.getByRole("list", { name: "Creation steps" });
  await expect(steps.getByRole("listitem")).toHaveCount(3);
  await expect(steps.locator('[aria-current="step"]')).toHaveText("1Source");
  await expect(page.getByRole("radio", { name: "Git repository", exact: true })).toBeChecked();
  await expect(page.getByLabel("Repository URL", { exact: true })).toHaveValue("https://github.com/example/plain-html.git");
  await page.getByLabel("Name", { exact: true }).fill("Manual project");
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(steps.locator('[aria-current="step"]')).toHaveText("2Configure");
  await expect(page.getByLabel("Build method", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Review deployment", exact: true }).click();
  await expect(steps.locator('[aria-current="step"]')).toHaveText("3Review");
  await expect(page.getByRole("heading", { name: "Review deployment", exact: true })).toBeFocused();
  await expect(page.locator(".creation-review")).toContainText("Manual project");
  await page.getByRole("button", { name: "Edit source", exact: true }).click();
  await expect(steps.locator('[aria-current="step"]')).toHaveText("1Source");
  await expect(page.getByLabel("Name", { exact: true })).toHaveValue("Manual project");
  await page.screenshot({ path: join(tmpdir(), "shipper-manual-source-mobile.png") });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
