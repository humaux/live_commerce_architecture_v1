// Purpose: Display frozen prices, recipient details and payment amounts for a revised home COD order.
// Depends on: the order DTO types, money formatter and local styles.
// Used by: order amendment review, saved order summary, and warehouse preview and confirmation.

import type { Locale } from "@live-commerce/i18n";
import { money } from "@/lib/client";
import type { OrderDetail, OrderItem } from "@/lib/orders-model";
import s from "./order-revision.module.css";

export const orderRevisionCopy = {
  "zh-TW": {
    review: "檢視原單變更", saved: "原單已更新", items: "商品明細", recipient: "收件資料", amounts: "金額明細",
    sku: "規格", shipping: "運費", fee: "到付手續費", subtotal: "商品小計", tax: "稅額", due: "應收金額",
    next: "查看更新後的出貨單", original: "返回原單", preview: "新版 v{n} 待交寄", fill: "填寫運單", confirm: "確認登記出貨 v{n}",
    recorded: "出貨已登記", stale: "舊版出貨單已失效，請使用新版。", delivery: "宅配到府",
  },
  "zh-CN": {
    review: "检查原单变更", saved: "原单已更新", items: "商品明细", recipient: "收件资料", amounts: "金额明细",
    sku: "规格", shipping: "运费", fee: "到付手续费", subtotal: "商品小计", tax: "税额", due: "应收金额",
    next: "查看更新后的发货单", original: "返回原单", preview: "新版 v{n} 待交寄", fill: "填写运单", confirm: "确认登记发货 v{n}",
    recorded: "已登记发货", stale: "旧版发货单已失效，请使用新版。", delivery: "送货上门",
  },
  en: {
    review: "Review order changes", saved: "Original order updated", items: "Items", recipient: "Delivery address", amounts: "Amounts",
    sku: "Variant", shipping: "Shipping", fee: "COD fee", subtotal: "Merchandise subtotal", tax: "Tax", due: "Cash due",
    next: "View updated shipping slip", original: "Back to original order", preview: "Shipping slip v{n} · Awaiting dispatch", fill: "Enter tracking", confirm: "Confirm shipment record v{n}",
    recorded: "Shipment recorded", stale: "The previous shipping slip has expired. Use the updated slip.", delivery: "Home delivery",
  },
};

export type RevisionLine = Pick<OrderItem, "code" | "name" | "quantity" | "unit_price_minor"> & {
  amount: Pick<OrderItem["amount"], "total_minor">;
};

export function OrderRevisionLines({ items, locale, currency }: {
  items: RevisionLine[]; locale: Locale; currency: string;
}) {
  const c = orderRevisionCopy[locale];
  return <section>
    <h3>{c.items}</h3>
    <ul className={s.items}>
      {items.map((item, index) => <li key={index}>
        <span><strong>{item.name}</strong><small>{c.sku} {item.code}</small></span>
        <span>{money(locale, currency, item.unit_price_minor)} × {item.quantity}</span>
        <strong>{money(locale, currency, item.amount.total_minor)}</strong>
      </li>)}
    </ul>
  </section>;
}

export function OrderRevisionAmounts({ locale, currency, totals, fee, due }: {
  locale: Locale; currency: string; totals: Pick<OrderDetail["totals"], "subtotal_minor" | "shipping_minor" | "tax_minor">;
  fee: number | null; due: number | null;
}) {
  const c = orderRevisionCopy[locale];
  const rows = [[c.subtotal, totals.subtotal_minor], [c.shipping, totals.shipping_minor],
    [c.tax, totals.tax_minor], [c.fee, fee], [c.due, due]] as const;
  return <section className={s.amounts}>
    <h3>{c.amounts}</h3>
    <dl>{rows.map(([label, value]) => value !== null && <div key={label}>
      <dt>{label}</dt><dd>{money(locale, currency, value)}</dd>
    </div>)}</dl>
  </section>;
}

export function OrderRevisionRecipient({ locale, address }: {
  locale: Locale; address: OrderDetail["destination"]["home_address"] & { recipient_name: string; phone: string };
}) {
  const { recipient_name, phone, postal_code, region, city, line1, line2 } = address;
  return <section className={s.recipient}>
    <h3>{orderRevisionCopy[locale].recipient}</h3>
    <p>{recipient_name} · {phone}</p>
    <p>{[postal_code, region, city, line1, line2].filter(Boolean).join(" ")}</p>
  </section>;
}
