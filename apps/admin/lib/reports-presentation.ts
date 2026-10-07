// Purpose: report-only sorting and conversion display helpers; never creates monetary totals.
// Depends on: report model types, shared decimal formatter, Intl.Collator and native sort.
// Used by: ReportViews.tsx and focused pure tests.
// Invariants: I05 currencies/environments never combined or ranked against each other.
import type { ProductRow } from "./reports-model.ts";
import { decimal } from "../../../packages/format/src/index.ts";

/** Supported sortable product columns. */
export const productSorts = ["name", "units", "captured_minor", "refunded_minor", "net_minor", "offline_units", "offline_minor"] as const;
export type ProductSort = (typeof productSorts)[number];

/** Page `sort`/`dir` query: both absent is net_minor descending; a known field alone takes its natural direction (name
 * ascending, numbers descending); `dir` alone applies to net_minor. Anything else is null (the page answers 404). */
export function parseProductSort(sort: string | undefined, dir: string | undefined): { sort: ProductSort; ascending: boolean } | null {
  if (sort !== undefined && !(productSorts as readonly string[]).includes(sort)) return null;
  if (dir !== undefined && dir !== "asc" && dir !== "desc") return null;
  const key = (sort ?? "net_minor") as ProductSort;
  return { sort: key, ascending: dir === undefined ? key === "name" : dir === "asc" };
}

/** Sort a copy within currency/environment groups; no totals, conversion, or data mutation. */
export function sortedProducts(rows: ProductRow[], key: ProductSort, ascending: boolean, locale: string): ProductRow[] {
  const compare = new Intl.Collator(locale).compare;
  return [...rows].sort((a, b) => {
    const group = a.currency.localeCompare(b.currency) || a.environment.localeCompare(b.environment);
    if (group) return group;
    const order = key === "name" ? compare(a.name, b.name) : (a[key] as number) - (b[key] as number);
    return (ascending ? order : -order) || a.sku_id.localeCompare(b.sku_id);
  });
}

/** Localized conversion ratio, or the supplied honest empty denominator label. */
export function conversion(locale: string, numerator: number, denominator: number, empty: string): string {
  if (denominator === 0) return empty;
  const percent = numerator / denominator * 100;
  return `${decimal(locale, percent, Number.isInteger(percent) ? 0 : 1)}%`;
}
