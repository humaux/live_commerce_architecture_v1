// Wire shapes of the catalog-v2 buyer reads (contracts/storefront-v2.md section A; Go internal/buyerhttp/catalogv2.go).
// Pure parsers shared by the server layer (lib/shop-upstream.ts), the public proxy routes (app/api/shop/*) and the client
// islands. They return a closed typed value or null; a null means "treat as unavailable", never "render half a page".
// They never decide price/stock (Go does) and never accept a field the contract does not list.

// product-media-v2 caps (contracts/catalog-inventory-v1.md); pinned to Go catalog.MaxMainImages / MaxDetailImages / MaxOptionImages
// by tests/product-media-parity (apps/storefront/tests/product-media.test.mjs). Change a number here only with the Go constant.
export const MAX_MAIN_IMAGES = 4;
export const MAX_DETAIL_IMAGES = 20;
export const MAX_OPTION_IMAGES = 50;

export type StockHint = "in" | "low" | "out";
export type ImageSize = { width: 360 | 720 | 1080; pixel_width: number };
export type ProductCard = {
  id: string;
  slug: string;
  title: string;
  price_min_minor: number;
  price_max_minor: number;
  compare_at_min_minor: number | null;
  cover_image_id: string | null;
  cover_image_sizes?: ImageSize[];
  in_stock: boolean;
};
export type ProductList = { store: { name: string; currency: string }; products: ProductCard[]; next: string | null };
export type Variant = {
  sku_id: string;
  title: string;
  option_values: string[];
  price_minor: number;
  compare_at_minor: number | null;
  stock: StockHint;
  // product-media-v2: the image linked to this variant's value on the image axis (null = use the cover).
  image_id: string | null;
};
export type DetailImage = { id: string; width: number | null; height: number | null; sizes?: ImageSize[] };
export type ProductDetail = {
  id: string;
  slug: string;
  title: string;
  description: string;
  seo: { title: string; description: string };
  images: DetailImage[]; // main images, <= MAX_MAIN_IMAGES, cover first
  detail_images: DetailImage[]; // ordered detail-page stack, <= MAX_DETAIL_IMAGES
  image_axis: string | null; // effective option axis that carries option_images
  option_images: { value: string; image_id: string }[];
  options: { name: string; values: string[] }[];
  variants: Variant[];
  collections: { slug: string; title: string }[];
};
export type CollectionCard = { id: string; slug: string; title: string; image_id: string | null; product_count: number };
export type CollectionInfo = { id: string; slug: string; title: string; description: string; image_id: string | null };

// Only Host-scoped PUBLIC catalog responses belong here, never an admin catalog or a global cache.
export const relatedCards = (cards: ProductCard[], currentID: string): ProductCard[] =>
  cards.filter((card, i) => card.id !== currentID && cards.findIndex((other) => other.id === card.id) === i).slice(0, 8);
export const nonEmptyCollections = (collections: CollectionCard[]): CollectionCard[] =>
  collections.filter((collection) => collection.product_count > 0);

const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const rec = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const str = (v: unknown, max = 100000): v is string => typeof v === "string" && v.length <= max;
const money = (v: unknown): v is number => typeof v === "number" && Number.isSafeInteger(v) && v >= 0 && v <= 1_000_000_000_000;
const maybe = <T>(v: unknown, ok: (x: unknown) => x is T): v is T | null => v === null || ok(v);
const uuid = (v: unknown): v is string => typeof v === "string" && UUID.test(v);
const dim = (v: unknown): v is number => typeof v === "number" && Number.isInteger(v) && v > 0 && v < 100000;
function imageSizes(v: unknown): ImageSize[] | null {
  if (v === undefined) return [];
  if (!Array.isArray(v) || v.length > 3) return null;
  const seen = new Set<number>();
  const out: ImageSize[] = [];
  for (const s of v) {
    if (!rec(s) || (s.width !== 360 && s.width !== 720 && s.width !== 1080) || !dim(s.pixel_width) || s.pixel_width > s.width || seen.has(s.width)) return null;
    seen.add(s.width); out.push({width: s.width, pixel_width: s.pixel_width});
  }
  return out;
}

