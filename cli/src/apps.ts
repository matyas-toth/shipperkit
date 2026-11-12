import { spawn } from "node:child_process";
import { Client, CliError } from "./client";
import { heading, muted, paint, printJson, relativeTime, statusText, table } from "./output";

export type Domain = {
  hostname: string;
  https?: boolean;
  primary?: boolean;
};

export type Deployment = {
  id: string;
  applicationName?: string;
  applicationId?: string;
  operation: string;
  state: string;
  commit?: string;
  imageId?: string;
  error?: string;
  createdAt: number;
  startedAt?: number;
  finishedAt?: number;
};

export type Application = {
  id: string;
  name: string;
  state: string;
  desiredState?: string;
  method?: string;
  sourceKind?: string;
  sourceRef?: string;
  config?: {
    domain?: string;
    repository?: string;
    branch?: string;
    variables?: Array<{ name: string; secret?: boolean }>;
    services?: Array<{ name: string }>;
  };
  domains?: Domain[];
  activeDeploymentId?: string;
  latestDeployment?: Deployment;
  containers?: Array<{ id: string; name: string; service?: string; state: string }>;
  updatedAt?: number;
};

type ListResponse = { items: Application[]; docker?: { connected?: boolean } };
type DetailResponse = { application: Application; activeDeployment?: Deployment; deployments?: Deployment[] };

const terminal = new Set(["succeeded", "failed", "interrupted", "cancelled"]);

export function primaryDomain(app: Application) {
  const domains = app.domains || [];
  return domains.find(domain => domain.primary) || domains[0] || (app.config?.domain ? { hostname: app.config.domain, https: false } : undefined);
}

export function applicationUrl(app: Application) {
  const domain = primaryDomain(app);
  if (!domain) return undefined;
  const secure = domain.https || (!domain.hostname.endsWith(".localhost") && !domain.hostname.startsWith("127."));
  return `${secure ? "https" : "http"}://${domain.hostname}`;
}

export async function applications(client: Client) {
  return client.get<ListResponse>("/applications");
}

export async function resolveApplication(client: Client, reference: string) {
  const list = await applications(client);
  const value = reference.toLowerCase();
  const match = list.items.find(app => app.id === reference) || list.items.find(app => app.name.toLowerCase() === value);
  if (match) return match;
  const partial = list.items.filter(app => app.name.toLowerCase().includes(value));
  if (partial.length === 1) return partial[0];
  if (partial.length > 1) throw new CliError(`"${reference}" matches ${partial.map(app => app.name).join(", ")}. Use the full name.`);
  throw new CliError(`Application "${reference}" was not found`);
}

export async function showApps(client: Client, json = false) {
  const result = await applications(client);
  if (json) {
    printJson(result);
    return;
  }
  if (!result.items.length) {
    process.stdout.write(muted("No applications yet. Create one with shipper apps create.") + "\n");
    return;
  }
  if (result.docker && result.docker.connected === false) {
    process.stdout.write(paint().yellow("Docker is disconnected. Status may be stale.") + "\n\n");
  }
  const rows = result.items.map(app => {
    const domain = primaryDomain(app);
    const latest = app.latestDeployment;
    return [
      app.name,
      statusText(app.desiredState === "stopped" ? "stopped" : app.state || "unknown"),
      domain?.hostname || "no domain",
      app.method || app.sourceKind || "unknown",
      latest ? `${relativeTime(latest.finishedAt || latest.createdAt)}` : "never",
    ];
  });
  process.stdout.write(table(["Name", "Status", "Domain", "Method", "Deployed"], rows) + "\n");
}

export async function showApp(client: Client, reference: string, json = false) {
  const app = await resolveApplication(client, reference);
  const detail = await client.get<DetailResponse>(`/applications/${app.id}`);
  if (json) {
    printJson(detail);
    return;
  }
  const application = detail.application;
  const domains = (application.domains || []).map(domain => domain.hostname).join(", ") || "none";
  const source = application.config?.repository
    ? `${application.config.repository}${application.config.branch ? ` @ ${application.config.branch}` : ""}`
    : application.sourceKind || "unknown";
  const release = detail.activeDeployment;
  process.stdout.write(heading(application.name) + "  " + muted(application.id) + "\n");
  process.stdout.write(statusText(application.desiredState === "stopped" ? "stopped" : application.state || "unknown") + "\n\n");
  const details: Array<[string, string]> = [
    ["Domain", domains],
    ["Source", source],
    ["Method", application.method || "unknown"],
    ["Release", release?.commit ? release.commit.slice(0, 12) : release?.id || "not deployed"],
    ["Deployed", release ? relativeTime(release.finishedAt || release.startedAt || release.createdAt) : "never"],
    ["Variables", String(application.config?.variables?.length || 0)],
  ];
  for (const [label, value] of details) process.stdout.write(muted(label.padEnd(10)) + " " + value + "\n");
  if (application.containers?.length) {
    process.stdout.write("\n" + heading("Containers") + "\n");
    process.stdout.write(table(["Name", "Service", "State"], application.containers.map(container => [
      container.name,
      container.service || "-",
      statusText(container.state),
    ])) + "\n");
  }
  const url = applicationUrl(application);
  if (url) process.stdout.write("\n" + muted(url) + "\n");
}

