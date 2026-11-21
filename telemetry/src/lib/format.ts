const number = new Intl.NumberFormat("en-US");
const compact = new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 });
const percent = new Intl.NumberFormat("en-US", { style: "percent", maximumFractionDigits: 0 });
const regions = new Intl.DisplayNames(["en"], { type: "region" });
const day = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", timeZone: "UTC" });
const date = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", year: "numeric" });

export const formatNumber = (value: number) => number.format(value);
export const formatCompact = (value: number) => (Math.abs(value) < 10_000 ? number.format(value) : compact.format(value));
export const formatPercent = (value: number) => percent.format(value);
export const formatDay = (iso: string) => day.format(new Date(iso + "T00:00:00Z"));
export const formatDate = (ms: number) => date.format(ms);

export function formatDuration(seconds: number) {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds % 60);
  return rest ? `${minutes}m ${rest}s` : `${minutes}m`;
}

export function formatAgo(ms: number, now = Date.now()) {
  const minutes = Math.max(0, Math.round((now - ms) / 60_000));
  if (minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

export function countryName(code: string) {
  if (!/^[A-Z]{2}$/.test(code)) return code;
  try {
    return regions.of(code) ?? code;
  } catch {
    return code;
  }
}

function parts(version: string) {
  const [core, pre = ""] = version.split("-", 2);
  return { core: core.split(".").map(Number), pre };
}

/** Semver order where a release outranks its own prereleases. */
export function compareVersions(a: string, b: string) {
  if (a === "Other") return -1;
  if (b === "Other") return 1;
  const left = parts(a);
  const right = parts(b);
  for (let i = 0; i < 3; i++) {
    const diff = (left.core[i] || 0) - (right.core[i] || 0);
    if (diff) return diff;
  }
  if (left.pre === right.pre) return 0;
  if (!left.pre) return 1;
  if (!right.pre) return -1;
  return left.pre.localeCompare(right.pre, "en", { numeric: true });
}

const LABELS: Record<string, string> = {
  auto: "Auto",
  dockerfile: "Dockerfile",
  static: "Static",
  compose: "Compose",
  image: "Image",
  inline: "Inline Dockerfile",
  nixpacks: "Nixpacks",
  railpack: "Railpack",
  git: "Git",
  github: "GitHub",
  gitlab: "GitLab",
  local: "Upload",
  catalog: "Catalog",
  registry: "Registry",
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  mongodb: "MongoDB",
  redis: "Redis",
  valkey: "Valkey",
  amd64: "AMD64",
  arm64: "ARM64",
  auto_deploy: "Auto deploy",
  auto_update: "Auto update",
  backups: "Backups",
  tasks: "Scheduled tasks",
  access_tokens: "Access tokens",
  two_factor: "Two-factor",
  sso: "Single sign-on",
  team: "Teams",
  https: "App HTTPS",
  dashboard_https: "Dashboard HTTPS",
  hooks: "Deploy hooks",
  volumes: "Volumes",
  none: "None",
  other: "Other",
};

export function label(key: string) {
  return LABELS[key] ?? key.charAt(0).toUpperCase() + key.slice(1).replace(/[_-]/g, " ");
}
