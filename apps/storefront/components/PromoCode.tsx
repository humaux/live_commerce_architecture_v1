"use client";

// Discount-code field of the quotation (storefront-v2 §F), mounted by ProductPurchase.tsx inside the quotation section. It re-quotes the
// SAME cart/market/country/method with `promo_code` through writePurchase (BFF POST /api/buyer/quotes -> Go POST /v1/buyer/quotes ->
// storefront.CreateQuote -> promotions.quote_check). The discount shown is the server quote's own discount_minor; this component computes no
// amount. A refused code (422 promo_*) keeps the current quote and shows one clear line next to the field; Remove re-quotes without a code.
// A new quote remounts the address form below (its key carries the quote id), which is the same behaviour as choosing another delivery.
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { BuyerClientError } from "../lib/buyer-client";
import { isPromoErrorCode, normalizePromoCode, type PromoErrorCode } from "../lib/promo-contract";
import { promoCopy } from "../lib/promo-copy";
import { writePurchase } from "../lib/purchase";
import type { Quote } from "../lib/purchase";

type Run = (work: (isCurrent: () => boolean) => Promise<void>) => Promise<void>;

export default function PromoCode({
  context,
  quote,
  locale,
  busy,
  run,
  onQuote,
  money,
}: {
  context: string;
  quote: Quote;
  locale: Locale;
  busy: boolean;
  run: Run;
  onQuote: (quote: Quote) => void;
  money: (amount: number, currency: string) => string;
}) {
  const copy = promoCopy[locale];
  const [text, setText] = useState("");
  const [problem, setProblem] = useState<PromoErrorCode | "failed" | null>(null);
  const [working, setWorking] = useState(false);

  // One re-quote. code === null removes the code. Only a definite promo_* refusal is handled here; anything else (session, uncertain
  // outcome, outage) is rethrown to the page's own recovery, which owns the purchase journal.
  async function requote(code: string | null) {
    await run(async (isCurrent) => {
      setWorking(true);
      setProblem(null);
      try {
        const result = await writePurchase(context, {
          kind: "quote",
          body: {
            cart_version: quote.cart_version,
            market_id: quote.market_id,
            country: quote.country,
            method: quote.method,
            ...(code === null ? {} : { promo_code: code }),
          },
        });
        if (!isCurrent()) return;
        if (result.kind !== "quote") throw new BuyerClientError("invalid_response");
        onQuote(result.value);
        if (code === null) setText("");
      } catch (reason) {
        if (reason instanceof BuyerClientError && reason.code === "request_failed" && isPromoErrorCode(reason.detail)) {
          if (isCurrent()) setProblem(reason.detail);
          return;
        }
        throw reason;
      } finally {
        setWorking(false);
      }
    });
  }

  function apply() {
    const code = normalizePromoCode(text);
    if (code === null) return setProblem("promo_invalid"); // cannot be a code: no request
    void requote(code);
  }

  const promotion = quote.promotion;
  return (
    <div className="promo-code" data-testid="promo-code">
      {promotion ? (
        <p role="status" data-testid="promo-applied">
          {copy.applied(promotion.code, money(quote.amount.discount_minor, quote.currency))}{" "}
          <button className="text-button" data-testid="promo-remove" disabled={busy || working} onClick={() => void requote(null)}>
            {copy.remove}
          </button>
        </p>
      ) : (
        <>
          <label htmlFor="promo-code-input">{copy.label}</label>
          <div className="promo-code-row">
            <input
              id="promo-code-input"
              data-testid="promo-input"
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              maxLength={24}
              placeholder={copy.placeholder}
              value={text}
              disabled={busy || working}
              aria-describedby="promo-code-hint"
              aria-invalid={problem !== null}
              onChange={(event) => setText(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  if (!busy && !working && text.trim() !== "") apply();
                }
              }}
            />
            <button data-testid="promo-apply" disabled={busy || working || text.trim() === ""} onClick={apply}>
              {working ? copy.applying : copy.apply}
            </button>
          </div>
          <small id="promo-code-hint">{copy.hint}</small>
        </>
      )}
      {problem && (
        <p role="alert" data-testid="promo-problem">
          {problem === "failed" ? copy.failed : copy.errors[problem]}
        </p>
      )}
    </div>
  );
}
