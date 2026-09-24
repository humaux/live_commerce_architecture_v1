---
name: "Buyer history and continued shopping"
description: "Built record of the bounded history extension to approved B inline purchase"
---

# Buyer history and continued shopping — built design record

Recorded 2026-09-25. Application/review baseline: `442cbe7`; current capture evidence: `0c223f6`.
Mode: Operate. This records the implemented local extension, not payment or release acceptance.

## Overview

The user-approved **B 商品详情直接选购** retains its continuous product/price/description,
native SKU and quantity controls, and persistent delivery summary. The extension adds
**Your orders**, owned history/detail/back navigation and explicit **Continue shopping**.
Its purpose is to inspect existing orders and start a deliberate subsequent purchase
while preserving previous order facts and unresolved purchase intent.

Authority: [PRODUCT.md](../../PRODUCT.md), [DESIGN.md](../../DESIGN.md),
[global sidecar](../../.impeccable/design.json), [B proposal](2026-09-25-buyer-surface-proposal.md),
[inline record](2026-09-25-buyer-inline-design-record.md), [order record](2026-09-25-buyer-order-design-record.md)
and [frozen history contract](../../contracts/buyer-order-history-v1.md).
The [approved ancestor](../../.impeccable/mocks/decision/buyer-inline-detail.png) governs product
composition only; history has no separately approved composition comp. Prior records retain
their historical gates and capture dimensions. This record identifies the current evidence.
Global design files remain byte-identical; no qualitative interview, replacement world or new token is needed.

## Colors

Inherited `navigation-navy`, `ledger-ink`, `muted-text`, `ledger-line`, `action-teal`,
`focus-blue` and `selection-mint` retain their existing storefront aliases and values.
Navy headings, white surfaces and fine rules organize facts; status is written in words.
Teal “Not paid” does not mean payment success. No new global color primitive was added.

## Typography

Arial / PingFang SC / Microsoft YaHei and the 16px base remain. The history title uses
the existing 28px heading, becoming 36px at 800px. Session disclosure is 14px; row totals
are 1.25rem with tabular figures. Complete order IDs can wrap anywhere. Local sizing
describes this implementation and does not revise merchant typography tokens.

## Layout

The 54px header and centered purchase column remain (760px maximum border-box, 21px
horizontal padding). Secondary history navigation is right-aligned with 20px below it.
History replaces the purchase/current-order content and hides the purchase footer.
Its ordered list has 24px vertical margins; rows use 20px vertical padding, a top rule
and 10px grid gaps. Date/state share a wrapping row; total, ID and **View order** follow.
The two captured rows remain readable at 390px without clipped identifiers or overlap.

The current order's continuation block follows the facts with a top rule, 24px margin and
20px top padding, using existing outlined controls. Historical detail
reuses the order reading sequence; it does not expose the continuation write action.
Inherited address layout remains one column, two from 600px. The actual scrolled address
viewport is the layout evidence; a full-page fixed-footer composite alone cannot prove overlap.

## Elevation & Depth

Flat white surfaces, rules and spacing remain the hierarchy; no shadow, raised card or decorative motion was added. Existing reduced-motion handling remains.

## Shapes

Controls retain 5px corners and at least 44px height. History is a semantic collection
of ruled rows, not a new card pattern. Visible focus remains a 3px blue outline with
3px offset; native selection and caret colors retain the inherited treatments.

## Components

| Source | Built responsibility |
| --- | --- |
| [OrderHistory.tsx](../../apps/storefront/components/OrderHistory.tsx) | Loads 20 summaries per page; deduplicates appended IDs; reads owned detail and reuses `OrderDetails`; provides list/back, refresh, older orders, loading, empty and retry states. Context epochs discard stale responses. |
| [ProductPurchase.tsx](../../apps/storefront/components/ProductPurchase.tsx) | Owns history toggle, preserved purchase context, current-order continuation and recovery; hides product/footer while history is open. |
| [history-copy.ts](../../apps/storefront/lib/history-copy.ts) | Supplies `zh-CN`, `zh-TW`, `en` navigation, session limits, loading/error/empty text and the explicit continuation boundary. |
| [globals.css](../../apps/storefront/app/globals.css) | Reuses incumbent tokens and controls; adds local ruled history rows and continuation spacing. |

**Read-only history.** Rows show server date, commercial state, currency/total and full
identifier, newest first. **View order** opens an owned projection; **Back to orders**
returns to the list, while **Back to shopping** returns to the outer purchase view.
Viewing history does not change the checkout locator, cancel an order or release a hold.
Loading uses `aria-busy`/status text; transient failure offers retry, and authorization
or context failure clears displayed history. List summaries exclude address/recipient PII.

**Identity limit.** All three locales state that these are the current shopping session's
orders and that a new session cannot recover an expired one. This is not an account-wide
or cross-device history promise. Order IDs, names and phone numbers are not authentication.
Locale changes replace the route/document language while preserving in-memory context;
dates use locale formatting, but order currency and ownership remain server facts.

