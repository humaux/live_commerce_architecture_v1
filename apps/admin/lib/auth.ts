import "server-only";
import { timingSafeEqual } from "node:crypto";
import type { APIError, Store } from "./model";

export const LOGIN_COOKIE = "__Host-commerce_login";
export const SESSION_COOKIE = "__Host-commerce_session";
export const CSRF_COOKIE = "__Host-commerce_csrf";
export const locales = ["zh-CN", "zh-TW", "en"] as const;
export type Locale = (typeof locales)[number];

type AuthConfig = {
  publicOrigin: string;
  apiOrigin: string;
  issuer: string;
  bffKey: string;
  allowLoopback: boolean;
};

export function isBase64URL32(value: string) {
  if (!/^[A-Za-z0-9_-]{43}$/.test(value)) return false;
  const bytes = Buffer.from(value, "base64url");
  return bytes.byteLength === 32 && bytes.toString("base64url") === value;
}

function exactOrigin(value: string, allowLoopback: boolean) {
  const url = new URL(value);
  const loopback =
    url.protocol === "http:" &&
    (url.hostname === "127.0.0.1" ||
      url.hostname === "[::1]" ||
      url.hostname === "localhost");
  if (
    (url.protocol !== "https:" && !(allowLoopback && loopback)) ||
    url.pathname !== "/" ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  )
    throw new Error("invalid commerce origin configuration");
  return url.origin;
}

function exactIssuer(value: string, allowLoopback: boolean) {
  const url = new URL(value);
  const loopback =
    url.protocol === "http:" &&
    (url.hostname === "127.0.0.1" ||
      url.hostname === "[::1]" ||
      url.hostname === "localhost");
  if (
    (url.protocol !== "https:" && !(allowLoopback && loopback)) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  )
    throw new Error("invalid commerce issuer configuration");
  return value;
}

function required(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`missing ${name}`);
  return value;
}

function readConfig(): AuthConfig | null {
  const enabled = process.env.COMMERCE_IDENTITY_ENABLED ?? "";
  if (enabled === "" || enabled === "0") return null;
  if (enabled !== "1") throw new Error("invalid COMMERCE_IDENTITY_ENABLED");
  if (process.env.COMMERCE_FIXTURE_ENABLED === "1")
    throw new Error("identity and fixture modes are mutually exclusive");
  const allowLoopback =
    process.env.COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS === "1";
  const key = required("COMMERCE_BFF_KEY");
  if (!isBase64URL32(key)) throw new Error("invalid COMMERCE_BFF_KEY");
  return {
    publicOrigin: exactOrigin(
      required("COMMERCE_PUBLIC_ORIGIN"),
      allowLoopback,
    ),
    apiOrigin: exactOrigin(required("COMMERCE_API_ORIGIN"), true),
    issuer: exactIssuer(required("COMMERCE_OIDC_ISSUER"), allowLoopback),
    bffKey: key,
    allowLoopback,
  };
}

export const authConfig = readConfig();

const messages: Record<string, string> = {
  unauthorized: "Sign-in required.",
  forbidden: "Access denied.",
  not_found: "Resource not found.",
  invalid_request: "Invalid request.",
  invalid_json: "Invalid JSON body.",
  json_required: "JSON content required.",
  retry_later: "Temporarily unavailable.",
  method_not_allowed: "Method not allowed.",
};

export function localError(status: number, code: string, allow?: string) {
  const requestID = crypto.randomUUID().replaceAll("-", "");
  const body: APIError = {
    code,
    message: messages[code] ?? "Request failed.",
    request_id: requestID,
    retryable: status >= 500,
    details: {},
  };
  return Response.json(body, {
    status,
    headers: {
      "Cache-Control": "no-store",
      "X-Request-ID": requestID,
      ...(allow ? { Allow: allow } : {}),
    },
  });
}

export function disabledResponse() {
  return localError(404, "not_found");
}

export function isLocale(value: string): value is Locale {
  return locales.includes(value as Locale);
}

function cookieParts(cookieHeader: string | null, name: string) {
  const values: string[] = [];
  for (const part of (cookieHeader ?? "").split(";")) {
    const at = part.indexOf("=");
    if (at < 0 || part.slice(0, at).trim() !== name) continue;
    values.push(part.slice(at + 1).trim());
  }
  return values;
}

export function exactCookie(request: Request, name: string) {
  return exactCookieHeader(request.headers.get("cookie"), name);
}

