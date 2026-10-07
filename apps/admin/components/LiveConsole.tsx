// Purpose: LC-U1 lifecycle, truthful status bar and product controls; no inbox or provider-broadcast commands.
// Depends on: frozen console clients/model, Studio detail/source reads, session hooks, packages/format and Next navigation.
// Used by: LiveWorkspace at /[locale]/studio/console; Go retains authorization, CAS and receipt authority.
"use client";
import { useEffect, useState, type RefObject } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { money, displayTime } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import { readStudioDetail } from "@/lib/studio-client";
import { readClaimSource } from "@/lib/claims-client";
import { readConsole, readOfferControls } from "@/src/features/live/console-client";
import type { ConsoleOffer, CopyResult } from "@/src/features/live/console-model";
import { liveRequest } from "@/src/features/live/command-journal";
import { primaryAction, stockDelta, facebookPostLink, liveStockAllowed, consoleCommentStat } from "@/src/features/live/workspace-model";
import { useLiveRead, useLiveCommand } from "@/src/features/live/use-live-workspace";
import { workspaceCopy, type WorkspaceCopy } from "@/src/features/live/workspace-copy";

/** Shows server-authored lifecycle and snapshots; unresolved writes remain fenced across refreshes. */
export function LiveConsole({ locale, store, sessionID, navigationGuard }: { locale: Locale; store: Store; sessionID: string; navigationGuard: RefObject<() => boolean> }) {
  const c = workspaceCopy[locale], router = useRouter(), scope = `${store.id}:${sessionID}`;
  const view = useLiveRead(scope, true, (signal) => readConsole(store.id, sessionID, signal), true);
  const detailView = useLiveRead(`${scope}:detail`, true, (signal) => readStudioDetail(store.id, sessionID, signal));
  const sourceView = useLiveRead(`${scope}:source`, true, (signal) => readClaimSource(store.id, sessionID, signal));
  const controlView = useLiveRead(`${scope}:controls`, true, (signal) => readOfferControls(store.id, sessionID, signal));
  const refresh = () => { view.refresh(); detailView.refresh(); sourceView.refresh(); controlView.refresh(); };
  const signedOut = [view, detailView, sourceView, controlView].some((v) => v.error === "signed-out");
  const command = useLiveCommand(scope, signedOut ? "" : view.boundary, refresh, (request, value) => {
    if (!request.path.endsWith("/copy")) return;
    const result = value as CopyResult;
    router.push(`/${locale}/studio/${result.conflicts.length ? "claims" : "console"}?store=${store.id}&scene=${result.session.session_id}`);
  });
  useEffect(() => {
    navigationGuard.current = () => {
      if ((command.busy || command.canRetry) && !window.confirm(c.leavePending)) return false;
      command.invalidate(); return true;
    };
    return () => { navigationGuard.current = () => true; };
  }, [navigationGuard, command.busy, command.canRetry, c.leavePending, command.invalidate]);
  const [copying, setCopying] = useState(false), [title, setTitle] = useState(""), [copyConflict, setCopyConflict] = useState("");
  const can = (permission: string) => store.role === "owner" || store.permissions?.includes(permission) === true;
  const data = signedOut ? null : view.data, detail = detailView.error ? null : detailView.data;
  const source = sourceView.error ? null : sourceView.data?.source;
  const controls = controlView.error ? [] : controlView.data ?? [];
  const facebookLink = source?.active && source.verified ? facebookPostLink(source.platform, source.source_object_id) : null;
  const action = data ? primaryAction(data.session.lifecycle) : null;
  const manage = can("live:manage") && detail?.can_manage === true;
  const sessionEditable = !!data && ["draft", "live"].includes(data.session.lifecycle) && !command.blocked;
  const editable = manage && sessionEditable;
  const comments = data ? consoleCommentStat(data.stream.source_platform, data.stats.comments) : null;
  const refusal = Object.hasOwn(c.refusals, command.reason) ? c.refusals[command.reason as keyof typeof c.refusals] : null;
  const feedback = refusal ?? (command.error === "uncertain" ? c.uncertain : command.error === "recovery" ? c.recovery : command.error === "conflict" ? c.conflict : command.error === "signed-out" ? c.signedOut : command.error === "forbidden" ? c.forbidden : c.failed);
  const primary = async () => {
    if (!data || !action || !manage) return;
    if (action === "copy") { setTitle(data.session.title); setCopying(true); return; }
    if (action === "end" && !window.confirm(c.confirmEnd)) return;
    await command.run(liveRequest(store.id, sessionID, "lifecycle", "POST", { action, expected_version: data.session.version }));
  };
  const submitCopy = async () => {
    if (!detail || !title.trim()) return;
    // A5 copy uses the planning version, never the separate A7 lifecycle CAS.
    await command.run<CopyResult>(liveRequest(store.id, sessionID, "copy", "POST", { title: title.trim(), scheduled_at: null, expected_version: detail.draft.version }), (result) => {
      setCopying(false);
      if (result.conflicts.length) setCopyConflict(result.session.session_id);
      else router.push(`/${locale}/studio/console?store=${store.id}&scene=${result.session.session_id}`);
    });
  };
  return <section className="live-console" data-testid="live-console">
    <div className="live-status-toolbar"><p>{c.polling}</p><button type="button" data-testid="live-console-refresh" disabled={signedOut || view.error === "forbidden"} onClick={refresh}>{c.refresh}</button></div>
    {!data ? <p role={view.error || signedOut ? "alert" : "status"} data-testid="live-console-unavailable">{signedOut ? c.signedOut : view.error === "forbidden" ? c.forbidden : view.error ? c.unavailable : c.loading}</p> : <>
      {view.error && <p role="alert">{c.unavailable}</p>}
      {[detailView, sourceView, controlView].some((v) => !!v.error) && <p role="alert">{c.secondaryUnavailable}</p>}
      <div className="live-phase-header">
        <div><h2>{data.session.title}</h2><ol className="live-phases" data-testid="live-phase" data-phase={data.session.lifecycle}>
          {([['draft', c.before], ['live', c.during], ['ended', c.after]] as const).map(([phase, label]) => <li key={phase} aria-current={data.session.lifecycle === phase || (phase === "ended" && data.session.lifecycle === "archived") ? "step" : undefined}>{label}</li>)}
        </ol></div>
        {action && !copying && <button type="button" className="primary" data-testid="live-primary-action" disabled={!manage || command.blocked} aria-describedby={!manage ? "live-management-reason" : undefined} onClick={() => void primary()}>{command.busy ? c.pending : c[action]}</button>}
        {!manage && <p id="live-management-reason">{c.manageRequired}</p>}
      </div>
      <dl className="live-status-bar">
        <div><dt>{comments?.source === "stream_seen" ? c.observed : c.comments}</dt><dd>{comments?.source === "unavailable" ? "—" : comments?.total ?? "—"}</dd></div>
        <div><dt>{c.keyword}</dt><dd>{data.stats.keyword_comments}</dd></div>
        <div><dt>{c.buyers}</dt><dd>{data.stats.buyers}</dd></div>
        <div><dt>{c.orders}</dt><dd>{data.stats.orders.count}</dd></div>
        <div><dt>{c.amount}</dt><dd>{money(locale, data.stats.currency, data.stats.orders.amount_minor)}</dd></div>
        <div><dt>{c.paid}</dt><dd>{money(locale, data.stats.currency, data.stats.paid.amount_minor)}</dd></div>
      </dl>
      <p className="live-helper">{c.updated} {displayTime(locale, data.stats.as_of)} · {c.taipei}</p>
      <p className="live-helper" role="status">{c.feed}: {c.stream[data.stream.state]}{data.stream.last_ok_at ? ` · ${displayTime(locale, data.stream.last_ok_at)} · ${c.taipei}` : ""}</p>
      {command.error && <div role="alert" className="live-feedback"><p>{feedback}</p>{command.canRetry && <button type="button" data-testid="live-command-retry" onClick={() => void command.retry()}>{c.retry}</button>}</div>}
      {copyConflict && <p role="status">{c.copiedConflicts} <a href={`/${locale}/studio/claims?store=${store.id}&scene=${copyConflict}`}>{c.configure}</a></p>}
      {copying && <form className="live-copy-form" onSubmit={(event) => { event.preventDefault(); void submitCopy(); }}>
        <label>{c.name}<input data-testid="live-copy-title" maxLength={200} value={title} onChange={(event) => setTitle(event.target.value)} autoFocus /></label>
        <button type="submit" className="primary" data-testid="live-copy-confirm" disabled={command.blocked || !title.trim()}>{c.confirmCopy}</button>
        <button type="button" disabled={command.busy} onClick={() => setCopying(false)}>{c.cancel}</button>
      </form>}
      <div className="live-console-columns">
        <section className="live-preview"><h3>{c.facebook}</h3>
          {facebookLink ? <><p className="live-helper">{c.facebookLinkNotice}</p><a href={facebookLink} target="_blank" rel="noopener noreferrer">{c.openFacebook}</a></> : <p>{c.noFacebookLink}</p>}
          {(data.stream.source_platform === "instagram" || !data.stream.video_embeddable) && <p className="live-instagram-notice">{c.instagram}</p>}
          <a href={`/${locale}/studio/claims?store=${store.id}&scene=${sessionID}`}>{c.configure}</a>
        </section>
        <section className="live-products"><h3>{c.offers}</h3>
          {!editable && manage && !["draft", "live"].includes(data.session.lifecycle) && <p>{c.endReadOnly}</p>}
          {!data.offers.length && <p>{c.empty}</p>}
          <ul>{data.offers.map((offer) => <OfferRow key={`${offer.offer_id}:${offer.version}:${offer.stock.balance_version}`} offer={offer} c={c} locale={locale} currency={data.stats.currency}
            editable={editable} stockEditable={sessionEditable} canToggle={controls.some((o) => o.offer_id === offer.offer_id && o.version === offer.version)} canStock={liveStockAllowed(store)} recommended={data.recommended?.offer_id === offer.offer_id}
            onToggle={() => { const control = controls.find((o) => o.offer_id === offer.offer_id && o.version === offer.version); if (!control) return Promise.resolve(); return command.run(liveRequest(store.id, sessionID, `claims/offers/${offer.offer_id}`, "PATCH", { expected_version: offer.version, active: !offer.active, max_quantity_per_claim: control.max_quantity_per_claim })); }}
            onRecommend={() => command.run(liveRequest(store.id, sessionID, `claims/offers/${offer.offer_id}/recommend`, "POST", { expected_version: offer.version, post_comment: false }))}
            onStock={(delta) => offer.stock.warehouse_id ? command.run(liveRequest(store.id, sessionID, "inventory/adjustments", "POST", { warehouse_id: offer.stock.warehouse_id, sku_id: offer.sku_id, delta, expected_version: offer.stock.balance_version, reason: "live_console_edit" })) : Promise.resolve()}
          />)}</ul>
        </section>
      </div>
    </>}
  </section>;
}

