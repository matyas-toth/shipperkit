import { spawn, spawnSync } from "node:child_process";
import { goCommand } from "./go.mjs";

const command = goCommand();
// Compile once so Ctrl+C owns the actual server, including on Windows.
const build = spawnSync(command, ["build", "-o", "bin/", "./cmd/shipper"], { stdio: "inherit" });
if (build.status !== 0) process.exit(build.status ?? 1);
const server = spawn(process.platform === "win32" ? "bin/shipper.exe" : "bin/shipper", [], { stdio: "inherit" });
const vite = spawn(process.execPath, ["web/node_modules/vite/bin/vite.js", "web", "--config", "web/vite.config.ts"], { stdio: "inherit" });
let stopping = false;
function stop() { if (stopping) return; stopping = true; server.kill(); vite.kill(); }
for (const child of [server, vite]) {
  child.on("error", error => { console.error(error.message); process.exitCode = 1; stop(); });
  child.on("exit", code => { if (!stopping) { process.exitCode = code ?? 1; stop(); } });
}
process.on("SIGINT", stop); process.on("SIGTERM", stop);
console.log("Shipper development: http://127.0.0.1:5173");

