// Purpose: Owns order list loading, filtering, bulk selection and selected-order detail.
// Depends on: react, react-dom, next/navigation, @live-commerce/i18n, @/lib/model, @/lib/settings-client, @/lib/orders-client, @/lib/orders-model, @/lib/orders-copy, @/lib/cod-copy, @/lib/orders-v2, @/lib/orders-v2-copy, ./OrderListFilters, ./WorkspaceFrame, ./AdminPageHeader, @live-commerce/ui, @/lib/presentation-copy, ./OrderDetailPanel, ./Icon, ./orders.css, ./order-actions.css, ./orders-v2.css
// Used by: apps/admin/app/[locale]/orders/page.tsx
"use client";

// Purpose: scoped merchant order list, existing inline details and fulfillment action entry points.
// Depends on: orders BFF/read models, WorkspaceFrame, PickList/picklist-model/copy and TrackingImport's independent CSV workflow.
// Used by: /[locale]/orders; existing controls keep their placement and permission semantics.
// Merchant orders page (approved C inline row). BFF: GET /api/stores/{store}/orders[/{id}] and order-actions
// -> Go internal/httpapi/orders.go + shipments.go. The refund and shipment sections live in OrderRefunds /
// OrderShipment (their BFF routes are listed there); the export button is a plain GET download of
// orders/unshipped.csv streamed by the BFF from Go, never fetched into JS memory. The CVS section (OrderCvsShipment:
// BFF orders/{id}/cvs-shipment*, collection, pay-at-pickup-release -> Go internal/httpapi/cvs.go) sits next to the
// 0063 section in the same inline row; the list filter `cvs_pending` is one more state in the existing filter.
// Live feel (ops-polish OP2): while the tab is visible the first page is re-read from the same list route every POLL_MS; ids not seen
// before get a "new" marker and the tab title a count. No new route, no websocket, no notification API.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import {
  exportUnshippedHref,
  readOrderActions,
  readOrderDetail,
  readOrderListV2,
  OrderReadError,
  type OrderReadCode,
} from "@/lib/orders-client";
import {
  displayTime,
  orderStates,
  type OrderActions,
  type OrderDetail,
  type OrderFilter,
  type OrderSummary,
} from "@/lib/orders-model";
import { ordersCopy, type OrdersCopy } from "@/lib/orders-copy";
import { codCopy } from "@/lib/cod-copy";
import { emptyFilters, appendOrderFilters, buckets, type OrderFilters, type OrderListV2, type OrderSummaryV2 } from "@/lib/orders-v2";
import { ordersV2Copy } from "@/lib/orders-v2-copy";
import { OrderListFilters } from "./OrderListFilters";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { TabStrip } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import { amount, badge, detailPanel, type Sections } from "./OrderDetailPanel";
import { Icon } from "./Icon";
import { PickList } from "./PickList";
import { selectOrders } from "@/lib/picklist-model";
import { picklistCopy } from "@/lib/picklist-copy";
import { TrackingImport } from "./TrackingImport";
import "./orders.css";
import "./order-actions.css";
import "./orders-v2.css";

const POLL_MS = 20_000;
const noActions: OrderActions = { refund: false, fulfillment_write: false, orders_export: false };

type Status = "initial" | "loading" | "ready" | "hidden" | OrderReadCode;
type View = {
  key: string;
  status: Status;
  page: OrderListV2 | null;
  detail: OrderDetail | null;
  detailStatus: Status;
};

function url(
  locale: Locale,
  store: string,
  state: OrderFilter,
  cursor: string,
  order: string,
  filters: OrderFilters,
) {
  const params = new URLSearchParams();
  if (store) params.set("store", store);
  if (state !== "active") params.set("state", state);
  if (cursor && !filters.q) params.set("cursor", cursor);
  if (order) params.set("order", order);
  appendOrderFilters(params, filters);
  return `/${locale}/orders${params.size ? `?${params}` : ""}`;
}

