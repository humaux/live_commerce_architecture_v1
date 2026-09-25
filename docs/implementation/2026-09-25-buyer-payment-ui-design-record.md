---
name: "Buyer order payment and neutral return"
description: "Built local record of the bounded payment extension to approved B inline purchase"
---

# Buyer order payment — built design record

Recorded 2026-09-25 at `c149a20`. Mode: Operate. This describes the local B
payment UI and neutral return, not provider, device or release acceptance.

## Authority and scope

The approved [B inline composition](../../.impeccable/mocks/decision/buyer-inline-detail.png)
governs the product/SKU/quantity purchase ancestor, not a separately approved
payment comp. [PRODUCT.md](../../PRODUCT.md), [DESIGN.md](../../DESIGN.md), the
[global sidecar](../../.impeccable/design.json), [inline record](2026-09-25-buyer-inline-design-record.md),
[order record](2026-09-25-buyer-order-design-record.md), [history record](2026-09-25-buyer-history-design-record.md),
[frozen BPU contract](../../contracts/buyer-payment-ui-v1.md) and
[bounded acceptance](2026-09-25-buyer-payment-ui-acceptance.md) set this boundary.
The coherent incumbent Operate system remains; neither global design file changes.
There was no new qualitative interview, visual world, comp or global token decision.

## Observed visual language

| Local element | Built rule and source |
| --- | --- |
| Palette and depth | `--navy` `#193c61`, `--ink` `#142942`, `--muted` `#64758a`, `--line` `#dde5ee`, `--teal` `#247965` in [globals.css](../../apps/storefront/app/globals.css). White surface, 1px rules and spacing carry hierarchy; no payment card shadow. These are existing storefront aliases of incumbent tokens. |
| Typography | Arial / PingFang SC / Microsoft YaHei at 16px body; 22px navy section heading, bold payment/test-state text, 14px muted notes. Amounts use tabular numerals in the enclosing order. The fixed return uses the same stack, 28px desktop heading and 26px at ≤480px. |
| Shape and action | Existing 5px control radius and 3px blue focus outline with 3px offset. The teal `payment-pay` button is 100% wide, at least 48px high, with `#1c6554` hover; disabled styling inherits the control rule. It sits inside the order, not the product sticky footer or a history row. |
| Placement | [OrderDetails](../../apps/storefront/components/OrderFlow.tsx) places one payment section after immutable order facts, delivery address and optional hold note, then **Refresh order**. Its top rule has 28px/20px block margins and 4px top padding. Current and owned historical detail reuse that sequence through [OrderHistory](../../apps/storefront/components/OrderHistory.tsx). |
| Responsive | Existing 54px header and centered 760px maximum purchase column remain. At 390px, the full-width pay action and wrapping notes stay in the same reading order; at ≥800px, the inherited main top space and title scale increase. The return is a separate 720px maximum single column, padded 48px/24px desktop and 32px/20px at ≤480px. These measurements are local, not merchant-ledger rules. |

## Components, copy and state

The [OrderPayment component](../../apps/storefront/components/OrderPayment.tsx)
reads an authoritative PaymentView for the selected order. It shows the frozen
test-mode notice when supplied, then explicit **Payment status** and **Order status**
labels from the same view; it does not combine them with the older order heading.
`NOT_STARTED`, `PENDING`, `AUTHORIZED`, `CAPTURED` and `REVIEW_REQUIRED` each have
text in [payment-copy.ts](../../apps/storefront/lib/payment-copy.ts) for `en`,
`zh-CN` and `zh-TW`. `CAPTURED` says a capture was recorded, not that the bank
settled. A payment handoff likewise never claims success.

The ready state shows the server-supplied localized method name, explains the
separate tab and offers **Pay in a new tab**. Only an existing original prepare
marker permits **Continue original payment**. Once a page has been issued or
status is uncertain, the panel becomes read-only and tells the buyer to refresh
or contact the store without paying again. Loading/sending use `role="status"`;
failure, blocked popup and uncertain handoff use an alert. The section exposes
`aria-labelledby` and `aria-busy`; visible keyboard focus comes from the shared
CSS. **Refresh order**, focus, pageshow and storage changes read again; they do
not initiate payment. The explicit click owns the separate-tab handoff.

[ProductPurchase.tsx](../../apps/storefront/components/ProductPurchase.tsx)
formats server currency without deriving it from locale. Only `zh-TW` + `TWD`
uses `currencyDisplay: "code"`, so the mobile history capture reads `TWD 25.00`
instead of ambiguous `$25.00`; other locale/currency rendering retains its
existing behavior. Amount, order status and payment status remain server facts.
The source has no new wizard, floating Pay action or success illustration.

The fixed [return handler](../../apps/storefront/lib/payment-return.ts) renders
one English heading and `en`, `zh-CN`, `zh-TW` guidance in ruled sections. It
tells buyers to return to the original store tab and refresh the order. There
is no tenant-specific link, payment receipt or return-page action. Its scriptless
HTML and restrictive response headers support a neutral page; backend/browser
acceptance, rather than this visual record, establishes the GET/POST boundary.

## Evidence and limits

This documenter read the named source and inspected the approved ancestor and
all eight existing run-7 screenshots under
[`output/playwright/buyer-payment-229897813/`](../../output/playwright/buyer-payment-229897813/):
`desktop-order-ready.png`, `desktop-payment-ready.png`, `mobile-payment-ready.png`,
`mobile-payment-readonly.png`, `mobile-native-history-ready.png`,
`mobile-native-history-readonly.png`, `desktop-payment-return.png` and
`mobile-payment-return.png`. The ready panel is legible at 718×318 desktop and
348×339 narrow crop; the complete order at 1440×1485 shows payment following
the facts. The 390px responsive read-only order keeps `NT$25.00` visible;
the separate touch/Android-UA Pixel 7 history capture shows `TWD 25.00` and
distinct Traditional Chinese payment/order labels. Native screenshot pixel
dimensions reflect device scale and are not CSS viewport sizes.

The [acceptance record](2026-09-25-buyer-payment-ui-acceptance.md) binds run 7:
11 actual browser cases PASS with production Next → Go → isolated PostgreSQL 18
and locally intercepted native form POST; 55 storefront Node tests and strict
typecheck PASS. Its retained log is `/Volumes/data/output/buyer-payment-ui-root-browser-7.log`
(SHA-256 `0f90ed3c6f963dac162ef28ff0c45f452252c9917cb60d2c6f357a635033cd5e`).
Fresh Impeccable review `85ae8e24-53a1-40c8-ab2f-65d5034e197d` required
explicit TWD and distinct status labels; verdict
`c162f2fe-f623-478f-b81d-fe5469e9c7d8` marks both resolved and `ship` only
at that fix scope. The one prior detector found only the inherited Arial advisory.
This documentation pass did not rerun browser, detector or runtime gates.

No real PSP request, charge, physical phone, deployed ReturnURL or production
service is certified here. Synthetic fixture prices and merchant names are not
customer facts. `DESIGN.md` SHA-256 remains
`54a5b00dc6c32dc8e596c93de96c79e873f85024c317446b0f002387156b57b8`;
`.impeccable/design.json` remains
`6ec2d6e5a409354142e479777f4407ef202d36e719c486b9685c3688cfc2c5fb`.
