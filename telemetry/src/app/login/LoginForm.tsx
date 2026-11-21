"use client";

import { useActionState } from "react";
import { IconAlertTriangle } from "@tabler/icons-react";
import { login, type LoginState } from "./actions";

export function LoginForm() {
  const [state, action, pending] = useActionState<LoginState, FormData>(login, {});
  return (
    <form action={action} className="grid gap-4">
      <div className="grid gap-2">
        <label htmlFor="key" className="text-[13px] font-[520] text-secondary">
          Admin key
        </label>
        <input
          id="key"
          name="key"
          type="password"
          required
          autoFocus
          autoComplete="current-password"
          spellCheck={false}
          aria-invalid={state.error ? true : undefined}
          className="h-10 w-full rounded-control border border-border bg-deep px-3 font-mono text-[13px] text-text outline-none transition-colors placeholder:text-muted focus:border-[#4a4b4b] aria-invalid:border-danger/60"
        />
      </div>
      {state.error && (
        <p role="alert" className="flex items-center gap-2 text-[13px] text-danger">
          <IconAlertTriangle size={16} stroke={1.75} />
          {state.error}
        </p>
      )}
      <button
        type="submit"
        disabled={pending}
        className="h-11 rounded-control bg-[#f2f2f2] text-[14px] font-[560] text-[#0b0b0b] transition-opacity hover:opacity-90 disabled:opacity-60"
      >
        {pending ? "Signing in" : "Sign in"}
      </button>
    </form>
  );
}
