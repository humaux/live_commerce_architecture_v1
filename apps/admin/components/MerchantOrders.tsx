// Purpose: Owns order list loading, filtering, bulk selection and selected-order detail.
// Depends on: react, react-dom, next/navigation, @live-commerce/i18n, @/lib/model, @/lib/settings-client, @/lib/orders-client, @/lib/orders-model, @/lib/orders-copy, @/lib/orders-v2, @/lib/orders-v2-copy, ./OrderListFilters, ./WorkspaceFrame, ./AdminStateBlock, ./AdminForbidden, @live-commerce/ui, @/lib/presentation-copy, ./OrderDetailPanel, ./Icon, ./orders.css, ./order-actions.css, ./orders-v2.css, ./ParcelGroup, @/lib/parcels-copy
// Depends on: ./OrderDetailView, ./orders/parity-copy, ./orders/parity.css, ./orders/OrderSelectionBar for the mounted shipping selection summary.
// Used by: apps/admin/app/[locale]/orders/page.tsx
// Invariants: Counts/rows/totals share the server snapshot; private search and selection stay in mounted memory.
"use client";

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
  orderStates,
  type OrderActions,
  type OrderDetail,
  type OrderFilter,
  type OrderSummary,
} from "@/lib/orders-model";
import { ordersCopy, type OrdersCopy } from "@/lib/orders-copy";
import { emptyFilters, appendOrderFilters, buckets, type OrderFilters, type OrderListV2, type OrderSummaryV2 } from "@/lib/orders-v2";
import { ordersV2Copy } from "@/lib/orders-v2-copy";
import { OrderListFilters } from "./OrderListFilters";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminStateBlock } from "./AdminStateBlock";
import { AdminForbidden } from "./AdminForbidden";
import { TabStrip } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import { amount, badge } from "./OrderDetailPanel";
import { OrderDetailView } from "./OrderDetailView";
import { parityCopy, shortOrderNumber } from "./orders/parity-copy";
import "./orders/parity.css";
import { Icon } from "./Icon";
import { PickList } from "./PickList";
import { selectOrders } from "@/lib/picklist-model";
import { picklistCopy } from "@/lib/picklist-copy";
import { TrackingImport } from "./TrackingImport";
import { OrderSelectionBar } from "./orders/OrderSelectionBar";
import { ParcelMerge, type ParcelGroupView } from "./ParcelGroup";
import { groupOfOrder, parcelGenAfterPoll, parcelGenAfterShipmentRefusal } from "@/lib/parcels-model.ts";
import { parcelCopy } from "@/lib/parcels-copy";
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
  appendOrderFilters(params, filters);
  return `/${locale}/orders${order ? `/${order}` : ""}${params.size ? `?${params}` : ""}`;
}

