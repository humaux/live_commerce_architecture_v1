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
