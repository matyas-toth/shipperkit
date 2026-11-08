import { createRequire } from "node:module";
const require = createRequire("C:/Users/reign/Documents/ChatGPT/Shipper App/package.json");
const { chromium } = require("playwright");
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
await page.goto("http://localhost:4330/", { waitUntil: "networkidle" });
const small = await page.evaluate(() => {
  return [...document.querySelectorAll("body *")].filter((el) => {
    const s = getComputedStyle(el);
    const text = (el.textContent || "").trim();
    return el.children.length === 0 && text && el.tagName !== "IMG" && el.tagName !== "CODE" &&
      parseFloat(s.fontSize) > 0 && parseFloat(s.fontSize) < 14 && !s.fontFamily.includes("Mono");
  }).map((el) => ({ tag: el.tagName, text: el.textContent.trim().slice(0, 50), size: getComputedStyle(el).fontSize, cls: el.className.toString().slice(0, 90) }));
});
console.log(JSON.stringify(small, null, 2));
await browser.close();
