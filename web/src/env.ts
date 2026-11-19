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

// The .env editor shows retained secrets with this mask. Leaving it in place keeps the saved value.
export const retainedValue = "••••••••";

function quote(value: string) {
  if (value === "" || /^[^\s"'#\\]*$/.test(value)) return value;
  return '"' + value.replaceAll("\\", "\\\\").replaceAll('"', '\\"').replaceAll("\n", "\\n") + '"';
}

// Like variablesForSave, but a revealed secret keeps its value so the editor can show it.
const editorRows = (variables: Variable[]) => variables.filter(variableHasContent).map(variable => ({ ...variable, name: variable.name.trim() }));

export function formatEnvironment(variables: Variable[]): string {
  return editorRows(variables).map(v => v.name + "=" + (v.keep && !v.value ? retainedValue : quote(v.value))).join("\n");
}

function unquote(raw: string, line: number) {
  const value = raw.trim();
  if (value.startsWith("'")) {
    if (value.length < 2 || !value.endsWith("'")) throw Error(`Line ${line} has an unclosed quote`);
    return value.slice(1, -1);
  }
  if (!value.startsWith('"')) return value;
  let result = "", closed = false;
  for (let i = 1; i < value.length; i++) {
    const char = value[i];
    if (char === "\\" && i + 1 < value.length) {
      const next = value[++i];
      result += next === "n" ? "\n" : next === "t" ? "\t" : next;
    } else if (char === '"') {
      if (value.slice(i + 1).trim()) throw Error(`Line ${line} has text after the closing quote`);
      closed = true;
      break;
    } else result += char;
  }
  if (!closed) throw Error(`Line ${line} has an unclosed quote`);
  return result;
}

// Parses the editor text against the saved rows so secrets, scopes and retained values survive.
// New names default to build and runtime.
export function parseEnvironmentFile(text: string, current: Variable[]): Variable[] {
  if (text.length > 128 * 1024) throw Error("The environment file is too large");
  const previous = new Map(editorRows(current).map(v => [v.name, v]));
  const variables: Variable[] = [], names = new Set<string>();
  for (const [index, raw] of text.replace(/^\uFEFF/, "").replaceAll("\r\n", "\n").split("\n").entries()) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const match = /^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]{0,127})\s*=(.*)$/.exec(line);
    if (!match) throw Error(`Line ${index + 1} must contain NAME=value`);
    const name = match[1];
    if (names.has(name)) throw Error(`Variable ${name} appears more than once`);
    names.add(name);
    const before = previous.get(name);
    if (match[2].trim() === retainedValue) {
      if (!before?.keep) throw Error(`Enter a value for ${name}`);
      variables.push({ ...before });
      continue;
    }
    const value = unquote(match[2], index + 1);
    if (value.length > 16384 || value.includes("\0")) throw Error(`Variable ${name} is too large or contains null bytes`);
    // Unchanged rows stay as saved, including secrets the editor revealed.
    if (before && before.value === value && (!before.keep || value !== "")) variables.push({ ...before });
    else if (before?.keep) variables.push({ ...before, value, keep: false, keepFrom: undefined, hasValue: undefined });
    else variables.push(before ? { ...before, value } : { ...emptyVariable(), name, value });
  }
  if (variables.length > 256) throw Error("Applications support up to 256 variables");
  return variables;
}
