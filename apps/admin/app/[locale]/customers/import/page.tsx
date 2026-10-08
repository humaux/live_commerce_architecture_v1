// Purpose: resolve an authorized store for the private customer/order import wizard.
// Depends on: existing customers/page-data loader and ImportWizard; only the store query is admitted.
// Used by: /[locale]/customers/import; files and preview/commit calls remain client-scoped through exact BFF routes.
import { loadPage } from "../page-data";
import { ImportWizard } from "@/components/ImportWizard";

/** Render the explicit import form after server session/store resolution; no CSV is read on the server. */
export default async function ImportPage({ params, searchParams }: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return <ImportWizard locale={page.locale} stores={page.stores} store={page.store} initialError={page.initialError} renderKey={page.renderKey} />;
}
