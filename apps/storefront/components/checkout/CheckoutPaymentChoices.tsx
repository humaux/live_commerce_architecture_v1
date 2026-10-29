"use client";

// Purpose: native payment choices in the final checkout review.
// Depends on: existing offered modes, server quote/option and localized payment copy.
// Used by: OrderFlow; selection invalidation and persistence remain in its controller.
// Invariants: shows server amounts only; never creates a method or submits an order.
import type { Locale } from "@live-commerce/i18n";
import type { PaymentMode } from "../../lib/cvs-contract";
import type { Option, Quote } from "../../lib/purchase";
import { bankTransferCopy } from "../../lib/bank-transfer-copy";
import { codCopy } from "../../lib/cod-copy";

export default function CheckoutPaymentChoices({ locale, quote, option, modes, selected, disabled, money, onChange }: {
  locale: Locale; quote: Quote; option: Option; modes: PaymentMode[]; selected: PaymentMode; disabled: boolean;
  money: (amount: number, currency: string) => string; onChange: (mode: PaymentMode) => void;
}) {
  const bank = bankTransferCopy[locale], cod = codCopy[locale];
  return <fieldset disabled={disabled} className="sku-options wide" data-testid="home-payment-mode">
    <legend>{bank.paymentLegend}</legend>
    {modes.map((mode) => <label key={mode} className={mode === selected ? "sku-row selected" : "sku-row"}
      style={{ display: "flex", gap: 16, minHeight: 44 }}>
      <input type="radio" name="home_payment_mode" style={{ width: 20, height: 20, padding: 0, flex: "none" }}
        value={mode} checked={mode === selected} onChange={() => onChange(mode)} />
      <span>{mode === "bank_transfer" ? bank.payBank : mode === "cash_on_delivery"
        ? cod.codLabel(money(quote.amount.total_minor, quote.currency), option.cod_surcharge_minor ? money(option.cod_surcharge_minor, option.currency) : null)
        : bank.payCard}</span>
    </label>)}
  </fieldset>;
}
