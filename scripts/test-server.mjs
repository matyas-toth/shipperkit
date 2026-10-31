// Isolated Docker API fixture for browser tests. It never touches the host engine.
import http from "node:http";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join, resolve, sep } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { goCommand } from "./go.mjs";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const directory = await mkdtemp(join(tmpdir(), "shipper-browser-"));
const socket = process.platform === "win32" ? "\\\\.\\pipe\\shipper-test-" + randomUUID() : join(directory, "docker.sock");
let containers = [
  { Id: "a".repeat(64), Names: ["/api"], Image: "ghcr.io/example/api:v1.4", State: "running", Status: "Up 2 hours", Created: 1793376000, Ports: [{ IP: "127.0.0.1", PrivatePort: 3000, PublicPort: 3000, Type: "tcp" }], Labels: { "io.shipper.managed": "true", "io.shipper.application.id": "application-api" } },
  { Id: "b".repeat(64), Names: ["/web"], Image: "httpd:2.4-alpine", State: "running", Status: "Up 2 hours (healthy)", Created: 1793376000, Ports: [{ IP: "127.0.0.1", PrivatePort: 80, PublicPort: 8088, Type: "tcp" }], Labels: { "io.shipper.managed": "true", "io.shipper.application.id": "application-web" } },
  { Id: "c".repeat(64), Names: ["/postgres"], Image: "postgres:17-alpine", State: "exited", Status: "Exited (0) 1 hour ago", Created: 1793289600, Ports: [{ IP: "", PrivatePort: 5432, Type: "tcp" }], Labels: {} },
];
const streams = new Set();
const engine = http.createServer((request, response) => {
  const path = new URL(request.url, "http://docker").pathname.replace(/^\/v[\d.]+/, "");
  response.setHeader("Content-Type", "application/json");
  if (path === "/_ping") { response.setHeader("API-Version", "1.56"); response.end("OK"); }
  else if (path === "/info") response.end(JSON.stringify({ Name: "local-docker", ServerVersion: "29.8.2", OperatingSystem: "Linux", Architecture: "x86_64", NCPU: 4, MemTotal: 8589934592 }));
  else if (path === "/version") response.end(JSON.stringify({ Version: "29.8.2", ApiVersion: "1.56", MinAPIVersion: "1.44", Os: "linux" }));
  else if (path === "/containers/json") response.end(JSON.stringify(containers));
  else if (path === "/events") {
    response.setHeader("Content-Type", "application/x-ndjson"); response.writeHead(200); response.flushHeaders();
    streams.add(response); request.on("close", () => streams.delete(response));
  } else { response.writeHead(404); response.end(JSON.stringify({ message: "Unknown fixture endpoint" })); }
});
await new Promise((ready, reject) => { engine.once("error", reject); engine.listen(socket, ready); });
const control = http.createServer(async (request, response) => {
  response.setHeader("Content-Type", "application/json");
  if (request.method === "POST" && request.url === "/state") {
    let data = ""; for await (const part of request) data += part;
    const input = JSON.parse(data);
    if (input.large) {
      containers = Array.from({ length: 600 }, (_, index) => ({
        Id: String(index).padStart(64, "0"), Names: ["/app-" + String(index).padStart(3, "0")],
        Image: "example/service:v1", State: "running", Status: "Up", Created: 1793376000, Ports: [], Labels: {},
      }));
    } else { containers[0].State = input.state ?? "running"; containers[0].Status = input.state === "exited" ? "Exited (0)" : "Up 2 hours"; }
    for (const stream of streams) stream.write(JSON.stringify({ Type: "container", Action: "start", Actor: { ID: containers[0].Id } }) + "\n");
    response.end("{}");
  } else { response.end(JSON.stringify({ ready: true })); }
});
await new Promise(resolve => control.listen(9083, "127.0.0.1", resolve));
const binary = join(directory, process.platform === "win32" ? "shipper.exe" : "shipper");
const build = spawnSync(goCommand(), ["build", "-o", binary, "./cmd/shipper"], { stdio: "inherit" });
if (build.status !== 0) throw Error("Could not build Shipper");
const child = spawn(binary, [], {
  stdio: "inherit",
  env: { ...process.env, SHIPPER_ADDR: "127.0.0.1:9081", SHIPPER_DATA_DIR: directory, SHIPPER_SETUP_TOKEN: token,
    SHIPPER_PUBLIC_URL: "", DOCKER_HOST: process.platform === "win32" ? "npipe://" + socket.replaceAll("\\", "/") : "unix://" + socket,
    DOCKER_TLS_VERIFY: "", DOCKER_CERT_PATH: "" },
});
let closing = false;
async function close() {
  if (closing) return; closing = true;
  child.kill();
  for (const stream of streams) stream.destroy();
  engine.closeAllConnections(); control.closeAllConnections();
  engine.close(); control.close();
  const target = resolve(directory);
  if (target.startsWith(resolve(tmpdir()) + sep) && basename(target).startsWith("shipper-browser-")) await rm(target, { recursive: true, force: true });
}
process.on("SIGINT", () => void close());
process.on("SIGTERM", () => void close());
child.on("exit", code => { void close(); process.exitCode = code ?? 1; });

