"use client";

// The buyer's order page (BFF /api/buyer/session[/prepare|activate], /api/buyer/orders/{id}[/payment|/bank-transfer...] -> Go /v1/buyer/*).
// It reuses the order detail the product page and the history already render (OrderDetails) so payment, bank-transfer proof and CVS views
// behave identically. A visitor without a valid capability (cookie missing, expired or another owner's order) gets the lookup link.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { BuyerClientError, readBuyerSession } from "../../../../lib/buyer-client";
import { lookupCopy } from "../../../../lib/lookup-copy";
import { readOrder } from "../../../../lib/purchase";
import type { Order } from "../../../../lib/purchase";
import { OrderDetails } from "../../../../components/OrderFlow";

export default function OrderPage({ locale, orderID }: { locale: Locale; orderID: string }) {
  const copy = lookupCopy[locale];
  const [order, setOrder] = useState<Order | null>(null);
  const [context, setContext] = useState("");
  const [state, setState] = useState<"loading" | "ready" | "failed">("loading");
  const [paymentBusy, setPaymentBusy] = useState(false);
  const epoch = useRef(0);

  async function load() {
    const mine = ++epoch.current;
    setState("loading");
    try {
      const session = await readBuyerSession();
      if (session.state !== "active" || !session.context) throw new BuyerClientError("requires_reset");
      const found = await readOrder(session.context, orderID);
      if (mine !== epoch.current) return;
      setContext(session.context);
      setOrder(found);
      setState("ready");
    } catch {
      if (mine === epoch.current) setState("failed");
    }
  }
  useEffect(() => {
    document.documentElement.lang = locale;
    void load();
    return () => {
      epoch.current++;
    };
  }, [orderID]);

  const money = (amount: number, currency: string) => {
    const formatter = new Intl.NumberFormat(locale, {
      style: "currency",
      currency,
      currencyDisplay: locale === "zh-TW" && currency === "TWD" ? "code" : "symbol",
    });
    return formatter.format(amount / 10 ** (formatter.resolvedOptions().maximumFractionDigits ?? 2));
  };

  return (
    <main data-testid="order-page" aria-busy={state === "loading"}>
      {state === "loading" && <p role="status">{copy.loading}</p>}
      {state === "failed" && (
        <div role="alert" data-testid="order-page-failed">
          <p>{copy.loadFailed}</p>
          <button type="button" onClick={() => void load()}>{copy.retry}</button>{" "}
          <a href={`/${locale}/orders/lookup`}>{copy.lookupLink}</a>
        </div>
      )}
      {state === "ready" && order && (
        <OrderDetails
          context={context}
          order={order}
          locale={locale}
          money={money}
          busy={paymentBusy}
          onPaymentBusy={setPaymentBusy}
          isSelected={() => true}
          refresh={() => void load()}
        />
      )}
    </main>
  );
}
