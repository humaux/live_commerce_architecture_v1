// Catalog v2 admin model (unit catalog-core, contracts/storefront-v2.md section A): wire types and parsers for the
// Go merchant routes behind the BFF (`catalog-products`, `products/{id}`, `collections*` -> internal/httpapi/
// collections.go, internal/catalog) plus the pure helpers the editor needs: the option-axis -> SKU matrix, the derived
// variant title, and currency-aware price parsing. A parser refuses a malformed read (type, bounds); the server stays
// the authority for every rule (slug uniqueness, option alignment, compare-at > price, versions). No network, no DOM.
import { canonicalCursor, canonicalUUID, wholeOnly } from "./orders-model.ts";

export type OptionAxis = { name: string; values: string[] };
export type ProductStatus = "draft" | "active" | "archived";
export type ProductSummary = {
  id: string;
  slug: string;
  name: string;
  status: ProductStatus;
  version: number;
  cover_image_id: string | null;
  price_min_minor: number | null;
  price_max_minor: number | null;
  currency: string;
  sku_count: number;
  available: number;
};
export type ProductSummaryPage = { items: ProductSummary[]; next_cursor: string };
export type Variant = {
  id: string;
  code: string;
  title: string;
  price_minor: number;
  compare_at_minor: number | null;
  option_values: string[];
  version: number;
  currency: string;
  available: number;
  // Logistics/customs fields the SKU PATCH replaces as a whole: the editor must send them back unchanged.
  weight_grams: number;
  length_mm: number;
  width_mm: number;
  height_mm: number;
  origin_country: string;
  customs_name: string;
  hs_candidate: string;
};
export type ProductDetail = {
  id: string;
  slug: string;
  name: string;
  description: string;
  status: ProductStatus;
  version: number;
  seo_title: string;
  seo_description: string;
  options: OptionAxis[];
  skus: Variant[];
};
export type CollectionStatus = "active" | "hidden";
export type CollectionSort = "manual" | "newest" | "price_asc" | "price_desc";
export type Collection = {
  id: string;
  slug: string;
  title: string;
  description: string;
  sort_mode: CollectionSort;
  status: CollectionStatus;
  image_id: string | null;
  product_count: number;
  version: number;
};
export type CollectionMember = {
  product_id: string;
  name: string;
  slug: string;
  status: ProductStatus;
  position: number;
  cover_image_id: string | null;
};
export type CollectionDetail = Collection & { products: CollectionMember[] };
export type CollectionPage = { items: Collection[]; next_cursor: string };

// Limits mirrored from the Go validators (internal/catalog options.go); the server repeats every one.
export const limits = {
  name: 120, description: 8000, slug: 80, seoTitle: 70, seoDescription: 160,
  axes: 3, axisName: 30, axisValues: 50, axisValue: 40, variants: 100,
  collectionTitle: 80, collectionDescription: 2000, collectionProducts: 500, code: 64,
} as const;
const maxMoney = 1_000_000_000_000;
// Plain module on purpose: server pages import these values (a "use client" module exports client references only).
export const productStatuses = ["all", "draft", "active", "archived"] as const;
export const slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
export const codePattern = /^[A-Za-z0-9_.-]{1,64}$/;

type Rec = Record<string, unknown>;
const rec = (v: unknown): Rec => {
  if (!v || typeof v !== "object" || Array.isArray(v)) throw new Error("invalid");
  return v as Rec;
};
const str = (v: unknown, max: number, min = 0) => {
  if (typeof v !== "string" || Array.from(v).length > max || Array.from(v).length < min) throw new Error("invalid");
  return v;
};
const int = (v: unknown, min: number, max: number) => {
  if (typeof v !== "number" || !Number.isInteger(v) || v < min || v > max) throw new Error("invalid");
  return v;
};
const nullable = <T>(v: unknown, parse: (x: unknown) => T): T | null => (v === null ? null : parse(v));
const uuid = (v: unknown) => {
  if (typeof v !== "string" || !canonicalUUID.test(v)) throw new Error("invalid");
  return v;
};
const oneOf = <T extends string>(v: unknown, allowed: readonly T[]): T => {
  if (typeof v !== "string" || !allowed.includes(v as T)) throw new Error("invalid");
  return v as T;
};
const list = <T>(v: unknown, max: number, parse: (x: unknown) => T): T[] => {
  if (!Array.isArray(v) || v.length > max) throw new Error("invalid");
  return v.map(parse);
};
const statuses = ["draft", "active", "archived"] as const;

