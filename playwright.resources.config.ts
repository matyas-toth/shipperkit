import { defineConfig } from "@playwright/test";
import config from "./playwright.config";
export default defineConfig({ ...config, testIgnore: [], testMatch: "**/resources-gallery.spec.ts", timeout: 240_000 });
