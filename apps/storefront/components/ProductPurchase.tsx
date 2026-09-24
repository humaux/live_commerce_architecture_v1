"use client";

import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import {
  BuyerClientError,
  initializeBuyerSession,
  readBuyerSession,
  resetBuyerSession,
} from "../lib/buyer-client";
import { purchaseCopy } from "../lib/purchase-copy";
import {
  cartSelection,
  assertPurchaseContext,
  optionKey,
  lineSubtotal,
  pendingPurchase,
  purchasePage,
  readPurchase,
  validCart,
  validOption,
  validProduct,
  validQuote,
  writePurchase,
} from "../lib/purchase";
import type { Cart, Product, Option, Quote } from "../lib/purchase";

export default function ProductPurchase({
  locale,
  productID,
  demonstration = false,
}: {
  locale: Locale;
  productID: string;
  demonstration?: boolean;
}) {
  const copy = purchaseCopy[locale];
  const [products, setProducts] = useState<Product[]>([]);
  const [cursor, setCursor] = useState("");
  const [context, setContext] = useState("");
  const [cart, setCart] = useState<Cart | null>(null);
  const [sku, setSKU] = useState("");
  const [quantity, setQuantity] = useState("1");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<
    "failed" | "session" | "conflict" | "pending" | null
  >(null);
  const [deliveryOpen, setDeliveryOpen] = useState(false);
  const [options, setOptions] = useState<Option[]>([]);
  const [optionCursor, setOptionCursor] = useState("");
  const [method, setMethod] = useState("");
  const [quote, setQuote] = useState<Quote | null>(null);
  const [pending, setPending] = useState(false);
  const epoch = useRef(0);
  const currentContext = useRef("");
  const working = useRef(false);
  const deliveryRef = useRef<HTMLElement>(null);
  const quoteRef = useRef<HTMLElement>(null);
  const selected = products.find((p) => p.sku_id === sku);
  const subtotal = selected
    ? lineSubtotal(selected.price_minor, Number(quantity))
    : null;
  const chosen = options.find((o) => optionKey(o) === method);
  const draftKey = (ctx: string) =>
    `commerce-product-selection-v1:${ctx}:${productID}`;
  const money = (amount: number, currency: string) => {
    const formatter = new Intl.NumberFormat(locale, {
      style: "currency",
      currency,
    });
    return formatter.format(
      amount / 10 ** (formatter.resolvedOptions().maximumFractionDigits ?? 2),
    );
  };

  function showError(reason: unknown) {
    if (
      reason instanceof BuyerClientError &&
      (["context_changed", "requires_reset"].includes(reason.code) ||
        reason.status === 401)
    ) {
      setError("session");
      setCart(null);
      setQuote(null);
    } else if (reason instanceof BuyerClientError && reason.status === 409)
      setError("conflict");
    else setError("failed");
  }

  async function load() {
    const version = ++epoch.current;
    setLoading(true);
    setPending(false);
    setError(null);
    setQuote(null);
    setDeliveryOpen(false);
    try {
      const session = await initializeBuyerSession();
      if (!session.context) throw new BuyerClientError("requires_reset");
      const ctx = session.context;
      let [page, current] = await Promise.all([
        purchasePage(
          `catalog?product_id=${productID}&limit=100`,
          ctx,
          validProduct,
        ),
        readPurchase("cart", ctx, validCart),
      ]);
      if (version !== epoch.current) return;
      if (
        page.items.some(
          (p) => p.product_id !== productID || p.currency !== current.currency,
        )
      )
        throw new BuyerClientError("invalid_response");
      const raw = sessionStorage.getItem(draftKey(ctx));
      let saved: { sku: string; quantity: string } | null = null;
      if (raw) {
        try {
          const d = JSON.parse(raw);
          if (
            typeof d.sku === "string" &&
            /^[0-9a-f-]{36}$/.test(d.sku) &&
            typeof d.quantity === "string" &&
            /^\d{1,10}$/.test(d.quantity)
          )
            saved = d;
        } catch {
          /* Optional selection only, not a command journal. */
        }
      }
      const visited = new Set<string>();
      while (
        saved &&
        !page.items.some((p) => p.sku_id === saved.sku) &&
        page.next_cursor
      ) {
        if (visited.has(page.next_cursor) || visited.size >= 100)
          throw new BuyerClientError("invalid_response");
        visited.add(page.next_cursor);
        const more = await purchasePage(
          `catalog?product_id=${productID}&limit=100&cursor=${encodeURIComponent(page.next_cursor)}`,
          ctx,
          validProduct,
        );
        if (
          more.items.some(
            (p) =>
              p.product_id !== productID || p.currency !== current.currency,
          )
        )
          throw new BuyerClientError("invalid_response");
        page = {
          items: [...page.items, ...more.items],
          next_cursor: more.next_cursor,
        };
      }
      await assertPurchaseContext(ctx);
      if (version !== epoch.current) return;
      currentContext.current = ctx;
      setContext(ctx);
      setCart(current);
      setProducts(page.items);
      setCursor(page.next_cursor);
      let selectedID =
        page.items.find((p) => current.items.some((i) => i.sku_id === p.sku_id))
          ?.sku_id ??
        page.items[0]?.sku_id ??
        "";
      let qty = String(
        current.items.find((i) => i.sku_id === selectedID)?.quantity ?? 1,
      );
      if (saved) {
        selectedID = page.items.some((p) => p.sku_id === saved.sku)
          ? saved.sku
          : "";
        qty = saved.quantity;
        if (!selectedID) setError("conflict");
      }
      setSKU(selectedID);
      setQuantity(qty);
      const unresolved = pendingPurchase(ctx) !== null;
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
          previous.cart_version === current.version
        )
          setQuote(previous);
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
    };
  }, [productID]); // Route locale never changes account/currency.

  useEffect(() => {
    if (!context) return;
    let active = true;
    const check = async () => {
      try {
        await assertPurchaseContext(context);
      } catch (reason) {
        if (active) {
          epoch.current++;
          showError(reason);
        }
      }
    };
    const changed = (event: StorageEvent) => {
      if (event.key === "commerce-buyer-pending-v1") void check();
    };
    window.addEventListener("storage", changed);
    window.addEventListener("focus", check);
    return () => {
      active = false;
      window.removeEventListener("storage", changed);
      window.removeEventListener("focus", check);
    };
  }, [context]);

  function select(nextSKU: string, nextQuantity: string) {
    setSKU(nextSKU);
    setQuantity(nextQuantity);
    setQuote(null);
    setDeliveryOpen(false);
    try {
      sessionStorage.setItem(
        draftKey(context),
        JSON.stringify({ sku: nextSKU, quantity: nextQuantity }),
      );
    } catch {
      setError("failed");
    }
  }

  async function act(fn: (isCurrent: () => boolean) => Promise<void>) {
    if (working.current) return;
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
        if (currentContext.current)
          setPending(pendingPurchase(currentContext.current) !== null);
      } catch {
        setPending(true);
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
      validOption,
    );
    if (version !== epoch.current || context !== currentContext.current) return;
    const rows = page.items.filter((o) => o.currency === cart?.currency);
    setOptions((old) => (nextCursor ? [...old, ...rows] : rows));
    setOptionCursor(page.next_cursor);
    if (!nextCursor) setMethod(rows[0] ? optionKey(rows[0]) : "");
    setDeliveryOpen(true);
    requestAnimationFrame(() =>
      deliveryRef.current?.scrollIntoView({ behavior: "auto", block: "start" }),
    );
  }

  return (
    <>
      <header className="shop-header">
        <span>{copy.store}</span>
        <label className="locale">
          <span className="sr-only">{copy.language}</span>
          <select
            value={locale}
            disabled={busy}
            onChange={(event) => {
              try {
                sessionStorage.setItem(
                  draftKey(context),
                  JSON.stringify({ sku, quantity }),
                );
                window.location.assign(
                  `/${event.target.value}/products/${productID}`,
                );
              } catch {
                setError("failed");
              }
            }}
          >
            <option value="zh-CN">简体中文</option>
            <option value="zh-TW">繁體中文</option>
            <option value="en">English</option>
          </select>
        </label>
      </header>
      {demonstration && <p className="demonstration">{copy.demonstration}</p>}
      <main className="purchase-main" aria-busy={loading || busy}>
        {error && (
          <div role="alert" className="purchase-error">
            <p>{copy[error]}</p>
            {pending && error !== "session" ? (
              <button
                disabled={busy}
                onClick={() =>
                  void act(async (isCurrent) => {
                    const result = await writePurchase(context);
                    if (!isCurrent()) return;
                    if (result.kind === "cart") {
                      setCart(result.value);
                      await deliveryOptions();
                    } else setQuote(result.value);
                    setPending(false);
                  })
                }
              >
                {copy.recover}
              </button>
            ) : (
              <button
                disabled={busy}
                onClick={() =>
                  void act(async () => {
                    if (error === "session") {
                      const current = await readBuyerSession();
                      if (current.context && current.state !== "active")
                        await resetBuyerSession(current.context);
                    }
                    await load();
                  })
                }
              >
                {error === "session" ? copy.renew : copy.retry}
              </button>
            )}
          </div>
        )}
        {loading ? (
          <p role="status">{copy.loading}</p>
        ) : !products.length ? (
          <p>{copy.empty}</p>
        ) : (
          <>
            <section className="product-detail" aria-labelledby="product-title">
              <h1 id="product-title">{selected?.name ?? products[0].name}</h1>
              {selected && (
                <p className="unit-price">
                  {money(selected.price_minor, selected.currency)}
                </p>
              )}
              <p className="description">
                {selected?.description ?? products[0].description}
              </p>
            </section>
            <fieldset className="sku-options" disabled={busy || pending}>
              <legend>{copy.choose}</legend>
              {products.map((p) => (
                <label
                  key={p.sku_id}
                  className={p.sku_id === sku ? "sku-row selected" : "sku-row"}
                >
                  <input
                    type="radio"
                    name="sku"
                    value={p.sku_id}
                    checked={p.sku_id === sku}
                    onChange={() => select(p.sku_id, quantity)}
                  />
                  <span>{p.sku_code}</span>
                </label>
              ))}
              {cursor && (
                <button
                  className="text-button"
                  onClick={() =>
                    void act(async (isCurrent) => {
                      const page = await purchasePage(
                        `catalog?product_id=${productID}&limit=100&cursor=${encodeURIComponent(cursor)}`,
                        context,
                        validProduct,
                      );
                      if (!isCurrent()) return;
                      if (
                        page.items.some(
                          (p) =>
                            p.product_id !== productID ||
                            p.currency !== cart?.currency,
                        )
                      )
                        throw new BuyerClientError("invalid_response");
                      setProducts((old) =>
                        Array.from(
                          new Map(
                            [...old, ...page.items].map((p) => [p.sku_id, p]),
                          ).values(),
                        ),
                      );
                      setCursor(page.next_cursor);
                    })
                  }
                >
                  {copy.more}
                </button>
              )}
            </fieldset>
            <div className="quantity-row">
              <label htmlFor="quantity">{copy.quantity}</label>
              <div className="stepper">
                <button
                  aria-label={copy.decrease}
                  disabled={busy || pending || Number(quantity) <= 1}
                  onClick={() => select(sku, String(Number(quantity) - 1))}
                >
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M5 12h14" />
                  </svg>
                </button>
                <input
                  id="quantity"
                  type="number"
                  inputMode="numeric"
                  min="1"
                  max="1000000000"
                  step="1"
                  value={quantity}
                  disabled={busy || pending}
                  onChange={(e) => select(sku, e.target.value)}
                  aria-invalid={subtotal === null}
                />
                <button
                  aria-label={copy.increase}
                  disabled={
                    busy || pending || Number(quantity) >= 1_000_000_000
                  }
                  onClick={() => select(sku, String(Number(quantity) + 1))}
                >
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M5 12h14M12 5v14" />
                  </svg>
                </button>
              </div>
            </div>
            {subtotal === null && <p role="status">{copy.amountInvalid}</p>}
            {!!cart?.items.some((i) => i.sku_id !== sku) && (
              <p className="cart-note">{copy.savedCart}</p>
            )}
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
                  <>
                    <label className="delivery-label" htmlFor="delivery">
                      {copy.selectDelivery}
                    </label>
                    <select
                      id="delivery"
                      value={method}
                      disabled={busy || pending}
                      onChange={(e) => {
                        setMethod(e.target.value);
                        setQuote(null);
                      }}
                    >
                      {options.map((o) => (
                        <option key={optionKey(o)} value={optionKey(o)}>
                          {locale === "zh-CN"
                            ? o.name_hans
                            : locale === "zh-TW"
                              ? o.name_hant
                              : o.name_en}{" "}
                          · {o.country}
                        </option>
                      ))}
                    </select>
                    {chosen?.delivery_kind !== "home" && <p>{copy.cvs}</p>}
                  </>
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
                <ul>
                  {quote.lines.map((line) => (
                    <li key={line.sku_id}>
                      <span>
                        {line.name} · {line.code} × {line.quantity}
                      </span>
                      <span>
                        {money(
                          line.unit_price_minor * line.quantity,
                          quote.currency,
                        )}
                      </span>
                    </li>
                  ))}
                </ul>
                <dl>
                  <div>
                    <dt>{copy.shipping}</dt>
                    <dd>
                      {money(quote.amount.shipping_minor, quote.currency)}
                    </dd>
                  </div>
                  <div>
                    <dt>{copy.taxes}</dt>
                    <dd>{money(quote.amount.tax_minor, quote.currency)}</dd>
                  </div>
                  <div>
                    <dt>{copy.discount}</dt>
                    <dd>
                      {money(quote.amount.discount_minor, quote.currency)}
                    </dd>
                  </div>
                  <div className="total">
                    <dt>{copy.total}</dt>
                    <dd>{money(quote.amount.total_minor, quote.currency)}</dd>
                  </div>
                </dl>
                <p>
                  {copy.expires}{" "}
                  {new Intl.DateTimeFormat(locale, {
                    dateStyle: "short",
                    timeStyle: "short",
                  }).format(new Date(quote.expires_at))}
                </p>
                <p>{copy.noPayment}</p>
                <button
                  className="text-button"
                  disabled={busy || pending}
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
          </>
        )}
      </main>
      {selected && (
        <footer className="purchase-footer">
          <div className="footer-inner">
            <div>
              <span>{quote ? copy.total : copy.subtotal}</span>
              <strong>
                {quote
                  ? money(quote.amount.total_minor, quote.currency)
                  : subtotal === null
                    ? "—"
                    : money(subtotal, selected.currency)}
              </strong>
              <small>{quote ? copy.unpaid : copy.shippingLater}</small>
            </div>
            <button
              className="primary"
              disabled={
                busy ||
                loading ||
                error === "session" ||
                pending ||
                subtotal === null ||
                (!quote &&
                  deliveryOpen &&
                  (!chosen || chosen.delivery_kind !== "home"))
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
                  if (!cart || subtotal === null) return;
                  if (!deliveryOpen) {
                    const result = await writePurchase(context, {
                      kind: "cart",
                      body: cartSelection(cart, sku, Number(quantity)),
                    });
                    if (!isCurrent()) return;
                    if (result.kind !== "cart")
                      throw new BuyerClientError("invalid_response");
                    setCart(result.value);
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
                    setQuote(result.value);
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
            </button>
          </div>
        </footer>
      )}
    </>
  );
}
