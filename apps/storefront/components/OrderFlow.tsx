"use client";

// Local extension of approved B: native address form after the real quotation;
// no new wizard, payment claim or persistent address cache. Transport/CAS and
// receipt recovery live in purchase.ts, not in this rendering component.
// OrderDetails hosts <OrderPayment> (BFF orders/{id}/payment[/prepare|handoff|refresh|cancel]);
// its Refresh order button also fires the Stripe payment/refresh signal via paymentSignalRef.
// The shipment block renders Go GET /v1/buyer/orders/{id} `shipment` (BFF orders/{id}); no route of its own.
// CVS options (taiwan-cvs-logistics-v1 §5, §16): the address form is replaced by <CvsPickup> (BFF cvs-selections,
// cvs-selections/{id}/verify, cvs-stores -> Go /v1/buyer/cvs-*), the destination is written with kind=cvs_* +
// pickup_id (BFF PUT destination -> Go SetDestination) and Begin carries payment_mode (BFF POST checkout ->
// Go checkout.Begin). A pay-at-pickup order takes no Stripe step: OrderPayment is not mounted for it.
import { useEffect, useRef, useState } from "react";
import OrderPayment from "./OrderPayment";
import { ConsentChoices, noConsentChoices, submitCheckoutConsents } from "./ConsentChoices";
import CvsPickup, { CvsOrderStatus, type PickupHandle } from "./CvsPickup";
import BankTransfer from "./BankTransfer";
import { CodOrderStatus } from "./CodOrderStatus";
import type { Locale } from "@live-commerce/i18n";
import { BuyerClientError } from "../lib/buyer-client";
import { carrierNames, orderCopy } from "../lib/order-copy";
import { purchaseCopy } from "../lib/purchase-copy";
import { cvsCopy } from "../lib/cvs-copy";
import { bankTransferCopy } from "../lib/bank-transfer-copy";
import { codCopy } from "../lib/cod-copy";
import { isTransferErrorCode, validBuyerEmail, type TransferErrorCode } from "../lib/bank-transfer-contract";
import { isPromoErrorCode, type PromoErrorCode } from "../lib/promo-contract";
import { promoCopy } from "../lib/promo-copy";
import {
  isCvsErrorCode,
  isCvsKind,
  type CvsDraft,
  type CvsErrorCode,
  type CvsKind,
  type PaymentMode,
} from "../lib/cvs-contract";
import {
  checkoutInput,
  offeredPaymentModes,
  currentDestination,
  pendingPurchase,
  purchasePage,
  readPurchase,
  validCart,
  validOptionRow,
  isUnavailable,
  validDestinationWrite,
  writeDestination,
  writeCheckout,
} from "../lib/purchase";
import type {
  Cart,
  Destination,
  DestinationWrite,
  HomeAddress,
  Option,
  Order,
  Quote,
  Shipment,
} from "../lib/purchase";

type Fields = HomeAddress & { recipient_name: string; phone: string };
const empty: Fields = {
  recipient_name: "",
  phone: "",
  region: "",
  city: "",
  postal_code: "",
  line1: "",
  line2: "",
};
const fieldsOf = (d: DestinationWrite | Destination): Fields => ({
  recipient_name: d.recipient_name,
  phone: d.phone,
  ...d.home_address,
});
type Run = (work: (isCurrent: () => boolean) => Promise<void>) => Promise<void>;
type Money = (amount: number, currency: string) => string;
const noHome: HomeAddress = { region: "", city: "", postal_code: "", line1: "", line2: "" };
const inputs = [
  ["recipient_name", "name", 120, true],
  ["phone", "tel", 32, true],
  ["region", "address-level1", 100, false],
  ["city", "address-level2", 100, true],
  ["postal_code", "postal-code", 20, false],
  ["line1", "address-line1", 200, true],
  ["line2", "address-line2", 200, false],
] as const;

