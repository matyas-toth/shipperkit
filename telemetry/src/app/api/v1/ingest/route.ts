import { NextResponse } from "next/server";
import { db } from "@/lib/db";
import { reportSchema } from "@/lib/report";

const MAX_BYTES = 16 * 1024;
// Reports arrive daily. Anything faster is a retry or a misbehaving client.
const MIN_INTERVAL_MS = 30 * 60 * 1000;

function country(request: Request) {
  // Only the CDN's country guess is kept. The IP address is never read.
  const value = (request.headers.get("x-vercel-ip-country") ?? request.headers.get("cf-ipcountry") ?? "").toUpperCase();
  return /^[A-Z]{2}$/.test(value) && value !== "XX" ? value : null;
}

export async function POST(request: Request) {
  const raw = await request.text();
  if (raw.length > MAX_BYTES) return NextResponse.json({ error: "too_large" }, { status: 413 });
  let json: unknown;
  try {
    json = JSON.parse(raw);
  } catch {
    return NextResponse.json({ error: "invalid_json" }, { status: 400 });
  }
  const parsed = reportSchema.safeParse(json);
  if (!parsed.success) return NextResponse.json({ error: "invalid_report" }, { status: 422 });
  const report = parsed.data;
  const now = new Date();
  const existing = await db.instance.findUnique({ where: { id: report.id }, select: { lastSeenAt: true, version: true, country: true } });
  if (existing && now.getTime() - existing.lastSeenAt.getTime() < MIN_INTERVAL_MS) {
    return NextResponse.json({ ok: true }, { status: 202 });
  }
  const latest = {
    version: report.version,
    channel: report.channel,
    install: report.install,
    os: report.platform.os,
    arch: report.platform.arch,
    hostOs: report.platform.hostOs,
    docker: report.platform.docker,
    cpus: report.platform.cpus,
    memoryMb: report.platform.memoryMb,
    applications: report.usage.applications,
    country: country(request) ?? existing?.country ?? null,
    latest: report,
    lastSeenAt: now,
  };
  await db.$transaction([
    db.instance.upsert({
      where: { id: report.id },
      create: { id: report.id, firstSeenAt: now, ...latest },
      update: { ...latest, reportCount: { increment: 1 } },
    }),
    db.report.create({ data: { instanceId: report.id, receivedAt: now, version: report.version, payload: report } }),
    ...(existing && existing.version !== report.version
      ? [db.upgrade.create({ data: { instanceId: report.id, fromVersion: existing.version, toVersion: report.version, at: now } })]
      : []),
  ]);
  return NextResponse.json({ ok: true }, { status: 202 });
}
