import type { APIRoute } from "astro";
import { docRoutes } from "../data/docs-navigation.mjs";
import { comparisons, posts, releases, isoDate, SITE } from "@/lib/content";

export const prerender = true;

type Url = { loc: string; changefreq: string; priority: string; lastmod?: string };

export const GET: APIRoute = async () => {
  const [blog, compare, changelog] = await Promise.all([posts(), comparisons(), releases()]);
  const urls: Url[] = [
    { loc: `${SITE}/`, changefreq: "weekly", priority: "1.0" },
    ...docRoutes.map((page: { slug: string; priority: string; changefreq: string }) => ({
      loc: page.slug === "index" ? `${SITE}/docs/` : `${SITE}/docs/${page.slug}/`,
      changefreq: page.changefreq,
      priority: page.priority,
    })),
    { loc: `${SITE}/vs/`, changefreq: "monthly", priority: "0.7" },
    ...compare.map((e) => ({ loc: `${SITE}/vs/${e.id}/`, changefreq: "monthly", priority: "0.8", lastmod: isoDate(e.data.checked) })),
    { loc: `${SITE}/blog/`, changefreq: "weekly", priority: "0.7", lastmod: blog[0] && isoDate(blog[0].data.date) },
    ...blog.map((e) => ({ loc: `${SITE}/blog/${e.id}/`, changefreq: "yearly", priority: "0.7", lastmod: isoDate(e.data.date) })),
    { loc: `${SITE}/changelog/`, changefreq: "weekly", priority: "0.6", lastmod: changelog[0] && isoDate(changelog[0].data.date) },
    ...changelog.map((e) => ({ loc: `${SITE}/changelog/${e.id}/`, changefreq: "yearly", priority: "0.4", lastmod: isoDate(e.data.date) })),
  ];
  const body = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.map((item) => `  <url>\n    <loc>${item.loc}</loc>\n${item.lastmod ? `    <lastmod>${item.lastmod}</lastmod>\n` : ""}    <changefreq>${item.changefreq}</changefreq>\n    <priority>${item.priority}</priority>\n  </url>`).join("\n")}
</urlset>
`;
  return new Response(body, {
    headers: {
      "Content-Type": "application/xml; charset=utf-8",
      "Cache-Control": "public, max-age=3600",
    },
  });
};
