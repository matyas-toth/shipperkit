import { test, expect, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { join } from "node:path";
import { installFixtures } from "./gallery-fixtures";

// UI fixtures exercise presentation. Real Docker acceptance is kept separate.
const output = join(process.cwd(), ".shipper", "resource-gallery", process.env.SHIPPER_RESOURCE_ROUND || "round-01");
let number = 0;
async function shot(page: Page, name: string) {
  await expect(page.getByText("Shipper could not display this page", {exact:true})).toHaveCount(0);
  await page.evaluate(() => document.fonts.ready);
  await page.mouse.move(0, 0);
  await page.screenshot({ path: join(output, String(++number).padStart(2, "0") + "-" + name + ".png"), animations: "disabled", fullPage: false });
}
test("capture database networking and terminal surfaces", async ({ page }) => {
  await mkdir(output, { recursive: true });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.getByLabel("Setup token").fill("shipper-browser-test-setup-token-00000000000000000000000000000000");
  await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
  await page.getByLabel("Password", { exact: true }).fill("shipper-browser-test-password");
  await page.getByRole("button", { name: "Create administrator", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();
  await page.request.post("http://127.0.0.1:9083/seed");
  const original = await (await page.request.get("/api/applications/application-web")).json();
  const session = await (await page.request.get("/api/auth/session")).json();
  // Capture the creation form and reviewed proposal for each actual preset.
  const presets: Record<string, any> = {};
  for (const [engine, version] of [["postgresql", "18"], ["mysql", "8.4"], ["mariadb", "11.8"], ["redis", "8"]]) {
    await page.goto("/new");
    await page.getByRole("button", { name: "Database", exact: true }).click();
    await page.getByLabel("Engine", { exact: true }).selectOption(engine);
    await page.getByLabel("Name", { exact: true }).fill(engine === "redis" ? "Job queue" : "Production " + engine);
    await shot(page, "create-" + engine);
    const response = await page.request.post("/api/database-resources/prepare", { headers: { "X-CSRF-Token": session.csrfToken }, data: { engine, version } });
    expect(response.ok()).toBe(true);
    presets[engine] = (await response.json()).config;
    await page.getByRole("button", { name: "Review database", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Ready to deploy", exact: true })).toBeVisible();
    await shot(page, "review-" + engine);
  }
  await installFixtures(page);
  let config = presets.postgresql;
  let name = "Production database";
  const release = () => ({ ...original.activeDeployment, config, resolvedMethod: "image", runtimePort: config.port });
  const application = () => ({ ...original.application, name, sourceKind: "image", method: "image", activeMethod: "image", sourceRef: config.image, config, domains: [], containers: [{ ...original.application.containers[0], name: "production-database", applicationId: "application-web", managed: true, state: "running", health: "healthy", image: config.image }] });
  await page.route("**/api/applications/application-web", r => r.fulfill({ json: { application: application(), activeDeployment: release(), deployments: [release()], database: { engine: config.database.engine, host: "app-application-web", port: config.port } } }));
  await page.route("**/api/applications/application-web/database/connection", r => r.fulfill({ json: { uri: "postgresql://app:gallery-password@app-application-web:5432/app?sslmode=require", deploymentId: release().id } }));
  await page.route("**/api/applications/application-web/diagnostics", r => r.fulfill({ json: { checks: [], observations: [], changesSinceServing: [], unknowns: [], servingRelease: release(), latestOperation: release(), durationSeconds: 14 } }));
  await page.route("**/api/applications/application-web/incidents", r => r.fulfill({ json: { items: [] } }));
  for (const engine of ["postgresql", "mysql", "mariadb", "redis"]) {
    config = presets[engine]; name = engine === "redis" ? "Job queue" : "Production " + engine;
    await page.goto("/applications/application-web");
    await expect(page.getByRole("button", { name: "Import dump", exact: true })).toBeVisible();
    await shot(page, "overview-" + engine);
    await page.getByRole("button", { name: "Import dump", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Import a copy" })).toBeVisible();
    await shot(page, "import-" + engine);
  }
  config = presets.postgresql; name = "Production database";
  await page.goto("/applications/application-web?tab=backups");
  await shot(page, "database-backups");
  await page.goto("/applications/application-web?tab=networking");
  await shot(page, "networking-empty");
  await page.getByRole("button", { name: "Connect application", exact: true }).click();
  await shot(page, "private-connection");
  await page.getByRole("button", { name: "Add port", exact: true }).click();
  await page.getByLabel("Public port", { exact: true }).fill("15432");
  await page.getByLabel("Container port", { exact: true }).fill("5432");
  await page.getByLabel("TLS hostname", { exact: true }).fill("db.example.com");
  await shot(page, "public-database-tls");
  await page.getByRole("button", { name: "Review changes", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await shot(page, "review-network-change");
  await page.getByRole("button", { name: "Keep editing", exact: true }).click();
  await page.getByRole("button", { name: "Add port", exact: true }).click();
  await page.getByLabel("Protocol", { exact: true }).last().selectOption("udp");
  await page.getByLabel("Public port", { exact: true }).last().fill("19000");
  await page.getByLabel("Container port", { exact: true }).last().fill("9000");
  await shot(page, "tcp-udp-mappings");
  await page.route("**/api/applications/application-web/changes", r => r.fulfill({ status: 409, json: { error: { message: "Port 15432/tcp is already in use" } } }));
  await page.getByRole("button", { name: "Review changes", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("already in use");
  await shot(page, "port-conflict");
  await page.route("**/api/applications/application-web/terminal", r => r.fulfill({ json: { id: "gallery-terminal", ticket: "gallery-ticket", expiresAt: Math.floor(Date.now()/1000)+900 } }));
  await page.route("**/api/applications/application-web/terminal/*/revoke", r => r.fulfill({ json: { revoked: true } }));
  let terminalSocket: any;
  await page.routeWebSocket("**/api/applications/application-web/terminal/gallery-terminal", socket => {
    terminalSocket = socket;
    socket.onMessage(message => { const frame = JSON.parse(String(message)); if (frame.type === "connect") { socket.send(JSON.stringify({ type: "ready" })); socket.send(JSON.stringify({ type: "output", data: Buffer.from("app@production-database:/$ psql --version\r\npsql (PostgreSQL) 18.0\r\napp@production-database:/$ ").toString("base64") })); } });
  });
  await page.goto("/applications/application-web?tab=terminal");
  await shot(page, "terminal-closed");
  await page.getByRole("button", { name: "Open terminal", exact: true }).click();
  await expect(page.getByRole("button", { name: "Close terminal", exact: true })).toBeVisible();
  await page.waitForTimeout(100);
  await shot(page, "terminal-connected");
  terminalSocket.send(JSON.stringify({ type: "end", reason: "Terminal session expired" }));
  await expect(page.getByRole("alert")).toContainText("expired");
  await shot(page, "terminal-expired");
  await page.setViewportSize({ width: 390, height: 844 });
  for (const [tab, state] of [["overview", "database"], ["networking", "networking"], ["backups", "backups"], ["terminal", "terminal"]]) {
    await page.goto("/applications/application-web?tab=" + tab);
    await shot(page, "mobile-" + state);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
});
