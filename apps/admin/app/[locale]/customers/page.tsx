// Purpose: resolve the scoped customer list and closed search/cursor/tag query.
// Depends on: page-data authorized stores, canonical cursor/UUID grammar, React Suspense and AdminLoading.
// Used by: /[locale]/customers; private records are fetched by the browser BFF.
// Invariants: locale, query and store guards finish before the content Suspense boundary.
// Route /{locale}/customers. BFF (browser, via Customers.tsx): GET /api/stores/{store}/customers?limit&after&q
// -> Go GET /v1/admin/stores/{id}/customers (internal/httpapi/customers.go, customers:read).
// This server file only resolves locale, query and authorized stores (page-data.ts); it fetches no customer data.
import { Suspense } from "react";
import AdminLoading from "@/components/AdminLoading";
import { notFound } from "next/navigation";
import { canonicalCursor, canonicalUUID } from "@/lib/orders-model";
import { Customers } from "@/components/Customers";
import { loadPage } from "./page-data";

export default async function CustomersPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store", "q", "after", "tag"]);
  const { q = "", after = "", tag = "" } = page.query;
  // Same bounds as the BFF grammar (lib/customers-request.ts): q at most 40 characters, opaque cursor.
  if ((tag && !canonicalUUID.test(tag)) || Array.from(q).length > 40 || /[\p{C}\p{Zl}\p{Zp}]/u.test(q) || (after && !canonicalCursor.test(after))) notFound();
  return (
    <Suspense fallback={<AdminLoading />}>
      <Customers locale={page.locale} stores={page.stores} store={page.store} q={q} after={after} tag={tag}
        initialError={page.initialError} renderKey={page.renderKey} />
    </Suspense>
  );
}
