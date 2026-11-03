import type { StaticConfig } from "./types";

const volatileDrafts = new Map<string,string>();
export function saveVolatileDraft(key:string,value:unknown){volatileDrafts.delete(key);volatileDrafts.set(key,JSON.stringify(value));if(volatileDrafts.size>64)volatileDrafts.delete(volatileDrafts.keys().next().value!);}
export function clearVolatileDrafts(){volatileDrafts.clear();}
export function readDraft<T>(key: string): T | undefined { try { const text = volatileDrafts.get(key) ?? sessionStorage.getItem("shipper:v3:" + key);return text ? JSON.parse(text) as T : undefined; } catch { return undefined; } }
export function saveDraft(key: string, value: unknown) { try { sessionStorage.setItem("shipper:v3:" + key, JSON.stringify(value)); } catch {} }
export function clearDraft(key: string) { volatileDrafts.delete(key);try { sessionStorage.removeItem("shipper:v3:" + key); } catch {} }
export function safeConfigDraft(config: StaticConfig): StaticConfig {
  return { ...config, composeYAML: "", composeKeep: config.composeKeep || config.composeHasValue || undefined, services: config.services?.map(s => ({ ...s, variables: (s.variables ?? []).filter(v => !v.secret || v.keep).map(v => v.secret ? { ...v, value: "" } : { ...v }) })), variables: (config.variables ?? []).filter(v => !v.secret || v.keep).map(v => v.secret ? { ...v, value: "" } : { ...v }) };
}
