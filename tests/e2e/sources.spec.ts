import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";
test("connections, private repository selection, immutable queue controls and image source", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("shipper-browser-test-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Applications", exact: true }),
  ).toBeVisible();
  const connections = [
    {
      id: "github-fixture",
      name: "Northstar",
      kind: "github_app",
      host: "github.com",
      updatedAt: 1,
    },
    {
      id: "registry-fixture",
      name: "Container registry",
      kind: "registry",
      host: "ghcr.io",
      updatedAt: 1,
    },
  ];
  await page.route("**/api/connections", (r) =>
    r.fulfill({ json: { items: connections } }),
  );
  await page.route("**/api/connections/github-fixture/repositories?**", (r) =>
    r.fulfill({
      json: {
        items: [
          {
            id: 55,
            full_name: "northstar/fieldnotes",
            clone_url: "https://github.com/northstar/fieldnotes.git",
            default_branch: "main",
            private: true,
          },
          {
            id: 56,
            full_name: "northstar/worker",
            clone_url: "https://github.com/northstar/worker.git",
            default_branch: "main",
            private: true,
          },
        ],
        nextPage: 0,
      },
    }),
  );
  await page.route("**/api/repositories/branches", (r) =>
    r.fulfill({
      json: { items: ["main", "develop"], default: "main", revisions: [] },
    }),
  );
  await page.goto("/settings#sources");
  await expect(
    page.getByRole("heading", { name: "Connections", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Northstar", { exact: true })).toBeVisible();
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase6-connections.png"),
  });
  await page.goto("/new");
  await page.getByLabel("Name", { exact: true }).fill("Fieldnotes");
  await page
    .getByLabel("Source", { exact: true })
    .selectOption("github-fixture");
  await page
    .getByLabel("Repository", { exact: true })
    .selectOption("https://github.com/northstar/fieldnotes.git");
  await expect(
    page.getByLabel("Branch or commit", { exact: true }),
  ).toHaveValue("main");
  await page.getByLabel("Branch or commit", { exact: true }).fill("develop");
  await expect(
    page
      .getByLabel("Repository", { exact: true })
      .locator('option[value="https://github.com/northstar/worker.git"]'),
  ).toHaveCount(1);

  await page.getByText("Automatic deployments", { exact: true }).click();
  await page.getByLabel("Deploy on push", { exact: true }).check();
  await page.getByLabel("Watch paths", { exact: true }).fill("apps/web/**");
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase6-private-source.png"),
  });
  let submitted: any;
  const stamp = Math.floor(Date.now() / 1000);
  let deployment: any;
  let history: any[] = [];
  await page.route("**/api/applications", async (r) => {
    if (r.request().method() !== "POST") {
      await r.fallback();
      return;
    }
    submitted = r.request().postDataJSON();
    deployment = {
      id: "private-build",
      applicationId: "private-fixture",
      applicationName: "Fieldnotes",
      operation: "deploy",
      state: "building",
      config: submitted.config,
      commit: "a".repeat(40),
      imageId: "",
      containerId: "",
      previousId: "",
      error: "",
      createdAt: stamp,
      startedAt: stamp,
      finishedAt: 0,
      services: [],
      resolvedMethod: "dockerfile",
      runtimePort: 3000,
      noCache: false,
    };
    history = [deployment];
    await r.fulfill({ status: 202, json: deployment });
  });
  await page.route("**/api/applications/private-fixture", (r) =>
    r.fulfill({
      json: {
        application: {
          id: "private-fixture",
          name: "Fieldnotes",
          sourceKind: "git",
          sourceRef: submitted.config.repository,
          method: "auto",
          state: "created",
          createdAt: stamp,
          updatedAt: stamp,
          containers: [],
          config: submitted.config,
          activeDeploymentId: "",
          latestDeployment: deployment,
          domains: [],
          desiredState: "running",
        },
        deployments: history,
        activeDeployment: null,
      },
    }),
  );
  await page.route(
    "**/api/applications/private-fixture/deployments/*/logs?**",
    (r) =>
      r.fulfill({
        json: {
          items: [
            {
              number: 1,
              sequence: 1,
              step: "building",
              line: "Building the selected commit",
              createdAt: stamp,
            },
          ],
          state: deployment.state,
        },
      }),
  );
  await page.route(
    "**/api/applications/private-fixture/deployments/*/cancel",
    async (r) => {
      deployment.state = "cancelled";
      deployment.finishedAt = stamp + 5;
      await r.fulfill({ status: 202, json: { cancelRequested: true } });
    },
  );
  await page.route(
    "**/api/applications/private-fixture/deployments/*/retry",
    async (r) => {
      deployment = {
        ...deployment,
        id: "retry-build",
        state: "queued",
        finishedAt: 0,
      };
      history = [deployment, ...history];
      await r.fulfill({ status: 202, json: deployment });
    },
  );
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Fieldnotes", exact: true }),
  ).toBeVisible();
  expect(submitted.config.sourceId).toBe("github-fixture");
  expect(submitted.config.repositoryId).toBe(55);
  expect(submitted.config.autoDeploy.paths).toEqual(["apps/web/**"]);
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.locator(".log-heading")).toContainText("Cancelled");
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page).toHaveURL(/deployment=retry-build/);
  await expect(page.locator(".log-heading")).toContainText("Queued");
  await page.goto("/new");
  await page
    .getByLabel("Deployment method", { exact: true })
    .selectOption("image");
  await expect(page.getByLabel("Git repository", { exact: true })).toHaveCount(
    0,
  );
  await page.getByLabel("Name", { exact: true }).fill("Image worker");
  await page
    .getByLabel("Image", { exact: true })
    .fill("ghcr.io/northstar/worker:latest");
  await page
    .getByLabel("Registry", { exact: true })
    .selectOption("registry-fixture");
  await page.getByText("Configuration", { exact: true }).click();
  await page.getByLabel("Readiness", { exact: true }).selectOption("process");
  await page.screenshot({ path: join(tmpdir(), "shipper-phase6-image.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase6-image-mobile.png"),
  });
  expect(
    await page.locator("body").evaluate((e) => e.scrollWidth <= innerWidth),
  ).toBe(true);
  expect(errors).toEqual([]);
});
