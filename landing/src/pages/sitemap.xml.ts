import type { APIRoute } from "astro";
import { docRoutes } from "../data/docs-navigation.mjs";

export const prerender = true;

export const GET: APIRoute = () => {
  const urls = [
    { loc: "https://shipperkit.dev/", changefreq: "weekly", priority: "1.0" },
    ...docRoutes.map((page: { slug: string; priority: string; changefreq: string }) => ({
      loc: page.slug === "index"
        ? "https://shipperkit.dev/docs/"
        : `https://shipperkit.dev/docs/${page.slug}/`,
      changefreq: page.changefreq,
      priority: page.priority,
    })),
  ];
  const body = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.map((item) => `  <url>\n    <loc>${item.loc}</loc>\n    <changefreq>${item.changefreq}</changefreq>\n    <priority>${item.priority}</priority>\n  </url>`).join("\n")}
</urlset>
`;
  return new Response(body, {
    headers: {
      "Content-Type": "application/xml; charset=utf-8",
      "Cache-Control": "public, max-age=3600",
    },
  });
};