export default function OrderFlow({
  context,
  cart,
  quote,
  recoveryCountry,
  locale,
  busy,
  blocked,
  recoveringDestination,
  run,
  reload,
  onOrder,
  money,
}: {
  context: string;
  cart: Cart;
  quote: Quote | null;
  recoveryCountry: string | null;
  locale: Locale;
  busy: boolean;
  blocked: boolean;
  recoveringDestination: boolean;
  run: Run;
  reload: () => Promise<void>;
  onOrder: (order: Order) => void;
  money: Money;
}) {
  const copy = orderCopy[locale];
  // A destination journal intentionally stores no PII or quotation. A fresh
  // tab must still offer explicit address recovery when it has no quote ID.
  const country = quote?.country ?? recoveryCountry ?? "";
  const [fields, setFields] = useState<Fields>(empty);
  const [head, setHead] = useState<Destination | null>(null);
  const [option, setOption] = useState<Option | null>(null);
  const [confirmed, setConfirmed] = useState<Destination | null>(null);
  const [consents, setConsents] = useState(noConsentChoices);
  const [notice, setNotice] = useState<
    "loading" | "recovered" | "failed" | "invalid" | "uncertain" | null
  >("loading");
  const [headLoaded, setHeadLoaded] = useState(false);
  const [expired, setExpired] = useState(
    !quote || Date.parse(quote.expires_at) <= Date.now(),
  );
  const live = useRef(0);
  const attempt = useRef<{ body: DestinationWrite; key: string } | null>(null);
  // CVS (§16): payment mode, the picker's ensure() handle and the one refusal shown next to the create button.
  const [paymentMode, setPaymentMode] = useState<PaymentMode>("card");
  const [createError, setCreateError] = useState<CvsErrorCode | null>(null);
  const [codError, setCodError] = useState<string | null>(null);
  const [optionRefresh, setOptionRefresh] = useState(0);
  // storefront-v2 §C: optional buyer email, and the one bank-transfer refusal (bank_transfer_unavailable) shown next to the create button.
  const [email, setEmail] = useState("");
  const [emailInvalid, setEmailInvalid] = useState(false);
  const [transferError, setTransferError] = useState<TransferErrorCode | null>(null);
  const [promoError, setPromoError] = useState<PromoErrorCode | null>(null); // §F: the one discount-code refusal shown next to the create button
  const bank = bankTransferCopy[locale];
  const cod = codCopy[locale];
  const pickup = useRef<PickupHandle | null>(null);
  const cvsOption = option && isCvsKind(option.delivery_kind) ? (option as Option & { delivery_kind: CvsKind }) : null;
  const paymentChoices = option && quote ? offeredPaymentModes(option, quote.amount.total_minor) : [];

  useEffect(() => {
    const version = ++live.current;
    void (async () => {
      try {
        const [current, currentCart] = await Promise.all([
          currentDestination(context),
          readPurchase("cart", context, validCart),
        ]);
        if (
          currentCart.id !== cart.id ||
          currentCart.version !== cart.version ||
          (quote &&
            (quote.cart_id !== cart.id || quote.cart_version !== cart.version))
        )
          throw new BuyerClientError("request_failed", 409);
        if (version !== live.current) return;
        setHead(current);
        setHeadLoaded(true);
        if (current?.kind === "home" && current.country === country) {
          setFields(fieldsOf(current));
          setNotice("recovered");
        } else setNotice(null);
        if (!quote) return;
        let found: Option | undefined,
          cursor = "";
        const seen = new Set<string>();
        do {
          const page = await purchasePage(
            `checkout-options?market_id=${quote.market_id}&country=${quote.country}&limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
            context,
            validOptionRow,
          );
          // A row the store lists but cannot sell yet ("coming soon") is never checkout-able here.
          found = page.items.find(
            (o): o is Option =>
              !isUnavailable(o) &&
              o.market_id === quote.market_id &&
              o.country === quote.country &&
              o.method === quote.method &&
              o.currency === quote.currency &&
              (o.delivery_kind !== "home" || o.mode === "MANUAL"),
          );
          cursor = page.next_cursor;
          if (cursor && (seen.has(cursor) || seen.size >= 100))
            throw new BuyerClientError("invalid_response");
          seen.add(cursor);
        } while (!found && cursor);
        if (!found) throw new BuyerClientError("request_failed");
        if (version !== live.current) return;
        setOption(found);
        // OP1: card may not be offered (no payment service); keep the buyer's mode only if this row still offers it, else pre-select its first.
        const offered = offeredPaymentModes(found, quote.amount.total_minor);
        if (offered?.length) setPaymentMode((current) => (offered.includes(current) ? current : offered[0]));
        // Options can arrive after the buyer edits or confirms a recovered
        // address. Never hydrate fields/head again from that older snapshot.
      } catch {
        if (version === live.current) setNotice("failed");
      }
    })();
    const timer = quote
      ? window.setTimeout(
          () => {
            setExpired(true);
            setConfirmed(null);
          },
          Math.max(
            0,
            Math.min(2_147_483_647, Date.parse(quote.expires_at) - Date.now()),
          ),
        )
      : undefined;
    return () => {
      live.current++;
      window.clearTimeout(timer);
      attempt.current = null;
    };
  }, [context, quote?.id, country, cart.id, cart.version, optionRefresh]);

  useEffect(() => {
    let active = true;
    const recheck = async () => {
      setConfirmed(null);
      try {
        const latest = await currentDestination(context);
        if (active) setHead(latest);
      } catch {}
    };
    const changed = (event: StorageEvent) => {
      if (event.key === `commerce-purchase-pending-v1:${context}`)
        void recheck();
    };
    window.addEventListener("storage", changed);
    window.addEventListener("focus", recheck);
    return () => {
      active = false;
      window.removeEventListener("storage", changed);
      window.removeEventListener("focus", recheck);
    };
  }, [context]);

  async function confirm(isCurrent: () => boolean) {
    const version = live.current;
    const current = () => version === live.current && isCurrent();
    setConfirmed(null);
    const quoteExpired = !quote || Date.parse(quote.expires_at) <= Date.now();
    if (quoteExpired) {
      setExpired(true);
      if (!recoveringDestination) return;
    }
    const { recipient_name, phone, ...home_address } = fields;
    let body: DestinationWrite = {
      expected_version: head?.version ?? 0,
      cart_version: cart.version,
      kind: "home",
      country,
      recipient_name,
      phone,
      home_address,
    };
    if (cvsOption) {
      // §16.1: the store first (map selection verified / buyer-entered record), then the ordinary destination
      // write with kind=cvs_* + pickup_id. A null means the picker already showed what is missing.
      setCreateError(null);
      const pickupID = await pickup.current?.ensure();
      if (!pickupID || !current()) return;
      body = {
        expected_version: head?.version ?? 0,
        cart_version: cart.version,
        kind: cvsOption.delivery_kind,
        country,
        recipient_name: recipient_name.trim(),
        phone: phone.trim(),
        home_address: { ...noHome },
        pickup_id: pickupID,
      };
    }
    if (!validDestinationWrite(body)) {
      setNotice("invalid");
      return;
    }
    const pending = pendingPurchase(context);
    let replace: string | undefined;
    if (pending?.kind === "destination") {
      if (
        attempt.current?.key === pending.key &&
        JSON.stringify(fieldsOf(attempt.current.body)) ===
          JSON.stringify(fieldsOf(body)) &&
        attempt.current.body.pickup_id === body.pickup_id
      )
        body = attempt.current.body;
      else replace = pending.key; // Explicit reconfirmation of the displayed head, never silent replay of lost PII.
    }
    try {
      const saved = await writeDestination(context, body, replace);
      if (!current()) return;
      setHead(saved);
      setConfirmed(
        !quote || Date.parse(quote.expires_at) <= Date.now() ? null : saved,
      );
      setNotice(null);
      attempt.current = null;
    } catch (reason) {
      if (current()) {
        const waiting = pendingPurchase(context);
        if (waiting?.kind === "destination")
          attempt.current = { key: waiting.key, body };
        setNotice("uncertain");
        // Re-read the observed CAS head, but never replace typed fields or mark
        // it confirmed. A new intent requires the buyer's next explicit click.
        try {
          const latest = await currentDestination(context);
          if (current()) setHead(latest);
        } catch {}
      }
      throw reason;
    }
  }

  // The create button stays disabled until the address and total are confirmed; say so next to it (and to assistive tech) while that is the reason.
  const needsConfirm = !confirmed && !blocked && !expired;

  return (
    <section
      className="address-section"
      data-testid="address-section"
      aria-labelledby="address-title"
    >
      <h2 id="address-title">{cvsOption ? cvsCopy[locale].title : copy.address}</h2>
      <p>{quote ? copy.explain : copy.recoverWithoutQuote}</p>
      <p className="address-total">
        {quote && (
          <>
            {purchaseCopy[locale].total}:{" "}
            <strong>{money(quote.amount.total_minor, quote.currency)}</strong>{" "}
            ·{" "}
          </>
        )}
        {copy.country}: {country}
      </p>
      {notice && (
        <p
          role={
            notice === "failed" || notice === "invalid" ? "alert" : "status"
          }
        >
          {copy[notice]}
        </p>
      )}
      {quote && expired && <p role="alert">{copy.expired}</p>}
      {(notice === "failed" || expired) && (
        <button
          className="text-button"
          disabled={busy || blocked || recoveringDestination}
          onClick={() => void run(reload)}
        >
          {copy.reload}
        </button>
      )}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void run(confirm);
        }}
      >
        <fieldset
          disabled={
            busy ||
            blocked ||
            !headLoaded ||
            (!recoveringDestination && (!option || expired))
          }
          className="address-fields"
        >
          <legend className="sr-only">
            {cvsOption ? cvsCopy[locale].title : copy.address}
          </legend>
          {cvsOption && (
            <CvsPickup
              context={context}
              locale={locale}
              option={cvsOption}
              cartVersion={cart.version}
              recipient={{ recipient_name: fields.recipient_name, phone: fields.phone }}
              onRecipient={(next) => {
                setFields({ ...fields, ...next });
                setConfirmed(null);
                if (notice === "invalid") setNotice(null);
              }}
              paymentMode={paymentMode}
              onPaymentMode={(mode) => {
                setPaymentMode(mode);
                setConfirmed(null);
                setCreateError(null);
              }}
              onRestore={(draft: CvsDraft) => {
                setFields((old) => ({
                  ...old,
                  recipient_name: draft.recipient_name,
                  phone: draft.phone,
                }));
                if (cvsOption.payment_modes?.includes(draft.payment_mode))
                  setPaymentMode(draft.payment_mode);
              }}
              onStoreChange={() => setConfirmed(null)}
              run={run}
              handle={pickup}
              total={quote ? money(quote.amount.total_minor, quote.currency) : ""}
            />
          )}
          {cvsOption && <label className="sku-row cod-home-only"><input type="radio" disabled aria-label={cod.homeOnly} />{cod.homeOnly}</label>}
          {!cvsOption && option?.payment_modes && (
            <fieldset className="sku-options wide" data-testid="home-payment-mode">
              <legend>{bank.paymentLegend}</legend>
              {paymentChoices.map((mode) => (
                // .address-fields label (grid) would win over .sku-row, so the row layout is stated inline (as in CvsPickup).
                <label
                  key={mode}
                  className={mode === paymentMode ? "sku-row selected" : "sku-row"}
                  style={{ display: "flex", gap: 16, minHeight: 44 }}
                >
                  <input
                    type="radio"
                    name="home_payment_mode"
                    style={{ width: 20, height: 20, padding: 0, flex: "none" }}
                    value={mode}
                    checked={mode === paymentMode}
                    onChange={() => {
                      setPaymentMode(mode);
                      setConfirmed(null);
                      setTransferError(null);
                      setCodError(null);
                    }}
                  />
                  <span>
                    {mode === "bank_transfer"
                      ? bank.payBank
                      : mode === "cash_on_delivery"
                        ? cod.codLabel(
                            quote
                              ? money(quote.amount.total_minor, quote.currency)
                              : "",
                            option && option.cod_surcharge_minor
                              ? money(option.cod_surcharge_minor, option.currency)
                              : null,
                          )
                        : bank.payCard}
                  </span>
                </label>
              ))}
            </fieldset>
          )}
          {!cvsOption && inputs.map(([name, autoComplete, maxLength, required]) => (
            <label
              key={name}
              className={
                name === "line1" || name === "line2" ? "wide" : undefined
              }
            >
              <span>{copy[name]}</span>
              <input
                name={name}
                type={name === "phone" ? "tel" : "text"}
                autoComplete={autoComplete}
                maxLength={maxLength}
                required={required}
                value={fields[name]}
                onChange={(event) => {
                  setFields({ ...fields, [name]: event.target.value });
                  setConfirmed(null);
                  if (notice === "invalid") setNotice(null);
                }}
              />
            </label>
          ))}
          <button
            type="submit"
            data-testid="confirm-address"
            className="address-confirm"
          >
            {expired && recoveringDestination
              ? copy.recoverAddress
              : cvsOption
                ? cvsCopy[locale].confirmPickup
                : copy.confirm}
          </button>
        </fieldset>
      </form>
      {confirmed && (
        <p role="status" className="address-confirmed">
          {cvsOption ? cvsCopy[locale].confirmedPickup : copy.confirmed}
        </p>
      )}
      {createError && (
        <p role="alert" data-testid="cvs-create-error">
          {cvsCopy[locale].errors[createError]}
        </p>
      )}
      {transferError && (
        <p role="alert" data-testid="transfer-create-error">
          {bank.errors[transferError]}
        </p>
      )}
      {promoError && (
        <p role="alert" data-testid="promo-create-error">
          {promoCopy[locale].errors[promoError]} {promoCopy[locale].atCheckout}
        </p>
      )}
      <label className="buyer-email" data-testid="buyer-email">
        <span>{bank.emailLabel}</span>
        <input
          name="buyer_email"
          type="email"
          inputMode="email"
          autoComplete="email"
          maxLength={254}
          value={email}
          disabled={busy || blocked}
          onChange={(event) => {
            setEmail(event.target.value);
            setEmailInvalid(false);
          }}
        />
        <small>{bank.emailHint}</small>
      </label>
      {emailInvalid && (
        <p role="alert" data-testid="buyer-email-invalid">
          {bank.emailInvalid}
        </p>
      )}
      <ConsentChoices locale={locale} value={consents} onChange={setConsents} disabled={busy || blocked} />
      {option?.payment_modes?.includes("cash_on_delivery") && !paymentChoices.includes("cash_on_delivery") &&
        <p role="status" data-testid="cod-cap-unavailable">{quote && quote.amount.total_minor % 100 !== 0 ? cod.wholeOnly : cod.capReached}</p>}
      {paymentMode === "cash_on_delivery" && option && quote && paymentChoices.includes("cash_on_delivery") && (
        <div className="cod-summary" data-testid="checkout-cod-amount">
          <p className="cod-amount">{cod.due(money(quote.amount.total_minor + (option.cod_surcharge_minor ?? 0), "TWD"), money(option.cod_surcharge_minor ?? 0, "TWD"))}</p>
          {option.cod_carrier && <p className="order-note">{cod.manualCarrier(cod.carriers[option.cod_carrier])}</p>}
        </div>
      )}
      {codError && <p role="alert" data-testid="cod-checkout-error">{codError}</p>}
      {/* Why the button below is disabled, in words (never colour alone): the address must be confirmed first. */}
      {needsConfirm && (
        <p id="create-order-hint" className="order-note" data-testid="create-order-hint">
          {copy.createNeedsConfirm}
        </p>
      )}
      <button
        data-testid="create-order"
        className="primary create-order"
        aria-describedby={needsConfirm ? "create-order-hint" : undefined}
        disabled={busy || blocked || expired || !confirmed || !option || !paymentChoices.includes(paymentMode)}
        onClick={() =>
          void run(async (isCurrent) => {
            if (!quote || !confirmed || !option) return;
            const version = live.current;
            // The email is optional; a non-empty invalid one stops here, before any request (Go and SQL re-validate).
            const typed = email.trim();
            if (typed !== "" && !validBuyerEmail(typed)) {
              setEmailInvalid(true);
              return;
            }
            setTransferError(null);
            setPromoError(null);
            setCodError(null);
            try {
              const result = await writeCheckout(
                context,
                checkoutInput(
                  quote,
                  option,
                  cart,
                  confirmed,
                  Date.now(),
                  cvsOption || option.payment_modes !== undefined ? paymentMode : undefined,
                  typed,
                ),
              );
              void submitCheckoutConsents(context, consents); // never blocks the order (customers-billing-v1 U7)
              if (isCurrent() && version === live.current) onOrder(result);
            } catch (reason) {
              if (version === live.current) setConfirmed(null);
              if (reason instanceof BuyerClientError && ["cash_on_delivery_unavailable", "cash_on_delivery_amount_exceeds", "cash_on_delivery_limit", "cod_surcharge_changed"].includes(reason.detail ?? "")) {
                if (version === live.current) {
                  setCodError(reason.detail === "cod_surcharge_changed" ? cod.changed : reason.detail === "cash_on_delivery_amount_exceeds" ? cod.capReached : reason.detail === "cash_on_delivery_limit" ? cod.limit : cod.unavailable);
                  setOption(null);
                  setOptionRefresh((value) => value + 1);
                }
                return;
              }
              // storefront-v2 §F: a definite discount-code refusal at placement (edited, paused, expired or used up since the quote). No order
              // exists; the quotation's code field above re-applies or removes it.
              if (reason instanceof BuyerClientError && isPromoErrorCode(reason.detail)) {
                if (version === live.current) setPromoError(reason.detail);
                return;
              }
              // A definite bank-transfer refusal (the store switched it off meanwhile) is shown here, not as a generic failure.
              if (reason instanceof BuyerClientError && isTransferErrorCode(reason.detail)) {
                if (version === live.current) setTransferError(reason.detail);
                return;
              }
              // A definite CVS/pay-at-pickup refusal (§16.2) is shown here, not as a generic failure.
              if (
                cvsOption &&
                reason instanceof BuyerClientError &&
                isCvsErrorCode(reason.detail)
              ) {
                if (version === live.current) setCreateError(reason.detail);
                return;
              }
              throw reason;
            }
          })
        }
      >
        {paymentMode === "bank_transfer" && option?.payment_modes?.includes("bank_transfer")
          ? bank.createBank
          : paymentMode === "cash_on_delivery" && option?.payment_modes?.includes("cash_on_delivery")
            ? cod.createCod
            : cvsOption && paymentMode === "pay_at_pickup"
              ? cvsCopy[locale].createPickup
              : copy.create}
      </button>
      <p className="order-note">
        {paymentMode === "bank_transfer" && option?.payment_modes?.includes("bank_transfer")
          ? bank.bankNote(option.transfer_window_hours ?? 0)
          : paymentMode === "cash_on_delivery" && option?.payment_modes?.includes("cash_on_delivery")
            ? cod.codNote
            : cvsOption && paymentMode === "pay_at_pickup"
              ? cvsCopy[locale].payAtPickupNote
              : copy.unavailable}
      </p>
    </section>
  );
}

// The seller's attestation of dispatch (manual-fulfilment-v1 §5.2): never "in transit"/"delivered".
// Link: plain external anchor, host shown so the buyer sees where it goes (ruling 14, Q6);
// rel="noopener noreferrer nofollow": ruling 25 (manual-fulfilment-v1 §3.2 governs; ruling 14 was incomplete).
function ShipmentBlock({
  shipment,
  locale,
}: {
  shipment: Shipment;
  locale: Locale;
}) {
  const copy = orderCopy[locale];
  const [copied, setCopied] = useState<"" | "ok" | "failed">("");
  const link = shipment.tracking_url;
  // Validated https by validOrder (validTrackingURL) before it reaches an href.
  const host = link ? new URL(link).hostname : "";
  async function copyNumber() {
    try {
      await navigator.clipboard.writeText(shipment.tracking_number);
      setCopied("ok");
    } catch {
      setCopied("failed");
    }
  }
  return (
    <section data-testid="order-shipment" aria-labelledby="shipment-title">
      <h2 id="shipment-title" data-testid="shipment-title">
        {copy.shipped}
      </h2>
      <p data-testid="shipment-carrier">
        {copy.carrier}:{" "}
        {shipment.carrier_name ?? carrierNames[locale][shipment.carrier_code]}
      </p>
      <p>
        {copy.tracking}:{" "}
        <span data-testid="shipment-tracking" className="order-id">
          {shipment.tracking_number}
        </span>{" "}
        <button
          type="button"
          data-testid="copy-tracking"
          onClick={() => void copyNumber()}
        >
          {copy.copyTracking}
        </button>
      </p>
      <p role="status" data-testid="copy-status">
        {copied === "ok" ? copy.copied : copied === "failed" ? copy.copyFailed : ""}
      </p>
      {link && (
        <p>
          <a
            data-testid="shipment-link"
            href={link}
            target="_blank"
            rel="noopener noreferrer nofollow"
          >
            {copy.trackLink}
          </a>{" "}
          <span data-testid="shipment-host">({host})</span>
        </p>
      )}
      <p className="order-note">{copy.shipNote}</p>
    </section>
  );
}

export function OrderDetails({
  context,
  order,
  locale,
  money,
  refresh,
  busy,
  onPaymentBusy,
  isSelected,
}: {
  context: string;
  order: Order;
  locale: Locale;
  money: Money;
  refresh: () => void;
  busy: boolean;
  onPaymentBusy: (busy: boolean) => void;
  isSelected: () => boolean;
}) {
  const [paymentRefresh, setPaymentRefresh] = useState(0);
  // OrderPayment registers here only while a Stripe attempt is live (payment/refresh signal).
  const paymentSignalRef = useRef<(() => Promise<void>) | null>(null);
  const copy = orderCopy[locale],
    common = purchaseCopy[locale],
    destination = order.snapshot.destination;
  return (
    <section
      className="order-section"
      data-testid="order-section"
      aria-labelledby="order-title"
    >
      <h1 id="order-title">{order.payment_mode === "cash_on_delivery" && order.collection_state ? codCopy[locale].orderStates[order.collection_state] : copy.order}</h1>
      <p
        className="order-state"
        data-testid="order-state"
        data-state={order.commercial_state}
      >
        {order.payment_mode === "cash_on_delivery" ? codCopy[locale].orderTitle : copy[order.commercial_state]}
      </p>
      <p>
        {copy.orderID}:{" "}
        <span data-testid="order-id" className="order-id">
          {order.order_id}
        </span>
      </p>
      {order.payment_mode === "cash_on_delivery" && <CodOrderStatus order={order} locale={locale} />}
      <ul className="order-lines">
        {order.snapshot.quote.lines.map((line) => (
          <li key={line.sku_id}>
            <span>
              {line.name} · {line.code} × {line.quantity}
            </span>
            <span>
              {money(
                line.unit_price_minor * line.quantity,
                order.snapshot.quote.currency,
              )}
            </span>
          </li>
        ))}
      </ul>
      <dl className="order-breakdown" data-testid="order-breakdown">
        {(
          [
            [common.shipping, "shipping_minor"],
            [common.taxes, "tax_minor"],
            [common.discount, "discount_minor"],
          ] as const
        ).map(([label, key]) => (
          <div key={key}>
            <dt>{label}</dt>
            <dd>
              {money(
                order.snapshot.quote.amount[key],
                order.snapshot.quote.currency,
              )}
            </dd>
          </div>
        ))}
      </dl>
      <p className="order-total">
        {common.total}{" "}
        <strong>
          {money(
            order.snapshot.quote.amount.total_minor,
            order.snapshot.quote.currency,
          )}
        </strong>
      </p>
      <h2>{copy.address}</h2>
      <address>
        {destination.recipient_name}
        <br />
        {destination.phone}
        <br />
        {[
          destination.home_address.region,
          destination.home_address.city,
          destination.home_address.postal_code,
          destination.home_address.line1,
          destination.home_address.line2,
        ]
          .filter(Boolean)
          .join(" · ")}
        {destination.pickup &&
          `${destination.pickup.name} · ${destination.pickup.code} · ${destination.pickup.address}`}
        {destination.pickup?.verification_kind === "BUYER_ENTERED" && (
          <>
            <br />
            <span data-testid="order-pickup-entered">{cvsCopy[locale].enteredLabel}</span>
          </>
        )}
        <br />
        {destination.country}
      </address>
      {order.payment_mode !== "cash_on_delivery" && (
        <CvsOrderStatus order={order} locale={locale} money={money} />
      )}
      {order.shipment && (
        <ShipmentBlock shipment={order.shipment} locale={locale} />
      )}
      {order.hold_expires_at && (
        <>
          <p>
            {copy.hold}{" "}
            {new Intl.DateTimeFormat(locale, {
              timeZone: "Asia/Taipei",
              dateStyle: "short",
              timeStyle: "short",
            }).format(new Date(order.hold_expires_at))} · {common.taipeiTime}
          </p>
          <p className="order-note">{copy.holdNote}</p>
        </>
      )}
      {/* Bank transfer (storefront-v2 §C): the shop's account, a countdown and the proof form; the shop confirms, nothing here does. */}
      {order.payment_mode === "bank_transfer" && (
        <BankTransfer
          key={`${context}:${order.order_id}:transfer`}
          context={context}
          orderID={order.order_id}
          locale={locale}
          money={money}
          refreshToken={paymentRefresh}
        />
      )}
      {/* Pay-at-pickup, bank transfer and cash-on-delivery are not Stripe payments: no payment read, no start, no refresh signal. */}
      {order.payment_mode !== "pay_at_pickup" &&
        order.payment_mode !== "bank_transfer" &&
        order.payment_mode !== "cash_on_delivery" && (
        <OrderPayment
          key={`${context}:${order.order_id}`}
          context={context}
          order={order}
          locale={locale}
          money={money}
          busy={busy}
          refreshToken={paymentRefresh}
          onBusy={onPaymentBusy}
          isSelected={isSelected}
          paymentSignalRef={paymentSignalRef}
        />
      )}
      <button
        data-testid="refresh-order"
        disabled={busy}
        onClick={async () => {
          // Errors are swallowed inside the handler (shown in the payment section).
          const signal = paymentSignalRef.current;
          if (signal) await signal();
          setPaymentRefresh((v) => v + 1);
          refresh();
        }}
      >
        {copy.refresh}
      </button>
    </section>
  );
}
