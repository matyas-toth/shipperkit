import { createContext, useContext, type Accessor } from "solid-js";
import type { Inventory, Session, UpdateChannel, UpdateState } from "./types";

export const AuthContext = createContext<{ session: Accessor<Session | undefined>; logout: () => Promise<void>; accept: (session: Session) => void }>();
export const InventoryContext = createContext<{
  data: Accessor<Inventory | undefined>; pending: Accessor<boolean>; error: Accessor<Error | null>;
  live: Accessor<boolean>; refresh: () => Promise<void>; refreshing: Accessor<boolean>;
}>();
export const UpdatesContext = createContext<{
  data: Accessor<UpdateState | undefined>; pending: Accessor<boolean>; error: Accessor<Error | null>;
  loadedVersion: Accessor<string>; check: () => Promise<void>; save: (channel: UpdateChannel, auto: boolean) => Promise<UpdateState>;
  apply: () => Promise<void>; open: () => void;
}>();
export function useAuth() { const value = useContext(AuthContext); if (!value) throw Error("Auth context is unavailable"); return value; }
export function useInventory() { const value = useContext(InventoryContext); if (!value) throw Error("Inventory context is unavailable"); return value; }
export function useUpdates() { const value = useContext(UpdatesContext); if (!value) throw Error("Updates context is unavailable"); return value; }

