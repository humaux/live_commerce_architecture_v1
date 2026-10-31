// Purpose: Signed catalog edit, variant/stock checks, media, review and saved task screens.
// Depends on: react, next/link, @live-commerce/i18n, @/lib/model, @/lib/client, @/lib/images-client, @/lib/catalog-v2-model, @/lib/catalog-v2-client, @/lib/catalog-v2-write, @/lib/product-document, @/lib/product-editor-copy, @/lib/use-product-leave-guard, ./ProductMediaManager, ./catalog/parity.css.
// Used by: ProductEditor on the existing product route.
// Invariants: stable SKU IDs, minimal CAS patch, no write before review; server inventory floor/atomicity authoritative; UNKNOWN fenced.
"use client";
import { useCallback, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { catalogTaskCopy, catalogText, type Locale } from '@live-commerce/i18n';
import type { Store, ProductImageList } from '@/lib/model';
import { money } from '@/lib/client';
import { listImages } from '@/lib/images-client';
import { toMinor, parseCreated, type ProductDetail } from '@/lib/catalog-v2-model';
import { command, readProduct } from '@/lib/catalog-v2-client';
import { useWrite } from '@/lib/catalog-v2-write';
import { draftFromDetail, editDocument, type DraftRow } from '@/lib/product-document';
import { productEditorCopy } from '@/lib/product-editor-copy';
import type { ProductNavigationState } from '@/lib/use-product-leave-guard';
import { ProductMediaManager } from './ProductMediaManager';
import "./catalog/parity.css";
type Screen = 'edit' | 'variants' | 'stock' | 'media' | 'review' | 'saved';
/** Compose bounded task screens over the incumbent catalog write and media lifecycles. */
export function CatalogSimplifiedEditor({ locale, store, detail, boundary, onNavigationChange }: {
  locale: Locale; store: Store; detail: ProductDetail; boundary: string;
  onNavigationChange: (state: ProductNavigationState) => void;
}) {
  const c = catalogTaskCopy[locale], pc = productEditorCopy[locale];
  const [baseline, setBaseline] = useState(detail), [draft, setDraft] = useState(() => draftFromDetail(detail));
  const [screen, setScreen] = useState<Screen>('edit'), [stockIndex, setStockIndex] = useState(0);
  const [stockDraft, setStockDraft] = useState<DraftRow | null>(null), [stockError, setStockError] = useState('');
  const [media, setMedia] = useState<ProductImageList | null>(null), [mediaBusy, setMediaBusy] = useState(false);
  const trigger = useRef<HTMLButtonElement | null>(null), title = useRef<HTMLHeadingElement>(null), returningToStock = useRef(false);
  const write = useWrite(store.id, boundary, code => code in pc ? pc[code as keyof typeof pc] : pc.failed, pc.uncertain, pc.recoveryRequired);
  const locked = mediaBusy || write.busy || write.message?.kind === 'uncertain';
  const dirty = JSON.stringify(draft) !== JSON.stringify(draftFromDetail(baseline));
  useEffect(() => { onNavigationChange({ dirty, locked }); return () => onNavigationChange({ dirty: false, locked: false }); }, [dirty, locked, onNavigationChange]);
  useEffect(() => {
    if (screen === 'variants' && returningToStock.current) { returningToStock.current = false; trigger.current?.focus(); }
    else if (screen !== 'edit') title.current?.focus();
  }, [screen]);
  useEffect(() => {
    let alive = true;
    void listImages(store.id, detail.id).then(result => { if (alive) setMedia(result.list ?? null); });
    return () => { alive = false; };
  }, [store.id, detail.id]);
  const acceptMedia = useCallback((value: ProductImageList | null) => setMedia(value), []);
  const patchRow = (i: number, patch: Partial<DraftRow>) => setDraft(now => ({ ...now, rows: now.rows.map((r, j) => j === i ? { ...r, ...patch } : r) }));
  const text = (template: string, values: Record<string, string | number>) => catalogText(template, values);
  const commonPrice = draft.rows.every(r => r.price === draft.rows[0]?.price) ? draft.rows[0]?.price ?? '' : '';
  const counts = { main: media?.items.filter(p => p.role === 'main').length ?? 0, sku: media?.items.filter(p => p.role === 'sku').length ?? 0,
    detail: media?.items.filter(p => p.role === 'detail').length ?? 0, variants: (baseline.options.find(axis => axis.name === media?.image_axis) ?? baseline.options[0])?.values.length ?? 0 };
  const imageSummary = media ? text(c.imagesSummary, counts) : pc.loading;
  function openStock(i: number, button: HTMLButtonElement) {
    trigger.current = button; setStockIndex(i); setStockDraft({ ...draft.rows[i] }); setStockError(''); setScreen('stock');
  }
  function returnToVariants() { returningToStock.current = true; setScreen('variants'); setStockError(''); }
  function checkStock() {
    if (!stockDraft) return;
    const committed = baseline.skus[stockIndex]?.committed;
    if (stockDraft.tracked && (!/^\d+$/.test(stockDraft.quantity) || Number(stockDraft.quantity) > 1e9)) { setStockError(pc.invalid_stock); return; }
    if (stockDraft.tracked && committed !== null && committed !== undefined && Number(stockDraft.quantity) < committed) {
      setStockError(text(c.stockError, { target: stockDraft.quantity, committed })); return;
    }
    if (!stockDraft.tracked && !/^[1-9]\d{0,2}$/.test(stockDraft.max)) { setStockError(pc.invalid_stock); return; }
    patchRow(stockIndex, stockDraft); returnToVariants();
  }
  function review() {
    if (locked) return;
    try { editDocument(draft, baseline, store.currency); write.notice(''); setScreen('review'); }
    catch (error) { write.fail(error instanceof Error && error.message in pc ? pc[error.message as keyof typeof pc] : pc.failed); }
  }
  async function save() {
    if (locked) return;
    try {
      const patch = editDocument(draft, baseline, store.currency);
      if (Object.keys(patch).length === 1) { write.notice(pc.noChanges); return; }
      await write.run(command('PUT', `products/${baseline.id}/document`, patch), parseCreated, async () => {
        const fresh = await readProduct(store.id, baseline.id, new AbortController().signal);
        setBaseline(fresh); setDraft(draftFromDetail(fresh)); setScreen('saved');
      }, c.saved);
    } catch (error) { write.fail(error instanceof Error && error.message in pc ? pc[error.message as keyof typeof pc] : pc.failed); }
  }
  const skuFacts = (i: number) => {
    const r = draft.rows[i], old = baseline.skus.find(s => s.id === r.id);
    return `${r.values.join(' / ') || old?.title || old?.code} · ${old?.code ?? ''}`;
  };
  const rowStock = (i: number) => {
    const row = draft.rows[i], sku = baseline.skus.find(s => s.id === row.id);
    if (!row.tracked) return `${pc.untracked} · ${pc.max}: ${row.max}`;
    const quantity = Number(row.quantity), committed = sku?.committed ?? 0;
    return `${text(c.onHand, { n: quantity })} · ${text(c.committed, { n: committed })} · ${quantity - committed > 0 ? text(c.available, { n: quantity - committed }) : c.soldOut}`;
  };
  const summaries = <><p>{draft.name}</p><p>{draft.description}</p><p>{c[baseline.status]}</p>
    <ul className="cat-sku-summary">{draft.rows.map((r, i) => <li key={r.id}><p>{skuFacts(i)} · {money(locale, store.currency, toMinor(r.price, store.currency) ?? 0)} · {rowStock(i)}</p></li>)}</ul>
    <p>{imageSummary}</p></>;
  return <div className="cat-task-editor" data-testid="catalog-task-editor" onKeyDown={e => { if (e.key === 'Escape' && screen === 'stock' && !locked) { e.preventDefault(); returnToVariants(); } }}>
    {screen === 'edit' ? <form onSubmit={e => { e.preventDefault(); review(); }}>
      <fieldset disabled={locked} className="cat-task-fields">
        <label>{c.name}<input data-testid="product-name" value={draft.name} maxLength={120} onChange={e => setDraft({ ...draft, name: e.target.value })} /></label>
        <label>{c.desc}<textarea aria-label={c.desc} data-testid="product-description" value={draft.description} maxLength={8000} rows={3} onChange={e => setDraft({ ...draft, description: e.target.value })} /></label>
        <label>{text(draft.rows.length === 2 ? c.price : draft.rows.length > 2 ? c.multiPrice : c.onePrice, { currency: store.currency, n: draft.rows.length })}<input data-testid="catalog-common-price" inputMode="decimal" value={commonPrice}
          onChange={e => setDraft({ ...draft, rows: draft.rows.map(row => ({ ...row, price: e.target.value })) })} /></label>
        {!commonPrice && draft.rows.length > 1 && <p>{c.noPriceChange}</p>}
        <label className="cat-status-radio"><input type="radio" name="catalog-status" aria-label={c[baseline.status]} checked readOnly />{c[baseline.status]}</label>
      </fieldset>
      <div className="cat-editor-links"><button type="button" disabled={locked} onClick={() => setScreen('variants')}>{c.variants}</button>
        <button type="button" disabled={locked} onClick={() => setScreen('media')}>{media ? media.items.length ? c.media : c.mediaNone : pc.loading}</button></div>
      <div className="cat-actions cat-save-bar" data-testid="catalog-save-bar"><Link role="button" href={`/${locale}/products?store=${store.id}`}>{c.cancel}</Link>
        <button className="product-primary" type="submit" disabled={locked}>{c.review}</button></div>
    </form> : screen === 'variants' ? <section>
      <h2 ref={title} tabIndex={-1}>{c.variants}</h2>
      <ul className="cat-variants">{draft.rows.map((r, i) => <li key={r.id} data-testid={`catalog-variant-${i}`}>
        <details><summary>{skuFacts(i)} · {money(locale, store.currency, toMinor(r.price, store.currency) ?? 0)} · {rowStock(i)}</summary><label>{text(c.onePrice, { currency: store.currency })}
          <input aria-label={`${text(c.onePrice, { currency: store.currency })}: ${r.values.join(' / ') || baseline.skus[i]?.code}`} data-testid={`catalog-price-${i}`} inputMode="decimal" value={r.price} disabled={locked} onChange={e => patchRow(i, { price: e.target.value })} /></label></details>
        <button type="button" data-testid={`catalog-stock-${i}`} ref={i === stockIndex ? trigger : undefined} disabled={locked || !baseline.warehouse_id} onClick={e => openStock(i, e.currentTarget)}>{c.stock}</button>
      </li>)}</ul>
      {!baseline.warehouse_id && <p>{pc.warehouse_required} <Link href={`/${locale}/inventory?store=${store.id}`}>{pc.inventoryLink}</Link></p>}
      <button type="button" disabled={locked} onClick={() => setScreen('edit')}>{c.backEditor}</button>
    </section> : screen === 'stock' && stockDraft ? <section>
      <h2 ref={title} tabIndex={-1}>{c.stock}</h2>
      <p>{skuFacts(stockIndex)}</p><p>{rowStock(stockIndex)}</p>
      <fieldset className="cat-task-fields" disabled={locked}>
        <label><input data-testid="catalog-stock-tracked" type="checkbox" checked={stockDraft.tracked} onChange={e => setStockDraft({ ...stockDraft, tracked: e.target.checked })} />{c.tracked}</label>
        {stockDraft.tracked ? <><label>{c.total}<input data-testid="catalog-stock-total" inputMode="numeric" value={stockDraft.quantity} onChange={e => setStockDraft({ ...stockDraft, quantity: e.target.value })} /></label>
          <label>{c.delta}<input data-testid="catalog-stock-delta" inputMode="numeric" value={Number(stockDraft.quantity) - (baseline.skus[stockIndex]?.on_hand ?? 0)} onChange={e => { if (/^[-+]?\d+$/.test(e.target.value)) setStockDraft({ ...stockDraft, quantity: String((baseline.skus[stockIndex]?.on_hand ?? 0) + Number(e.target.value)) }); }} /></label></> :
          <label>{pc.max}<input data-testid="catalog-stock-max" inputMode="numeric" value={stockDraft.max} onChange={e => setStockDraft({ ...stockDraft, max: e.target.value })} /></label>}
        <p>{c.reason}</p>
        {stockDraft.tracked && <p className="cat-stock-floor">{text(c.stockFloor, { n: baseline.skus[stockIndex]?.committed ?? 0 })}</p>}
      </fieldset>
      {stockError && <p role="alert">{stockError}</p>}
      <div className="cat-actions cat-save-bar"><button type="button" disabled={locked} onClick={returnToVariants}>{stockError ? c.keep : c.backVariants}</button>
        <button type="button" disabled={locked} onClick={checkStock}>{c.checkStock}</button></div>
    </section> : screen === 'media' ? <section>
      <h2 ref={title} tabIndex={-1}>{c.media}</h2>
      <ProductMediaManager compact locale={locale} store={store.id} productID={baseline.id} boundary={boundary} axes={baseline.options} disabled={write.busy}
        onChanged={acceptMedia} onLocked={setMediaBusy} />
      <button type="button" disabled={mediaBusy} onClick={() => setScreen('edit')}>{c.backEditor}</button>
    </section> : <section>
      <h2 ref={title} tabIndex={-1}>{screen === 'saved' ? c.saved : c.reviewTitle}</h2>
      {summaries}
      <p>{c.unchangedOrders}</p>
      <div className="cat-actions">{screen === 'saved' ? <Link href={`/${locale}/products?store=${store.id}`}>{c.view}</Link> : <>
        <button type="button" disabled={locked} onClick={() => setScreen('edit')}>{c.continueEdit}</button>
        <button className="product-primary" type="button" data-testid="catalog-confirm-save" disabled={locked} onClick={() => void save()}>{c.confirm}</button></>}</div>
    </section>}
    {write.message?.text && screen !== 'saved' && <div role="alert"><p>{write.message.text}</p>
      {write.canRetry && <button type="button" disabled={write.busy} onClick={() => void write.retry()}>{pc.retry}</button>}</div>}
  </div>;
}
