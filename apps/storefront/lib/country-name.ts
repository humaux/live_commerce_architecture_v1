// Display name of an ISO 3166-1 alpha-2 country code in the buyer's language ("TW" -> 台灣 / 台湾 / Taiwan). Display only: the stored code is
// unchanged. Falls back to the code itself when the runtime has no region names, so an address never loses its country.
import type { Locale } from "@live-commerce/i18n";

export function countryName(locale: Locale, code: string): string {
  try {
    return new Intl.DisplayNames(locale, { type: "region" }).of(code) ?? code;
  } catch {
    return code;
  }
}
