"use client";

// Purpose: checkout delivery/address/payment over the buyer cart, with claim merge disclosure.
// Depends on: buyer BFF/recovery, checkout display components, checkout-copy and codFooter (OC-09b bottom bar).
// Used by: /{locale}/checkout, including claim and CVS returns.
// Invariants: server quote matches its option; changed totals need acknowledgment; unknown checkout withholds payable and replays its original intent/key; brand-change contact stays ephemeral.

// Checkout surface at /{locale}/checkout (lib/routes.ts: the CVS map return allowlist): the delivery choice, quotation,
// address, payment and order screens over the buyer's EXISTING multi-SKU cart. It is the former single-product purchase page
// minus the product picker: items are added on the product page / drawer / cart page (CartProvider), this page starts at the
// cart and ends at the placed order. All crash-recovery journals (pending cart/quote/destination/checkout, order locator) and
// data-testids are unchanged, so the order, payment, CVS and bank-transfer flows below behave exactly as before.
// (BFF /api/buyer/{cart,checkout-options,quotes,destination,checkout,orders} -> Go /v1/buyer/*). Delivery list (checkout-options): home and the four CVS chains; a chain the store cannot
// sell yet arrives as {available:false, reason} and is shown disabled "Coming soon"
// (contracts/taiwan-cvs-logistics-v1.md §5.1). The CVS store picker itself lives in OrderFlow/CvsPickup and
// returns to this route with ?cvs_selection= (§5.2), which is why the route keeps checkout state across a reload.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import {
  BuyerClientError,
  initializeBuyerSession,
  readBuyerSession,
  resetBuyerSession,
} from "../lib/buyer-client";
import { purchaseCopy } from "../lib/purchase-copy";
import { orderCopy } from "../lib/order-copy";
import { isCvsKind, type CvsDraft, type CvsKind } from "../lib/cvs-contract";
import { codFooter } from "../lib/cod-copy";
import { checkoutCopy } from "../lib/checkout-copy";
import { changedCheckoutQuote, rememberCheckoutQuote } from "../lib/checkout-review";
import OrderFlow, { OrderDetails } from "./OrderFlow";
import CheckoutPendingReview from "./checkout/CheckoutPendingReview";
import CheckoutQuoteAmounts from "./checkout/CheckoutQuoteAmounts";
import CheckoutDeliveryOptions from "./checkout/CheckoutDeliveryOptions";
import PromoCode from "./PromoCode";
import OrderHistory from "./OrderHistory";
import { CartLines } from "./CartLines";
import { useCart } from "./CartProvider";
import { useCartDetails } from "./CartLines";
import { checkoutPath } from "../lib/routes";
import { shopCopy } from "../lib/shop-copy";
import { formatMoney } from "../lib/money";
import Link from "next/link";
import { claimCopy } from "../lib/claim-copy";
import { historyCopy } from "../lib/history-copy";
import {
  assertPurchaseContext,
  optionKey,
  pendingPurchase,
  purchasePage,
  readPurchase,
  validCart,
  validOptionRow,
  isUnavailable,
  validQuote,
  writePurchase,
  knownOrderID,
  readOrder,
  writeCheckout,
  orderRecoveryRequired,
  forgetAddressAttempt,
  continueShopping,
} from "../lib/purchase";
import type { Cart, OptionRow, Quote, Order } from "../lib/purchase";

