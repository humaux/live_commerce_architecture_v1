// Server-only reads of the public storefront data (React server components and app/api/shop/*): the store design
// document and the catalog-v2 lists. BFF routes it reaches: none of /api/buyer/* (that is the session-bound cart BFF);
// this is the same cookie-less trust shape as app/media/p/[productID]/[imageID]/route.ts: the BFF key plus the verified
// Host-derived origin, never a caller header, cookie or tenant/store id. Go endpoints behind it (internal/buyerhttp):
//   GET /v1/buyer/design/published | design/preview (token in X-Commerce-Design-Preview)  -> design.go
//   GET /v1/buyer/catalog/v2/{products,products/{slug_or_id},collections,collections/{slug}} -> catalogv2.go
// The store is chosen by Go from the origin (buyer.resolve_published_store); a miss is 404 for unknown host, unpublished
// store, draft/archived product and foreign rows alike, so nothing is learned from it. 404/422 -> "missing", anything
// else that is not a clean 200 -> "down" (retryable; pages show the branded error state, never a half page).
// It never caches across requests (stock hints and price must be current; the page is dynamic by host anyway).
import { cache } from "react";
import { headers } from "next/headers";
import { candidateOrigin, readBounded, upstreamConfig } from "./public-upstream.ts";
import { normalizeDesign } from "./design.ts";
import type { Design } from "./design.ts";
import {
  parseCollection,
  parseCollections,
  parseProductDetail,
  parseProductList,
} from "./shop-contract.ts";
import type { CollectionCard, CollectionInfo, ProductDetail, ProductList } from "./shop-contract.ts";

export type Fetched<T> = { state: "ok"; value: T } | { state: "missing" } | { state: "down" };

const MAX_BODY = 1024 * 1024;
const PREVIEW_TOKEN = /^[A-Za-z0-9_-]{43}$/;
// Written by proxy.ts from the ?preview= query only (it deletes any client-sent copy); format re-checked here.
export const PREVIEW_HEADER = "x-shop-preview";

export async function previewToken(): Promise<string | null> {
  const value = (await headers()).get(PREVIEW_HEADER);
  return value && PREVIEW_TOKEN.test(value) ? value : null;
}

export async function shopOrigin(): Promise<string | null> {
  return candidateOrigin((await headers()).get("host"));
}

async function getJSON(path: string, preview: string | null = null): Promise<Fetched<unknown>> {
  const cfg = upstreamConfig(process.env);
  const origin = await shopOrigin();
  if (!cfg) return { state: "down" };
  if (!origin) return { state: "missing" };
  const outbound = new Headers({
    Accept: "application/json",
    "X-Commerce-Buyer-BFF-Key": cfg.bff,
    "X-Commerce-Storefront-Origin": origin,
  });
  if (preview) outbound.set("X-Commerce-Design-Preview", preview);
  let response: Response;
  try {
    response = await fetch(`${cfg.api}/v1/buyer/${path}`, {
      method: "GET",
      headers: outbound,
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.timeout(8000),
    });
  } catch {
    return { state: "down" };
  }
  if (response.status === 404 || response.status === 422) {
    await response.body?.cancel();
    return { state: "missing" };
  }
  if (response.status !== 200 || (response.headers.get("content-type") ?? "").split(";", 1)[0].trim().toLowerCase() !== "application/json")
    return { state: "down" };
  const bytes = await readBounded(response.body, MAX_BODY);
  if (!bytes) return { state: "down" };
  try {
    return { state: "ok", value: JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)) as unknown };
  } catch {
    return { state: "down" };
  }
}

function parsed<T>(f: Fetched<unknown>, parse: (v: unknown) => T | null): Fetched<T> {
  if (f.state !== "ok") return f;
  const value = parse(f.value);
  return value === null ? { state: "down" } : { state: "ok", value };
}

export type Shop =
  | { state: "ok"; design: Design; version: number; origin: string; draft: boolean }
  | { state: "closed" }
  | { state: "down" };

// Design for this request: the draft when a valid preview token is present, else the published version. An unknown,
// expired or stale token falls back to the published view (Go answers it as 404, indistinguishable from "no draft").
// React cache(): the layout and the page share one fetch per request.
export const loadShop = cache(async (preview: string | null): Promise<Shop> => {
  const origin = await shopOrigin();
  if (!origin) return { state: "closed" };
  let draft = false;
  let result: Fetched<unknown> | null = null;
  if (preview) {
    result = await getJSON("design/preview", preview);
    draft = result.state === "ok";
  }
  if (!result || result.state !== "ok") {
    if (result && result.state === "down") return { state: "down" };
    result = await getJSON("design/published");
  }
  if (result.state === "missing") return { state: "closed" };
  if (result.state === "down") return { state: "down" };
  const body = result.value as { version?: unknown; document?: unknown } | null;
  const version = typeof body?.version === "number" ? body.version : 0;
  return { state: "ok", design: normalizeDesign(body?.document, ""), version, origin, draft };
});

export type ListQuery = {
  collection?: string;
  q?: string;
  sort?: string;
  min?: number | null;
  max?: number | null;
  after?: string | null;
  limit?: number;
};

// Same canonical query the Go parser accepts (parseV2Query: known keys only, one value each, no empty value).
export function listPath(query: ListQuery): string {
  const params = new URLSearchParams();
  if (query.collection) params.set("collection", query.collection);
  if (query.q) params.set("q", query.q);
  if (query.sort && query.sort !== "newest") params.set("sort", query.sort);
  if (query.min != null) params.set("min", String(query.min));
  if (query.max != null) params.set("max", String(query.max));
  if (query.after) params.set("after", query.after);
  if (query.limit) params.set("limit", String(query.limit));
  const qs = params.toString();
  return `catalog/v2/products${qs ? `?${qs}` : ""}`;
}

export const listProducts = async (query: ListQuery): Promise<Fetched<ProductList>> => parsed(await getJSON(listPath(query)), parseProductList);
// Store currency for pages whose own read has none (the product detail): one card from the list route carries it.
export const storeCurrency = cache(async (): Promise<Fetched<string>> => {
  const page = await listProducts({ limit: 1 });
  return page.state === "ok" ? { state: "ok", value: page.value.store.currency } : page;
});
export const getProduct = cache(async (key: string): Promise<Fetched<ProductDetail>> => parsed(await getJSON(`catalog/v2/products/${encodeURIComponent(key)}`), parseProductDetail));
export const listCollections = cache(async (): Promise<Fetched<CollectionCard[]>> => parsed(await getJSON("catalog/v2/collections"), parseCollections));
export const getCollection = cache(async (slug: string): Promise<Fetched<CollectionInfo>> => parsed(await getJSON(`catalog/v2/collections/${encodeURIComponent(slug)}`), parseCollection));

// Every active product card for the sitemap: follows the cursor up to `maxPages` pages of 48 (ponytail: 2400 products;
// a larger store needs a sitemap index, add it when a pilot store passes that).
export async function allProducts(maxPages = 50): Promise<Fetched<ProductList["products"]>> {
  const out: ProductList["products"] = [];
  let after: string | null = null;
  for (let i = 0; i < maxPages; i++) {
    const page: Fetched<ProductList> = await listProducts({ after, limit: 48 });
    if (page.state !== "ok") return i === 0 ? page : { state: "ok", value: out };
    out.push(...page.value.products);
    if (!page.value.next) break;
    after = page.value.next;
  }
  return { state: "ok", value: out };
}

