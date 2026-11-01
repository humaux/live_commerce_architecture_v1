// Purpose: Route /{locale}/returns (server): resolves locale, the query (`store`, one RMA `state`, requests view and validated RMA identity) and the
//   authorized stores via the shared customers loader; it fetches no returns data. The browser reads through the BFF
//   /api/stores/{store}/returns* -> Go internal/httpapi/returns.go (components/ReturnsList.tsx; unit W3-U5,
//   contracts/returns-v1.md).
// Depends on: next/navigation, @/lib/returns-model (rmaStates), @/components/ReturnsList, ../customers/page-data.
// Used by: Next.js app router.
// Invariants: store authorization comes from the session; view/RMA selectors cannot mutate an order.
import { notFound } from "next/navigation";
import { rmaStates, type RmaState } from "@/lib/returns-model";
import { ReturnsList } from "@/components/ReturnsList";
import { loadPage } from "../customers/page-data";

export default async function ReturnsPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store", "state", "view", "rma"]);
  const state = page.query.state ?? "";
  if (state && !rmaStates.includes(state as RmaState)) notFound();
  const view = page.query.view ?? "";
  const rma = page.query.rma ?? "";
  if (view !== "" && view !== "requests") notFound();
  if (rma && !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(rma)) notFound();
  return (
    <ReturnsList
      locale={page.locale}
      stores={page.stores}
      store={page.store}
      state={state as RmaState | ""}
      initialError={page.initialError}
      renderKey={page.renderKey}
      view={view}
      rma={rma}
    />
  );
}
