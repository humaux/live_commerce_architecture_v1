// Route /{locale}/settings/settlements (billing:manage). BFF (browser, via Settlements.tsx):
// GET /api/stores/{store}/settlements[/{statement_id}] -> Go settlement reads (internal/payments/settlement,
// W4-S2; contract stripe-platform-account-v1). Read-only; no query beyond ?store.
import { Settlements } from "@/components/Settlements";
import { loadPage } from "../../customers/page-data";

export default async function SettlementsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return (
    <Settlements locale={page.locale} stores={page.stores} store={page.store}
      initialError={page.initialError} renderKey={page.renderKey} />
  );
}
