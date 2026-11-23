"use client";

// Purpose: confirmed-address checkout and current order details, with stable layout while delivery facts load.
// Depends on: purchase.ts CAS/recovery, order-fields, buyer BFF routes, CheckoutReview/CheckoutPaymentChoices and payment/fulfilment components.
// Used by: the storefront checkout route's purchase controller; no persistent address cache or client-owned transaction rules.
// Invariants: matching server quote/options, confirmed destination and changed-total acknowledgment gate submit; brand changes requote; money is never added here.
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
import CvsPickup, { CvsOrderStatus, type CvsCheckoutDraft, type PickupHandle } from "./CvsPickup";
import { ShipmentBlock } from "./ShipmentBlock";
import BankTransfer from "./BankTransfer";
import { CodAmount, CodOrderStatus } from "./CodOrderStatus";
import type { Locale } from "@live-commerce/i18n";
import { BuyerClientError } from "../lib/buyer-client";
import { orderCopy } from "../lib/order-copy";
import { purchaseCopy } from "../lib/purchase-copy";
import { cvsCopy } from "../lib/cvs-copy";
import { bankTransferCopy } from "../lib/bank-transfer-copy";
import { codCopy } from "../lib/cod-copy";
import { checkoutCopy } from "../lib/checkout-copy";
import CheckoutReview from "./checkout/CheckoutReview";
import CheckoutAddressFields, { CheckoutDestinationAddress } from "./checkout/CheckoutAddressFields";
import CheckoutPaymentChoices from "./checkout/CheckoutPaymentChoices";
import { checkoutOption } from "../lib/checkout-review";
import { countryName } from "../lib/country-name";
import {
  emptyOrderFields as empty,
  orderFieldsOf as fieldsOf,
  type OrderFields as Fields,
} from "../lib/order-fields";
import {
  isTransferErrorCode,
  settledCommercialState,
  validBuyerEmail,
  type TransferErrorCode,
  type TransferState,
} from "../lib/bank-transfer-contract";
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
  codQuoteMatches,
  refreshCodQuote,
  offeredPaymentModes,
  currentDestination,
  pendingPurchase,
  readPurchase,
  readCheckoutCart,
  purchaseReadPath,
  purchaseScope,
  validQuote,
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
} from "../lib/purchase";

type Run = (work: (isCurrent: () => boolean) => Promise<void>) => Promise<void>;
type Money = (amount: number, currency: string) => string;
const noHome: HomeAddress = { region: "", city: "", postal_code: "", line1: "", line2: "" };

