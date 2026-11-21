import { defineConfig } from "prisma/config";

try {
  process.loadEnvFile();
} catch {
  // Production injects variables directly.
}

export default defineConfig({
  schema: "prisma/schema.prisma",
  migrations: { path: "prisma/migrations", seed: "tsx prisma/seed.ts" },
  // Generation needs no database, so a missing URL only fails commands that connect.
  datasource: { url: process.env.DATABASE_URL ?? "" },
});
