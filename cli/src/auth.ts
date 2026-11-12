import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import http from "node:http";
import { hostname } from "node:os";
import { createInterface } from "node:readline/promises";
import { stdin, stdout } from "node:process";
import { CliError, capabilities } from "./client";
import { configPath, deleteConfig, loadConfig, normalizeUrl, resolveAuth, saveConfig, type Auth } from "./config";
import { heading, muted, printJson, statusText, table } from "./output";

async function prompt(question: string) {
  const reader = createInterface({ input: stdin, output: stdout });
  try {
    return (await reader.question(question)).trim();
  } finally {
    reader.close();
  }
}

export async function promptInstanceUrl(initial?: string) {
  if (initial) return normalizeUrl(initial);
  if (process.env.SHIPPER_URL) return normalizeUrl(process.env.SHIPPER_URL);
  return normalizeUrl(await prompt("Enter your Shipper instance URL: "));
}

function openBrowser(url: string) {
  try {
    if (process.platform === "win32") {
      spawn("rundll32", ["url.dll,FileProtocolHandler", url], { detached: true, stdio: "ignore" }).unref();
      return;
    }
    spawn(process.platform === "darwin" ? "open" : "xdg-open", [url], { detached: true, stdio: "ignore" }).unref();
  } catch {
    // The URL is printed for manual opening.
  }
}

function callbackPage(message: string) {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Shipper CLI</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#080909;color:#f4f4f0;font:15px/1.6 Inter,system-ui,sans-serif}.card{width:min(420px,calc(100vw - 32px));padding:28px;border:1px solid #2c2d2d;border-radius:16px;background:#151616;box-shadow:0 30px 80px #000a}.mark{display:grid;width:40px;height:40px;place-items:center;border-radius:10px;background:#222323;margin-bottom:18px;font-size:18px}h1{margin:0 0 8px;font-size:24px;font-weight:560;letter-spacing:-.5px}p{margin:0;color:#a4a5a0}</style></head>
<body><div class="card"><div class="mark">→</div><h1>${message}</h1><p>You can close this tab and return to the terminal.</p></div></body></html>`;
}

function expiry(epoch?: number) {
  if (!epoch) return "30 days";
  const days = Math.max(1, Math.round((epoch - Date.now() / 1000) / 86400));
  return `in ${days}d`;
}

export async function login(options: { url?: string; name?: string }, json = false) {
  const url = await promptInstanceUrl(options.url);
  const name = (options.name || hostname() || "this computer").slice(0, 60);
  const state = randomBytes(24).toString("hex");
  let complete: ((value: Auth | Error) => void) | undefined;
  let finished = false;
  let timer: NodeJS.Timeout | undefined;

  const settle = (value: Auth | Error) => {
    if (finished) return;
    finished = true;
    if (timer) clearTimeout(timer);
    server.close();
    complete?.(value);
  };

  const server = http.createServer((request, response) => {
    const address = new URL(request.url || "/", "http://127.0.0.1");
    if (address.pathname !== "/callback") {
      response.writeHead(404).end();
      return;
    }
    if (address.searchParams.get("state") !== state) {
      response.writeHead(400, { "Content-Type": "text/html" }).end(callbackPage("Authorization failed"));
      return;
    }
    const error = address.searchParams.get("error");
    if (error) {
      response.writeHead(200, { "Content-Type": "text/html" }).end(callbackPage("Authorization cancelled"));
      settle(new CliError(error === "forbidden" ? "Only an administrator can authorize the CLI" : "Authorization was cancelled"));
      return;
    }
    const token = address.searchParams.get("token") || "";
    if (!/^[a-f0-9]{64}$/i.test(token)) {
      response.writeHead(422, { "Content-Type": "text/html" }).end(callbackPage("Authorization failed"));
      settle(new CliError("Shipper did not return a valid CLI token"));
      return;
    }
    const auth: Auth = {
      url,
      token,
      tokenId: address.searchParams.get("id") || undefined,
      expiresAt: Number(address.searchParams.get("expiresAt")) || undefined,
      email: address.searchParams.get("email") || undefined,
      name,
    };
    saveConfig(auth);
    response.writeHead(200, { "Content-Type": "text/html" }).end(callbackPage("CLI authorized"));
    settle(auth);
  });

  const result = await new Promise<Auth>((resolve, reject) => {
    complete = value => (value instanceof Error ? reject(value) : resolve(value));
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        settle(new CliError("Could not start the local authorization listener"));
        return;
      }
      const authorize = new URL("/cli/authorize", url);
      authorize.searchParams.set("state", state);
      authorize.searchParams.set("port", String(address.port));
      authorize.searchParams.set("name", name);
      if (!json) {
        process.stdout.write(heading("Opening your browser\n"));
        process.stdout.write(muted("Authorize the CLI in Shipper, then return here.\n"));
        process.stdout.write(muted(`If the browser did not open, visit ${authorize.toString()}\n\n`));
      }
      openBrowser(authorize.toString());
      timer = setTimeout(() => settle(new CliError("Authorization timed out after 5 minutes")), 5 * 60 * 1000);
    });
  });

  try {
    const info = await capabilities(result);
    if (json) {
      printJson({ url: result.url, email: result.email, expiresAt: result.expiresAt, version: info.version });
      return;
    }
    process.stdout.write("\n" + statusText("Authorized") + "\n");
    process.stdout.write(table(["Instance", "Account", "Version", "Expires"], [[result.url, result.email || "unknown", info.version, expiry(result.expiresAt)]]));
    process.stdout.write("\n\n" + muted("Try: shipper apps") + "\n");
  } catch (error) {
    deleteConfig();
    throw error;
  }
}

export async function status(options: { url?: string; token?: string }, json = false) {
  const auth = resolveAuth(options);
  let version = "unreachable";
  try {
    version = (await capabilities(auth)).version;
  } catch {
    // Local credentials still show when the instance is offline.
  }
  if (json) {
    printJson({ url: auth.url, email: auth.email, name: auth.name, expiresAt: auth.expiresAt, version, config: configPath() });
    return;
  }
  process.stdout.write(heading("Shipper CLI\n\n"));
  process.stdout.write(table(["Instance", "Account", "Version", "Expires"], [[auth.url, auth.email || "unknown", version, expiry(auth.expiresAt)]]));
  process.stdout.write("\n" + muted(`Config ${configPath()}`) + "\n");
}

export function logout(json = false) {
  const config = loadConfig();
  deleteConfig();
  if (json) {
    printJson({ removed: true, config: configPath() });
    return;
  }
  if (config) process.stdout.write(statusText("Signed out") + "\n");
  else process.stdout.write(muted("No local CLI session was stored.") + "\n");
  process.stdout.write(muted("The token remains valid until it expires. Revoke it in System > Access tokens.") + "\n");
}