export default function OrderFlow({
  context,
  cart,
  buyNowID,
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
  onCodFee,
  reconfirmRequired = false,
  onFeeChanged,
  onQuoteRefresh,
  onReviewState,
  detailsOpen = false,
  inReview, onContinue, entryDraft, onEntryDraftChange,
  cvsResume,
  cvsDraft,
  onCvsDraftChange,
  onCvsBrand,
  cvsKinds,
}: {
  context: string;
  cart: Cart;
  buyNowID?: string;
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
  // OC-09b: reports the selected cash-on-delivery option's server surcharge (null = another mode or COD not offered) so the
  // checkout bottom bar can show the quote's cod_due_minor; never an amount of its own.
  onCodFee?: (fee: number | null) => void;
  reconfirmRequired?: boolean;
  onFeeChanged?: () => void;
  onQuoteRefresh?: (quote: Quote) => void;
  detailsOpen?: boolean;
  inReview: boolean; onContinue: (quoteID: string) => void; entryDraft: Fields | null;
  onEntryDraftChange?: (fields: Fields) => void;
  onReviewState?: (state: { quoteID: string; ready: boolean; timing: string; quoteReady?: boolean; destinationReady?: boolean; submitting?: boolean; needsConfirm?: boolean }) => void;
  cvsResume?: CvsDraft;
  cvsDraft?: CvsCheckoutDraft;
  onCvsDraftChange?: (draft: CvsCheckoutDraft) => void;
  onCvsBrand?: (kind: CvsKind, draft: CvsDraft) => void;
  cvsKinds?: CvsKind[];
}) {
  const copy = orderCopy[locale];
  // A destination journal intentionally stores no PII or quotation. A fresh
  // tab must still offer explicit address recovery when it has no quote ID.
  const country = quote?.country ?? recoveryCountry ?? "";
  const [fields, setFields] = useState<Fields>(() => ({
    ...(entryDraft ?? empty),
    ...(cvsDraft?.recipient_name !== undefined ? { recipient_name: cvsDraft.recipient_name } : {}),
    ...(cvsDraft?.phone !== undefined ? { phone: cvsDraft.phone } : {}),
    ...(cvsResume ? { recipient_name: cvsResume.recipient_name, phone: cvsResume.phone } : {}),
  }));
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
  const [paymentMode, setPaymentMode] = useState<PaymentMode>(cvsResume?.payment_mode ?? cvsDraft?.payment_mode ?? "card");
  const [createError, setCreateError] = useState<CvsErrorCode | null>(null);
  const [codError, setCodError] = useState<string | null>(null);
  const [optionRefresh, setOptionRefresh] = useState(0);
  // storefront-v2 §C: optional buyer email, and the one bank-transfer refusal (bank_transfer_unavailable) shown next to the create button.
  const [submitting, setSubmitting] = useState(false);
  const [email, setEmail] = useState("");
  const [emailInvalid, setEmailInvalid] = useState(false);
  const [transferError, setTransferError] = useState<TransferErrorCode | null>(null);
  const [promoError, setPromoError] = useState<PromoErrorCode | null>(null); // §F: the one discount-code refusal shown next to the create button
  const bank = bankTransferCopy[locale];
  const cod = codCopy[locale];
  const pickup = useRef<PickupHandle | null>(null);
  const cvsOption = option && isCvsKind(option.delivery_kind) ? (option as Option & { delivery_kind: CvsKind }) : null;
  const mapPickup = cvsOption?.pickup_selection === "ecpay_map" && head?.kind === cvsOption.delivery_kind &&
    head.country === country && head.cart_id === cart.id && head.cart_version === cart.version &&
    Date.parse(head.expires_at) > Date.now() && head.pickup?.verification_kind === "PROVIDER_DIRECTORY_VERIFIED" ? head.pickup : null;
  const paymentChoices = option && quote ? offeredPaymentModes(option, quote.amount.total_minor)
    .filter((mode) => mode !== "cash_on_delivery" || codQuoteMatches(quote, option)) : [];
  const codFee = paymentMode === "cash_on_delivery" && option && paymentChoices.includes("cash_on_delivery") ? option.cod_surcharge_minor ?? 0 : null;
  const checkout = checkoutCopy[locale];
  const timing = paymentMode === "cash_on_delivery" ? checkout.cod : paymentMode === "pay_at_pickup" ? checkout.pickup : checkout.online;
  const { recipient_name, phone, ...home_address } = fields;
  const homeBody: DestinationWrite = {
    expected_version: head?.version ?? 0,
    cart_version: cart.version,
    kind: "home",
    country,
    recipient_name,
    phone,
    home_address,
  };
  const destinationReady = cvsOption ? !!recipient_name.trim() && !!phone.trim() : validDestinationWrite(homeBody);
  const canSubmit = inReview && !busy && !blocked && !expired && !!confirmed && !!option && paymentChoices.includes(paymentMode) && !reconfirmRequired && (codFee === null || quote?.cod_due_minor !== undefined);
  const quoteReady = headLoaded && !blocked && !expired && !!option && paymentChoices.includes(paymentMode) && (codFee === null || quote?.cod_due_minor !== undefined);
  useEffect(() => {
    if (!inReview && confirmed && quote && quoteReady && Date.parse(quote.expires_at) > Date.now()) onContinue(quote.id);
  }, [inReview, confirmed, quote, quoteReady, onContinue]);
  // The fixed and form actions share the visible reason while an address still needs confirmation.
  const needsConfirm = !confirmed && !blocked && !expired;
  useEffect(() => {
    if (quote) onReviewState?.({ quoteID: quote.id, ready: canSubmit, timing, quoteReady, destinationReady, submitting, needsConfirm });
    return () => { if (quote) onReviewState?.({ quoteID: quote.id, ready: false, timing }); };
  }, [quote?.id, canSubmit, timing, quoteReady, destinationReady, submitting, needsConfirm, onReviewState]);
  useEffect(() => {
    onCodFee?.(codFee);
    return () => onCodFee?.(null);
  }, [codFee, onCodFee]);

  useEffect(() => {
    if (!quote || !option || !paymentChoices.includes(paymentMode)) return;
    // Remember automatic choices too: reload must compare the payable the buyer actually reviewed.
    try { sessionStorage.setItem(`commerce-checkout-mode-v2:${context}:${quote.id}`, paymentMode); }
    catch { setOption(null); setConfirmed(null); setNotice("failed"); }
  }, [context, quote, option, paymentMode]);

  useEffect(() => {
    const version = ++live.current;
    setOption(null);
    setConfirmed(null);
    void (async () => {
      try {
        const [current, currentCart] = await Promise.all([
          currentDestination(context, buyNowID),
          readCheckoutCart(context, buyNowID),
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
        // The current delivery draft wins over older home fields; the CAS head still loads above.
        if (!entryDraft && !cvsDraft && !cvsResume && current?.kind === "home" && current.country === country) {
          setFields(fieldsOf(current));
          setNotice((notice) => notice === "failed" ? notice : "recovered");
        } else setNotice((notice) => notice === "failed" ? notice : null);
      } catch {
        if (version === live.current) setNotice("failed");
      }
    })();
    // Options must not wait for the destination head: their controls affect page height.
    void (async () => {
      try {
        if (!quote) return;
        // A fee refusal invalidates both reads. Publish neither until both are ready.
        const [currentQuote, found] = await Promise.all([
          optionRefresh ? readPurchase(purchaseReadPath(`quotes/${quote.id}`, buyNowID), context, validQuote) : Promise.resolve(quote),
          checkoutOption(context, quote),
        ]);
        const savedMode = sessionStorage.getItem(`commerce-checkout-mode-v2:${context}:${quote.id}`);
        const modes = offeredPaymentModes(found, currentQuote.amount.total_minor);
        const selected = modes.includes(savedMode as PaymentMode) ? savedMode
          : modes.includes(paymentMode) ? paymentMode : modes[0];
        // A COD-selected review, or an option refresh, rethrows every refresh failure. A card/online review keeps
        // currentQuote only when the refresh fails without an HTTP status (e.g. a pinned card re-quote); the COD
        // due is hidden for non-COD modes (codFooter returns null), so a stale due is never shown or submitted.
        let latest = currentQuote;
        try {
          latest = await refreshCodQuote(context, currentQuote, found, buyNowID);
        } catch (reason) {
          if (optionRefresh || selected === "cash_on_delivery" || (reason instanceof BuyerClientError && reason.status !== undefined))
            throw reason;
        }
        if (version !== live.current) return;
        if (optionRefresh || latest !== quote) onQuoteRefresh?.(latest);
        // OP1: card may not be offered (no payment service); keep the buyer's mode only if this row still offers it, else pre-select its first.
        const offered = offeredPaymentModes(found, latest.amount.total_minor)
          .filter((mode) => mode !== "cash_on_delivery" || codQuoteMatches(latest, found));
        if (offered?.length) setPaymentMode((current) => offered.includes(savedMode as PaymentMode) ? savedMode as PaymentMode : offered.includes(current) ? current : offered[0]);
        setOption(found);
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
  }, [context, buyNowID, quote?.id, country, cart.id, cart.version, optionRefresh, cvsResume]);

  useEffect(() => {
    let active = true;
    const recheck = async () => {
      setConfirmed(null);
      try {
        const latest = await currentDestination(context, buyNowID);
        if (active) setHead(latest);
      } catch {}
    };
    const changed = (event: StorageEvent) => {
      if (event.key === `commerce-purchase-pending-v1:${purchaseScope(context, buyNowID)}`)
        void recheck();
    };
    window.addEventListener("storage", changed);
    window.addEventListener("focus", recheck);
    return () => {
      active = false;
      window.removeEventListener("storage", changed);
      window.removeEventListener("focus", recheck);
    };
  }, [context, buyNowID]);

  async function confirm(isCurrent: () => boolean) {
    const version = live.current;
    const current = () => version === live.current && isCurrent();
    const quoteExpired = !quote || Date.parse(quote.expires_at) <= Date.now();
    if (quoteExpired) {
      setExpired(true);
      if (!recoveringDestination) return;
    }
    if (!recoveringDestination && (!headLoaded || !quoteReady || blocked)) return;
    let body = homeBody;
    if (cvsOption) {
      // §16.1: the store first (map selection verified / buyer-entered record), then the ordinary destination
      // write with kind=cvs_* + pickup_id. A null means the picker already showed what is missing.
      setCreateError(null);
      const pickupID = await pickup.current?.ensure(current);
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
    setConfirmed(null);
    const pending = pendingPurchase(context, buyNowID);
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
      const saved = await writeDestination(context, body, replace, buyNowID);
      if (!current()) return;
      setHead(saved);
      if (quote && Date.parse(quote.expires_at) > Date.now()) setConfirmed(saved);
      setNotice(null);
      attempt.current = null;
    } catch (reason) {
      if (current()) {
        const waiting = pendingPurchase(context, buyNowID);
        if (waiting?.kind === "destination")
          attempt.current = { key: waiting.key, body };
        setNotice("uncertain");
        // Re-read the observed CAS head, but never replace typed fields or mark
        // it confirmed. A new intent requires the buyer's next explicit click.
        try {
          const latest = await currentDestination(context, buyNowID);
          if (current()) setHead(latest);
        } catch {}
      }
      throw reason;
    }
  }

  return (
    <section
      className="address-section"
      hidden={!!quote && !option && !recoveringDestination && notice !== "failed"}
      data-testid="address-section"
      aria-labelledby="address-title"
    >
      <div hidden={!!confirmed && !detailsOpen}>
      <h2 id="address-title">{cvsOption ? cvsCopy[locale].title : copy.address}</h2>
      {(inReview || recoveringDestination) && <p>{quote ? copy.explain : copy.recoverWithoutQuote}</p>}
      <p className="address-total">{copy.country}: {countryName(locale, country)}</p>
      {/* Keep the loading line's space: collapsing it can move a Safari pointer target between down/up. */}
      <p
        role={notice ? (notice === "failed" || notice === "invalid" ? "alert" : "status") : undefined}
        aria-hidden={notice === null}
      >
        {notice ? copy[notice] : "\u00a0"}
      </p>
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
      <form id="checkout-address-form"
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
              buyNowID={buyNowID}
              recipient={{ recipient_name: fields.recipient_name, phone: fields.phone }}
              initialStore={cvsDraft?.store}
              persistedMapStore={mapPickup?.id ? {
                pickup_id: mapPickup.id, kind: cvsOption.delivery_kind, code: mapPickup.code,
                name: mapPickup.name, address: mapPickup.address, source: "map",
              } : undefined}
              onRecipient={(next) => {
                setFields({ ...fields, ...next });
                onCvsDraftChange?.(next);
                setConfirmed(null);
                if (notice === "invalid") setNotice(null);
              }}
              paymentMode={paymentMode}
              onPaymentMode={(mode) => {
                setPaymentMode(mode);
                onCvsDraftChange?.({ payment_mode: mode });
                setConfirmed(null);
                setCreateError(null);
              }}
              onRestore={(draft: CvsDraft) => {
                onCvsDraftChange?.(draft);
                setFields((old) => ({
                  ...old,
                  recipient_name: draft.recipient_name,
                  phone: draft.phone,
                }));
                if (cvsOption.payment_modes?.includes(draft.payment_mode))
                  setPaymentMode(draft.payment_mode);
              }}
              onStoreChange={(store) => {
                onCvsDraftChange?.({ store });
                setConfirmed(null);
              }}
              run={run}
              handle={pickup}
              total={quote ? money(quote.amount.total_minor, quote.currency) : ""}
              onBrand={onCvsBrand}
              availableKinds={cvsKinds}
              pickerInitiallyOpen={!!cvsResume}
            />
          )}
          {cvsOption && <label className="sku-row cod-home-only"><input type="radio" disabled aria-label={cod.homeOnly} />{cod.homeOnly}</label>}
          {!cvsOption && <CheckoutAddressFields entry={!inReview && !recoveringDestination} locale={locale} fields={fields} onChange={(next) => {
            setFields(next);
            onEntryDraftChange?.(next);
            setConfirmed(null);
            if (notice === "invalid") setNotice(null);
          }} />}
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
      {paymentMode === "cash_on_delivery" && option && quote && quote.cod_due_minor !== undefined && paymentChoices.includes("cash_on_delivery") && (
        <div className="cod-summary" data-testid="checkout-cod-amount">
          {/* OC-09b: the server quote's cod_due_minor, shown as given (no client sum). */}
          <p className="cod-amount"><CodAmount locale={locale} total={money(quote.cod_due_minor, "TWD")} fee={money(option.cod_surcharge_minor ?? 0, "TWD")} /></p>
          {option.cod_carrier && <p className="order-note">{cod.manualCarrier(cod.carriers[option.cod_carrier])}</p>}
        </div>
      )}
      {codError && <p role="alert" data-testid="cod-checkout-error">{codError}</p>}
      </div>
      {quote && <CheckoutReview readOnly={!quoteReady} paymentChoicesOnly={!inReview} locale={locale} quote={quote} destination={confirmed} fee={codFee} timing={timing} detailsOpen={detailsOpen}
        method={paymentMode === "bank_transfer" ? bank.payBank : paymentMode === "cash_on_delivery" ? cod.orderTitle : paymentMode === "pay_at_pickup" ? cvsCopy[locale].createPickup : bank.payCard}>
          {!cvsOption && option && <CheckoutPaymentChoices locale={locale} quote={quote} option={option} modes={paymentChoices}
            selected={paymentMode} detailsOpen={detailsOpen} disabled={busy || blocked || expired || !headLoaded} money={money} onChange={(mode) => {
              try { sessionStorage.setItem(`commerce-checkout-mode-v2:${context}:${quote.id}`, mode); }
              catch { setOption(null); setConfirmed(null); setNotice("failed"); return; }
              setPaymentMode(mode);
              setConfirmed(null);
              setTransferError(null);
              setCodError(null);
            }} />}
      </CheckoutReview>}
      {/* Why the button below is disabled, in words (never colour alone): the address must be confirmed first. */}
      {needsConfirm && (
        <p id="create-order-hint" className="order-note" data-testid="create-order-hint">
          {copy.createNeedsConfirm}
        </p>
      )}
      <div hidden={!!confirmed && !detailsOpen}>
      <form hidden={!inReview} id="checkout-order-form" onSubmit={(event) => {
        event.preventDefault();
        if (!canSubmit) return;
        void run(async (isCurrent) => {
            if (!quote || !confirmed || !option) return;
            const version = live.current;
            // The email is optional; a non-empty invalid one stops here, before any request (Go and SQL re-validate).
            const typed = email.trim();
            if (typed !== "" && !validBuyerEmail(typed)) {
              setEmailInvalid(true);
              setConfirmed(null);
              return;
            }
            setTransferError(null);
            setPromoError(null);
            setCodError(null);
            setSubmitting(true);
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
                buyNowID,
              );
              void submitCheckoutConsents(context, consents); // never blocks the order (customers-billing-v1 U7)
              if (isCurrent() && version === live.current) onOrder(result);
            } catch (reason) {
              if (version === live.current) setConfirmed(null);
              if (reason instanceof BuyerClientError && ["cash_on_delivery_unavailable", "cash_on_delivery_amount_exceeds", "cash_on_delivery_limit", "cod_surcharge_changed"].includes(reason.detail ?? "")) {
                if (version === live.current) {
                  setCodError(reason.detail === "cod_surcharge_changed" ? cod.changed : reason.detail === "cash_on_delivery_amount_exceeds" ? cod.capReached : reason.detail === "cash_on_delivery_limit" ? cod.limit : cod.unavailable);
                  if (reason.detail === "cod_surcharge_changed") onFeeChanged?.();
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
            } finally {
              if (version === live.current) setSubmitting(false);
            }
          });
      }}>
      <button
        type="submit"
        data-testid="create-order"
        className="primary create-order"
        aria-describedby={needsConfirm ? "create-order-hint" : undefined}
        disabled={!canSubmit}
      >
        {paymentMode === "bank_transfer" && option?.payment_modes?.includes("bank_transfer")
          ? bank.createBank
          : paymentMode === "cash_on_delivery" && option?.payment_modes?.includes("cash_on_delivery")
            ? cod.createCod
            : cvsOption && paymentMode === "pay_at_pickup"
              ? cvsCopy[locale].createPickup
              : copy.create}
      </button>
      </form>
      <p className="order-note">
        {paymentMode === "bank_transfer" && option?.payment_modes?.includes("bank_transfer")
          ? bank.bankNote(option.transfer_window_hours ?? 0)
          : paymentMode === "cash_on_delivery" && option?.payment_modes?.includes("cash_on_delivery")
            ? cod.codNote
            : cvsOption && paymentMode === "pay_at_pickup"
              ? cvsCopy[locale].payAtPickupNote
              : copy.unavailable}
      </p>
      </div>
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
  // D06: the transfer panel reports its state; a confirmed/expired transfer settles the heading before the order read catches up, and
  // that disagreement triggers one order reload so the order itself (hold line, shipment, history) agrees.
  const [transferState, setTransferState] = useState<TransferState | null>(null);
  const shownState = settledCommercialState(order.commercial_state, transferState);
  const stale = shownState !== order.commercial_state;
  useEffect(() => {
    if (stale) refresh();
  }, [stale]);
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
      <h1 id="order-title">{copy.order}</h1>
      {/* A cash-on-delivery order states its status once, inside its own "Cash on delivery" section (CodOrderStatus). */}
      {!(order.payment_mode === "cash_on_delivery" && order.collection_state) && (
        <p className="order-state" data-testid="order-state" data-state={shownState}>
          {copy[shownState]}
        </p>
      )}
      <p className="order-ref">
        <span>{copy.orderID}</span>
        <span data-testid="order-id" className="order-id">
          {order.order_id}
        </span>
      </p>
      {order.payment_mode === "cash_on_delivery" && <CodOrderStatus order={order} locale={locale} />}
      <ul className="order-lines">
        {order.snapshot.quote.lines.map((line, index) => (
          <li key={`${line.sku_id}:${index}`}>
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
      {/* Only amounts that changed the total are listed: "Delivery NT$0 / Tax NT$0 / Discount NT$0" says nothing the total does not. */}
      {(() => {
        const amount = order.snapshot.quote.amount;
        const rows = (
          [
            [common.shipping, "shipping_minor"],
            [common.taxes, "tax_minor"],
            [common.discount, "discount_minor"],
          ] as const
        ).filter(([, key]) => amount[key] !== 0);
        return (
          rows.length > 0 && (
            <dl className="order-breakdown" data-testid="order-breakdown">
              {rows.map(([label, key]) => (
                <div key={key}>
                  <dt>{label}</dt>
                  <dd>{money(amount[key], order.snapshot.quote.currency)}</dd>
                </div>
              ))}
            </dl>
          )
        );
      })()}
      <p className={`order-total${order.payment_mode === "cash_on_delivery" ? " cod-order-subtotal" : ""}`}>
        {common.orderTotal}{" "}
        <strong>
          {money(
            order.snapshot.quote.amount.total_minor,
            order.snapshot.quote.currency,
          )}
        </strong>
      </p>
      <h2>{copy.address}</h2>
      <CheckoutDestinationAddress destination={destination} locale={locale} />
      {order.payment_mode !== "cash_on_delivery" && (
        <CvsOrderStatus order={order} locale={locale} money={money} />
      )}
      {order.shipment && (
        <ShipmentBlock shipment={order.shipment} locale={locale} />
      )}
      {order.hold_expires_at && !stale && (
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
          onState={setTransferState}
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
