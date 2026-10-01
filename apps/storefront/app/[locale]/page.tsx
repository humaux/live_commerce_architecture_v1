import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import ShopHome from "../../components/ShopHome";
import { buyerDemoLabel } from "../../lib/demo-label";

// GET /{locale}: the shop home. Server GET only validates the locale; like the product page, buyer capability
// activation and the private catalog read happen in the browser coordinator, never SSR (components/ShopHome.tsx:
// BFF GET /api/buyer/catalog -> Go /v1/buyer/catalog). Replaces the 404 a buyer got at the bare shop address.
export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  return <ShopHome locale={locale} demonstration={buyerDemoLabel(process.env)} />;
}
