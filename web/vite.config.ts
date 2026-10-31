import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import tailwind from "@tailwindcss/vite";
import { writeFileSync } from "node:fs";

export default defineConfig({
  plugins: [solid(), tailwind(), { name: "preserve-embed-directory", closeBundle() { writeFileSync(new URL("../internal/ui/dist/.gitkeep", import.meta.url), ""); } }],
  build: {
    outDir: "../internal/ui/dist",
    emptyOutDir: true,
    target: "es2022",
    sourcemap: false,
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: { "/api": { target: "http://127.0.0.1:8080", changeOrigin: false } },
  },
});

