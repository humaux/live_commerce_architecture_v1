// Purpose: issue and copy a bundle-only claim link using the existing claims routes, without sending a DM.
// Depends on: claims-client M6/M7/catalog origin, StudioError/sessionBoundary and the inbox privacy fence.
// Used by: keyed Inbox bundle selection; link tokens and uncertain receipts are memory-only and cleared on hide/unmount.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import type { Store } from "@/lib/model";
import type { ConversationItem } from "@/lib/inbox-types";
import { issueClaimLink, readClaimBundles, readClaimProducts, readStorefrontOrigin } from "@/lib/claims-client";
import { StudioError } from "@/lib/studio-client";
import { sessionBoundary } from "@/lib/settings-client";
import { permitted } from "./privacy";
import { useInboxPrivacy } from "./use-privacy";
import { inboxCopy, inboxError } from "./copy";
import styles from "./Inbox.module.css";

type Pending = {
  key: string;
  body: Readonly<{ expected_generation: number; release_binding: boolean }>;
  origin: string;
  boundary: string;
};

/** Copy one explicitly issued claim link; an uncertain issuance retries only its original key and CAS body. */
export function BundleRecovery({ store, conversation, locale, onUnauthorized }: {
  store: Store; conversation: ConversationItem; locale: string; onUnauthorized: () => void;
}) {
  const c = inboxCopy(locale);
  const { bundle_id: bundle, session_id: session } = conversation;
  const pending = useRef<Pending | null>(null);
  const issued = useRef<{ url: string; boundary: string; expiresAt: string } | null>(null);
  // A captured click handler must also stop after a terminal replay/expiry, before React commits disabled state.
  const stopped = useRef(false);
  const inFlight = useRef(false);
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const clear = useCallback(() => {
    pending.current = null;
    issued.current = null;
    stopped.current = false;
    inFlight.current = false;
    setBusy(false);
    setStatus(null);
  }, []);
  const privacy = useInboxPrivacy(clear);
  const allowed = permitted(store, "live:read") && permitted(store, "live:manage");
  const eligible = !!bundle && !!session && conversation.link_pending_manual === true;
  useEffect(() => () => {
    privacy.fence.invalidate(false);
    pending.current = null;
    issued.current = null;
    inFlight.current = false;
  }, [store.id, bundle, session, locale, privacy.fence]);

  async function copy() {
    if (!eligible || !allowed || inFlight.current || stopped.current || !privacy.visible || privacy.blocked.current ||
      document.visibilityState !== "visible") return;
    const ticket = privacy.fence.begin();
    if (!privacy.fence.current(ticket)) return;
    inFlight.current = true;
    setBusy(true);
    setStatus(null);
    try {
      let boundary: string;
      try { boundary = await sessionBoundary(); }
      catch { throw new StudioError("signed-out"); }
      if (!privacy.fence.current(ticket)) return;
      const previous = pending.current ?? issued.current;
      if (previous && previous.boundary !== boundary) throw new StudioError("signed-out");
      if (!issued.current) {
        if (!pending.current) {
          const signal = AbortSignal.any([ticket.signal, AbortSignal.timeout(15000)]);
          let cursor = "", generation: number | undefined;
          const seen = new Set<string>();
          // M6 is the existing authoritative generation read; older flagged bundles can be on a later page.
          do {
            if (seen.has(cursor)) throw new StudioError("unavailable");
            seen.add(cursor);
            const page = await readClaimBundles(store.id, session!, cursor, signal);
            if (!privacy.fence.current(ticket)) return;
            generation = page.items.find((item) => item.bundle_id === bundle)?.link.generation;
            cursor = page.next_cursor;
          } while (generation === undefined && cursor);
          if (generation === undefined) throw new StudioError("not-found");
          const products = await readClaimProducts(store.id, signal);
          const origin = await readStorefrontOrigin(store.id, products, signal);
          if (!privacy.fence.current(ticket)) return;
          if (!origin) { setStatus("claimLinkNoOrigin"); return; }
          pending.current = {
            key: crypto.randomUUID(), origin, boundary,
            body: Object.freeze({ expected_generation: generation, release_binding: false }),
          };
        }
        const request = pending.current;
        // Calls existing M7 (live-console-v1 A1.5 P2-2); no inbox send or fresh key on UNKNOWN.
        const result = await issueClaimLink(store.id, session!, bundle!, request.body, request.key, request.boundary);
        if (!privacy.fence.current(ticket)) return;
        pending.current = null;
        if (!result.token) { stopped.current = true; setStatus("claimLinkReplayed"); return; }
        const buyerLocale = ["zh-TW", "zh-CN", "en"].includes(locale) ? locale : "en";
        // Same buyer URL format as StudioClaims; the secret URL is never rendered or navigated to.
        issued.current = { url: `${request.origin}/${buyerLocale}/claim#t=${result.token}`,
          boundary: request.boundary, expiresAt: result.expires_at };
      }
      if (!privacy.fence.current(ticket) || document.visibilityState !== "visible") return;
      if (Date.parse(issued.current.expiresAt) <= Date.now()) {
        issued.current = null;
        stopped.current = true;
        setStatus("claimLinkExpired");
        return;
      }
      try {
        await navigator.clipboard.writeText(issued.current.url);
        if (privacy.fence.current(ticket)) setStatus("copied");
      } catch {
        if (privacy.fence.current(ticket)) setStatus("claimLinkClipboard");
      }
    } catch (cause) {
      if (!privacy.fence.current(ticket)) return;
      const code = cause instanceof StudioError ? cause.code : "unavailable";
      if (code !== "uncertain") pending.current = null;
      if (code === "signed-out") { privacy.expire(); onUnauthorized(); }
      else setStatus(code === "uncertain" ? "claimLinkUncertain" :
        code === "conflict" ? "claimLinkConflict" : code === "not-found" ? "not_found" : code);
    } finally {
      if (privacy.fence.current(ticket)) { inFlight.current = false; setBusy(false); }
    }
  }

  return <div className={styles.notice} data-testid="bundle-recovery">
    <p>{c.manual}</p>
    <p>{c.manualRecovery}</p>
    {!allowed && <p>{c.claimLinkPermission}</p>}
    {!eligible && <p>{c.linkUnavailable}</p>}
    {status && <p role="status">{inboxError(locale, status)}</p>}
    <button type="button" className={styles.button}
      data-testid={pending.current ? "bundle-link-retry" : "bundle-copy-link"}
      disabled={busy || stopped.current || !allowed || !eligible || !privacy.visible || privacy.blocked.current}
      onClick={() => void copy()}>
      {busy ? c.loading : pending.current ? c.claimLinkRetry : c.copyLink}
    </button>
  </div>;
}
