// Purpose: Server-owned buyer order facts with signed read and access recovery.
// Depends on: buyer-client, purchase, lookup-copy, order-status/OrderStatusView (signed frame layout) and state/StorefrontState.
// Used by: localized order detail route.
// Invariants: a failed read never fabricates an order or retries payment; denial clears prior details.
"use client";

// The buyer's order page (BFF /api/buyer/session[/prepare|activate], /api/buyer/orders/{id}[/payment|/bank-transfer...] -> Go /v1/buyer/*).
// It draws the order in the signed frame layout (OrderStatusView); payment, bank-transfer proof and CVS views run on the same controllers as the
// checkout / history order detail (OrderDetails in OrderFlow.tsx, unchanged). A visitor without a valid capability (cookie missing, expired or another owner's order) gets the lookup link.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { BuyerClientError, readBuyerSession } from "../../../../lib/buyer-client";
import { lookupCopy } from "../../../../lib/lookup-copy";
import { formatMoney } from "../../../../lib/money";
import { readOrder } from "../../../../lib/purchase";
import type { Order } from "../../../../lib/purchase";
import { StorefrontState } from "../../../../components/state/StorefrontState";
import OrderStatusView from "../../../../components/order-status/OrderStatusView";

export default function OrderPage({ locale, orderID }: { locale: Locale; orderID: string }) {
  const copy = lookupCopy[locale];
  const [order, setOrder] = useState<Order | null>(null);
  const [context, setContext] = useState("");
  const [state, setState] = useState<"loading" | "ready" | "failed" | "forbidden">("loading");
  const [retried, setRetried] = useState(false);
  const [paymentBusy, setPaymentBusy] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const epoch = useRef(0);
  const orderRef = useRef<Order | null>(null);
  orderRef.current = order;

  async function load(isRetry = false) {
    const mine = ++epoch.current;
    if (!isRetry && !orderRef.current) setState("loading");
    else if (orderRef.current) setRefreshing(true);
    try {
      const session = await readBuyerSession();
      if (session.state !== "active" || !session.context) throw new BuyerClientError("requires_reset");
      const found = await readOrder(session.context, orderID);
      if (mine !== epoch.current) return;
      setContext(session.context);
      setOrder(found);
      setRetried(false);
      setState("ready");
    } catch (reason) {
      if (mine === epoch.current) {
        setOrder(null);
        setContext("");
        const denied = reason instanceof BuyerClientError &&
          ([401, 403, 404].includes(reason.status ?? 0) || reason.code === "requires_reset" || reason.code === "context_changed");
        setState(denied ? "forbidden" : "failed");
      }
    } finally {
      if (mine === epoch.current) setRefreshing(false);
    }
  }
  useEffect(() => {
    document.documentElement.lang = locale;
    void load();
    return () => {
      epoch.current++;
    };
  }, [orderID]);

  const money = (amount: number, currency: string) => formatMoney(locale, amount, currency);

  return (
    <main className="purchase-main claim-main os-main" data-testid="order-page" aria-busy={state === "loading" || refreshing}>
      {(state === "loading" || refreshing) && <p role="status">{copy.loading}</p>}
      {(state === "failed" || state === "forbidden") && (
        <div data-testid="order-page-failed">
          <StorefrontState locale={locale} variant={state === "forbidden" ? "forbidden" : "offline"}
            title={state === "failed" ? copy.loadFailed : undefined}
            body={state === "failed" ? "" : undefined}
            note={state === "failed" ? "" : undefined}
            failed={retried && state === "failed"}
            onRead={state === "failed" ? () => { setRetried(true); void load(true); } : undefined} />
        </div>
      )}
      {state === "ready" && order && (
        <OrderStatusView
          context={context}
          order={order}
          locale={locale}
          money={money}
          busy={paymentBusy || refreshing}
          onPaymentBusy={setPaymentBusy}
          isSelected={() => true}
          refresh={() => void load()}
        />
      )}
    </main>
  );
}
