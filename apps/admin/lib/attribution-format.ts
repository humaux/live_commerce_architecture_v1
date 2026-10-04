// Display only: the frozen Go/PG report supplies the ratio and amount facts.
import type { Locale } from "@live-commerce/i18n";

export function formatROAS(
  locale: Locale,
  value: number | null,
  unknown: string,
): string {
  if (value === null || !Number.isFinite(value)) return unknown;
  return `${new Intl.NumberFormat(locale, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)}×`;
}
