import type { Container, Port } from "./types";

export function statusOf(container: Container): string {
  if (container.health === "unhealthy") return "unhealthy";
  if (container.state === "running" && container.health === "starting") return "starting";
  return container.state;
}
export function statusLabel(value: string): string {
  const labels: Record<string, string> = {
    before_deploy: "Before deploy", prepared: "Preparing", uncertain: "Uncertain", cancelled:"Cancelled",pulling:"Pulling image",completed: "Completed", serving: "Serving", unavailable: "Unavailable", running: "Running", exited: "Stopped", created: "Created", paused: "Paused",
    restarting: "Restarting", dead: "Failed", removing: "Removing",
    unhealthy: "Unhealthy", starting: "Starting", missing: "Missing", failed: "Failed", degraded: "Degraded",
    queued: "Queued", cloning: "Cloning", packaging: "Packaging", checking: "Checking", promoting: "Promoting",
    succeeded: "Deployed", interrupted: "Interrupted", deleting: "Deleting",
    building: "Building", routing: "Updating domains", operating: "Applying action", rolling_back: "Restoring services", verified: "Verified",
  };
  return labels[value] ?? value;
}

export const operationPending = (state?: string) => !!state && !["succeeded", "failed", "interrupted","cancelled"].includes(state);
export function applicationState(app: import("./types").Application) {
  if (operationPending(app.latestDeployment?.state)) return app.latestDeployment!.state;
  if(!app.activeDeploymentId&&app.latestDeployment?.state==="cancelled")return "created";
  if (!app.activeDeploymentId && app.latestDeployment?.state === "failed") return "failed";
  if (app.state === "missing" && app.activeDeploymentId && (app.sourceKind === "git" || app.sourceKind === "compose")) return "unavailable";
  if (app.state === "running" && isStaticApplication(app)) return "serving";
  return app.state;
}
export function methodLabel(app: import("./types").Application) {
  const methods: Record<string, string> = { image:"Container image",inline:"Dockerfile",static: "Static site", compose: "Compose", dockerfile: "Dockerfile", railpack: "Railpack" };
  return methods[app.method] ?? methods[app.latestDeployment?.resolvedMethod ?? ""] ?? (app.method === "auto" ? "Automatic" : app.containers.some(c => c.project) ? "Compose" : "Docker");
}
export function deploymentLabel(d: import("./types").Deployment) { return d.state !== "succeeded" ? statusLabel(d.state) : ({ rollback: "Restored", deploy: "Deployed", domains: "Domains updated", start: "Started", stop: "Stopped", restart: "Restarted", delete: "Deleted", apply: "Applied", task: "Task completed", service_start:"Service started",service_stop:"Service stopped",service_restart:"Service restarted" }[d.operation] ?? "Completed"); }
export function primaryDomain(app: import("./types").Application) { return app.domains?.find(d => d.primary) ?? app.domains?.[0]; }
export function domainURL(domain: import("./types").Domain, proxy?: import("./types").ProxyConfig) { const port = domain.https ? proxy?.httpsPort : proxy?.httpPort; return (domain.https ? "https://" : "http://") + domain.hostname + (domain.hostname.endsWith(".localhost") && port && port !== (domain.https ? 443 : 80) ? ":" + port : "")+(domain.pathPrefix?domain.pathPrefix.replace(/\/$/,"")+"/":""); }
export function applicationURL(config: import("./types").StaticConfig, proxy?: import("./types").ProxyConfig) {
  if (!config.domain) return "";
  const port = config.https ? proxy?.httpsPort : proxy?.httpPort;
  const localPort = config.domain.endsWith(".localhost") && port && port !== (config.https ? 443 : 80) ? ":" + port : "";
  return (config.https ? "https://" : "http://") + config.domain + localPort;
}
export function filterContainers(items: Container[], search: string, scope: string, status: string): Container[] {
  const query = search.trim().toLocaleLowerCase();
  return items.filter(item =>
    (scope !== "managed" || item.managed) &&
    (status === "all" || statusOf(item) === status) &&
    (!query || [item.name, item.image, item.id, item.project, item.service].some(value => value?.toLocaleLowerCase().includes(query))),
  );
}
export function portLabel(ports: Port[]): string {
  return ports.map(port => port.public ? port.public + ":" + port.private + "/" + port.protocol : port.private + "/" + port.protocol).join(", ");
}
export function bytes(value: number): string {
  if (!value) return "0 B";
  const unit = Math.min(Math.floor(Math.log(value) / Math.log(1024)), 3);
  return (value / 1024 ** unit).toFixed(unit ? 1 : 0) + " " + ["B", "KiB", "MiB", "GiB"][unit];
}
export function createdDate(seconds: number): string {
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric" }).format(new Date(seconds * 1000));
}


export function isStaticApplication(app: import("./types").Application) { return app.method === "static" || app.activeMethod === "static" || (app.method === "auto" && app.latestDeployment?.resolvedMethod === "static"); }
export function deploymentSeconds(deployment: import("./types").Deployment, now: number): number {
  const end = operationPending(deployment.state) ? now : deployment.finishedAt || deployment.startedAt || deployment.createdAt;
  return Math.max(0, Math.floor(end - deployment.createdAt));
}
export function formatDuration(seconds: number): string {
  seconds = Math.max(0, Math.floor(seconds));
  if (seconds < 60) return seconds + "s";
  const minutes = Math.floor(seconds / 60), remainder = String(seconds % 60).padStart(2,"0");
  return minutes < 60 ? minutes + "m " + remainder + "s" : Math.floor(minutes / 60) + "h " + String(minutes % 60).padStart(2,"0") + "m " + remainder + "s";
}
export function logDateTime(seconds: number): string {
  const date = new Date(seconds * 1000), pad = (value: number) => String(value).padStart(2,"0");
  return `${date.getFullYear()}-${pad(date.getMonth()+1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
