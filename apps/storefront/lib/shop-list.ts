// Server-side loader shared by the three list pages (collection, all products, search). BFF/Go: the catalog-v2 list route
// (lib/shop-upstream.ts listProducts). URL `min`/`max` are what the buyer typed in the price filter, in MAJOR units
// ("500" = NT$500); the Go list takes MINOR units, so the conversion needs the store currency. The currency is only known
// from a list response, so a filtered request first asks for one card to learn it (an extra round trip only when filtering).
import type { Locale } from "@live-commerce/i18n";
import { majorToMinor, minorToMajor } from "./money.ts";
import { parseListQuery } from "./shop-query.ts";
import { listProducts } from "./shop-upstream";
import type { Fetched, ListQuery } from "./shop-upstream";
import type { ProductList } from "./shop-contract.ts";

export type SearchParams = Record<string, string | string[] | undefined>;
const first = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v);

export type Loaded = {
  query: ListQuery & { sort: string };
  list: Fetched<ProductList>;
  form: { sort: string; min: string; max: string; q: string };
  filtered: boolean;
};

export async function loadList(locale: Locale, params: SearchParams, fixed: { collection?: string } = {}): Promise<Loaded> {
  const get = (key: string) => (key === "min" || key === "max" ? null : first(params[key]));
  const base = parseListQuery(get);
  const query = { ...base, collection: fixed.collection ?? base.collection, min: null, max: null } as Loaded["query"];
  const typedMin = (first(params.min) ?? "").trim();
  const typedMax = (first(params.max) ?? "").trim();
  if (typedMin || typedMax) {
    const probe = await listProducts({ limit: 1 });
    if (probe.state === "ok") {
      const currency = probe.value.store.currency;
      query.min = majorToMinor(locale, currency, typedMin);
      query.max = majorToMinor(locale, currency, typedMax);
      if (query.min !== null && query.max !== null && query.min > query.max) [query.min, query.max] = [query.max, query.min];
    }
  }
  const list = await listProducts(query);
  const currency = list.state === "ok" ? list.value.store.currency : "TWD";
  return {
    query,
    list,
    form: {
      sort: query.sort,
      min: query.min != null ? minorToMajor(locale, currency, query.min) : "",
      max: query.max != null ? minorToMajor(locale, currency, query.max) : "",
      q: query.q ?? "",
    },
    filtered: query.min != null || query.max != null,
  };
}

// Canonical query string for the client "load more" (minor units, no cursor, defaults omitted).
export function clientQuery(query: ListQuery): string {
  const params = new URLSearchParams();
  if (query.collection) params.set("collection", query.collection);
  if (query.q) params.set("q", query.q);
  if (query.sort && query.sort !== "newest") params.set("sort", query.sort);
  if (query.min != null) params.set("min", String(query.min));
  if (query.max != null) params.set("max", String(query.max));
  if (query.limit) params.set("limit", String(query.limit));
  return params.toString();
}

// The no-JS "next page" link: same page, same filters in the buyer's units, plus the opaque cursor.
export function nextHref(path: string, params: SearchParams, cursor: string): string {
  const out = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    const v = first(value);
    if (key !== "after" && v !== undefined) out.set(key, v);
  }
  out.set("after", cursor);
  return `${path}?${out}`;
}