function card(v: unknown): ProductCard | null {
  if (!rec(v) || !uuid(v.id) || !str(v.slug, 80) || !str(v.title, 300) || !money(v.price_min_minor) || !money(v.price_max_minor)) return null;
  if (!maybe(v.compare_at_min_minor, money) || !maybe(v.cover_image_id, uuid) || typeof v.in_stock !== "boolean") return null;
  const sizes = imageSizes(v.cover_image_sizes);
  if (!sizes) return null;
  return {
    id: v.id,
    slug: v.slug,
    title: v.title,
    price_min_minor: v.price_min_minor,
    price_max_minor: v.price_max_minor,
    compare_at_min_minor: v.compare_at_min_minor as number | null,
    cover_image_id: v.cover_image_id as string | null,
    ...(sizes.length ? {cover_image_sizes: sizes} : {}),
    in_stock: v.in_stock,
  };
}

export function parseProductList(v: unknown): ProductList | null {
  if (!rec(v) || !rec(v.store) || !str(v.store.name, 200) || !/^[A-Z]{3}$/.test(String(v.store.currency))) return null;
  if (!Array.isArray(v.products) || v.products.length > 100 || !maybe(v.next, (x): x is string => str(x, 2048))) return null;
  const products: ProductCard[] = [];
  for (const p of v.products) {
    const parsed = card(p);
    if (!parsed) return null;
    products.push(parsed);
  }
  return { store: { name: v.store.name, currency: String(v.store.currency) }, products, next: v.next as string | null };
}

// One image list of the detail wire shape, capped at `max` (the Go per-role cap); null when malformed or over the cap.
function parseDetailImages(list: unknown, max: number): DetailImage[] | null {
  if (!Array.isArray(list) || list.length > max) return null;
  const out: DetailImage[] = [];
  for (const i of list) {
    if (!rec(i) || !uuid(i.id) || !maybe(i.width, dim) || !maybe(i.height, dim)) return null;
    const sizes = imageSizes(i.sizes);
    if (!sizes) return null;
    out.push({ id: i.id, width: i.width as number | null, height: i.height as number | null, ...(sizes.length ? { sizes } : {}) });
  }
  return out;
}

export function parseProductDetail(v: unknown): ProductDetail | null {
  if (!rec(v) || !uuid(v.id) || !str(v.slug, 80) || !str(v.title, 300) || !str(v.description, 20000)) return null;
  if (!rec(v.seo) || !str(v.seo.title, 200) || !str(v.seo.description, 400)) return null;
  if (!Array.isArray(v.options) || v.options.length > 3) return null;
  if (!Array.isArray(v.variants) || v.variants.length < 1 || v.variants.length > 500 || !Array.isArray(v.collections)) return null;
  const images = parseDetailImages(v.images, MAX_MAIN_IMAGES);
  const detailImages = v.detail_images === undefined ? [] : parseDetailImages(v.detail_images, MAX_DETAIL_IMAGES); // absent = a backend before product-media-v2
  if (!images || !detailImages) return null;
  if (v.image_axis !== undefined && !maybe(v.image_axis, (x): x is string => str(x, 60))) return null;
  const optionImages: ProductDetail["option_images"] = [];
  if (v.option_images !== undefined) {
    if (!Array.isArray(v.option_images) || v.option_images.length > MAX_OPTION_IMAGES) return null;
    for (const o of v.option_images) {
      if (!rec(o) || !str(o.value, 80) || !uuid(o.image_id)) return null;
      optionImages.push({ value: o.value, image_id: o.image_id });
    }
  }
  const options: ProductDetail["options"] = [];
  for (const o of v.options) {
    if (!rec(o) || !str(o.name, 60) || !Array.isArray(o.values) || o.values.length < 1 || o.values.length > 50 || !o.values.every((x) => str(x, 80))) return null;
    options.push({ name: o.name, values: o.values as string[] });
  }
  const variants: Variant[] = [];
  for (const x of v.variants) {
    if (!rec(x) || !uuid(x.sku_id) || !str(x.title, 200) || !Array.isArray(x.option_values) || !x.option_values.every((y) => str(y, 80))) return null;
    if (x.option_values.length !== options.length || !money(x.price_minor) || !maybe(x.compare_at_minor, money)) return null;
    if (x.stock !== "in" && x.stock !== "low" && x.stock !== "out") return null;
    if (x.image_id !== undefined && !maybe(x.image_id, uuid)) return null;
    variants.push({
      sku_id: x.sku_id,
      title: x.title,
      option_values: x.option_values as string[],
      price_minor: x.price_minor,
      compare_at_minor: x.compare_at_minor as number | null,
      stock: x.stock,
      image_id: (x.image_id as string | null | undefined) ?? null,
    });
  }
  const collections: ProductDetail["collections"] = [];
  for (const c of v.collections) {
    if (!rec(c) || !str(c.slug, 80) || !str(c.title, 200)) return null;
    collections.push({ slug: c.slug, title: c.title });
  }
  return { id: v.id, slug: v.slug, title: v.title, description: v.description, seo: { title: v.seo.title, description: v.seo.description }, images, detail_images: detailImages, image_axis: (v.image_axis as string | null | undefined) ?? null, option_images: optionImages, options, variants, collections };
}

