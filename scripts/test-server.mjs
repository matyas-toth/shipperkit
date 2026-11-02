// Isolated Docker API fixture for browser tests. It never touches the host engine.
import http from "node:http";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join, resolve, sep } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { goCommand } from "./go.mjs";
import { DatabaseSync } from "node:sqlite";

const token = "shipper-browser-test-setup-token-00000000000000000000000000000000";
const directory = await mkdtemp(join(tmpdir(), "shipper-browser-"));
const socket = process.platform === "win32" ? "\\\\.\\pipe\\shipper-test-" + randomUUID() : join(directory, "docker.sock");
let containers = [
  { Id: "a".repeat(64), Names: ["/api"], Image: "ghcr.io/example/api:v1.4", State: "running", Status: "Up 2 hours", Created: 1793376000, Ports: [{ IP: "127.0.0.1", PrivatePort: 3000, PublicPort: 3000, Type: "tcp" }], Labels: { "io.shipper.managed": "true", "io.shipper.application.id": "application-api" } },
  { Id: "b".repeat(64), Names: ["/web"], Image: "httpd:2.4-alpine", State: "running", Status: "Up 2 hours (healthy)", Created: 1793376000, Ports: [{ IP: "127.0.0.1", PrivatePort: 80, PublicPort: 8088, Type: "tcp" }], Labels: { "io.shipper.managed": "true", "io.shipper.application.id": "application-web", "io.shipper.deployment.id": "deployment-web-2" } },
  { Id: "c".repeat(64), Names: ["/postgres"], Image: "postgres:17-alpine", State: "exited", Status: "Exited (0) 1 hour ago", Created: 1793289600, Ports: [{ IP: "", PrivatePort: 5432, Type: "tcp" }], Labels: {} },
  { Id: "d".repeat(64), Names: ["/api-worker"], Image: "ghcr.io/example/worker:v1.4", State: "running", Status: "Up 2 hours", Created: 1793376000, Ports: [], Labels: { "io.shipper.managed": "true", "io.shipper.application.id": "application-api" } },
];
const websiteContainer = containers[1];
const streams = new Set();
let runtimeGeneration = 0;
const engine = http.createServer((request, response) => {
  const path = new URL(request.url, "http://docker").pathname.replace(/^\/v[\d.]+/, "");
  response.setHeader("Content-Type", "application/json");
  if (path === "/_ping") { response.setHeader("API-Version", "1.56"); response.end("OK"); }
  else if (path === "/info") response.end(JSON.stringify({ Name: "local-docker", ServerVersion: "29.8.2", OperatingSystem: "Linux", Architecture: "x86_64", NCPU: 4, MemTotal: 8589934592 }));
  else if (path === "/version") response.end(JSON.stringify({ Version: "29.8.2", ApiVersion: "1.56", MinAPIVersion: "1.44", Os: "linux" }));
  else if (/^\/containers\/[^/]+\/json$/.test(path)) { const id = path.split("/")[2], c = containers.find(c => c.Id === id);if (!c) { response.writeHead(404);response.end("{}");return; }response.end(JSON.stringify({ Id:c.Id,Config:{Labels:c.Labels,Tty:true},State:{Running:true} })); }
  else if (/^\/containers\/[^/]+\/logs$/.test(path)) { response.setHeader("Content-Type","application/vnd.docker.raw-stream");response.end("2026-11-01T12:00:00Z Application listening on port 3000\n2026-11-01T12:00:01Z GET /health 200\nRuntime generation " + runtimeGeneration + "\n"); }
  else if (path === "/containers/json") response.end(JSON.stringify(containers));
  else if (path === "/events") {
    response.setHeader("Content-Type", "application/x-ndjson"); response.writeHead(200); response.flushHeaders();
    streams.add(response); request.on("close", () => streams.delete(response));
  } else { response.writeHead(404); response.end(JSON.stringify({ message: "Unknown fixture endpoint" })); }
});
await new Promise((ready, reject) => { engine.once("error", reject); engine.listen(socket, ready); });
const control = http.createServer(async (request, response) => {
  response.setHeader("Content-Type", "application/json");
  if (request.method === "POST" && request.url === "/runtime-options") {
    let content="";for await(const chunk of request)content+=chunk;const {count}=JSON.parse(content);
    containers=containers.filter(c=>c.Labels["io.shipper.application.id"]!=="application-web");
    if(count>0)containers.push(websiteContainer);
    if(count>1){const worker=structuredClone(websiteContainer);worker.Id="e".repeat(64);worker.Names=["/shp-website-worker-fixture"];worker.Labels["io.shipper.infrastructure"]="";worker.Labels["com.docker.compose.service"]="worker";containers.push(worker);}
    for(const stream of streams)stream.write(JSON.stringify({Type:"container",Action:"start",Actor:{ID:websiteContainer.Id}})+"\n");response.end("{}");return;
  }
  if (request.method === "POST" && request.url === "/runtime-restart") {
    runtimeGeneration++;
    const db=new DatabaseSync(join(directory,"shipper.db"));db.exec("PRAGMA busy_timeout=5000");
    db.prepare("INSERT INTO deployments(id,application_id,operation,state,config_json,created_at,started_at,finished_at) SELECT ?,'application-web','restart','succeeded',config_json,1793532000,1793532000,1793532001 FROM applications WHERE id='application-web'").run("runtime-restart-"+runtimeGeneration);db.close();
    for(const stream of streams) stream.write(JSON.stringify({Type:"container",Action:"start",Actor:{ID:containers[0].Id}})+"\n");response.end("{}");return;
  }
  if (request.method === "POST" && request.url === "/logs") {
    const db = new DatabaseSync(join(directory,"shipper.db"));db.exec("PRAGMA busy_timeout=5000; BEGIN");
    const insert = db.prepare("INSERT INTO deployment_logs(deployment_id,step,line,created_at) VALUES ('deployment-web-2','packaging',?,?)");
    for(let index=0;index<1500;index++) insert.run("Build output line " + index,1793531000);
    db.exec("COMMIT");db.close();for(const stream of streams)stream.write(JSON.stringify({Type:"container",Action:"start",Actor:{ID:containers[0].Id}})+"\n");response.end("{}");return;
  }
  if (request.method === "POST" && request.url === "/seed") {
    const db = new DatabaseSync(join(directory, "shipper.db"));
    db.exec("PRAGMA busy_timeout=5000");
    const instance = db.prepare("SELECT value FROM settings WHERE key='instance_id'").get().value;
    for (const c of containers) if (c.Labels["io.shipper.managed"]) c.Labels["io.shipper.instance"] = instance;
    websiteContainer.Names=["/shipper-"+instance.slice(0,12)+"-site-deployment-web-2"];websiteContainer.Labels["io.shipper.infrastructure"]="packaging";
    const config = { repository: "https://github.com/mdn/beginner-html-site.git", branch: "main", publishDirectory: ".", routing: "files", htaccess: true, domain: "website.localhost", https: false };
    db.prepare("UPDATE applications SET name=?,source_kind='git',source_ref=?,method='static',config_json=?,active_deployment_id=? WHERE id='application-web'").run("Website", config.repository, JSON.stringify(config), "deployment-web-2");
    db.prepare("UPDATE applications SET domains_json=? WHERE id='application-web'").run(JSON.stringify([{ id:"primary",hostname:"website.localhost",https:false,primary:true,redirect:false }]));
    db.prepare("INSERT OR IGNORE INTO deployments(id,application_id,operation,state,config_json,commit_sha,image_id,container_id,created_at,started_at,finished_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)").run("deployment-web-2", "application-web", "deploy", "succeeded", JSON.stringify(config), "d6a4f792a0ce4d2cf5197aa9e54972be35f24a10", "sha256:apache-fixture", "b".repeat(64), 1793531000, 1793531000, 1793531015);
    db.prepare("INSERT OR IGNORE INTO deployments(id,application_id,operation,state,config_json,created_at,started_at,finished_at) VALUES ('deployment-web-1','application-web','deploy','succeeded',?,1793530000,1793530000,1793530010)").run(JSON.stringify(config));
    const earlier=db.prepare("INSERT INTO deployment_logs(deployment_id,step,line,created_at) VALUES ('deployment-web-1','building',?,1793530000)");for(let index=0;index<100;index++)earlier.run("Earlier build line "+index);
    const insert = db.prepare("INSERT INTO deployment_logs(deployment_id,step,line,created_at) VALUES ('deployment-web-2',?,?,?)");
    db.exec("BEGIN");
    for (const [step, line] of [["cloning", "Cloning https://github.com/mdn/beginner-html-site.git"], ["cloning", "Checked out d6a4f792 on main"], ["packaging", "Packaging Apache image"], ["starting", "Starting candidate container"], ["checking", "Candidate HTTP check passed"], ["promoting", "Traefik acknowledged the routing update"], ["promoting", "Serving website.localhost"]]) insert.run(step, line, 1793531000);
    db.exec("COMMIT");db.close();
    for (const stream of streams) stream.write(JSON.stringify({ Type: "container", Action: "start", Actor: { ID: containers[0].Id } }) + "\n");
    response.end("{}");return;
  }
  if (request.method === "POST" && request.url === "/state") {
    let data = ""; for await (const part of request) data += part;
    const input = JSON.parse(data);
    if (input.large) {
      containers = Array.from({ length: 600 }, (_, index) => ({
        Id: String(index).padStart(64, "0"), Names: ["/app-" + String(index).padStart(3, "0")],
        Image: "example/service:v1", State: "running", Status: "Up", Created: 1793376000, Ports: [], Labels: { "io.shipper.managed": "true", "io.shipper.application.id": "application-" + String(index).padStart(3,"0") },
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

