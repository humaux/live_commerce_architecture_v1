// Purpose: signed UX-LIVE v3 console, authoritative session statistics and scoped product/window controls.
// Depends on: frozen console clients/model, Studio detail/source reads, session hooks, packages/format and Next navigation.
// Used by: LiveWorkspace at /[locale]/studio/console; Go retains authorization, CAS and receipt authority.
// Invariants: no provider playback/send inference; uncertain writes remain fenced, and tabs never remount reply ownership.
"use client";
import { useEffect, useRef, useState, type RefObject } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { money, displayTime } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import { readStudioDetail } from "@/lib/studio-client";
import { readClaimsBoard, readClaimSource } from "@/lib/claims-client";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import { readConsole, readOfferControls } from "@/src/features/live/console-client";
import type { ConsoleOffer, CopyResult } from "@/src/features/live/console-model";
import { liveRequest, type LiveRequest } from "@/src/features/live/command-journal";
import { primaryAction, stockDelta, facebookPostLink, liveStockAllowed, consoleCommentStat } from "@/src/features/live/workspace-model";
import { useLiveRead, useLiveCommand } from "@/src/features/live/use-live-workspace";
import { workspaceCopy, type WorkspaceCopy } from "@/src/features/live/workspace-copy";
import { CommentStream } from "./CommentStream";

