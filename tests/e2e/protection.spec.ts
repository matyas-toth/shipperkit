import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "wait" });
});
test("instance recovery, encrypted downloads, schedule choices and mobile backup UI", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("shipper-browser-test-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Applications", exact: true }),
  ).toBeVisible();
  await page.goto("/settings#recovery");
  const recovery = page.locator("#recovery");
  const restoreSteps = recovery.locator("summary").filter({ hasText: "Recover an installation" });
  await expect(recovery.getByText("Empty data directory", { exact: true })).toBeHidden();
  await restoreSteps.focus();
  await restoreSteps.press("Enter");
  await expect(recovery.getByText("Empty data directory", { exact: true })).toBeVisible();
  await restoreSteps.click();
  await expect(recovery.getByText("Empty data directory", { exact: true })).toBeHidden();
  await recovery
    .getByRole("button", { name: "New schedule", exact: true })
    .click();
  await page
    .getByLabel("Name", { exact: true })
    .fill("Daily instance recovery");
  await page
    .locator(".backup-editor summary")
    .filter({ hasText: "Schedule" })
    .click();
  await page.getByLabel("Repeat", { exact: true }).selectOption("weekly");
  await page.getByRole("button", { name: "Wed", exact: true }).click();
  await page.getByLabel("At", { exact: true }).fill("03:30");
  await page.getByLabel("Timezone", { exact: true }).fill("Europe/Budapest");
  await expect(page.getByLabel("Cron expression", { exact: true })).toHaveValue(
    "30 3 * * 1,3",
  );
  await expect(
    page.getByLabel("Cron expression", { exact: true }),
  ).toHaveAttribute("readonly", "");
  await expect(page.locator(".schedule-next time")).toHaveCount(3);
  await page
    .locator(".backup-editor")
    .screenshot({ path: join(tmpdir(), "shipper-phase8-schedule.png") });
  await page.getByRole("button", { name: "Save schedule", exact: true }).click();
  await expect(
    recovery.getByRole("button", {
      name: "Daily instance recovery",
      exact: true,
    }),
  ).toBeVisible();
  const keyPromise = page.waitForEvent("download");
  await recovery
    .getByRole("button", { name: "Download recovery key", exact: true })
    .click();
  expect((await keyPromise).suggestedFilename()).toBe(
    "shipper-recovery-key.txt",
  );
  await recovery.getByRole("button", { name: "Run now", exact: true }).click();
  await expect(recovery.locator(".backup-history").getByText("Backed up", { exact: true })).toBeVisible({
    timeout: 15000,
  });
  await recovery.locator(".backup-history tr[role=button]").first().click();
  await expect(
    recovery.locator(".backup-checksum"),
  ).toHaveText(/^[a-f0-9]{64}$/);
  const archive = page.waitForEvent("download");
  await recovery
    .getByRole("button", { name: "Download archive", exact: true })
    .click();
  expect((await archive).suggestedFilename()).toMatch(/^shipper-.*\.age$/);
  await recovery
    .getByRole("button", { name: "Close backup details", exact: true })
    .click();
  await recovery
    .getByRole("button", { name: "New schedule", exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase8-recovery.png"),
  });
  await recovery
    .getByRole("button", { name: "Add S3 destination", exact: true })
    .click();
  await page.getByLabel("Name", { exact: true }).fill("Off-server backups");
  await page
    .getByLabel("Endpoint", { exact: true })
    .fill("https://s3.eu-central-1.amazonaws.com");
  await page.getByLabel("Region", { exact: true }).fill("eu-central-1");
  await page.getByLabel("Bucket", { exact: true }).fill("shipper-recovery");
  await page
    .getByLabel("Access key", { exact: true })
    .fill("browser-test-access");
  await page
    .getByLabel("Secret key", { exact: true })
    .fill("browser-test-secret");
  await page
    .getByRole("button", { name: "Save destination", exact: true })
    .click();
  await expect(
    recovery.getByRole("button", { name: "Off-server backups", exact: true }),
  ).toBeVisible();
  expect(await page.locator("body").innerText()).not.toContain(
    "browser-test-secret",
  );
  await page.goto("/applications/application-web?tab=backups");
  await expect(
    page.getByRole("button", { name: "New schedule", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "New schedule", exact: true }).click();
  await page
    .getByRole("radio", { name: "Persistent volume", exact: true })
    .check();
  await expect(page.getByLabel("Volume", { exact: true })).toBeVisible();
  await page.getByRole("radio", { name: "Database", exact: true }).check();
  await page.getByLabel("Database name", { exact: true }).fill("appdb");
  await page
    .locator(".backup-editor summary")
    .filter({ hasText: "Schedule" })
    .click();
  await page.getByLabel("Repeat", { exact: true }).selectOption("daily");
  await expect(page.locator(".schedule-next time")).toHaveCount(3);
  await page.screenshot({
    path: join(tmpdir(), "shipper-phase8-application.png"),
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator(".backup-editor").scrollIntoViewIfNeeded();
  await page.screenshot({ path: join(tmpdir(), "shipper-phase8-mobile.png") });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
});

test("reviewed database copy keeps existing data and requires the application name", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("shipper-browser-test-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Applications", exact: true }),
  ).toBeVisible();
  const now = Math.floor(Date.now() / 1000);
  let submitted: Record<string, unknown> | undefined;
  await page.route(
    "**/api/applications/application-web/protection",
    async (route) => {
      const body = {
        plans: [],
        destinations: [],
        volumes: [],
        volumeError: "",
        recipient: "age1fixture",
        revision: "",
        runs: [] as unknown[],
      };
      body.runs = [
        {
          id: "database-fixture",
          planId: "",
          operation: "backup",
          state: "succeeded",
          createdAt: now - 300,
          startedAt: now - 300,
          finishedAt: now - 298,
          error: "",
          bytes: 65344,
          checksum: "a".repeat(64),
          objectKey: "",
          result: "Encrypted archive verified; pg_dump (PostgreSQL) 17",
          sourceId: "",
          verifiedAt: 0,
        },
      ];
      await route.fulfill({ json: body });
    },
  );
  await page.route(
    "**/api/applications/application-web/protection/runs/database-fixture/restore",
    async (route) => {
      const input = route.request().postDataJSON();
      if (input.review) {
        await route.fulfill({
          json: {
            revision: "fixture-review",
            kind: "database",
            engine: "postgres",
            database: "appdb",
            service: "",
            volume: "",
          },
        });
      } else {
        submitted = input;
        await route.fulfill({
          status: 202,
          json: { id: "restore-fixture", state: "queued" },
        });
      }
    },
  );
  await page.goto("/applications/application-web?tab=backups");
  await page
    .getByRole("button", {
      name: "Inspect backup database-fixture",
      exact: true,
    })
    .click();
  await page
    .getByRole("button", { name: "Restore a copy", exact: true })
    .click();
  await expect(page.getByText("Preserved", { exact: true })).toBeVisible();
  await expect(page.getByLabel("New database", { exact: true })).toHaveValue(
    "appdb_recovered",
  );
  await expect(
    page.getByRole("button", { name: "Restore copy", exact: true }),
  ).toBeDisabled();
  await page.getByLabel("Confirm application name", { exact: true }).fill("Website");
  await expect(page.getByRole("button", { name: "Restore copy", exact: true })).toBeEnabled();
  await page
    .locator(".restore-review")
    .screenshot({ path: join(tmpdir(), "shipper-phase8-restore.png") });
  await page.getByRole("button", { name: "Restore copy", exact: true }).click();
  await expect(page.locator(".restore-review")).toHaveCount(0);
  expect(submitted).toMatchObject({
    target: "appdb_recovered",
    confirm: "Website",
    revision: "fixture-review",
  });
  expect(errors).toEqual([]);
});
