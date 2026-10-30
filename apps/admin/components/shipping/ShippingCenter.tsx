// Purpose: Read-only shipping selection, review, per-order packing slips and native browser printing.
// Depends on: React/react-dom, i18n, settings-client, orders-client/model/copy/v2, shell/api, model, ShippingShell and shipping.module.css.
// Used by: apps/admin/app/[locale]/orders/shipping/page.tsx; tests/admin/shipping-center.spec.ts.
// Invariants: only validated scoped reads; URL stores identities/step only; failed reads expose no partial slips;
//   print never purchases labels, records shipments or claims carrier handover; session teardown clears all recipient data.
'use client';
import { useEffect, useState } from 'react';
import { flushSync } from 'react-dom';
import { shippingCopy, type Locale } from '@live-commerce/i18n';
import type { Store } from '@/lib/model';
import { readOrderListV2, readOrderDetail, OrderReadError, type OrderReadCode } from '@/lib/orders-client';
import { canonicalCursor, canonicalUUID, type OrderDetail } from '@/lib/orders-model';
import { ordersCopy } from '@/lib/orders-copy';
import { emptyFilters, type OrderSummaryV2 } from '@/lib/orders-v2';
import { readWorkspace } from '@/src/shell/api';
import { csrfCookie } from '@/lib/settings-client';
import { ShippingShell } from './ShippingShell';
import s from './shipping.module.css';

export type ShippingStep = 'select' | 'review' | 'preview' | 'done';
type Data = { store: Store; rows: OrderSummaryV2[]; details: OrderDetail[]; next: string; cursor: string };
const itemsText = (order: OrderDetail) => order.items.map(item => `${item.name}×${item.quantity}`).join(' · ');
// Display-only short number; full server identities remain in links, scoped reads and URL selection.
const shortNumber = (id: string, rows: OrderSummaryV2[]) => `#${(rows.find(row => row.order_id === id)?.order_number ?? id.replaceAll('-', '')).slice(-5).toUpperCase()}`;

