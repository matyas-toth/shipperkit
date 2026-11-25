import rss from "@astrojs/rss";
import type { APIRoute } from "astro";
import { posts, SITE } from "@/lib/content";

export const prerender = true;

export const GET: APIRoute = async () =>
  rss({
    title: "ShipperKit blog",
    description: "How Shipper works and guides for hosting on your own server.",
    site: SITE,
    items: (await posts()).map((post) => ({
      title: post.data.title,
      description: post.data.description,
      pubDate: post.data.date,
      link: `/blog/${post.id}/`,
    })),
  });
