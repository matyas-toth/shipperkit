import { Show, createMemo } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { api } from "../api";
import { databaseIdentity, operationTime } from "../data";
import type { Application, ApplicationDetail } from "../types";
import { Button, Status } from "./Controls";
import { Icon } from "./Icon";
import DatabaseConnection from "./DatabaseConnection";
import DatabaseOperations from "./DatabaseOperations";
import { domainPorts } from "./Networking";
import "./DatabaseResource.css";

type BackupPlan = { id: string; name: string; enabled: boolean; schedule: string; nextAt: number; config: { kind: string } };
type BackupRun = { id: string; state: string; createdAt: number; finishedAt: number; kind?: string };
type Protection = { plans: BackupPlan[]; runs: BackupRun[] };
const engineNames: Record<string, string> = { postgresql: "PostgreSQL", mysql: "MySQL", mariadb: "MariaDB", mongodb: "MongoDB", clickhouse: "ClickHouse", redis: "Redis", sqlite: "SQLite" };

export default function DatabaseOverview(props: {
  application: Application;
  detail?: ApplicationDetail;
  openTerminal: () => void;
  openBackups: () => void;
  openNetworking: () => void;
  openLogs: () => void;
}) {
  const database = () => props.application.config.database;
  const active = () => props.detail?.activeDeployment;
  const info = () => props.detail?.database;
  const protection = createQuery(() => ({
    queryKey: ["applications", props.application.id, "protection"],
    queryFn: () => api<Protection>(`/applications/${props.application.id}/protection`),
  }));
  const plan = createMemo(() => protection.data?.plans.find(entry => entry.config.kind === "database"));
  const latestRun = createMemo(() => [...(protection.data?.runs ?? [])].filter(run => run.kind === "database").sort((a, b) => b.createdAt - a.createdAt)[0]);
  const container = createMemo(() => props.application.containers.find(entry => entry.managed && entry.applicationId === props.application.id));
  const ports = () => props.application.config.networking?.ports ?? [];
  // The serving release decides where the public domain answers.
  const domain = () => active()?.config.database?.domain ?? "";
  const address = () => (info()?.host ?? "app-" + props.application.id) + ":" + (info()?.port ?? 0);
  // Mirrors the strings the server builds: plain inside the server, TLS on the domain.
  const connectionString = (open: boolean) => {
    const engine = database()?.engine;
    if (!engine || !info()) return undefined;
    if (engine === "sqlite") return "sqlite:///home/sqlite/app.db";
    const scheme = engine === "postgresql" ? "postgresql" : engine === "mongodb" ? "mongodb" : engine === "clickhouse" ? "clickhouse" : engine === "redis" ? (open ? "rediss" : "redis") : "mysql";
    const identity = databaseIdentity(props.application.config);
    const user = engine === "redis" ? "default" : identity?.user ?? "app";
    const path = engine === "redis" ? "0" : identity?.database ?? "app";
    const query = engine === "mongodb" ? (open ? "?authSource=admin&directConnection=true&tls=true" : "?authSource=admin&replicaSet=rs0") : open && engine === "postgresql" ? "?sslmode=require" : "";
    const host = open ? domain() + ":" + domainPorts[engine] : address();
    return scheme + "://" + user + ":••••@" + host + "/" + path + query;
  };
  const publicDomain = () => !!domain() && !!domainPorts[database()?.engine ?? ""];
  const publicAccess = () => database()?.engine === "sqlite" ? "File volume" : [publicDomain() ? domain() + ":" + domainPorts[database()!.engine] : "", ...ports().map(port => port.hostPort + "/" + port.protocol + " → " + port.containerPort)].filter(Boolean).join(", ") || "None";
  const lastBackup = () => {
    const run = latestRun();
    if (!run) return "";
    return "Latest " + operationTime(run.finishedAt || run.createdAt);
  };
  const nextBackup = () => {
    const at = plan()?.nextAt;
    if (!at) return "";
    const run = latestRun();
    const previous = run ? (run.finishedAt || run.createdAt) : 0;
    const sameDay = previous > 0 && new Date(previous * 1000).toDateString() === new Date(at * 1000).toDateString();
    const label = sameDay ? new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(at * 1000) : operationTime(at);
    return "Next " + label;
  };
  const backupDetail = () => [lastBackup(), nextBackup()].filter(Boolean).join(" · ");
  return <div class="database-overview">
    <div class="database-overview-primary">
      <Show when={active() && info()} fallback={<div class="database-connection database-connection-empty"><Icon name="database" /><span>Deploy the database to generate its connection string.</span></div>}>
        <DatabaseConnection applicationId={props.application.id} deploymentId={String(active()!.id)} preview={connectionString(false)} publicPreview={publicDomain() ? connectionString(true) : undefined} />
      </Show>
      <DatabaseOperations application={props.application} openTerminal={props.openTerminal} />
    </div>
    <aside class="database-overview-aside">
      <dl class="settings-group database-facts">
        <div class="setting-row"><dt>Engine</dt><dd>{engineNames[database()?.engine ?? ""] ?? database()?.engine} {database()?.version}</dd></div>
        <div class="setting-row"><dt>Storage</dt><dd>Persistent volume</dd></div>
        <div class="setting-row"><dt>Public access</dt><dd class="database-public-access"><Show when={database()?.engine !== "sqlite"} fallback={publicAccess()}><button type="button" class="fact-link" classList={{ mono: publicAccess() !== "None" }} onClick={props.openNetworking}>{publicAccess()}</button></Show></dd></div>
        <Show when={container()}>{value => <div class="setting-row"><dt>Container</dt><dd class="database-container-fact"><span class="mono">{value().name}</span></dd></div>}</Show>
      </dl>
    </aside>
    <section class="database-backup-strip">
      <span class="database-strip-icon"><Icon name="clock" /></span>
      <div class="database-strip-main"><strong>{plan()?.enabled !== false && plan() ? plan()!.name : "Daily backup"}</strong>
        <span>{backupDetail() || "Scheduled backup"}</span></div>
      <Show when={latestRun()}>{run => <div class="database-strip-state"><Status value={run().state} label={run().state === "succeeded" ? "Backed up" : run().state === "failed" ? "Failed" : "Running"} /></div>}</Show>
      <Button small variant="ghost" onClick={props.openBackups}>View backups<Icon name="chevron" /></Button>
    </section>
  </div>;
}
