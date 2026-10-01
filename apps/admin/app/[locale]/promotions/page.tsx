// Route /{locale}/promotions. BFF (browser, via Promotions.tsx): GET|POST /api/stores/{store}/promotions, POST .../promotions/{id} -> Go
// internal/httpapi/promotions.go (pricing:read / pricing:write; migration 0091, contracts/storefront-v2.md §F). Locale, query (`?store=<id>` only)
// and the authorized store list come from the shared loader of the customers pages; the codes themselves are read in the browser through the
// BFF so the response is fenced by the session boundary (useGuardedRead).
import { Promotions } from "@/components/Promotions";
import { loadPage } from "../customers/page-data";

export default async function PromotionsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return <Promotions locale={page.locale} stores={page.stores} store={page.store} initialError={page.initialError} renderKey={page.renderKey} />;
}
