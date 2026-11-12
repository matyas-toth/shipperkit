import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import { sidebar } from "./src/data/docs-navigation.mjs";

export default defineConfig({
  site: "https://shipperkit.dev",
  base: "/docs",
  trailingSlash: "always",
  srcDir: "./docs/src",
  publicDir: "./docs/public",
  outDir: "./docs-dist",
  devToolbar: { enabled: false },
  server: { port: 4322 },
  integrations: [
    starlight({
      title: "Shipper Docs",
      description:
        "Documentation for Shipper, the self-hosted deployment console. Install, deploy, operate, secure, and recover your server.",
      logo: {
        src: "./docs/src/assets/shipper.svg",
        alt: "Shipper",
      },
      favicon: "/favicon.svg",
      components: {
        Head: "./docs/src/components/DocsHead.astro",
        SiteTitle: "./docs/src/components/DocsSiteTitle.astro",
        ThemeProvider: "./docs/src/components/DocsThemeProvider.astro",
        ThemeSelect: "./docs/src/components/DocsThemeSelect.astro",
      },
      sidebar,
      social: [
        {
          icon: "github",
          label: "Shipper on GitHub",
          href: "https://github.com/matyas-toth/shipperkit",
        },
      ],
      editLink: {
        baseUrl:
          "https://github.com/matyas-toth/shipperkit/edit/master/landing/",
      },
      lastUpdated: false,
      pagination: true,
      credits: false,
      expressiveCode: {
        defaultProps: { frame: "code" },
      },
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
      head: [
        {
          tag: "meta",
          attrs: { property: "og:image", content: "https://shipperkit.dev/og.png" },
        },
        {
          tag: "meta",
          attrs: { property: "og:image:width", content: "1200" },
        },
        {
          tag: "meta",
          attrs: { property: "og:image:height", content: "630" },
        },
        {
          tag: "meta",
          attrs: {
            property: "og:image:alt",
            content: "Shipper self-hosted deployment console",
          },
        },
        {
          tag: "meta",
          attrs: { name: "twitter:image", content: "https://shipperkit.dev/og.png" },
        },
        {
          tag: "meta",
          attrs: { name: "theme-color", content: "#050606" },
        },
      ],
    }),
  ],
});
