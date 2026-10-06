// Purpose: LC-U1 session selection inside the existing authenticated W0 shell.
// Depends on: Studio private reads, workspace hooks/copy, WorkspaceFrame and Next navigation.
// Used by: /[locale]/studio/console; data and writes remain scoped by the existing BFF and Go authorization.
"use client";
import { useRouter } from "next/navigation";
import { useRef } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { readStudioPage, type StudioErrorCode } from "@/lib/studio-client";
import { useLiveRead } from "@/src/features/live/use-live-workspace";
import { workspaceCopy } from "@/src/features/live/workspace-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { LiveConsole } from "./LiveConsole";
import "@/src/features/live/workspace.css";

/** Renders the current store's scene picker; a shell store change remounts all scene-owned data. */
export function LiveWorkspace({ locale, store, scene, initialError }: {
  locale: Locale; store: Store | null; scene: string; initialError: StudioErrorCode | null;
}) {
  const router = useRouter(), c = workspaceCopy[locale], storeID = store?.id ?? "";
  const beforeLeave = useRef<() => boolean>(() => true);
  // A temporary list refresh/conceal must not unmount the scene's UNKNOWN command owner.
  const retained = useRef({ store: storeID, scene: "" });
  const view = useLiveRead(`${storeID}:sessions`, !!storeID && !initialError, (signal) => readStudioPage(storeID, "", signal));
  if (retained.current.store !== storeID) retained.current = { store: storeID, scene: "" };
  const selected = scene || retained.current.scene || view.data?.items[0]?.session_id || "";
  retained.current.scene = selected;
  const error = initialError || view.error;
  const denied = !!initialError || error === "signed-out" || error === "forbidden";
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? ""} active="live" onBeforeNavigate={() => beforeLeave.current()}>
    <div className="live-workspace" data-testid="live-workspace">
      <AdminPageHeader locale={locale} description={c.lifecycleHint} />
      {denied ? <p role="alert">{error === "signed-out" ? c.signedOut : error === "forbidden" ? c.forbidden : c.unavailable}</p> : <>
        {error && <p role="alert">{c.unavailable}</p>}
        <div className="live-workspace-toolbar">
          <label htmlFor="live-session-picker">{c.choose}<select id="live-session-picker" value={selected} onChange={(event) => { if (beforeLeave.current()) router.push(`/${locale}/studio/console?store=${storeID}&scene=${event.target.value}`); }}>
            {!view.data && <option value="">{c.loading}</option>}
            {selected && !view.data?.items.some((item) => item.session_id === selected) && <option value={selected}>{c.choose}</option>}
            {view.data?.items.map((item) => <option key={item.session_id} value={item.session_id}>{item.title}</option>)}
          </select></label>
          <a data-testid="live-session-results" href={`/${locale}/studio?store=${storeID}${selected ? `&scene=${selected}` : ""}`} onClick={(event) => { if (!beforeLeave.current()) event.preventDefault(); }}>{c.sessions}</a>
          {selected && <a href={`/${locale}/studio/claims?store=${storeID}&scene=${selected}`} onClick={(event) => { if (!beforeLeave.current()) event.preventDefault(); }}>{c.source}</a>}
        </div>
        {selected && store ? <LiveConsole key={`${storeID}:${selected}`} locale={locale} store={store} sessionID={selected} navigationGuard={beforeLeave} /> : <p role="status">{view.data ? c.empty : c.loading}</p>}
      </>}
    </div>
  </WorkspaceFrame>;
}
