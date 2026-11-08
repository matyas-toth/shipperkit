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

test("live application discloses health details and exports the selected failed deployment", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.addInitScript(() => Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: async (text: string) => { (window as any).diagnosticContext = text; } } }));
  await login(page);
  const original = await (await page.request.get("/api/applications/application-web")).json();
  const now = Math.floor(Date.now() / 1000);
  const failed = { ...original.activeDeployment, id: "failed-attempt", state: "failed", error: "Failed to collect page data for /api/auth/register", finishedAt: now, startedAt: now - 229, createdAt: now - 230, resolvedMethod: "railpack" };
  let latestFailed = true;
  await page.route("**/api/applications/application-web", r => r.fulfill({ json: { ...original, application: { ...original.application, state: "running", desiredState: "running", latestDeployment: latestFailed ? failed : original.activeDeployment }, deployments: [failed, ...original.deployments] } }));
  await page.route("**/api/applications/application-web/diagnostics", r => r.fulfill({ json: {
    servingRelease: original.activeDeployment, latestOperation: latestFailed ? failed : original.activeDeployment, failedStage: "building", durationSeconds: 230,
    checks: [{ layer: "runtime", scope: "current", state: "healthy", observedAt: now, summary: "Container running" }, { layer: "build", scope: "deployment", state: "failed", observedAt: now, summary: "Latest build failed" }, { layer: "readiness", scope: "deployment", state: "healthy", observedAt: now - 86400 * 5, summary: "Recorded readiness" }, { layer: "route", state: "healthy", observedAt: now - 86400, summary: "website.localhost" }],
    observations: [], changesSinceServing: [], unknowns: [],
  } }));
  const context = { schemaVersion: "1", deployment: { id: failed.id, status: "failed", stage: "building" }, logs: [{ line: "visible output" }, { line: "full unfiltered output" }] };
  await page.route("**/deployments/failed-attempt/context", r => r.fulfill({ json: context }));
  await page.route("**/deployments/failed-attempt/logs?**", r => r.fulfill({ json: { items: [] } }));
  await page.goto("/applications/application-web");
  await expect(page.locator(".application-title")).toContainText("Live");
  await expect(page.getByRole("region", { name: "Application issue" })).toContainText("Latest deployment failed");
  await expect(page.getByRole("table")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Recheck", exact: true })).toHaveCount(0);
  await page.screenshot({ path: join(tmpdir(), "shipper-panel-live-failed-deploy.png") });
  await page.getByText("Health details", { exact: true }).click();
  await expect(page.getByRole("heading", { name: "Current checks" })).toBeVisible();
  await expect(page.getByRole("row").filter({ hasText: "Routing" })).toContainText("Out of date");
  await expect(page.getByRole("row").filter({ hasText: "Readiness" })).toContainText("Passed at deployment");
  await page.getByRole("button", { name: "Recheck", exact: true }).click();
  await expect(page.locator(".health-disclosure")).toHaveAttribute("open", "");
  await page.getByRole("button", { name: "View deployment", exact: true }).click();
  await page.getByRole("button", { name: "Copy diagnostic context", exact: true }).click();
  await expect(page.getByRole("button", { name: "Context copied", exact: true })).toBeVisible();
  expect(JSON.parse(await page.evaluate(() => (window as any).diagnosticContext))).toEqual(context);
  await page.screenshot({ path: join(tmpdir(), "shipper-panel-context.png") });
  latestFailed = false;
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  await page.reload();
  await expect(page.locator(".application-title")).toContainText("Live");
  await expect(page.getByRole("region", { name: "Application issue" })).toHaveCount(0);
  await expect(page.getByRole("table")).toHaveCount(0);
  await page.screenshot({ path: join(tmpdir(), "shipper-panel-healthy.png") });
});

test("source inspection proposes Compose and inspection failure keeps the source editable", async ({ page }) => {
  await login(page);
  await page.route("**/api/connections", r => r.fulfill({ json: { items: [] } }));
  let fail = true;
  await page.route("**/api/repositories/inspect", r => fail ? r.fulfill({ status: 422, json: { error: { message: "Repository is unavailable" } } }) : r.fulfill({ json: { method: r.request().postDataJSON().config.repository.includes("plain-html") ? "static" : "compose", composeFile: "docker-compose.yml", commit: "a".repeat(40), variables: [] } }));
  await page.route("**/api/compose/preview", r => {
    expect(r.request().postDataJSON().config.composeFile).toBe("docker-compose.yml");
    return r.fulfill({ json: { services: [{ name: "web", image: "nginx:alpine", port: 80, volumes: [] }], composeFile: "docker-compose.yml", commit: "a".repeat(40) } });
  });
  await page.route("**/api/plan", r => { const input = r.request().postDataJSON(); return r.fulfill({ json: { ...input, applicationId: "new-application", revision: "proposal" } }); });
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
});
