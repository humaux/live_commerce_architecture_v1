// Purpose: Browse catalog summaries and review explicit bulk changes before submitting them.
// Depends on: react, next/link, next/navigation, @live-commerce/i18n, @live-commerce/ui, @/lib/model, @/lib/client, @/lib/customers-client, @/lib/catalog-v2-client, @/lib/catalog-v2-model, @/lib/catalog-v2-copy, @/lib/product-editor-copy, @/lib/product-document, @/lib/catalog-v2-write, @/lib/use-product-leave-guard, @/lib/images-client, ./WorkspaceFrame, ./AdminPageHeader, ./ProductPhoto, ./orders.css, ./ProductAdmin.css, ./ProductDocument.css, ./catalog/parity.css
// Used by: apps/admin/app/[locale]/products/page.tsx
// Invariants: product facts require a ready guarded read; selection replaces browse chrome; one Edit action per row; commands are fenced; archive preserves history; no inline editors.
"use client";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { catalogTaskCopy, catalogText, type Locale } from "@live-commerce/i18n";
import { TabStrip } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { command, readProducts } from "@/lib/catalog-v2-client";
import { parseCreated, type ProductStatus, type ProductSummary } from "@/lib/catalog-v2-model";
import { catalogCopy, catalogPresentationCopy } from "@/lib/catalog-v2-copy";
import { productEditorCopy } from "@/lib/product-editor-copy";
import { parseBulk, type BulkResult } from "@/lib/product-document";
import { useWrite } from "@/lib/catalog-v2-write";
import { useProductLeaveGuard } from "@/lib/use-product-leave-guard";
import { imageURL } from "@/lib/images-client";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { ProductPhoto } from "./ProductPhoto";
import "./orders.css";
import "./ProductAdmin.css";
import "./ProductDocument.css";
import "./catalog/parity.css";
const href = (locale: Locale, store: string, q: string, status: string, after: string) => {
  const p = new URLSearchParams();
  if (store) p.set('store', store);
  if (q) p.set('q', q);
  if (status && status !== 'all') p.set('status', status);
  if (after) p.set('after', after);
  return `/${locale}/products${p.size ? `?${p}` : ''}`;
};
/** Build the existing localized editor route. */
export const editHref = (locale: Locale, store: string, id: string) =>
  `/${locale}/products/${id}${store ? `?store=${store}` : ''}`;