export function exactCookieHeader(cookieHeader: string | null, name: string) {
  const values = cookieParts(cookieHeader, name);
  return values.length === 1 ? values[0] : null;
}

export function sessionToken(request: Request) {
  const value = exactCookie(request, SESSION_COOKIE);
  return value && isBase64URL32(value) ? value : null;
}

function secureEqual(a: string, b: string) {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  return left.byteLength === right.byteLength && timingSafeEqual(left, right);
}

export function requireOrigin(request: Request) {
  return (
    !!authConfig && request.headers.get("origin") === authConfig.publicOrigin
  );
}

export function requireCSRF(request: Request) {
  const cookie = exactCookie(request, CSRF_COOKIE);
  const header = request.headers.get("x-csrf-token");
  return !!(
    cookie &&
    header &&
    isBase64URL32(cookie) &&
    isBase64URL32(header) &&
    secureEqual(cookie, header)
  );
}

export function hasNoQuery(request: Request) {
  return new URL(request.url).search === "";
}

export async function readBody(request: Request, mime: string, limit = 65536) {
  if (
    request.headers
      .get("content-type")
      ?.split(";", 1)[0]
      ?.trim()
      .toLowerCase() !== mime
  )
    throw new Error("mime");
  const length = Number(request.headers.get("content-length") ?? "0");
  if (!Number.isFinite(length) || length < 0 || length > limit)
    throw new Error("size");
  const reader = request.body?.getReader();
  if (!reader) return "";
  const chunks: Uint8Array[] = [];
  let size = 0;
  while (true) {
    const part = await reader.read();
    if (part.done) break;
    size += part.value.byteLength;
    if (size > limit) {
      await reader.cancel();
      throw new Error("size");
    }
    chunks.push(part.value);
  }
  const bytes = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
}

export async function exactJSON(
  request: Request,
  keys: readonly string[],
): Promise<Record<string, unknown>> {
  const text = await readBody(request, "application/json");
  const value: unknown = JSON.parse(text);
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new Error("json");
  const record = value as Record<string, unknown>;
  const actual = Object.keys(record).sort();
  const expected = [...keys].sort();
  if (
    actual.length !== expected.length ||
    actual.some((key, i) => key !== expected[i])
  )
    throw new Error("fields");
  return record;
}

