// The ONE money formatter of the storefront (shop window, cart, checkout bar and quotation, orders, history, claim page; server and
// client components alike; apps/storefront/tests/money.test.mjs fails if a component builds its own again). Display only: Go decides
// every amount (I05: the quote, never the browser, owns money); this turns integer minor units into text. It never parses or sums.
// zh-TW shows the ISO code ("TWD 1,200") because "$" is ambiguous when an order is shared across markets; the number of minor digits
// comes from Intl (TWD = 2 in this system, JPY = 0) and whole amounts drop them, so a line, a subtotal and a total read alike.
import type { Locale } from "@live-commerce/i18n";

export function minorDigits(locale: Locale, currency: string): number {
  return new Intl.NumberFormat(locale, { style: "currency", currency }).resolvedOptions().maximumFractionDigits ?? 2;
}

export function formatMoney(locale: Locale, amount: number, currency: string): string {
  const digits = minorDigits(locale, currency);
  if (currency === "TWD") return `NT$${new Intl.NumberFormat(locale, {
    minimumFractionDigits: amount % 100 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(amount / 100)}`;
  const formatter = new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    currencyDisplay: locale === "zh-TW" && currency === "TWD" ? "code" : "symbol",
    // "TWD 980", never "TWD 980.00", on every screen; cents appear only when there are cents (then always two digits).
    minimumFractionDigits: amount % 10 ** digits === 0 ? 0 : digits,
  });
  return formatter.format(amount / 10 ** digits);
}

// "1200" or "12.5" typed in a price filter -> minor units, or null when empty/invalid/too large (the Go list route
// accepts at most 13 digits). Filters are a browsing aid; the server re-validates the number.
export function majorToMinor(locale: Locale, currency: string, text: string): number | null {
  const trimmed = text.trim();
  if (!/^\d{1,10}(?:\.\d{1,4})?$/.test(trimmed)) return null;
  const minor = Math.round(Number(trimmed) * 10 ** minorDigits(locale, currency));
  return Number.isSafeInteger(minor) && minor <= 1_000_000_000_000 ? minor : null;
}

// Inverse for refilling the filter inputs from the URL.
export function minorToMajor(locale: Locale, currency: string, minor: number): string {
  const digits = minorDigits(locale, currency);
  return String(minor / 10 ** digits);
}