export function parseAxis(v: unknown): OptionAxis {
  const r = rec(v);
  return { name: str(r.name, limits.axisName, 1), values: list(r.values, limits.axisValues, (x) => str(x, limits.axisValue, 1)) };
}
export function parseSummary(v: unknown): ProductSummary {
  const r = rec(v);
  return {
    id: uuid(r.id), slug: str(r.slug, limits.slug, 1), name: str(r.name, limits.name, 1), status: oneOf(r.status, statuses),
    version: int(r.version, 1, Number.MAX_SAFE_INTEGER), cover_image_id: nullable(r.cover_image_id, uuid),
    price_min_minor: nullable(r.price_min_minor, (x) => int(x, 0, maxMoney)), price_max_minor: nullable(r.price_max_minor, (x) => int(x, 0, maxMoney)),
    currency: str(r.currency, 3), sku_count: int(r.sku_count, 0, 1000), available: int(r.available, -maxMoney, maxMoney),
  };
}
export function parseSummaryPage(v: unknown): ProductSummaryPage {
  const r = rec(v);
  const next = typeof r.next_cursor === "string" ? r.next_cursor : "";
  if (next && !canonicalCursor.test(next)) throw new Error("invalid");
  return { items: list(r.items, 100, parseSummary), next_cursor: next };
}
export function parseVariant(v: unknown): Variant {
  const r = rec(v);
  return {
    id: uuid(r.id), code: str(r.code, limits.code, 1), title: str(r.title, 200, 1), price_minor: int(r.price_minor, 0, maxMoney),
    compare_at_minor: nullable(r.compare_at_minor, (x) => int(x, 1, maxMoney)), option_values: list(r.option_values, limits.axes, (x) => str(x, limits.axisValue, 1)),
    version: int(r.version, 1, Number.MAX_SAFE_INTEGER), currency: str(r.currency, 3, 3), available: int(r.available, -maxMoney, maxMoney),
    weight_grams: int(r.weight_grams, 0, 1_000_000_000), length_mm: int(r.length_mm, 0, 1_000_000), width_mm: int(r.width_mm, 0, 1_000_000),
    height_mm: int(r.height_mm, 0, 1_000_000), origin_country: str(r.origin_country, 2), customs_name: str(r.customs_name, 240), hs_candidate: str(r.hs_candidate, 12),
  };
}
export function parseProduct(v: unknown): ProductDetail {
  const r = rec(v);
  return {
    id: uuid(r.id), slug: str(r.slug, limits.slug, 1), name: str(r.name, limits.name, 1), description: str(r.description, limits.description),
    status: oneOf(r.status, statuses), version: int(r.version, 1, Number.MAX_SAFE_INTEGER), seo_title: str(r.seo_title, limits.seoTitle),
    seo_description: str(r.seo_description, limits.seoDescription), options: list(r.options, limits.axes, parseAxis),
    skus: list(r.skus, 200, parseVariant),
  };
}
// Create/patch/price/archive answers only need to be well-formed objects with an id: the editor re-reads after every write.
// inventory.Balance answer of POST inventory/adjustments: identified by warehouse_id + sku_id (no `id`).
export const parseBalance = (v: unknown): { sku_id: string } => {
  const r = rec(v);
  uuid(r.warehouse_id);
  return { sku_id: uuid(r.sku_id) };
};
export const parseCreated = (v: unknown): { id: string } => ({ id: uuid(rec(v).id) });
// A SKU as the create/patch/price routes answer it (no stock): the editor re-reads the product after every write.
export function parseWarehouses(v: unknown): { id: string; name: string }[] {
  return list(rec(v).items, 100, (x) => {
    const w = rec(x);
    return { id: uuid(w.id), name: str(w.name, 120, 1) };
  });
}
export type LedgerStock = { sku_id: string; balance_version: number };
export function parseLedgerStock(v: unknown): LedgerStock[] {
  return list(rec(v).items, 100, (x) => {
    const r = rec(x);
    return { sku_id: uuid(r.sku_id), balance_version: int(r.balance_version, 0, Number.MAX_SAFE_INTEGER) };
  });
}
export function parseCollection(v: unknown): Collection {
  const r = rec(v);
  return {
    id: uuid(r.id), slug: str(r.slug, limits.slug, 1), title: str(r.title, limits.collectionTitle, 1),
    description: str(r.description, limits.collectionDescription), sort_mode: oneOf(r.sort_mode, ["manual", "newest", "price_asc", "price_desc"] as const),
    status: oneOf(r.status, ["active", "hidden"] as const), image_id: nullable(r.image_id, uuid),
    product_count: int(r.product_count, 0, limits.collectionProducts), version: int(r.version, 1, Number.MAX_SAFE_INTEGER),
  };
}
export function parseCollectionDetail(v: unknown): CollectionDetail {
  const r = rec(v);
  return {
    ...parseCollection(v),
    products: list(r.products, limits.collectionProducts, (x) => {
      const m = rec(x);
      return {
        product_id: uuid(m.product_id), name: str(m.name, limits.name, 1), slug: str(m.slug, limits.slug, 1), status: oneOf(m.status, statuses),
        position: int(m.position, 0, limits.collectionProducts - 1), cover_image_id: nullable(m.cover_image_id, uuid),
      };
    }),
  };
}
export function parseCollectionPage(v: unknown): CollectionPage {
  const r = rec(v);
  const next = typeof r.next_cursor === "string" ? r.next_cursor : "";
  if (next && !canonicalCursor.test(next)) throw new Error("invalid");
  return { items: list(r.items, 100, parseCollection), next_cursor: next };
}

