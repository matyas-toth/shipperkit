import { readFileSync } from "node:fs";
import { join } from "node:path";

const policy: { names: string[]; suffixes: string[]; envExamples: string[] } = JSON.parse(readFileSync(join(__dirname, "source-policy.json"), "utf8"));

export function excludedSource(name: string) {
  return name.split("/").some(part => {
    const v = part.toLowerCase();
    return policy.names.includes(v) || policy.suffixes.some(suffix => v.endsWith(suffix)) || v.startsWith(".env.") && !policy.envExamples.includes(v);
  });
}
