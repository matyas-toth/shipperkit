import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";

test("routing options, runtime limits, recorded release review and certificate diagnostics", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("shipper-browser-test-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Applications", exact: true }),
  ).toBeVisible();
  const config = {
    repository: "https://github.com/northstar/fieldnotes",
    branch: "main",
    method: "dockerfile",
    rootDirectory: ".",
    dockerfile: "Dockerfile",
    port: 3000,
    healthPath: "/",
    memoryMB: 512,
    variables: [],
    volumes: [],
    hooks: [],
    runtime: {},
  };
  const stamp = Math.floor(Date.now() / 1000);
  const first = {
    id: "release-one",
    applicationId: "release-fixture",
    applicationName: "Fieldnotes",
    operation: "deploy",
    state: "succeeded",
    config,
    commit: "a".repeat(40),
    imageId: "sha256:first",
    containerId: "",
    previousId: "",
    error: "",
    createdAt: stamp - 600,
    startedAt: stamp - 600,
    finishedAt: stamp - 570,
    resolvedMethod: "dockerfile",
    runtimePort: 3000,
    noCache: false,
  };
  let latest = {
    ...first,
    id: "release-two",
    commit: "b".repeat(40),
    imageId: "sha256:second",
    createdAt: stamp - 300,
    finishedAt: stamp - 270,
  };
  let queued = false;
  let domains = [
    {
      id: "primary",
      hostname: "fieldnotes.example.com",
      https: true,
      primary: true,
      redirect: false,
      verificationState: "verified",
      certificateIssuer: "Let's Encrypt",
      certificateExpiresAt: stamp + 86400 * 70,
    },
    {
      id: "alias",
      hostname: "alias.fieldnotes.example.com",
      https: true,
      primary: false,
      redirect: false,
      verificationState: "",
      certificateIssuer: "",
      certificateExpiresAt: 0,
      responseHeaders: { "X-Test": "alias" },
    },
  ];
  let submitted: any;
  await page.route("**/api/applications/release-fixture", (r) =>
    r.fulfill({
      json: {
        application: {
          id: "release-fixture",
          name: "Fieldnotes",
          sourceKind: "git",
          sourceRef: config.repository,
          method: "dockerfile",
          state: "running",
          createdAt: stamp - 700,
          updatedAt: stamp,
          containers: [],
          config,
          activeDeploymentId: "release-two",
          latestDeployment: latest,
          domains,
          desiredState: "running",
        },
        deployments: [latest, first],
        activeDeployment: { ...latest, id: "release-two", state: "succeeded" },
      },
    }),
  );
  await page.route(
    "**/api/applications/release-fixture/deployments/*/logs?**",
    (r) =>
      r.fulfill({
        json: {
          items: [
            {
              number: 1,
              sequence: 1,
              step: "promoting",
              line: "Application is serving",
              createdAt: stamp - 570,
            },
          ],
          state: "succeeded",
        },
      }),
  );
  await page.route("**/api/proxy", (r) =>
    r.fulfill({
      json: {
        bindIP: "0.0.0.0",
        httpPort: 80,
        httpsPort: 443,
        acmeEmail: "owner@example.com",
        acmeCA: "",
        dashboardURL: "https://shipper.example.com",
        dashboardUpstream: "http://dashboard:8080",
      },
    }),
  );
  await page.route("**/api/applications/release-fixture/domains", async (r) => {
    submitted = r.request().postDataJSON();
    domains = submitted.domains;
    await r.fulfill({
      status: 202,
      json: {
        ...latest,
        id: "route-change",
        operation: "domains",
        state: "queued",
      },
    });
  });
  await page.route(
    "**/api/applications/release-fixture/deployments/release-one/rollback",
    async (r) => {
      const input = r.request().postDataJSON();
      if (input.review) {
        await r.fulfill({
          json: {
            operation: "rollback",
            revision: "reviewed-revision",
            changes: [
              "Recorded image and runtime settings",
              "Current domains retained",
              "Persistent data retained",
              "Deployment migration hooks skipped",
            ],
          },
        });
      } else {
        expect(input.revision).toBe("reviewed-revision");
        queued = true;
        latest = { ...first, id: "restored-release", state: "queued" };
        await r.fulfill({ status: 202, json: latest });
      }
    },
  );
  await page.goto("/applications/release-fixture?tab=domains");
  await page
    .getByLabel("Routing options for fieldnotes.example.com", { exact: true })
    .click();
  const routingOptions = page
    .locator(".domain-route-options")
    .filter({
      has: page.getByLabel("Routing options for fieldnotes.example.com", {
        exact: true,
      }),
    });
  await routingOptions.getByLabel("Path prefix", { exact: true }).fill("/api");
  await routingOptions.getByLabel("Strip prefix", { exact: true }).check();
  await routingOptions.getByLabel("Compression", { exact: true }).check();
  await routingOptions
    .getByLabel("Response headers", { exact: true })
    .pressSequentially("Cache-Control: no-store");
  await expect(
    page.getByText("Certificate expires", { exact: true }),
  ).toBeVisible();
  await page.screenshot({ path: join(tmpdir(), "shipper-phase7-routing.png") });
  await page.getByRole("button", { name: "Save domains", exact: true }).click();
  expect(submitted.domains[0]).toMatchObject({
    pathPrefix: "/api",
    stripPrefix: true,
    compress: true,
    responseHeaders: { "Cache-Control": "no-store" },
  });
  await page
    .getByRole("button", { name: "Remove fieldnotes.example.com", exact: true })
    .click();
  await expect(
    page.getByLabel("Response headers", { exact: true }),
  ).toHaveValue("X-Test: alias");
  await page.goto("/applications/release-fixture?tab=settings");
  await page.getByText("Runtime limits", { exact: true }).click();
  await page.getByLabel("CPU limit", { exact: true }).fill("2");
  await page.getByLabel("Stop grace (seconds)", { exact: true }).fill("20");
  await page.screenshot({ path: join(tmpdir(), "shipper-phase7-runtime.png") });
  await page.goto(
    "/applications/release-fixture?tab=deployments&deployment=release-one",
  );
  await page
    .getByRole("button", { name: "Restore release", exact: true })
    .click();
  await expect(
    page.getByRole("dialog", { name: "Restore release", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Persistent data retained", { exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase7-rollback.png"),
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase7-rollback-mobile.png"),
  });
  expect(
    await page.locator("body").evaluate((e) => e.scrollWidth <= innerWidth),
  ).toBe(true);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Restore release", exact: true })
    .click();
  await expect(page).toHaveURL(/deployment=restored-release/);
  expect(queued).toBe(true);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.route("**/api/proxy/check", (r) =>
    r.fulfill({
      json: {
        state: "verified",
        message: "Verified",
        certificateIssuer: "Let's Encrypt",
        certificateExpiresAt: stamp + 86400 * 70,
      },
    }),
  );
  await page.goto("/settings#proxy");
  await page
    .getByRole("button", { name: "Verify dashboard", exact: true })
    .click();
  await expect(page.getByText("Verified", { exact: true })).toBeVisible();
  await page.screenshot({ path: join(tmpdir(), "shipper-phase7-proxy.png") });
  expect(errors).toEqual([]);
});
