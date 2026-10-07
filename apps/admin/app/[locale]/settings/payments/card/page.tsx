// Purpose: server page of /{locale}/settings/payments/card: resolves locale, query and the authorized stores (customers/page-data.ts) and renders CardPayments; it fetches no card data itself.
// Depends on: @/components/CardPayments, ../../../customers/page-data (loadPage).
// Used by: Next routing (route registry entry "card-payments", permission integration:read, nav:false; entry link in SettingsWizard step 1).
// Route /{locale}/settings/payments/card. BFF (browser, via CardPayments.tsx): GET /api/stores/{store}/payments/card,
// PUT .../payments/card -> Go /v1/admin/stores/{id}/payments/card (internal/httpapi/payment_card.go;
// integration:read / billing:manage, contract stripe-platform-account-v1).
import { CardPayments } from "@/components/CardPayments";
import { loadPage } from "../../../customers/page-data";

export default async function CardPaymentsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return (
    <CardPayments locale={page.locale} stores={page.stores} store={page.store}
      initialError={page.initialError} renderKey={page.renderKey} />
  );
}
