// Route /{locale}/products (server): resolves locale, the page's whole query grammar and the authorized stores via the
// shared customers loader (session cookie -> Go GET /v1/admin/stores); it fetches no catalog data. The browser reads
// through the BFF: GET /api/stores/{store}/catalog-products -> Go internal/httpapi/collections.go (components/ProductList.tsx).
import { notFound } from "next/navigation";
import { canonicalCursor } from "@/lib/orders-model";
import { ProductList } from "@/components/ProductList";
import { productStatuses } from "@/lib/catalog-v2-model";
import { loadPage } from "../customers/page-data";

export default async function ProductsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store", "q", "status", "after"]);
  const { q = "", status = "all", after = "" } = page.query;
  // Same bounds as the Go grammar: q at most 120 characters without control characters, a known status, an opaque cursor.
  if (Array.from(q).length > 120 || /[\p{C}\p{Zl}\p{Zp}]/u.test(q) || !(productStatuses as readonly string[]).includes(status) ||
    (after && !canonicalCursor.test(after))) notFound();
  return (
    <ProductList locale={page.locale} stores={page.stores} store={page.store} q={q} status={status} after={after}
      initialError={page.initialError} renderKey={page.renderKey} />
  );
}
