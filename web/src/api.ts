export class APIError extends Error {
  constructor(public status: number, public code: string, message: string) { super(message); }
}
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch("/api" + path, {
    credentials: "same-origin",
    ...init,
    headers: { ...(init?.body ? { "Content-Type": "application/json" } : {}), ...init?.headers },
  });
  const body = await response.json();
  if (!response.ok) {
    if (response.status === 403 && body.error?.code === "factor_required" && !path.startsWith("/auth/")) {
      window.dispatchEvent(new Event("shipper:session-changed"));
    }
    if (response.status === 401 && !path.startsWith("/auth/")) {
      window.dispatchEvent(new Event("shipper:session-expired"));
    }
    throw new APIError(response.status, body.error?.code ?? "request_failed", body.error?.message ?? "The request failed");
  }
  return body as T;
}
export function post<T>(path: string, body: unknown, csrf?: string, idempotencyKey?:string) {
  return api<T>(path, { method: "POST", body: JSON.stringify(body), headers: {...(csrf ? { "X-CSRF-Token": csrf } : {}),...(idempotencyKey?{"Idempotency-Key":idempotencyKey}:{})} });
}

