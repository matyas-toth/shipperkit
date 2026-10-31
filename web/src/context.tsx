import { createContext, useContext, type Accessor } from "solid-js";
import type { Inventory, Session } from "./types";

export const AuthContext = createContext<{ session: Accessor<Session | undefined>; logout: () => Promise<void> }>();
export const InventoryContext = createContext<{
  data: Accessor<Inventory | undefined>; pending: Accessor<boolean>; error: Accessor<Error | null>;
  live: Accessor<boolean>; refresh: () => Promise<void>; refreshing: Accessor<boolean>;
}>();
export function useAuth() { const value = useContext(AuthContext); if (!value) throw Error("Auth context is unavailable"); return value; }
export function useInventory() { const value = useContext(InventoryContext); if (!value) throw Error("Inventory context is unavailable"); return value; }

