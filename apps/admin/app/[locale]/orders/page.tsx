// Purpose: Validate the scoped order query and render merchant order content.
// Depends on: auth store lookup, order models, React Suspense, AdminLoading and MerchantOrders.
// Used by: Next.js /[locale]/orders.
// Invariants: every locale, query and store notFound guard finishes before content may stream.
import { Suspense } from "react";
import AdminLoading from "@/components/AdminLoading";
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import {
  authConfig,
  authenticatedStores,
  exactCookieHeader,
  isBase64URL32,
  safeError,
  SESSION_COOKIE,
} from "@/lib/auth";
import type { Store } from "@/lib/model";
import { canonicalCursor, canonicalUUID, orderStates, type OrderFilter } from "@/lib/orders-model";
import type { OrderReadCode } from "@/lib/orders-client";
import { MerchantOrders } from "@/components/MerchantOrders";
import { emptyFilters, filterKeys, validOrderFilters, type OrderFilters } from "@/lib/orders-v2";

export default async function OrdersPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string; order?: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale, order: pathOrder } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  if (Object.keys(query).some((key) => key === "q" || !["store", "state", "order", "cursor", ...filterKeys].includes(key))) notFound();
  const single = (key: string) => {
    const value = query[key];
    if (value !== undefined && (typeof value !== "string" || value === "")) notFound();
    return value ?? "";
  };
  const requested = single("store");
  const state = single("state") || "active";
  const filters = { ...emptyFilters, ...Object.fromEntries(filterKeys.map((key) => [key, single(key) || emptyFilters[key]])) } as OrderFilters;
  if (!validOrderFilters(filters)) notFound();
  const order = pathOrder ?? single("order");
  if(pathOrder && query.order!==undefined) notFound();
  const cursor = single("cursor");
  if ((requested && !canonicalUUID.test(requested)) ||
    !orderStates.includes(state as OrderFilter) ||
    (order && !canonicalUUID.test(order)) ||
    (cursor && !canonicalCursor.test(cursor))) notFound();

  let stores: Store[] = [];
  let store: Store | null = null;
  let error: OrderReadCode | null = null;
  if (!authConfig) error = "signed-out";
  else {
    const token = exactCookieHeader((await headers()).get("cookie"), SESSION_COOKIE);
    if (!token || !isBase64URL32(token)) error = "signed-out";
    else {
      const listed = await authenticatedStores(token);
      if (!listed.stores) {
        const safe = await safeError(listed.response);
        error = safe.status === 401 ? "signed-out" : safe.status === 403 ? "forbidden" : "unavailable";
      } else {
        stores = listed.stores;
        store = requested
          ? (stores.find((item) => item.id === requested) ?? null)
          : ([...stores].sort((a, b) => a.id.localeCompare(b.id))[0] ?? null);
        if (requested && !store) notFound();
      }
    }
  }
  return (
    <Suspense fallback={<AdminLoading />}>
      <MerchantOrders locale={locale} store={store}
        state={state as OrderFilter} filters={filters} order={order} cursor={cursor} initialError={error}
        renderKey={crypto.randomUUID()} />
    </Suspense>
  );
}
