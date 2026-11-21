// Fills a local database with synthetic instances so the dashboard can be
// developed without real traffic. It refuses to touch non-local databases.
import { PrismaPg } from "@prisma/adapter-pg";
import { PrismaClient } from "../src/generated/prisma/client";
import type { Report } from "../src/lib/report";

const url = process.env.DATABASE_URL ?? "";
if (!/@(localhost|127\.0\.0\.1)(:\d+)?\//.test(url)) {
  console.error("Seeding only runs against a local DATABASE_URL.");
  process.exit(1);
}

const db = new PrismaClient({ adapter: new PrismaPg({ connectionString: url }) });
const DAY = 86_400_000;
const DAYS = 200;

let state = 42;
const random = () => ((state = (state * 1664525 + 1013904223) % 4294967296) / 4294967296);
const pick = <T,>(items: [T, number][]) => {
  const total = items.reduce((sum, [, weight]) => sum + weight, 0);
  let roll = random() * total;
  for (const [item, weight] of items) if ((roll -= weight) <= 0) return item;
  return items[items.length - 1][0];
};
const hex = (length: number) => Array.from({ length }, () => Math.floor(random() * 16).toString(16)).join("");

const RELEASES: [string, number][] = [
  ["1.0.0-rc.20", 0], ["1.0.0-rc.24", 20], ["1.0.0-rc.28", 50], ["1.0.0-rc.30", 85], ["1.0.0-rc.32", 130], ["1.0.0-rc.33", 160], ["1.0.0-rc.34", 185],
];
const versionAt = (daysAgo: number, lag: number) => {
  const day = DAYS - daysAgo - lag;
  let current = RELEASES[0][0];
  for (const [version, released] of RELEASES) if (day >= released) current = version;
  return current;
};

