// Route /{locale}/products/{id|new} (server): resolves locale, the product segment (a canonical UUID or the literal
// `new`), the query (only `store`) and the authorized stores via the shared customers loader; it fetches no product
// data. The browser reads and writes through the BFF /api/stores/{store}/products* -> Go internal/httpapi (components/ProductEditor.tsx).
import { notFound } from "next/navigation";
import { canonicalUUID } from "@/lib/orders-model";
import { ProductEditor } from "@/components/ProductEditor";
import { loadPage } from "../../customers/page-data";

export default async function ProductPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string; product: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { product } = await params;
  if (product !== "new" && !canonicalUUID.test(product)) notFound();
  const page = await loadPage(params, searchParams, ["store"]);
  return <ProductEditor locale={page.locale} stores={page.stores} store={page.store} productID={product} initialError={page.initialError} renderKey={page.renderKey} />;
}
