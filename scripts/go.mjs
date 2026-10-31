import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

export function goCommand() {
  if (process.env.SHIPPER_GO) return process.env.SHIPPER_GO;
  if (spawnSync("go", ["version"], { stdio: "ignore" }).status === 0) return "go";
  const cached = join(homedir(), ".cache", "shipper-tools", "go", "bin", process.platform === "win32" ? "go.exe" : "go");
  if (existsSync(cached)) return cached;
  throw new Error("Go 1.26 or newer is required. Install Go or set SHIPPER_GO to its executable.");
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const child = spawn(goCommand(), process.argv.slice(2), { stdio: "inherit" });
  child.on("error", error => { console.error(error.message); process.exitCode = 1; });
  child.on("exit", code => { process.exitCode = code ?? 1; });
  process.on("SIGINT", () => child.kill("SIGINT"));
}

