// Purpose: Give the global live-settings navigation a scene selection before opening contextual controls.
// Depends on: server-selected Store, existing Studio read transport, W0 shell and live workspace presentation/hooks.
// Used by: /[locale]/studio/claims when no scene is selected; deep links still render StudioClaims.
"use client";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { readStudioPage, type StudioErrorCode } from "@/lib/studio-client";
import { useLiveRead } from "@/src/features/live/use-live-workspace";
import { workspaceCopy } from "@/src/features/live/workspace-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import "@/src/features/live/workspace.css";

/** Reads the current store's sessions; only an explicit selection navigates to its settings. */
export function LiveSettingsEntry({ locale, store, initialError }: { locale: Locale; store: Store | null; initialError: StudioErrorCode | null }) {
  const router = useRouter(), c = workspaceCopy[locale], storeID = store?.id ?? "";
  const view = useLiveRead(`${storeID}:settings-sessions`, !!storeID && !initialError, (signal) => readStudioPage(storeID, "", signal));
  const error = initialError || view.error;
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? ""} active="live">
    <section className="live-workspace" data-testid="live-settings-entry">
      <AdminPageHeader locale={locale} description={c.settingsChoose} />
      {error ? <p role="alert">{error === "signed-out" ? c.signedOut : error === "forbidden" ? c.forbidden : c.unavailable}</p> :
        <div className="live-workspace-toolbar"><label htmlFor="live-settings-picker">{c.choose}
          <select id="live-settings-picker" value="" disabled={!view.data?.items.length} onChange={(event) => {
            if (view.data?.items.some((item) => item.session_id === event.target.value))
              router.push(`/${locale}/studio/claims?store=${storeID}&scene=${event.target.value}`);
          }}><option value="">{view.data ? c.choose : c.loading}</option>
            {view.data?.items.map((item) => <option key={item.session_id} value={item.session_id}>{item.title}</option>)}
          </select></label></div>}
      {!error && view.data && !view.data.items.length && <p role="status">{c.emptySessions}</p>}
      <a href={`/${locale}/studio${storeID ? `?store=${storeID}` : ""}`}>{c.sessions}</a>
    </section>
  </WorkspaceFrame>;
}
