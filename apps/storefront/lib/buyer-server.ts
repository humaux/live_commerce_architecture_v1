import { createHmac, randomBytes, timingSafeEqual } from "node:crypto";
import { isIP } from "node:net";

const COOKIE = "__Host-commerce_buyer";
const MAX_JSON = 64 * 1024;
const MAX_UPSTREAM = 1024 * 1024;
const INBOUND_DEADLINE = 12_000;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const KEY = /^[A-Za-z0-9_.:-]{8,128}$/;
const TOKEN = /^[A-Za-z0-9_-]{43}$/;

type Config = { api: string; bff: string; signing: Buffer; ttl: number };
type Envelope = {
  v: 1;
  token: string;
  iat: number;
  exp: number;
  origin: string;
};
type Session = {
  state: "absent" | "expired" | "inactive" | "active";
  context: string | null;
  expires_at: string | null;
};
type Route = {
  method: string;
  privatePath: string;
  body?: "empty" | "cart" | "quote" | "destination" | "checkout";
  query?: "catalog" | "options" | "orders";
  session?: string;
};

const messages: Record<string, string> = {
  unauthorized: "Session unavailable.",
  forbidden: "Request not permitted.",
  not_found: "Resource not found.",
  method_not_allowed: "Method not allowed.",
  invalid_request: "Request validation failed.",
  invalid_json: "Malformed JSON body.",
  json_required: "JSON content type required.",
  context_changed: "Buyer session changed. Reload before continuing.",
  conflict: "Request conflicts with current state.",
  insufficient_inventory: "Insufficient available inventory.",
  rate_limited: "Too many requests.",
  unavailable: "Temporarily unavailable.",
};

function headers(extra?: HeadersInit) {
  const result = new Headers(extra);
  result.set("Cache-Control", "no-store");
  result.set("X-Content-Type-Options", "nosniff");
  return result;
}

function failure(status: number, code: string, nonretryable = false): Response {
  const request_id = randomBytes(16).toString("hex");
  const safeCode = Object.hasOwn(messages, code) ? code : "unavailable";
  const h = headers({ "X-Request-ID": request_id });
  return Response.json(
    {
      code: safeCode,
      message: messages[safeCode],
      request_id,
      retryable: !nonretryable && (status === 503 || status === 429),
      details: {},
    },
    { status, headers: h },
  );
}

function success(value: unknown, extra?: HeadersInit): Response {
  return Response.json(value, { headers: headers(extra) });
}

function canonical32(value: string): boolean {
  return (
    TOKEN.test(value) &&
    Buffer.from(value, "base64url").length === 32 &&
    Buffer.from(value, "base64url").toString("base64url") === value
  );
}

function config(): Config | null {
  const enabled = process.env.COMMERCE_BUYER_WEB_ENABLED ?? "";
  if (enabled === "" || enabled === "0") return null;
  if (enabled !== "1") throw new Error("invalid enabled flag");
  const raw = process.env.COMMERCE_BUYER_API_ORIGIN ?? "";
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error("invalid API origin");
  }
  const loopback =
    url.protocol === "http:" &&
    (url.hostname === "127.0.0.1" || url.hostname === "localhost") &&
    !!url.port;
  if (
    raw !== url.origin ||
    (url.protocol !== "https:" && !loopback) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  )
    throw new Error("invalid API origin");
  const bff = process.env.COMMERCE_BUYER_BFF_KEY ?? "";
  const cookie = process.env.COMMERCE_BUYER_COOKIE_KEY ?? "";
  if (!canonical32(bff) || !canonical32(cookie) || bff === cookie)
    throw new Error("invalid keys");
  const rawTTL = process.env.COMMERCE_BUYER_SESSION_TTL ?? "";
  if (!/^[1-9][0-9]{1,6}$/.test(rawTTL)) throw new Error("invalid TTL");
  const ttl = Number(rawTTL);
  if (ttl < 60 || ttl > 2592000) throw new Error("invalid TTL");
  return { api: raw, bff, signing: Buffer.from(cookie, "base64url"), ttl };
}

function candidateOrigin(request: Request): string | null {
  const host = request.headers.get("host");
  if (
    !host ||
    host.length > 253 ||
    host !== host.toLowerCase() ||
    host.includes(":") ||
    host.includes(",") ||
    host.includes("%") ||
    host.includes("@") ||
    host.endsWith(".") ||
    isIP(host)
  )
    return null;
  const labels = host.split(".");
  if (
    labels.length < 2 ||
    labels.some(
      (label) =>
        label.length > 63 || !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(label),
    )
  )
    return null;
  return `https://${host}`;
}

