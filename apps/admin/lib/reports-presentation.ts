// Purpose: report-only sorting and conversion display helpers; never creates monetary totals.
// Depends on: report model types, shared decimal formatter, Intl.Collator and native sort.
// Used by: ReportViews.tsx and focused pure tests.
// Invariants: I05 currencies/environments never combined or ranked against each other.
import type { ProductRow } from "./reports-model.ts";
import { decimal } from "../../../packages/format/src/index.ts";

/** Supported sortable product columns. */
export type ProductSort = "name" | "units" | "captured_minor" | "refunded_minor" | "net_minor" | "offline_units" | "offline_minor";

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
