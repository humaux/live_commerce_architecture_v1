// Purpose: report page resolves locale, authorized store and calendar range only.
// Depends on: customers/page-data loadPage, report query grammar, finance-day helper, Reports.
// Used by: /[locale]/finance/reports; navigation registry is owned by the integrator.
// Invariants: I01 trusted store, I05 report data is read through authenticated BFF, I15 private reads.
import { notFound } from "next/navigation";
import { loadPage } from "../../customers/page-data";
import { financeDay } from "@/lib/customers-model";
import { reportFromQuery, validReportsQuery } from "@/lib/reports-request";
import { Reports } from "@/components/Reports";

/** Render report controls after trusted store resolution; no report facts are fetched on the server. */
export default async function ReportsPage({ params, searchParams }: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store", "from", "to", "report"]);
  const now = new Date();
  const from = page.query.from ?? financeDay(now, -29), to = page.query.to ?? financeDay(now);
  const tab = reportFromQuery(page.query.report);
  if (!tab || !!page.query.from !== !!page.query.to || !validReportsQuery("products", `/?from=${from}&to=${to}`)) notFound();
  return <Reports locale={page.locale} stores={page.stores} store={page.store} initialError={page.initialError}
    renderKey={page.renderKey} initialFrom={from} initialTo={to} initialTab={tab} />;
}
