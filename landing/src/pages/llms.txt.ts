import type { APIRoute } from "astro";
import { docGroups } from "../data/docs-navigation.mjs";
import { INSTALL, FEATURES } from "@/components/parts";
import { comparisons, posts, releases, SITE } from "@/lib/content";

/* llms.txt (https://llmstxt.org): a plain map of the site for language models
   and agents, generated from the same sources as the pages so it never drifts. */

export const prerender = true;

const docSources = import.meta.glob<string>("../../docs/src/content/docs/*.mdx", { eager: true, query: "?raw", import: "default" });
const frontmatter = (slug: string) => {
  const raw = Object.entries(docSources).find(([path]) => path.endsWith(`/${slug}.mdx`))?.[1] ?? "";
  const field = (name: string) => raw.match(new RegExp(`^${name}:\s*(.+)$`, "m"))?.[1]?.trim().replace(/^["']|["']$/g, "");
  return { title: field("title"), description: field("description") };
};
const line = (title: string, url: string, description?: string) => `- [${title}](${url})${description ? `: ${description}` : ""}`;

export const GET: APIRoute = async () => {
  const docs = docGroups.map((group: { label: string; items: { slug: string; label: string }[] }) => [
    `### ${group.label}`,
    ...group.items.map(({ slug, label }) => {
      const meta = frontmatter(slug);
      return line(meta.title ?? label, slug === "index" ? `${SITE}/docs/` : `${SITE}/docs/${slug}/`, meta.description);
    }),
  ].join("\n"));

  const body = [
    "# ShipperKit",
    "",
    "> Shipper (ShipperKit) is a free, MIT licensed, self-hosted platform that turns one Ubuntu or Debian server into a hosting platform. It deploys apps, databases and self-hosted services with automatic HTTPS, releases that fail safely, rollbacks and scheduled backups.",
    "",
    `Install on Ubuntu 22.04/24.04 or Debian 12/13 (x86-64 or ARM64, 1 GiB RAM, ports 80, 443 and 8080 open): \`${INSTALL}\``,
    "",
    "The `shipper` CLI and the v1 API use scoped, expiring tokens, so a coding agent can deploy, read logs and diagnose failures without SSH access. Every CLI command has a `--json` form.",
    "",
    "## Features",
    "",
    ...FEATURES.map((f) => `- **${f.title}:** ${f.body}`),
    "",
    "## Docs",
    "",
    docs.join("\n\n"),
    "",
    "## Comparisons",
    "",
    ...(await comparisons()).map((e) => line(`ShipperKit vs ${e.data.name}`, `${SITE}/vs/${e.id}/`, e.data.description)),
    "",
    "## Blog",
    "",
    ...(await posts()).map((e) => line(e.data.title, `${SITE}/blog/${e.id}/`, e.data.description)),
    "",
    "## Changelog",
    "",
    ...(await releases()).map((e) => line(`${e.data.version}: ${e.data.title}`, `${SITE}/changelog/${e.id}/`, e.data.description)),
    "",
    "## Optional",
    "",
    line("Source code", "https://github.com/matyas-toth/shipperkit", "Go control plane, SolidJS dashboard, SQLite state, Traefik routing"),
    line("CLI reference", "https://github.com/matyas-toth/shipperkit/blob/master/cli/README.md"),
    line("All releases", "https://github.com/matyas-toth/shipperkit/releases"),
    "",
  ].join("\n");

  return new Response(body, { headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "public, max-age=3600" } });
};
