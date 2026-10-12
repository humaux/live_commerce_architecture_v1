// Purpose: Resolve the live-settings scene picker and validated store/scene deep links using the signed session.
// Used by: W0 live-settings navigation and Studio/Console settings links.
// Owns the /{locale}/studio/settings?store=&scene= route: server-side locale/query
// validation and signed-session store resolution for W3-U2 Live Settings.
// Non-goals: no claims data on the server (the client reads M1–M7 through the BFF after
// hydration, so nothing claim-related is rendered into cached HTML), no store fallback
// for an unknown requested store (404), no fixture authority.
// Depends on: lib/auth.ts (authConfig, authenticatedStores, cookie parsing — the same
// path the Studio page uses) and components/LiveSettings.tsx.

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
import type { StudioErrorCode } from "@/lib/studio-client";

import { LiveSettings } from "@/components/LiveSettings";
import { liveSettingsSelection } from "@/src/features/live/workspace-model";

/** Validates contextual links; a global entry offers explicit scene selection after server-side store resolution. */
export default async function LiveSettingsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  const selection = liveSettingsSelection(query);
  if (!selection) notFound();
  const { store, scene } = selection;

  let selected: Store | null = null;
  let error: StudioErrorCode | null = null;
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
        selected = store
          ? (listed.stores.find((item) => item.id === store) ?? null)
          : ([...listed.stores].sort((a, b) => a.id.localeCompare(b.id))[0] ??
            null);
        if (store && !selected) notFound();
      }
    }
  }
  return (
    <LiveSettings
      key={`${locale}:${selected?.id ?? "signed-out"}:${scene}`}
      locale={locale}
      store={selected}
      scene={scene}
      initialError={error}
    />
  );
}
