# Merchant orders surface: audit and direction round

2026-09-25, main `254c8a6`. UI not implemented or accepted. The accepted
merchant read API is the sole data source; no fulfillment/refund mutations.

## Incumbent evidence and duplicate-structure audit

`WorkspaceFrame.tsx` already owns rail, store title, locale navigation and mobile
drawer. `globals.css` owns controls, tabular numbers and ledger/tray grammar.
Settings A has a full-width four-step setup panel: that setup composition is not
appropriate for high-frequency order review and must not be copied as a second
wizard. No KPI-card wall, duplicate summary sections or repeated primary CTA.
Orders need one filter area, one list and one selected detail, not a separate
dashboard of counters unsupported by the API. Separate commercial/payment/work
states remain legible; payment return is not proof of capture.

Reference: `.impeccable/review/merchant-settings/hero-repro.png` (actual prior
1586×992 browser capture, viewed). Only palette/type/chrome/control grammar carry
forward, never the account fields, steps or setup status content. Global
PRODUCT.md and DESIGN.md remain unchanged. Mode Operate, restrained palette.

## Grounded structural candidates

Ordered candidates: (1) wide list with bottom inspector, (2) full list then
dedicated detail page, (3) narrow order index with dominant receipt, (4) date-grouped
chronological list, (5) full table with inline detail, (6) side-by-side list and
detail, (7) top order index with one full-width receipt. All expose the same
real read contract; no mock export/ship/refund controls or invented totals.

Surface seed `549adef8` dealt **6, 7, 5**. Display A/B/C respectively via
`.impeccable/merchant-orders-options.json`. Existing comp-led preference applies.
All sample names, orders, store details and prices are visibly synthetic.
These decision comps are unapproved until an explicit selection is recorded;
buyer choice B does not approve a new merchant page by implication.

## Separate work allowed before visual approval

Authenticated BFF read transport and its independently tested security contract
do not determine page layout. Extend only the existing allowlist; forbid the
shared local fixture for order PII. Reuse the established cookie session and
membership check; server API retains orders:read and final authority checks.
Merchant UI, responsive/localized implementation and browser acceptance wait for
this surface's selection. No production deployment or external provider calls.
