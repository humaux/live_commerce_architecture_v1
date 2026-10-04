import "server-only";
import { headers } from "next/headers";
import {
  authConfig,
  authenticatedStores,
  exactCookieHeader,
  isBase64URL32,
  merchantBackend,
  safeError,
  SESSION_COOKIE,
} from "./auth";
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
  if (authConfig) return null;
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

export function onboardingPolicy() {
  const enabled = process.env.COMMERCE_ONBOARDING_ENABLED ?? "";
  if (enabled === "" || enabled === "0")
    return { enabled: false, currencies: [] as string[] };
  if (enabled !== "1") throw new Error("invalid COMMERCE_ONBOARDING_ENABLED");
  const raw = process.env.COMMERCE_ONBOARDING_CURRENCIES ?? "";
  const currencies = raw.split(",");
  if (
    currencies.length === 0 ||
    currencies.some((currency) => !/^[A-Z]{3}$/.test(currency)) ||
    new Set(currencies).size !== currencies.length
  )
    throw new Error("invalid COMMERCE_ONBOARDING_CURRENCIES");
  return { enabled: true, currencies };
}

async function currentSessionToken() {
  if (!authConfig) return null;
  const token =
    exactCookieHeader((await headers()).get("cookie"), SESSION_COOKIE) ?? "";
  return isBase64URL32(token) ? token : null;
}

async function responseError(response: Response) {
  return (await (await safeError(response)).json()) as APIError;
}

export async function callBackend(
  resource: string,
  init: RequestInit = {},
  token?: string,
  storeID?: string,
) {
  if (authConfig) {
    const current = token ?? (await currentSessionToken());
    if (!current || !storeID)
      return Response.json(noSessionError, { status: 401 });
    return merchantBackend(
      `/v1/admin/stores/${storeID}/${resource}`,
      current,
      init,
    );
  }
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
  // The dashboard landing only needs "signed in, which store" (merchant-tools): skip the warehouse and ledger reads.
  signInOnly = false,
): Promise<WorkspaceData> {
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
  if (authConfig) {
    const token = await currentSessionToken();
    if (!token) return empty;
    const listed = await authenticatedStores(token);
    if (!listed.stores)
      return { ...empty, error: await responseError(listed.response) };
    const store = [...listed.stores].sort((a, b) =>
      a.id.localeCompare(b.id),
    )[0];
    if (!store) return { ...empty, error: null };
    if (signInOnly) return { ...empty, storeID: store.id, storeName: store.name, error: null };
    const warehouseResult = await callBackend(
      "warehouses",
      {},
      token,
      store.id,
    );
    if (!warehouseResult.ok)
      return {
        ...empty,
        storeID: store.id,
        storeName: store.name,
        error: await responseError(warehouseResult),
      };
    const warehouses = (await warehouseResult.json()) as Page<Warehouse>;
    const warehouseID = warehouse || warehouses.items[0]?.id || "";
    const initial = {
      ...empty,
      storeID: store.id,
      storeName: store.name,
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
    const rows = await callBackend(
      `catalog-ledger?${params}`,
      {},
      token,
      store.id,
    );
    return rows.ok
      ? { ...initial, rows: (await rows.json()) as Page<LedgerRow> }
      : { ...initial, error: await responseError(rows) };
  }
  const session = fixtureSession();
  if (!session) return empty;
  if (signInOnly) return { ...empty, storeID: session.storeID, storeName: "Demo store", fixture: true, error: null };
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
