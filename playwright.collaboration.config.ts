import { defineConfig } from "@playwright/test";
import config from "./playwright.config";
export default defineConfig({ ...config, testIgnore: [], testMatch: "**/collaboration-gallery.spec.ts", timeout: 120_000 });
