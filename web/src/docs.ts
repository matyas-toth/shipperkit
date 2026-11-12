export const DOCS_ORIGIN = "https://shipperkit.dev";

/** Stable public documentation URL for in-app help links. */
export function docs(path = ""): string {
  const suffix = path.replace(/^\/+/, "");
  return `${DOCS_ORIGIN}/docs/${suffix}`;
}
