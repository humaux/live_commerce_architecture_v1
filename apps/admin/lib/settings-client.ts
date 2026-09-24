import type { APIError } from "./model";

export type Pending = {
  key: string;
  method: "POST" | "PUT";
  resource: string;
  body?: string;
  secret?: "create" | "rotate";
  context?: {
    provider?: "payuni";
    environment?: string;
    account_id?: string;
    expected_version?: number;
  };
};

const cookieName = "__Host-commerce_csrf=";
export function csrfCookie() {
  const found = document.cookie
    .split(";")
    .map((item) => item.trim())
    .filter((item) => item.startsWith(cookieName));
  const value = found.length === 1 ? found[0].slice(cookieName.length) : "";
  return /^[A-Za-z0-9_-]{43}$/.test(value) ? value : "";
}

export async function sessionBoundary(cookie = csrfCookie()) {
  if (!cookie) throw new Error("session_changed");
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(cookie),
  );
  // Hashing yields to the browser: reject a cookie changed during that await.
  if (csrfCookie() !== cookie) throw new Error("session_changed");
  return [...new Uint8Array(digest)]
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

export function safeError(value: unknown): APIError {
  const item =
    value && typeof value === "object"
      ? (value as Record<string, unknown>)
      : {};
  return {
    code: typeof item.code === "string" ? item.code : "retry_later",
    message: "",
    request_id: typeof item.request_id === "string" ? item.request_id : "",
    retryable: item.retryable === true,
    details: {},
  };
}

export async function readSettings<T>(
  store: string,
  resource: string,
): Promise<T> {
  const response = await fetch(`/api/stores/${store}/${resource}`, {
    cache: "no-store",
    credentials: "same-origin",
  });
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw safeError(null);
  }
  if (!response.ok) throw safeError(body);
  return body as T;
}

export async function writeSettings<T>(
  store: string,
  pending: Pending,
  body: string,
  boundary: string,
) {
  // Recheck after journal/lock work and directly before the network write.
  const csrf = csrfCookie();
  if (
    !csrf ||
    (await sessionBoundary(csrf)) !== boundary ||
    csrfCookie() !== csrf
  )
    throw new Error("session_changed");
  let response: Response;
  try {
    response = await fetch(`/api/stores/${store}/${pending.resource}`, {
      method: pending.method,
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": pending.key,
        "X-CSRF-Token": csrf,
      },
      body,
      signal: AbortSignal.timeout(8000),
    });
  } catch {
    return { uncertain: true, error: safeError(null), value: null as T | null };
  }
  let value: unknown;
  try {
    value = await response.json();
  } catch {
    return { uncertain: true, error: safeError(null), value: null as T | null };
  }
  if (response.status >= 500)
    return {
      uncertain: true,
      error: safeError(value),
      value: null as T | null,
    };
  if (!response.ok)
    return {
      uncertain: false,
      error: safeError(value),
      value: null as T | null,
    };
  return { uncertain: false, error: null, value: value as T };
}

export function validCode(value: string) {
  return /^[a-z][a-z0-9_-]{0,39}$/.test(value);
}
export function validCountry(value: string) {
  return /^[A-Z]{2}$/.test(value);
}
export function integer(value: string, min: number, max: number) {
  if (!value.trim()) return null;
  const result = Number(value);
  return Number.isSafeInteger(result) && result >= min && result <= max
    ? result
    : null;
}
