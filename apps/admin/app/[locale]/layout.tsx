import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import "../globals.css";

export const metadata: Metadata = {
  title: "Commerce workspace",
  robots: { index: false, follow: false },
};
export const dynamic = "force-dynamic";

// React escapes raw comment strings; an inert first-child template preserves
// this source-to-render contract in production without affecting layout or SRs.
const directionContract = `<!--
THESIS: One auditable merchant workspace; never imply an internal setup publishes a store.
OWN-WORLD: Navy frame, teal actions, cool-white work surface, system sans, fine-line controls.
STORY: OIDC sign-in; merchant name, store/currency, warehouse; submit four fields once, then enter the SKU ledger.
FIRST VIEWPORT: Approved C uses a light identity/language bar, centered title and three-step rail, an 866px form, one current primary action; mobile preserves task order.
FORM: Operate, C three-step wizard, ranked structure 2, surface seed 57bb98dc. Existing ledger seed 3046f272 remains unchanged.
SETTINGS: Approved A horizontal four-step sequence and right read-only status column. The form owns the primary action; mobile places status below. PAYUNi credentials start blank, and saved configuration never claims provider qualification or live collection. Merchant-arranged manual delivery is a separate choice. See .impeccable/merchant-settings-brief.md and contracts/merchant-settings-wizard-v1.md.
ORDERS: Approved C inline table expansion, surface seed 549adef8. Preserve the full-width list around one selected row; its detail presents frozen items/totals, recipient snapshot and independent financial/work/fulfillment states. One filter and refresh; mobile stacks detail without a second PII tree. No fake global count, shipping/refund action or provider-ready claim. See .impeccable/merchant-orders-brief.md and contracts/merchant-orders-ui-v1.md.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
-->`;

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  return (
    <html lang={locale}>
      <body>
        <template
          data-commerce-design-contract="57bb98dc"
          dangerouslySetInnerHTML={{ __html: directionContract }}
        />
        {children}
      </body>
    </html>
  );
}
