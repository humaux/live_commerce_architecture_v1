import { isLocale } from "@live-commerce/i18n";
import { matchRoute } from "./routes";
import { shellCopy } from "./shell-copy";

// Server-rendered public pages do not mount WorkspaceFrame, but share its title registry.
export async function routeMetadata(path: string, params: Promise<{ locale: string }>) {
  const { locale } = await params;
  const route = matchRoute(path);
  return { title: route && isLocale(locale) ? shellCopy[locale][route.labelKey] : "Commerce workspace" };
}
