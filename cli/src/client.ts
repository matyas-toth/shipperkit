import type { Auth } from "./config";

export class CliError extends Error {
  constructor(message: string, readonly status = 0, readonly code = "") {
    super(message);
    this.name = "CliError";
  }
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
  timeout?: number;
  headers?: Record<string, string>;
};

export class Client {
  constructor(readonly auth: Auth) {}

  private endpoint(path: string) {
    return `${this.auth.url}/api/v1${path.startsWith("/") ? path : "/" + path}`;
  }

  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const headers: Record<string, string> = {
      Authorization: `Bearer ${this.auth.token}`,
      Accept: "application/json",
      ...options.headers,
    };
    let body: string | undefined;
    if (options.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(options.body);
    }
    const timeout = options.timeout ?? 30_000;
    const signals = [AbortSignal.timeout(timeout)];
    if (options.signal) signals.push(options.signal);
    let response: Response;
    try {
      response = await fetch(this.endpoint(path), {
        method: options.method ?? "GET",
        headers,
        body,
        signal: AbortSignal.any(signals),
      });
    } catch (error) {
      if (error instanceof Error && error.name === "TimeoutError") {
        throw new CliError(`Shipper did not answer within ${Math.round(timeout / 1000)} seconds`);
      }
      throw new CliError(`Could not reach ${this.auth.url}`);
    }
    if (!response.ok) {
      let message = `Shipper returned HTTP ${response.status}`;
      let code = "";
      try {
        const payload = (await response.json()) as { error?: { code?: string; message?: string } };
        message = payload.error?.message || message;
        code = payload.error?.code || "";
      } catch {
        // Non-JSON failures keep the HTTP status text.
      }
      throw new CliError(message, response.status, code);
    }
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
  }

  get<T>(path: string, options: Omit<RequestOptions, "method" | "body"> = {}) {
    return this.request<T>(path, options);
  }

  post<T>(path: string, body: unknown = {}, options: Omit<RequestOptions, "method" | "body"> = {}) {
    return this.request<T>(path, { ...options, method: "POST", body });
  }

  async upload<T>(path: string, body: Buffer, contentType: string): Promise<T> {
    let response: Response;
    try {
      response = await fetch(this.endpoint(path), {
        method: "POST",
        headers: {
          Authorization: `Bearer ${this.auth.token}`,
          Accept: "application/json",
          "Content-Type": contentType,
        },
        body: new Uint8Array(body),
        signal: AbortSignal.timeout(120_000),
      });
    } catch (error) {
      if (error instanceof Error && error.name === "TimeoutError") throw new CliError("The source upload timed out");
      throw new CliError(`Could not reach ${this.auth.url}`);
    }
    if (!response.ok) {
      let message = `Shipper returned HTTP ${response.status}`;
      let code = "";
      try {
        const payload = (await response.json()) as { error?: { code?: string; message?: string } };
        message = payload.error?.message || message;
        code = payload.error?.code || "";
      } catch {
        // Keep the status message.
      }
      throw new CliError(message, response.status, code);
    }
    return (await response.json()) as T;
  }

  async stream(path: string, signal: AbortSignal) {
    let response: Response;
    try {
      response = await fetch(this.endpoint(path), {
        headers: {
          Authorization: `Bearer ${this.auth.token}`,
          Accept: "text/event-stream",
        },
        signal,
      });
    } catch {
      throw new CliError(`Could not reach ${this.auth.url}`);
    }
    if (!response.ok) {
      let message = `Shipper returned HTTP ${response.status}`;
      try {
        const payload = (await response.json()) as { error?: { message?: string } };
        message = payload.error?.message || message;
      } catch {
        // Keep the status message.
      }
      throw new CliError(message, response.status);
    }
    if (!response.body) throw new CliError("Shipper returned an empty log stream");
    return response.body;
  }
}

export async function capabilities(auth: Auth) {
  return new Client(auth).get<{ version: string; scopes: string[]; instanceId: string }>("/capabilities");
}
