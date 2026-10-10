// Purpose: Reuses active catalog search, SKU selection and controlled quantities across manual-order forms.
// Depends on: react, @live-commerce/i18n, @live-commerce/ui, catalog-v2-client/model, client money, merchant-tools-copy, manual-order-form, OperationalForms.module.css.
// Used by: ManualOrderFormFields.
// Invariants: I01, I05, I08; catalog prices are display-only and never submitted or totalled here.
"use client";
import { useEffect, useMemo, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Field, FormRow } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { readProduct, readProducts } from "@/lib/catalog-v2-client";
import type { ProductDetail, ProductSummary } from "@/lib/catalog-v2-model";
import type { ManualFormLine } from "@/lib/manual-order-form";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import s from "./OperationalForms.module.css";

// Product search (active products only: a draft cannot hold stock) and variant pick. Reads are the existing catalog BFF routes.
/** Renders catalog picks and controlled item quantities; reads only the existing catalog API. */
export function ManualOrderItemPicker({ locale, store, lines, setLines, idPrefix, quantityControls }: {
  locale: Locale; store: Store; lines: ManualFormLine[]; setLines: (next: ManualFormLine[]) => void;
  idPrefix: string; quantityControls: "input" | "stepper";
}) {
  const c = toolsCopy[locale].manual;
  const [q, setQ] = useState("");
  const [results, setResults] = useState<ProductSummary[] | null>(null);
  const [open, setOpen] = useState<ProductDetail | null>(null);
  const [searching, setSearching] = useState(false);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), []);
  async function search() {
    controller.current?.abort();
    const active = new AbortController();
    controller.current = active;
    setSearching(true);
    setOpen(null);
    try {
      const page = await readProducts(store.id, q.trim(), "active", "", active.signal);
      if (!active.signal.aborted) setResults(page.items);
    } catch {
      if (!active.signal.aborted) setResults([]);
    }
    if (!active.signal.aborted) setSearching(false);
  }
  async function expand(id: string) {
    controller.current?.abort();
    const active = new AbortController();
    controller.current = active;
    try {
      const detail = await readProduct(store.id, id, active.signal);
      if (!active.signal.aborted) setOpen(detail);
    } catch {
      /* the list stays; the merchant can retry */
    }
  }
  const add = (v: ProductDetail["skus"][number], product: ProductDetail) => {
    if (lines.some((l) => l.sku_id === v.id)) return;
    setLines([...lines, { sku_id: v.id, quantity: 1, label: `${product.name} · ${v.title}`, code: v.code, price: money(locale, v.currency, v.price_minor) }]);
  };
  const ordered = useMemo(() => lines, [lines]);
  function setQuantity(sku: string, quantity: number) {
    const bounded = Math.max(0, Math.min(1000, Math.trunc(quantity)));
    setLines(bounded === 0 ? lines.filter((line) => line.sku_id !== sku)
      : lines.map((line) => line.sku_id === sku ? { ...line, quantity: bounded } : line));
  }
  return (
    <section className="mt-card" data-testid={`${idPrefix}-items`}>
      <h2>{c.itemsTitle}</h2>
      {/* Not a <form>: this picker lives inside the order form, and HTML forbids nested forms (the parser would drop the inner one). */}
      <FormRow className={s.searchRow}>
        <Field id={`${idPrefix}-search`} label={c.search} width="long">
          <input id={`${idPrefix}-search`} data-testid={`${idPrefix}-search`} value={q} maxLength={120} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); void search(); } }} />
        </Field>
      </FormRow>
      <div className={`mt-actions ${s.searchActions}`}><button type="button" data-testid={`${idPrefix}-search-button`} disabled={searching} onClick={() => void search()}>{c.searchButton}</button></div>
      {results && (results.length === 0 ? <p className="mt-note">{c.noResults}</p> : (
        <ul className="mt-results">
          {results.map((p) => (
            <li key={p.id}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                <span>{p.name}<small>{p.sku_count} · {c.available} {p.available}</small></span>
                <button type="button" className="mt-btn" onClick={() => void expand(p.id)}>{c.add}</button>
              </div>
              {open?.id === p.id && (
                <ul className="mt-variants">
                  {open.skus.filter((v) => v.price_minor >= 0).map((v) => (
                    <li key={v.id}>
                      <span>{v.title}<small>{v.code} · {money(locale, v.currency, v.price_minor)} · {c.available} {v.available}</small></span>
                      <button type="button" className="mt-btn" disabled={lines.some((l) => l.sku_id === v.id)} onClick={() => add(v, open)}>{c.add}</button>
                    </li>
                  ))}
                </ul>
              )}
            </li>
          ))}
        </ul>
      ))}
      {ordered.length === 0 ? <p className="mt-note">{c.noLines}</p> : (
        <ul className="mt-lines" data-testid={`${idPrefix}-lines`}>
          {ordered.map((l) => (
            <li key={l.sku_id}>
              <span>{l.label}<small>{l.code} · {l.price}</small>{l.note && <small>{l.note}</small>}</span>
              {quantityControls === "stepper" ? (
                <div className="mt-quantity-stepper">
                  <button type="button" className="mt-btn" data-testid={`${idPrefix}-minus-${l.sku_id}`}
                    aria-label={`${c.quantity} − · ${l.label}`} onClick={() => setQuantity(l.sku_id, l.quantity - 1)}>−</button>
                  <input type="number" min={0} max={1000} step={1} aria-label={`${c.quantity} · ${l.label}`}
                    data-testid={`${idPrefix}-quantity-${l.sku_id}`} value={l.quantity}
                    onChange={(e) => setQuantity(l.sku_id, Number(e.target.value) || 0)} />
                  <button type="button" className="mt-btn" data-testid={`${idPrefix}-plus-${l.sku_id}`}
                    aria-label={`${c.quantity} + · ${l.label}`} disabled={l.quantity >= 1000}
                    onClick={() => setQuantity(l.sku_id, l.quantity + 1)}>+</button>
                </div>
              ) : (
                <input type="number" min={1} max={1000} step={1} aria-label={c.quantity} value={l.quantity}
                  onChange={(e) => setLines(lines.map((x) => (x.sku_id === l.sku_id ? { ...x, quantity: Math.max(1, Math.min(1000, Math.trunc(Number(e.target.value) || 1))) } : x)))} />
              )}
              <button type="button" className="mt-btn" onClick={() => setLines(lines.filter((x) => x.sku_id !== l.sku_id))}>{c.remove}</button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
