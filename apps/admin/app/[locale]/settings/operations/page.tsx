// Purpose: resolve the authenticated store for W6-U2's operation ledger page.
// Depends on: Next headers/navigation; auth store listing; OperationsLedger; W6-05B.
// Used by: /[locale]/settings/operations, settings link and browser gate.
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
import { operationID } from "@/lib/operations-model";
import { OperationsLedger } from "@/components/OperationsLedger";
/** Server-only session/store resolution; never authorize from a query's store ID. */
export default async function OperationsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  if (Object.keys(query).some((k) => !["store", "operation"].includes(k)))
    notFound();
  const single = (k: string) => {
    const v = query[k];
    if (v !== undefined && (typeof v !== "string" || !operationID.test(v)))
      notFound();
    return v as string | undefined;
  };
  const requested = single("store") ?? "";
  const operation = single("operation") ?? "";
  let stores: Store[] = [];
  let store: Store | null = null;
  let error: "signed-out" | "forbidden" | "unavailable" | null = null;
  if (!authConfig) error = "signed-out";
  else {
    const token = exactCookieHeader(
      (await headers()).get("cookie"),
      SESSION_COOKIE,
    );
    if (!token || !isBase64URL32(token)) error = "signed-out";
    else {
      const listed = await authenticatedStores(token);
      if (!listed.stores) {
        const safe = await safeError(listed.response);
        error =
          safe.status === 401
            ? "signed-out"
            : safe.status === 403
              ? "forbidden"
              : "unavailable";
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
    <OperationsLedger
      key={`${locale}:${store?.id ?? ""}`}
      locale={locale}
      stores={stores}
      store={store}
      operation={operation}
      initialError={error}
    />
  );
}
