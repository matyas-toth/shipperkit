import type { Container, Port } from "./types";

export function statusOf(container: Container): string {
  if (container.health === "unhealthy") return "unhealthy";
  if (container.state === "running" && container.health === "starting") return "starting";
  return container.state;
}
export function statusLabel(value: string): string {
  const labels: Record<string, string> = {
    running: "Running", exited: "Stopped", created: "Created", paused: "Paused",
    restarting: "Restarting", dead: "Failed", removing: "Removing",
    unhealthy: "Unhealthy", starting: "Starting", missing: "Missing", failed: "Failed", degraded: "Degraded",
    queued: "Queued", cloning: "Cloning", packaging: "Packaging", checking: "Checking", promoting: "Promoting",
    succeeded: "Deployed", interrupted: "Interrupted", deleting: "Deleting",
  };
  return labels[value] ?? value;
}

export const operationPending = (state?: string) => !!state && !["succeeded", "failed", "interrupted"].includes(state);
export function applicationState(app: import("./types").Application) {
  if (operationPending(app.latestDeployment?.state)) return app.latestDeployment!.state;
  if (!app.activeDeploymentId && app.latestDeployment?.state === "failed") return "failed";
  return app.state;
}
export function methodLabel(app: import("./types").Application) {
  const methods: Record<string, string> = { static: "Static site", compose: "Compose", dockerfile: "Dockerfile", railpack: "Railpack" };
  return methods[app.method] ?? (app.containers.some(c => c.project) ? "Compose" : "Docker");
}
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

