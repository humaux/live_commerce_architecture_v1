// Route /{locale}/collections (server): resolves locale, query (only `store`) and the authorized stores via the shared
// customers loader; it fetches no collection data. The browser reads and writes through the BFF
// /api/stores/{store}/collections* -> Go internal/httpapi/collections.go (components/CollectionManager.tsx).
import { CollectionManager } from "@/components/CollectionManager";
import { loadPage } from "../customers/page-data";

export default async function CollectionsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return <CollectionManager locale={page.locale} stores={page.stores} store={page.store} initialError={page.initialError} renderKey={page.renderKey} />;
}
