import { headers } from "next/headers";
import { notFound } from "next/navigation";
import {
  companyConfig,
  platformLocales,
  requestHostname,
} from "../../../../lib/company";
import "../../../../components/platform/platform.css";

export const dynamic = "force-dynamic";
const direction = `<!--
THESIS: From a Facebook comment to a recorded order, not a generic dashboard sales pitch.
OWN-WORLD: White canvas, slate type, orange actions, fine borders, 8px corners, system sans.
STORY: Understand the illustrated workflow, inspect operator policies, then open a merchant account.
FIRST VIEWPORT: Brand and login above a large left headline and right benefits; three illustrated workflow panels below with an explicit setup disclaimer. Operator facts begin below the first viewport.
FORM: Approved A workflow-led presentation, decision 880ef0da (selection key, not a roll seed). Comp-led extension; owner chose A on 2026-10-04; no universal connection claims.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
-->`;

export default async function PublicLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  const h = await headers();
  if (
    !platformLocales.some((v) => v === locale) ||
    requestHostname(h.get("host")) !== companyConfig().platformHost
  )
    notFound();
  return (
    <html lang={locale}>
      <body className="ps-body">
        <template
          data-commerce-design-contract="880ef0da"
          dangerouslySetInnerHTML={{ __html: direction }}
        />
        {children}
      </body>
    </html>
  );
}