/** Owns order list loading, filtering and selected-order detail. Loads and refreshes orders through orders-client; section components own action commands. */
export function MerchantOrders({
  locale,
  store,
  state,
  filters: routeFilters,
  order: routeOrder,
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
  const p = parityCopy[locale];
  const [queuesOpen,setQueuesOpen] = useState(false);
  const [selection,setSelection] = useState({renderKey,id:routeOrder});
  const order = selection.renderKey===renderKey ? selection.id : routeOrder;
  const returnPath = useRef("");
  const pc = picklistCopy[locale];
  const bulkScope = `${store?.id ?? ""}:${csrfCookie()}`;
  // Keep only server order numbers and delivery eligibility, never recipient data, across page navigation.
  const [bulk, setBulk] = useState<{scope:string; rows:Record<string,{cvs:boolean; number:string}>}>({scope:"",rows:{}});
  const pickTools = useRef<HTMLDivElement>(null);
  const bulkRows = bulk.scope === bulkScope ? bulk.rows : {};
  const selectedIDs = Object.keys(bulkRows);
  // W3-07B: parcel group panels, fenced by the same store+session scope as the bulk selection. OPEN groups come from GET
  // parcel-groups on every load (so a reload keeps them, W3-U4); groups this session shipped/dissolved stay as history until
  // closed. The server guard in_parcel_group stays the authority for stale single-order shipments.
  const parcelC = parcelCopy[locale];
  const [parcels, setParcels] = useState<{ scope: string; groups: ParcelGroupView[] }>({ scope: "", groups: [] });
  const parcelGroups = parcels.scope === bulkScope ? parcels.groups : [];
  const groupOf = (orderID: string) => groupOfOrder(parcelGroups, orderID); // OPEN group first, then history
  function checkRows(rows:OrderSummaryV2[], checked:boolean) {
    setBulk(current => {
      const retained = current.scope === bulkScope ? current.rows : {};
      try {
        const ids = selectOrders(Object.keys(retained), rows.map(r=>r.order_id), checked);
        const summaries = new Map(rows.map(r=>[r.order_id,{cvs:r.delivery_kind.startsWith("cvs_"),number:shortOrderNumber(r.order_id)}]));
        return {scope:bulkScope,rows:Object.fromEntries(ids.map(id=>[id,summaries.get(id) ?? retained[id]!]))};
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
  // Bumped by every successful poll; with `refresh` (Refresh button, retry, writes) it is the generation the parcel reads follow.
  const [pollGen, setPollGen] = useState(0);
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
      if (status === "signed-out" || status === "forbidden" || status === "not-found") {
        setBulk({scope:"", rows:{}});
        setSearch({ store: "", value: "", cursor: "" });
        setParcels({ scope: "", groups: [] });
        session.current = "";
        setActions(null);
      }
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

  // Parcel reads share order authority; a denial must revoke the whole page, including in-flight order polls.
  const loseParcelScope = useCallback(
    (code: Exclude<OrderReadCode, "unavailable">) => clear(code, true),
    [clear],
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
        Boolean(order);
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
    readOrderActions(store.id, active.signal).then((value) => {
      if (!active.signal.aborted && !blocked.current) setActions(value);
    }, () => {
      if (!active.signal.aborted && !blocked.current) setActions(noActions);
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
      setPollGen(parcelGenAfterPoll); // every successful poll, on any page: the parcel suggestions and OPEN groups are re-read
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
    const onHistory = () => {
      clear("loading");
      const id=window.location.pathname.match(/\/orders\/([0-9a-f-]{36})$/)?.[1] ?? new URLSearchParams(window.location.search).get("order") ?? "";
      setSelection({renderKey,id});
      setRefresh(value=>value+1);
    };
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
    setSelection({renderKey,id:nextOrder});
    const searchChanged = search.store !== nextStore || search.value !== nextFilters.q || search.cursor !== nextCursor;
    setSearch(old => old.store === nextStore && old.value === nextFilters.q && old.cursor === nextCursor ? old : { store: nextStore, value: nextFilters.q, cursor: nextCursor });
    const destination = url(locale, nextStore, nextState, nextCursor, nextOrder, nextFilters);
    if (destination !== window.location.pathname + window.location.search) router.push(destination);
    else if (!searchChanged) setRefresh(value => value + 1);
  }
  function choose(row: OrderSummary) {
    if (fresh.has(row.order_id)) setFresh(old=>new Set([...old].filter(id=>id!==row.order_id)));
    returnPath.current=window.location.pathname+window.location.search;
    clear("loading");
    setSelection({renderKey,id:row.order_id});
    // Next supports native history navigation. Keep private search in this mounted
    // controller; hard refresh independently resolves the same real detail route.
    window.history.pushState(null,"",url(locale,store?.id??"",state,cursor,row.order_id,filters));
    window.scrollTo(0,0);
  }
  function returnToList() {
    clear("loading");
    setSelection({renderKey,id:""});
    window.history.pushState(null,"",returnPath.current||url(locale,store?.id??"",state,cursor,"",filters));
    window.scrollTo(0,0);
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
      <div className="orders-page orders-v2 signed-orders" data-testid="merchant-orders">
        {!order && <header className="signed-orders-heading"><h1>{c.title}</h1><a className="primary" href={`/${locale}/orders/shipping${store?`?store=${store.id}`:""}`}>{p.ship}</a></header>}
        {!order && !["hidden", "signed-out", "forbidden", "not-found"].includes(current.status) && <OrderListFilters key={`${store?.id}|${filterKey}`} locale={locale} filters={filters} sessions={current.page?.sessions ?? []} disabled={!store}
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
        {(current.status === "loading" || current.status === "initial") && (
          <AdminStateBlock variant="loading" title={c.loading} />
        )}
        {current.status === "forbidden" && <AdminForbidden locale={locale} description={c.forbidden} />}
        {current.status !== "ready" &&
          current.status !== "loading" &&
          current.status !== "initial" &&
          current.status !== "hidden" &&
          current.status !== "forbidden" && (
            <AdminStateBlock
              variant="error"
              title={current.status === "not-found" && !store ? c.noStore : message(current.status)}
              onRetry={retry}
              retryLabel={c.retry}
            />
          )}
        <details className="signed-orders-tools" hidden={["hidden","signed-out","forbidden","not-found"].includes(current.status)}><summary>{p.advanced}</summary><div ref={pickTools}>
        {store && session.current && !["hidden", "signed-out", "forbidden", "not-found"].includes(current.status) && <PickList key={`${store.id}:${session.current}`} locale={locale} store={store.id} boundary={session.current}
          ids={selectedIDs} cvsIDs={selectedIDs.filter(id=>bulkRows[id]!.cvs)} sessionID={filters.session_id}
          canExport={!!actions?.orders_export} canShip={!!actions?.fulfillment_write} disabled={current.status !== "ready"}
          onClear={()=>setBulk({scope:bulkScope,rows:{}})}
          onViewOrder={id=>{previous.current=[];navigate(store.id,"all","",id,{...emptyFilters,q:`LC-${id.replaceAll("-","").toUpperCase()}`});}} />}
        </div>
        {/* W3-07B: merge-suggestion banner + group panels, above the list; only writers may merge (Go re-checks). */}
        {store && session.current && actions?.fulfillment_write && !["hidden", "signed-out", "forbidden", "not-found"].includes(current.status) && (
          <ParcelMerge
            key={bulkScope}
            store={store.id}
            boundary={session.current}
            disabled={current.status !== "ready"}
            c={parcelC}
            shipmentCopy={c}
            groups={parcelGroups}
            refreshGen={refresh + pollGen}
            onGroups={(next) => {
              if (!blocked.current && !hidden.current) setParcels({ scope: bulkScope, groups: next });
            }}
            onScopeLost={loseParcelScope}
            onChanged={() => setRefresh((v) => v + 1)}
          />
        )}
        </details>
        {order && current.status === "ready" && <>
          {current.detailStatus === "ready" && current.detail && store ? <OrderDetailView key={`${order}:${locale}`} detail={current.detail} locale={locale} c={c} onBack={returnToList}
            sections={{store:store.id,actions:actions??noActions,boundary:session.current,onChanged:reload,
              onShipmentRefused:(code:string)=>setPollGen(v=>parcelGenAfterShipmentRefusal(v,code)),
              parcelBadge:groupOf(order)?parcelC.badge(groupOf(order)!.short):undefined,
              parcelBlockText:orderID=>groupOf(orderID)?.state==="OPEN"?parcelC.blocked:undefined}} /> :
          <div><button type="button" data-testid="order-detail-back" onClick={returnToList}>{p.back}</button><AdminStateBlock variant={current.detailStatus==="loading"?"loading":"error"} title={message(current.detailStatus)||c.detailLoading} onRetry={retry} retryLabel={c.retry}/></div>}
        </>}
        {!order && current.status === "ready" && current.page && (
          <>
            <div className="signed-queue-control" data-open={queuesOpen}>
            <button className="signed-queue-menu" type="button" data-testid="orders-queue-menu" aria-label={v2.counts} aria-expanded={queuesOpen} onClick={()=>setQueuesOpen(open=>!open)}>{p.tabs[filters.bucket]} {current.page.counts[filters.bucket]}</button>
            <TabStrip className="signed-queue-options" label={v2.counts} previousLabel={presentationCopy[locale].previous} nextLabel={presentationCopy[locale].next} data-testid="orders-tabs">
              {buckets.map(bucket => <button type="button" key={bucket} data-testid={`orders-bucket-${bucket}`} aria-pressed={filters.bucket === bucket}
                onClick={() => { setQueuesOpen(false); previous.current = []; navigate(store?.id ?? "", state, "", "", { ...filters, bucket }); }}>
                {p.tabs[bucket]} <span data-testid={`orders-count-${bucket}`}>{current.page!.counts[bucket]}</span>
              </button>)}
            </TabStrip></div>
            <div className="orders-list-tools"><p className="orders-v2-total" data-testid="orders-total">{v2.total}: {current.page.total}</p><label className="pick-row-check orders-page-selection"><input type="checkbox" aria-label={pc.page}
                      checked={current.page.items.length>0 && current.page.items.every(row=>Object.hasOwn(bulkRows,row.order_id))}
                      ref={node=>{if(node)node.indeterminate=current.page!.items.some(row=>Object.hasOwn(bulkRows,row.order_id))&&!current.page!.items.every(row=>Object.hasOwn(bulkRows,row.order_id));}}
                      disabled={!current.page.items.length || (new Set([...selectedIDs,...current.page.items.map(r=>r.order_id)]).size>500 && !current.page.items.every(row=>Object.hasOwn(bulkRows,row.order_id)))}
                      onChange={e=>checkRows(current.page!.items,e.target.checked)}/><span>{pc.page}</span></label></div>
            {filters.bucket === "completed" && <p className="orders-v2-note">{v2.completedNote}</p>}
            <div className="signed-orders-list" data-testid="orders-list">
              {current.page.items.map(row=><OrderRow key={row.order_id} row={row} c={c} locale={locale}
                checked={Object.hasOwn(bulkRows,row.order_id)} checkDisabled={selectedIDs.length>=500&&!Object.hasOwn(bulkRows,row.order_id)}
                onCheck={checked=>checkRows([row],checked)} onSelect={()=>choose(row)} isNew={fresh.has(row.order_id)}
                parcelBadge={groupOf(row.order_id)?parcelC.badge(groupOf(row.order_id)!.short):undefined}/>) }
            </div>
            {current.page.items.length === 0 && (
              <AdminStateBlock
                variant="empty"
                title={v2.emptyTitle}
                description={v2.emptyDescription}
              />
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
            {selectedIDs.length > 0 && <OrderSelectionBar numbers={selectedIDs.map(id => bulkRows[id]!.number)} c={c}
              shippingHref={`/${locale}/orders/shipping${store?`?store=${store.id}`:""}`} shippingLabel={p.ship}
              onClear={() => setBulk({ scope: bulkScope, rows: {} })}
              onView={() => {
                const outer = pickTools.current?.closest<HTMLDetailsElement>("details");
                if (outer) outer.open=true;
                const tools = pickTools.current?.querySelector<HTMLDetailsElement>("details");
                if (!tools) return;
                tools.open = true;
                tools.scrollIntoView({ block: "start" });
                tools.querySelector<HTMLElement>("summary")?.focus();
              }} />}
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}

function OrderRow({row,checked,checkDisabled,onCheck,c,locale,onSelect,isNew,parcelBadge}:{
  row:OrderSummaryV2;checked:boolean;checkDisabled:boolean;onCheck:(checked:boolean)=>void;c:OrdersCopy;locale:Locale;onSelect:()=>void;isNew:boolean;parcelBadge?:string;
}) {
  const p=parityCopy[locale];
  const signedCopy={...c,statuses:{...c.statuses,CAPTURED:p.paid,MANUAL_UNASSIGNED:p.tabs.ready_to_ship,REVIEW_REQUIRED:p.reviewState,MERCHANT_SHIPPED:p.tabs.shipped}};
  return <article data-testid={`order-row-${row.order_id}`} className="signed-order-card">
    <label className="signed-order-check"><input type="checkbox" checked={checked} disabled={checkDisabled} aria-label={shortOrderNumber(row.order_id)} onChange={event=>onCheck(event.target.checked)}/></label>
    <div className="signed-order-identity"><strong>{shortOrderNumber(row.order_id)}</strong>{isNew&&<span>{c.newOrder}</span>}
      <p>{row.item_summary.items.map((item,index)=><span key={index}>{index>0?" · ":""}{item.product_name}{item.variant_label?` · ${item.variant_label}`:""} ×{item.quantity}</span>)}<span className="signed-mobile-amount"> · {amount(locale,row.currency,row.total_minor)}</span> · {row.recipient_masked}</p>
      {row.item_summary.total_item_count>row.item_summary.items.reduce((sum,item)=>sum+item.quantity,0)&&<small>{p.moreItems(row.item_summary.total_item_count)}</small>}
      {(row.live_sessions.length>0||row.source==="merchant_manual")&&<small>{row.live_sessions.length?row.live_sessions.map(session=>session.name).join(" · ")+" · "+ordersV2Copy[locale].live:c.manualOrder}</small>}
    </div>
    <strong className="signed-order-amount">{amount(locale,row.currency,row.total_minor)}</strong>
    <div className="signed-order-badges"><span data-testid="order-payment-cell">{badge(row.payment_state,signedCopy)}</span>{badge(row.fulfillment_state,signedCopy)}{parcelBadge&&<span data-testid={`parcel-badge-${row.order_id}`}>{parcelBadge}</span>}</div>
    <button type="button" data-testid={`order-expand-${row.order_id}`} onClick={onSelect}>{p.view}</button>
  </article>;
}
