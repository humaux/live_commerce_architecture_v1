// Purpose: Review and save a scoped pre-dispatch home COD amendment.
// Depends on: React, orders-client and OrderRevisionSummary.
// Used by: OrderDetailView.
// Invariants: Quote reads do not write. A retry of an uncertain save uses the same request and key.

"use client";

import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { CatalogProduct, CatalogSKU } from "@/lib/claims-model";
import { amendmentProducts, amendmentSKUs, putOrderAmendment, reviewOrderAmendment } from "@/lib/orders-client";
import type { OrderDetail } from "@/lib/orders-model";
import type { Sections } from "../OrderDetailPanel";
import { orderRevisionCopy, OrderRevisionAmounts, OrderRevisionLines, OrderRevisionRecipient, type RevisionLine } from "./OrderRevisionSummary";
import s from "./order-revision.module.css";
import { shortOrderNumber } from "./parity-copy";

const words = {
  "zh-TW": { edit: "修改原單", heading: "發貨前變更", add: "加購商品", search: "搜尋商品", product: "商品", sku: "規格", qty: "數量", remove: "移除", name: "收件人", phone: "收件電話", region: "縣市", city: "區域", postal_code: "郵遞區號", line1: "地址", line2: "地址補充", review: "檢視變更", confirm: "確認並儲存", cancel: "取消", current: "目前版本", stale: "訂單或報價已變更。請重新檢視。", stock: "庫存不足。請調整加購數量。", hold: "此訂單暫不能修改。請先查核目前狀態。", uncertain: "結果未確認。可以用相同資料重試。", error: "暫時無法處理。" },
  "zh-CN": { edit: "修改原单", heading: "发货前变更", add: "加购商品", search: "搜索商品", product: "商品", sku: "规格", qty: "数量", remove: "移除", name: "收件人", phone: "收件电话", region: "县市", city: "区域", postal_code: "邮政编码", line1: "地址", line2: "地址补充", review: "检查变更", confirm: "确认并保存", cancel: "取消", current: "当前版本", stale: "订单或报价已变更。请重新检查。", stock: "库存不足。请调整加购数量。", hold: "此订单暂不能修改。请先核查当前状态。", uncertain: "结果未确认。可以用相同数据重试。", error: "暂时无法处理。" },
  en: { edit: "Amend original order", heading: "Changes before dispatch", add: "Add items", search: "Search products", product: "Product", sku: "Variant", qty: "Quantity", remove: "Remove", name: "Recipient", phone: "Phone", region: "Region", city: "City", postal_code: "Postal code", line1: "Address", line2: "Address line 2", review: "Review changes", confirm: "Confirm and save", cancel: "Cancel", current: "Current revision", stale: "The order or quote changed. Review again.", stock: "Stock is insufficient. Adjust the quantities.", hold: "This order is on hold. Check its current status.", uncertain: "The result is unconfirmed. Retry the same request.", error: "Temporarily unavailable." },
};

type DraftAddition = { sku_id: string; quantity: number; title: string };
const recipient = (order: OrderDetail) => ({
  recipient_name: order.destination.recipient_name, phone: order.destination.phone, ...order.destination.home_address,
});
type RecipientField = keyof ReturnType<typeof recipient>;
type Quote = { proposal: string; subtotal: number; shipping: number; tax: number; fee: number; due: number;
  items: RevisionLine[] };

function quoteResponse(value: unknown, order: OrderDetail): Quote {
  const row = value as Record<string, unknown> | null;
  const totals = row?.totals as Record<string, unknown> | undefined;
  const values = [totals?.subtotal_minor, totals?.shipping_minor, totals?.tax_minor, row?.cod_surcharge_minor, row?.cod_collect_minor];
  if (row?.order_id !== order.order_id || row.order_revision !== order.order_revision ||
    typeof row.proposal !== "string" || !/^[a-f0-9]{64}$/.test(row.proposal) ||
    !values.every(value => Number.isSafeInteger(value) && (value as number) >= 0) ||
    !Array.isArray(row.items) || !row.items.every(item => item && typeof item.name === "string" && typeof item.code === "string" &&
      Number.isSafeInteger(item.quantity) && item.quantity > 0 &&
      Number.isSafeInteger(item.unit_price_minor) && item.unit_price_minor >= 0 &&
      Number.isSafeInteger(item.amount?.total_minor) && item.amount.total_minor >= 0)) throw new Error("unavailable");
  const [subtotal, shipping, tax, fee, due] = values as number[];
  return { proposal: row.proposal, subtotal, shipping, tax, fee, due, items: row.items };
}

