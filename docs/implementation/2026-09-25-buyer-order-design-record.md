---
name: "Buyer address and unpaid order"
description: "Built record of the local address and order extension to approved B inline purchase"
---

# Buyer address and unpaid order — built design record

Recorded 2026-09-25 from application source `83a28e0`, included in documentation
base `ebbef1d9f05178f3622591d6928fb69e701bc69a`. Mode: Operate.

## Overview

The user-approved **B 商品详情直接选购** remains the composition authority: a
continuous product page with native selection and a persistent total summary.
This extension adds home-address entry, explicit address/total confirmation,
creation of an unpaid DRAFT order and recovery of that same order. It introduces
no replacement visual world or global token/component contract.

Authority: [PRODUCT.md](../../PRODUCT.md), [DESIGN.md](../../DESIGN.md),
[its sidecar](../../.impeccable/design.json), the
[B proposal](2026-09-25-buyer-surface-proposal.md),
[prior built record](2026-09-25-buyer-inline-design-record.md) and
[frozen order contract](../../contracts/buyer-order-ui-v1.md). The
[approved ancestor comp](../../.impeccable/mocks/decision/buyer-inline-detail.png)
is product-page direction, not a new address/order reference. The prior record's
quote-only scope is historical. Both global design files remain byte-for-byte
unchanged; this local extension requires no new qualitative interview or comp.

## Colors

The extension uses the incumbent CSS aliases: `--navy` for headings and ordinary
controls, `--ink` for content, `--muted` for explanatory copy, `--line` for rules,
`--teal` for the submit action and textual confirmation, and `--focus` for visible
keyboard focus. Values are unchanged from the global frontmatter and the prior
buyer record. White surfaces and thin borders establish the hierarchy.

The pale blue input border (`#becddd`) and 60% disabled opacity are observed
storefront details, not new global primitives. “Not paid” is written explicitly;
the teal state text does not imply payment success.

## Typography

The existing Arial / PingFang SC / Microsoft YaHei stack and 16px base remain.
Address headings use the existing 22px navy section heading. Order state and
total are 22px; explanatory order notes are 14px. The order title inherits the
existing 28px heading, becoming 36px from 800px width.

Paragraphs and the semantic address use 1.55 line height and can wrap long
content. The address is upright, not italic. Order identifiers can wrap anywhere.
Line amounts and breakdown amounts use tabular figures and stay on one line;
the item amount column cannot shrink while the product/SKU text can wrap.

## Layout

The existing B page/header/footer geometry remains as documented in the prior
record. The form follows the quote in the same column: a 1px top rule, 32px
separation, 24px top padding and 20px anchor scroll offset. No modal, admin rail,
progress wizard or repeated card group was added.

The borderless fieldset has 24px top / 16px bottom margin and an 18px grid gap;
each visible label sits 8px above its input. Mobile uses one column. From 600px,
fields use two equal columns; both address lines and the confirmation action
span the full width. The create-order action is full-width on mobile and auto
width with a 240px minimum from 600px. Its inherited primary height is at least
58px. Existing narrow-screen adaptations remain local to B.

After a quote exists, the fixed total/unpaid footer uses the white outlined
**View quotation** navigation control, at least 44px tall. **Create unpaid order**
is the form's filled teal action. The footer retains safe-area bottom padding;
main content retains 160px bottom space. Once an owned order is rendered, the
product form and purchase footer are replaced by the order view.

The order view places title/state and number before immutable lines, then
delivery/tax/discount, a ruled total, delivery address, hold explanation and
**Refresh order**. Line rows have 20px column gaps and 14px vertical margins;
the total has rules on both edges and 18px vertical padding. This is a local
reading sequence, not a new global invoice or order-history template.

## Elevation & Depth

The extension remains flat. White background, fine rules and spacing separate
quotation, address and order facts. No shadow, raised card, blur or decorative
motion vocabulary was introduced. Existing reduced-motion handling is retained.

## Shapes

Inputs and buttons retain 5px corners. Address inputs have a 1px border, 12px
padding and full available width. The native form's grouping has no visible box;
the order is not enclosed in a new rounded card. This does not change the global
7px merchant-surface or status-chip vocabulary.

## Components

### Native address and confirmation

The form uses labeled native inputs with autocomplete: recipient and phone,
optional region, city, optional postal code, required address line 1 and optional
line 2. Phone uses `type="tel"`; the fieldset has a semantic legend. Country is
server quote/recovery context, not a locale-derived editable assumption. Normal
entry requires an exact eligible MANUAL/home option; trusted CVS selection is
not implemented here.