// ---- pure editor helpers ----------------------------------------------------------------------------------------

// Derived variant title, the same rule as the server (values joined " / ", "預設" without axes).
export const variantTitle = (values: string[]) => (values.length ? values.join(" / ") : "預設");

// One option-value combination per SKU, in axis order (first axis varies slowest). An axis with no value yields none.
export function variantMatrix(axes: OptionAxis[]): string[][] {
  return axes.reduce<string[][]>((rows, axis) => rows.flatMap((row) => axis.values.map((value) => [...row, value])), [[]])
    .filter((row) => row.length === axes.length);
}
export const sameValues = (a: string[], b: string[]) => a.length === b.length && a.every((value, i) => value === b[i]);

// Axes the editor may send: trimmed, empty values dropped, an axis without name or values removed. Returns an error
// code for a duplicate or an over-limit axis, so the form can show it before the server does.
export function cleanAxes(axes: OptionAxis[]): { axes: OptionAxis[]; error: "" | "duplicate" | "limit" } {
  const cleaned = axes
    .map((axis) => ({ name: axis.name.trim(), values: axis.values.map((v) => v.trim()).filter(Boolean) }))
    .filter((axis) => axis.name && axis.values.length);
  if (cleaned.length > limits.axes) return { axes: cleaned, error: "limit" };
  const names = new Set(cleaned.map((axis) => axis.name));
  const dup = names.size !== cleaned.length || cleaned.some((axis) => new Set(axis.values).size !== axis.values.length);
  const over = cleaned.some((axis) => axis.values.length > limits.axisValues || Array.from(axis.name).length > limits.axisName ||
    axis.values.some((v) => Array.from(v).length > limits.axisValue));
  return { axes: cleaned, error: dup ? "duplicate" : over ? "limit" : "" };
}

// Currency exponent from Intl: the number of minor digits on the wire (TWD 2 in this system, so NT$60 is 6000; USD 2; JPY 0), the same
// source `money()` divides by.
export const fractionDigits = (currency: string) => {
  try {
    return new Intl.NumberFormat("en", { style: "currency", currency }).resolvedOptions().maximumFractionDigits ?? 2;
  } catch {
    return 2;
  }
};
// wholeOnly(currency) (orders-model): TWD is typed and shown as WHOLE dollars although the wire is x100 (Go refuses price_minor % 100 != 0,
// product-editor §c10); every other currency keeps its own decimals.
// Major-unit text -> wire minor units: "60" -> 6000 (TWD), "12.5" -> 1250 (USD). null = not a price, or a fraction the currency does
// not take ("60.5" for TWD, "1.5" for JPY).
export function toMinor(input: string, currency: string): number | null {
  const digits = fractionDigits(currency);
  const m = /^(\d{1,12})(?:\.(\d{1,6}))?$/.exec(input.trim());
  const fraction = m?.[2] ?? "";
  if (!m || fraction.length > (wholeOnly(currency) ? 0 : digits)) return null;
  const minor = Number(m[1] + fraction.padEnd(digits, "0"));
  return Number.isSafeInteger(minor) && minor <= maxMoney ? minor : null;
}
// Wire minor units -> the text the merchant edits: 6000 -> "60" (TWD), 1250 -> "12.50" (USD). A legacy TWD amount with cents (50) stays
// "0.50" so it is never shown as another price; toMinor then refuses it until it is retyped as whole dollars.
export function fromMinor(minor: number, currency: string): string {
  const digits = fractionDigits(currency);
  if (digits === 0) return String(minor);
  const text = String(minor).padStart(digits + 1, "0");
  const whole = text.slice(0, -digits), fraction = text.slice(-digits);
  return wholeOnly(currency) && Number(fraction) === 0 ? whole : `${whole}.${fraction}`;
}

// Proposed SKU code for one option combination: product slug + the 1-based position of each value on its axis
// (`summer-tee-2-1`). ASCII by construction for any script of option values (a CJK value used to vanish from the code and
// collide), unique per product because values are unique per axis, stable when values are appended. Codes are unique per store.
export function proposedCode(slug: string, axes: OptionAxis[], values: string[]): string {
  const suffix = ["", ...values.map((v, i) => String(axes[i].values.indexOf(v) + 1))].join("-") || "-1";
  return slug.slice(0, limits.code - suffix.length).replace(/-+$/, "") + suffix;
}
