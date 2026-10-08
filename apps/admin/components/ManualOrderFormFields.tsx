// Purpose: Renders controlled manual-order customer, item, delivery and payment fields for both entry points.
// Depends on: @live-commerce/i18n, @live-commerce/ui, model Store, manual-order-form, merchant-tools-model/copy, cod-copy, client money, ManualOrderItemPicker.
// Used by: ManualOrder and CreateOrderDrawer.
// Invariants: I05, I08, I09, I11; edits emit patches only, catalog prices stay informational.
"use client";
import type { Locale } from "@live-commerce/i18n";
import { Field, FormRow } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import type { ManualFormValues } from "@/lib/manual-order-form";
import type { ManualOption } from "@/lib/merchant-tools-model";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { codCopy } from "@/lib/cod-copy";
import { money } from "@/lib/client";
import { ManualOrderItemPicker } from "./ManualOrderItemPicker";

/** Shares controlled order fields without taking ownership of submit, retry or result state. */
export function ManualOrderFormFields({ locale, store, value, onChange, available, optionsReady,
  idPrefix = "mo", quantityControls = "input",
}: {
  locale: Locale; store: Store; value: ManualFormValues; onChange: (patch: Partial<ManualFormValues>) => void;
  available: ManualOption[]; optionsReady: boolean; idPrefix?: string; quantityControls?: "input" | "stepper";
}) {
  const c = toolsCopy[locale].manual;
  const { lines, name, phone, email, optionKey, mode, home, cvs, buyerLocale } = value;
  const option = available.find((item) => item.option_key === optionKey) ?? null;
  const mapOnly = !!option && option.delivery_kind !== "home" && option.pickup_selection !== "buyer_entered";
  const optionName = (o: ManualOption) =>
    `${locale === "zh-CN" ? o.name_hans : locale === "zh-TW" ? o.name_hant : o.name_en || o.name_hant} · ${c.kinds[o.delivery_kind] ?? o.delivery_kind}`;
  function selectOption(key: string) {
    const next = available.find((item) => item.option_key === key);
    onChange({ optionKey: key, mode: next && next.payment_modes.length === 1 ? next.payment_modes[0] : "" });
  }
  return <>
    <ManualOrderItemPicker locale={locale} store={store} lines={lines} setLines={(next) => onChange({ lines: next })} idPrefix={idPrefix} quantityControls={quantityControls} />
    <section className="mt-card">
      <h2>{c.customerTitle}</h2>
      <FormRow>
        <Field id={`${idPrefix}-name`} label={c.name}><input id={`${idPrefix}-name`} data-testid={`${idPrefix}-name`} value={name} maxLength={120} autoComplete="off" onChange={(e) => onChange({ name: e.target.value })} /></Field>
        <Field id={`${idPrefix}-phone`} label={c.phone} width="short"><input id={`${idPrefix}-phone`} data-testid={`${idPrefix}-phone`} value={phone} inputMode="tel" maxLength={32} autoComplete="off" onChange={(e) => onChange({ phone: e.target.value })} /></Field>
        <Field id={`${idPrefix}-email`} label={c.email}><input id={`${idPrefix}-email`} data-testid={`${idPrefix}-email`} value={email} type="email" maxLength={254} autoComplete="off" onChange={(e) => onChange({ email: e.target.value })} /></Field>
      </FormRow>
    </section>
    <section className="mt-card">
      <h2>{c.deliveryTitle}</h2>
      <FormRow><Field id={`${idPrefix}-option`} label={c.delivery} width="long">
        <select id={`${idPrefix}-option`} data-testid={`${idPrefix}-option`} value={optionKey} disabled={!optionsReady} onChange={(e) => selectOption(e.target.value)}>
          <option value="">{c.choose}</option>
          {available.map((o) => <option key={o.option_key} value={o.option_key}>{optionName(o)}</option>)}
        </select>
      </Field></FormRow>
      {mapOnly && <p className="mt-warn" role="status" style={{ marginTop: 12 }}>{c.mapOnly}</p>}
      {option?.delivery_kind === "home" && (
        <FormRow style={{ marginTop: 12 }}>
          <Field id={`${idPrefix}-region`} label={c.region}><input id={`${idPrefix}-region`} value={home.region} maxLength={100} onChange={(e) => onChange({ home: { ...home, region: e.target.value } })} /></Field>
          <Field id={`${idPrefix}-city`} label={c.city}><input id={`${idPrefix}-city`} data-testid={`${idPrefix}-city`} value={home.city} maxLength={100} onChange={(e) => onChange({ home: { ...home, city: e.target.value } })} /></Field>
          <Field id={`${idPrefix}-postal`} label={c.postal} width="short"><input id={`${idPrefix}-postal`} value={home.postal_code} maxLength={20} onChange={(e) => onChange({ home: { ...home, postal_code: e.target.value } })} /></Field>
          <Field id={`${idPrefix}-line1`} label={c.line1} width="long"><input id={`${idPrefix}-line1`} data-testid={`${idPrefix}-line1`} value={home.line1} maxLength={200} onChange={(e) => onChange({ home: { ...home, line1: e.target.value } })} /></Field>
          <Field id={`${idPrefix}-line2`} label={c.line2} width="long"><input id={`${idPrefix}-line2`} value={home.line2} maxLength={200} onChange={(e) => onChange({ home: { ...home, line2: e.target.value } })} /></Field>
        </FormRow>
      )}
      {option && option.delivery_kind !== "home" && !mapOnly && (
        <>
          <p className="mt-note">{c.cvsHint}</p>
          <FormRow style={{ marginTop: 8 }}>
            <Field id={`${idPrefix}-store-code`} label={c.storeCode} width="short"><input id={`${idPrefix}-store-code`} data-testid={`${idPrefix}-store-code`} value={cvs.store_code} maxLength={32} onChange={(e) => onChange({ cvs: { ...cvs, store_code: e.target.value } })} /></Field>
            <Field id={`${idPrefix}-store-name`} label={c.storeName}><input id={`${idPrefix}-store-name`} data-testid={`${idPrefix}-store-name`} value={cvs.store_name} maxLength={40} onChange={(e) => onChange({ cvs: { ...cvs, store_name: e.target.value } })} /></Field>
            <Field id={`${idPrefix}-store-address`} label={c.storeAddress} width="long"><input id={`${idPrefix}-store-address`} data-testid={`${idPrefix}-store-address`} value={cvs.store_address} maxLength={120} onChange={(e) => onChange({ cvs: { ...cvs, store_address: e.target.value } })} /></Field>
          </FormRow>
        </>
      )}
    </section>
    <section className="mt-card">
      <h2>{c.paymentTitle}</h2>
      {option ? option.payment_modes.map((m) => (
        <label key={m} className="mt-radio">
          <input type="radio" name={idPrefix === "mo" ? "mode" : `${idPrefix}-mode`} value={m} checked={mode === m} onChange={() => onChange({ mode: m })} data-testid={`${idPrefix}-mode-${m}`} />
          {m === "bank_transfer" ? c.bank : m === "cash_on_delivery" ? c.cod : c.pickup}
        </label>
      )) : <p className="mt-note">{c.choose}</p>}
      {mode === "cash_on_delivery" && option?.cod_max_minor !== undefined && option.cod_carrier && (
        <p className="mt-note" data-testid={`${idPrefix}-cod-note`}>
          {c.codNote(money(locale, option.currency, option.cod_surcharge_minor ?? 0), money(locale, option.currency, option.cod_max_minor),
            option.cod_carrier === "black_cat" ? codCopy[locale].carrierBlackCat : codCopy[locale].carrierHsinchu)}
        </p>
      )}
      <p className="mt-note">{c.noCard}</p>
      <FormRow style={{ marginTop: 14 }}><Field id={`${idPrefix}-buyer-locale`} label={c.linkTitle} width="short">
        <select id={`${idPrefix}-buyer-locale`} data-testid={`${idPrefix}-buyer-locale`} value={buyerLocale} onChange={(e) => onChange({ buyerLocale: e.target.value as Locale })}>
          {(["zh-TW", "zh-CN", "en"] as const).map((l) => <option key={l} value={l}>{l}</option>)}
        </select>
      </Field></FormRow>
    </section>
  </>;
}