**Confirm address** is a secondary submit button. The filled create-order action
is disabled while busy, blocked, expired, unconfirmed or missing a valid option.
The current quotation total and country are repeated beside the form so the
buyer can inspect what confirmation covers. Confirmation text uses `role=status`;
invalid/load-failure and expired-quote messages use explicit alert text.

| State | Visible behavior and boundary |
| --- | --- |
| Owned address loaded/reloaded | Populate the owned head once; show that a saved address was found and require confirmation again. Later option loading cannot overwrite edits or a confirmed address. |
| Address edited, relevant cross-tab change or window refocus | Invalidate confirmation. An observed newer head is never silently accepted as the buyer's confirmation. |
| Language-only change | Replace the locale path/history and document language, preserving in-memory fields. `zh-CN`, `zh-TW` and `en` do not change currency, country or ownership. |
| Expired quotation | Clear confirmation and request a new quote. A pending destination can still be explicitly recovered without a session quote before quoting again. |
| Uncertain address write | Explain the uncertainty; retry the same in-memory body/key, or after reload explicitly confirm a new CAS intent against the observed head. No address body is reconstructed from browser storage. |
| Pending or known checkout | Recover the original request/order; block duplicate creation and automatic session reset. A later denial does not prove the earlier request never committed. |
| Owned DRAFT order | Show “Not paid,” immutable amounts/address, current server state and a refresh action. No payment control is present. |
| Hold time reached | The timer alone does not mean cancellation. Refresh reads current server state; the accepted worker-expiry case becomes CANCELLED. |

### Localized order facts

The exact create labels are **创建未付款订单**, **建立未付款訂單** and
**Create unpaid order**. DRAFT is **尚未付款** / **尚未付款** / **Not paid**.
All three locales explicitly say that payment is not open on the page and
creating an order does not take payment. Additional server-state labels exist
for awaiting payment, confirmed and cancelled; their presence is not evidence
that this page can initiate or complete payment.

The order's item lines, shipping, tax, discount, total and currency come from
`order.snapshot.quote`; destination comes from `order.snapshot.destination`.
The refreshed commercial state is separate from that immutable snapshot. No
current catalog price, locale currency or historical checkout receipt replaces
those authoritative facts.

### Dependency trace and PII boundary

| Source | Responsibility and dependency |
| --- | --- |
| [ProductPurchase.tsx](../../apps/storefront/components/ProductPurchase.tsx) | Owns cart/quote context, session epoch, locale history, recovery dispatch, footer hierarchy and owned-order rendering; uses the existing buyer client and purchase coordinator. |
| [OrderFlow.tsx](../../apps/storefront/components/OrderFlow.tsx) | Owns the in-memory native form, observed head, confirmation/expiry and exact eligible option; calls `currentDestination`, `writeDestination`, `checkoutInput` and `writeCheckout`. `OrderDetails` consumes the owned server projection. |
| [purchase.ts](../../apps/storefront/lib/purchase.ts) | Reuses `commerce-purchase-write-v1`, exact-shape journals and an order locator; no parallel mutation coordinator or transaction engine. |
| [order-copy.ts](../../apps/storefront/lib/order-copy.ts), [purchase-copy.ts](../../apps/storefront/lib/purchase-copy.ts), [globals.css](../../apps/storefront/app/globals.css) | Supply localized state/amount labels and surface-local styling; no new dependency, API, schema, media asset or global token. |

Recipient, phone and address fields and returned order/destination snapshots
remain in application memory. Local/session storage and URLs do not receive
PII, PII hashes, snapshots, bearer or provider keys. Destination recovery stores
only its key and validated cart/head/kind/country metadata; checkout stores the
original exact five-ID/version intent and key. Success persists and reads back
an order-ID-only same-context locator before journal removal, then reads the
owned order. A different owner cannot inherit its snapshot. The server remains
the authority for prices, stock, eligibility, revisions, ownership and expiry.

## Do's and Don'ts

- **Do** keep explicit confirmation, unpaid language, recovery feedback and the
  distinction between immutable order facts and current server state.
- **Do** keep quotation navigation secondary once the form supplies the create
  action; preserve readable unbroken amounts while item text wraps.
- **Don't** turn the synthetic product, address, USD amounts, generic storefront
  label or these local dimensions into customer facts or global design tokens.
- **Don't** treat approval of the ancestor comp or the bounded F1/F2 verdict as
  address/order comp approval, all-state visual certification or release approval.
