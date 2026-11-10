import type { APIRoute } from "astro";

export const prerender = true;

export const GET: APIRoute = () => {
  const urls = [
    { loc: "https://shipperkit.dev/", changefreq: "weekly", priority: "1.0" },
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