function mac(config: Config, purpose: string, text: string): Buffer {
  return createHmac("sha256", config.signing)
    .update(purpose)
    .update("\0")
    .update(text)
    .digest();
}

function envelopeValue(config: Config, envelope: Envelope): string {
  const payload = Buffer.from(JSON.stringify(envelope)).toString("base64url");
  return `${payload}.${mac(config, "buyer-cookie-v1", payload).toString("base64url")}`;
}

function context(config: Config, origin: string, cookie: string): string {
  return mac(config, "buyer-context-v1", `${origin}\0${cookie}`).toString(
    "base64url",
  );
}

function cookieValue(request: Request): {
  kind: "absent" | "invalid" | "present";
  value?: string;
} {
  const source = request.headers.get("cookie") ?? "";
  if (source.length > 8192) return { kind: "invalid" };
  const values: string[] = [];
  for (const part of source.split(";")) {
    const at = part.indexOf("=");
    if (at >= 0 && part.slice(0, at).trim() === COOKIE)
      values.push(part.slice(at + 1).trim());
  }
  return values.length === 0
    ? { kind: "absent" }
    : values.length === 1 && values[0].length <= 1024
      ? { kind: "present", value: values[0] }
      : { kind: "invalid" };
}

function parseEnvelope(
  config: Config,
  origin: string,
  value: string,
): Envelope | null {
  const pieces = value.split(".");
  if (
    pieces.length !== 2 ||
    !/^[A-Za-z0-9_-]+$/.test(pieces[0]) ||
    !canonical32(pieces[1])
  )
    return null;
  const expected = mac(config, "buyer-cookie-v1", pieces[0]);
  if (!timingSafeEqual(expected, Buffer.from(pieces[1], "base64url")))
    return null;
  try {
    const bytes = Buffer.from(pieces[0], "base64url");
    if (bytes.toString("base64url") !== pieces[0]) return null;
    const decoded: unknown = JSON.parse(
      new TextDecoder("utf-8", { fatal: true }).decode(bytes),
    );
    if (!decoded || typeof decoded !== "object" || Array.isArray(decoded))
      return null;
    const e = decoded as Envelope;
    if (
      e.v !== 1 ||
      !canonical32(e.token) ||
      !Number.isSafeInteger(e.iat) ||
      !Number.isSafeInteger(e.exp) ||
      e.origin !== origin ||
      e.exp <= e.iat ||
      e.exp - e.iat > config.ttl ||
      e.iat > Math.floor(Date.now() / 1000) + 60 ||
      JSON.stringify(e) !== bytes.toString("utf8") ||
      Object.keys(e).join(",") !== "v,token,iat,exp,origin"
    )
      return null;
    return e;
  } catch {
    return null;
  }
}

function identify(
  request: Request,
  config: Config,
  origin: string,
): {
  envelope: Envelope | null;
  cookie: string | null;
  context: string | null;
  invalid: boolean;
} {
  const found = cookieValue(request);
  if (found.kind === "absent")
    return { envelope: null, cookie: null, context: null, invalid: false };
  if (found.kind === "invalid")
    return { envelope: null, cookie: null, context: null, invalid: true };
  const e = parseEnvelope(config, origin, found.value!);
  return e
    ? {
        envelope: e,
        cookie: found.value!,
        context: context(config, origin, found.value!),
        invalid: false,
      }
    : { envelope: null, cookie: null, context: null, invalid: true };
}

