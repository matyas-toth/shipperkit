import type { Variable } from "./types";

export const variableScopeLabel = (scope: Variable["scope"] | undefined) =>
  scope === "build" ? "Build" : scope === "both" ? "Build and runtime" : "Runtime";

export const emptyVariable = (): Variable => ({ name: "", value: "", secret: true, scope: "both" });
export const variableHasContent = (variable: Variable) => !!(variable.name.trim() || variable.value || variable.keep);
export const variablesForSave = (variables: Variable[]) => variables.filter(variableHasContent).map(variable => ({ ...variable, name: variable.name.trim(), value: variable.keep ? "" : variable.value }));

// Revealing a retained value is a display change, not a configuration change.
export function variableSignature(variables: Variable[]): string {
  return JSON.stringify(variablesForSave(variables).map(v => ({ name: v.name, value: v.keep ? "" : v.value, secret: v.secret, scope: v.scope || "runtime", keep: !!v.keep, keepFrom: v.keep ? v.keepFrom || v.name : "" })));
}

export function looksLikeEnvironment(text: string): boolean {
  const first = text.split(/\r?\n/).find(line => line.trim() && !line.trim().startsWith("#"));
  return !!first && /^(?:export\s+)?[A-Za-z_][A-Za-z0-9_]*\s*=/.test(first.trim());
}

// Paste stays local until Save and deploy. Do not evaluate shell expressions.
export function parseEnvironment(text: string): Variable[] {
  if (text.length > 128 * 1024) throw Error("The environment file is too large");
  const variables: Variable[] = [], names = new Set<string>();
  for (const [index, raw] of text.replace(/^\uFEFF/, "").replaceAll("\r\n", "\n").split("\n").entries()) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const match = /^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]{0,127})\s*=(.*)$/.exec(line);
    if (!match) throw Error(`Line ${index + 1} must contain NAME=value`);
    const name = match[1];let value = match[2].trim();
    if (names.has(name)) throw Error(`Variable ${name} appears more than once`);
    if (value.length >= 2 && ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'")))) value = value.slice(1, -1);
    if (value.length > 16384 || value.includes("\0")) throw Error(`Variable ${name} is too large or contains null bytes`);
    names.add(name);variables.push({ ...emptyVariable(), name, value });
  }
  if (variables.length > 128) throw Error("Applications support up to 128 variables");
  return variables;
}

export function mergeEnvironment(current: Variable[], pasted: Variable[]): Variable[] {
  const merged = variablesForSave(current).map(v => ({ ...v }));
  for (const variable of pasted) {
    const index = merged.findIndex(v => v.name === variable.name);
    if (index < 0) merged.push(variable);
    else merged[index] = { ...merged[index], value: variable.value, keep: false, keepFrom: undefined, hasValue: undefined };
  }
  if (merged.length > 128) throw Error("Applications support up to 128 variables");
  return merged;
}
