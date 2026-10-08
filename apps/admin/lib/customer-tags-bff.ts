// Purpose: server authority adapter for /api/stores/{store}/customers tag/note leaf routes.
// Depends on: auth.ts session/store/Origin/CSRF guards, backend.ts -> Go customer_tags.go, proxy grammar.
// Used by: the five exact W6-01B route leaves; no customer detail/privacy route is shadowed.
import "server-only";
import { authConfig, authenticatedStores, clearAuthCookies, localError, readBody, requireCSRF, requireOrigin, safeError, sessionToken } from "./auth";
import { callBackend, fixtureSession } from "./backend";
import { proxyCustomerTags } from "./customer-tags-proxy";

/** Authorized W6-01B leaf; production sessions require both Origin and CSRF on every write. */
export async function customerTagsBFF(request: Request, store: string, resource: string): Promise<Response> {
  return proxyCustomerTags(request, store, resource, {
    error: (status, code) => {
      const response = localError(status, code);
      if (status === 401) clearAuthCookies(response.headers);
      return response;
    },
    body: async (r) => {
      if (r.method !== "DELETE") return r.body === null ? "" : readBody(r, "application/json", 16 << 10);
      // Next represents a bodyless DELETE as an empty stream. No representation means no MIME requirement;
      // still reject actual bytes, regardless of Content-Length, before either deletion can reach Go.
      const reader = r.body?.getReader();
      if (!reader) return "";
      try {
        while (true) {
          const part = await reader.read();
          if (part.done) return "";
          if (part.value.byteLength) { await reader.cancel(); throw new Error("body"); }
        }
      } finally { reader.releaseLock(); }
    },
    forward: callBackend,
    authorize: async (r, selected) => {
      if (authConfig) {
        const token = sessionToken(r);
        if (!token) { const denied = localError(401, "unauthorized"); clearAuthCookies(denied.headers); return denied; }
        if (r.method !== "GET" && (!requireOrigin(r) || !requireCSRF(r))) return localError(403, "forbidden");
        const listed = await authenticatedStores(token);
        if (!listed.stores) {
          const denied = await safeError(listed.response);
          if (denied.status === 401) clearAuthCookies(denied.headers);
          return denied;
        }
        return listed.stores.some((s) => s.id === selected) ? token : localError(404, "not_found");
      }
      // Existing explicit loopback-only development fixture; never enabled in a production Next build.
      const fixture = fixtureSession();
      if (!fixture) return localError(401, "unauthorized");
      if (selected !== fixture.storeID) return localError(404, "not_found");
      const url = new URL(r.url); const host = r.headers.get("host");
      if (!["127.0.0.1", "localhost"].includes(url.hostname) || !["127.0.0.1:3100", "localhost:3100"].includes(host ?? ""))
        return localError(403, "forbidden");
      if (r.method !== "GET" && r.headers.get("origin") !== `http://${host}`) return localError(403, "forbidden");
      return fixture.token;
    },
  });
}
