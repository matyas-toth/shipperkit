import { stdin } from "node:process";
import type { Client } from "./client";
import { printJson } from "./output";

async function readStdin() {
  const chunks: Buffer[] = [];
  for await (const chunk of stdin) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString("utf8");
}

export async function api(client: Client, method: string, path: string, body?: string) {
  let route = path.trim();
  if (route.startsWith("/api/v1")) route = route.slice("/api/v1".length);
  else if (route.startsWith("/api")) route = route.slice("/api".length);
  if (!route.startsWith("/")) route = "/" + route;
  const verb = method.toUpperCase();
  let payload: unknown;
  if (body !== undefined) {
    const raw = body === "-" ? await readStdin() : body;
    try {
      payload = JSON.parse(raw);
    } catch {
      throw new Error("Send the request body as JSON, or - to read it from stdin");
    }
  } else if (verb !== "GET" && verb !== "HEAD") {
    payload = {};
  }
  const result = await client.request(route, { method: verb, body: payload });
  printJson(result ?? {});
}
