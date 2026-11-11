import { test, expect, type Page } from "@playwright/test";

test.use({ viewport: { width: 1440, height: 900 } });

async function login(page: Page) {
  await page.goto("/");
  const setup = page.getByLabel("Setup token");
  if (await setup.count()) {
    await setup.fill("shipper-browser-test-setup-token-00000000000000000000000000000000");
    await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
    await page.getByLabel("Password", { exact: true }).fill("shipper-browser-test-password");
    await page.getByRole("button", { name: "Create administrator", exact: true }).click();
  } else {
    await page.getByLabel("Email", { exact: true }).fill("owner@example.test");
    await page.getByLabel("Password", { exact: true }).fill("shipper-browser-test-password");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
  }
  await expect(page.getByRole("heading", { name: "Applications", exact: true })).toBeVisible();
}

test("select menus mirror the native options and write back", async ({ page }) => {
  await login(page);
  await page.goto("/new");
  await page.getByRole("button", { name: "Configure manually", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Controls");
  await page.getByRole("textbox", { name: "Repository URL", exact: true }).fill("https://github.com/acme/portfolio.git");
  await page.getByRole("button", { name: "Continue", exact: true }).click();

  const select = page.getByLabel("Build method", { exact: true });
  await expect(select).toHaveValue("auto");

  await select.click();
  await expect(page.locator(".select-menu")).toBeVisible();
  await expect(page.locator(".select-menu")).toHaveAttribute("role", "listbox");
  await expect(select).toBeFocused();
  await expect(select).toHaveAttribute("aria-expanded", "true");
  await expect(select).toHaveAttribute("aria-activedescendant", /select-menu-/);
  await page.locator(".select-menu-option", { hasText: "Dockerfile" }).click();
  await expect(select).toHaveValue("dockerfile");
  await expect(page.locator(".select-menu")).toHaveCount(0);
  await expect(select).toHaveAttribute("aria-expanded", "false");

  await page.keyboard.press("ArrowDown");
  await expect(page.locator(".select-menu")).toBeVisible();
  await page.keyboard.press("ArrowDown");
  await expect(select).toHaveValue("railpack");
  await page.keyboard.press("Enter");
  await expect(page.locator(".select-menu")).toHaveCount(0);

  await select.selectOption("static");
  await expect(select).toHaveValue("static");
});

test("row action menus escape the table and manage focus", async ({ page }) => {
  await login(page);
  await page.goto("/settings#users");
  await expect(page.getByRole("heading", { name: "Users", exact: true })).toBeVisible();

  const trigger = page.locator(".action-trigger").first();
  await trigger.click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  await expect(menu.getByRole("menuitem").first()).toBeFocused();
  expect(await menu.evaluate(node => node.closest(".users-tray"))).toBeNull();

  await page.keyboard.press("ArrowDown");
  await expect(menu.getByRole("menuitem").nth(1)).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
  await expect(trigger).toBeFocused();
});