/** Renders the existing purchase lifecycle; consumes the non-sensitive claim origin hint. */
export default function CheckoutFlow({
  locale: initialLocale,
  demonstration = false,
}: {
  locale: Locale;
  demonstration?: boolean;
}) {
  const [locale, setLocale] = useState(initialLocale);
  const [fromClaim, setFromClaim] = useState<"claim" | "claim-recovery" | null>(null);
  useEffect(() => {
    const url = new URL(window.location.href);
    const source = url.searchParams.get("from");
    if (source !== "claim" && source !== "claim-recovery") return;
    setFromClaim(source);
    url.searchParams.delete("from");
    window.history.replaceState(window.history.state, "", url.pathname + url.search + url.hash);
  }, []);
  const copy = purchaseCopy[locale];
  const [context, setContext] = useState("");
  const [cart, setCart] = useState<Cart | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [paymentBusy, setPaymentBusy] = useState(false);
  const paymentWorking = useRef(false);
  const onPaymentBusy = (value: boolean) => {
    paymentWorking.current = value;
    setPaymentBusy(value);
  };
  const [error, setError] = useState<
    "failed" | "session" | "conflict" | "pending" | null
  >(null);
  const [deliveryOpen, setDeliveryOpen] = useState(false);
  const [options, setOptions] = useState<OptionRow[]>([]);
  const [optionCursor, setOptionCursor] = useState("");
  // OC-09b: the selected cash-on-delivery option's server surcharge reported by OrderFlow (null = card or another mode).
  const [codFee, setCodFee] = useState<number | null>(null);
  const [method, setMethod] = useState("");
  const [quote, setQuote] = useState<Quote | null>(null);
  const [quoteChange, setQuoteChange] = useState<number | null>(null);
  const [reviewState, setReviewState] = useState<{ quoteID: string; ready: boolean; timing: string; quoteReady?: boolean } | null>(null);
  const v2 = checkoutCopy[locale];
  // Contact survives an explicit brand change in component memory only, never a new storage journal.
  const [cvsResume, setCvsResume] = useState<{ context: string; quoteID: string; draft: CvsDraft } | null>(null);
  const [pending, setPending] = useState(false);
  const [pendingKind, setPendingKind] = useState<string | null>(null);
  const [recoveryCountry, setRecoveryCountry] = useState<string | null>(null);
  const [orderLocked, setOrderLocked] = useState(false);
  const [order, setOrder] = useState<Order | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);
  const historyVisible = useRef(false);
  historyVisible.current = historyOpen;
  const epoch = useRef(0);
  const currentContext = useRef("");
  const currentOrder = useRef<Order | null>(null);
  const working = useRef(false);
  const deliveryRef = useRef<HTMLElement>(null);
  const quoteRef = useRef<HTMLElement>(null);
  const footerRef = useRef<HTMLElement>(null);
  const [footerHeight, setFooterHeight] = useState(180);
  useEffect(() => {
    const footer = footerRef.current;
    if (!footer) return;
    const observer = new ResizeObserver(() => setFooterHeight(footer.getBoundingClientRect().height));
    observer.observe(footer);
    return () => observer.disconnect();
  }, [loading, orderLocked, order, cart, historyOpen]);
  const shell = useCart();
  const { details } = useCartDetails();
  const subtotal = details ? details.subtotal : null;
  const found = options.find((o) => optionKey(o) === method);
  // Unavailable rows are listed disabled, so they can never be the chosen delivery.
  const chosen = found && !isUnavailable(found) ? found : undefined;
  // One money format for the whole storefront (lib/money.ts): the bar, the quotation, the order and the history read like the cart lines.
  const money = (amount: number, currency: string) => formatMoney(locale, amount, currency);

  // Only quote identifiers and server totals, scoped to this buyer context; never recipient data or authority to charge.
  // Keeping the previous total through delivery selection AND reload prevents a changed quote silently becoming accepted.
  function acceptQuote(value: Quote, ctx = currentContext.current) {
    const old = changedCheckoutQuote(value, ctx);
    setQuoteChange(old);
    setReviewState(null);
    setQuote(value);
  }

  function requireFeeConfirmation() {
    if (!quote) return;
    const old = quote.cod_due_minor ?? quote.amount.total_minor;
    rememberCheckoutQuote(quote, context, old);
    setQuoteChange(old);
  }

  function showError(reason: unknown) {
    if (
      reason instanceof BuyerClientError &&
      (["context_changed", "requires_reset"].includes(reason.code) ||
        reason.status === 401)
    ) {
      setError("session");
      setCart(null);
      setQuote(null);
      setOrder(null);
      currentOrder.current = null;
      setHistoryOpen(false);
    } else if (reason instanceof BuyerClientError && reason.status === 409)
      setError("conflict");
    else setError("failed");
  }

  function syncRecovery(ctx: string) {
    const waiting = pendingPurchase(ctx);
    setPending(waiting !== null);
    setPendingKind(waiting?.kind ?? null);
    setRecoveryCountry(
      waiting?.kind === "destination" ? waiting.body.country : null,
    );
    const id = knownOrderID(ctx);
    setOrderLocked(
      waiting?.kind === "checkout" ||
        waiting?.kind === "next-cart" ||
        id !== null,
    );
    return { waiting, id };
  }

  function acceptOrder(value: Order) {
    currentOrder.current = value;
    setOrder(value);
    setOrderLocked(true);
    setQuote(null);
    setDeliveryOpen(false);
    forgetAddressAttempt();
  }

  async function load() {
    const version = ++epoch.current;
    setLoading(true);
    setPending(false);
    setError(null);
    setQuote(null);
    setOrder(null);
    currentOrder.current = null;
    setDeliveryOpen(false);
    try {
      // Capture the expired context BEFORE initialization rejects it. This
      // preserves the reset guard for an unknown or already-known old order.
      const before = await readBuyerSession();
      if (before.context) {
        if (version !== epoch.current) return;
        currentContext.current = before.context;
        setContext(before.context);
        syncRecovery(before.context);
      }
      const session = await initializeBuyerSession();
      if (!session.context) throw new BuyerClientError("requires_reset");
      const ctx = session.context;
      if (version !== epoch.current) return;
      currentContext.current = ctx;
      setContext(ctx);
      const recovery = syncRecovery(ctx);
      if (["checkout", "next-cart"].includes(recovery.waiting?.kind ?? ""))
        setError("pending");
      if (recovery.id) {
        const existing = await readOrder(ctx, recovery.id);
        if (version !== epoch.current) return;
        // The locator can change while this initial GET is in flight. A late
        // old-order response must not undo another tab's explicit continuation.
        if (knownOrderID(ctx) === recovery.id) acceptOrder(existing);
        else await load();
        return;
      }
      const current = await readPurchase("cart", ctx, validCart);
      if (version !== epoch.current) return;
      await assertPurchaseContext(ctx);
      if (version !== epoch.current) return;
      currentContext.current = ctx;
      setContext(ctx);
      setCart(current);
      void shell.refresh();
      const unresolved = syncRecovery(ctx).waiting !== null;
      setPending(unresolved);
      if (unresolved) setError("pending");
      const quoteID = sessionStorage.getItem(
        `commerce-purchase-quote-v1:${ctx}`,
      );
      if (quoteID && /^[0-9a-f-]{36}$/.test(quoteID)) {
        const previous = await readPurchase(
          `quotes/${quoteID}`,
          ctx,
          validQuote,
        );
        if (
          version === epoch.current &&
          previous.cart_version === current.version &&
          previous.cart_id === current.id
        )
          acceptQuote(previous, ctx);
      }
    } catch (reason) {
      if (version === epoch.current) showError(reason);
    } finally {
      if (version === epoch.current) setLoading(false);
    }
  }

  useEffect(() => {
    void load();
    return () => {
      epoch.current++;
      forgetAddressAttempt();
    };
  }, []); // Route locale never changes account/currency.

  useEffect(() => {
    setLocale(initialLocale);
    document.documentElement.lang = initialLocale;
  }, [initialLocale]);

  useEffect(() => {
    if (order || orderLocked)
      requestAnimationFrame(() =>
        document
          .querySelector(
            order ? '[data-testid="order-section"]' : ".purchase-error",
          )
          ?.scrollIntoView({ behavior: "auto", block: "start" }),
      );
  }, [order?.order_id, orderLocked]);

  useEffect(() => {
    if (!context) return;
    let active = true;
    const check = async () => {
      // Opening/returning to the child can focus this tab mid-handoff. Do not
      // replace the selected order object underneath its one-shot transaction.
      if (paymentWorking.current) return;
      const version = epoch.current;
      try {
        await assertPurchaseContext(context);
        if (active && version === epoch.current) {
          const recovery = syncRecovery(context);
          if (["checkout", "next-cart"].includes(recovery.waiting?.kind ?? ""))
            setError("pending");
          if (recovery.id) {
            const current = await readOrder(context, recovery.id);
            if (
              active &&
              version === epoch.current &&
              knownOrderID(context) === recovery.id
            )
              acceptOrder(current);
          } else if (!recovery.waiting && currentOrder.current) {
            await load();
          }
        }
      } catch (reason) {
        if (active && version === epoch.current) {
          epoch.current++;
          showError(reason);
        }
      }
    };
    const changed = (event: StorageEvent) => {
      if (
        event.key === "commerce-buyer-pending-v1" ||
        event.key === `commerce-purchase-pending-v1:${context}` ||
        event.key === `commerce-purchase-order-v1:${context}`
      ) {
        // Another tab finished continuation. Reload current cart/quote rather
        // than leaving this tab's old order/quotation actionable.
        void check();
      }
    };
    window.addEventListener("storage", changed);
    window.addEventListener("focus", check);
    return () => {
      active = false;
      window.removeEventListener("storage", changed);
      window.removeEventListener("focus", check);
    };
  }, [context]);

  async function act(fn: (isCurrent: () => boolean) => Promise<void>) {
    if (working.current || paymentWorking.current) return;
    const version = epoch.current;
    const startedContext = currentContext.current;
    const isCurrent = () =>
      version === epoch.current && startedContext === currentContext.current;
    working.current = true;
    setBusy(true);
    setError(null);
    try {
      await fn(isCurrent);
    } catch (reason) {
      if (isCurrent()) showError(reason);
    } finally {
      try {
        if (currentContext.current) syncRecovery(currentContext.current);
      } catch {
        setPending(true);
        setOrderLocked(true);
      }
      working.current = false;
      setBusy(false);
    }
  }

  async function deliveryOptions(nextCursor = "") {
    const version = epoch.current;
    const page = await purchasePage(
      `checkout-options?limit=100${nextCursor ? `&cursor=${encodeURIComponent(nextCursor)}` : ""}`,
      context,
      validOptionRow,
    );
    if (version !== epoch.current || context !== currentContext.current) return;
    const rows = page.items.filter((o) => o.currency === cart?.currency);
    setOptions((old) => (nextCursor ? [...old, ...rows] : rows));
    setOptionCursor(page.next_cursor);
    if (!nextCursor) {
      const first = rows.find((o) => !isUnavailable(o));
      setMethod(first ? optionKey(first) : "");
    }
    setDeliveryOpen(true);
    requestAnimationFrame(() =>
      deliveryRef.current?.scrollIntoView({ behavior: "auto", block: "start" }),
    );
  }

  return (
    <>
      <style>{`
        body:has(.checkout-fixed-bar) .sf-footer { padding-bottom: calc(${footerHeight + 40}px + env(safe-area-inset-bottom, 0px)); }
        @media (max-width: 600px) {
          .checkout-fixed-bar .footer-inner { grid-template-columns: minmax(0, 1fr); gap: 8px; }
          .checkout-fixed-bar .footer-inner > div { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 4px 12px; }
          .checkout-fixed-bar .footer-inner small { grid-column: 1 / -1; }
          .checkout-fixed-bar .footer-inner > button { width: 100%; min-height: 52px; }
        }
      `}</style>
      {demonstration && <p className="demonstration">{copy.demonstration}</p>}
      <main
        className="purchase-main"
        style={{ paddingBottom: footerHeight + 40 }}
        aria-busy={loading || busy || paymentBusy}
      >
        {fromClaim && <p role="status" className="order-note" data-testid="claim-checkout-notice">{fromClaim === "claim-recovery" ? claimCopy[locale].recovery : claimCopy[locale].checkoutNotice}</p>}
        <nav
          className="purchase-navigation"
          aria-label={historyCopy[locale].title}
        >
          <button
            data-testid="toggle-order-history"
            disabled={
              busy || paymentBusy || loading || pending || error === "session"
            }
            onClick={() => setHistoryOpen((value) => !value)}
          >
            {historyOpen ? historyCopy[locale].back : historyCopy[locale].title}
          </button>
        </nav>
        {error && (
          <div role="alert" className="purchase-error">
            <p data-testid={pendingKind === "checkout" ? "checkout-unknown" : undefined}>
              {pendingKind === "checkout" ? v2.unknown : error === "session" && orderLocked
                ? orderCopy[locale].recovery
                : copy[error]}
            </p>
            {pending && error !== "session" ? (
              <button
                data-testid="recover-purchase"
                disabled={busy}
                onClick={() =>
                  void act(async (isCurrent) => {
                    if (pendingKind === "next-cart") {
                      await continueShopping(context);
                      if (isCurrent()) await load();
                      return;
                    }
                    if (pendingKind === "checkout") {
                      const current = await writeCheckout(context);
                      if (isCurrent()) acceptOrder(current);
                      return;
                    }
                    if (pendingKind === "destination") {
                      document
                        .querySelector('[data-testid="address-section"]')
                        ?.scrollIntoView({ behavior: "auto", block: "start" });
                      return;
                    }
                    const result = await writePurchase(context);
                    if (!isCurrent()) return;
                    if (result.kind === "cart") {
                      setCart(result.value);
                      await deliveryOptions();
                    } else acceptQuote(result.value);
                    setPending(false);
                  })
                }
              >
                {pendingKind === "checkout" ? v2.query : pendingKind === "destination"
                  ? orderCopy[locale].review
                  : copy.recover}
              </button>
            ) : (
              <button
                disabled={busy}
                onClick={() =>
                  void act(async () => {
                    if (error === "session") {
                      const current = await readBuyerSession();
                      if (
                        current.context &&
                        current.state !== "active" &&
                        orderRecoveryRequired(current.context)
                      )
                        throw new BuyerClientError("requires_reset");
                      if (current.context && current.state !== "active")
                        await resetBuyerSession(current.context);
                    }
                    await load();
                  })
                }
              >
                {error === "session" && !orderLocked ? copy.renew : copy.retry}
              </button>
            )}
          </div>
        )}
        {historyOpen && context && (
          <OrderHistory
            key={context}
            context={context}
            locale={locale}
            money={money}
            onError={showError}
            onPaymentBusy={onPaymentBusy}
          />
        )}
        {order && !historyOpen && (
          <>
            <OrderDetails
              context={context}
              order={order}
              locale={locale}
              money={money}
              busy={busy || paymentBusy}
              onPaymentBusy={onPaymentBusy}
              isSelected={(() => {
                const generation = epoch.current;
                return () =>
                  generation === epoch.current &&
                  !historyVisible.current &&
                  currentOrder.current?.order_id === order.order_id;
              })()}
              refresh={() =>
                void act(async (isCurrent) => {
                  const current = await readOrder(context, order.order_id);
                  if (isCurrent()) acceptOrder(current);
                })
              }
            />
            <div className="next-purchase">
              <p className="order-note">{historyCopy[locale].note}</p>
              <button
                data-testid="continue-shopping"
                disabled={busy || paymentBusy || pending || error === "session"}
                onClick={() =>
                  void act(async () => {
                    await continueShopping(context, order.order_id);
                    await load();
                  })
                }
              >
                {historyCopy[locale].next}
              </button>
            </div>
          </>
        )}
        {historyOpen ? null : loading ? (
          <p role="status">{copy.loading}</p>
        ) : order ? null : !cart || !cart.items.length ? (
          <div className="sf-empty" data-testid="checkout-empty">
            <p className="sf-empty__title">{shopCopy[locale].checkoutEmpty}</p>
            <Link className="sf-btn sf-btn--primary" href={`/${locale}/products`}>
              {shopCopy[locale].continueShopping}
            </Link>
          </div>
        ) : (
          <>
            <section className="checkout-lines" aria-labelledby="checkout-title">
              <h1 id="checkout-title">{shopCopy[locale].checkoutTitle}</h1>
              <CartLines locale={locale} readOnly />
              <Link className="sf-link" href={`/${locale}/cart`}>
                {shopCopy[locale].checkoutBack}
              </Link>
            </section>
            {deliveryOpen && (
              <section
                className="delivery-section"
                ref={deliveryRef}
                aria-labelledby="delivery-title"
              >
                <h2 id="delivery-title">{copy.deliveryTitle}</h2>
                {!options.length ? (
                  <p>{copy.noDelivery}</p>
                ) : (
                  <CheckoutDeliveryOptions
                    locale={locale} options={options} method={method} chosen={chosen}
                    subtotal={quote ? null : subtotal} currency={cart.currency}
                    disabled={busy || pending || orderLocked || error === "session"}
                    onSelect={(value) => { setMethod(value); setQuote(null); }}
                  />
                )}
                {optionCursor && (
                  <button
                    disabled={busy}
                    onClick={() =>
                      void act(() => deliveryOptions(optionCursor))
                    }
                  >
                    {copy.more}
                  </button>
                )}
              </section>
            )}
            {quote && (
              <section
                className="quotation"
                ref={quoteRef}
                aria-labelledby="quote-title"
              >
                <h2 id="quote-title">{copy.lines}</h2>
                <CheckoutQuoteAmounts quote={quote} locale={locale} money={money} />
                <p>{copy.unpaid}</p>
                {/* storefront-v2 §F: discount code; re-quotes this cart + delivery through writePurchase and replaces the quote. */}
                {!orderLocked && error !== "session" && (
                  <PromoCode context={context} quote={quote} locale={locale} busy={busy || pending} run={act} onQuote={acceptQuote} money={money} />
                )}
                <button className="quote-navigation" disabled={busy || pending || orderLocked || error === "session"}
                  onClick={() => quoteRef.current?.scrollIntoView({ behavior: "auto", block: "start" })}>{copy.viewQuote}</button>
                <button
                  className="text-button"
                  disabled={
                    busy || pending || orderLocked || error === "session"
                  }
                  onClick={() =>
                    void act(async (isCurrent) => {
                      await deliveryOptions();
                      if (isCurrent()) setQuote(null);
                    })
                  }
                >
                  {copy.changeDelivery}
                </button>
              </section>
            )}
            {quote && quoteChange !== null && <section data-testid="quote-reconfirm" role="status" style={{ marginTop: 24 }}>
              <p>{v2.changed}</p>
              <p>{money(quoteChange, quote.currency)} → {money(codFee === null ? quote.amount.total_minor : quote.cod_due_minor ?? quote.amount.total_minor, quote.currency)}</p>
              <button style={{ minHeight: 44, minWidth: 44 }} disabled={busy || pending || orderLocked || error === "session" || reviewState?.quoteID !== quote.id || !reviewState.quoteReady}
                onClick={() => { try {
                  rememberCheckoutQuote(quote, context, null);
                  setQuoteChange(null);
                } catch (reason) { showError(reason); } }}>{v2.confirm}</button>
            </section>}
            {(quote || pendingKind === "destination") &&
              cart &&
              !orderLocked &&
              error !== "session" && (
                <OrderFlow
                  key={`${context}:${quote?.id ?? "address-recovery"}`}
                  context={context}
                  cart={cart}
                  quote={quote}
                  recoveryCountry={recoveryCountry}
                  locale={locale}
                  busy={busy}
                  blocked={pending && pendingKind !== "destination"}
                  recoveringDestination={pendingKind === "destination"}
                  run={act}
                  money={money}
                  onCodFee={setCodFee}
                  reconfirmRequired={quoteChange !== null}
                  onFeeChanged={requireFeeConfirmation}
                  onQuoteRefresh={acceptQuote}
                  onReviewState={setReviewState}
                  cvsResume={cvsResume?.context === context && cvsResume.quoteID === quote?.id ? cvsResume.draft : undefined}
                  cvsKinds={options.filter((row) => !isUnavailable(row) && row.pickup_selection === "buyer_entered" && row.market_id === quote?.market_id && row.country === quote?.country && row.currency === quote?.currency && isCvsKind(row.delivery_kind)).map((row) => row.delivery_kind as CvsKind)}
                  onCvsBrand={(kind, draft) => {
                    void act(async (current) => {
                      if (!cart || !quote) return;
                      const target = options.find((row) => !isUnavailable(row) && row.pickup_selection === "buyer_entered" && row.market_id === quote.market_id && row.country === quote.country && row.currency === quote.currency && row.delivery_kind === kind);
                      if (!target) throw new BuyerClientError("invalid_response");
                      // The server recomputes the quote for this chain; client selection supplies no price or tenant.
                      const result = await writePurchase(context, { kind: "quote", body: { cart_version: cart.version, market_id: target.market_id, country: target.country, method: target.method } });
                      if (!current()) return;
                      if (result.kind !== "quote") throw new BuyerClientError("invalid_response");
                      setCvsResume({ context, quoteID: result.value.id, draft });
                      setMethod(optionKey(target));
                      acceptQuote(result.value);
                    });
                  }}
                  onOrder={acceptOrder}
                  reload={async () => {
                    // Requotation may unmount the form, so first resolve its
                    // PII-free destination journal through explicit confirmation.
                    if (pendingPurchase(context)?.kind === "destination")
                      return;
                    sessionStorage.removeItem(
                      `commerce-purchase-quote-v1:${context}`,
                    );
                    await load();
                  }}
                />
              )}
            {quote && pendingKind === "checkout" && <CheckoutPendingReview context={context} quote={quote} locale={locale} onCodFee={setCodFee} onReviewState={setReviewState} />}
          </>
        )}
        <div data-testid="checkout-content-end" />
      </main>
      {cart && cart.items.length > 0 && !historyOpen && !order && (!orderLocked || pendingKind === "checkout") && (
        <footer ref={footerRef} className={`purchase-footer checkout-fixed-bar${quote ? " has-quote" : ""}`} style={{ position: "fixed", insetInline: 0, bottom: 0 }}>
          <div className="footer-inner">
            {/* OC-09b: 「到貨需付」 shows the server quote due (codFooter) while cash on delivery is selected; hidden for card. */}
            {(() => {
              const due = quote && pendingKind !== "checkout"
                ? codFooter(locale, quote.cod_due_minor, codFee, (minor) => money(minor, quote.currency))
                : null;
              return due ? (
                <div data-testid="footer-cod-due">
                  <span>{due.label}</span>
                  <strong>{due.amount}</strong>
                  {due.note && <small>{due.note}</small>}
                  <small>{reviewState && reviewState.quoteID === quote?.id ? reviewState.timing : v2.cod}</small>
                </div>
              ) : (
                <div>
                  <span>{quote ? copy.orderTotal : copy.subtotal}</span>
                  <strong>
                    {pendingKind === "checkout"
                      ? "—"
                      : quote
                        ? money(quote.amount.total_minor, quote.currency)
                        : subtotal === null
                          ? "—"
                          : money(subtotal, cart.currency)}
                  </strong>
                  <small>{quote ? reviewState?.quoteID === quote.id ? reviewState.timing : v2.online : copy.shippingLater}</small>
                </div>
              );
            })()}
            {(quote || pendingKind === "checkout") ? <button type="submit" form="checkout-order-form" data-testid="checkout-submit" className="primary"
              disabled={busy || loading || pending || orderLocked || error === "session" || quoteChange !== null || !quote || reviewState?.quoteID !== quote.id || !reviewState.ready}>
              {pendingKind === "checkout" ? v2.unknown : v2.submit}
            </button> : <button
              className={quote ? "quote-navigation" : "primary"}
              disabled={
                busy ||
                loading ||
                error === "session" ||
                pending ||
                (!quote && deliveryOpen && !chosen)
              }
              onClick={() =>
                void act(async (isCurrent) => {
                  if (quote) {
                    quoteRef.current?.scrollIntoView({
                      behavior: "auto",
                      block: "start",
                    });
                    return;
                  }
                  if (!cart) return;
                  if (!deliveryOpen) {
                    // The cart was already written by the product page / cart page; checkout only reads delivery options.
                    setQuote(null);
                    await deliveryOptions();
                  } else if (chosen) {
                    const result = await writePurchase(context, {
                      kind: "quote",
                      body: {
                        cart_version: cart.version,
                        market_id: chosen.market_id,
                        country: chosen.country,
                        method: chosen.method,
                      },
                    });
                    if (!isCurrent()) return;
                    if (result.kind !== "quote")
                      throw new BuyerClientError("invalid_response");
                    acceptQuote(result.value);
                  }
                })
              }
            >
              {busy
                ? copy.loading
                : quote
                  ? copy.viewQuote
                  : deliveryOpen
                    ? copy.quote
                    : copy.delivery}
            </button>}
          </div>
        </footer>
      )}
    </>
  );
}
