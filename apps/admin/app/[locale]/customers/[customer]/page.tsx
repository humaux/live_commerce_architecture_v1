// Purpose: resolve authorized customer detail and bounded search context for Back to results.
// Depends on: page-data authorized stores, canonical UUID and the existing customer search grammar.
// Used by: /[locale]/customers/[customer], CustomerDetail.
// Invariants: no unrecognized/repeated query values; customer facts and commands remain in the existing BFF.
// Route /{locale}/customers/{customer}. BFF (browser, via CustomerDetail.tsx): GET /api/stores/{store}/customers/{id},
// POST .../customers/{id}/consent-withdrawals | exports | erasure
// -> Go /v1/admin/stores/{id}/customers/{id}[/...] (internal/httpapi/customers.go; customers:read / customers:privacy).
// This server file only resolves locale, the customer id, query and authorized stores (page-data.ts).
import { notFound } from "next/navigation";
import { canonicalUUID } from "@/lib/orders-model";
import { CustomerDetail } from "@/components/CustomerDetail";
import { loadPage } from "../page-data";

export default async function CustomerPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string; customer: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { customer } = await params;
  if (!canonicalUUID.test(customer)) notFound();
  const page = await loadPage(params, searchParams, ["store", "q", "tag"]);
  const { q = "", tag = "" } = page.query;
  if ((tag && !canonicalUUID.test(tag)) || Array.from(q).length > 40 || /[\p{C}\p{Zl}\p{Zp}]/u.test(q)) notFound();
  return (
    <CustomerDetail locale={page.locale} stores={page.stores} store={page.store} customerID={customer} q={q} tag={tag}
      initialError={page.initialError} renderKey={page.renderKey} />
  );
}
