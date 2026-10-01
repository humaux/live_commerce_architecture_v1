// Catalog v2 admin requests (unit catalog-core). Browser -> BFF apps/admin/app/api/stores/[store]/[...resource]/route.ts
// -> Go internal/httpapi/collections.go (GET catalog-products, GET products/{id}, collections CRUD + PUT
// collections/{id}/products + the collection image), plus the existing product/SKU/inventory routes the editor reuses
// (PATCH products/{id}, POST skus, PATCH skus/{id}, POST skus/{id}/price, POST inventory/adjustments, GET warehouses,
// GET catalog-ledger). Reads reuse customers-client.get (session fence, no-store check); writes carry the CSRF cookie and
// the session boundary of the read that showed the screen, send ONE Idempotency-Key per user action, and report an
// `uncertain` outcome (5xx / lost answer) so the caller retries with the SAME key and bytes, never a fresh command.
// It never decides validity: the server is the authority; these only shape requests and parse answers.
import { csrfCookie, safeError, sessionBoundary } from "./settings-client";
import { get } from "./customers-client";
import {
  parseCollection, parseCollectionDetail, parseCollectionPage, parseLedgerStock, parseProduct, parseSummaryPage, parseWarehouses,
  type Collection, type CollectionDetail, type CollectionPage, type LedgerStock, type ProductDetail, type ProductSummaryPage,
} from "./catalog-v2-model";

const base = (store: string) => `/api/stores/${store}`;

export const readProducts = (store: string, q: string, status: string, cursor: string, signal: AbortSignal): Promise<ProductSummaryPage> => {
  const params = new URLSearchParams({ limit: "25" });
  if (q) params.set("q", q);
  if (status && status !== "all") params.set("status", status);
  if (cursor) params.set("cursor", cursor);
  return get(`${base(store)}/catalog-products?${params}`, parseSummaryPage, signal);
};
export const readProduct = (store: string, id: string, signal: AbortSignal): Promise<ProductDetail> =>
  get(`${base(store)}/products/${id}`, parseProduct, signal);
export const readCollections = (store: string, signal: AbortSignal): Promise<CollectionPage> =>
  get(`${base(store)}/collections?limit=100`, parseCollectionPage, signal);
export const readCollection = (store: string, id: string, signal: AbortSignal): Promise<CollectionDetail> =>
  get(`${base(store)}/collections/${id}`, parseCollectionDetail, signal);
export const readWarehouses = (store: string, signal: AbortSignal) => get(`${base(store)}/warehouses?limit=100`, parseWarehouses, signal);
// One SKU's balance version in one warehouse (0 = no balance yet), via the ledger; the adjust route needs it as expected_version.
export const readStockVersion = async (store: string, warehouse: string, code: string, skuID: string, signal: AbortSignal): Promise<number> => {
  const rows: LedgerStock[] = await get(`${base(store)}/catalog-ledger?warehouse_id=${warehouse}&q=${encodeURIComponent(code)}&limit=100`, parseLedgerStock, signal);
  return rows.find((row) => row.sku_id === skuID)?.balance_version ?? 0;
};
export const collectionImageURL = (store: string, id: string, imageID: string) => `${base(store)}/collections/${id}/image?v=${imageID}`;

export type Command = { key: string; method: "POST" | "PATCH" | "PUT"; resource: string; body: string };
export const command = (method: Command["method"], resource: string, body: unknown): Command =>
  ({ key: crypto.randomUUID(), method, resource, body: JSON.stringify(body) });
export type Outcome<T> = { ok: true; value: T } | { ok: false; code: string; uncertain: boolean };

// One fenced write. `boundary` is the session fence of the read that rendered the screen: a changed session sends nothing.
export async function send<T>(store: string, c: Command, boundary: string, parse: (value: unknown) => T): Promise<Outcome<T>> {
  const csrf = csrfCookie();
  try {
    if (!csrf || (await sessionBoundary(csrf)) !== boundary || csrfCookie() !== csrf) return { ok: false, code: "unauthorized", uncertain: false };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false };
  }
  try {
    const response = await fetch(`${base(store)}/${c.resource}`, {
      method: c.method, credentials: "same-origin", cache: "no-store",
      headers: { "Content-Type": "application/json", "Idempotency-Key": c.key, "X-CSRF-Token": csrf },
      body: c.body, signal: AbortSignal.timeout(12000),
    });
    const value: unknown = await response.json().catch(() => null);
    if (!response.ok) return { ok: false, code: safeError(value).code, uncertain: response.status >= 500 };
    try {
      return { ok: true, value: parse(value) };
    } catch {
      // Committed server side but unreadable here: the same key replays the stored answer, so a retry is safe.
      return { ok: false, code: "retry_later", uncertain: true };
    }
  } catch {
    return { ok: false, code: "retry_later", uncertain: true };
  }
}

// Collection image: multipart `file` part (server sniffs the type, 2 MiB cap) and delete. Photos of a product use
// components/ProductPhoto.tsx (lib/images-client.ts); this is the single-image variant for collections.
export const MAX_COLLECTION_IMAGE = 2 * 1024 * 1024;
export async function uploadCollectionImage(store: string, id: string, file: File, boundary: string): Promise<Outcome<Collection | null>> {
  const csrf = csrfCookie();
  try {
    if (!csrf || (await sessionBoundary(csrf)) !== boundary) return { ok: false, code: "unauthorized", uncertain: false };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false };
  }
  const form = new FormData();
  form.append("file", file);
  try {
    const response = await fetch(`${base(store)}/collections/${id}/image`, {
      method: "POST", credentials: "same-origin", cache: "no-store",
      headers: { "Idempotency-Key": crypto.randomUUID(), "X-CSRF-Token": csrf }, body: form, signal: AbortSignal.timeout(20000),
    });
    const value: unknown = await response.json().catch(() => null);
    return response.ok ? { ok: true, value: null } : { ok: false, code: response.status === 413 ? "too_large" : safeError(value).code, uncertain: response.status >= 500 };
  } catch {
    return { ok: false, code: "retry_later", uncertain: true };
  }
}
export const deleteCollectionImage = (store: string, id: string, boundary: string) =>
  send(store, command("POST", `collections/${id}/image/delete`, {}), boundary, parseCollection);