export function parseCollections(v: unknown): CollectionCard[] | null {
  if (!rec(v) || !Array.isArray(v.collections) || v.collections.length > 200) return null;
  const out: CollectionCard[] = [];
  for (const c of v.collections) {
    if (!rec(c) || !uuid(c.id) || !str(c.slug, 80) || !str(c.title, 200) || !maybe(c.image_id, uuid) || typeof c.product_count !== "number" || !Number.isInteger(c.product_count) || c.product_count < 0) return null;
    // `id` (contracts/storefront-v2.md section A, amended with migration 0093) builds /media/c/{id}/{image_id}.
    out.push({ id: c.id, slug: c.slug, title: c.title, image_id: c.image_id as string | null, product_count: c.product_count });
  }
  return out;
}

export function parseCollection(v: unknown): CollectionInfo | null {
  if (!rec(v) || !uuid(v.id) || !str(v.slug, 80) || !str(v.title, 200) || !str(v.description, 4000) || !maybe(v.image_id, uuid)) return null;
  return { id: v.id, slug: v.slug, title: v.title, description: v.description, image_id: v.image_id as string | null };
}

// ---- derived display helpers (no I/O) ---------------------------------------------------------------------------------

// Cheapest in-stock variant first (what the card promises), else the cheapest variant.
export function priceBounds(variants: Variant[]): { min: number; max: number; compareAtMin: number | null } {
  const sellable = variants.filter((x) => x.stock !== "out");
  const pool = sellable.length ? sellable : variants;
  const prices = pool.map((x) => x.price_minor);
  const compares = pool.filter((x) => x.compare_at_minor !== null && x.compare_at_minor > x.price_minor).map((x) => x.compare_at_minor as number);
  return { min: Math.min(...prices), max: Math.max(...prices), compareAtMin: compares.length ? Math.min(...compares) : null };
}

// Variant for a selection of axis values (index-aligned); null while some axis is unchosen. A product with no axes
// has exactly one variant.
export function resolveVariant(options: ProductDetail["options"], variants: Variant[], chosen: (string | null)[]): Variant | null {
  if (options.length === 0) return variants[0] ?? null;
  if (chosen.length !== options.length || chosen.some((x) => x === null)) return null;
  return variants.find((x) => x.option_values.every((v, i) => v === chosen[i])) ?? null;
}

// A value on axis `axis` is selectable when some in-stock variant carries it together with the other axes' current picks.
export function valueAvailable(variants: Variant[], axis: number, value: string, chosen: (string | null)[]): boolean {
  return variants.some(
    (x) => x.stock !== "out" && x.option_values[axis] === value && x.option_values.every((v, i) => i === axis || chosen[i] === null || chosen[i] === v),
  );
}

// Initial selection: the cheapest in-stock variant (so the page opens on something buyable), else the first variant.
export function initialChoice(variants: Variant[]): string[] {
  const buyable = variants.filter((x) => x.stock !== "out").sort((a, b) => a.price_minor - b.price_minor);
  return (buyable[0] ?? variants[0])?.option_values ?? [];
}

// Free-shipping progress (cart page). `threshold` is the optional per-policy field of the checkout options; the quote
// applies it server-side (contracts/storefront-v2.md section C), this only words the hint. null = no hint to show.
export function freeShippingProgress(subtotal: number, threshold: number | null): { reached: boolean; remaining: number; ratio: number } | null {
  if (threshold === null || threshold <= 0) return null;
  return { reached: subtotal >= threshold, remaining: Math.max(0, threshold - subtotal), ratio: Math.min(1, subtotal / threshold) };
}
