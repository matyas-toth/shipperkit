import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { readdir, stat } from "node:fs/promises";
import { basename, join, resolve } from "node:path";
import { c as createTar } from "tar";
import { Client, CliError } from "./client";
import { applications, waitForDeployment, type Deployment } from "./apps";
import { heading, muted } from "./output";

const limitFiles = 10_000;
const limitBytes = 64 << 20;

function excluded(relative: string) {
  for (const part of relative.split("/")) {
    const lower = part.toLowerCase();
    if ([".git", "node_modules", ".shipper", ".ssh", ".aws", ".vpsenv", ".env", "id_rsa", "id_ed25519"].includes(lower)) return true;
    if (lower.startsWith(".env.") && ![".env.example", ".env.sample", ".env.template"].includes(lower)) return true;
    if (lower.endsWith(".pem") || lower.endsWith(".key")) return true;
  }
  return false;
}

function globMatch(value: string, pattern: string) {
  const source = pattern.replace(/[.+^${}()|[\]\\]/g, "\\$&").replace(/\*/g, ".*").replace(/\?/g, ".");
  return new RegExp("^" + source + "$").test(value);
}

async function collect(root: string, relative: string, files: string[], ignore: string[], state: { bytes: number }) {
  const entries = await readdir(join(root, relative), { withFileTypes: true });
  for (const entry of entries) {
    const path = relative ? `${relative}/${entry.name}` : entry.name;
    if (excluded(path) || ignore.some(pattern => globMatch(path, pattern))) continue;
    if (entry.isSymbolicLink()) throw new CliError(`Source symlinks are unsupported: ${path}`);
    if (entry.isDirectory()) {
      await collect(root, path, files, ignore, state);
      continue;
    }
    if (!entry.isFile()) throw new CliError("Only regular source files are supported");
    const info = await stat(join(root, path));
    state.bytes += info.size;
    files.push(path);
    if (files.length > limitFiles || state.bytes > limitBytes) {
      throw new CliError("Source is limited to 64 MiB and 10000 files; add .shipperignore exclusions");
    }
  }
}

async function archive(root: string) {
  const ignore = existsSync(join(root, ".shipperignore"))
    ? readFileSync(join(root, ".shipperignore"), "utf8").split("\n").map(line => line.trim()).filter(line => line && !line.startsWith("#"))
    : [];
  const files: string[] = [];
  const state = { bytes: 0 };
  await collect(root, "", files, ignore, state);
  if (!files.length) throw new CliError("This folder has no files to deploy");
  const stream = createTar({ cwd: root, portable: true, follow: false }, files);
  const chunks: Buffer[] = [];
  await new Promise<void>((ready, reject) => {
    stream.on("data", (chunk: Buffer) => chunks.push(Buffer.from(chunk)));
    stream.on("end", ready);
    stream.on("error", reject);
  });
  return { body: Buffer.concat(chunks), files: files.length, bytes: state.bytes };
}

function detectMethod(root: string) {
  if (existsSync(join(root, "Dockerfile"))) return "dockerfile";
  for (const name of ["compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"]) {
    if (existsSync(join(root, name))) throw new CliError("Local Compose uploads are not supported yet. Connect the Git repository instead.");
  }
  if (existsSync(join(root, "package.json"))) return "auto";
  if (existsSync(join(root, "index.html"))) return "static";
  return "auto";
}

function provenance(root: string) {
  try {
    const commit = execFileSync("git", ["-C", root, "rev-parse", "HEAD"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
    const status = execFileSync("git", ["-C", root, "status", "--porcelain", "--ignore-submodules=all"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
    return (commit + (status ? " dirty" : "")).slice(0, 256);
  } catch {
    return basename(root).slice(0, 256);
  }
}

export async function deployLocal(
  client: Client,
  target: string,
  options: { name?: string; domain?: string; method?: string },
  json = false,
) {
  const root = resolve(target);
  const info = await stat(root).catch(() => undefined);
  if (!info?.isDirectory()) throw new CliError(`Folder not found: ${target}`);
  const name = (options.name || basename(root) || "Local application").trim();
  if (!name || name.length > 80) throw new CliError("Use an application name of up to 80 characters");
  const existing = await applications(client);
  if (existing.items.some(app => app.name.toLowerCase() === name.toLowerCase())) {
    throw new CliError(`Application "${name}" already exists. Redeploy it with shipper deploy "${name}".`);
  }
  const method = options.method || detectMethod(root);
  const source = await archive(root);
  if (!json) {
    const size = source.bytes >= 1 << 20 ? `${(source.bytes / (1 << 20)).toFixed(1)} MiB` : `${Math.max(1, Math.round(source.bytes / 1024))} KiB`;
    process.stdout.write(heading(`Uploading ${source.files} files\n`));
    process.stdout.write(muted(`${size} from ${root}`) + "\n");
  }
  const snapshot = await client.upload<{ id: string }>(
    `/sources?provenance=${encodeURIComponent(provenance(root))}`,
    source.body,
    "application/x-tar",
  );
  const config: Record<string, unknown> = {
    method,
    localSource: snapshot.id,
    publishDirectory: ".",
  };
  if (options.domain) config.domain = options.domain;
  const plan = await client.post<{ applicationId: string; revision: string; name: string; commit: string }>("/plan", { name, config });
  if (!json) process.stdout.write(heading(`Starting ${plan.name}\n`));
  const deployment = await client.post<Deployment>("/applications", {
    id: plan.applicationId,
    revision: plan.revision,
    name: plan.name,
    config,
    commit: plan.commit || undefined,
  });
  return waitForDeployment(client, plan.applicationId, deployment, json);
}
