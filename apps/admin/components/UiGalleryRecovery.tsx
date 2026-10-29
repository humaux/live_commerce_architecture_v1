// Purpose: Developer-only StatePanel/InlineError acceptance fixtures, mounted inside the flag-guarded gallery.
// Depends on: shared DS components, i18n copy, useGalleryRecord, lib/customers-client readJSON; synthetic GET responses intercepted by the CI browser.
// Used by: UiGallerySections; DS-STATES SSR and real-click browser acceptance.
// Invariants: no real data or writes; unknown reads one fixed operation; raw denial diagnostics never become visible copy.
"use client";
import { StatePanel, InlineError, type StatePanelVariant } from "@live-commerce/ui/ds";
import { stateRecoveryCopy, type Locale } from "@live-commerce/i18n";
import { useGalleryRecord } from "../src/ui-gallery-state";
import { readJSON } from "../lib/customers-client";

type ReadFailure = { status: number; message: string; resourceName: string };
const denial: ReadFailure = { status: 403, message: "Access denied to private-order-123", resourceName: "private-order-123" };
/** Actual gallery caller boundary: choose by status; diagnostics cannot supply title/body/note/status. */
export function GalleryForbidden({ locale, failure, href }: { locale: Locale; failure: ReadFailure; href: string }) {
  const c = stateRecoveryCopy[locale];
  if (failure.status !== 403) return <InlineError message={c.failed} />;
  return <StatePanel variant="forbidden" title={c.forbidden} body={c.forbiddenBody} note={c.forbiddenNote} action={{ label: c.back, href }} />;
}
export function UiGalleryRecovery({ locale }: { locale: Locale }) {
  const c = stateRecoveryCopy[locale];
  const [state, save, ready] = useGalleryRecord(`ds-states:${locale}`, { offline: "", unknown: "", inline: "" });
  const read = async (kind: "offline" | "unknown" | "inline") => {
    try {
      const value = await readJSON(`/api/__gallery/recovery?operation=ds-states-example&kind=${kind}`, AbortSignal.timeout(12000));
      if (!value || typeof value !== "object" || !("operation" in value) || value.operation !== "ds-states-example" ||
          !("status" in value) || value.status !== (kind === "unknown" ? "pending" : "loaded")) throw new Error("invalid fixture response");
      save({ ...state, [kind]: kind === "unknown" ? c.checked : c.loaded });
    } catch { save({ ...state, [kind]: c.failed }); }
  };
  const href = `/${locale}/ui-gallery?tab=b#sec-switch`;
  const panels: StatePanelVariant[] = ["empty", "forbidden", "offline", "expired", "unknown"];
  return <div data-recovery-ready={ready}>
    {panels.map(variant => {
      if (variant === "forbidden") return <div key={variant} data-recovery={variant}><GalleryForbidden locale={locale} failure={denial} href={href} /></div>;
      const reading = variant === "offline" || variant === "unknown";
      const title = c[variant];
      const body = c[`${variant}Body`];
      return <div key={variant} data-recovery={variant}>
        <StatePanel variant={variant} title={title} body={body}
          status={variant === "unknown" ? state.unknown || c.pending : variant === "offline" ? state.offline : undefined}
          action={reading ? { label: variant === "unknown" ? c.check : c.retry, onClick: () => void read(variant) } : { label: c.back, href }} />
      </div>;
    })}
    <div data-recovery="inline-callback">
      <InlineError message={c.failed} action={{ label: c.retry, onClick: () => void read("inline") }} />
      <p role="status">{state.inline}</p>
    </div>
    <div data-recovery="inline-link"><InlineError message={c.failed} action={{ label: c.back, href }} /></div>
  </div>;
}
