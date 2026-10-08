// Purpose: authorize the inbox store before mounting a memory-only messages client.
// Depends on: existing session/store auth and locale registry; fixture-only calibration env.
// Used by: /[locale]/messages; no DM content is fetched or serialized during SSR.
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import {
  authConfig,
  authenticatedStores,
  exactCookieHeader,
  isBase64URL32,
  SESSION_COOKIE,
} from "@/lib/auth";
import type { Store } from "@/lib/model";
import { Inbox } from "@/components/Inbox";
import { inboxCalibration } from "@/src/features/messages/privacy";

/** Mount only a store from the authenticated list; inbox API authorization remains server-side. */
export default async function MessagesPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  if (Object.keys(query).some((key) => key !== "store")) notFound();
  const requested = query.store;
  if (
    requested !== undefined &&
    (typeof requested !== "string" ||
      !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(requested))
  )
    notFound();
  let store: Store | null = null;
  let error: string | null = null;
  const token = exactCookieHeader(
    (await headers()).get("cookie"),
    SESSION_COOKIE,
  );
  if (!authConfig || !token || !isBase64URL32(token)) error = "unauthorized";
  else {
    const listed = await authenticatedStores(token);
    if (!listed.stores)
      error =
        listed.response.status === 401
          ? "unauthorized"
          : listed.response.status === 403
            ? "forbidden"
            : "unavailable";
    else {
      store = requested
        ? (listed.stores.find((item) => item.id === requested) ?? null)
        : ([...listed.stores].sort((a, b) => a.id.localeCompare(b.id))[0] ??
          null);
      if (requested && !store) notFound();
    }
  }
  const calibration = inboxCalibration(process.env, authConfig?.publicOrigin);
  return (
    <Inbox
      key={`${store?.id ?? "none"}:${crypto.randomUUID()}`}
      locale={locale}
      store={store}
      initialError={error}
      calibration={calibration}
    />
  );
}