function route(
  path: string,
  method: string,
): { route?: Route; known: boolean; invalidID?: boolean } {
  const prefix = "/api/buyer/";
  if (!path.startsWith(prefix) || path.includes("%") || path.includes("//"))
    return { known: false };
  const suffix = path.slice(prefix.length);
  const session = /^session(?:\/(prepare|activate|reset|logout))?$/.exec(
    suffix,
  );
  if (session) {
    const allowed = session[1] ? "POST" : "GET";
    return {
      known: true,
      route:
        method === allowed
          ? {
              method,
              privatePath: "session",
              body: session[1] ? "empty" : undefined,
              session: session[1] ?? "status",
            }
          : undefined,
    };
  }
  const exact: Record<string, Record<string, Omit<Route, "method">>> = {
    catalog: { GET: { privatePath: "catalog", query: "catalog" } },
    "checkout-options": {
      GET: { privatePath: "checkout-options", query: "options" },
    },
    cart: {
      GET: { privatePath: "cart" },
      PUT: { privatePath: "cart", body: "cart" },
    },
    quotes: { POST: { privatePath: "quotes", body: "quote" } },
    destination: {
      GET: { privatePath: "destination" },
      PUT: { privatePath: "destination", body: "destination" },
    },
    checkout: { POST: { privatePath: "checkout", body: "checkout" } },
    orders: { GET: { privatePath: "orders", query: "orders" } },
  };
  if (Object.hasOwn(exact, suffix)) {
    const selected = exact[suffix][method];
    return {
      known: true,
      route: selected ? { ...selected, method } : undefined,
    };
  }
  const item = /^(quotes|destinations|orders)\/([^/]+)$/.exec(suffix);
  if (!item) return { known: false };
  if (!UUID.test(item[2])) return { known: true, invalidID: true };
  return {
    known: true,
    route: method === "GET" ? { method, privatePath: suffix } : undefined,
  };
}

function validQuery(rawURL: string, kind?: Route["query"]): string | null {
  const mark = rawURL.indexOf("?");
  if (mark < 0) return "";
  if (!kind) return null;
  const raw = rawURL.slice(mark + 1).split("#", 1)[0];
  if (
    !raw ||
    raw.length > 2048 ||
    raw.includes(";") ||
    raw.includes("+") ||
    raw.split("&").some((part) => !part || !part.includes("="))
  )
    return null;
  const allowed =
    kind === "catalog"
      ? ["product_id", "limit", "cursor"]
      : kind === "orders"
        ? ["limit", "cursor"]
        : ["market_id", "country", "limit", "cursor"];
  const seen = new Set<string>();
  for (const part of raw.split("&")) {
    const at = part.indexOf("=");
    let name: string, value: string;
    try {
      name = decodeURIComponent(part.slice(0, at));
      value = decodeURIComponent(part.slice(at + 1));
    } catch {
      return null;
    }
    if (!allowed.includes(name) || seen.has(name) || !value) return null;
    seen.add(name);
    if ((name === "product_id" || name === "market_id") && !UUID.test(value))
      return null;
    if (name === "country" && !/^[A-Z]{2}$/.test(value)) return null;
    if (
      name === "limit" &&
      (!/^[0-9]{1,3}$/.test(value) || +value < 1 || +value > 100)
    )
      return null;
    if (
      name === "cursor" &&
      (value.length > 1024 || !/^[A-Za-z0-9_-]+$/.test(value))
    )
      return null;
  }
  return `?${raw}`;
}

async function readBounded(
  stream: ReadableStream<Uint8Array> | null,
  limit: number,
  timeoutMs = 0,
): Promise<Uint8Array | null> {
  if (!stream) return new Uint8Array();
  const reader = stream.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let timedOut = false;
  const deadline =
    timeoutMs > 0
      ? new Promise<never>((_, reject) => {
          timer = setTimeout(() => {
            timedOut = true;
            reject(new Error("body deadline"));
            void reader.cancel().catch(() => {});
          }, timeoutMs);
        })
      : null;
  try {
    while (true) {
      const part = deadline
        ? await Promise.race([reader.read(), deadline])
        : await reader.read();
      if (timedOut) throw new Error("body deadline");
      if (part.done) break;
      length += part.value.length;
      if (length > limit) {
        void reader.cancel().catch(() => {});
        return null;
      }
      chunks.push(part.value);
    }
  } finally {
    if (timer) clearTimeout(timer);
  }
  const result = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    result.set(chunk, offset);
    offset += chunk.length;
  }
  return result;
}

