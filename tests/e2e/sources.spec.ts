import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";

async function checkRepositoryLoading(page: import("@playwright/test").Page) {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/api/connections", route => route.fulfill({ json: { items: [
    { id: "first-account", name: "Northstar", kind: "github_app" },
    { id: "second-account", name: "Acme", kind: "github_app" },
  ] } }));
  const pending: import("@playwright/test").Route[] = [];
  await page.route("**/api/connections/*/repositories?**", route => { pending.push(route); });
  const repo = (id: number, owner: string) => ({ id, full_name: owner + "/app", clone_url: "https://github.com/" + owner + "/app.git", default_branch: "main" });
  await page.goto("/new");
  await page.getByLabel("Connection", { exact: true }).selectOption("first-account");
  const picker = page.getByLabel("Repository", { exact: true });
  await expect(picker).toBeDisabled();
  await expect(picker).toContainText("Loading repositories");
  await expect.poll(() => pending.length).toBe(1);
  await page.screenshot({ path: join(tmpdir(), "shipper-issue-11-loading.png") });
  await pending[0].fulfill({ json: { items: [repo(1, "northstar")], nextPage: 2 } });
  await expect(picker).toBeEnabled();
  await picker.selectOption(repo(1, "northstar").clone_url);
  await page.getByRole("button", { name: "Load more repositories" }).click();
  await expect.poll(() => pending.length).toBe(2);
  await expect(picker).toBeEnabled();
  await page.getByLabel("Connection", { exact: true }).selectOption("second-account");
  await expect(picker).toBeDisabled();
  await expect(picker.locator("option")).not.toContainText(["northstar/app"]);
  await expect(page.getByLabel("Branch or commit")).toHaveValue("");
  await expect.poll(() => pending.length).toBe(3);
  await pending[1].fulfill({ json: { items: [repo(2, "old-account")], nextPage: 0 } });
  await pending[2].fulfill({ json: { items: [repo(3, "acme")], nextPage: 0 } });
  await expect(picker).toBeEnabled();
  await expect(picker).not.toContainText("old-account");
  await picker.selectOption(repo(3, "acme").clone_url);
  await expect(page.getByLabel("Branch or commit")).toHaveValue("main");
  await page.screenshot({ path: join(tmpdir(), "shipper-issue-11-loaded.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: join(tmpdir(), "shipper-issue-11-mobile.png") });
  expect(errors).toEqual([]);
  await page.getByLabel("Connection", { exact: true }).selectOption("");
  await page.unroute("**/api/connections");
  await page.unroute("**/api/connections/*/repositories?**");
  await page.setViewportSize({ width: 1440, height: 1000 });
}

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
  await checkRepositoryLoading(page);
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
  await page.getByRole("button", { name: "Connect GitHub", exact: true }).click();
  await expect(page.getByRole("radio", { name: "Personal account", exact: true })).toBeChecked();
  await expect(page.getByRole("radio", { name: "Personal account", exact: true })).toBeFocused();
  await page.getByRole("form", { name: "Connect GitHub" }).getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByRole("button", { name: "Connect GitHub", exact: true })).toBeFocused();
  await page.getByRole("button", { name: "Connect GitHub", exact: true }).click();
  await page.getByRole("radio", { name: "Organization", exact: true }).check();
  await page.getByRole("button", { name: "Continue to GitHub", exact: true }).click();
  expect(await page.getByLabel("Organization name").evaluate((input: HTMLInputElement) => input.validity.valueMissing)).toBe(true);
  await page.getByLabel("Organization name").fill("invalid/organization");
  expect(await page.getByLabel("Organization name").evaluate((input: HTMLInputElement) => input.validity.patternMismatch)).toBe(true);
  await page.getByLabel("Organization name", { exact: true }).fill("northstar");
  const spacing = async () => {
    const button = await page.getByRole("button", { name: "Continue to GitHub", exact: true }).boundingBox();
    const table = await page.locator(".connection-table").boundingBox();
    expect(table!.y - button!.y - button!.height).toBeGreaterThanOrEqual(32);
    const choice = await page.getByRole("group", { name: "GitHub account" }).boundingBox();
    const field = await page.locator(".github-connect-form .field").boundingBox();
    expect(field!.y - choice!.y - choice!.height).toBeGreaterThanOrEqual(24);
  };
  await spacing();
  await page.screenshot({ path: join(tmpdir(), "shipper-issue-10-organization.png") });
  for (const width of [320, 390, 767]) {
    await page.setViewportSize({ width, height: 844 });
    await spacing();
    for (const name of await page.locator(".connection-name").all()) {
      const box = await name.boundingBox();
      expect(box!.height).toBeLessThan(24);
    }
    for (const action of await page.locator(".connection-actions .button").all()) {
      const box = await action.boundingBox();
      expect(box!.height).toBeGreaterThanOrEqual(40);
      expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    }
    expect(await page.locator(".connection-table tbody td").evaluateAll(cells => cells.every(cell => getComputedStyle(cell).borderBottomWidth === "0px"))).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.setViewportSize({ width: 390, height: 1080 });
  await page.locator(".page-scroll").evaluate(el => { el.scrollTop = 0; });
  await page.screenshot({ path: join(tmpdir(), "shipper-issue-10-mobile.png") });
  await page.setViewportSize({ width: 1440, height: 1000 });
  // Capture the real manifest POST without registering an app on GitHub.
  await page.route("https://github.com/**", route => route.fulfill({ contentType: "text/html", body: "<title>GitHub fixture</title>" }));
  const registration = page.waitForRequest(request => request.url().startsWith("https://github.com/organizations/northstar/settings/apps/new?state="));
  await page.getByRole("button", { name: "Continue to GitHub", exact: true }).click();
  const manifest = JSON.parse(new URLSearchParams((await registration).postData()!).get("manifest")!);
  expect(manifest.public).toBe(false);
  expect(manifest.default_permissions).toEqual({ contents: "read", metadata: "read" });
  await page.waitForURL("https://github.com/**");
  await page.goto("/settings#sources");
  await page.getByRole("button", { name: "Connect GitHub", exact: true }).click();
  await expect(page.getByLabel("Organization name")).toHaveCount(0);
  const personalRegistration = page.waitForRequest(request => request.url().startsWith("https://github.com/settings/apps/new?state="));
  await page.getByRole("button", { name: "Continue to GitHub", exact: true }).click();
  expect(new URLSearchParams((await personalRegistration).postData()!).has("manifest")).toBe(true);
  await page.waitForURL("https://github.com/**");
  await page.goto("/settings#sources");
  await page.unroute("https://github.com/**");
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase6-connections.png"),
  });
  await page.goto("/new");
  await page.getByLabel("Name", { exact: true }).fill("Fieldnotes");
  await page
    .getByLabel("Connection", { exact: true })
    .selectOption("github-fixture");
  await page
    .getByLabel("Repository", { exact: true })
    .selectOption("https://github.com/northstar/fieldnotes.git");
  await expect(
    page.getByLabel("Branch or commit", { exact: true }),
  ).toHaveValue("main");
  await page.getByRole("button", { name: "Browse branches", exact: true }).click();
  await page.getByRole("group", { name: "Branches", exact: true }).getByRole("button", { name: "develop", exact: true }).click();
  await expect(page.getByLabel("Branch or commit", { exact: true })).toHaveValue("develop");
  await expect(
    page
      .getByLabel("Repository", { exact: true })
      .locator('option[value="https://github.com/northstar/worker.git"]'),
  ).toHaveCount(1);

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
  await page.route("**/api/repositories/inspect", r => r.fulfill({json:{method:"dockerfile",commit:"a".repeat(40),variables:[]}}));
  await page.route("**/api/plan", r => {
    const input = r.request().postDataJSON();
    return r.fulfill({json:{name:input.name,config:input.config,applicationId:"d".repeat(24),revision:"fixture",commit:input.commit}});
  });
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await page.getByText("Automatic deployments", { exact: true }).click();
  await page.getByLabel("Deploy on push", { exact: true }).check();
  await page.getByLabel("Watch paths", { exact: true }).fill("apps/web/**");
  await page.getByRole("button", { name: "Review deployment", exact: true }).click();
  await expect(page.locator(".creation-review")).toContainText("Dockerfile (automatic)");
  await page.getByRole("button", { name: "Deploy", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Fieldnotes", exact: true }),
  ).toBeVisible();
  expect(submitted.config.sourceId).toBe("github-fixture");
  expect(submitted.config.repositoryId).toBe(55);
  expect(submitted.config.branch).toBe("develop");
  expect(submitted.commit).toBe("a".repeat(40));
  expect(submitted.config.autoDeploy.paths).toEqual(["apps/web/**"]);
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByRole("region", { name: "Selected release" })).toContainText("Cancelled");
  await page.getByRole("button", { name: "Retry release", exact: true }).click();
  await expect(page).toHaveURL(/deployment=retry-build/);
  await expect(page.getByRole("region", { name: "Selected release" })).toContainText("Queued");
  await page.goto("/new");
  await page
    .getByRole("radio", {name:"Container image",exact:true})
    .check();
  await expect(page.getByRole("textbox", { name: "Repository URL", exact: true })).toHaveCount(
    0,
  );
  await page.getByLabel("Name", { exact: true }).fill("Image worker");
  await page
    .getByLabel("Image", { exact: true })
    .fill("ghcr.io/northstar/worker:latest");
  await page
    .getByLabel("Registry", { exact: true })
    .selectOption("registry-fixture");
  await page.getByRole("button",{name:"Continue",exact:true}).click();
  await page.getByRole("radio",{name:"Worker",exact:true}).check();
  await expect(page.getByLabel("Domain",{exact:true})).toHaveCount(0);
  await expect(page.getByLabel("Container port",{exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:"Review deployment",exact:true}).click();
  await expect(page.locator(".creation-review")).toContainText("Worker");
  await expect(page.locator(".creation-review")).not.toContainText("Domain");
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
