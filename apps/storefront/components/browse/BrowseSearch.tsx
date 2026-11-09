// Purpose: the signed browse search field (Figma 4:9192 home, 6:19933 collection): visible label over a full-width input.
// Depends on: lib/browse-copy.ts labels; GET /{locale}/search (no BFF/Go call of its own).
// Used by: components/HomeSections.tsx and app/[locale]/collections/[slug]/page.tsx.
// Invariants: plain GET form, works without JavaScript (one search field = implicit submission on Enter); the preview token is
//   carried only while a draft is shown; input font-size >= 16px so iOS does not zoom.
import type { Locale } from "@live-commerce/i18n";
import { browseCopy } from "../../lib/browse-copy";
import styles from "./browse.module.css";

export default function BrowseSearch({ locale, preview }: { locale: Locale; preview: string | null }) {
  const t = browseCopy[locale].browse;
  return (
    <form className={styles.search} role="search" action={`/${locale}/search`} method="get">
      <label htmlFor="sf-browse-q">{t.searchLabel}</label>
      <input id="sf-browse-q" name="q" type="search" placeholder={t.searchHint} maxLength={60} autoComplete="off" enterKeyHint="search" />
      {preview && <input type="hidden" name="preview" value={preview} />}
    </form>
  );
}
