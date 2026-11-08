import { createRequire } from "node:module";
import { mkdir } from "node:fs/promises";
const require = createRequire("C:/Users/reign/Documents/ChatGPT/Shipper App/package.json");
const { chromium } = require("playwright");

const browser = await chromium.launch();
await mkdir("reviews/final", { recursive: true });
const out = {};
for (const width of [1440, 1024, 390]) {
  const page = await browser.newPage({ viewport: { width, height: 900 } });
  await page.goto("http://localhost:4330/", { waitUntil: "networkidle" });
  await page.evaluate(() => window.scrollTo(0, 2000));
  await page.waitForTimeout(400);
  out[width] = await page.evaluate(() => {
    const header = document.querySelector("header");
    const rect = header?.getBoundingClientRect();
    const style = header ? getComputedStyle(header) : null;
    return {
      headerTop: rect ? Math.round(rect.top) : null,
      headerVisible: rect ? rect.bottom > 0 && rect.top < innerHeight : false,
      position: style?.position,
      scrollY: Math.round(window.scrollY),
      overflowX: document.documentElement.scrollWidth - innerWidth,
    };
  });
  if (width === 1440) {
    await page.screenshot({ path: "reviews/final/nav-sticky.png" });
  }
  await page.close();
}
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
await page.goto("http://localhost:4330/", { waitUntil: "networkidle" });
await page.evaluate(async () => {
  for (const img of document.querySelectorAll("img")) img.loading = "eager";
  await document.fonts.ready;
});
await page.screenshot({ path: "reviews/final/fold.png" });
await page.screenshot({ path: "reviews/final/full.png", fullPage: true });
// hero fade region
const hero = await page.evaluate(() => {
  const h = document.querySelector("section");
  return h ? { top: h.getBoundingClientRect().top + scrollY, height: h.getBoundingClientRect().height } : null;
});
out.hero = hero;
await browser.close();
console.log(JSON.stringify(out, null, 2));
