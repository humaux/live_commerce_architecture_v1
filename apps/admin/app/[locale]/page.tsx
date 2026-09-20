import { notFound } from "next/navigation";
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
  const one = (key: string) =>
    typeof query[key] === "string" ? (query[key] as string) : "";
  const initial = await workspaceData(
    one("warehouse"),
    one("q"),
    one("status") || "all",
    one("cursor"),
  );
  return <Ledger locale={locale} initial={initial} />;
}
