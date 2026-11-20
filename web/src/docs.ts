export const DOCS_ORIGIN = "https://shipperkit.dev";

/** Community Discord invite. The subdomain redirects to the current invite. */
export const DISCORD_URL = "https://discord.shipperkit.dev";

/** Stable public documentation URL for in-app help links. */
export function docs(path = ""): string {
  const suffix = path.replace(/^\/+/, "");
  return `${DOCS_ORIGIN}/docs/${suffix}`;
}
