import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import ProductPurchase from "../../../../components/ProductPurchase";
import { buyerDemoLabel } from "../../../../lib/demo-label";

// Server GET only validates the route. Buyer capability activation and private
// catalog loading happen through the existing browser coordinator, never SSR.
export default async function Page({
  params,
}: {
  params: Promise<{ locale: string; productID: string }>;
}) {
  const { locale, productID } = await params;
  if (
    !isLocale(locale) ||
    !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(productID)
  )
    notFound();
  return (
    <ProductPurchase
      locale={locale}
      productID={productID}
      demonstration={buyerDemoLabel(process.env)}
    />
  );
}
