// BFF for the merchant tools (contracts/storefront-v2.md section G): the browser calls /api/stores/{store}/tools/<resource> and this file
// forwards to Go /v1/admin/stores/{store}/<resource> (internal/httpapi/merchanttools.go) with the merchant bearer from the HttpOnly session
// cookie. The ONLY resources are the six of lib/merchant-tools-model.ts toolsRoute: GET dashboard, GET products/export.csv, GET
// orders/manual/options, POST products/import/{preview,commit} (text/csv, <= 2 MiB, no key), POST orders/manual (JSON, Idempotency-Key).
// It decides nothing: Go and the SQL definers own every rule and permission. It never forwards a tenant, a query string or a header the
// browser chose, never retries (a repeated import or order is the caller's explicit, keyed choice), and rebuilds every error locally from an
// allow-listed code. It is a sibling of the generic stores/[store]/[...resource] BFF because these routes need their own body types and
// longer upstream budgets (import 75 s, manual order 16 s) than that file's 6 s.
import {
  authConfig,
  authenticatedStores,
  clearAuthCookies,
  localError,
  requireCSRF,
  requireOrigin,
  safeError,
  sessionToken,
} from "@/lib/auth";
import { fixtureSession } from "@/lib/backend";
import { MAX_CSV_BYTES, parseImportResult, toolsRoute, validManualBody, validRegenerateBody, type ToolsRoute } from "@/lib/merchant-tools-model";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const keyPattern = /^[A-Za-z0-9_.:-]{8,128}$/;
const MAX_JSON = 64 * 1024;
const nostore = { "Cache-Control": "private, no-store" };
type Context = { params: Promise<{ store: string; resource: string[] }> };

const budgets: Record<ToolsRoute, number> = {
  dashboard: 8000, export: 20000, "manual-options": 10000, "import-preview": 75000, "import-commit": 75000, "manual-place": 16000,
  "manual-regenerate": 16000,
};

async function readCapped(request: Request, limit: number): Promise<Uint8Array | null> {
  const declared = Number(request.headers.get("content-length") ?? "0");
  if (!Number.isFinite(declared) || declared > limit) return null;
  const reader = request.body?.getReader();
  if (!reader) return new Uint8Array(0);
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const part = await reader.read();
    if (part.done) break;
    size += part.value.byteLength;
    if (size > limit) {
      await reader.cancel();
      return null;
    }
    chunks.push(part.value);
  }
  const data = new Uint8Array(size);
  let at = 0;
  for (const chunk of chunks) {
    data.set(chunk, at);
    at += chunk.length;
  }
  return data;
}

// One upstream call with a per-route budget: the generic callBackend has a fixed 6 s, too short for an import.
async function upstream(path: string, init: RequestInit, token: string | undefined, store: string, timeoutMs: number) {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json, text/csv");
  headers.delete("Cookie");
  headers.delete("X-Commerce-BFF-Key");
  let origin: string;
  if (authConfig) {
    if (!token) return Response.json({ code: "unauthorized" }, { status: 401 });
    headers.set("Authorization", `Bearer ${token}`);
    origin = `${authConfig.apiOrigin}/v1/admin/stores/${store}/${path}`;
  } else {
    const session = fixtureSession();
    if (!session) return Response.json({ code: "unauthorized" }, { status: 401 });
    headers.set("Authorization", `Bearer ${session.token}`);
    origin = `${session.origin}/v1/admin/stores/${session.storeID}/${path}`;
  }
  try {
    return await fetch(origin, { ...init, headers, cache: "no-store", redirect: "error", signal: AbortSignal.timeout(timeoutMs) });
  } catch {
    return localError(503, "retry_later");
  }
}

