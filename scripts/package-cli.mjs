import { execFileSync } from "node:child_process";
import { mkdtempSync, copyFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const temporary = mkdtempSync(join(tmpdir(), "shipper-cli-package-"));
try {
  const npm = process.env.npm_execpath;
  if (!npm) throw new Error("Run npm run cli:package to package the CLI");
  const args = [npm, "pack", "./cli", "--ignore-scripts", "--json", "--pack-destination", temporary];
  const output = execFileSync(process.execPath, args, { cwd: root, encoding: "utf8" });
  const [result] = JSON.parse(output);
  const destination = resolve(process.argv[2] || "shipper-cli.tgz");
  copyFileSync(join(temporary, result.filename), destination);
  process.stdout.write(destination + "\n");
} finally {
  if (!resolve(temporary).startsWith(resolve(tmpdir()) + (process.platform === "win32" ? "\\" : "/"))) throw new Error("Invalid package workspace");
  rmSync(temporary, { recursive: true, force: true });
}
