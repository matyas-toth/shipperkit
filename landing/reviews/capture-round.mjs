import { createRequire } from "node:module";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";

const require = createRequire("C:/Users/reign/Documents/ChatGPT/Shipper App/package.json");
const { chromium } = require("playwright");

const round = process.argv[2] || "round-01";
const dir = join(process.cwd(), "reviews", round);
await mkdir(dir, { recursive: true });

const targets = [
  { key: "ours", url: "http://localhost:4330/", local: true },
  { key: "coolify", url: "https://coolify.io/" },
  { key: "vercel", url: "https://vercel.com/" },
  { key: "t3code", url: "https://t3.codes/" },
];

async function settle(page, local) {
  // Make every image eager and decoded so full-page captures never show empty frames.
  await page.evaluate(async () => {
    for (const img of document.querySelectorAll("img")) {
      img.loading = "eager";
      img.decoding = "sync";
    }
    const step = 500;
    const height = Math.max(document.body.scrollHeight, document.documentElement.scrollHeight);
    for (let y = 0; y < height; y += step) {
      window.scrollTo(0, y);
      await new Promise((resolve) => setTimeout(resolve, 60));
    }
    window.scrollTo(0, 0);
    await document.fonts.ready;
  });
  await page.waitForTimeout(local ? 500 : 2500);
  await page.evaluate(async () => {
    await Promise.all(
      [...document.querySelectorAll("img")].map((img) =>
        img.complete && img.naturalWidth > 0
          ? Promise.resolve()
          : img.decode().catch(() => {}),
      ),
    );
  });
  await page.waitForTimeout(300);
}

async function dismissCookies(page) {
  for (const label of ["Accept all", "Accept all cookies", "Accept", "Allow all", "Got it"]) {
    const button = page.getByRole("button", { name: label, exact: true });
    if (await button.count()) {
      await button.first().click({ timeout: 1500 }).catch(() => {});
      break;
    }
  }
}

async function shoot(browser, target, viewport, suffix = "") {
  const page = await browser.newPage({ viewport, deviceScaleFactor: 1 });
  let status = null;
  try {
    const response = await page.goto(target.url, { waitUntil: "domcontentloaded", timeout: 90_000 });
    status = response?.status() ?? null;
    await page.waitForTimeout(target.local ? 1200 : 3000);
    await dismissCookies(page);
    await settle(page, target.local);
    const title = await page.title();
    const height = await page.evaluate(() => Math.max(document.body.scrollHeight, document.documentElement.scrollHeight));
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    await page.screenshot({ path: join(dir, `${target.key}${suffix}-fold.png`), animations: "disabled", caret: "hide" });
    await page.screenshot({ path: join(dir, `${target.key}${suffix}-full.png`), fullPage: true, animations: "disabled", caret: "hide" });
    return { key: `${target.key}${suffix}`, url: target.url, width: viewport.width, status, title, height, overflow, error: null };
  } catch (error) {
    return { key: `${target.key}${suffix}`, url: target.url, width: viewport.width, status, title: null, height: null, overflow: null, error: String(error).slice(0, 400) };
  } finally {
    await page.close();
  }
}

const browser = await chromium.launch();
const desktop = { width: 1440, height: 1000 };
const metadata = [];

for (const target of targets) {
  metadata.push(await shoot(browser, target, desktop));
}

const ours = targets[0];
metadata.push(await shoot(browser, ours, { width: 1024, height: 900 }, "-1024"));
metadata.push(await shoot(browser, ours, { width: 390, height: 844 }, "-390"));

await writeFile(join(dir, "capture.json"), JSON.stringify(metadata, null, 2));
await browser.close();
console.log(JSON.stringify({ dir, metadata }, null, 2));
