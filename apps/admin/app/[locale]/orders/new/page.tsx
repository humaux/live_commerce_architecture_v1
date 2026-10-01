// Route /{locale}/orders/new (server): the admin Create Order form. Resolves locale, the page's whole query grammar (`store` only) and the
// authorized stores through the shared customers loader (session cookie -> Go GET /v1/admin/stores); it fetches no order data. The browser
// reads/writes through the BFF: /api/stores/{store}/tools/orders/manual[/options] -> Go internal/httpapi/merchanttools.go (components/ManualOrder.tsx).
import { ManualOrder } from "@/components/ManualOrder";
import { loadPage } from "../../customers/page-data";

export default async function NewOrderPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return <ManualOrder locale={page.locale} stores={page.stores} store={page.store} initialError={page.initialError} renderKey={page.renderKey} />;
}