/** Server summaries remain the source of row identity, money, stock and status. */
export function ProductList({ locale, store, q, status, after, initialError, renderKey }: {
  locale: Locale; stores: Store[]; store: Store | null; q: string; status: string; after: string;
  initialError: ReadCode | null; renderKey: string;
}) {
  const c = catalogTaskCopy[locale], pc = productEditorCopy[locale], legacy = catalogCopy[locale].list;
  const router = useRouter(), sid = store?.id ?? '';
  const [selected, setSelected] = useState<string[]>([]);
  const [review, setReview] = useState<ProductSummary[] | null>(null), [results, setResults] = useState<BulkResult[]>([]);
  const [resultRows, setResultRows] = useState<ProductSummary[]>([]);
  const previous = useRef<string[]>([]);
  const [copyReview, setCopyReview] = useState<ProductSummary | null>(null);
  const [copied, setCopied] = useState<{ id: string; name: string; source: string } | null>(null);
  const read = useGuardedRead(`${renderKey}|${locale}|${sid}|${q}|${status}|${after}`,
    store ? signal => readProducts(sid, q, status, after, signal) : null, initialError);
  const write = useWrite(sid, read.boundary, code => code in pc ? pc[code as keyof typeof pc] : pc.failed, pc.uncertain, pc.listRecoveryRequired);
  const locked = write.busy || write.message?.kind === 'uncertain';
  useProductLeaveGuard({ dirty: false, locked }, pc.leave);
  const page = read.data, rows = page?.items ?? [];
  useEffect(() => { setSelected([]); setReview(null); setResults([]); setResultRows([]); setCopyReview(null); setCopied(null); }, [sid]);
  useEffect(() => {
    // Conceal snapshots during revalidation; a denied scope must not resurrect them after retry.
    if (read.status === 'signed-out' || read.status === 'forbidden' || read.status === 'not-found') {
      setSelected([]); setReview(null); setResults([]); setResultRows([]); setCopyReview(null); setCopied(null);
    }
  }, [read.status]);
  const go = (nextStatus: string, nextQ = q, nextAfter = '') => {
    setSelected([]); setResults([]); setReview(null); setCopyReview(null); setCopied(null);
    router.push(href(locale, sid, nextQ, nextStatus, nextAfter));
  };
  const failure = read.status === 'signed-out' ? legacy.signedOut : read.status === 'forbidden' ? legacy.forbidden :
    read.status === 'not-found' ? legacy.noStore : read.status === 'unavailable' ? legacy.unavailable : '';
  const price = (r: ProductSummary) => r.price_min_minor === null ? legacy.noVariants :
    money(locale, r.currency, r.price_min_minor) + (r.price_max_minor !== null && r.price_max_minor !== r.price_min_minor ? ` – ${money(locale, r.currency, r.price_max_minor)}` : '');
  const stock = (r: ProductSummary) => !r.inventory_tracked ? pc.mixedTracking :
    r.available <= 0 ? c.soldOut : catalogText(c.available, { n: r.available });
  async function bulk(target: ProductStatus, items = rows.filter(r => selected.includes(r.id))) {
    if (read.status !== 'ready' || locked || !items.length) return;
    if (target === 'active' && items.some(r => !r.cover_image_id)) { write.fail(pc.imageRequired); return; }
    await write.run(command('POST', 'products/bulk-status', { ids: items.map(r => r.id), status: target }), parseBulk,
      async value => { setResultRows(items); setResults(value); setSelected([]); setReview(null); await read.refresh(); }, pc.success);
  }
  async function copySelected() {
    const row = copyReview;
    if (read.status !== 'ready' || locked || !row) return;
    // The returned identity/name belongs to the acknowledged new draft; never guess its id from list order.
    await write.run(command('POST', `products/${row.id}/copy`, { expected_version: row.version }), value => {
      const created = parseCreated(value);
      const name = (value as { name?: unknown }).name;
      if (typeof name !== 'string' || !name.trim() || Array.from(name).length > 120) throw new Error('invalid_response');
      return { ...created, name };
    }, async value => { setCopied({ ...value, source: row.name }); setCopyReview(null); setSelected([]); await read.refresh(); }, c.copySaved);
  }
  const archiveResult = results.length > 0 && results.every(r => r.status === 'archived' || !!r.error);
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? legacy.noStore} active="products" locked={locked}>
    <div className="orders-page product-admin pe-catalog cat-task-list" data-testid="products-page">
      {!copyReview && !copied && <AdminPageHeader locale={locale} title={c.title} actions={store && (!selected.length || locked) && <Link className="product-primary" data-testid="product-new" href={`/${locale}/products/new?store=${sid}`}>{legacy.newProduct}</Link>} />}
      {write.message && write.message.kind !== 'success' && <div role="status"><p>{write.message.text}</p>
        {write.canRetry && <button type="button" disabled={write.busy} onClick={() => void write.retry()}>{pc.retry}</button>}
      </div>}
      {!copyReview && !copied && <details className="cat-operations"><summary data-testid="products-operations"><span aria-hidden="true">{c.operations}</span></summary>
        <button type="button" data-testid="products-copy-entry" disabled={read.status !== 'ready' || locked || selected.length !== 1}
          onClick={() => { const row = rows.find(r => selected.includes(r.id)); if (row) { setCopied(null); setReview(null); setResults([]); setCopyReview(row); } }}>{c.copyEntry}</button>
        <Link data-testid="products-ledger-link" href={`/${locale}/inventory?store=${sid}`}>{legacy.inventory}</Link>
      </details>}
      {read.status === 'ready' && copyReview ? <section className="cat-task-panel" data-testid="product-copy-review">
        <h2>{c.copyTitle}</h2><p>{copyReview.name}</p><p>{c.copyKeep}</p><p>{c.copyExclude}</p>
        <div className="cat-actions cat-save-bar"><button type="button" className="product-primary" data-testid="products-copy-selected" disabled={locked} onClick={() => void copySelected()}>{c.copyConfirm}</button>
          <button type="button" disabled={locked} onClick={() => setCopyReview(null)}>{c.copyCancel}</button></div>
      </section> : read.status === 'ready' && copied ? <section className="cat-task-panel" data-testid="product-batch-results">
        <h2>{c.copySaved}</h2><p>{copied.source}</p><p>{copied.name}</p><p>{c.copyFacts}</p>
        <div className="cat-save-bar"><Link className="product-primary" data-testid="product-copy-edit" href={editHref(locale, sid, copied.id)}>{c.copyEdit}</Link></div>
      </section> : read.status === 'ready' && review ? <section className="cat-task-panel" aria-labelledby="archive-review-title">
        <h2 id="archive-review-title">{catalogText(c.archiveTitle, { n: review.length })}</h2>
        <p>{c.archivePolicy}</p>
        <ul>{review.map(r => <li key={r.id}><strong>{r.name}</strong><p>{price(r)} · {stock(r)}</p></li>)}</ul>
        <div className="cat-actions"><button type="button" disabled={locked} onClick={() => setReview(null)}>{c.backSelection}</button>
          <button type="button" className="product-primary" disabled={locked} onClick={() => void bulk('archived', review)}>{c.confirmArchive}</button></div>
      </section> : read.status === 'ready' && results.length ? <section className="cat-task-panel" data-testid="product-batch-results">
        <h2>{archiveResult ? catalogText(c.result, { saved: results.filter(r => !r.error).length, failed: results.filter(r => !!r.error).length }) : pc.results}</h2>
        <ul>{results.map((r, i) => {
          const row = page?.items.find(row => row.id === r.id) ?? resultRows.find(row => row.id === r.id) ?? resultRows[i];
          return <li key={r.id}><strong>{row ? `${row.name} · ${price(row)} · ${stock(row)}` : pc.success}</strong>
            <p>{r.error === 'live_window_open' ? c.live : r.error ? (r.error in pc ? pc[r.error as keyof typeof pc] : pc.failed) : r.status === 'archived' ? c.archived : r.status === 'draft' ? c.draft : pc.success}</p></li>;
        })}</ul>
        <button type="button" onClick={() => setResults([])}>{c.view}</button>
      </section> : <>
        {!selected.length && <>
        <div className="cat-chip-tabs"><TabStrip label={legacy.status} previousLabel={catalogPresentationCopy[locale].previousTabs} nextLabel={catalogPresentationCopy[locale].nextTabs}>
          {(['all', 'active', 'draft', 'archived'] as const).map(s => <button type="button" key={s} disabled={locked}
            data-testid={`products-tab-${s}`} aria-pressed={status === s} onClick={() => { previous.current = []; go(s); }}>
            {c[s]} {page ? s === 'all' ? page.total : page.status_counts[s] : ''}</button>)}
        </TabStrip></div>

        </>}
        {(read.status === 'loading' || read.status === 'hidden') && <p role="status">{legacy.loading}</p>}
        {failure && <div role="alert"><p>{failure}</p><button type="button" onClick={read.reload}>{legacy.retry}</button></div>}
        {read.status === 'ready' && page && <>
          <div className="cat-list-tools">
            <label className="pe-select cat-select-all"><span aria-hidden="true">{catalogText(selected.length ? c.selected : c.select, { n: selected.length || rows.length })}</span><input type="checkbox" data-testid="products-select-all" aria-label={catalogText(c.select, { n: rows.length })}
              checked={!!rows.length && rows.every(r => selected.includes(r.id))} disabled={locked || !rows.length}
              onChange={e => setSelected(e.target.checked ? rows.slice(0, 100).map(r => r.id) : [])} /></label>
            <div data-testid="product-batch" className="cat-actions">
              {(selected.length > 0 || locked) && <button type="button" disabled={locked} onClick={() => setReview(rows.filter(r => selected.includes(r.id)))}>{c.archive}</button>}
              {selected.length > 0 && <>
                <button type="button" disabled={locked} onClick={() => void bulk('active')}>{pc.publishSelected}</button>
                <button type="button" disabled={locked} onClick={() => void bulk('draft')}>{pc.unpublish}</button>
              </>}

            </div>
          </div>
          {rows.length ? <table data-testid="products-table" aria-label={c.title} className="cat-products-table"><thead className="cat-sr-only"><tr>
            <th scope="col" aria-label={pc.select} /><th scope="col" aria-label={legacy.product} /><th scope="col" aria-label={legacy.status} />
            <th scope="col" aria-label={`${legacy.price} · ${legacy.stock}`} /><th scope="col" aria-label={c.edit} />
          </tr></thead><tbody>{rows.map(row => <tr data-testid={`product-row-${row.id}`} key={row.id}>
            <td className="pe-selection-cell"><label className="pe-select"><input type="checkbox" aria-label={`${pc.select}: ${row.name}`} checked={selected.includes(row.id)} disabled={locked}
              onChange={() => setSelected(now => now.includes(row.id) ? now.filter(id => id !== row.id) : [...now, row.id].slice(0, 100))} /></label></td>
            <td className="pe-product-cell"><div className="product-cell">
              <ProductPhoto code="" name={row.name} demo={false} imageSrc={row.cover_image_id ? imageURL(sid, row.id, row.cover_image_id) : undefined} />
              <span className="pe-product-identity"><strong>{row.name}</strong></span></div></td>
            <td className="pe-status-cell"><span className="product-status">{c[row.status]}</span></td>
            <td className="pe-price-cell pe-stock-cell"><span>{price(row)} · {stock(row)}</span></td>
            <td className="pe-actions-cell"><div className="pe-row-actions"><Link href={editHref(locale, sid, row.id)}>{c.edit}</Link></div></td>
          </tr>)}</tbody></table> : <p role="status">{status === 'archived' ? c.empty : q || status !== 'all' ? legacy.emptySearch : legacy.empty}</p>}
          {(after || page.next_cursor) && <footer className="orders-pager">
            <button type="button" data-testid="products-previous" disabled={locked || (!after && !previous.current.length)} onClick={() => go(status, q, previous.current.pop() ?? '')}>{legacy.previous}</button>
            <button type="button" data-testid="products-next" disabled={locked || !page.next_cursor} onClick={() => { previous.current.push(after); go(status, q, page.next_cursor); }}>{legacy.next}</button>
          </footer>}
        </>}
      </>}
    </div>
  </WorkspaceFrame>;
}
