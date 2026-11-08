import { createRequire } from "node:module";
const require = createRequire("C:/Users/reign/Documents/ChatGPT/Shipper App/package.json");
const { chromium } = require("playwright");

const browser = await chromium.launch();
const out = {};
for (const width of [1440, 1024, 390]) {
  const page = await browser.newPage({ viewport: { width, height: 900 } });
  await page.goto("http://localhost:4330/", { waitUntil: "networkidle" });
  await page.evaluate(async () => {
    for (const img of document.querySelectorAll("img")) img.loading = "eager";
    const step = 400;
    const h = document.body.scrollHeight;
    for (let y = 0; y < h; y += step) {
      window.scrollTo(0, y);
      await new Promise((r) => setTimeout(r, 30));
    }
    window.scrollTo(0, 0);
    await document.fonts.ready;
  });
  out[width] = await page.evaluate(() => {
    const broken = [...document.querySelectorAll("img")].filter((i) => !i.complete || i.naturalWidth === 0).map((i) => i.getAttribute("src"));
    const small = [...document.querySelectorAll("body *")].filter((el) => {
      const s = getComputedStyle(el);
      const text = (el.textContent || "").trim();
      return el.children.length === 0 && text && el.tagName !== "IMG" && el.tagName !== "CODE" &&
        parseFloat(s.fontSize) > 0 && parseFloat(s.fontSize) < 14 && !s.fontFamily.includes("Mono");
    }).length;
    const missing = [...new Set([...document.querySelectorAll('a[href^="#"]')].map((a) => a.getAttribute("href")).filter((h) => h && h !== "#" && !document.getElementById(h.slice(1))))];
    return {
      height: document.body.scrollHeight,
      overflow: document.documentElement.scrollWidth - window.innerWidth,
      broken,
      small,
      missing,
      desktopShot: !!document.querySelector("img[src='/shots/applications.png']")?.offsetParent,
      mobileConsole: !!document.querySelector("div.sm\\:hidden > div")?.offsetParent,
      screenshotsVisible: [...document.querySelectorAll("img[src^='/shots/']")].filter((i) => i.offsetParent).length,
    };
  });
  if (width === 1440) {
    const copy = page.locator("#install [data-copy]").first();
    await copy.scrollIntoViewIfNeeded();
    await copy.click();
    await page.waitForFunction(() => document.querySelector("#install [data-copy-label]")?.textContent === "Copied", null, { timeout: 3000 });
    out.copyState = await page.locator("#install [data-copy-label]").first().textContent();
    await page.locator("header a", { hasText: "Install Shipper" }).first().click();
    await page.waitForTimeout(900);
    out.targetHasCopy = await page.locator("#get-shipper [data-copy]").count();
    out.atBottom = await page.evaluate(() => window.scrollY + innerHeight > document.body.scrollHeight - 200);
  }
  await page.close();
}
await browser.close();
console.log(JSON.stringify(out, null, 2));