export function OrderAmendment({ order, sections, locale, canAdd, onEditing, onSaved }: {
  order: OrderDetail; sections: Sections; locale: Locale; canAdd: boolean; onEditing: (editing: boolean) => void; onSaved: (revision: number) => void;
}) {
  const c = words[locale];
  const [editing, setEditing] = useState(false);
  const [address, setAddress] = useState(() => recipient(order));
  const [additions, setAdditions] = useState<DraftAddition[]>([]);
  const [query, setQuery] = useState("");
  const [products, setProducts] = useState<CatalogProduct[]>([]);
  const [cursor, setCursor] = useState("");
  const [variants, setVariants] = useState<CatalogSKU[]>([]);
  const [sku, setSKU] = useState("");
  const [quantity, setQuantity] = useState(1);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [pending, setPending] = useState<{ key: string; body: string; expectedRevision: number } | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (quote) {
      heading.current?.focus({preventScroll:true});
      window.scrollTo({top:0});
    }
  }, [quote]);
  const changed = Object.fromEntries(Object.entries(address).filter(([field, value]) => recipient(order)[field as RecipientField] !== value));
  const draft = { expected_revision: order.order_revision, ...(Object.keys(changed).length ? {recipient: changed} : {}),
    ...(additions.length ? {additions: additions.map(({sku_id, quantity}) => ({sku_id, quantity}))} : {}) };
  const field = (key: RecipientField, label: string, maxLength: number) =>
    <label>{label}<input name={key} value={address[key]} maxLength={maxLength}
      onChange={event => setAddress({...address, [key]: event.target.value})} /></label>;

  async function loadProducts(next = "") {
    setBusy(true); setMessage("");
    if (!next) { setProducts([]); setVariants([]); setSKU(""); setCursor(""); }
    try {
      const page = await amendmentProducts(sections.store, next, AbortSignal.timeout(8000));
      const match = query.trim().toLocaleLowerCase();
      const items = page.items.filter(product => product.name.toLocaleLowerCase().includes(match));
      setProducts(previous => next ? [...previous, ...items] : items);
      setCursor(page.next_cursor);
    } catch { setMessage(c.error); } finally { setBusy(false); }
  }

  function dismiss() {
    setEditing(false);
    onEditing(false);
    setAddress(recipient(order));
    setAdditions([]);
    setQuote(null);
    setMessage("");
  }
  async function inspect() {
    setBusy(true); setMessage("");
    try {
      setQuote(quoteResponse(await reviewOrderAmendment(sections.store, order.order_id, draft, AbortSignal.timeout(8000)), order));
    } catch (error) {
      const code = error instanceof Error ? error.message : "";
      setMessage(code === "state_changed" ? c.hold : code === "version_changed" ? c.stale : c.error);
      if (code === "version_changed") await sections.onChanged();
    } finally { setBusy(false); }
  }
  async function save() {
    if (!quote || !order.order_revision || busy) return;
    const request = pending ?? {
      key: `amend-${crypto.randomUUID()}`, body: JSON.stringify({...draft, proposal: quote.proposal}),
      expectedRevision: order.order_revision,
    };
    setBusy(true); setMessage("");
    try {
      const result = await putOrderAmendment(sections.store, order.order_id, request.key, request.body, sections.boundary);
      if (result.ok) {
        setPending(null); dismiss();
        if (await sections.onChanged()) onSaved(request.expectedRevision + 1);
        else setMessage(c.error);
      } else if (result.uncertain) {
        setPending(request); setMessage(c.uncertain);
      } else {
        setPending(null); setQuote(null);
        setMessage(result.code === "version_changed" ? c.stale : result.code === "insufficient_inventory" ? c.stock : result.code === "state_changed" ? c.hold : c.error);
        if (result.code === "version_changed") {
          setEditing(false);
          onEditing(false);
          await sections.onChanged();
        }
      }
    } finally { setBusy(false); }
  }

  return <section className={quote ? s.panel : "signed-order-panel"} data-testid="order-amendment">
    <h2 ref={heading} tabIndex={-1}>{quote ? orderRevisionCopy[locale].review : c.heading}</h2>
    <p>{quote && `${shortOrderNumber(order.order_id)} · `}{c.current} v{order.order_revision}</p>
    {message && <p role="status">{message}</p>}
    {!editing ? <button type="button" onClick={() => {setAddress(recipient(order)); setMessage(""); setEditing(true); onEditing(true);}}>{c.edit}</button> :
      quote ? <>
        <div className={s.layout}>
          <div>
            <OrderRevisionLines items={quote.items} locale={locale} currency={order.currency} />
            <OrderRevisionRecipient locale={locale} address={address} />
          </div>
          <OrderRevisionAmounts locale={locale} currency={order.currency} totals={{
            subtotal_minor: quote.subtotal, shipping_minor: quote.shipping, tax_minor: quote.tax,
          }} fee={quote.fee} due={quote.due} />
        </div>
        <div className={s.actions}>
          <button type="button" className="primary" onClick={() => void save()} disabled={busy}>{c.confirm}</button>
          <button type="button" onClick={dismiss} disabled={busy || pending !== null}>{c.cancel}</button>
        </div>
      </> : <div className="orders-form">
        {field("recipient_name", c.name, 120)}{field("phone", c.phone, 32)}{field("region", c.region, 100)}
        {field("city", c.city, 100)}{field("postal_code", c.postal_code, 20)}{field("line1", c.line1, 200)}{field("line2", c.line2, 200)}
        {canAdd && <fieldset disabled={busy}>
          <legend>{c.add}</legend>
          <label>{c.search}<input value={query} onChange={event => setQuery(event.target.value)} /></label>
          <button type="button" onClick={() => void loadProducts()}>{c.search}</button>
          {cursor && <button type="button" onClick={() => void loadProducts(cursor)}>{locale === "zh-TW" ? "搜尋下一頁" : locale === "zh-CN" ? "搜索下一页" : "Search next page"}</button>}
          <label>{c.product}<select aria-label={c.product} defaultValue="" onChange={async event => {
            setSKU(""); setVariants([]);
            if (!event.target.value) return;
            setBusy(true);
            try { setVariants(await amendmentSKUs(sections.store, event.target.value, order.currency, AbortSignal.timeout(8000))); }
            catch { setMessage(c.error); } finally { setBusy(false); }
          }}><option value="">—</option>{products.map(product => <option key={product.id} value={product.id}>{product.name}</option>)}</select></label>
          <label>{c.sku}<select aria-label={c.sku} value={sku} onChange={event => setSKU(event.target.value)}><option value="">—</option>
            {variants.map(variant => <option value={variant.id} key={variant.id}>{variant.code}</option>)}</select></label>
          <label>{c.qty}<input type="number" min="1" max="999999999" value={quantity} onChange={event => setQuantity(Number(event.target.value))} /></label>
          <button type="button" disabled={!sku || !Number.isSafeInteger(quantity) || quantity < 1 || quantity > 999999999 || additions.some(item => item.sku_id === sku)}
            onClick={() => setAdditions([...additions, { sku_id: sku, quantity, title: variants.find(variant => variant.id === sku)?.code ?? "" }])}>{c.add}</button>
          <ul>{additions.map(item => <li key={item.sku_id}>{item.title} × {item.quantity} <button type="button"
            onClick={() => setAdditions(additions.filter(candidate => candidate !== item))}>{c.remove}</button></li>)}</ul>
        </fieldset>}
        <button type="button" className="primary" onClick={() => void inspect()} disabled={busy || (!Object.keys(changed).length && !additions.length)}>{c.review}</button>
        <button type="button" onClick={dismiss} disabled={busy}>{c.cancel}</button>
      </div>}
  </section>;
}
