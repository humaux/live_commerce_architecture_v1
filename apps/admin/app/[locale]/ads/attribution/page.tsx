// Resolves server-authenticated store only. Browser GET /api/stores/{store}/ads/attribution -> Go /v1/admin/stores/{store}/ads/attribution.
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { instantToTaipei } from "@live-commerce/format";
import {
  authConfig,
  authenticatedStores,
  exactCookieHeader,
  isBase64URL32,
  safeError,
  SESSION_COOKIE,
} from "@/lib/auth";
import { canonicalUUID } from "@/lib/ads-model";
import { validAdsQuery } from "@/lib/ads-request";
import type { Store } from "@/lib/model";
import type { AttributionError } from "@/lib/attribution-client";
import { Attribution } from "@/components/Attribution";

export default async function AttributionPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  if (
    Object.keys(query).some(
      (k) => !["store", "from", "to", "draft", "session"].includes(k),
    )
  )
    notFound();
  const single = (key: string) => {
    const value = query[key];
    if (value !== undefined && (typeof value !== "string" || !value))
      notFound();
    return value ?? "";
  };
  const requested = single("store"),
    draftID = single("draft"),
    sessionID = single("session");
  if (
    [requested, draftID, sessionID].some((id) => id && !canonicalUUID.test(id))
  )
    notFound();
  const today = instantToTaipei(new Date().toISOString()).slice(0, 10);
  const first = new Date(`${today}T00:00:00Z`);
  first.setUTCDate(first.getUTCDate() - 6);
  const suppliedFrom = single("from"),
    suppliedTo = single("to");
  if (!!suppliedFrom !== !!suppliedTo) notFound();
  const from = suppliedFrom || first.toISOString().slice(0, 10),
    to = suppliedTo || today;
  if (
    !validAdsQuery(
      `https://local.invalid/?from=${from}&to=${to}`,
      "ads/attribution",
    )
  )
    notFound();
  let store: Store | null = null,
    error: AttributionError | null = null;
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
        store = requested
          ? (listed.stores.find((s) => s.id === requested) ?? null)
          : ([...listed.stores].sort((a, b) => a.id.localeCompare(b.id))[0] ??
            null);
        if (requested && !store) notFound();
      }
    }
  }
  return (
    <Attribution
      locale={locale}
      store={store}
      from={from}
      to={to}
      draftID={draftID}
      sessionID={sessionID}
      initialError={error}
    />
  );
}
