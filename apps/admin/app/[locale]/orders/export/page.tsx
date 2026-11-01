// Purpose: authorized OrderExportHelp real route.
// Depends on: shared server session/store loader, WorkspaceFrame and OrderExportHelp.
// Used by: Next app router. Invariants: exact locale/store query only; no writes; original server-auth scope retained.
import { OrderExportHelp } from "@/components/orders/OrderExportHelp";
import { WorkspaceFrame } from "@/components/WorkspaceFrame";
import { loadPage } from "../../customers/page-data";
export default async function Page({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return (
    <WorkspaceFrame
      locale={page.locale}
      storeName={page.store?.name ?? ""}
      active="orders"
    >
      <OrderExportHelp
        key={page.renderKey}
        locale={page.locale}
        store={page.store}
        initialError={page.initialError}
      />
    </WorkspaceFrame>
  );
}
