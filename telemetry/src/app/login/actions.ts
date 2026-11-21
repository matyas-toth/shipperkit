"use server";

import { cookies, headers } from "next/headers";
import { redirect } from "next/navigation";
import { SESSION_COOKIE, createSession, keyMatches } from "@/lib/session";

const attempts = new Map<string, { count: number; reset: number }>();
const WINDOW_MS = 10 * 60 * 1000;
const LIMIT = 10;

export type LoginState = { error?: string };

export async function login(_: LoginState, form: FormData): Promise<LoginState> {
  const source = (await headers()).get("x-forwarded-for")?.split(",")[0]?.trim() || "local";
  const now = Date.now();
  const entry = attempts.get(source);
  if (entry && entry.reset > now && entry.count >= LIMIT) return { error: "Too many attempts. Try again in a few minutes." };
  const key = String(form.get("key") ?? "").trim();
  if (!key || !(await keyMatches(key))) {
    attempts.set(source, entry && entry.reset > now ? { count: entry.count + 1, reset: entry.reset } : { count: 1, reset: now + WINDOW_MS });
    return { error: "That admin key is not valid." };
  }
  attempts.delete(source);
  const session = await createSession();
  (await cookies()).set(SESSION_COOKIE, session.value, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    expires: session.expires,
  });
  redirect("/");
}

export async function logout() {
  (await cookies()).delete(SESSION_COOKIE);
  redirect("/login");
}
