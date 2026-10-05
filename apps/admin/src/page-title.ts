import { matchRoute } from "./routes.ts";
import { shellCopy, type ShellLabel } from "./shell-copy.ts";

type Locale = keyof typeof shellCopy;
// /products/new is intentionally served by [product], not a second Next route.
export function pageLabel(path: string): ShellLabel | undefined {
  const clean =
    path.replace(/^\/(zh-TW|zh-CN|en)(?=\/|$)/, "").replace(/\/$/, "") || "/";
  return clean === "/products/new" ? "productNew" : matchRoute(clean)?.labelKey;
}
export function pageTitle(locale: Locale, path: string): string {
  const label = pageLabel(path);
  return label ? shellCopy[locale][label] : shellCopy[locale].navigation;
}
