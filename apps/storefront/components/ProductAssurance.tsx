"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { browseCopy } from "../lib/browse-copy";
import { bankTransferCopy } from "../lib/bank-transfer-copy";
import { cvsCopy } from "../lib/cvs-copy";
import { codCopy } from "../lib/cod-copy";
import { isUnavailable, purchasePage, validOptionRow, type Option } from "../lib/purchase";
import { useCart } from "./CartProvider";

// Reuse an active buyer session; a read-only product page must not bootstrap one.
// No delivery-time promises, card fallback, policy body or inferred payment modes.
export default function ProductAssurance({ locale, policies }: {
  locale: Locale; policies: { slug: string; title: string }[];
}) {
  const { context } = useCart();
  const [result, setResult] = useState<{ context: string; rows: Option[] } | null>(null);
  useEffect(() => {
    let active = true;
    const read = async () => {
      const rows: Option[] = [], seen = new Set<string>();
      let cursor = "";
      // Bound a corrupt pagination chain. Never present a partial result as the available methods.
      for (let count = 0; count < 10 && active; count++) {
        const page = await purchasePage(`checkout-options?limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`, context, validOptionRow);
        rows.push(...page.items.filter((row): row is Option => !isUnavailable(row)));
        if (!page.next_cursor) return rows;
        if (seen.has(page.next_cursor)) throw new Error("repeated options cursor");
        seen.add(page.next_cursor); cursor = page.next_cursor;
      }
      throw new Error("options pagination incomplete");
    };
    if (context) void read()
      .then(rows => { if (active) setResult({ context, rows }); })
      .catch(() => { if (active) setResult(null); });
    return () => { active = false; };
  }, [context]);
  const rows = result?.context === context ? result.rows : [];
  const field = locale === "zh-TW" ? "name_hant" : locale === "zh-CN" ? "name_hans" : "name_en";
  const delivery = [...new Set(rows.map(row => row[field]).filter(Boolean))];
  const modes = [...new Set(rows.flatMap(row => row.payment_modes ?? []))];
  const paymentLabels = { card: bankTransferCopy[locale].payCard, bank_transfer: bankTransferCopy[locale].payBank, pay_at_pickup: cvsCopy[locale].payAtPickup, cash_on_delivery: codCopy[locale].orderTitle };
  const copy = browseCopy[locale];
  const shipping = policies.find(policy => policy.slug === "shipping");
  if (!delivery.length && !policies.length) return null;
  return <aside className="sf-assurance" data-testid="product-assurance">
    <dl>
      {(!!delivery.length || shipping) && <div><dt>{copy.delivery}</dt><dd>{delivery.length > 0 && <span>{delivery.join(" · ")}</span>}{shipping && <Link href={`/${locale}/pages/shipping`}>{shipping.title}</Link>}</dd></div>}
      {!!modes.length && <div><dt>{copy.payment}</dt><dd>{modes.map(mode => paymentLabels[mode]).join(" · ")}</dd></div>}
      {policies.filter(policy => policy.slug !== "shipping").map(policy => <div key={policy.slug}><dt>{policy.slug === "refunds" ? copy.refunds : copy.returns}</dt><dd><Link href={`/${locale}/pages/${policy.slug}`}>{policy.title}</Link></dd></div>)}
    </dl>
    {!!delivery.length && <p>{copy.optionsNote}</p>}
  </aside>;
}