function OfferRow({ offer, c, locale, currency, editable, stockEditable, canToggle, canStock, recommended, onToggle, onRecommend, onStock }: {
  offer: ConsoleOffer; c: WorkspaceCopy; locale: Locale; currency: string; editable: boolean; stockEditable: boolean; canToggle: boolean; canStock: boolean; recommended: boolean;
  onToggle: () => Promise<void>; onRecommend: () => Promise<void>; onStock: (delta: number) => Promise<void>;
}) {
  const [quantity, setQuantity] = useState(String(offer.stock.sellable)), [invalid, setInvalid] = useState(false);
  const canAdjust = stockEditable && canStock && offer.stock.tracked && !!offer.stock.warehouse_id;
  const reason = !canStock ? c.stockPermission : !offer.stock.warehouse_id ? c.missingWarehouse : "";
  return <li className="live-offer" data-testid={`live-offer-${offer.offer_id}`} data-recommended={recommended || undefined}>
    <div className="live-offer-heading"><div><h4>{offer.keyword} · {offer.product_name}</h4><p>{offer.variant_label}</p></div>
      <label className="live-offer-toggle"><input type="checkbox" role="switch" data-testid={`live-offer-toggle-${offer.offer_id}`} checked={offer.active} disabled={!editable || !canToggle} onChange={() => void onToggle()} />{offer.active ? c.open : c.closed}</label>
    </div>
    <div className="live-offer-facts"><strong>{money(locale, currency, offer.live_price_minor ?? offer.sku_price_minor)}</strong><span>{c.claimed} {offer.claimed.quantity}</span><span>{c.paidQty} {offer.paid_qty}</span>{offer.sold_out && <span>{c.soldOut}</span>}{offer.low_stock && <span>{c.lowStock}</span>}</div>
    <form className="live-stock-form" onSubmit={(event) => { event.preventDefault(); const delta = stockDelta(quantity, offer.stock.sellable); if (delta === null || delta === 0) { setInvalid(delta === null); return; } setInvalid(false); void onStock(delta); }}>
      <label>{c.stock}{offer.stock.tracked ? <input inputMode="numeric" data-testid={`live-stock-${offer.offer_id}`} value={quantity} disabled={!canAdjust} aria-describedby={reason ? `stock-reason-${offer.offer_id}` : undefined} onChange={(event) => { setQuantity(event.target.value); setInvalid(false); }} /> : <strong>{c.untracked} ∞</strong>}</label>
      {offer.stock.tracked && <button type="submit" data-testid={`live-stock-save-${offer.offer_id}`} disabled={!canAdjust || stockDelta(quantity, offer.stock.sellable) === 0}>{c.saveStock}</button>}
      <button type="button" data-testid={`live-recommend-${offer.offer_id}`} disabled={!editable || !offer.active} title={c.recommendHint} onClick={() => void onRecommend()}>{recommended ? c.recommended : c.recommend}</button>
    </form>
    {reason && offer.stock.tracked && <p className="live-helper" id={`stock-reason-${offer.offer_id}`}>{reason}</p>}
    {invalid && <p role="alert">{c.stockInvalid}</p>}
    <p className="live-helper">{c.recommendHint}</p>
  </li>;
}