/** Shows server-authored lifecycle and snapshots; unresolved writes remain fenced across refreshes. */
export function LiveConsole({ locale, store, sessionID, navigationGuard, onRefreshList, commentCalibration=false }: { locale: Locale; store: Store; sessionID: string; navigationGuard: RefObject<() => boolean>; onRefreshList?: () => void; commentCalibration?:boolean }) {
  const c = workspaceCopy[locale], router = useRouter(), scope = `${store.id}:${sessionID}`;
  const view = useLiveRead(scope, true, (signal) => readConsole(store.id, sessionID, signal), true);
  const detailView = useLiveRead(`${scope}:detail`, true, (signal) => readStudioDetail(store.id, sessionID, signal));
  const sourceView = useLiveRead(`${scope}:source`, true, (signal) => readClaimSource(store.id, sessionID, signal));
  const controlView = useLiveRead(`${scope}:controls`, true, (signal) => readOfferControls(store.id, sessionID, signal));
  // List reads are user/command initiated, never part of A1's periodic reader.
  const refresh = () => { view.refresh(); detailView.refresh(); sourceView.refresh(); controlView.refresh(); onRefreshList?.(); };
  const signedOut = [view, detailView, sourceView, controlView].some((v) => v.error === "signed-out");
  const command = useLiveCommand(scope, signedOut ? "" : view.boundary, refresh, (request, value) => {
    if (!request.path.endsWith("/copy")) return;
    const result = value as CopyResult;
    router.push(`/${locale}/studio/${result.conflicts.length ? "claims" : "console"}?store=${store.id}&scene=${result.session.session_id}`);
  });
  const [tab, setTab] = useState("comments");
  const [windowBusy, setWindowBusy] = useState(false), [windowIssue, setWindowIssue] = useState(""), [legacyIssue, setLegacyIssue] = useState("");
  const windowActive = useRef(false), windowMounted = useRef(true), windowRead = useRef<AbortController | null>(null), windowEpoch = useRef(0);
  const windowIdentity = useRef(""); windowIdentity.current = `${scope}|${signedOut ? "" : view.boundary}`;
  useEffect(() => { windowMounted.current = true; return () => { windowMounted.current = false; windowRead.current?.abort(); }; }, []);
  const windowFence = `live-console-window:${scope}`;
  const readLegacyWindow = () => { try { return sessionStorage.getItem(windowFence) !== null ? c.uncertain : ""; } catch { return c.recovery; } };
  useEffect(() => {
    // Old bool markers have no recoverable request/key. Preserve them without creating a fresh command.
    setLegacyIssue(readLegacyWindow());
  }, [windowFence, view.boundary, c.uncertain, c.recovery]);
  useEffect(() => {
    const cancel = () => { ++windowEpoch.current; windowRead.current?.abort(); windowActive.current = false; if (windowMounted.current) setWindowBusy(false); };
    const visibility = () => { if (document.visibilityState !== "visible") cancel(); };
    const storage = (event: StorageEvent) => { if (event.key === "commerce-session-logout") cancel(); };
    const channel = typeof BroadcastChannel !== "undefined" ? new BroadcastChannel("commerce-session") : null;
    if (channel) channel.onmessage = (event) => { if (event.data?.type === "logout") cancel(); };
    document.addEventListener("visibilitychange", visibility); window.addEventListener("pagehide", cancel);
    window.addEventListener("commerce-session-logout", cancel); window.addEventListener("storage", storage);
    return () => { cancel(); channel?.close(); document.removeEventListener("visibilitychange", visibility); window.removeEventListener("pagehide", cancel); window.removeEventListener("commerce-session-logout", cancel); window.removeEventListener("storage", storage); };
  }, [scope, view.boundary, signedOut]);
  const blocked = command.blocked || windowBusy || !!legacyIssue;
  const run = async <T,>(input: LiveRequest, complete?: (value: T) => void) => {
    const legacy = readLegacyWindow();
    if (legacy) { setLegacyIssue(legacy); return; }
    if (windowActive.current || blocked) return;
    await command.run<T>(input, complete);
  };
  const retry = async () => {
    const legacy = readLegacyWindow();
    if (legacy) { setLegacyIssue(legacy); return; }
    if (!windowActive.current) await command.retry();
  };
  useEffect(() => {
    navigationGuard.current = () => {
      if ((command.busy || command.canRetry || windowActive.current || legacyIssue) && !window.confirm(c.leavePending)) return false;
      ++windowEpoch.current; windowRead.current?.abort(); command.invalidate(); return true;
    };
    return () => { navigationGuard.current = () => true; };
  }, [navigationGuard, command.busy, command.canRetry, legacyIssue, c.leavePending, command.invalidate]);
  const toggleWindow = async () => {
    if (windowActive.current || blocked || !view.boundary || !view.data || !can("live:manage")) return;
    const legacy = readLegacyWindow(); if (legacy) { setLegacyIssue(legacy); return; }
    const target = view.data?.window.state === "OPEN" ? "CLOSED" : "OPEN";
    const epoch = ++windowEpoch.current, identity = windowIdentity.current, csrf = csrfCookie();
    windowActive.current = true; setWindowBusy(true); setWindowIssue("");
    try {
      windowRead.current = new AbortController();
      const signal = AbortSignal.any([windowRead.current.signal, AbortSignal.timeout(15000)]);
      const board = await readClaimsBoard(store.id, sessionID, signal);
      const boundary = await sessionBoundary(csrf);
      if (!windowMounted.current || epoch !== windowEpoch.current || identity !== windowIdentity.current || signal.aborted || document.visibilityState !== "visible" || !csrf || csrfCookie() !== csrf || boundary !== view.boundary) return;
      // This board read prepares only a new command. Existing UNKNOWN commands bypass it and replay their durable descriptor.
      await command.run(liveRequest(store.id, sessionID, "claims/window", "POST", { expected_version: board.window.version, state: target, match_mode: board.window.match_mode }));
      // A read may update the displayed window after a lost ACK; only the original command receipt settles its journal.
      if (windowMounted.current && epoch === windowEpoch.current) view.refresh();
    } catch (error) {
      const code = error && typeof error === "object" && "code" in error ? error.code : "uncertain";
      if (windowMounted.current && epoch === windowEpoch.current) setWindowIssue(code === "forbidden" ? c.forbidden : code === "signed-out" ? c.signedOut : c.failed);
    } finally {
      if (windowMounted.current && epoch === windowEpoch.current) { windowActive.current = false; setWindowBusy(false); }
    }
  };
  const [copying, setCopying] = useState(false), [title, setTitle] = useState(""), [copyConflict, setCopyConflict] = useState("");
  const can = (permission: string) => store.role === "owner" || store.permissions?.includes(permission) === true;
  const data = signedOut ? null : view.data, detail = detailView.error ? null : detailView.data;
  const source = sourceView.error ? null : sourceView.data?.source;
  const controls = controlView.error ? [] : controlView.data ?? [];
  const facebookLink = source?.active && source.verified ? facebookPostLink(source.platform, source.source_object_id) : null;
  const action = data ? primaryAction(data.session.lifecycle) : null;
  const manage = can("live:manage") && detail?.can_manage === true;
  const sessionEditable = !!data && ["draft", "live"].includes(data.session.lifecycle) && !blocked;
  const editable = manage && sessionEditable;
  const comments = data ? consoleCommentStat(data.stream.source_platform, data.stats.comments) : null;
  const refusal = Object.hasOwn(c.refusals, command.reason) ? c.refusals[command.reason as keyof typeof c.refusals] : null;
  const feedback = refusal ?? (command.error === "uncertain" ? c.uncertain : command.error === "recovery" ? c.recovery : command.error === "conflict" ? c.conflict : command.error === "signed-out" ? c.signedOut : command.error === "forbidden" ? c.forbidden : c.failed);
  const primary = async () => {
    if (!data || !action || !manage || blocked || windowActive.current) return;
    if (action === "copy") { setTitle(data.session.title); setCopying(true); return; }
    if (action === "end" && !window.confirm(c.confirmEnd)) return;
    await run(liveRequest(store.id, sessionID, "lifecycle", "POST", { action, expected_version: data.session.version }));
  };
  const submitCopy = async () => {
    if (!detail || !title.trim()) return;
    // A5 copy uses the planning version, never the separate A7 lifecycle CAS.
    await run<CopyResult>(liveRequest(store.id, sessionID, "copy", "POST", { title: title.trim(), scheduled_at: null, expected_version: detail.draft.version }), (result) => {
      setCopying(false);
      if (result.conflicts.length) setCopyConflict(result.session.session_id);
      else router.push(`/${locale}/studio/console?store=${store.id}&scene=${result.session.session_id}`);
    });
  };
  return <section className="live-console" data-testid="live-console" data-phase={data?.session.lifecycle} data-mobile-tab={tab}>
    <div className="live-status-toolbar"><button type="button" data-testid="live-console-refresh" disabled={signedOut || view.error === "forbidden"} onClick={refresh}>{c.refresh}</button></div>
    {!data ? <p role={view.error || signedOut ? "alert" : "status"} data-testid="live-console-unavailable">{signedOut ? c.signedOut : view.error === "forbidden" ? c.forbidden : view.error ? c.unavailable : c.loading}</p> : <>
      {view.error && <p role="alert">{c.unavailable}</p>}
      {[detailView, sourceView, controlView].some((v) => !!v.error) && <p role="alert">{c.secondaryUnavailable}</p>}
      <header className="live-parity-header">
        <h2>{data.session.title}</h2>
        {Object.values(data.capabilities).some((platform) => Object.values(platform ?? {}).some((capability) => capability?.evidence === "MOCK")) && <span className="live-demo-tag">{c.demoData}</span>}
        <span className="live-order-state">{data.window.state === "OPEN" ? c.ordersOpen : c.ordersPaused}</span>
        <button type="button" onClick={() => void toggleWindow()} disabled={!manage || blocked}>{data.window.state === "OPEN" ? c.pauseOrders : c.resumeOrders}</button>
      </header>
      {windowIssue && <p role="alert">{windowIssue}</p>}
      {legacyIssue && <p role="alert">{legacyIssue}</p>}
      <dl className="live-status-bar">
        <div><dt>{comments?.source === "stream_seen" ? c.observed : c.comments}</dt><dd>{comments?.source === "unavailable" ? "—" : comments?.total ?? "—"}</dd></div>
        <div><dt>{c.keyword}</dt><dd>{data.stats.keyword_comments}</dd></div>
        <div><dt>{c.orders}</dt><dd>{data.stats.orders.count}</dd></div>
        <div><dt>{c.paidOrders}</dt><dd>{data.stats.paid.count}</dd></div>
      </dl>
      <div className="live-advanced">
        <div className="live-phase-header">
          {action && !copying && <button type="button" className="primary" data-testid="live-primary-action" disabled={!manage || blocked} aria-describedby={!manage ? "live-management-reason" : undefined} onClick={() => void primary()}>{command.busy ? c.pending : c[action]}</button>}
          {!manage && <p id="live-management-reason">{c.manageRequired}</p>}
        </div>
        <dl className="live-revenue-stats">
          <div><dt>{c.buyers}</dt><dd>{data.stats.buyers}</dd></div>
          <div><dt>{c.amount}</dt><dd>{money(locale, data.stats.currency, data.stats.orders.amount_minor)}</dd></div>
          <div><dt>{c.paid}</dt><dd>{money(locale, data.stats.currency, data.stats.paid.amount_minor)}</dd></div>
        </dl>
        <p className="live-helper">{c.updated} {displayTime(locale, data.stats.as_of)} · {c.taipei}</p>
        <p className="live-helper" role="status">{c.feed}: {c.stream[data.stream.state]}</p>
      </div>
      <div className="live-mobile-tabs" role="tablist" aria-label={c.title}>
        {([['live', c.liveTab], ['comments', c.commentsTab], ['buyer', c.buyerTab]] as const).map(([key, label]) => <button key={key} type="button" role="tab" aria-selected={tab === key} aria-controls={`live-panel-${key}`} onClick={() => setTab(key)}>{label}</button>)}
      </div>
      {command.error && <div role="alert" className="live-feedback"><p>{feedback}</p>{command.canRetry && !legacyIssue && <button type="button" data-testid="live-command-retry" disabled={windowBusy} onClick={() => void retry()}>{c.retry}</button>}</div>}
      {copyConflict && <p role="status">{c.copiedConflicts} <a href={`/${locale}/studio/claims?store=${store.id}&scene=${copyConflict}`}>{c.configure}</a></p>}
      {copying && <form className="live-copy-form" onSubmit={(event) => { event.preventDefault(); void submitCopy(); }}>
        <label>{c.name}<input data-testid="live-copy-title" maxLength={200} value={title} disabled={blocked} onChange={(event) => setTitle(event.target.value)} autoFocus /></label>
        <button type="submit" className="primary" data-testid="live-copy-confirm" disabled={blocked || !title.trim()}>{c.confirmCopy}</button>
        <button type="button" disabled={blocked} onClick={() => setCopying(false)}>{c.cancel}</button>
      </form>}
      <div className="live-console-columns">
        <div className="live-console-left" id="live-panel-live">
        <section className="live-preview"><h3>{c.currentLive}</h3><div className="live-video-surface"><strong>{data.session.title}</strong><span>{data.stream.source_platform}</span></div>
          {facebookLink ? <><p className="live-helper">{c.facebookLinkNotice}</p><a href={facebookLink} target="_blank" rel="noopener noreferrer">{c.openFacebook}</a></> : <p>{c.noFacebookLink}</p>}
          {(data.stream.source_platform === "instagram" || !data.stream.video_embeddable) && <p className="live-instagram-notice">{c.instagram}</p>}
          <a href={`/${locale}/studio/claims?store=${store.id}&scene=${sessionID}`}>{c.configure}</a>
        </section>
        <section className="live-products"><h3>{c.offers}</h3>
          {!editable && manage && !["draft", "live"].includes(data.session.lifecycle) && <p>{c.endReadOnly}</p>}
          {!data.offers.length && <p>{c.empty}</p>}
          <ul>{data.offers.map((offer) => <OfferRow key={offer.offer_id} offer={offer} maxQuantity={controls.find((control) => control.offer_id === offer.offer_id)?.max_quantity_per_claim} c={c} locale={locale} currency={data.stats.currency}
            editable={editable} stockEditable={sessionEditable} canToggle={controls.some((o) => o.offer_id === offer.offer_id && o.version === offer.version)} canStock={liveStockAllowed(store)} recommended={data.recommended?.offer_id === offer.offer_id}
            onToggle={() => { const control = controls.find((o) => o.offer_id === offer.offer_id && o.version === offer.version); if (!control) return Promise.resolve(); return run(liveRequest(store.id, sessionID, `claims/offers/${offer.offer_id}`, "PATCH", { expected_version: offer.version, active: !offer.active, max_quantity_per_claim: control.max_quantity_per_claim })); }}
            onRecommend={() => run(liveRequest(store.id, sessionID, `claims/offers/${offer.offer_id}/recommend`, "POST", { expected_version: offer.version, post_comment: false }))}
            onStock={(delta, version, saved) => offer.stock.warehouse_id ? run(liveRequest(store.id, sessionID, "inventory/adjustments", "POST", { warehouse_id: offer.stock.warehouse_id, sku_id: offer.sku_id, delta, expected_version: version, reason: "live_console_edit" }), saved) : Promise.resolve()}
          />)}</ul>
        </section>
        </div>
        <CommentStream locale={locale} store={store} session={sessionID} platform={data.stream.source_platform} capabilities={data.capabilities} offers={data.offers} limits={controls} mobileTab={tab} calibration={commentCalibration}/>
      </div>
    </>}
  </section>;
}