async function route(request: Request, context: Context) {
  const { store, resource } = await context.params;
  const path = resource.join("/");
  const kind = toolsRoute(request.method, path);
  if (!uuid.test(store) || !kind) return localError(404, "not_found");
  // Exact resources: no query at all (not even a bare "?").
  if (request.url.includes("?")) return localError(422, "invalid_request");
  const isGet = request.method === "GET";
  if (isGet && (request.body !== null || request.headers.has("transfer-encoding") || request.headers.has("idempotency-key") ||
    (request.headers.has("content-length") && request.headers.get("content-length") !== "0"))) return localError(422, "invalid_request");

  let token: string | undefined;
  if (authConfig) {
    token = sessionToken(request) ?? undefined;
    if (!token) {
      const denied = localError(401, "unauthorized");
      clearAuthCookies(denied.headers);
      return denied;
    }
    if (!isGet && (!requireOrigin(request) || !requireCSRF(request))) return localError(403, "forbidden");
    const listed = await authenticatedStores(token);
    if (!listed.stores) {
      const denied = await safeError(listed.response);
      if (denied.status === 401) clearAuthCookies(denied.headers);
      return denied;
    }
    if (!listed.stores.some((item) => item.id === store)) return localError(404, "not_found");
  } else {
    const session = fixtureSession();
    if (!session) return localError(401, "unauthorized");
    if (store !== session.storeID) return localError(404, "not_found");
    const host = request.headers.get("host");
    if (host !== "127.0.0.1:3100" && host !== "localhost:3100") return localError(403, "forbidden");
    if (!isGet && request.headers.get("origin") !== `http://${host}`) return localError(403, "forbidden");
  }

  const init: RequestInit = { method: request.method };
  if (!isGet) {
    const type = request.headers.get("content-type")?.split(";", 1)[0]?.trim().toLowerCase();
    const csv = kind === "import-preview" || kind === "import-commit";
    if (type !== (csv ? "text/csv" : "application/json")) return localError(415, csv ? "invalid_request" : "json_required");
    const key = request.headers.get("idempotency-key") ?? "";
    // The import is idempotent by file hash and takes no key; an order is a keyed command.
    if (csv ? key !== "" : !keyPattern.test(key)) return localError(422, "invalid_request");
    const data = await readCapped(request, csv ? MAX_CSV_BYTES : MAX_JSON);
    if (!data || data.byteLength === 0) return localError(csv ? 413 : 400, csv ? "invalid_request" : "invalid_json");
    if (csv) {
      init.body = data as unknown as BodyInit;
      init.headers = { "Content-Type": "text/csv; charset=utf-8" };
    } else {
      let text: string;
      let parsed: unknown;
      try {
        text = new TextDecoder("utf-8", { fatal: true }).decode(data);
        parsed = JSON.parse(text);
      } catch {
        return localError(400, "invalid_json");
      }
      if (kind === "manual-regenerate" ? !validRegenerateBody(parsed) : !validManualBody(parsed)) return localError(422, "invalid_request");
      init.body = text;
      init.headers = { "Content-Type": "application/json", "Idempotency-Key": key };
    }
  }

  const response = await upstream(path, init, token, store, budgets[kind]);
  if (authConfig && response.status === 401) {
    const denied = await safeError(response);
    clearAuthCookies(denied.headers);
    return denied;
  }
  const requestID = response.headers.get("x-request-id") ?? "";
  if (kind === "export") {
    // The export streams straight through, only as the exact attachment shape; never buffered or stored here.
    const disposition = response.headers.get("content-disposition") ?? "";
    if (!response.ok) return safeError(response);
    if (response.status !== 200 || !response.body || response.headers.get("content-type")?.toLowerCase() !== "text/csv; charset=utf-8" ||
      !/^attachment; filename="products-[0-9]{4}-[0-9]{2}-[0-9]{2}\.csv"$/.test(disposition)) return localError(503, "retry_later");
    return new Response(response.body, {
      status: 200,
      headers: { "Content-Type": "text/csv; charset=utf-8", "Content-Disposition": disposition, ...nostore, "X-Content-Type-Options": "nosniff", "X-Request-ID": requestID },
    });
  }
  if (kind === "import-preview" || kind === "import-commit") {
    // 200 (preview or commit) and 422 (commit refused) both carry the ImportResult; anything else is a coded error.
    const text = await response.text();
    if (response.status === 200 || (kind === "import-commit" && response.status === 422)) {
      try {
        const result = parseImportResult(JSON.parse(text));
        return Response.json(result, { status: response.status, headers: { ...nostore, "X-Request-ID": requestID } });
      } catch {
        // A 422 that is not an ImportResult is an ordinary coded refusal below; a 200 that is not one is a broken upstream.
        if (response.status === 200) return localError(503, "retry_later");
      }
    }
    return safeError(new Response(text, { status: response.status, headers: response.headers }));
  }
  if (!response.ok) return safeError(response);
  return new Response(response.body, {
    status: response.status,
    headers: { "Content-Type": "application/json", ...nostore, "X-Request-ID": requestID },
  });
}

export const GET = route;
export const POST = route;
const unsupported = () => localError(405, "method_not_allowed", "GET, POST");
export const PUT = unsupported;
export const PATCH = unsupported;
export const DELETE = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
