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
    unhealthy: "Unhealthy", starting: "Starting", missing: "Missing",
  };
  return labels[value] ?? value;
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