// JSON.parse alone drops duplicate keys. This small scanner rejects them and
// null at any depth before a commerce command reaches the private transport.
function strictJSON(text: string): unknown {
  let at = 0;
  const space = () => {
    while (/\s/.test(text[at] ?? "") && at < text.length) at++;
  };
  const string = (): string => {
    const start = at++;
    while (at < text.length) {
      if (text[at] === "\\") {
        at += 2;
        continue;
      }
      if (text[at++] === '"')
        return JSON.parse(text.slice(start, at)) as string;
    }
    throw new Error("string");
  };
  const value = (): void => {
    space();
    if (text[at] === "{") {
      at++;
      space();
      const keys = new Set<string>();
      if (text[at] === "}") {
        at++;
        return;
      }
      while (true) {
        if (text[at] !== '"') throw new Error("key");
        const key = string();
        if (keys.has(key)) throw new Error("duplicate");
        keys.add(key);
        space();
        if (text[at++] !== ":") throw new Error("colon");
        value();
        space();
        if (text[at] === "}") {
          at++;
          return;
        }
        if (text[at++] !== ",") throw new Error("comma");
        space();
      }
    }
    if (text[at] === "[") {
      at++;
      space();
      if (text[at] === "]") {
        at++;
        return;
      }
      while (true) {
        value();
        space();
        if (text[at] === "]") {
          at++;
          return;
        }
        if (text[at++] !== ",") throw new Error("comma");
      }
    }
    if (text[at] === '"') {
      string();
      return;
    }
    for (const literal of ["true", "false"])
      if (text.startsWith(literal, at)) {
        at += literal.length;
        return;
      }
    if (text.startsWith("null", at)) throw new Error("null");
    const number = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/.exec(
      text.slice(at),
    );
    if (!number || !Number.isFinite(Number(number[0])))
      throw new Error("value");
    at += number[0].length;
  };
  value();
  space();
  if (at !== text.length) throw new Error("trailing");
  return JSON.parse(text) as unknown;
}

type Shape = { [key: string]: "string" | "integer" | Shape | [Shape] };
const shapes: Record<Exclude<Route["body"], undefined>, Shape> = {
  empty: {},
  cart: {
    expected_version: "integer",
    items: [{ sku_id: "string", quantity: "integer" }],
  },
  quote: {
    cart_version: "integer",
    market_id: "string",
    country: "string",
    method: "string",
  },
  destination: {
    expected_version: "integer",
    cart_version: "integer",
    kind: "string",
    country: "string",
    recipient_name: "string",
    phone: "string",
    home_address: {
      region: "string",
      city: "string",
      postal_code: "string",
      line1: "string",
      line2: "string",
    },
    pickup_id: "string",
  },
  checkout: {
    quote_id: "string",
    destination_id: "string",
    cart_version: "integer",
    service_version: "integer",
    allocation_version: "integer",
  },
};

function matchesShape(value: unknown, shape: Shape): boolean {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  return Object.entries(value).every(([key, field]) => {
    if (!Object.hasOwn(shape, key)) return false;
    const expected = shape[key];
    if (!expected) return false;
    if (expected === "string") return typeof field === "string";
    if (expected === "integer") return Number.isSafeInteger(field);
    if (Array.isArray(expected))
      return (
        Array.isArray(field) &&
        field.every((item) => matchesShape(item, expected[0]))
      );
    return matchesShape(field, expected);
  });
}

async function bodyJSON(
  request: Request,
  shape: Shape,
  nonretryable = false,
): Promise<{ body?: string; error?: Response }> {
  const mime = request.headers.get("content-type") ?? "";
  if (!/^application\/json(?:\s*;\s*charset=(?:utf-8|"utf-8"))?$/i.test(mime))
    return { error: failure(415, "json_required", nonretryable) };
  const length = request.headers.get("content-length");
  if (length !== null && (!/^[0-9]+$/.test(length) || +length > MAX_JSON))
    return { error: failure(422, "invalid_request", nonretryable) };
  let bytes: Uint8Array | null;
  try {
    bytes = await readBounded(request.body, MAX_JSON, INBOUND_DEADLINE);
  } catch {
    return { error: failure(503, "unavailable", nonretryable) };
  }
  if (!bytes) return { error: failure(422, "invalid_request", nonretryable) };
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return { error: failure(400, "invalid_json", nonretryable) };
  }
  try {
    if (!matchesShape(strictJSON(text), shape))
      return { error: failure(400, "invalid_json", nonretryable) };
  } catch {
    return { error: failure(400, "invalid_json", nonretryable) };
  }
  return { body: text };
}

async function noBody(
  request: Request,
): Promise<"empty" | "invalid" | "unavailable"> {
  if (
    request.headers.get("content-length") &&
    request.headers.get("content-length") !== "0"
  )
    return "invalid";
  try {
    const bytes = await readBounded(request.body, 0, INBOUND_DEADLINE);
    return bytes !== null && bytes.length === 0 ? "empty" : "invalid";
  } catch {
    return "unavailable";
  }
}