function OfferRow({ offer, maxQuantity, c, locale, currency, editable, stockEditable, canToggle, canStock, recommended, onToggle, onRecommend, onStock }: {
  offer: ConsoleOffer; maxQuantity?: number; c: WorkspaceCopy; locale: Locale; currency: string; editable: boolean; stockEditable: boolean; canToggle: boolean; canStock: boolean; recommended: boolean;
  onToggle: () => Promise<void>; onRecommend: () => Promise<void>; onStock: (delta: number, version: number, saved: () => void) => Promise<void>;
}) {
  // Idle values follow A1 directly. Once edited, retain the input and its CAS
  // snapshot until explicit confirmation/save/cancel; a poll must not rebase it.
  const [edit, setEdit] = useState<{ quantity: string; stock: ConsoleOffer["stock"] } | null>(null), [invalid, setInvalid] = useState(false);
  const quantity = edit?.quantity ?? String(offer.stock.sellable), baseline = edit?.stock ?? offer.stock;
  const changed = !!edit && (baseline.balance_version !== offer.stock.balance_version || baseline.sellable !== offer.stock.sellable || baseline.warehouse_id !== offer.stock.warehouse_id || baseline.tracked !== offer.stock.tracked);
  const canAdjust = stockEditable && canStock && offer.stock.tracked && !!offer.stock.warehouse_id;
  const reason = !canStock ? c.stockPermission : !offer.stock.warehouse_id ? c.missingWarehouse : "";
  return <li className="live-offer" data-testid={`live-offer-${offer.offer_id}`} data-recommended={recommended || undefined}>
    <div className="live-offer-heading"><div><h3>{offer.product_name}</h3><p>{offer.variant_label}</p></div>
    </div>
    <p>{c.keywordHint.replace("{keyword}", offer.keyword)}</p>
    <p><strong>{c.remaining} {offer.stock.tracked ? offer.stock.sellable : "—"} · {c.soldCount} {offer.paid_qty}</strong></p>
    <p className="live-helper">{c.maxPerBuyer} {maxQuantity ?? "—"} · {c.reserved} {offer.stock.reserved}. {c.stockHelper}</p>
    <button className="live-product-toggle" type="button" disabled={!editable || !canToggle} onClick={() => void onToggle()}>{offer.active ? c.offProduct : c.onProduct}</button>
    <div>
      <label className="live-offer-toggle"><input type="checkbox" role="switch" data-testid={`live-offer-toggle-${offer.offer_id}`} checked={offer.active} disabled={!editable || !canToggle} onChange={() => void onToggle()} />{offer.active ? c.open : c.closed}</label>
    </div>
    <div className="live-offer-facts"><strong>{money(locale, currency, offer.live_price_minor ?? offer.sku_price_minor)}</strong><span>{c.claimed} {offer.claimed.quantity}</span><span>{c.paidQty} {offer.paid_qty}</span>{offer.sold_out && <span>{c.soldOut}</span>}{offer.low_stock && <span>{c.lowStock}</span>}</div>
    <form className="live-stock-form" onSubmit={(event) => { event.preventDefault(); if (!canAdjust || changed) return; const delta = stockDelta(quantity, baseline.sellable); if (delta === null || delta === 0) { setInvalid(delta === null); return; } setInvalid(false); void onStock(delta, baseline.balance_version, () => setEdit(null)); }}>
      <label>{c.stock}{offer.stock.tracked ? <input inputMode="numeric" data-testid={`live-stock-${offer.offer_id}`} value={quantity} disabled={!canAdjust} aria-describedby={[reason ? `stock-reason-${offer.offer_id}` : "", changed ? `stock-changed-${offer.offer_id}` : ""].filter(Boolean).join(" ") || undefined} onChange={(event) => { setEdit({ quantity: event.target.value, stock: edit?.stock ?? { ...offer.stock } }); setInvalid(false); }} /> : <strong>{c.untracked} ∞</strong>}</label>
      {offer.stock.tracked && <button type="submit" data-testid={`live-stock-save-${offer.offer_id}`} disabled={!canAdjust || changed || stockDelta(quantity, baseline.sellable) === 0}>{c.saveStock}</button>}
      {edit && <button type="button" disabled={!canAdjust} onClick={() => { setEdit(null); setInvalid(false); }}>{c.cancel}</button>}
      <button type="button" data-testid={`live-recommend-${offer.offer_id}`} disabled={!editable || !offer.active} title={c.recommendHint} onClick={() => void onRecommend()}>{recommended ? c.recommended : c.recommend}</button>
    </form>
    {changed && <div className="live-helper" role="status" id={`stock-changed-${offer.offer_id}`} data-testid={`live-stock-changed-${offer.offer_id}`}>
      <p>{c.stockChanged} {offer.stock.sellable}</p>
      <button type="button" data-testid={`live-stock-confirm-${offer.offer_id}`} disabled={!canAdjust} onClick={() => { setEdit({ quantity, stock: { ...offer.stock } }); setInvalid(false); }}>{c.confirmStock}</button>
    </div>}
    {reason && offer.stock.tracked && <p className="live-helper" id={`stock-reason-${offer.offer_id}`}>{reason}</p>}
    {invalid && <p role="alert">{c.stockInvalid}</p>}
    <p className="live-helper">{c.recommendHint}</p>
  </li>;
}
