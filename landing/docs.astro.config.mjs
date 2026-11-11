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
        "Install, deploy, back up, recover, and troubleshoot Shipper on a server you own. The operational documentation for the self-hosted deployment console.",
      logo: {
        src: "./docs/src/assets/shipper.svg",
        alt: "Shipper",
      },
      favicon: "/favicon.svg",
      customCss: ["./docs/src/styles/docs.css"],
      components: {
        Head: "./docs/src/components/DocsHead.astro",
        Header: "./docs/src/components/DocsHeader.astro",
        SiteTitle: "./docs/src/components/DocsSiteTitle.astro",
        Hero: "./docs/src/components/DocsHero.astro",
        Footer: "./docs/src/components/DocsFooter.astro",
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
