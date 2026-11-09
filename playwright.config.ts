import { defineConfig } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";

export default defineConfig({
  testDir: "./tests/e2e",
  // Gallery and diagnostic captures need a separate, fresh setup session.
  testIgnore: ["**/gallery.spec.ts", "**/debug.spec.ts", "**/resources-gallery.spec.ts"],
  fullyParallel: false,
  workers: 1,
  timeout: 30_000,
  outputDir: join(tmpdir(), "shipper-playwright-results"),
  use: { baseURL: "http://127.0.0.1:9081", browserName: "chromium", trace: "retain-on-failure" },
  webServer: { command: "node scripts/test-server.mjs", url: "http://127.0.0.1:9081/api/health", reuseExistingServer: false, timeout: 60_000 },
});

