// Route /{locale}/design. BFF (browser, via Design.tsx): GET|PUT /api/stores/{store}/design/draft, POST design/publish,
// design/rollback, design/preview-token, GET design/versions, GET|POST design/media[/{id}[/delete]] -> Go
// /v1/admin/stores/{id}/design/* (internal/httpapi/design.go, integration:read / integration:manage). Server side this
// page only resolves locale, `?store=` and the authorized stores (../customers/page-data.ts), like billing.
import { Design } from "@/components/Design";
import { loadPage } from "../customers/page-data";

export default async function DesignPage({
  params,
  searchParams,
}: {
  params: Promise<{ locale: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const page = await loadPage(params, searchParams, ["store"]);
  return <Design locale={page.locale} stores={page.stores} store={page.store} initialError={page.initialError} renderKey={page.renderKey} />;
}
