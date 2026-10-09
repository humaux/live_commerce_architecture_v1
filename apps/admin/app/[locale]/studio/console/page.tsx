// Purpose: Authenticated LC-U1 console page using the same server-selected store boundary as Studio.
// Depends on: Next headers/navigation, admin auth, i18n, LiveWorkspace; no provider access.
// Used by: App Router /[locale]/studio/console and the W0 route registry.
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { authConfig, authenticatedStores, exactCookieHeader, isBase64URL32, safeError, SESSION_COOKIE } from "@/lib/auth";
import { studioUUID } from "@/lib/studio-model";
import type { StudioErrorCode } from "@/lib/studio-client";
import type { Store } from "@/lib/model";
import { LiveWorkspace } from "@/components/LiveWorkspace";

/** Resolves store membership server-side before mounting the private console. */
export default async function LiveConsolePage({ params, searchParams }: {
  params: Promise<{ locale: string }>; searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  if (Object.keys(query).some((key) => !["store", "scene"].includes(key))) notFound();
  const single = (name: string) => { const v = query[name]; if (v !== undefined && (typeof v !== "string" || !studioUUID.test(v))) notFound(); return v ?? ""; };
  const requested = single("store"), scene = single("scene");
  let store: Store | null = null, error: StudioErrorCode | null = null;
  const token = exactCookieHeader((await headers()).get("cookie"), SESSION_COOKIE);
  if (!authConfig || !token || !isBase64URL32(token)) error = "signed-out";
  else {
    const listed = await authenticatedStores(token);
    if (!listed.stores) { const failure = await safeError(listed.response); error = failure.status === 401 ? "signed-out" : failure.status === 403 ? "forbidden" : "unavailable"; }
    else { store = requested ? listed.stores.find((s) => s.id === requested) ?? null : [...listed.stores].sort((a, b) => a.id.localeCompare(b.id))[0] ?? null; if (requested && !store) notFound(); }
  }
  const commentCalibration=process.env.LC_CONSOLE_CALIBRATION==="retain-on-reset" && process.env.LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE==="1" && process.env.COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS==="1" && /^http:\/\/127\.0\.0\.1:[1-9][0-9]{0,4}$/.test(authConfig?.publicOrigin??"");
  return <LiveWorkspace key={store?.id ?? "signed-out"} locale={locale} store={store} scene={scene} initialError={error} commentCalibration={commentCalibration} />;
}
