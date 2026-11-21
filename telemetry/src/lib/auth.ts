import "server-only";
import { cookies } from "next/headers";
import { connection } from "next/server";
import { redirect } from "next/navigation";
import { SESSION_COOKIE, verifySession } from "./session";

/** Every data read calls this, so access never depends on Proxy alone. */
export async function requireAdmin() {
  await connection();
  const store = await cookies();
  if (!(await verifySession(store.get(SESSION_COOKIE)?.value))) redirect("/login");
}
