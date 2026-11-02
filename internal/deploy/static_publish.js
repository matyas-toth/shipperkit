// Runs inside the Node build stage. Match StaticArchive's publication policy.
const fs = require("node:fs");
const path = require("node:path");
const root = fs.realpathSync(process.argv[1]);
const allowHTAccess = process.argv[2] === "true";
const output = "/shipper-output";

if (root !== "/app" && !root.startsWith("/app/")) {
  throw Error("Publish directory escapes the application");
}
if (!fs.lstatSync(path.join(root, "index.html")).isFile()) {
  throw Error("Publish directory must contain index.html");
}
fs.mkdirSync(output, { mode: 0o755 });
let count = 0, size = 0;

function copy(source, destination) {
  for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
    const name = entry.name.toLowerCase();
    const hidden = name.startsWith(".") && name !== ".well-known" && !(name === ".htaccess" && allowHTAccess);
    if (hidden || ["node_modules", "dockerfile", "compose.yaml", "compose.yml", "id_rsa"].includes(name) || name.endsWith(".pem") || name.endsWith(".key")) continue;
    const from = path.join(source, entry.name), to = path.join(destination, entry.name);
    const stat = fs.lstatSync(from);
    if (stat.isSymbolicLink() || (!stat.isDirectory() && !stat.isFile())) {
      throw Error("Unsupported publish file: " + entry.name);
    }
    if (++count > 10000 || (size += stat.size) > 100 * 1024 * 1024) {
      throw Error("Static sites are limited to 100 MiB and 10000 files");
    }
    if (stat.isDirectory()) {
      fs.mkdirSync(to, { mode: 0o755 });
      copy(from, to);
    } else {
      fs.copyFileSync(from, to);
      fs.chmodSync(to, 0o644);
    }
  }
}
copy(root, output);
