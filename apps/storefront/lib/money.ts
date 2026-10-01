// Money display for the storefront shell (server and client components alike). Display only: Go decides every amount
// (I05: the quote, never the browser, owns money); this turns integer minor units into text. It never parses or sums.
// Same rule as the checkout flow's formatter: zh-TW shows the ISO code ("TWD 1,200") because "$" is ambiguous when an
// order is shared across markets; the number of minor digits comes from Intl (TWD = 2 in this system, JPY = 0).
import type { Locale } from "@live-commerce/i18n";

export function minorDigits(locale: Locale, currency: string): number {
  return new Intl.NumberFormat(locale, { style: "currency", currency }).resolvedOptions().maximumFractionDigits ?? 2;
}

export function formatMoney(locale: Locale, amount: number, currency: string): string {
  const digits = minorDigits(locale, currency);
  const formatter = new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    currencyDisplay: locale === "zh-TW" && currency === "TWD" ? "code" : "symbol",
    // A shop window reads "TWD 980", not "TWD 980.00"; cents appear only when there are cents. (Checkout screens keep the
    // always-two-digit form of components/CheckoutFlow.tsx.)
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
