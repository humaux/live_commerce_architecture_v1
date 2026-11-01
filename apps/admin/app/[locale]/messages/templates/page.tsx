// Purpose: authorized MessageTemplateList real route.
// Depends on: shared server session/store loader, WorkspaceFrame and MessageTemplateList.
// Used by: Next app router. Invariants: exact locale/store query only; no writes; original server-auth scope retained.
import { MessageTemplateList } from "@/components/inbox/MessageTemplateList";
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
      active="messages"
    >
      <MessageTemplateList
        key={page.renderKey}
        locale={page.locale}
        store={page.store}
        initialError={page.initialError}
        renderKey={page.renderKey}
      />
    </WorkspaceFrame>
  );
}
