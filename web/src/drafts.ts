import type { StaticConfig } from "./types";

export function readDraft<T>(key: string): T | undefined { try { const text = sessionStorage.getItem("shipper:v3:" + key);return text ? JSON.parse(text) as T : undefined; } catch { return undefined; } }
export function saveDraft(key: string, value: unknown) { try { sessionStorage.setItem("shipper:v3:" + key, JSON.stringify(value)); } catch {} }
export function clearDraft(key: string) { try { sessionStorage.removeItem("shipper:v3:" + key); } catch {} }
export function safeConfigDraft(config: StaticConfig): StaticConfig {
  return { ...config, composeYAML: "", composeKeep: config.composeKeep || config.composeHasValue || undefined, services: config.services?.map(s => ({ ...s, variables: (s.variables ?? []).filter(v => !v.secret || v.keep).map(v => v.secret ? { ...v, value: "" } : { ...v }) })), variables: (config.variables ?? []).filter(v => !v.secret || v.keep).map(v => v.secret ? { ...v, value: "" } : { ...v }) };
}
