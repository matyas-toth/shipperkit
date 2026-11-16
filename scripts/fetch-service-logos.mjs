// Downloads service logos into the SPA public directory. Logos remain the
// property of their projects; the manifest only stores a local path.
//
// Usage:
//   node scripts/fetch-service-logos.mjs --source coolify-templates.json
//
// The Coolify template bundle is the same input used by cmd/cataloggen.
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

const args = new Map();
for (let index = 2; index < process.argv.length; index += 2) {
  args.set(process.argv[index], process.argv[index + 1]);
}
const sourcePath = args.get("--source");
if (!sourcePath) {
  console.error("fetch-service-logos: --source is required");
  process.exit(2);
}
const catalogPath = args.get("--catalog") ?? "internal/catalog/catalog.json";
const outDir = args.get("--out") ?? "web/public/service-logos";
const coolifyRoot = "https://raw.githubusercontent.com/coollabsio/coolify/main/public/";
const simpleIcons = {
  postgresql: "postgresql", mysql: "mysql", mariadb: "mariadb", mongodb: "mongodb",
  redis: "redis", clickhouse: "clickhouse", prometheus: "prometheus", loki: "grafana",
  minio: "minio", outline: "outline", calcom: "caldotcom", wordpress: "wordpress",
  influxdb: "influxdb", peertube: "peertube", "obsidian-livesync": "obsidian",
};

const source = JSON.parse(await readFile(sourcePath, "utf8"));
const catalog = JSON.parse(await readFile(catalogPath, "utf8"));
await mkdir(outDir, { recursive: true });

const tasks = catalog.services.map(service => async () => {
  const fileName = service.logo?.split("/").pop();
  if (!fileName) return null;
  const urls = [];
  if (source[service.id]?.logo) urls.push(coolifyRoot + encodeURI(source[service.id].logo));
  if (simpleIcons[service.id]) urls.push("https://cdn.simpleicons.org/" + simpleIcons[service.id]);
  if (!urls.length) return { id: service.id, skipped: true };
  let lastFailure = "no source";
  for (const url of urls) {
    try {
      const response = await fetch(url);
      if (!response.ok) { lastFailure = response.status; continue; }
      await writeFile(join(outDir, fileName), Buffer.from(await response.arrayBuffer()));
      return { id: service.id, url, fileName };
    } catch (error) {
      lastFailure = String(error);
    }
  }
  return { id: service.id, urls, failed: lastFailure };
});

const results = [];
let cursor = 0;
async function worker() {
  while (cursor < tasks.length) {
    const task = tasks[cursor++];
    results.push(await task());
  }
}
await Promise.all(Array.from({ length: 10 }, worker));

const failed = results.filter(result => result?.failed || result?.skipped);
console.log(`fetch-service-logos: saved ${results.length - failed.length} logos to ${outDir}`);
if (failed.length) {
  console.warn("fetch-service-logos: missing logos");
  for (const result of failed) console.warn(" ", result.id, result.failed ?? "no source");
}
