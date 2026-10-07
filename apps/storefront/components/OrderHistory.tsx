"use client";

// Purpose: show buyer-owned order history and read-only order details.
// Depends on: buyer purchase reads, frozen order copy, and shared Taipei time formatting.
// Used by: ProductPurchase order recovery and history view.
// Invariants: list totals and states are server projections; viewing another order never changes checkout recovery.

import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { historyCopy } from "../lib/history-copy";
import { orderCopy } from "../lib/order-copy";
import { purchasePage, readOrder, validOrderSummary } from "../lib/purchase";
import type { Order, OrderSummary } from "../lib/purchase";
import { OrderDetails } from "./OrderFlow";
import { BuyerClientError } from "../lib/buyer-client";
import { displayTime } from "../../../packages/format/src/index";

// Read-only navigation. Never writes the active checkout locator: inspecting A
// while B is pending must not redirect B's receipt/recovery to the older order.
/** Show buyer-owned orders without changing the active checkout locator. */
export default function OrderHistory({
  context,
  locale,
  money,
  onError,
  onPaymentBusy,
}: {
  context: string;
  locale: Locale;
  money: (amount: number, currency: string) => string;
  onError: (reason: unknown) => void;
  onPaymentBusy: (busy: boolean) => void;
}) {
  const copy = historyCopy[locale];
  const [items, setItems] = useState<OrderSummary[]>([]);
  const [cursor, setCursor] = useState("");
  const [detail, setDetail] = useState<Order | null>(null);
  const [busy, setBusy] = useState(true);
  const [failed, setFailed] = useState(false);
  const [paymentBusy, setPaymentBusy] = useState(false);
  const epoch = useRef(0);
  const selected = useRef<string | null>(null);
  const selectionGeneration = useRef(0);
  const retry = useRef<() => Promise<void>>(async () => {});
  async function run(action: () => Promise<void>) {
    const version = epoch.current;
    setBusy(true);
    setFailed(false);
    retry.current = () => run(action);
    try {
      await action();
    } catch (reason) {
      if (version !== epoch.current) return;
      if (
        reason instanceof BuyerClientError &&
        (reason.status === 401 ||
          ["context_changed", "requires_reset"].includes(reason.code))
      ) {
        setItems([]);
        setDetail(null);
        onError(reason);
      } else setFailed(true);
    } finally {
      if (version === epoch.current) setBusy(false);
    }
  }
  async function load(after = "") {
    const version = epoch.current;
    await run(async () => {
      const page = await purchasePage(
        `orders?limit=20${after ? `&cursor=${encodeURIComponent(after)}` : ""}`,
        context,
        validOrderSummary,
      );
      if (version !== epoch.current) return;
      setItems((old) =>
        after
          ? Array.from(
              new Map(
                [...old, ...page.items].map((o) => [o.order_id, o]),
              ).values(),
            )
          : page.items,
      );
      setCursor(page.next_cursor);
    });
  }
  async function view(id: string) {
    const version = epoch.current;
    const generation = ++selectionGeneration.current;
    selected.current = id;
    await run(async () => {
      const order = await readOrder(context, id);
      if (
        version === epoch.current &&
        generation === selectionGeneration.current &&
        selected.current === id
      )
        setDetail(order);
    });
  }
  useEffect(() => {
    epoch.current++;
    void load();
    return () => {
      epoch.current++;
      selected.current = null;
      selectionGeneration.current++;
    };
  }, [context]);
  return (
    <section
      className="order-history"
      data-testid="order-history"
      aria-busy={busy}
    >
      {failed && (
        <div role="alert">
          <p>{copy.failed}</p>
          <button disabled={busy} onClick={() => void retry.current()}>
            {copy.retry}
          </button>
        </div>
      )}
      {detail ? (
        <>
          <button
            disabled={busy || paymentBusy}
            onClick={() => {
              selected.current = null;
              selectionGeneration.current++;
              setDetail(null);
            }}
          >
            {copy.list}
          </button>
          <OrderDetails
            context={context}
            order={detail}
            locale={locale}
            money={money}
            busy={busy || paymentBusy}
            onPaymentBusy={(value) => {
              setPaymentBusy(value);
              onPaymentBusy(value);
            }}
            isSelected={(() => {
              const id = detail.order_id,
                generation = selectionGeneration.current;
              return () =>
                selected.current === id &&
                selectionGeneration.current === generation;
            })()}
            refresh={() => void view(detail.order_id)}
          />
        </>
      ) : (
        <>
          <h1>{copy.title}</h1>
          <p className="order-note">{copy.scope} {copy.timeZone}</p>
          {!busy && !failed && !items.length && <p>{copy.empty}</p>}
          <ol className="history-list">
            {items.map((order) => (
              <li key={order.order_id}>
                <div>
                  <time dateTime={order.created_at}>
                    {displayTime(locale, order.created_at)}
                  </time>
                  <span>{orderCopy[locale][order.commercial_state]}</span>
                </div>
                <strong>{money(order.total_minor, order.currency)}</strong>
                <span className="order-id">{order.order_id}</span>
                <button
                  data-order-id={order.order_id}
                  disabled={busy}
                  onClick={() => void view(order.order_id)}
                >
                  {copy.details}
                </button>
              </li>
            ))}
          </ol>
          {cursor && (
            <button disabled={busy} onClick={() => void load(cursor)}>
              {copy.more}
            </button>
          )}
        </>
      )}
      {busy && <p role="status">{copy.loading}</p>}
    </section>
  );
}
