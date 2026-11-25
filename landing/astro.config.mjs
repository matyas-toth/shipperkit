import { defineConfig } from "astro/config";
import mdx from "@astrojs/mdx";
import react from "@astrojs/react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://shipperkit.dev",
  trailingSlash: "ignore",
  integrations: [react(), mdx()],
  markdown: {
    shikiConfig: { theme: "github-dark-default", wrap: false },
  },
  devToolbar: { enabled: false },
  vite: {
    plugins: [tailwindcss()],
  },
});
