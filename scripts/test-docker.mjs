// Opt-in real-engine smoke test; only this run's labeled fixture may be removed.
import { spawn, spawnSync } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join, resolve, sep } from "node:path";
import { randomBytes } from "node:crypto";
import { createServer } from "node:net";
import { goCommand } from "./go.mjs";

function docker(...args) {
  const result = spawnSync("docker", args, { encoding: "utf8", windowsHide: true });
  if (result.status !== 0) throw Error(result.stderr || "Docker command failed");
  return result.stdout.trim();
}
const id = randomBytes(6).toString("hex");
const name = "shipper-smoke-" + id;
const directory = await mkdtemp(join(tmpdir(), "shipper-smoke-"));
const binary = join(directory, process.platform === "win32" ? "shipper.exe" : "shipper");
const setupToken = randomBytes(32).toString("hex");
const password = randomBytes(24).toString("hex");
const probe = createServer();
await new Promise(resolve => probe.listen(0, "127.0.0.1", resolve));
const port = probe.address().port; await new Promise(resolve => probe.close(resolve));
const origin = "http://127.0.0.1:" + port;
let child, cookie, csrf, fixture = false;
const checks = [];
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
async function poll(fn, message, timeout = 20_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) { try { const result = await fn(); if (result) return result; } catch {} await sleep(100); }
  throw Error(message);
}
function start() {
  child = spawn(binary, [], {
    env: { ...process.env, SHIPPER_ADDR: "127.0.0.1:" + port, SHIPPER_DATA_DIR: directory, SHIPPER_SETUP_TOKEN: setupToken, SHIPPER_PUBLIC_URL: "" },
    stdio: ["ignore", "ignore", "pipe"],
  });
  child.stderr.on("data", chunk => process.stderr.write(chunk));
}
async function stop() {
  if (!child || child.exitCode !== null) return;
  const current = child; const stopped = new Promise(resolve => current.once("exit", resolve));
  current.kill(); await stopped;
}
async function api(path, body) {
  const response = await fetch(origin + "/api" + path, {
    method: body ? "POST" : "GET",
    headers: { ...(cookie ? { Cookie: cookie } : {}), ...(body ? { "Content-Type": "application/json", "X-CSRF-Token": csrf ?? "" } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) throw Error(path + ": HTTP " + response.status);
  return response.json();
}
try {
  const build = spawnSync(goCommand(), ["build", "-o", binary, "./cmd/shipper"], { stdio: "inherit" });
  if (build.status !== 0) throw Error("Could not build Shipper");
  docker("info", "--format", "{{.ServerVersion}}");
  start();
  await poll(() => fetch(origin + "/api/health").then(response => response.ok), "Shipper did not start");
  const setup = await fetch(origin + "/api/auth/setup", { method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email: "smoke@example.test", password, setupToken }) });
  if (setup.status !== 201) throw Error("First-admin setup failed");
  cookie = setup.headers.get("set-cookie").split(";")[0]; csrf = (await setup.json()).csrfToken;
  await poll(async () => (await api("/containers")).docker.connected, "Docker SDK did not connect");
  checks.push("secure setup and real Docker SDK connection");
  const streamController = new AbortController();
  const stream = await fetch(origin + "/api/events", { headers: { Cookie: cookie }, signal: streamController.signal });
  const reader = stream.body.getReader();
  const first = await reader.read();
  if (!new TextDecoder().decode(first.value).includes("event: inventory")) throw Error("SSE did not send initial state");
  docker("run", "-d", "--name", name, "--label", "io.shipper.test-run=" + id, "--label", "io.shipper.managed=true",
    "--label", "io.shipper.application.id=" + name, "--network", "none", "--memory", "32m", "busybox:1.37.0", "sleep", "300");
  fixture = true;
  await poll(async () => (await api("/containers")).items.find(item => item.name === name && item.state === "running"), "Running fixture did not reach the dashboard");
  checks.push("owned container discovery");
  docker("stop", "--time", "1", name);
  await poll(async () => (await api("/containers")).items.find(item => item.name === name && item.state === "exited"), "Docker stop event was not reconciled");
  checks.push("live Docker stop event -> SQLite -> API");
  const update = await reader.read();
  if (!new TextDecoder().decode(update.value).includes("event: inventory")) throw Error("SSE did not invalidate");
  streamController.abort();
  checks.push("authenticated SSE invalidation");
  await stop(); start();
  await poll(async () => (await api("/applications")).items.find(item => item.id === name), "Application/session did not survive process restart");
  checks.push("SQLite and session restart persistence");
  const owner = docker("inspect", "--format", '{{index .Config.Labels "io.shipper.test-run"}}', name);
  if (owner !== id) throw Error("Refusing to remove a fixture without this run's ownership label");
  docker("rm", "-f", name); fixture = false;
  await poll(async () => (await api("/applications")).items.find(item => item.id === name && item.state === "missing"), "Application identity was lost after removal");
  checks.push("container removal preserves application identity");
  await api("/auth/logout", {});
  if ((await fetch(origin + "/api/containers", { headers: { Cookie: cookie } })).status !== 401) throw Error("Logout did not revoke the session");
  checks.push("logout revokes access");
  console.log(JSON.stringify({ result: "passed", checks }, null, 2));
} finally {
  await stop();
  if (fixture) {
    const owner = docker("inspect", "--format", '{{index .Config.Labels "io.shipper.test-run"}}', name);
    if (owner === id) docker("rm", "-f", name);
  }
  const target = resolve(directory);
  if (target.startsWith(resolve(tmpdir()) + sep) && basename(target).startsWith("shipper-smoke-")) await rm(target, { recursive: true, force: true });
}

