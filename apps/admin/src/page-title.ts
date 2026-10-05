// Purpose: Resolves localized admin titles from the route registry.
// Depends on: ./routes.ts, ./shell-copy.ts
// Used by: apps/admin/components/AdminPageHeader.tsx, apps/admin/components/WorkspaceFrame.tsx, tests/admin/attribution.test.ts, tests/admin/shell-registry.test.ts
import { matchRoute } from "./routes.ts";
import { shellCopy, type ShellLabel } from "./shell-copy.ts";

type Locale = keyof typeof shellCopy;
// /products/new is intentionally served by [product], not a second Next route.
/** Resolves a navigation label key from a route path. */
export function pageLabel(path: string): ShellLabel | undefined {
  const clean =
    path.replace(/^\/(zh-TW|zh-CN|en)(?=\/|$)/, "").replace(/\/$/, "") || "/";
  return clean === "/products/new" ? "productNew" : matchRoute(clean)?.labelKey;
}
/** Formats a localized title using the route navigation label. */
export function pageTitle(locale: Locale, path: string): string {
  const label = pageLabel(path);
  return label ? shellCopy[locale][label] : shellCopy[locale].navigation;
}