function deploymentLine(deployment: Deployment) {
  const label = `${deployment.operation} ${deployment.id.slice(0, 10)}`;
  return `${statusText(deployment.state)} ${muted(label)}${deployment.error ? " " + paint().red(deployment.error) : ""}`;
}

export async function waitForDeployment(client: Client, appId: string, initial: Deployment, json = false) {
  let current = initial;
  let last = "";
  const deadline = Date.now() + 20 * 60 * 1000;
  while (!terminal.has(current.state)) {
    if (Date.now() > deadline) throw new CliError("The operation did not finish within 20 minutes");
    if (!json) {
      const line = deploymentLine(current);
      if (line !== last) {
        process.stdout.write(line + "\n");
        last = line;
      }
    }
    await new Promise(resolve => setTimeout(resolve, 1000));
    current = await client.get<Deployment>(`/applications/${appId}/deployments/${initial.id}`);
  }
  if (json) {
    printJson(current);
  } else {
    process.stdout.write("\n" + deploymentLine(current) + "\n");
    if (current.state === "succeeded") {
      process.stdout.write(muted(current.commit ? `commit ${current.commit.slice(0, 12)}` : "operation complete") + "\n");
    }
  }
  if (current.state !== "succeeded") throw new CliError(current.error || `Operation ${current.state}`, 1, current.state);
  return current;
}

export async function deploy(client: Client, reference: string, noCache: boolean, json = false) {
  const app = await resolveApplication(client, reference);
  if (!json) process.stdout.write(heading(`Deploying ${app.name}\n`));
  const deployment = await client.post<Deployment>(`/applications/${app.id}/deployments`, { noCache });
  return waitForDeployment(client, app.id, deployment, json);
}

export async function lifecycle(client: Client, reference: string, action: "start" | "stop" | "restart", json = false) {
  const app = await resolveApplication(client, reference);
  if (!json) process.stdout.write(heading(`${action[0].toUpperCase() + action.slice(1)} ${app.name}\n`));
  const deployment = await client.post<Deployment>(`/applications/${app.id}/lifecycle`, { action });
  return waitForDeployment(client, app.id, deployment, json);
}

export async function remove(client: Client, reference: string, yes: boolean, json = false) {
  const app = await resolveApplication(client, reference);
  if (!yes) throw new CliError(`Add --yes to remove ${app.name}`);
  if (!json) process.stdout.write(heading(`Removing ${app.name}\n`));
  const deployment = await client.post<Deployment>(`/applications/${app.id}/delete`, { confirmName: app.name });
  if (deployment && deployment.id) return waitForDeployment(client, app.id, deployment, json);
  if (json) printJson({ removed: true, applicationId: app.id });
  else process.stdout.write(statusText("Removed") + "\n");
}

export async function create(
  client: Client,
  name: string,
  options: { repo: string; branch?: string; domain?: string; method?: string },
  json = false,
) {
  const config: Record<string, unknown> = {
    method: options.method || "auto",
    repository: options.repo,
    branch: options.branch || "main",
  };
  if (options.domain) config.domain = options.domain;
  if (!json) process.stdout.write(heading(`Creating ${name}\n`));
  const plan = await client.post<{ applicationId: string; revision: string; name: string; commit: string; config: Record<string, unknown> }>("/plan", { name, config });
  const deployment = await client.post<Deployment>("/applications", {
    id: plan.applicationId,
    revision: plan.revision,
    name: plan.name,
    config,
    commit: plan.commit || undefined,
  });
  return waitForDeployment(client, plan.applicationId, deployment, json);
}

export function openApplication(app: Application) {
  const url = applicationUrl(app);
  if (!url) throw new CliError("This application has no domain yet");
  if (process.platform === "win32") {
    spawn("rundll32", ["url.dll,FileProtocolHandler", url], { detached: true, stdio: "ignore" }).unref();
  } else {
    spawn(process.platform === "darwin" ? "open" : "xdg-open", [url], { detached: true, stdio: "ignore" }).unref();
  }
  return url;
}
