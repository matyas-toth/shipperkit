import "server-only";
import { cacheLife } from "next/cache";
import { db } from "./db";
import { compareVersions } from "./format";

export const RANGES = { "30d": 30, "90d": 90, "1y": 365 } as const;
export type RangeKey = keyof typeof RANGES;
export const ACTIVE_DAYS = Math.max(1, Number(process.env.ACTIVE_DAYS) || 3);
const VERSION_SERIES = 6;

export type Slice = { label: string; value: number };
export type DayPoint = { day: string; active: number; new: number; upgrades: number; versions: Record<string, number> };

export type Overview = {
  generatedAt: number;
  kpis: { active: number; activeBefore: number; total: number; new: number; inactive: number; retention: number | null; applications: number; deployments: number };
  days: DayPoint[];
  versionKeys: string[];
  versions: Slice[];
  hostOs: Slice[];
  arch: Slice[];
  install: Slice[];
  channel: Slice[];
  docker: Slice[];
  cpus: Slice[];
  memory: Slice[];
  countries: Slice[];
  methods: Slice[];
  databases: Slice[];
  catalog: Slice[];
  sources: Slice[];
  connections: Slice[];
  features: Slice[];
  appsPerInstance: Slice[];
  lifetime: Slice[];
  usage: { applications: number; services: number; domains: number; users: number; deployments: number; failed: number; rollbacks: number; medianDeploy: number | null; backups: number; tasks: number };
};

const activeWhere = () => ({ lastSeenAt: { gt: new Date(Date.now() - ACTIVE_DAYS * 86_400_000) } });

function sortSlices(map: Map<string, number>, limit = 8): Slice[] {
  const sorted = [...map.entries()].filter(([, value]) => value > 0).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  if (sorted.length <= limit) return sorted.map(([label, value]) => ({ label, value }));
  const head = sorted.slice(0, limit - 1).map(([label, value]) => ({ label, value }));
  const rest = sorted.slice(limit - 1).reduce((sum, [, value]) => sum + value, 0);
  return [...head, { label: "Other", value: rest }];
}

function add(map: Map<string, number>, key: string, value: number) {
  map.set(key, (map.get(key) ?? 0) + value);
}

export function hostFamily(value: string) {
  const text = value.trim();
  if (!text) return "Unknown";
  const debian = text.match(/^Debian GNU\/Linux (\d+)/);
  if (debian) return `Debian ${debian[1]}`;
  const versioned = text.match(/^([A-Za-z][A-Za-z ]*?) (\d+(?:\.\d+)?)/);
  if (versioned) return `${versioned[1]} ${versioned[2]}`;
  return text.replace(/\s*\(.*\)$/, "");
}

function cpuBucket(cpus: number) {
  if (cpus <= 0) return "Unknown";
  if (cpus <= 1) return "1";
  if (cpus <= 2) return "2";
  if (cpus <= 4) return "3–4";
  if (cpus <= 8) return "5–8";
  if (cpus <= 16) return "9–16";
  return "17+";
}
const CPU_ORDER = ["1", "2", "3–4", "5–8", "9–16", "17+", "Unknown"];

function memoryBucket(mb: number) {
  if (mb <= 0) return "Unknown";
  const gb = mb / 1024;
  if (gb < 2.5) return "≤ 2 GB";
  if (gb < 4.5) return "4 GB";
  if (gb < 8.5) return "8 GB";
  if (gb < 16.5) return "16 GB";
  if (gb < 32.5) return "32 GB";
  return "64 GB+";
}
const MEMORY_ORDER = ["≤ 2 GB", "4 GB", "8 GB", "16 GB", "32 GB", "64 GB+", "Unknown"];

function appsBucket(apps: number) {
  if (apps <= 0) return "0";
  if (apps === 1) return "1";
  if (apps <= 5) return "2–5";
  if (apps <= 10) return "6–10";
  if (apps <= 25) return "11–25";
  return "26+";
}
const APPS_ORDER = ["0", "1", "2–5", "6–10", "11–25", "26+"];

function ordered(map: Map<string, number>, order: string[]): Slice[] {
  return order.map(label => ({ label, value: map.get(label) ?? 0 })).filter(slice => slice.value > 0 || slice.label !== "Unknown");
}

