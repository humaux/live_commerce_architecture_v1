// GET /[locale]/ (server). Signed in: the merchant dashboard (components/Dashboard.tsx, BFF /api/stores/{store}/tools/dashboard -> Go
// GET /v1/admin/stores/{id}/dashboard; contract storefront-v2 G1). The stock ledger moved to /[locale]/inventory (nav "Inventory").
// Signed out state renders PasswordAuth when COMMERCE_PASSWORD_LOGIN_ENABLED=1, else the OIDC entry; the sign-in state still comes from
// lib/backend.ts workspaceData → GET /v1/admin/stores (Go), and the stores of the dashboard from the shared loader of the customers pages.
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { onboardingPolicy, workspaceData } from "@/lib/backend";
import { authConfig } from "@/lib/auth";
import { Dashboard } from "@/components/Dashboard";
import { Entry } from "@/components/Entry";
import { loadPage } from "./customers/page-data";

export default async function Page({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  const one = (key: string) =>
    typeof query[key] === "string" ? (query[key] as string) : "";
  // Only the sign-in state is needed here: no warehouse, search or cursor (those belong to /inventory).
  const initial = await workspaceData("", "", "all", "", true);
  if (initial.storeID || initial.fixture) {
    // The dashboard's whole query grammar is `store`; any other key (for example a stale ledger link) is ignored, never a 404.
    const page = await loadPage(params, Promise.resolve(one("store") ? { store: one("store") } : {}), ["store"]);
    const fixtureStore = initial.fixture ? { id: initial.storeID, name: initial.storeName, currency: "" } : null;
    return (
      <Dashboard
        locale={locale}
        stores={page.stores.length ? page.stores : fixtureStore ? [fixtureStore] : []}
        store={page.store ?? fixtureStore}
        initialError={fixtureStore ? null : page.initialError}
        renderKey={page.renderKey}
      />
    );
  }
  const policy = onboardingPolicy();
  const status = !authConfig
    ? "disabled"
    : initial.error?.code === "unauthorized"
      ? "signed-out"
      : initial.error
        ? "unavailable"
        : "onboarding";
  return (
    <Entry
      locale={locale}
      status={status}
      authResult={one("auth")}
      onboardingEnabled={policy.enabled}
      currencies={policy.currencies}
      passwordMode={status === "signed-out" && authConfig?.passwordLogin ? "signin" : undefined}
      oidc={!!authConfig?.issuer}
    />
  );
}
