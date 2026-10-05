"use client";

// The in-page language chooser of the stand-alone legal pages (/privacy and /data-deletion): ONE styled select for both, so the two pages do not
// show two different language controls. /privacy keeps its live buyer state and swaps language in place (onSelect); a static page navigates to the
// same page in the other language. No BFF/Go call. The shell footer's LocaleSwitch (links) remains the no-script path on every page.
import { localeNames, locales } from "@live-commerce/i18n";
import type { Locale } from "@live-commerce/i18n";

export default function LocaleSelect({ locale, label, path, disabled = false, onSelect }: { locale: Locale; label: string; path: string; disabled?: boolean; onSelect?: (next: Locale) => void }) {
  return (
    <label className="locale">
      <span className="sr-only">{label}</span>
      <select
        value={locale}
        disabled={disabled}
        onChange={(event) => {
          const next = event.target.value as Locale;
          if (onSelect) onSelect(next);
          else window.location.assign(`/${next}${path}`);
        }}
      >
        {locales.map((item) => (
          <option key={item} value={item}>
            {localeNames[item]}
          </option>
        ))}
      </select>
    </label>
  );
}
