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
import { readOrderActions, readOrderListV2, readOrderDetail, readOrderDocument, OrderReadError, type OrderReadCode } from '@/lib/orders-client';
import { canonicalCursor, canonicalUUID, type OrderDetail } from '@/lib/orders-model';
import { ordersCopy } from '@/lib/orders-copy';
import { emptyFilters, type OrderSummaryV2 } from '@/lib/orders-v2';
import { readWorkspace } from '@/src/shell/api';
import { csrfCookie, sessionBoundary } from '@/lib/settings-client';
import { codCopy } from '@/lib/cod-copy';
import { OrderShipment } from '../OrderShipment';
import { amount } from '../OrderDetailPanel';
import { orderRevisionCopy, OrderRevisionAmounts, OrderRevisionLines, OrderRevisionRecipient } from '../orders/OrderRevisionSummary';
import revisionStyle from '../orders/order-revision.module.css';
import { parityCopy } from '../orders/parity-copy';
import { ShippingShell } from './ShippingShell';
import s from './shipping.module.css';

export type ShippingStep = 'select' | 'review' | 'preview' | 'done';
type Data = { store: Store; rows: OrderSummaryV2[]; details: OrderDetail[]; next: string; cursor: string; canShip: boolean; boundary: string };
const itemsText = (order: OrderDetail) => order.items.map(item => `${item.name}×${item.quantity}`).join(' · ');
const isHomeCOD = (order: OrderDetail) => order.payment_mode === 'cash_on_delivery' && order.destination.kind === 'home';
// Display-only short number; full server identities remain in links, scoped reads and URL selection.
const shortNumber = (id: string, rows: OrderSummaryV2[]) => `#${(rows.find(row => row.order_id === id)?.order_number ?? id.replaceAll('-', '')).slice(-5).toUpperCase()}`;

