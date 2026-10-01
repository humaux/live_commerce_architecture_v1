// Query handling for catalog lists, shared by the server pages (searchParams) and app/api/shop/products (client "load
// more"). Pure: strings in, a clean ListQuery out. Unknown or malformed values are dropped, never forwarded, so the Go
// parser (strict: known keys, one value each) is only ever sent a canonical query.
import type { ListQuery } from "./shop-upstream.ts";

export const SORTS = ["newest", "price_asc", "price_desc", "title"] as const;
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

type Get = (key: string) => string | null | undefined;

export function parseListQuery(get: Get): ListQuery & { sort: (typeof SORTS)[number] } {
  const sort = SORTS.find((s) => s === get("sort")) ?? "newest";
  const q = (get("q") ?? "").replace(/[\u0000-\u001f\u007f]/g, " ").trim();
  const collection = get("collection") ?? "";
  const num = (key: string) => {
    const v = get(key) ?? "";
    return /^\d{1,13}$/.test(v) && Number(v) <= 1_000_000_000_000 ? Number(v) : null;
  };
  let min = num("min");
  let max = num("max");
  if (min !== null && max !== null && min > max) [min, max] = [max, min];
  const after = get("after") ?? "";
  const limit = Number(get("limit") ?? "");
  return {
    sort,
    q: [...q].length <= 60 ? q : [...q].slice(0, 60).join(""),
    collection: SLUG.test(collection) && collection.length <= 80 ? collection : "",
    min,
    max,
    after: /^[A-Za-z0-9_-]{1,256}$/.test(after) ? after : null,
    limit: Number.isInteger(limit) && limit >= 1 && limit <= 48 ? limit : 24,
  };
}