async function main() {
  await db.$transaction([db.upgrade.deleteMany(), db.report.deleteMany(), db.instance.deleteMany()]);
  const now = Date.now();
  const instances = [];
  const reports = [];
  const upgrades = [];
  for (let n = 0; n < 420; n++) {
    // Growth accelerates toward today.
    const start = Math.floor(DAYS * Math.sqrt(random()));
    const firstDaysAgo = DAYS - start;
    const churn = pick([["trial", 30], ["leaves", 15], ["stays", 55]] as [string, number][]);
    const lifetime = churn === "trial" ? random() * 2 : churn === "leaves" ? 3 + random() * 60 : firstDaysAgo;
    const lastDaysAgo = Math.max(0, firstDaysAgo - lifetime);
    const lag = Math.floor(random() * 25);
    const id = hex(32);
    const host = pick<[string, string, number, number]>([
      [["Ubuntu 24.04.1 LTS", "amd64", 4, 8192], 34], [["Ubuntu 22.04.5 LTS", "amd64", 2, 4096], 18], [["Debian GNU/Linux 12 (bookworm)", "amd64", 2, 4096], 16],
      [["Ubuntu 24.04.1 LTS", "arm64", 4, 24576], 10], [["Docker Desktop", "arm64", 8, 16384], 9], [["Fedora Linux 41 (Server Edition)", "amd64", 8, 32768], 4],
      [["Rocky Linux 9.5 (Blue Onyx)", "amd64", 4, 16384], 3], [["Alpine Linux v3.21", "amd64", 1, 2048], 3], [["Raspbian GNU/Linux 12 (bookworm)", "arm64", 4, 8192], 3],
    ]);
    const country = pick([["US", 24], ["DE", 14], ["GB", 8], ["FR", 7], ["NL", 6], ["HU", 5], ["IN", 6], ["BR", 5], ["CA", 5], ["PL", 4], ["JP", 3], ["AU", 3], [null, 4]] as [string | null, number][]);
    const install = random() < 0.82 ? "managed" : "manual";
    const channel = random() < 0.12 ? "nightly" : "main";
    const autoUpdate = random() < 0.45;
    const docker = pick([["29.1.2", 40], ["28.5.1", 30], ["27.4.1", 18], ["26.1.4", 8], ["25.0.5", 4]] as [string, number][]);
    const apps = churn === "trial" ? Math.floor(random() * 2) : Math.floor(Math.pow(random(), 2) * 30) + 1;
    const methods: Record<string, number> = {};
    const sources: Record<string, number> = {};
    const databases: Record<string, number> = {};
    const catalog: Record<string, number> = {};
    for (let a = 0; a < apps; a++) {
      const method = pick([["dockerfile", 30], ["compose", 22], ["image", 20], ["static", 12], ["nixpacks", 10], ["inline", 6]] as [string, number][]);
      methods[method] = (methods[method] ?? 0) + 1;
      const source = pick([["github", 45], ["git", 20], ["image", 18], ["local", 10], ["catalog", 7]] as [string, number][]);
      sources[source] = (sources[source] ?? 0) + 1;
      if (random() < 0.3) {
        const engine = pick([["postgres", 55], ["redis", 20], ["mysql", 12], ["mongodb", 8], ["mariadb", 5]] as [string, number][]);
        databases[engine] = (databases[engine] ?? 0) + 1;
      }
      if (random() < 0.18) {
        const service = pick([["plausible", 18], ["n8n", 16], ["uptime-kuma", 14], ["umami", 10], ["minio", 9], ["gitea", 8], ["ghost", 7], ["metabase", 6], ["vaultwarden", 6], ["grafana", 5], ["pocketbase", 4], ["listmonk", 3]] as [string, number][]);
        catalog[service] = (catalog[service] ?? 0) + 1;
      }
    }
    const features = ["auto_deploy", "backups", "tasks", "two_factor", "sso", "team", "https", "dashboard_https", "volumes", "hooks", "access_tokens"].filter(
      (_, i) => random() < [0.55, 0.38, 0.22, 0.31, 0.12, 0.2, 0.7, 0.62, 0.5, 0.08, 0.15][i],
    );
    if (autoUpdate) features.push("auto_update");
    let previous = "";
    let last: Report | null = null;
    let count = 0;
    for (let d = firstDaysAgo; d >= lastDaysAgo; d--) {
      if (random() < 0.06 && d !== firstDaysAgo) continue;
      const at = new Date(now - d * DAY - Math.floor(random() * 6) * 3_600_000);
      const version = channel === "nightly" ? versionAt(d, 0) : versionAt(d, lag);
      const deployments = Math.floor(random() * apps * 1.5);
      const report: Report = {
        schema: 1, id, version, channel: channel as Report["channel"], autoUpdate, install: install as Report["install"], uptimeHours: Math.floor(random() * 400),
        platform: { os: "linux", arch: host[1], hostOs: host[0], docker, cpus: host[2], memoryMb: host[3] },
        usage: {
          applications: apps, services: apps + Math.floor(random() * apps), domains: Math.floor(apps * 1.3), users: 1 + Math.floor(random() * 4),
          deployments, failedDeployments: Math.floor(deployments * random() * 0.15), rollbacks: random() < 0.1 ? 1 : 0,
          deployMedianSeconds: 20 + Math.floor(random() * 140), backupPlans: features.includes("backups") ? 2 : 0, tasks: features.includes("tasks") ? 3 : 0,
          accessTokens: features.includes("access_tokens") ? 1 : 0,
        },
        methods, sources, databases, catalog, connections: random() < 0.6 ? { github: 1 } : {}, features: [...features].sort(),
      };
      if (previous && previous !== version) upgrades.push({ instanceId: id, fromVersion: previous, toVersion: version, at });
      previous = version;
      reports.push({ instanceId: id, receivedAt: at, version, payload: report });
      last = report;
      count++;
    }
    if (!last) continue;
    instances.push({
      id, firstSeenAt: new Date(now - firstDaysAgo * DAY), lastSeenAt: reports[reports.length - 1].receivedAt, reportCount: count,
      version: last.version, channel, install, os: "linux", arch: host[1], hostOs: host[0], docker, cpus: host[2], memoryMb: host[3],
      country, applications: apps, latest: last,
    });
  }
  await db.instance.createMany({ data: instances });
  for (let i = 0; i < reports.length; i += 5000) await db.report.createMany({ data: reports.slice(i, i + 5000) });
  await db.upgrade.createMany({ data: upgrades });
  console.log(`Seeded ${instances.length} instances, ${reports.length} reports and ${upgrades.length} upgrades.`);
}

main().finally(() => db.$disconnect());