export function ShippingCenter({ locale, store, initialIDs, initialStep, initialCursor, initialError }: {
  locale: Locale; store: Store | null; initialIDs: string[]; initialStep: ShippingStep;
  initialCursor: string; initialError: OrderReadCode | null;
}) {
  const c = shippingCopy[locale];
  const [ids, setIDs] = useState(initialIDs), [step, setStep] = useState(initialStep);
  const [cursor, setCursor] = useState(initialCursor), [retry, setRetry] = useState(0);
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState<OrderReadCode | null>(initialError);
  const [printError, setPrintError] = useState(false);
  const [expired, setExpired] = useState(false);

  useEffect(() => {
    const abort = new AbortController();
    let current = true;
    let concealed = false;
    const sessionCookie = csrfCookie();
    const clear = () => {
      current = false;
      abort.abort();
      flushSync(() => setData(null));
      setIDs([]);
      setStep('select');
      setExpired(true);
      setError('signed-out');
    };
    const conceal = () => {
      current = false;
      concealed = true;
      abort.abort();
      // Clear the rendered recipient snapshot before the browser captures a bfcache entry.
      flushSync(() => setData(null));
    };
    const reveal = () => {
      if (!current && !concealed) return;
      if (csrfCookie() !== sessionCookie) { clear(); return; }
      if (concealed) {
        concealed = false;
        setRetry(value => value + 1);
      }
    };
    const visibility = () => document.visibilityState === 'hidden' ? conceal() : reveal();
    const storage = (event: StorageEvent) => { if (event.key === 'commerce-session-logout') clear(); };
    window.addEventListener('commerce-session-logout', clear);
    window.addEventListener('storage', storage);
    document.addEventListener('visibilitychange', visibility);
    window.addEventListener('pagehide', conceal);
    window.addEventListener('pageshow', reveal);
    window.addEventListener('focus', reveal);
    let channel: BroadcastChannel | null = null;
    try {
      channel = new BroadcastChannel('commerce-session');
      channel.onmessage = event => { if (event.data?.type === 'logout') clear(); };
    } catch { /* The same-tab and storage events also clear this read-only view. */ }
    setData(null);
    if (!expired && !initialError && store) {
      setError(null);
      void (async () => {
        try {
          const stores = await readWorkspace(abort.signal);
          const authenticated = stores.find(item => item.id === store.id);
          if (!authenticated) throw new OrderReadError('forbidden');
          // The server's "unshipped" filter excludes REVIEW_REQUIRED. The signed center must retain those
          // orders for document review without implying that they are authorized to ship.
          const page = await readOrderListV2(authenticated.id, 'active', cursor, abort.signal, emptyFilters);
          const rows = page.items.filter(row => row.fulfillment_state === 'MANUAL_UNASSIGNED' &&
            (row.commercial_state === 'CONFIRMED' || row.commercial_state === 'AWAITING_COLLECTION'));
          const requested = [...new Set([...rows.map(row => row.order_id), ...ids])];
          // Bounded batches keep large restored selections from opening 100 concurrent detail requests.
          const details: OrderDetail[] = [];
          for (let offset = 0; offset < requested.length; offset += 10) {
            details.push(...await Promise.all(requested.slice(offset, offset + 10).map(id => readOrderDetail(authenticated.id, id, abort.signal))));
          }
          if (current && !sessionCookie) throw new OrderReadError('signed-out');
          if (current && csrfCookie() !== sessionCookie) { clear(); return; }
          if (current) setData({ store: authenticated, rows, details, next: page.next_cursor, cursor });
        } catch (failure) {
          if (!current) return;
          abort.abort();
          setData(null);
          setError(failure instanceof OrderReadError ? failure.code :
            failure instanceof Error && failure.message === 'workspace_session_expired' ? 'signed-out' : 'unavailable');
        }
      })();
    }
    return () => {
      current = false;
      abort.abort();
      channel?.close();
      window.removeEventListener('commerce-session-logout', clear);
      window.removeEventListener('storage', storage);
      document.removeEventListener('visibilitychange', visibility);
      window.removeEventListener('pagehide', conceal);
      window.removeEventListener('pageshow', reveal);
      window.removeEventListener('focus', reveal);
    };
    // Selection changes use already validated page details. Paging/retry/popstate revalidate the entire snapshot.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [store, initialError, cursor, retry, expired]);

  useEffect(() => {
    const restore = () => {
      const query = new URLSearchParams(window.location.search);
      if (query.get('store') !== store?.id) { window.location.reload(); return; }
      const selected = query.get('selected')?.split(',') ?? [], nextStep = query.get('step') ?? 'select';
      const nextCursor = query.get('cursor') ?? '';
      if (selected.length > 100 || selected.some(id => !canonicalUUID.test(id)) ||
        new Set(selected).size !== selected.length || !['select', 'review', 'preview', 'done'].includes(nextStep) ||
        (nextCursor && !canonicalCursor.test(nextCursor))) { window.location.reload(); return; }
      setIDs(selected);
      setStep(nextStep as ShippingStep);
      setCursor(nextCursor);
      setRetry(value => value + 1);
      setPrintError(false);
    };
    window.addEventListener('popstate', restore);
    return () => window.removeEventListener('popstate', restore);
  }, [store?.id]);

  function navigate(next: ShippingStep, selected = ids, nextCursor = cursor, replace = false) {
    if (!store || expired) return;
    const query = new URLSearchParams({ store: store.id });
    if (selected.length) query.set('selected', selected.join(','));
    if (next !== 'select') query.set('step', next);
    if (nextCursor) query.set('cursor', nextCursor);
    window.history[replace ? 'replaceState' : 'pushState'](null, '', `/${locale}/orders/shipping?${query}`);
    setIDs(selected);
    setStep(next);
    setCursor(nextCursor);
    setPrintError(false);
  }
  const active = data?.cursor === cursor ? data : null;
  const selected = ids.map(id => active?.details.find(order => order.order_id === id)).filter((order): order is OrderDetail => !!order);
  const complete = !!active && ids.length > 0 && selected.length === ids.length;
  const count = c.selected.replace('{n}', String(ids.length));
  const center = step === 'select';
  const number = (id: string) => shortNumber(id, active?.rows ?? []);
  const ordersHref = `/${locale}/orders${store ? `?store=${store.id}` : ''}`;
  const detailHref = (id: string) => `${ordersHref}&order=${id}`;
  const title = center ? c.center : step === 'review' ? c.review : step === 'preview' ? c.preview : c.print;
  const summary = (order: OrderDetail) => `${number(order.order_id)} · ${order.items.map(item => `${item.name.replace(/\s*·\s*/g, ' · ')} × ${item.quantity}`).join(' · ')}`;
  const currentStore = active?.store ?? (expired || error === 'signed-out' ? null : store);
  const demo = !!active?.rows.length && active.rows.every(row => row.test_mode);
  const stateLabel = (order: OrderDetail) => order.work_state === 'REVIEW_REQUIRED' ? c.needsReview :
    order.work_state === 'READY' && order.fulfillment_state === 'MANUAL_UNASSIGNED' ? c.ready : ordersCopy[locale].statuses[order.commercial_state];
  const deliveryLabel = (order: OrderDetail) => ({ home: c.manual, cvs_711: c.seven, cvs_familymart: c.family,
    cvs_hilife: c.hilife, cvs_okmart: c.ok })[order.destination.kind];

  return <ShippingShell locale={locale} store={currentStore} demo={demo}>
    <title>{title}</title>
    <main className={s.task} data-testid="shipping-center">
      <div className={`${s.heading} ${s.screenOnly}`}>
        <h1>{title}</h1>
        {center ? <a className={s.control} href={ordersHref}>{c.backOrders}</a> :
          step !== 'done' && <button className={s.control} onClick={() => navigate('select')}>{c.back}</button>}
      </div>
      {center && <>
        <p className={`${s.notice} ${s.desktop}`}>{c.note}</p>
        <p className={`${s.notice} ${s.mobile}`}>{c.mobile}</p>
      </>}
      <div className={`${s.viewport} ${center ? s.centerViewport : ''}`}>
        {error ? <div className={s.error} role="alert">
          <p>{error === 'signed-out' ? c.signedOut : error === 'forbidden' ? c.forbidden : c.unavailable}</p>
          {error === 'signed-out' ? <a className={s.control} href={`/${locale}/`}>{c.signIn}</a> :
            <button className={s.control} onClick={() => initialError ? window.location.reload() : setRetry(value => value + 1)}>{c.retry}</button>}
        </div> : !store ? <p>{c.noOrders}</p> : !active ? <p role="status">{c.loading}</p> : center ? <>
          <div className={s.rows}>
            {active.rows.map(row => {
              const order = active.details.find(detail => detail.order_id === row.order_id)!;
              return <div key={row.order_id} className={s.order} data-testid={`shipping-order-${row.order_id}`}>
                <label className={s.checkbox}>
                  <input type="checkbox" aria-label={number(row.order_id)} checked={ids.includes(row.order_id)}
                    disabled={!ids.includes(row.order_id) && ids.length >= 100}
                    onChange={() => navigate('select', ids.includes(row.order_id) ? ids.filter(id => id !== row.order_id) : [...ids, row.order_id], cursor, true)} />
                </label>
                <div className={s.orderText}><strong>{number(row.order_id)}</strong><span>{itemsText(order)}</span></div>
                <span className={`${s.status} ${order.work_state === 'REVIEW_REQUIRED' ? s.reviewStatus : ''}`}>{stateLabel(order)}</span>
                <a className={`${s.control} ${s.rowView}`} href={detailHref(row.order_id)}>{c.rowView}</a>
                <a className={s.mobileOrder} href={detailHref(row.order_id)}>
                  <span>{`${number(row.order_id)}\n${itemsText(order)}\n${stateLabel(order)}`}</span>
                  <svg aria-hidden="true" viewBox="0 0 8 14"><path d="m1 1 6 6-6 6" /></svg>
                </a>
              </div>;
            })}
          </div>
          {!active.rows.length && <p>{c.noOrders}</p>}
          {(cursor || active.next) && <nav className={s.paging}>
            {cursor && <button className={s.control} onClick={() => navigate('select', ids, '')}>{c.firstPage}</button>}
            {active.next && <button className={s.control} onClick={() => navigate('select', ids, active.next)}>{c.nextPage}</button>}
          </nav>}
        </> : !complete ? <div role="alert"><p>{c.selectionUnavailable}</p>
          <button className={s.control} onClick={() => navigate('select', [])}>{c.back}</button>
        </div> : step === 'done' ? <section className={s.result}>
          <h2>{c.done}</h2><p className={s.notice}>{c.outcome}</p>
          <p>{selected.map(summary).join('\n')}</p>
          <div className={s.resultActions}>
            <button className={s.control} onClick={() => navigate('preview')}>{c.backPreview}</button>
            <button className={s.control} onClick={() => navigate('select')}>{c.backCenter}</button>
          </div>
        </section> : <>
          <h2 className={`${s.count} ${s.screenOnly}`}>{count}</h2>
          <div className={s.selectedOrders} data-testid="shipping-selected-orders">
            {selected.map(order => <article className={s.selectedOrder} key={order.order_id}>
              <strong className={s.screenOnly}>{number(order.order_id)}</strong>
              <p className={s.screenOnly}>{itemsText(order)}</p>
              {step === 'preview' && <section className={s.slip} data-testid="shipping-packing-slip">
                <strong>{active.store.name}</strong><h3>{c.order} {number(order.order_id)}</h3>
                <dl><div><dt>{c.recipient}</dt><dd>{order.destination.recipient_name}</dd></div>
                  <div><dt>{c.delivery}</dt><dd>{deliveryLabel(order)}</dd></div>
                  <div className={s.printOnly}><dt>{c.address}</dt><dd>{order.destination.pickup ?
                    `${order.destination.pickup.name} · ${order.destination.pickup.code} · ${order.destination.pickup.address}` :
                    [order.destination.home_address.postal_code, order.destination.home_address.region, order.destination.home_address.city,
                      order.destination.home_address.line1, order.destination.home_address.line2].filter(Boolean).join(' ')}</dd></div>
                  <div className={s.printOnly}><dt>{c.phone}</dt><dd>{order.destination.phone}</dd></div>
                </dl>
                <p className={s.printOnly}>{itemsText(order)}</p><small className={s.screenOnly}>{c.source}</small>
              </section>}
            </article>)}
          </div>
          <p className={`${s.notice} ${s.screenOnly}`}>{c.boundary}</p>
          {printError && <p role="alert">{c.printFailed}</p>}
        </>}
      </div>
      {center ? <footer className={`${s.selectionBar} ${s.desktop}`}>
        <strong>{count}</strong><span>{selected.length ? selected.map(summary).join('；') : c.empty}</span>
        <button className={`${s.control} ${s.primary}`} disabled={!complete || !!error} onClick={() => navigate('review')}>{c.view}</button>
        <button className={s.control} onClick={() => navigate('select', [], cursor, true)}>{c.cancel}</button>
      </footer> : complete && !error && step !== 'done' && <footer className={`${s.actions} ${s.screenOnly}`}>
        <button className={s.control} onClick={() => navigate('select')}>{c.cancel}</button>
        <button className={s.control} onClick={() => navigate('select')}>{c.edit}</button>
        {step === 'review' ? <button className={`${s.control} ${s.primary}`} onClick={() => navigate('preview')}>{c.confirm}</button> :
          <button className={`${s.control} ${s.primary}`} onClick={() => {
            try { window.print(); navigate('done'); } catch { setPrintError(true); }
          }}>{c.print}</button>}
      </footer>}
    </main>
  </ShippingShell>;
}
