import type { Problem } from "./types";

const API_BASE = "/api/v1";

let csrfToken: string | null = null;

export function setCsrfToken(token: string | null) {
  csrfToken = token;
}

export function getCsrfToken(): string | null {
  return csrfToken;
}

export class ApiError extends Error {
  status: number;
  problem?: Problem;

  constructor(status: number, message: string, problem?: Problem) {
    super(message);
    this.status = status;
    this.problem = problem;
  }
}

async function parseProblem(res: Response): Promise<Problem | undefined> {
  const ct = res.headers.get("content-type") ?? "";
  if (ct.includes("application/problem+json") || ct.includes("application/json")) {
    try {
      return (await res.json()) as Problem;
    } catch {
      return undefined;
    }
  }
  return undefined;
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  const method = (init.method ?? "GET").toUpperCase();
  const isMutation = !["GET", "HEAD", "OPTIONS"].includes(method);

  if (isMutation && csrfToken) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  if (res.status === 204) {
    return undefined as T;
  }

  if (!res.ok) {
    const problem = await parseProblem(res);
    throw new ApiError(
      res.status,
      problem?.detail ?? problem?.title ?? `HTTP ${res.status}`,
      problem,
    );
  }

  const ct = res.headers.get("content-type") ?? "";
  if (ct.includes("application/json")) {
    return (await res.json()) as T;
  }

  return (await res.text()) as T;
}

export async function apiFetchRaw(
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  const headers = new Headers(init.headers);
  const method = (init.method ?? "GET").toUpperCase();
  const isMutation = !["GET", "HEAD", "OPTIONS"].includes(method);

  if (isMutation && csrfToken) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    const problem = await parseProblem(res);
    throw new ApiError(
      res.status,
      problem?.detail ?? problem?.title ?? `HTTP ${res.status}`,
      problem,
    );
  }

  return res;
}

export async function checkSetupStatus(): Promise<"needs_setup" | "configured"> {
  try {
    await apiFetch("/auth/setup", { method: "POST", body: "{}" });
    return "needs_setup";
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      return "configured";
    }
    return "needs_setup";
  }
}

export async function healthCheck(path: string): Promise<boolean> {
  try {
    const res = await fetch(path, { credentials: "include" });
    return res.ok;
  } catch {
    return false;
  }
}
