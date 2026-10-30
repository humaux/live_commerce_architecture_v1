// Purpose: Signed session product and immediate inventory adjustment tasks.
// Depends on: A1 console projection, Studio permissions and existing inventory command journal.
// Used by: StudioClaims products state.
// Invariants: stock/version/warehouse come from Go; inventory permission is rechecked server-side; UNKNOWN retains its key.
"use client";
import { useState } from "react";
import { liveSessionsCopy, type Locale } from "@live-commerce/i18n";
import { money } from "@live-commerce/format";
import { readStudioDetail } from "@/lib/studio-client";
import { readConsole } from "@/src/features/live/console-client";
import { liveRequest } from "@/src/features/live/command-journal";
import { useLiveCommand, useLiveRead } from "@/src/features/live/use-live-workspace";
import { useSessionTask } from "./SessionTasks";
import s from "./session-tasks.module.css";

/** The selected offer survives refresh in navigation state; it supplies no write authority. */
export function SessionProducts({ locale, store, scene }: { locale: Locale; store: string; scene: string }) {
  const c = liveSessionsCopy[locale], task = useSessionTask();
  const view = useLiveRead(`${store}:${scene}:products`, true, signal => readConsole(store, scene, signal));
  const authority = useLiveRead(`${store}:${scene}:product-authority`, true, signal => readStudioDetail(store, scene, signal));
  const canManage = authority.data?.can_manage === true && authority.boundary === view.boundary;
  const command = useLiveCommand(`${store}:${scene}`, view.boundary, view.refresh);
  const [selected, setSelected] = useState("");
  const offer = view.data?.offers.find(row => row.offer_id === (task.startsWith("stock-") ? task.slice(6) : selected));
  const adjust = (delta: number) => {
    if (!offer?.stock.tracked || !offer.stock.warehouse_id || !canManage || command.blocked) return;
    void command.run(liveRequest(store, scene, "inventory/adjustments", "POST", { warehouse_id: offer.stock.warehouse_id, sku_id: offer.sku_id, delta, expected_version: offer.stock.balance_version, reason: "live_console_edit" }));
  };
  return <section className={s.page}>
    <h1>{offer ? c.stock : c.products}</h1>
    {view.error || command.error ? <p role="alert">{command.canRetry ? c.uncertain : c.unavailable}</p> : !view.data && <p role="status">{c.loading}</p>}
    {command.canRetry && <button type="button" onClick={() => void command.retry()}>{c.retry}</button>}
    {offer ? <div className={s.card}><h2>{offer.product_name}</h2><p>{offer.keyword} · {offer.variant_label}</p>
      <dl className={s.facts}><div><dt>{c.available}</dt><dd data-testid="session-stock-available">{offer.stock.sellable}</dd></div><div><dt>{c.adjusted}</dt><dd>{offer.stock.sellable}</dd></div></dl>
      <div className={s.footer}><button disabled={!canManage || command.blocked || !offer.stock.tracked || !offer.stock.warehouse_id} onClick={() => adjust(-1)}>{c.minus}</button><button className="primary" disabled={!canManage || command.blocked || !offer.stock.tracked || !offer.stock.warehouse_id} onClick={() => adjust(1)}>{c.plus}</button></div>
      <button type="button" disabled={command.busy || command.canRetry} onClick={() => { setSelected(""); window.location.hash = "products"; }}>{c.products}</button>
    </div> : view.data?.offers.map(row => <article key={row.offer_id} className={s.card}><h2>{row.product_name}</h2><p>{row.keyword} · {row.variant_label} · {money(locale, view.data!.stats.currency, row.live_price_minor ?? row.sku_price_minor)}</p><p>{c.available} {row.stock.tracked ? row.stock.sellable : "—"}</p><button type="button" disabled={!canManage || command.blocked || !row.stock.tracked} onClick={() => { setSelected(row.offer_id); window.location.hash = `stock-${row.offer_id}`; }}>{c.stock}</button></article>)}
    <a href={`/${locale}/studio?store=${store}`}>{c.back}</a>
  </section>;
}