- **Don't** erase an uncertain order by creating a new session or another order.
  Secure access recovery and a deliberate subsequent-purchase flow remain required.

## Verification and handoff

The documenter inspected current source and these existing main-workspace
captures; no new browser, detector, runtime test or polishing cycle was run:

| Capture under `.impeccable/review/buyer-order/` | Native pixels | Observed state |
| --- | --- | --- |
| [mobile-address.png](../../.impeccable/review/buyer-order/mobile-address.png) | 390×2452 | Confirmed address, filled create action and outlined quotation navigation. |
| [mobile-order.png](../../.impeccable/review/buyer-order/mobile-order.png) | 390×1126 | Unpaid order, intact monetary values and full breakdown. |
| [desktop-address.png](../../.impeccable/review/buyer-order/desktop-address.png) | 1440×2105 | Two-column unconfirmed form; full-page fixed-footer compositing is not sole layout evidence. |
| [desktop-order.png](../../.impeccable/review/buyer-order/desktop-order.png) | 1440×1001 | Unpaid order with aligned breakdown and refresh action. |
| [desktop-address-viewport.png](../../.impeccable/review/buyer-order/desktop-address-viewport.png) | 1440×900 | Actual scrolled viewport, native field focus and outlined footer action. |

Both order captures show synthetic item 12.50, delivery 0.50, tax 0.66, discount
0.00 and total 13.66 in the fixture's USD currency. These are evidence values,
not commercial promises. All captures visibly disclose the synthetic environment.

Independent final verdict `bb4c4c60-4bbb-42b9-a1d8-d60adb2ca3fc` supersedes
`5fc38654-9f1e-474e-9a73-c8c2991387af`: **ship, limited to F1/F2**. F1 is resolved
by demoting quotation navigation while preserving its handler. F2 is resolved
by showing immutable delivery/tax/discount before the total. Both scored regions
passed; no further F1/F2 check is owed. This is not a fresh whole-surface audit,
all-locale/error-state certification, runtime gate or payment/release acceptance.

The documenter read the retained logs below; execution belongs to the root
integrator and independent test worker. See the
[integration record](2026-09-25-buyer-order-ui-progress.md) and
[browser gate record](2026-09-25-buyer-order-browser-gate.md) for causal cases,
fixture authority, failed runs and cleanup.

| Retained gate | Reported result | Log in `/Volumes/data/output/` |
| --- | --- | --- |
| Storefront client tests | 26 PASS, zero failures/skips | `buyer-order-ui-node-final.log` |
| Real address/order UI after `83a28e0` | 15 scenarios, six buyers; foundation 14.954s PASS | `buyer-order-gate-root-visual-fixes.log` |
| Existing buyer regression after `83a28e0` | 13 scenarios; foundation 9.455s PASS | `buyer-order-ui-existing-regression-post-visual.log` |
| Full isolated Go/PostgreSQL regression, race and vet | Root integration record reports 376 top-level PASS; foundation 150.085s | `buyer-order-ui-full-regression.log` |

The two browser logs include successful production builds; they do not show a
production deployment. The existing single detector result
`/Volumes/data/output/buyer-order-ui-detector.json` contains one inherited Arial
warning. It is recorded without a second scan or unauthorized font replacement.

Documentation acceptance: only this new record changes, relative links resolve,
and `git diff --check` passes. SHA-256 remains
`54a5b00dc6c32dc8e596c93de96c79e873f85024c317446b0f002387156b57b8`
for root `DESIGN.md` and
`6ec2d6e5a409354142e479777f4407ef202d36e719c486b9685c3688cfc2c5fb`
for `.impeccable/design.json`.

Remaining product gates include secure guest/account and erased-storage recovery,
explicit next purchase retaining order history, trusted 7-Eleven/FamilyMart pickup,
per-order hosted payment and signed authoritative callbacks, real provider
sandbox acceptance, cross-border carrier integration, deployment/DNS/TLS/rollback
and performance/release gates. This slice's local order/expiry evidence does not
waive end-to-end payment/stock reconciliation or whole-SaaS delivery.

Author: `buyer-order-design-record-20260925` (Impeccable documenter), isolated
branch `commerce/buyer-order-design-record-20260925`, worktree
`/Volumes/data/live-commerce-worktrees/buyer-order-design-record-20260925`.
The tool does not expose the exact runtime model/effort identifiers; none are
invented. The root integrator owns independent review and merge. This task starts
no servers and creates no fixture, temporary asset or runtime change.