export async function getOverview(range: RangeKey): Promise<Overview> {
  "use cache";
  cacheLife("minutes");
  const days = RANGES[range];
  const active = activeWhere();

  const [kpiRows, versionRows, newRows, upgradeRows, platform, jsonRows, featureRows, usageRows, lifetimeRows] = await Promise.all([
    db.$queryRaw<{ total: number; active: number; new: number; eligible: number; retained: number }[]>`
      SELECT count(*)::int AS total,
        count(*) FILTER (WHERE last_seen_at > now() - make_interval(days => ${ACTIVE_DAYS}::int))::int AS active,
        count(*) FILTER (WHERE first_seen_at > now() - make_interval(days => ${days}::int))::int AS new,
        count(*) FILTER (WHERE first_seen_at < now() - interval '7 days')::int AS eligible,
        count(*) FILTER (WHERE first_seen_at < now() - interval '7 days' AND last_seen_at >= first_seen_at + interval '7 days')::int AS retained
      FROM instances`,
    // An instance counts as active on a day when it reported within the
    // preceding window, using the version from its most recent report.
    db.$queryRaw<{ day: Date; version: string; count: number }[]>`
      WITH per_day AS (
        SELECT DISTINCT ON (instance_id, date_trunc('day', received_at)) instance_id, date_trunc('day', received_at) AS day, version
        FROM reports
        WHERE received_at >= date_trunc('day', now()) - make_interval(days => ${days + ACTIVE_DAYS}::int)
        ORDER BY instance_id, date_trunc('day', received_at), received_at DESC
      ), covered AS (
        SELECT DISTINCT ON (p.instance_id, c.day) c.day, p.version
        FROM per_day p
        CROSS JOIN LATERAL generate_series(p.day, p.day + make_interval(days => ${ACTIVE_DAYS - 1}::int), interval '1 day') AS c(day)
        WHERE c.day >= date_trunc('day', now()) - make_interval(days => ${days - 1}::int) AND c.day <= date_trunc('day', now())
        ORDER BY p.instance_id, c.day, p.day DESC
      )
      SELECT day, version, count(*)::int AS count FROM covered GROUP BY day, version`,
    db.$queryRaw<{ day: Date; count: number }[]>`
      SELECT date_trunc('day', first_seen_at) AS day, count(*)::int AS count FROM instances
      WHERE first_seen_at >= date_trunc('day', now()) - make_interval(days => ${days - 1}::int)
      GROUP BY 1`,
    db.$queryRaw<{ day: Date; count: number }[]>`
      SELECT date_trunc('day', at) AS day, count(*)::int AS count FROM upgrades
      WHERE at >= date_trunc('day', now()) - make_interval(days => ${days - 1}::int)
      GROUP BY 1`,
    db.instance.findMany({
      where: active,
      select: { version: true, channel: true, install: true, arch: true, hostOs: true, docker: true, cpus: true, memoryMb: true, country: true, applications: true },
    }),
    db.$queryRaw<{ dim: string; key: string; value: number }[]>`
      WITH a AS (SELECT latest FROM instances WHERE last_seen_at > now() - make_interval(days => ${ACTIVE_DAYS}::int))
      SELECT dim, e.key, sum(e.value::int)::int AS value
      FROM a, LATERAL (VALUES ('methods'), ('databases'), ('catalog'), ('sources'), ('connections')) AS d(dim),
        LATERAL jsonb_each_text(COALESCE(a.latest -> d.dim, '{}'::jsonb)) e
      GROUP BY dim, e.key`,
    db.$queryRaw<{ key: string; value: number }[]>`
      SELECT f AS key, count(*)::int AS value
      FROM instances, jsonb_array_elements_text(COALESCE(latest -> 'features', '[]'::jsonb)) f
      WHERE last_seen_at > now() - make_interval(days => ${ACTIVE_DAYS}::int)
      GROUP BY f`,
    db.$queryRaw<{ applications: number; services: number; domains: number; users: number; deployments: number; failed: number; rollbacks: number; median: number | null; backups: number; tasks: number }[]>`
      SELECT
        COALESCE(sum((latest #>> '{usage,applications}')::int), 0)::int AS applications,
        COALESCE(sum((latest #>> '{usage,services}')::int), 0)::int AS services,
        COALESCE(sum((latest #>> '{usage,domains}')::int), 0)::int AS domains,
        COALESCE(sum((latest #>> '{usage,users}')::int), 0)::int AS users,
        COALESCE(sum((latest #>> '{usage,deployments}')::int), 0)::int AS deployments,
        COALESCE(sum((latest #>> '{usage,failedDeployments}')::int), 0)::int AS failed,
        COALESCE(sum((latest #>> '{usage,rollbacks}')::int), 0)::int AS rollbacks,
        percentile_cont(0.5) WITHIN GROUP (ORDER BY (latest #>> '{usage,deployMedianSeconds}')::int)
          FILTER (WHERE (latest #>> '{usage,deployMedianSeconds}')::int > 0) AS median,
        count(*) FILTER (WHERE (latest #>> '{usage,backupPlans}')::int > 0)::int AS backups,
        count(*) FILTER (WHERE (latest #>> '{usage,tasks}')::int > 0)::int AS tasks
      FROM instances WHERE last_seen_at > now() - make_interval(days => ${ACTIVE_DAYS}::int)`,
    db.$queryRaw<{ bucket: string; count: number }[]>`
      SELECT CASE
          WHEN last_seen_at - first_seen_at < interval '1 day' THEN '< 1 day'
          WHEN last_seen_at - first_seen_at < interval '7 days' THEN '1–7 days'
          WHEN last_seen_at - first_seen_at < interval '30 days' THEN '1–4 wk'
          WHEN last_seen_at - first_seen_at < interval '90 days' THEN '1–3 mo'
          ELSE '3 mo+' END AS bucket,
        count(*)::int AS count
      FROM instances GROUP BY 1`,
  ]);

  const dayKey = (value: Date) => value.toISOString().slice(0, 10);
  const series = new Map<string, DayPoint>();
  const start = new Date();
  start.setUTCHours(0, 0, 0, 0);
  for (let i = days - 1; i >= 0; i--) {
    const key = dayKey(new Date(start.getTime() - i * 86_400_000));
    series.set(key, { day: key, active: 0, new: 0, upgrades: 0, versions: {} });
  }
  const versionTotals = new Map<string, number>();
  const platformVersions = new Map<string, number>();
  for (const row of platform) add(platformVersions, row.version, 1);
  for (const row of versionRows) {
    const point = series.get(dayKey(row.day));
    if (!point) continue;
    point.active += row.count;
    point.versions[row.version] = (point.versions[row.version] ?? 0) + row.count;
    add(versionTotals, row.version, row.count);
  }
  // The newest versions get their own series so adoption of a release is
  // visible. Older ones fold into Other.
  const versionKeys = [...new Set([...versionTotals.keys(), ...platformVersions.keys()])].sort(compareVersions).reverse().slice(0, VERSION_SERIES);
  for (const point of series.values()) {
    const folded: Record<string, number> = {};
    for (const [version, count] of Object.entries(point.versions)) {
      const key = versionKeys.includes(version) ? version : "Other";
      folded[key] = (folded[key] ?? 0) + count;
    }
    point.versions = folded;
  }
  if ([...series.values()].some(point => point.versions.Other)) versionKeys.push("Other");
  for (const row of newRows) {
    const point = series.get(dayKey(row.day));
    if (point) point.new = row.count;
  }
  for (const row of upgradeRows) {
    const point = series.get(dayKey(row.day));
    if (point) point.upgrades = row.count;
  }
  const points = [...series.values()];

  const maps = { hostOs: new Map<string, number>(), arch: new Map<string, number>(), install: new Map<string, number>(), channel: new Map<string, number>(), docker: new Map<string, number>(), cpus: new Map<string, number>(), memory: new Map<string, number>(), countries: new Map<string, number>(), apps: new Map<string, number>() };
  for (const row of platform) {
    add(maps.hostOs, hostFamily(row.hostOs), 1);
    add(maps.arch, row.arch, 1);
    add(maps.install, row.install === "managed" ? "Installer" : "Manual", 1);
    add(maps.channel, row.channel === "nightly" ? "Nightly" : "Main", 1);
    add(maps.docker, row.docker ? row.docker.split(".")[0] : "Unknown", 1);
    add(maps.cpus, cpuBucket(row.cpus), 1);
    add(maps.memory, memoryBucket(row.memoryMb), 1);
    add(maps.countries, row.country ?? "Unknown", 1);
    add(maps.apps, appsBucket(row.applications), 1);
  }
  const dims: Record<string, Map<string, number>> = { methods: new Map(), databases: new Map(), catalog: new Map(), sources: new Map(), connections: new Map() };
  for (const row of jsonRows) if (dims[row.dim]) add(dims[row.dim], row.key, row.value);

  const kpi = kpiRows[0];
  const usage = usageRows[0];
  const lifetime = new Map(lifetimeRows.map(row => [row.bucket, row.count]));
  const before = points.length > 7 ? points[points.length - 8].active : 0;

  return {
    generatedAt: Date.now(),
    kpis: {
      active: kpi.active,
      activeBefore: before,
      total: kpi.total,
      new: kpi.new,
      inactive: kpi.total - kpi.active,
      retention: kpi.eligible ? kpi.retained / kpi.eligible : null,
      applications: usage.applications,
      deployments: usage.deployments,
    },
    days: points,
    versionKeys,
    versions: [...platformVersions.entries()].sort((a, b) => compareVersions(b[0], a[0])).map(([label, value]) => ({ label, value })),
    hostOs: sortSlices(maps.hostOs),
    arch: sortSlices(maps.arch, 4),
    install: sortSlices(maps.install, 3),
    channel: sortSlices(maps.channel, 3),
    docker: [...maps.docker.entries()].sort((a, b) => Number(b[0]) - Number(a[0])).slice(0, 6).map(([label, value]) => ({ label: label === "Unknown" ? label : `Docker ${label}`, value })),
    cpus: ordered(maps.cpus, CPU_ORDER),
    memory: ordered(maps.memory, MEMORY_ORDER),
    countries: sortSlices(maps.countries, 10),
    methods: sortSlices(dims.methods),
    databases: sortSlices(dims.databases),
    catalog: sortSlices(dims.catalog, 10),
    sources: sortSlices(dims.sources),
    connections: sortSlices(dims.connections),
    features: sortSlices(new Map(featureRows.map(row => [row.key, row.value])), 16),
    appsPerInstance: APPS_ORDER.map(label => ({ label, value: maps.apps.get(label) ?? 0 })),
    lifetime: ["< 1 day", "1–7 days", "1–4 wk", "1–3 mo", "3 mo+"].map(label => ({ label, value: lifetime.get(label) ?? 0 })),
    usage: {
      applications: usage.applications,
      services: usage.services,
      domains: usage.domains,
      users: usage.users,
      deployments: usage.deployments,
      failed: usage.failed,
      rollbacks: usage.rollbacks,
      medianDeploy: usage.median == null ? null : Number(usage.median),
      backups: usage.backups,
      tasks: usage.tasks,
    },
  };
}

