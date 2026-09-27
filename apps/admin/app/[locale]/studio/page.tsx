import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import {
  authConfig, authenticatedStores, exactCookieHeader, isBase64URL32,
  safeError, SESSION_COOKIE,
} from "@/lib/auth";
import type { Store } from "@/lib/model";
import { studioCursor, studioUUID } from "@/lib/studio-model";
import type { StudioErrorCode } from "@/lib/studio-client";
import { Studio } from "@/components/Studio";

export default async function StudioPage({
  params, searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  if (Object.keys(query).some((key) => !["store", "scene", "cursor"].includes(key))) notFound();
  const single = (key: string) => {
    const value = query[key];
    if (value !== undefined && (typeof value !== "string" || value === "")) notFound();
    return value ?? "";
  };
  const requestedStore = single("store");
  const scene = single("scene");
  const cursor = single("cursor");
  if ((requestedStore && !studioUUID.test(requestedStore)) ||
    (scene && !studioUUID.test(scene)) || (cursor && !studioCursor.test(cursor))) notFound();

  let stores: Store[] = [];
  let store: Store | null = null;
  let error: StudioErrorCode | null = null;
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
        store = requestedStore
          ? (stores.find((item) => item.id === requestedStore) ?? null)
          : ([...stores].sort((a, b) => a.id.localeCompare(b.id))[0] ?? null);
        if (requestedStore && !store) notFound();
      }
    }
  }
  return <Studio locale={locale} stores={stores} store={store} scene={scene}
    cursor={cursor} initialError={error} />;
}
