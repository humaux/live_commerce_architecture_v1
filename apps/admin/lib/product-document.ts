// Product editor draft and Cartesian matrix. No I/O; the Go document command is authoritative.
import {
  cleanAxes,
  toMinor,
  variantMatrix,
  type OptionAxis,
} from "./catalog-v2-model.ts";

export type DraftRow = {
  id?: string;
  values: string[];
  price: string;
  compare: string;
  quantity: string;
  tracked: boolean;
  max: string;
  code: string;
  keyword: string;
  active: boolean;
};
export type ProductDraft = {
  name: string;
  description: string;
  slug: string;
  seo_title: string;
  seo_description: string;
  axes: OptionAxis[];
  rows: DraftRow[];
  collections: string[];
  weight: string;
  length: string;
  width: string;
  height: string;
  warehouse: string;
};
export type BulkField = "price" | "compare" | "quantity" | "code" | "keyword";
export const newRow = (values: string[]): DraftRow => ({
  values,
  price: "",
  compare: "",
  quantity: "",
  tracked: true,
  max: "",
  code: "",
  keyword: "",
  active: true,
});
export const rowKey = (row: Pick<DraftRow, "values">) =>
  JSON.stringify(row.values);
export function syncMatrix(
  axes: OptionAxis[],
  previous: DraftRow[],
): DraftRow[] {
  const cleaned = cleanAxes(axes);
  if (cleaned.error) throw new Error(cleaned.error);
  if (cleaned.axes.reduce((n, a) => n * a.values.length, 1) > 100)
    throw new Error("limit");
  const byValues = new Map(previous.map((row) => [rowKey(row), row]));
  return variantMatrix(cleaned.axes).map(
    (values) => byValues.get(JSON.stringify(values)) ?? newRow(values),
  );
}
export function applyBulk(
  rows: DraftRow[],
  field: BulkField,
  value: string,
  scope: "all" | "empty",
  operation: "set" | "add" | "subtract",
): { rows: DraftRow[]; skipped: number } {
  let skipped = 0;
  const next = rows.map((row, i) => {
    if (scope === "empty" && row[field].trim()) return row;
    if (field === "quantity" && !row.tracked) {
      skipped++;
      return row;
    }
    let result = field === "code" ? `${value}${i + 1}` : value;
    if (field === "quantity") {
      const amount = /^\d+$/.test(value) ? Number(value) : NaN;
      const current = /^\d+$/.test(row.quantity) ? Number(row.quantity) : NaN;
      const n =
        operation === "set"
          ? amount
          : operation === "add"
            ? current + amount
            : current - amount;
      if (!Number.isSafeInteger(n) || n < 0 || n > 1_000_000_000) {
        skipped++;
        return row;
      }
      result = String(n);
    }
    return { ...row, [field]: result };
  });
  return { rows: next, skipped };
}
const count = (value: string, maximum = 1_000_000_000) => {
  if (
    !/^\d+$/.test(value) ||
    !Number.isSafeInteger(Number(value)) ||
    Number(value) > maximum
  )
    throw new Error("invalid_stock");
  return Number(value);
};
export function createDocument(draft: ProductDraft, currency: string) {
  if (!draft.name.trim() || Array.from(draft.name.trim()).length > 120)
    throw new Error("invalid_name");
  const cleaned = cleanAxes(draft.axes);
  if (cleaned.error || draft.rows.length < 1 || draft.rows.length > 100)
    throw new Error("invalid_options");
  const codes = new Set<string>(),
    keywords = new Set<string>();
  const skus = draft.rows.map((row) => {
    const price = toMinor(row.price, currency),
      compare = row.compare.trim() ? toMinor(row.compare, currency) : null;
    if (
      price === null ||
      (row.compare.trim() && (compare === null || compare <= price))
    )
      throw new Error("invalid_price");
    const keyword = row.keyword.trim().toUpperCase(),
      code = row.code.trim();
    if (keyword && (!/^[A-Z0-9]{1,16}$/.test(keyword) || keywords.has(keyword)))
      throw new Error("keyword_taken");
    if (code && (!/^[A-Za-z0-9_.-]{1,64}$/.test(code) || codes.has(code)))
      throw new Error("invalid_code");
    if (keyword) keywords.add(keyword);
    if (code) codes.add(code);
    const maximum = row.tracked ? null : count(row.max, 999);
    if (maximum === 0) throw new Error("invalid_stock");
    return {
      ...(row.id ? { id: row.id } : {}),
      option_values: row.values,
      code,
      keyword,
      active: row.active,
      price_minor: price,
      compare_at_minor: compare,
      stock: row.tracked
        ? { mode: "tracked", opening_qty: count(row.quantity) }
        : { mode: "untracked", max_per_order: maximum! },
    };
  });
  const mm = (s: string) => {
    if (!s.trim()) return 0;
    if (!/^\d+(\.\d)?$/.test(s) || Number(s) > 100000)
      throw new Error("invalid_dimensions");
    return Math.round(Number(s) * 10);
  };
  return {
    expected_version: 0,
    name: draft.name.trim(),
    description: draft.description,
    slug: draft.slug,
    status: "draft",
    seo_title: draft.seo_title,
    seo_description: draft.seo_description,
    options: cleaned.axes,
    skus,
    collection_ids: draft.collections,
    ...(draft.warehouse ? { warehouse_id: draft.warehouse } : {}),
    weight_grams: draft.weight.trim() ? count(draft.weight) : 0,
    length_mm: mm(draft.length),
    width_mm: mm(draft.width),
    height_mm: mm(draft.height),
  };
}
// 39bb8ec full-replace edit would clear keywords not returned by GET, overwrite per-SKU dimensions,
// and treats target_qty=0 as no adjustment. No client-side guess or legacy second writer is safe.
// Remove this guard only when the backend edit/read contract is corrected and PE14 proves preservation.
export const safeEditProblem = () => "document_readback_incomplete" as const;

export type BulkResult = { id: string; status?: string; error?: string };
export function parseBulk(value: unknown): BulkResult[] {
  if (!Array.isArray(value) || !value.length || value.length > 100)
    throw new Error("invalid");
  const seen = new Set<string>();
  return value.map((item) => {
    if (!item || typeof item !== "object") throw new Error("invalid");
    const r = item as Record<string, unknown>;
    if (
      typeof r.id !== "string" ||
      !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(
        r.id,
      ) ||
      seen.has(r.id) ||
      (r.status === undefined) === (r.error === undefined) ||
      (r.status !== undefined &&
        !["draft", "active", "archived"].includes(String(r.status))) ||
      (r.error !== undefined &&
        !["not_found", "live_window_open"].includes(String(r.error)))
    )
      throw new Error("invalid");
    seen.add(r.id);
    return {
      id: r.id,
      ...(r.status ? { status: String(r.status) } : {}),
      ...(r.error ? { error: String(r.error) } : {}),
    };
  });
}