function privateHeaders(token?: string) {
  if (!authConfig) throw new Error("identity disabled");
  return {
    Accept: "application/json",
    "Content-Type": "application/json",
    "X-Commerce-BFF-Key": authConfig.bffKey,
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

export async function privateIdentity(
  path: string,
  body: unknown,
  token?: string,
  idempotencyKey?: string,
) {
  if (!authConfig) return disabledResponse();
  try {
    return await fetch(`${authConfig.apiOrigin}/v1/identity/${path}`, {
      method: "POST",
      headers: {
        ...privateHeaders(token),
        ...(idempotencyKey ? { "Idempotency-Key": idempotencyKey } : {}),
      },
      body: JSON.stringify(body),
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(6000),
    });
  } catch {
    return localError(503, "retry_later");
  }
}

export async function merchantBackend(
  path: string,
  token: string,
  init: RequestInit = {},
) {
  if (!authConfig) return disabledResponse();
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  headers.set("Authorization", `Bearer ${token}`);
  headers.delete("Cookie");
  headers.delete("X-Commerce-BFF-Key");
  try {
    return await fetch(`${authConfig.apiOrigin}${path}`, {
      ...init,
      headers,
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(6000),
    });
  } catch {
    return localError(503, "retry_later");
  }
}

export async function safeJSON<T>(response: Response): Promise<T | null> {
  if (
    response.headers.get("content-type")?.split(";", 1)[0] !==
    "application/json"
  )
    return null;
  try {
    return (await response.json()) as T;
  } catch {
    return null;
  }
}

export async function safeError(response: Response) {
  const body = await safeJSON<Partial<APIError>>(response);
  const code =
    typeof body?.code === "string" && /^[a-z0-9_]{1,64}$/.test(body.code)
      ? body.code
      : "retry_later";
  return localError(
    response.status >= 400 && response.status <= 599 ? response.status : 503,
    code,
  );
}

function maxAge(expiresAt: unknown, ceiling: number) {
  if (typeof expiresAt !== "string") return 0;
  const remaining = Math.floor((Date.parse(expiresAt) - Date.now()) / 1000);
  return Number.isFinite(remaining) && remaining > 0
    ? Math.min(remaining, ceiling)
    : 0;
}

function cookie(name: string, value: string, age: number, httpOnly: boolean) {
  return `${name}=${value}; Path=/; Max-Age=${age}; Secure; ${httpOnly ? "HttpOnly; " : ""}SameSite=Lax`;
}

export function clearAuthCookies(headers: Headers, includeLogin = false) {
  const expired =
    "Path=/; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Secure; HttpOnly; SameSite=Lax";
  headers.append("Set-Cookie", `${SESSION_COOKIE}=; ${expired}`);
  headers.append(
    "Set-Cookie",
    `${CSRF_COOKIE}=; Path=/; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Secure; SameSite=Lax`,
  );
  if (includeLogin)
    headers.append("Set-Cookie", `${LOGIN_COOKIE}=; ${expired}`);
}

export function clearLoginCookie(headers: Headers) {
  headers.append(
    "Set-Cookie",
    `${LOGIN_COOKIE}=; Path=/; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Secure; HttpOnly; SameSite=Lax`,
  );
}

export function setLoginCookie(
  headers: Headers,
  binding: string,
  locale: Locale,
  expiresAt: unknown,
) {
  const age = maxAge(expiresAt, 300);
  if (age === 0 || !isBase64URL32(binding)) return false;
  const value = Buffer.from(
    JSON.stringify({ binding, locale }),
    "utf8",
  ).toString("base64url");
  if (value.length > 3072) return false;
  headers.append("Set-Cookie", cookie(LOGIN_COOKIE, value, age, true));
  return true;
}

export function loginBinding(
  request: Request,
): { binding: string; locale: Locale } | null {
  const value = exactCookie(request, LOGIN_COOKIE);
  if (!value || value.length > 3072) return null;
  try {
    const parsed: unknown = JSON.parse(
      Buffer.from(value, "base64url").toString("utf8"),
    );
    if (!parsed || typeof parsed !== "object") return null;
    const item = parsed as Record<string, unknown>;
    if (typeof item.binding !== "string" || !isBase64URL32(item.binding))
      return null;
    return {
      binding: item.binding,
      locale: isLocale(String(item.locale)) ? (item.locale as Locale) : "zh-CN",
    };
  } catch {
    return null;
  }
}

export function setSessionCookies(
  headers: Headers,
  token: unknown,
  expiresAt: unknown,
) {
  const age = maxAge(expiresAt, 86400);
  if (typeof token !== "string" || !isBase64URL32(token) || age === 0)
    return false;
  const csrf = Buffer.from(crypto.getRandomValues(new Uint8Array(32))).toString(
    "base64url",
  );
  headers.append("Set-Cookie", cookie(SESSION_COOKIE, token, age, true));
  headers.append("Set-Cookie", cookie(CSRF_COOKIE, csrf, age, false));
  return true;
}

export function redirect(location: string, status = 303) {
  return new Response(null, {
    status,
    headers: { Location: location, "Cache-Control": "no-store" },
  });
}

export function authFailure(locale: Locale = "zh-CN") {
  return redirect(`/${locale}/?auth=failed`);
}

export function validAuthorizationURL(value: unknown) {
  if (typeof value !== "string" || value.length > 4096) return null;
  try {
    const url = new URL(value);
    const loopback =
      url.protocol === "http:" &&
      (url.hostname === "127.0.0.1" || url.hostname === "localhost");
    if (
      (url.protocol !== "https:" && !(authConfig?.allowLoopback && loopback)) ||
      url.username ||
      url.password ||
      url.hash
    )
      return null;
    return url.toString();
  } catch {
    return null;
  }
}

export async function authenticatedStores(token: string) {
  const response = await merchantBackend("/v1/admin/stores", token, {
    method: "GET",
  });
  if (!response.ok) return { response, stores: null as Store[] | null };
  const body = await safeJSON<{ items?: unknown }>(response);
  if (!Array.isArray(body?.items))
    return {
      response: localError(503, "retry_later"),
      stores: null as Store[] | null,
    };
  const stores = body.items.filter(
    (item): item is Store =>
      !!item &&
      typeof item === "object" &&
      /^[0-9a-f-]{36}$/.test(String((item as Store).id)) &&
      typeof (item as Store).name === "string" &&
      /^[A-Z]{3}$/.test(String((item as Store).currency)),
  );
  if (stores.length !== body.items.length)
    return {
      response: localError(503, "retry_later"),
      stores: null as Store[] | null,
    };
  return { response, stores };
}