async function upstream(
  request: Request,
  config: Config,
  origin: string,
  token: string,
  path: string,
  method: string,
  body?: string,
  key?: string,
): Promise<Response> {
  const outbound = new Headers({
    Accept: "application/json",
    "X-Commerce-Buyer-BFF-Key": config.bff,
    "X-Commerce-Storefront-Origin": origin,
    Authorization: `Bearer ${token}`,
  });
  if (body !== undefined) outbound.set("Content-Type", "application/json");
  if (key) outbound.set("Idempotency-Key", key);
  try {
    return await fetch(`${config.api}/v1/buyer/${path}`, {
      method,
      headers: outbound,
      body,
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(12000)]),
    });
  } catch {
    return failure(503, "unavailable");
  }
}

async function upstreamJSON(response: Response): Promise<unknown | null> {
  if (
    response.headers
      .get("content-type")
      ?.split(";", 1)[0]
      .trim()
      .toLowerCase() !== "application/json"
  )
    return null;
  const advertised = response.headers.get("content-length");
  if (
    advertised !== null &&
    (!/^[0-9]+$/.test(advertised) || +advertised > MAX_UPSTREAM)
  )
    return null;
  try {
    const bytes = await readBounded(response.body, MAX_UPSTREAM);
    if (!bytes) return null;
    return JSON.parse(
      new TextDecoder("utf-8", { fatal: true }).decode(bytes),
    ) as unknown;
  } catch {
    return null;
  }
}

async function upstreamError(
  response: Response,
  nonretryable = false,
): Promise<Response> {
  const body = await upstreamJSON(response);
  const code =
    body && typeof body === "object" && !Array.isArray(body)
      ? (body as { code?: unknown }).code
      : undefined;
  if (
    typeof code !== "string" ||
    !Object.hasOwn(messages, code) ||
    ![400, 401, 403, 404, 409, 415, 422, 429, 503].includes(response.status)
  )
    return failure(503, "unavailable", nonretryable);
  return failure(response.status, code, nonretryable);
}

function session(
  state: Session["state"],
  contextValue: string | null,
  expires: number | null,
): Session {
  return {
    state,
    context: contextValue,
    expires_at:
      expires === null ? null : new Date(expires * 1000).toISOString(),
  };
}

function prepared(config: Config, origin: string): Response {
  const iat = Math.floor(Date.now() / 1000);
  const e: Envelope = {
    v: 1,
    token: randomBytes(32).toString("base64url"),
    iat,
    exp: iat + config.ttl,
    origin,
  };
  const value = envelopeValue(config, e);
  const setCookie = `${COOKIE}=${value}; Path=/; Max-Age=${config.ttl}; Secure; HttpOnly; SameSite=Lax`;
  return success(session("inactive", context(config, origin, value), e.exp), {
    "Set-Cookie": setCookie,
  });
}

function forbiddenHeaders(request: Request): boolean {
  return [
    "authorization",
    "x-tenant-id",
    "x-store-id",
    "x-commerce-buyer-bff-key",
    "x-commerce-storefront-origin",
    "x-commerce-bff-key",
  ].some((name) => request.headers.has(name));
}

