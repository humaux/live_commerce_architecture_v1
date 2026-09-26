"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import {
  readOrderDetail,
  readOrderList,
  OrderReadError,
  type OrderReadCode,
} from "@/lib/orders-client";
import {
  orderStates,
  type OrderDetail,
  type OrderFilter,
  type OrderList,
  type OrderSummary,
} from "@/lib/orders-model";
import { ordersCopy, type OrdersCopy } from "@/lib/orders-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { Icon } from "./Icon";
import "./orders.css";

type Status = "initial" | "loading" | "ready" | "hidden" | OrderReadCode;
type View = {
  key: string;
  status: Status;
  page: OrderList | null;
  detail: OrderDetail | null;
  detailStatus: Status;
};

function url(
  locale: Locale,
  store: string,
  state: OrderFilter,
  cursor: string,
  order: string,
) {
  const params = new URLSearchParams();
  if (store) params.set("store", store);
  if (state !== "all") params.set("state", state);
  if (cursor) params.set("cursor", cursor);
  if (order) params.set("order", order);
  return `/${locale}/orders${params.size ? `?${params}` : ""}`;
}

function displayTime(locale: Locale, value: string) {
  return new Intl.DateTimeFormat(locale, {
    timeZone: "UTC",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(value));
}
function amount(locale: Locale, currency: string, minor: number) {
  return money(locale, currency, minor);
}
function badge(state: string, c: OrdersCopy) {
  return (
    <span
      className={`orders-badge orders-badge-${state.toLowerCase()}`}
      data-state={state}
    >
      {c.statuses[state as keyof OrdersCopy["statuses"]]}
    </span>
  );
}
function detailPanel(detail: OrderDetail, locale: Locale, c: OrdersCopy) {
  const m = (value: number) => amount(locale, detail.currency, value);
  const dest = detail.destination;
  const address = dest.pickup
    ? dest.pickup.address
    : [
        dest.home_address.region,
        dest.home_address.city,
        dest.home_address.postal_code,
        dest.home_address.line1,
        dest.home_address.line2,
      ]
        .filter(Boolean)
        .join(" · ");
  return (
    <section
      className="orders-expanded"
      data-testid="order-detail"
      aria-label={`${c.order} ${detail.order_id}`}
    >
      <div className="orders-items">
        <h2>{c.items}</h2>
        <div className="orders-items-scroll">
          <table>
            <thead>
              <tr>
                <th>{c.product}</th>
                <th>{c.unit}</th>
                <th>{c.quantity}</th>
                <th>{c.amount}</th>
              </tr>
            </thead>
            <tbody>
              {detail.items.map((item) => (
                <tr key={item.sku_id}>
                  <td>
                    <strong>{item.name}</strong>
                    <small>{item.code}</small>
                  </td>
                  <td>{m(item.unit_price_minor)}</td>
                  <td>{item.quantity}</td>
                  <td>{m(item.amount.total_minor)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <dl className="orders-totals">
          <div>
            <dt>{c.subtotal}</dt>
            <dd>{m(detail.totals.subtotal_minor)}</dd>
          </div>
          <div>
            <dt>{c.discount}</dt>
            <dd>{m(detail.totals.discount_minor)}</dd>
          </div>
          <div>
            <dt>{c.shipping}</dt>
            <dd>{m(detail.totals.shipping_minor)}</dd>
          </div>
          <div>
            <dt>{c.shippingTax}</dt>
            <dd>{m(detail.totals.shipping_tax_minor)}</dd>
          </div>
          <div>
            <dt>{c.tax}</dt>
            <dd>{m(detail.totals.tax_minor)}</dd>
          </div>
          <div className="orders-grand">
            <dt>{c.grandTotal}</dt>
            <dd>{m(detail.totals.total_minor)}</dd>
          </div>
        </dl>
      </div>
      <div className="orders-recipient">
        <h2>{c.recipient}</h2>
        <dl>
          <div>
            <dt>{c.name}</dt>
            <dd>{dest.recipient_name}</dd>
          </div>
          <div>
            <dt>{c.phone}</dt>
            <dd>{dest.phone}</dd>
          </div>
          <div>
            <dt>{c.method}</dt>
            <dd>{c.statuses[dest.kind]}</dd>
          </div>
          {dest.pickup && (
            <>
              <div>
                <dt>{c.pickupName}</dt>
                <dd>{dest.pickup.name}</dd>
              </div>
              <div>
                <dt>{c.pickupCode}</dt>
                <dd>{dest.pickup.code}</dd>
              </div>
            </>
          )}
          <div>
            <dt>{c.address}</dt>
            <dd>{address}</dd>
          </div>
        </dl>
        <p>{c.snapshot}</p>
      </div>
      <div className="orders-statuses">
        <h2>{c.commercial}</h2>
        <dl>
          <div>
            <dt>{c.commercial}</dt>
            <dd>{badge(detail.commercial_state, c)}</dd>
          </div>
          <div>
            <dt>{c.payment}</dt>
            <dd>{badge(detail.payment_state, c)}</dd>
          </div>
          <div>
            <dt>{c.fulfillment}</dt>
            <dd>{badge(detail.fulfillment_state, c)}</dd>
          </div>
          <div>
            <dt>{c.work}</dt>
            <dd>{badge(detail.work_state, c)}</dd>
          </div>
        </dl>
        {detail.test_mode && (
          <p className="orders-test" data-testid="order-test-mode">
            {c.test}
          </p>
        )}
      </div>
    </section>
  );
}

export function MerchantOrders({
  locale,
  stores,
  store,
  state,
  order,
  cursor,
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  state: OrderFilter;
  order: string;
  cursor: string;
  initialError: OrderReadCode | null;
  renderKey: string;
}) {
  const c = ordersCopy[locale];
  const router = useRouter();
  const [refresh, setRefresh] = useState(0);
  const key = `${renderKey}|${locale}|${store?.id ?? ""}|${state}|${cursor}|${order}|${initialError ?? ""}|${refresh}`;
  const [view, setView] = useState<View>({
    key: "",
    status: "initial",
    page: null,
    detail: null,
    detailStatus: "initial",
  });
  const generation = useRef(0);
  const controller = useRef<AbortController | null>(null);
  const hidden = useRef(false);
  const blocked = useRef(false);
  const session = useRef("");
  const cookie = useRef("");
  const previous = useRef<string[]>([]);
  const current =
    view.key === key && (!cookie.current || csrfCookie() === cookie.current)
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
      if (block) blocked.current = true;
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
      const page = await readOrderList(store.id, state, cursor, active.signal);
      if (!(await stillCurrent(boundary)))
        throw new OrderReadError("signed-out");
      session.current = boundary; // The cookie is a change fence, not authority; BFF just authorized this read.
      cookie.current = csrfCookie();
      const selected =
        order && page.items.some((row) => row.order_id === order);
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
  }, [key, initialError, store, state, cursor, order]);

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
  ) {
    clear("loading");
    router.push(url(locale, nextStore, nextState, nextCursor, nextOrder));
  }
  function choose(row: OrderSummary) {
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
      <div className="orders-page" data-testid="merchant-orders">
        <header className="orders-heading">
          <h1>{c.title}</h1>
          <p>{c.subtitle}</p>
        </header>
        <div className="orders-controls">
          {stores.length > 1 && (
            <label>
              {c.store}
              <select
                data-testid="store-selector"
                value={store?.id ?? ""}
                onChange={(event) => {
                  previous.current = [];
                  navigate(event.target.value, state, "", "");
                }}
              >
                {stores.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          <label>
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
        </div>
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
        {current.status === "ready" && current.page && (
          <>
            <div className="orders-table-scroll">
              <table className="orders-table" data-testid="orders-table">
                <thead>
                  <tr>
                    <th>{c.order}</th>
                    <th>{c.created}</th>
                    <th>{c.total}</th>
                    <th>{c.commercial}</th>
                    <th>{c.payment}</th>
                  </tr>
                </thead>
                <tbody>
                  {current.page.items.map((row) => (
                    <OrderRow
                      key={row.order_id}
                      row={row}
                      c={c}
                      locale={locale}
                      selected={order === row.order_id}
                      onSelect={() => choose(row)}
                      detail={order === row.order_id ? current.detail : null}
                      detailStatus={
                        order === row.order_id
                          ? current.detailStatus
                          : "initial"
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
  row,
  c,
  locale,
  selected,
  onSelect,
  detail,
  detailStatus,
}: {
  row: OrderSummary;
  c: OrdersCopy;
  locale: Locale;
  selected: boolean;
  onSelect: () => void;
  detail: OrderDetail | null;
  detailStatus: Status;
}) {
  return (
    <>
      <tr
        className={selected ? "orders-selected" : ""}
        data-testid={`order-row-${row.order_id}`}
      >
        <td data-label={c.order}>
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
            <span>
              {row.order_id.slice(0, 4)}…{row.order_id.slice(-4)}
            </span>
          </button>
        </td>
        <td data-label={c.created}>{displayTime(locale, row.created_at)}</td>
        <td data-label={c.total}>
          {amount(locale, row.currency, row.total_minor)}
        </td>
        <td data-label={c.commercial}>{badge(row.commercial_state, c)}</td>
        <td data-label={c.payment}>
          {badge(row.payment_state, c)}
          {row.test_mode && <span className="orders-test">{c.test}</span>}
        </td>
      </tr>
      {selected && (
        <tr className="orders-detail-row">
          <td colSpan={5}>
            {detail ? (
              detailPanel(detail, locale, c)
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