export type InstanceRow = { id: string; active: boolean; version: string; hostOs: string; arch: string; install: string; channel: string; applications: number; country: string | null; reports: number; firstSeenAt: number; lastSeenAt: number };
export type InstanceFilter = "all" | "active" | "inactive";
export const PAGE_SIZE = 50;

export async function getInstances(filter: InstanceFilter, page: number) {
  "use cache";
  cacheLife("minutes");
  const cutoff = new Date(Date.now() - ACTIVE_DAYS * 86_400_000);
  const where = filter === "active" ? { lastSeenAt: { gt: cutoff } } : filter === "inactive" ? { lastSeenAt: { lte: cutoff } } : {};
  const [rows, count] = await Promise.all([
    db.instance.findMany({
      where,
      orderBy: [{ lastSeenAt: "desc" }, { id: "asc" }],
      skip: (page - 1) * PAGE_SIZE,
      take: PAGE_SIZE,
      select: { id: true, version: true, hostOs: true, arch: true, install: true, channel: true, applications: true, country: true, reportCount: true, firstSeenAt: true, lastSeenAt: true },
    }),
    db.instance.count({ where }),
  ]);
  return {
    count,
    generatedAt: Date.now(),
    rows: rows.map((row): InstanceRow => ({
      id: row.id,
      active: row.lastSeenAt > cutoff,
      version: row.version,
      hostOs: hostFamily(row.hostOs),
      arch: row.arch,
      install: row.install === "managed" ? "Installer" : "Manual",
      channel: row.channel === "nightly" ? "Nightly" : "Main",
      applications: row.applications,
      country: row.country,
      reports: row.reportCount,
      firstSeenAt: row.firstSeenAt.getTime(),
      lastSeenAt: row.lastSeenAt.getTime(),
    })),
  };
}
