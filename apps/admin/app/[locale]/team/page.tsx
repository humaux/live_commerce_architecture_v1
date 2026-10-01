// Route /{locale}/team. BFF (browser, via Team.tsx): POST /api/team/{list,invite,revoke-invite,set-role,remove} -> Go
// /v1/identity/staff/* (internal/identityhttp/staff.go; owner role checked in SQL, migration 0089, contracts/storefront-v2.md §D).
// Locale, query (`?store=<id>` only) and the authorized store list come from the shared loader of the customers pages.
import { Team } from "@/components/Team";
import { loadPage } from "../customers/page-data";

export default async function TeamPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return <Team locale={page.locale} stores={page.stores} store={page.store} initialError={page.initialError} renderKey={page.renderKey} />;
}