export async function handleBuyerRequest(request: Request): Promise<Response> {
  const pathname = new URL(request.url).pathname;
  const changingCookie =
    request.method === "POST" &&
    /^\/api\/buyer\/session\/(?:prepare|reset)$/.test(pathname);
  const fail = (status: number, code: string) =>
    failure(status, code, changingCookie);
  let cfg: Config | null;
  try {
    cfg = config();
  } catch {
    return fail(503, "unavailable");
  }
  if (!cfg) return fail(404, "not_found");
  const origin = candidateOrigin(request);
  if (!origin || forbiddenHeaders(request)) return fail(403, "forbidden");
  const url = new URL(request.url);
  const selected = route(url.pathname, request.method);
  if (!selected.known) return fail(404, "not_found");
  if (selected.invalidID) return fail(422, "invalid_request");
  if (!selected.route) return fail(405, "method_not_allowed");
  const target = selected.route;
  const query = validQuery(request.url, target.query);
  if (query === null || url.hash) return fail(422, "invalid_request");
  const isMutation = request.method !== "GET";
  if (isMutation && request.headers.get("origin") !== origin)
    return fail(403, "forbidden");
  const found = identify(request, cfg, origin);
  if (found.invalid) return fail(401, "unauthorized");
  if (
    target.session === "prepare" &&
    !found.envelope &&
    request.headers.has("x-buyer-context")
  )
    return fail(409, "context_changed");
  const isSession = !!target.session;
  const key = request.headers.get("idempotency-key");
  if (isSession || request.method === "GET") {
    if (key !== null) return fail(422, "invalid_request");
  } else if (!key || !KEY.test(key)) return fail(422, "invalid_request");
  if (
    isMutation &&
    !(target.session === "prepare" && !found.envelope) &&
    request.headers.get("x-buyer-context") !== found.context
  )
    return fail(409, "context_changed");
  if (
    !isSession &&
    (!found.envelope || found.envelope.exp <= Math.floor(Date.now() / 1000))
  )
    return fail(401, "unauthorized");
  if (
    isSession &&
    target.session !== "prepare" &&
    target.session !== "status" &&
    !found.envelope
  )
    return fail(401, "unauthorized");
  if (isSession && target.session === "prepare" && found.envelope)
    return fail(409, "conflict");
  if (
    isSession &&
    target.session === "activate" &&
    found.envelope!.exp <= Math.floor(Date.now() / 1000)
  )
    return fail(401, "unauthorized");
  if (!isSession && request.headers.get("x-buyer-context") !== found.context)
    return fail(409, "context_changed");
  let body: string | undefined;
  if (target.body) {
    const result = await bodyJSON(request, shapes[target.body], changingCookie);
    if (result.error) return result.error;
    body = result.body;
  } else {
    const result = await noBody(request);
    if (result !== "empty")
      return fail(
        result === "unavailable" ? 503 : 422,
        result === "unavailable" ? "unavailable" : "invalid_request",
      );
  }
  if (request.signal.aborted) return fail(503, "unavailable");
  if (target.session === "prepare") return prepared(cfg, origin);
  if (target.session === "reset" || target.session === "logout") {
    const response = await upstream(
      request,
      cfg,
      origin,
      found.envelope!.token,
      "session/retire",
      "POST",
      "{}",
    );
    if (request.signal.aborted) return fail(503, "unavailable");
    if (response.status !== 204)
      return upstreamError(response, target.session === "reset");
    return target.session === "reset"
      ? prepared(cfg, origin)
      : new Response(null, { status: 204, headers: headers() });
  }
  if (target.session === "status") {
    if (!found.envelope) return success(session("absent", null, null));
    if (found.envelope.exp <= Math.floor(Date.now() / 1000))
      return success(session("expired", found.context, found.envelope.exp));
    const response = await upstream(
      request,
      cfg,
      origin,
      found.envelope.token,
      "session",
      "GET",
    );
    if (request.signal.aborted) return fail(503, "unavailable");
    if (response.status === 401)
      return success(session("inactive", found.context, found.envelope.exp));
    if (!response.ok) return upstreamError(response);
    const result = await upstreamJSON(response);
    if (
      !result ||
      typeof result !== "object" ||
      (result as { authenticated?: unknown }).authenticated !== true
    )
      return fail(503, "unavailable");
    return success(session("active", found.context, found.envelope.exp));
  }
  if (target.session === "activate") {
    const response = await upstream(
      request,
      cfg,
      origin,
      found.envelope!.token,
      "session/bootstrap",
      "POST",
      "{}",
    );
    if (request.signal.aborted) return fail(503, "unavailable");
    if (!response.ok) return upstreamError(response);
    const result = await upstreamJSON(response);
    const expiry =
      result && typeof result === "object"
        ? (result as { authenticated?: unknown; expires_at?: unknown })
        : null;
    const effective =
      typeof expiry?.expires_at === "string"
        ? Date.parse(expiry.expires_at)
        : NaN;
    if (
      expiry?.authenticated !== true ||
      !Number.isFinite(effective) ||
      effective <= Date.now()
    )
      return fail(503, "unavailable");
    return success(
      session(
        "active",
        found.context,
        Math.min(found.envelope!.exp, Math.floor(effective / 1000)),
      ),
    );
  }
  const response = await upstream(
    request,
    cfg,
    origin,
    found.envelope!.token,
    target.privatePath + query,
    request.method,
    body,
    key ?? undefined,
  );
  if (request.signal.aborted) return fail(503, "unavailable");
  if (!response.ok) return upstreamError(response);
  const data = await upstreamJSON(response);
  if (request.signal.aborted || data === null || typeof data !== "object")
    return fail(503, "unavailable");
  return success(data);
}
