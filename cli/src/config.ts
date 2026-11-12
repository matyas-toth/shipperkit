import { chmodSync, existsSync, mkdirSync, readFileSync, renameSync, unlinkSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join } from "node:path";

export type Auth = {
  url: string;
  token: string;
  tokenId?: string;
  expiresAt?: number;
  email?: string;
  name?: string;
};

export function configPath() {
  if (process.env.SHIPPER_CONFIG) return process.env.SHIPPER_CONFIG;
  const base = process.env.SHIPPER_CONFIG_DIR
    ? process.env.SHIPPER_CONFIG_DIR
    : process.platform === "win32"
      ? join(process.env.APPDATA || join(homedir(), "AppData", "Roaming"), "shipper")
      : join(process.env.XDG_CONFIG_HOME || join(homedir(), ".config"), "shipper");
  return join(base, "config.json");
}

export function normalizeUrl(input: string) {
  let value = input.trim();
  if (!value) throw new Error("Enter your Shipper instance URL");
  if (!/^https?:\/\//i.test(value)) value = "https://" + value;
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new Error("Enter a valid Shipper instance URL");
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("Shipper needs an http or https URL");
  return url.origin + url.pathname.replace(/\/+$/, "");
}

export function loadConfig(): Auth | null {
  const path = configPath();
  if (!existsSync(path)) return null;
  try {
    const value = JSON.parse(readFileSync(path, "utf8")) as Partial<Auth>;
    if (!value.url || !value.token) return null;
    return {
      url: normalizeUrl(value.url),
      token: value.token,
      tokenId: value.tokenId,
      expiresAt: value.expiresAt,
      email: value.email,
      name: value.name,
    };
  } catch {
    return null;
  }
}

export function saveConfig(auth: Auth) {
  const path = configPath();
  mkdirSync(dirname(path), { recursive: true, mode: 0o700 });
  const temporary = path + ".tmp";
  writeFileSync(temporary, JSON.stringify(auth, null, 2) + "\n", { mode: 0o600 });
  try {
    chmodSync(temporary, 0o600);
  } catch {
    // Windows ACLs are managed by the user profile.
  }
  renameSync(temporary, path);
}

export function deleteConfig() {
  const path = configPath();
  if (existsSync(path)) unlinkSync(path);
}

export function resolveAuth(options: { url?: string; token?: string } = {}): Auth {
  const config = loadConfig();
  const url = options.url || process.env.SHIPPER_URL || config?.url;
  const token = options.token || process.env.SHIPPER_TOKEN || config?.token;
  if (!url) throw new Error("No Shipper instance configured. Run shipper auth login, or set SHIPPER_URL.");
  if (!token) throw new Error("No Shipper token configured. Run shipper auth login, or set SHIPPER_TOKEN.");
  if (!/^[a-f0-9]{64}$/i.test(token)) throw new Error("The Shipper token is not valid");
  return {
    url: normalizeUrl(url),
    token,
    tokenId: config?.tokenId,
    expiresAt: config?.expiresAt,
    email: config?.email,
    name: config?.name,
  };
}