**Explicit continuation.** The current order action says that starting a new purchase
does not cancel or pay the existing order. It uses the existing purchase-write lock and
non-PII `next-cart` journal, verifies the owned order/current cart, and preserves a newer
cart. Only authoritative continuation permits clearing the convenience locator/quote.
Unknown checkout remains blocking; network uncertainty retries the original intent.
History persists in SQL after locator removal; no local array of PII snapshots replaces it.

## Do's and Don'ts

- **Do** preserve B's product hierarchy, secondary history navigation, unpaid language and immutable order amounts/address alongside current server state.
- **Do** distinguish readable repeated order rows from decorative repeated sections; keep the deliberate continuation separate from read-only history.
- **Don't** infer cancellation from a hold timer or payment from order creation/continuation.
- **Don't** promote synthetic fixture content, USD prices, generic store labels or local dimensions into customer claims or global tokens. No shipping raster asset was added.

## Verification and handoff

The documenter read source and opened these eight existing main-workspace captures;
no browser, detector, context script, runtime gate or fresh review was rerun. Paths below
are under `.impeccable/review/`; hashes bind the `0c223f6` capture set, not older images.

| Capture | Pixels | SHA-256 |
| --- | --- | --- |
| [buyer-history/desktop-history.png](../../.impeccable/review/buyer-history/desktop-history.png) | 1440×900 | `b9de56c69da5c08ce4b4641a6c4bc97433afcef42cac00fcc1b1a141024bf220` |
| [buyer-history/mobile-history.png](../../.impeccable/review/buyer-history/mobile-history.png) | 390×844 | `79bce185ebad15672ee80fc44d76fff52467f8c09ec31213ed83e6091d831c4f` |
| [buyer-order/desktop-order.png](../../.impeccable/review/buyer-order/desktop-order.png) | 1440×1170 | `fdfcdacb707dff76f07a12c18d8facddc27b251cddf2c36a237ab0ec410a70e4` |
| [buyer-order/mobile-order.png](../../.impeccable/review/buyer-order/mobile-order.png) | 390×1311 | `d6edadf65930b428297344b5a08035b0c0a61ddff5c7ff3e67e954df3c501f09` |
| [buyer-order/desktop-address-viewport.png](../../.impeccable/review/buyer-order/desktop-address-viewport.png) | 1440×900 | `c0719548935a6b1063a0ebc630522c9c2a9d2cd192d328f36c224bb13f4b1839` |
| [buyer-order/desktop-address.png](../../.impeccable/review/buyer-order/desktop-address.png) | 1440×2169 | `027cc2c0cd689fde7916149419f1927f2548aa06c9aebd42fc261cd9a54ae905` |
| [buyer-order/mobile-address.png](../../.impeccable/review/buyer-order/mobile-address.png) | 390×2516 | `a8cf5983d12f4b7e3e1a367f136ee71a49aff971d86217bf4e85bfeba4f1a124` |
| [buyer-inline/hero-repro.png](../../.impeccable/review/buyer-inline/hero-repro.png) | 887×1774 | `6e951075cd4b9c05536c259397c2ae262d87b969a57314fdfd96954eb15a865f` |

All captures disclose synthetic data/test environment. The hero preserves B at the
ancestor's dimensions; its USD currency is actual fixture API truth, independent of locale.
Independent Impeccable verdict `dbea47ab-f474-471e-a3c2-6906d49dde14` is **ship** for
bounded history/continuation and B preservation, with no material fixes. No standalone
QUALITY BAR card was supplied. Dedicated historical-detail, empty/error and all-locale
states were not separately visually certified; browser behavior checks are distinct evidence.

Retained root log `/Volumes/data/output/buyer-history-browser-root-final.log` reports
actual Next build → Go → isolated PostgreSQL: **23 cases, six buyers, seven orders PASS**,
`TestBrowserBuyerOrderUI` 14.97s, package 16.695s; exact order/hold/job/receipt/reserve counts.
`/Volumes/data/output/buyer-history-detector.json` has one inherited Arial warning.
Neither artifact proves real PSP payment, deployment, assistive-technology acceptance or whole-SaaS release.

Documentation gate: one allowed document, `git diff --check`, linked-file verification
against current main evidence, and unchanged global SHA-256 values:
`DESIGN.md` = `54a5b00dc6c32dc8e596c93de96c79e873f85024c317446b0f002387156b57b8`;
`.impeccable/design.json` = `6ec2d6e5a409354142e479777f4407ef202d36e719c486b9685c3688cfc2c5fb`.
Author: `buyer-history-design-record-20260925`, Impeccable documenter; base `442cbe7`,
branch `commerce/buyer-history-design-record-20260925`, worktree
`/Volumes/data/live-commerce-worktrees/buyer-history-design-record-20260925`.
Exact runtime model/effort are not exposed by the tool. Root owns review/merge; this record is the sole write path. No server, fixture, temporary asset or production application resource was created.
