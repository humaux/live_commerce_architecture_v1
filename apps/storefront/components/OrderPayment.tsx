"use client";

import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Order } from "../lib/purchase";
import type { OrderPayment as PaymentView } from "../lib/payment-contract";
import {
  openPaymentDestination,
  payOrder,
  pendingOrderPayment,
  readOrderPayment,
} from "../lib/order-payment";
import { paymentCopy } from "../lib/payment-copy";
import { orderCopy } from "../lib/order-copy";

// Local extension of approved B: one inline payment section, no second checkout
// or generic retry path. The controller owns durable intent and one-shot Take;
// this component owns only the selected-screen lifetime and visible read state.
export default function OrderPayment({
  context,
  order,
  locale,
  busy,
  refreshToken,
  onBusy,
  isSelected,
}: {
  context: string;
  order: Order;
  locale: Locale;
  busy: boolean;
  refreshToken: number;
  onBusy: (busy: boolean) => void;
  isSelected: () => boolean;
}) {
  const copy = paymentCopy[locale];
  const [view, setView] = useState<PaymentView | null>(null);
  const [marker, setMarker] =
    useState<ReturnType<typeof pendingOrderPayment>>(null);
  const [reading, setReading] = useState(true);
  const [sending, setSending] = useState(false);
  const [message, setMessage] = useState<
    "failed" | "blocked" | "uncertain" | "submitted" | null
  >(null);
  const epoch = useRef(0);
  const working = useRef(false);
  const readVersion = useRef(0);
  const busyCallback = useRef(onBusy);
  busyCallback.current = onBusy;
  const selection = useRef(isSelected);
  selection.current = isSelected;

  async function read(version: number) {
    const request = ++readVersion.current;
    setReading(true);
    try {
      const value = await readOrderPayment(context, order.order_id);
      if (version !== epoch.current || request !== readVersion.current) return;
      if (
        value.currency !== order.snapshot.quote.currency ||
        value.total_minor !== order.snapshot.quote.amount.total_minor
      )
        throw new Error("snapshot");
      const pending = pendingOrderPayment(context, order.order_id);
      setView(value);
      setMarker(pending);
      setMessage((old) => (old === "failed" ? null : old));
    } catch {
      if (version === epoch.current && request === readVersion.current) {
        setView(null);
        setMessage("failed");
      }
    } finally {
      if (version === epoch.current && request === readVersion.current)
        setReading(false);
    }
  }

  useEffect(() => {
    const version = ++epoch.current;
    setView(null);
    setMarker(null);
    setMessage(null);
    void read(version);
    const refresh = () => {
      if (!working.current) void read(version);
    };
    // Returning from the provider can only GET. No mutation belongs to a focus,
    // pageshow, storage event or effect (including React StrictMode remount).
    window.addEventListener("focus", refresh);
    window.addEventListener("pageshow", refresh);
    const changed = (event: StorageEvent) => {
      if (
        event.key === null ||
        event.key === `commerce-order-payment-v1:${context}:${order.order_id}`
      )
        refresh();
    };
    window.addEventListener("storage", changed);
    return () => {
      epoch.current++;
      window.removeEventListener("focus", refresh);
      window.removeEventListener("pageshow", refresh);
      window.removeEventListener("storage", changed);
    };
  }, [context, order, refreshToken]);

  const fresh =
    !marker &&
    view?.payment_state === "NOT_STARTED" &&
    view.commercial_state === "DRAFT" &&
    view.handoff_state === "NONE" &&
    view.methods.length === 1;
  const recover =
    marker?.stage === "prepare" &&
    view &&
    ((view.payment_state === "NOT_STARTED" &&
      view.commercial_state === "DRAFT" &&
      view.handoff_state === "NONE") ||
      (view.payment_state === "PENDING" &&
        view.commercial_state === "AWAITING_PAYMENT" &&
        view.handoff_state === "PREPARED"));
  const available = fresh || recover;

  async function pay() {
    if (working.current || busy || reading || !available) return;
    working.current = true;
    setSending(true);
    busyCallback.current(true);
    setMessage(null);
    const version = epoch.current;
    const selectedAtStart = selection.current;
    let destination: ReturnType<typeof openPaymentDestination> | undefined;
    try {
      // Must run in the click's activation BEFORE the first await.
      destination = openPaymentDestination(locale);
      await payOrder({
        context,
        order,
        locale,
        method: view?.methods[0],
        destination,
        isCurrent: () => epoch.current === version && selectedAtStart(),
      });
      if (epoch.current === version) setMessage("submitted");
    } catch {
      destination?.close();
      if (epoch.current === version)
        setMessage(destination ? "uncertain" : "blocked");
    } finally {
      working.current = false;
      busyCallback.current(false);
      if (epoch.current === version) {
        setSending(false);
        await read(version);
      }
    }
  }

  return (
    <section
      className="order-payment"
      data-testid="order-payment"
      aria-labelledby="payment-title"
      aria-busy={reading || sending}
    >
      <h2 id="payment-title">{copy.title}</h2>
      {view && (
        <>
          {view.test_mode && (
            <p className="payment-test-mode" data-testid="payment-test-mode">
              {copy.test}
            </p>
          )}
          <p
            className="payment-state"
            data-testid="payment-status"
            data-state={view.payment_state}
          >
            {copy.paymentState}: {copy[view.payment_state]}
          </p>
          <p className="order-note" data-testid="payment-commercial-status">
            {copy.orderState}: {orderCopy[locale][view.commercial_state]}
          </p>
        </>
      )}
      {message && (
        <p
          role={message === "submitted" ? "status" : "alert"}
          data-testid="payment-error"
        >
          {copy[message]}
        </p>
      )}
      {sending ? (
        <p role="status">{copy.sending}</p>
      ) : reading ? (
        <p role="status">{copy.loading}</p>
      ) : available ? (
        <>
          {fresh && view && (
            <p>
              {
                view.methods[0][
                  locale === "en"
                    ? "name_en"
                    : locale === "zh-TW"
                      ? "name_hant"
                      : "name_hans"
                ]
              }
            </p>
          )}
          <p className="order-note">{copy.explain}</p>
          <button
            className="payment-pay"
            data-testid="pay-order"
            disabled={busy}
            onClick={() => void pay()}
          >
            {recover ? copy.recover : copy.pay}
          </button>
        </>
      ) : (
        view && (
          <p className="order-note">
            {view.payment_state === "NOT_STARTED" && !marker
              ? copy.unavailable
              : view.payment_state === "PENDING"
                ? copy.readOnly
                : ""}
          </p>
        )
      )}
    </section>
  );
}