export function ShippingCenter({ locale, store, initialIDs, initialStep, initialCursor, initialError, initialRevision }: {
  locale: Locale; store: Store | null; initialIDs: string[]; initialStep: ShippingStep;
  initialCursor: string; initialError: OrderReadCode | null; initialRevision?: number;
}) {
  const c = shippingCopy[locale];
  const revisionCopy = orderRevisionCopy[locale];
  const [ids, setIDs] = useState(initialIDs), [step, setStep] = useState(initialStep);
  const [cursor, setCursor] = useState(initialCursor), [retry, setRetry] = useState(0);
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState<OrderReadCode | null>(initialError);
  const [printError, setPrintError] = useState(false);
  const [expired, setExpired] = useState(false);
  const [documents, setDocuments] = useState<OrderDetail[]>([]);
  const [documentBusy, setDocumentBusy] = useState(false);
  const [handingOffPrint, setHandingOffPrint] = useState(false);
  const [documentChanged, setDocumentChanged] = useState(false);
  const [revision, setRevision] = useState(initialRevision);
  const [enteringTracking, setEnteringTracking] = useState(false);

  useEffect(() => {
    const abort = new AbortController();
    let current = true;
    let concealed = false;
    const sessionCookie = csrfCookie();
    const clear = () => {
      current = false;
      abort.abort();
      flushSync(() => setData(null));
      setDocuments([]);
      setIDs([]);
      setStep('select');
      setRevision(undefined);
      setEnteringTracking(false);
      setExpired(true);
      setError('signed-out');
    };
    const conceal = () => {
      current = false;
      concealed = true;
      abort.abort();
      // Clear the rendered recipient snapshot before the browser captures a bfcache entry.
      flushSync(() => { setData(null); setEnteringTracking(false); });
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
          const [actions, boundary] = await Promise.all([
            details.some(isHomeCOD) ? readOrderActions(authenticated.id, abort.signal) : Promise.resolve(null), sessionBoundary(sessionCookie),
          ]);
          let pinnedDocument: OrderDetail | undefined;
          let pinnedChanged = false;
          if (revision !== undefined) {
            const order = details.find(row => row.order_id === ids[0]);
            pinnedChanged = !order || !isHomeCOD(order);
            if (order && isHomeCOD(order) && !(order.shipment && order.order_revision === revision)) {
              try { pinnedDocument = await readOrderDocument(authenticated.id, order.order_id, revision, abort.signal); }
              catch (failure) {
                if (failure instanceof Error && failure.message === 'version_changed') pinnedChanged = true;
                else throw failure;
              }
            }
          }
          if (current && csrfCookie() === sessionCookie) {
            if (revision !== undefined) {
              setDocuments(pinnedDocument ? [pinnedDocument] : []);
              setDocumentChanged(pinnedChanged);
            }
            setData({ store: authenticated, rows, details, next: page.next_cursor, cursor,
              canShip: actions?.fulfillment_write === true, boundary });
          }
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
      if (query.get('store') !== store?.id || query.has('revision')) { window.location.reload(); return; }
      const selected = query.get('selected')?.split(',') ?? [], nextStep = query.get('step') ?? 'select';
      const nextCursor = query.get('cursor') ?? '';
      if (selected.length > 100 || selected.some(id => !canonicalUUID.test(id)) ||
        new Set(selected).size !== selected.length || !['select', 'review', 'preview', 'done'].includes(nextStep) ||
        (nextCursor && !canonicalCursor.test(nextCursor))) { window.location.reload(); return; }
      setIDs(selected);
      setRevision(undefined);
      setEnteringTracking(false);
      setDocumentChanged(false);
      setStep(nextStep as ShippingStep);
      setDocuments([]);
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
    if (revision !== undefined && (next === 'preview' || next === 'done')) query.set('revision', String(revision));
    else setRevision(undefined);
    setEnteringTracking(false);
    window.history[replace ? 'replaceState' : 'pushState'](null, '', `/${locale}/orders/shipping?${query}`);
    setIDs(selected);
    setStep(next);
    setCursor(nextCursor);
    setPrintError(false);
    setDocumentChanged(false);
    if (next === 'select' || next === 'review') setDocuments([]);
  }
  const active = data?.cursor === cursor ? data : null;
  const selected = ids.map(id => active?.details.find(order => order.order_id === id)).filter((order): order is OrderDetail => !!order);
  const complete = !!active && ids.length > 0 && selected.length === ids.length;
  const revised = step === 'preview' && selected.length === 1 && isHomeCOD(selected[0]) &&
    (selected[0].order_revision ?? 0) > 1 ? selected[0] : undefined;
  // An amendment is refused once any shipment head exists. A persisted head belongs to the current order revision.
  const revisedReceipt = revised?.shipment ? revised : undefined;
  const pinnedStale = revision !== undefined && (documentChanged || !!active && selected[0]?.order_revision !== revision);
  const controlled = selected.filter(isHomeCOD);
  const validDocuments = controlled.every(order => documents.some(doc => doc.order_id === order.order_id &&
    doc.order_revision === order.order_revision));
  const dispatched = controlled.some(order => order.fulfillment_state === 'MERCHANT_SHIPPED');
  const staleCopy = locale === 'zh-TW' ? '出貨文件已變更。請丟棄舊文件並重新檢視。' :
    locale === 'zh-CN' ? '发货文件已变更。请丢弃旧文件并重新检查。' : 'The shipping document changed. Discard the old copy and review again.';
  const count = c.selected.replace('{n}', String(ids.length));
  const center = step === 'select';
  const number = (id: string) => shortNumber(id, active?.rows ?? []);
  const ordersHref = `/${locale}/orders${store ? `?store=${store.id}` : ''}`;
  const detailHref = (id: string) => `${ordersHref}&order=${id}`;
  const title = !pinnedStale && revisedReceipt ? revisionCopy.recorded :
    !pinnedStale && revised && validDocuments ? revisionCopy.preview.replace('{n}', String(revised.order_revision)) :
    center ? c.center : step === 'review' ? c.review : step === 'preview' ? c.preview : c.print;
  const summary = (order: OrderDetail) => `${number(order.order_id)} · ${order.items.map(item => `${item.name.replace(/\s*·\s*/g, ' · ')} × ${item.quantity}`).join(' · ')}`;
  const currentStore = active?.store ?? (expired || error === 'signed-out' ? null : store);
  const demo = !!active?.rows.length && active.rows.every(row => row.test_mode);
  const stateLabel = (order: OrderDetail) => order.work_state === 'REVIEW_REQUIRED' ? c.needsReview :
    order.work_state === 'READY' && order.fulfillment_state === 'MANUAL_UNASSIGNED' ? c.ready : ordersCopy[locale].statuses[order.commercial_state];
  const deliveryLabel = (order: OrderDetail) => ({ home: c.manual, cvs_711: c.seven, cvs_familymart: c.family,
    cvs_hilife: c.hilife, cvs_okmart: c.ok })[order.destination.kind];

  async function validateDocuments(orders: OrderDetail[]) {
    if (!active) throw new Error("unavailable");
    const token = csrfCookie();
    if (await sessionBoundary(token) !== active.boundary) throw new Error("unauthorized");
    return Promise.all(orders.map(order => {
      if (!order.order_revision) throw new Error("version_changed");
      return readOrderDocument(active.store.id, order.order_id, order.order_revision, AbortSignal.timeout(8000));
    }));
  }
  async function prepareDocuments() {
    if (!controlled.length) { navigate('preview'); return; }
    setDocumentBusy(true); setDocumentChanged(false);
    try {
      setDocuments(await validateDocuments(controlled));
      navigate('preview');
    } catch {
      setDocuments([]); setDocumentChanged(true); setRetry(value => value+1);
    } finally { setDocumentBusy(false); }
  }
  async function printDocuments() {
    setDocumentBusy(true);
    try {
      if (controlled.length) {
        if (!validDocuments) throw new Error("version_changed");
        const fresh = await validateDocuments(documents);
        flushSync(() => {setDocuments(fresh); setHandingOffPrint(true);});
      }
      window.print();
      navigate('done');
    } catch {
      if (controlled.length) { setDocuments([]); setDocumentChanged(true); setRetry(value => value+1); }
      else setPrintError(true);
    } finally {
      setHandingOffPrint(false); setDocumentBusy(false);
    }
  }

  async function refreshShipment(orderID: string) {
    if (!active) return false;
    try {
      const latest = await readOrderDetail(active.store.id, orderID, AbortSignal.timeout(8000));
      if (await sessionBoundary(csrfCookie()) !== active.boundary) return false;
      setData(previous => previous && {...previous, details: previous.details.map(row => row.order_id === latest.order_id ? latest : row)});
      setDocuments(previous => previous.flatMap(row => row.order_id !== latest.order_id ? [row] :
        row.order_revision === latest.order_revision ? [latest] : []));
      return true;
    } catch { return false; }
  }

  return <ShippingShell locale={locale} store={currentStore} demo={demo}>
    <title>{title}</title>
    <main className={`${s.task} ${revised ? s.revisedTask : ''}`} data-testid="shipping-center">
      <div className={`${s.heading} ${s.screenOnly}`}>
        <h1>{title}</h1>
        {center ? <a className={s.control} href={ordersHref}>{c.backOrders}</a> :
          revised || pinnedStale ? <a className={s.control} href={detailHref(ids[0])}>{revisionCopy.original}</a> :
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
              const mobileHref = isHomeCOD(order) ? `/${locale}/orders/shipping?${new URLSearchParams({
                store: active.store.id, selected: order.order_id, step: 'review',
              })}` : detailHref(row.order_id);
              return <div key={row.order_id} className={s.order} data-testid={`shipping-order-${row.order_id}`}>
                <label className={s.checkbox}>
                  <input type="checkbox" aria-label={number(row.order_id)} checked={ids.includes(row.order_id)}
                    disabled={!ids.includes(row.order_id) && ids.length >= 100}
                    onChange={() => navigate('select', ids.includes(row.order_id) ? ids.filter(id => id !== row.order_id) : [...ids, row.order_id], cursor, true)} />
                </label>
                <div className={s.orderText}><strong>{number(row.order_id)}</strong><span>{itemsText(order)}</span></div>
                <span className={`${s.status} ${order.work_state === 'REVIEW_REQUIRED' ? s.reviewStatus : ''}`}>{stateLabel(order)}</span>
                <a className={`${s.control} ${s.rowView}`} href={detailHref(row.order_id)}>{c.rowView}</a>
                <a className={s.mobileOrder} href={mobileHref}>
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
        </> : pinnedStale ? <div className={s.error} role="alert">
          <p>{staleCopy}</p><a className={s.control} href={detailHref(ids[0])}>{c.backOrders}</a>
        </div> : !complete ? <div role="alert"><p>{c.selectionUnavailable}</p>
          <button className={s.control} onClick={() => navigate('select', [])}>{c.back}</button>
        </div> : step === 'done' ? <section className={s.result}>
          <h2>{c.done}</h2><p className={s.notice}>{c.outcome}</p>
          <p>{selected.map(summary).join('\n')}</p>
          <div className={s.resultActions}>
            <button className={s.control} onClick={() => navigate('preview')}>{c.backPreview}</button>
            <button className={s.control} onClick={() => navigate('select')}>{c.backCenter}</button>
          </div>
        </section> : revisedReceipt ? <section className={`${revisionStyle.panel} ${revisionStyle.summary} ${s.screenOnly}`}>
          <strong>{number(revisedReceipt.order_id)} · v{revisedReceipt.order_revision}</strong>
          <p>{parityCopy[locale].handover}</p>
          <p>{codCopy[locale].orderStates[revisedReceipt.collection_state ?? '']}</p>
          <div className={revisionStyle.layout}>
            <OrderShipment store={active.store.id} detail={revisedReceipt} orderNumber={number(revisedReceipt.order_id)}
              locale={locale} c={ordersCopy[locale]} canWrite={active.canShip} boundary={active.boundary}
              onChanged={() => refreshShipment(revisedReceipt.order_id)} />
            <OrderRevisionAmounts locale={locale} currency={revisedReceipt.currency} totals={revisedReceipt.totals}
              fee={revisedReceipt.cod_surcharge_minor}
              due={revisedReceipt.collection_state === 'PENDING' ? revisedReceipt.cod_collect_minor : null} />
          </div>
          <div className={revisionStyle.actions}>
            <a className={revisionStyle.next} href={ordersHref}>{c.backOrders}</a>
          </div>
        </section> : <>
          {!revised && <h2 className={`${s.count} ${s.screenOnly}`}>{count}</h2>}
          <div className={s.selectedOrders} data-testid="shipping-selected-orders">
            {selected.map(current => {
              const controlled = isHomeCOD(current);
              const order = controlled && step === 'preview' ? documents.find(doc => doc.order_id === current.order_id &&
                doc.order_revision === current.order_revision) : current;
              if (!order) return null;
              return <article className={`${s.selectedOrder} ${revised ? s.revisedOrder : ''}`} key={order.order_id}>
              {!revised && <><strong className={s.screenOnly}>{number(order.order_id)}</strong>
              <p className={s.screenOnly}>{itemsText(order)}</p></>}
              {step === 'preview' && <section className={`${s.slip} ${revised ? s.revisedSlip : ''}`} data-testid="shipping-packing-slip" data-print-blocked={controlled && !handingOffPrint}>
                <strong className={revised ? s.printOnly : undefined}>{active.store.name}</strong><h3>{c.order} {number(order.order_id)}{revised && <span className={s.screenOnly}> · v{order.order_revision}</span>}</h3>
                {revised && <div className={s.screenOnly}>
                  <p className={`${s.notice} ${s.revisedNotice}`}>{revisionCopy.stale}</p>
                  <div className={revisionStyle.layout}>
                    <div>
                      <OrderRevisionLines items={order.items} locale={locale} currency={order.currency} />
                      <OrderRevisionRecipient locale={locale} address={{...order.destination.home_address,
                        recipient_name: order.destination.recipient_name, phone: order.destination.phone}} />
                    </div>
                    <OrderRevisionAmounts locale={locale} currency={order.currency} totals={order.totals}
                      fee={order.cod_surcharge_minor} due={order.cod_collect_minor} />
                  </div>
                </div>}
                <dl className={revised ? s.revisedPrintFacts : undefined}><div><dt>{c.recipient}</dt><dd>{order.destination.recipient_name}</dd></div>
                  <div><dt>{c.delivery}</dt><dd>{deliveryLabel(order)}</dd></div>
                  {controlled && <div><dt>v{order.order_revision}</dt><dd>{codCopy[locale].collectAmount} {amount(locale,order.currency,order.cod_collect_minor ?? 0)}</dd></div>}
                  <div className={s.printOnly}><dt>{c.address}</dt><dd>{order.destination.pickup ?
                    `${order.destination.pickup.name} · ${order.destination.pickup.code} · ${order.destination.pickup.address}` :
                    [order.destination.home_address.postal_code, order.destination.home_address.region, order.destination.home_address.city,
                      order.destination.home_address.line1, order.destination.home_address.line2,
                      order.destination.country].filter(Boolean).join(' ')}</dd></div>
                  <div className={s.printOnly}><dt>{c.phone}</dt><dd>{order.destination.phone}</dd></div>
                </dl>
                <p className={s.printOnly}>{itemsText(order)}</p>{!revised && <small className={s.screenOnly}>{c.source}</small>}
                {!revised && controlled && (order.order_revision ?? 0) > 1 && <p className={s.screenOnly}>{staleCopy}</p>}
              </section>}
              {step === 'preview' && controlled && active.canShip && (!revised || enteringTracking) && <div className={s.screenOnly}>
                <OrderShipment store={active.store.id} detail={order.work_state==='NONE' ? {...order,work_state:'READY'} : order} orderNumber={number(order.order_id)}
                  locale={locale} c={ordersCopy[locale]} canWrite={active.canShip} boundary={active.boundary}
                  inlineReview={!!revised} onChanged={() => refreshShipment(order.order_id)}/>
                {order.shipment && <p>{parityCopy[locale].handover} · {codCopy[locale].orderStates[order.collection_state ?? '']}</p>}
              </div>}
            </article>;
            })}
          </div>
          {(documentChanged || step === 'preview' && !validDocuments) && <p role="alert">{staleCopy}</p>}
          {!revised && <p className={`${s.notice} ${s.screenOnly}`}>{c.boundary}</p>}
          {printError && <p role="alert">{c.printFailed}</p>}
        </>}
      </div>
      {center ? <footer className={`${s.selectionBar} ${s.desktop}`}>
        <strong>{count}</strong><span>{selected.length ? selected.map(summary).join('；') : c.empty}</span>
        <button className={`${s.control} ${s.primary}`} disabled={!complete || !!error} onClick={() => navigate('review')}>{c.view}</button>
        <button className={s.control} onClick={() => navigate('select', [], cursor, true)}>{c.cancel}</button>
      </footer> : complete && !error && !pinnedStale && !revisedReceipt && step !== 'done' && <footer className={`${s.actions} ${s.screenOnly}`}>
        {revised && validDocuments && !dispatched ? <>
          {active?.canShip && !enteringTracking && <button className={`${s.control} ${s.primary}`} onClick={() => setEnteringTracking(true)}>{revisionCopy.fill}</button>}
          <button className={s.control} disabled={documentBusy} onClick={() => void printDocuments()}>{c.print}</button>
        </> : <>
        <button className={s.control} onClick={() => navigate('select')}>{c.cancel}</button>
        <button className={s.control} onClick={() => navigate('select')}>{c.edit}</button>
        {step === 'review' ? <button className={`${s.control} ${s.primary}`} disabled={documentBusy} onClick={() => void prepareDocuments()}>{c.confirm}</button> :
          validDocuments && !dispatched ? <button className={`${s.control} ${s.primary}`} disabled={documentBusy} onClick={() => void printDocuments()}>{c.print}</button> :
            !dispatched && <button className={s.control} onClick={() => navigate('review')}>{c.review}</button>}
        </>}
      </footer>}
    </main>
  </ShippingShell>;
}