/** Owns order list loading, filtering and selected-order detail. Loads and refreshes orders through orders-client; section components own action commands. */
export function MerchantOrders({
  locale,
  store,
  state,
  filters: routeFilters,
  order,
  cursor: routeCursor,
  initialError,
  renderKey,
}: {
  locale: Locale;
  store: Store | null;
  state: OrderFilter;
  filters: OrderFilters;
  order: string;
  cursor: string;
  initialError: OrderReadCode | null;
  renderKey: string;
}) {
  const c = ordersCopy[locale];
  const pc = picklistCopy[locale];
  const bulkScope = `${store?.id ?? ""}:${csrfCookie()}`;
  // Keep only identifiers and delivery eligibility, never recipient data, across page navigation.
  const [bulk, setBulk] = useState<{scope:string; rows:Record<string,boolean>}>({scope:"",rows:{}});
  const bulkRows = bulk.scope === bulkScope ? bulk.rows : {};
  const selectedIDs = Object.keys(bulkRows);
  function checkRows(rows:OrderSummaryV2[], checked:boolean) {
    setBulk(current => {
      const retained = current.scope === bulkScope ? current.rows : {};
      try {
        const ids = selectOrders(Object.keys(retained), rows.map(r=>r.order_id), checked);
        const types = new Map(rows.map(r=>[r.order_id,r.delivery_kind.startsWith("cvs_")]));
        return {scope:bulkScope,rows:Object.fromEntries(ids.map(id=>[id,types.get(id) ?? retained[id] ?? false]))};
      } catch { return current; }
    });
  }
  const v2 = ordersV2Copy[locale];
  // Search text may identify a buyer. Memory only: never URL/history/storage.
  const [search, setSearch] = useState({ store: store?.id ?? "", value: "", cursor: "" });
  const cursor = search.store === store?.id && search.value ? search.cursor : routeCursor;
  const filters = useMemo(() => ({ ...routeFilters, q: search.store === store?.id ? search.value : "" }), [routeFilters, search, store?.id]);
  const filterKey = JSON.stringify(filters);
  const router = useRouter();
  const [refresh, setRefresh] = useState(0);
  const key = `${renderKey}|${locale}|${store?.id ?? ""}|${state}|${filterKey}|${cursor}|${order}|${initialError ?? ""}|${refresh}`;
  const [view, setView] = useState<View>({
    key: "",
    status: "initial",
    page: null,
    detail: null,
    detailStatus: "initial",
  });
  const [actions, setActions] = useState<OrderActions | null>(null);
  const generation = useRef(0);
  const controller = useRef<AbortController | null>(null);
  const hidden = useRef(false);
  const blocked = useRef(false);
  const session = useRef("");
  const cookie = useRef("");
  const previous = useRef<string[]>([]);
  // OP2: ids already shown for this store+filter, and the ones that arrived after. Order ids only, never PII.
  const seen = useRef<{ scope: string; ids: Set<string> } | null>(null);
  const polling = useRef(false);
  const [fresh, setFresh] = useState<ReadonlySet<string>>(new Set());
  const current =
    (blocked.current && ["signed-out", "forbidden", "not-found"].includes(view.status)) ||
    (view.key === key && (!cookie.current || csrfCookie() === cookie.current))
      ? view
      : {
          key,
          status: "initial" as Status,
          page: null,
          detail: null,
          detailStatus: "initial" as Status,
        };

  const clear = useCallback(
    (status: Status, block = false) => {
      generation.current++;
      controller.current?.abort();
      cookie.current = "";
      if (status === "signed-out" || status === "forbidden" || status === "not-found") setBulk({scope:"", rows:{}});
      if (status === "signed-out" || status === "forbidden" || status === "not-found") setSearch({ store: "", value: "", cursor: "" });
      if (block) {
        blocked.current = true;
        seen.current = null;
        setFresh(new Set());
      }
      // A hidden document can enter bfcache before effects run; remove PII now.
      flushSync(() =>
        setView({
          key,
          status,
          page: null,
          detail: null,
          detailStatus: status,
        }),
      );
    },
    [key],
  );

  // Marks rows not seen before in this store+filter scope (first page only; later pages are not "newest"). A scope change starts clean.
  const noteSeen = useCallback(
    (page: OrderListV2) => {
      if (!store || cursor) return;
      const scope = `${store.id}|${state}|${filterKey}`;
      if (seen.current?.scope !== scope) {
        seen.current = { scope, ids: new Set(page.items.map((row) => row.order_id)) };
        setFresh(new Set());
        return;
      }
      const known = seen.current.ids;
      const added = page.items.filter((row) => !known.has(row.order_id)).map((row) => row.order_id);
      if (!added.length) return;
      added.forEach((id) => known.add(id));
      setFresh((old) => new Set([...old, ...added]));
    },
    [store, state, cursor, filterKey],
  );

  const load = useCallback(async () => {
    if (hidden.current || blocked.current) return;
    if (initialError || !store) {
      setView({
        key,
        status: initialError ?? "not-found",
        page: null,
        detail: null,
        detailStatus: "initial",
      });
      return;
    }
    const epoch = ++generation.current;
    controller.current?.abort();
    const active = new AbortController();
    controller.current = active;
    setView({
      key,
      status: "loading",
      page: null,
      detail: null,
      detailStatus: "initial",
    });
    const stillCurrent = async (boundary: string) => {
      if (
        generation.current !== epoch ||
        active.signal.aborted ||
        hidden.current
      )
        return false;
      const latest = await sessionBoundary();
      return (
        generation.current === epoch &&
        !active.signal.aborted &&
        !hidden.current &&
        latest === boundary
      );
    };
    try {
      const boundary = await sessionBoundary();
      if (
        generation.current !== epoch ||
        hidden.current ||
        active.signal.aborted
      )
        return;
      const page = await readOrderListV2(store.id, state, cursor, active.signal, filters);
      if (!(await stillCurrent(boundary)))
        throw new OrderReadError("signed-out");
      session.current = boundary; // The cookie is a change fence, not authority; BFF just authorized this read.
      cookie.current = csrfCookie();
      const selected =
        order && page.items.some((row) => row.order_id === order);
      noteSeen(page);
      setView({
        key,
        status: "ready",
        page,
        detail: null,
        detailStatus: selected ? "loading" : order ? "not-found" : "initial",
      });
      if (!selected) return;
      try {
        const detail = await readOrderDetail(store.id, order, active.signal);
        if (!(await stillCurrent(boundary)))
          throw new OrderReadError("signed-out");
        setView({ key, status: "ready", page, detail, detailStatus: "ready" });
      } catch (error) {
        if (active.signal.aborted || generation.current !== epoch) return;
        const code =
          error instanceof OrderReadError ? error.code : "unavailable";
        if (code === "signed-out" || code === "forbidden") {
          blocked.current = code === "signed-out";
          setView({
            key,
            status: code,
            page: null,
            detail: null,
            detailStatus: code,
          });
        } else
          setView({
            key,
            status: "ready",
            page,
            detail: null,
            detailStatus: code,
          });
      }
    } catch (error) {
      if (active.signal.aborted || generation.current !== epoch) return;
      const code = error instanceof OrderReadError ? error.code : "unavailable";
      if (code === "signed-out") blocked.current = true;
      setView({
        key,
        status: code,
        page: null,
        detail: null,
        detailStatus: code,
      });
    }
  }, [key, initialError, store, state, cursor, order, noteSeen, filters]);

  // Permission probe for the action buttons only; every write is re-authorized by Go. A failed probe hides actions.
  useEffect(() => {
    if (!store || initialError) {
      setActions(null);
      return;
    }
    const active = new AbortController();
    readOrderActions(store.id, active.signal).then(setActions, () => {
      if (!active.signal.aborted) setActions(noActions);
    });
    return () => active.abort();
  }, [store, initialError, refresh]);

  // After a refund/shipment response: re-GET list + selected detail from the server (no optimistic state).
  // If the order no longer matches the filter (e.g. just shipped under "ready to ship") the old page is kept.
  const reload = useCallback(async () => {
    if (!store || !order || hidden.current || blocked.current || !session.current) return false;
    const epoch = generation.current;
    const boundary = session.current;
    const signal = controller.current?.signal ?? new AbortController().signal;
    try {
      const [page, detail] = await Promise.all([
        readOrderListV2(store.id, state, cursor, signal, filters),
        readOrderDetail(store.id, order, signal),
      ]);
      if (
        generation.current !== epoch ||
        hidden.current ||
        signal.aborted ||
        (await sessionBoundary()) !== boundary
      )
        return false;
      setView((previous) =>
        previous.key === key
          ? {
              ...previous,
              page: page.items.some((row) => row.order_id === order) ? page : previous.page,
              detail,
              detailStatus: "ready",
            }
          : previous,
      );
      return true;
    } catch (error) {
      if (generation.current === epoch && !signal.aborted && error instanceof OrderReadError && error.code !== "unavailable") clear(error.code, true);
      return false;
    }
  }, [key, store, state, cursor, order, filters, clear]);

  // ponytail: polling; switch to SSE once more than ~50 merchant tabs hold this page open at once.
  // Same guards as reload(): never while hidden/blocked, never overlapping, dropped if the session or load epoch changed.
  const poll = useCallback(async () => {
    if (!store || hidden.current || blocked.current || !session.current || polling.current) return;
    polling.current = true;
    const epoch = generation.current;
    const boundary = session.current;
    const signal = controller.current?.signal ?? new AbortController().signal;
    try {
      const page = await readOrderListV2(store.id, state, "", signal, filters);
      if (
        generation.current !== epoch ||
        hidden.current ||
        signal.aborted ||
        (await sessionBoundary()) !== boundary
      )
        return;
      // Later pages still probe authority, but never replace their rows with page one.
      if (cursor) return;
      noteSeen(page);
      // Only the rows change: filters, scroll position, the open detail and the cursor stack stay as the merchant left them.
      setView((old) => (old.key === key && old.status === "ready" && old.page ? { ...old, page } : old));
    } catch (error) {
      // Transient transport failure can retry; an authoritative denial clears PII immediately.
      if (generation.current === epoch && !signal.aborted && error instanceof OrderReadError && error.code !== "unavailable") clear(error.code, true);
    } finally {
      polling.current = false;
    }
  }, [key, store, state, cursor, noteSeen, filters, clear]);
  useEffect(() => {
    const timer = window.setInterval(() => void poll(), POLL_MS);
    return () => window.clearInterval(timer);
  }, [poll]);
  useEffect(() => {
    if (!fresh.size) return;
    const base = document.title;
    // orders.title: the tab badge names this page, not the static app/layout title
    // (lib/copy.ts "Commerce workspace"); restoring `base` on cleanup keeps the
    // layout title intact when the badge count drops back to zero.
    document.title = `(${fresh.size}) ${c.title}`;
    return () => {
      document.title = base;
    };
  }, [fresh.size, c.title]);

  useEffect(() => {
    void load();
    return () => {
      generation.current++;
      controller.current?.abort();
    };
  }, [load]);
  useEffect(() => {
    const conceal = () => {
      hidden.current = true;
      session.current = "";
      clear("hidden");
    };
    const reveal = () => {
      if (!hidden.current) return;
      hidden.current = false;
      if (!blocked.current) void load();
    };
    const visibility = () =>
      document.visibilityState === "hidden" ? conceal() : reveal();
    const onMessage = (event: MessageEvent) => {
      if (event.data?.type === "logout") clear("signed-out", true);
    };
    const onStorage = (event: StorageEvent) => {
      if (event.key === "commerce-session-logout") clear("signed-out", true);
    };
    const onLocalLogout = () => clear("signed-out", true);
    const onHistory = () => clear("loading");
    const onFocus = () => {
      if (hidden.current) {
        reveal();
        return;
      }
      if (!session.current) return;
      void sessionBoundary()
        .then((value) => {
          if (session.current && value !== session.current)
            clear("signed-out", true);
        })
        .catch(() => clear("signed-out", true));
    };
    let channel: BroadcastChannel | null = null;
    try {
      channel = new BroadcastChannel("commerce-session");
      channel.addEventListener("message", onMessage);
    } catch {
      /* storage event still works */
    }
    document.addEventListener("visibilitychange", visibility);
    window.addEventListener("pagehide", conceal);
    window.addEventListener("pageshow", reveal);
    window.addEventListener("storage", onStorage);
    window.addEventListener("commerce-session-logout", onLocalLogout);
    window.addEventListener("popstate", onHistory);
    window.addEventListener("focus", onFocus);
    if (document.visibilityState === "hidden") conceal();
    return () => {
      document.removeEventListener("visibilitychange", visibility);
      window.removeEventListener("pagehide", conceal);
      window.removeEventListener("pageshow", reveal);
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("commerce-session-logout", onLocalLogout);
      window.removeEventListener("popstate", onHistory);
      window.removeEventListener("focus", onFocus);
      channel?.removeEventListener("message", onMessage);
      channel?.close();
    };
  }, [clear, load]);

  function navigate(
    nextStore: string,
    nextState: OrderFilter,
    nextCursor: string,
    nextOrder: string,
    nextFilters: OrderFilters = filters,
  ) {
    clear("loading");
    const searchChanged = search.store !== nextStore || search.value !== nextFilters.q || search.cursor !== nextCursor;
    setSearch(old => old.store === nextStore && old.value === nextFilters.q && old.cursor === nextCursor ? old : { store: nextStore, value: nextFilters.q, cursor: nextCursor });
    const destination = url(locale, nextStore, nextState, nextCursor, nextOrder, nextFilters);
    if (destination !== window.location.pathname + window.location.search) router.push(destination);
    else if (!searchChanged) setRefresh(value => value + 1);
  }
  function choose(row: OrderSummary) {
    if (fresh.has(row.order_id)) setFresh((old) => new Set([...old].filter((id) => id !== row.order_id)));
    navigate(
      store?.id ?? "",
      state,
      cursor,
      order === row.order_id ? "" : row.order_id,
    );
  }
  function retry() {
    if (current.status === "loading") return;
    blocked.current = false;
    clear("loading");
    if (initialError || !store) router.refresh();
    else setRefresh((value) => value + 1);
  }
  function next() {
    if (!current.page?.next_cursor || current.status !== "ready") return;
    previous.current.push(cursor);
    navigate(store?.id ?? "", state, current.page.next_cursor, "");
  }
  function back() {
    if (!previous.current.length || current.status !== "ready") return;
    navigate(store?.id ?? "", state, previous.current.pop() ?? "", "");
  }
  const message = (status: Status) =>
    status === "signed-out"
      ? c.signedOut
      : status === "forbidden"
        ? c.forbidden
        : status === "not-found"
          ? c.notFound
          : status === "unavailable"
            ? c.unavailable
            : status === "loading"
              ? c.loading
              : "";
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.noStore}
      active="orders"
    >
      <div className="orders-page orders-v2" data-testid="merchant-orders">
        <AdminPageHeader locale={locale} description={c.subtitle} />
        {!["hidden", "signed-out", "forbidden", "not-found"].includes(current.status) && <OrderListFilters key={`${store?.id}|${filterKey}`} locale={locale} filters={filters} sessions={current.page?.sessions ?? []} disabled={!store}
          onApply={next => { previous.current = []; navigate(store?.id ?? "", state, "", "", next); }}>
        <div className="orders-controls orders-v2-state-controls">
          <label id="orders-state-field" className="orders-v2-state-field">
            {c.filter}
            <select
              data-testid="state-filter"
              value={state}
              onChange={(event) => {
                previous.current = [];
                navigate(
                  store?.id ?? "",
                  event.target.value as OrderFilter,
                  "",
                  "",
                );
              }}
            >
              {orderStates.map((value) => (
                <option key={value} value={value}>
                  {c.statuses[value]}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            data-testid="orders-refresh"
            disabled={current.status === "loading"}
            onClick={retry}
          >
            <Icon name="refresh" size={18} />
            {c.refresh}
          </button>
          {store && current.status === "ready" && actions?.orders_export && (
            <>
              <a
                className="orders-export"
                data-testid="orders-export"
                href={exportUnshippedHref(store.id)}
                download
              >
                {c.exportCsv}
              </a>
              <p className="orders-export-hint">{c.exportHint}</p>
            </>
          )}
          {store && actions?.fulfillment_write && <TrackingImport key={`${store.id}:${locale}`} locale={locale} store={store} onComplete={retry} />}
        </div>
        </OrderListFilters>}
        {current.status === "loading" && (
          <p className="orders-message" role="status">
            {c.loading}
          </p>
        )}
        {current.status !== "ready" &&
          current.status !== "loading" &&
          current.status !== "hidden" && (
            <div className="orders-message" role="status">
              <p>
                {current.status === "initial"
                  ? c.loading
                  : current.status === "not-found" && !store
                    ? c.noStore
                    : message(current.status)}
              </p>
              {current.status !== "initial" && (
                <button type="button" onClick={retry}>
                  {c.retry}
                </button>
              )}
            </div>
          )}
        {store && session.current && !["hidden", "signed-out", "forbidden", "not-found"].includes(current.status) && <PickList key={`${store.id}:${session.current}`} locale={locale} store={store.id} boundary={session.current}
          ids={selectedIDs} cvsIDs={selectedIDs.filter(id=>bulkRows[id])} sessionID={filters.session_id}
          canExport={!!actions?.orders_export} canShip={!!actions?.fulfillment_write} disabled={current.status !== "ready"}
          onClear={()=>setBulk({scope:bulkScope,rows:{}})}
          onViewOrder={id=>{previous.current=[];navigate(store.id,"all","",id,{...emptyFilters,q:`LC-${id.replaceAll("-","").toUpperCase()}`});}} />}
        {current.status === "ready" && current.page && (
          <>
            <TabStrip label={v2.counts} previousLabel={presentationCopy[locale].previous} nextLabel={presentationCopy[locale].next} data-testid="orders-tabs">
              {buckets.map(bucket => <button type="button" key={bucket} data-testid={`orders-bucket-${bucket}`} aria-pressed={filters.bucket === bucket}
                onClick={() => { previous.current = []; navigate(store?.id ?? "", state, "", "", { ...filters, bucket }); }}>
                {v2.tabs[bucket]} <span data-testid={`orders-count-${bucket}`}>{current.page!.counts[bucket]}</span>
              </button>)}
            </TabStrip>
            <p className="orders-v2-total" data-testid="orders-total">{v2.total}: {current.page.total}</p>
            {filters.bucket === "completed" && <p className="orders-v2-note">{v2.completedNote}</p>}
            <div className="orders-table-scroll">
              <table className="orders-table" data-testid="orders-table">
                <thead>
                  <tr>
                    <th><label className="pick-row-check"><input type="checkbox" aria-label={pc.page}
                      checked={current.page.items.length>0 && current.page.items.every(row=>Object.hasOwn(bulkRows,row.order_id))}
                      ref={node=>{if(node)node.indeterminate=current.page!.items.some(row=>Object.hasOwn(bulkRows,row.order_id))&&!current.page!.items.every(row=>Object.hasOwn(bulkRows,row.order_id));}}
                      disabled={!current.page.items.length || (new Set([...selectedIDs,...current.page.items.map(r=>r.order_id)]).size>500 && !current.page.items.every(row=>Object.hasOwn(bulkRows,row.order_id)))}
                      onChange={e=>checkRows(current.page!.items,e.target.checked)}/></label>{c.order}</th>
                    <th>{c.created}</th>
                    <th>{v2.recipient}</th>
                    <th>{c.total}</th>
                    <th>{v2.paymentColumn}</th>
                    <th>{v2.deliveryColumn}</th>
                    <th>{v2.source}</th>
                  </tr>
                </thead>
                <tbody>
                  {current.page.items.map((row) => (
                    <OrderRow
                      key={row.order_id}
                      row={row}
                      checked={Object.hasOwn(bulkRows,row.order_id)}
                      checkDisabled={selectedIDs.length>=500 && !Object.hasOwn(bulkRows,row.order_id)}
                      onCheck={checked=>checkRows([row],checked)}
                      c={c}
                      locale={locale}
                      selected={order === row.order_id}
                      isNew={fresh.has(row.order_id)}
                      onSelect={() => choose(row)}
                      detail={order === row.order_id ? current.detail : null}
                      detailStatus={
                        order === row.order_id
                          ? current.detailStatus
                          : "initial"
                      }
                      sections={
                        store
                          ? {
                              store: store.id,
                              actions: actions ?? noActions,
                              boundary: session.current,
                              onChanged: reload,
                            }
                          : null
                      }
                    />
                  ))}
                </tbody>
              </table>
            </div>
            {current.page.items.length === 0 && (
              <p className="orders-message" role="status" aria-live="polite">
                {c.empty}
              </p>
            )}
            <footer className="orders-pager">
              <span>
                {c.pageCount}: {current.page.items.length}
              </span>
              <div>
                <button
                  type="button"
                  data-testid="orders-previous"
                  disabled={!previous.current.length}
                  onClick={back}
                >
                  {c.previous}
                </button>
                <button
                  type="button"
                  data-testid="orders-next"
                  disabled={!current.page.next_cursor}
                  onClick={next}
                >
                  {c.next}
                </button>
              </div>
            </footer>
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}

function OrderRow({
  row, checked, checkDisabled, onCheck,
  c,
  locale,
  selected,
  isNew,
  onSelect,
  detail,
  detailStatus,
  sections,
}: {
  row: OrderSummaryV2;
  checked:boolean; checkDisabled:boolean; onCheck:(checked:boolean)=>void;
  c: OrdersCopy;
  locale: Locale;
  selected: boolean;
  isNew: boolean;
  onSelect: () => void;
  detail: OrderDetail | null;
  detailStatus: Status;
  sections: Sections | null;
}) {
  const v2 = ordersV2Copy[locale];
  return (
    <>
      <tr
        className={selected ? "orders-selected" : ""}
        data-testid={`order-row-${row.order_id}`}
      >
        <td data-label={c.order}>
          <label className="pick-row-check"><input type="checkbox" aria-label={`${picklistCopy[locale].select}: ${row.order_number}`} checked={checked} disabled={checkDisabled} onChange={e=>onCheck(e.target.checked)}/></label>
          <button
            type="button"
            data-testid={`order-expand-${row.order_id}`}
            aria-expanded={selected}
            aria-label={`${selected ? c.collapse : c.expand}: ${row.order_id}`}
            onClick={onSelect}
          >
            <Icon
              name="chevron"
              size={16}
              style={{ transform: selected ? "rotate(90deg)" : undefined }}
            />
            <span title={row.order_number}>
              {row.order_number.length > 16 ? `${row.order_number.slice(0, 11)}…` : row.order_number}
            </span>
            {isNew && (
              <span className="orders-badge" data-testid={`order-new-${row.order_id}`}>
                {c.newOrder}
              </span>
            )}
            {row.source === "merchant_manual" && (
              <span className="orders-badge" data-testid={`order-manual-${row.order_id}`}>
                {c.manualOrder}
              </span>
            )}
          </button>
        </td>
        <td data-label={c.created}>{displayTime(locale, row.created_at)}</td>
        <td data-label={v2.recipient}>{row.recipient_masked}</td>
        <td data-label={c.total}>
          {amount(locale, row.currency, row.total_minor)}
          {row.payment_mode === "cash_on_delivery" && <strong className="orders-cod-amount" data-testid="order-row-collect">{codCopy[locale].collectAmount}: {amount(locale, row.currency, row.cod_collect_minor ?? 0)}</strong>}
        </td>
        <td data-label={v2.paymentColumn} data-testid="order-payment-cell">
          <span>{v2.modes[row.payment_mode]}</span>
          {badge(row.payment_state, c)}
          {(row.payment_mode === "cash_on_delivery" || row.payment_mode === "pay_at_pickup") && <span>{row.collection_state === "COLLECTED" ? v2.collected : v2.pendingCollection}</span>}
          {row.test_mode && <span className="orders-test">{c.test}</span>}
        </td>
        <td data-label={v2.deliveryColumn}><span>{v2.deliveries[row.delivery_kind]}</span>{badge(row.fulfillment_state, c)}{badge(row.commercial_state, c)}</td>
        <td data-label={v2.source}>{row.live_sessions.length ? <><span>{v2.live}</span>{row.live_sessions.map(s => <span key={s.id}>{s.name}</span>)}</> : row.source === "merchant_manual" ? v2.manual : v2.storefront}</td>
      </tr>
      {selected && (
        <tr className="orders-detail-row">
          <td colSpan={7}>
            {detail && sections ? (
              detailPanel(detail, locale, c, sections)
            ) : (
              <p className="orders-detail-message" role="status">
                {detailStatus === "loading"
                  ? c.detailLoading
                  : detailStatus === "not-found"
                    ? c.notFound
                    : c.unavailable}
              </p>
            )}
          </td>
        </tr>
      )}
    </>
  );
}
