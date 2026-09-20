import "server-only";
import type {
  APIError,
  LedgerRow,
  Page,
  Warehouse,
  WorkspaceData,
} from "./model";

// Temporary, explicit local acceptance adapter. It is impossible to enable in a
// production Next build. OIDC-backed per-user sessions replace it in T03; an env
// bearer must never become a shared production merchant login.
export function fixtureSession() {
  if (
    process.env.NODE_ENV !== "development" ||
    process.env.COMMERCE_FIXTURE_ENABLED !== "1"
  )
    return null;
  const endpoint = process.env.COMMERCE_API_ORIGIN;
  const token = process.env.COMMERCE_FIXTURE_TOKEN;
  const storeID = process.env.COMMERCE_FIXTURE_STORE_ID;
  if (
    !endpoint ||
    !token ||
    token.length < 32 ||
    !storeID ||
    !/^[0-9a-f-]{36}$/.test(storeID)
  )
    return null;
  try {
    const url = new URL(endpoint);
    if (
      url.protocol !== "http:" ||
      url.hostname !== "127.0.0.1" ||
      url.pathname !== "/" ||
      url.username ||
      url.password ||
      url.search ||
      url.hash
    )
      return null;
    return { origin: url.origin, token, storeID };
  } catch {
    return null;
  }
}

export const noSessionError: APIError = {
  code: "unauthorized",
  message: "Sign-in required.",
  request_id: "",
  retryable: false,
  details: {},
};

export async function callBackend(resource: string, init: RequestInit = {}) {
  const session = fixtureSession();
  if (!session) return Response.json(noSessionError, { status: 401 });
  // Resource is built only by the bounded route allowlist below, never a URL.
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${session.token}`);
  headers.set("Accept", "application/json");
  try {
    return await fetch(
      `${session.origin}/v1/admin/stores/${session.storeID}/${resource}`,
      {
        ...init,
        headers,
        cache: "no-store",
        redirect: "error",
        signal: AbortSignal.timeout(6000),
      },
    );
  } catch {
    return Response.json(
      {
        code: "retry_later",
        message: "Temporarily unavailable.",
        request_id: "",
        retryable: true,
        details: {},
      },
      { status: 503 },
    );
  }
}

export async function workspaceData(
  warehouse?: string,
  query = "",
  status = "all",
  cursor = "",
): Promise<WorkspaceData> {
  const session = fixtureSession();
  const empty: WorkspaceData = {
    storeID: "",
    storeName: "",
    fixture: false,
    rows: { items: [], next_cursor: "" },
    warehouses: [],
    warehouseCursor: "",
    warehouseID: "",
    error: noSessionError,
  };
  if (!session) return empty;
  const result = await callBackend("warehouses");
  if (!result.ok)
    return {
      ...empty,
      storeID: session.storeID,
      fixture: true,
      error: (await result.json()) as APIError,
    };
  const warehouses = (await result.json()) as Page<Warehouse>;
  const warehouseID = warehouse || warehouses.items[0]?.id || "";
  const initial = {
    ...empty,
    storeID: session.storeID,
    storeName: "Demo store",
    fixture: true,
    warehouses: warehouses.items,
    warehouseCursor: warehouses.next_cursor,
    warehouseID,
    error: null,
  };
  if (!warehouseID) return initial;
  const params = new URLSearchParams({
    warehouse_id: warehouseID,
    q: query,
    status,
    cursor,
  });
  const rows = await callBackend(`catalog-ledger?${params}`);
  return rows.ok
    ? { ...initial, rows: (await rows.json()) as Page<LedgerRow> }
    : { ...initial, error: (await rows.json()) as APIError };
}
