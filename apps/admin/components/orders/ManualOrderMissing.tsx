// Figma local links: https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=750-3454 and https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=750-3596
// Purpose: signed 訂單缺少收件資料 pre-submit state of the manual order: lists the recipient fields still missing and returns to the draft.
// Depends on: react, Locale from @live-commerce/i18n, RecipientGap from @/lib/merchant-tools-model, ./ManualOrderMissing.copy, ./ManualOrderMissing.module.css.
// Used by: components/ManualOrder.tsx (orders/new) when recipientGaps(draft) is non-empty and the merchant asks to review.
// Invariants: information only; no order, quote or payment request is made here, and the draft (items, quantities) is kept by the caller.
"use client";
import type { Locale } from "@live-commerce/i18n";
import type { RecipientGap } from "@/lib/merchant-tools-model";
import { manualOrderMissingCopy } from "./ManualOrderMissing.copy";
import s from "./ManualOrderMissing.module.css";

/** Replaces the manual-order form until the merchant goes back and completes the listed recipient fields. */
export function ManualOrderMissing({ locale, gaps, onBack }: { locale: Locale; gaps: RecipientGap[]; onBack: () => void }) {
  const c = manualOrderMissingCopy[locale];
  return (
    <section className={s.page} data-figma-node="750:3454 750:3596" data-testid="manual-order-missing" aria-labelledby="manual-order-missing-title">
      <h1 id="manual-order-missing-title">{c.title}</h1>
      <p className={s.lead}>{c.lead}</p>
      <div className={s.card}>
        <h2>{c.pending}</h2>
        <dl>
          {gaps.map((gap) => (
            <div key={gap}>
              <dt>{gap === "phone" ? c.phone : c.store}</dt>
              <dd>{gap === "phone" ? c.phoneHint : c.storeHint}</dd>
            </div>
          ))}
        </dl>
        <p className={s.note}>{c.note}</p>
      </div>
      <div className={s.dock}>
        <button type="button" onClick={onBack}>{c.back}</button>
      </div>
    </section>
  );
}
