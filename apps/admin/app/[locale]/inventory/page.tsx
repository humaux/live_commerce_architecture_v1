// GET /[locale]/inventory (server): the stock ledger (products + inventory by warehouse) that used to be the admin landing. The landing is now
// the dashboard (unit merchant-tools, contract storefront-v2 G1); this route keeps every ledger query (?warehouse, q, status, cursor) exactly as
// before. Data comes from lib/backend.ts workspaceData -> GET /v1/admin/stores + warehouses + catalog-ledger (Go). Signed out: back to the
// landing, which owns the sign-in / onboarding states.
import { notFound, redirect } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { workspaceData } from "@/lib/backend";
import { Ledger } from "@/components/Ledger";

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
  const one = (key: string) => (typeof query[key] === "string" ? (query[key] as string) : "");
  const initial = await workspaceData(one("warehouse"), one("q"), one("status") || "all", one("cursor"));
  if (!initial.storeID && !initial.fixture) redirect(`/${locale}/`);
  return <Ledger locale={locale} initial={initial} />;
}
