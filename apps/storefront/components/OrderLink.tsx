"use client";

// Manual-order buyer link page (/{locale}/order-link, contracts/storefront-v2.md section G3): read `#o=<order>&t=<token>` from the fragment, remove it
// from the address bar and history AT ONCE, keep the token only in this component's memory, and POST it ONCE to the BFF (POST /api/buyer/orders/link ->
// Go POST /v1/buyer/orders/link). On 200 the BFF has already set the buyer cookie (a full capability of the order's owner, so the buyer can pay), and the
// page replaces itself with /{locale}/orders/{order id}. Every refusal (404, 422) is the same "cannot be used" text; only a transient failure
// (throttle, 5xx, network) offers a retry, which re-sends the same in-memory token (the server never consumed it unless it answered 200).
// Non-goals: no storage of the token (no local/session storage, no cache), no third-party script, no order data rendered here.
// Depends on: lib/order-link-contract.ts (fragment grammar, result validator), lib/order-link-copy.ts.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { orderLinkCopy } from "../lib/order-link-copy";
import { orderLinkFragment, validLinkResult } from "../lib/order-link-contract";

type View = "opening" | "refused" | "unavailable";

export default function OrderLink({ locale }: { locale: Locale }) {
  const copy = orderLinkCopy[locale];
  const [view, setView] = useState<View>("opening");
  const link = useRef<{ orderID: string; token: string } | null>(null);
  const started = useRef(false);

  async function exchange() {
    const held = link.current;
    if (!held) return setView("refused");
    setView("opening");
    try {
      const response = await fetch("/api/buyer/orders/link", {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ order_id: held.orderID, token: held.token }),
      });
      if (response.status === 200) {
        const value: unknown = await response.json().catch(() => null);
        if (validLinkResult(value, held.orderID)) {
          link.current = null;
          window.location.replace(`/${locale}/orders/${held.orderID}`);
          return;
        }
        return setView("unavailable");
      }
      if (response.status === 404 || response.status === 422) {
        link.current = null;
        return setView("refused");
      }
      setView("unavailable");
    } catch {
      setView("unavailable");
    }
  }

  useEffect(() => {
    if (started.current) return; // React strict mode runs effects twice: the fragment is read and posted once
    started.current = true;
    link.current = orderLinkFragment(window.location.hash);
    // Gone from the address bar and history before anything else happens (also drops any query string).
    window.history.replaceState(null, "", window.location.pathname);
    void exchange();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <main className="order-section" data-testid="order-link" data-view={view}>
      <h1>{copy.title}</h1>
      {view === "opening" && <p role="status">{copy.opening}</p>}
      {view === "refused" && (
        <div role="alert" data-testid="order-link-refused">
          <p>{copy.refused}</p>
          <p className="order-note">{copy.refusedHelp}</p>
          <p><a href={`/${locale}/orders/lookup`}>{copy.lookup}</a></p>
        </div>
      )}
      {view === "unavailable" && (
        <div role="alert" data-testid="order-link-unavailable">
          <p>{copy.unavailable}</p>
          <button type="button" onClick={() => void exchange()}>{copy.retry}</button>
        </div>
      )}
    </main>
  );
}
