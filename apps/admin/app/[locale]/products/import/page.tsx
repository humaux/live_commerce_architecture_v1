// Route /{locale}/products/import (server): the product CSV import / export wizard. Resolves locale, the page's whole query grammar (`store`
// only) and the authorized stores through the shared customers loader; the browser talks to the BFF
// /api/stores/{store}/tools/products/{export.csv,import/preview,import/commit} -> Go internal/httpapi/merchanttools.go (components/ProductImport.tsx).
import { ProductImport } from "@/components/ProductImport";
import { loadPage } from "../../customers/page-data";

export default async function ProductImportPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return <ProductImport locale={page.locale} stores={page.stores} store={page.store} />;
}
