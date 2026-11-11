import type { APIRoute } from "astro";
import { getCollection } from "astro:content";

export const prerender = true;

type DocEntry = {
  id: string;
  data: { title: string; description?: string };
};

export const GET: APIRoute = async () => {
  const entries = (await (getCollection as unknown as (name: string) => Promise<DocEntry[]>)("docs")) as DocEntry[];
  const ordered = entries.sort((a, b) => a.id.localeCompare(b.id));
  const lines = [
    "# Shipper documentation",
    "",
    "> Install, deploy, back up, recover, and troubleshoot Shipper on a server you own.",
    "",
    "Canonical docs: https://shipperkit.dev/docs/",
    "",
  ];
  for (const entry of ordered) {
    const path = entry.id === "index" ? "" : `${entry.id}/`;
    lines.push(
      `- [${entry.data.title}](https://shipperkit.dev/docs/${path}): ${entry.data.description ?? ""}`,
    );
  }
  lines.push("");
  return new Response(lines.join("\n"), {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "public, max-age=3600",
    },
  });
};
