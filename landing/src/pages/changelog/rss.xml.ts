import rss from "@astrojs/rss";
import type { APIRoute } from "astro";
import { releases, SITE } from "@/lib/content";

export const prerender = true;

export const GET: APIRoute = async () =>
  rss({
    title: "ShipperKit changelog",
    description: "What changed in each Shipper release.",
    site: SITE,
    items: (await releases()).map((entry) => ({
      title: `${entry.data.version}: ${entry.data.title}`,
      description: entry.data.description,
      pubDate: entry.data.date,
      link: `/changelog/${entry.id}/`,
    })),
  });
