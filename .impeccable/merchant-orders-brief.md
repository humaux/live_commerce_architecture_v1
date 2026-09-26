# Merchant orders — approved C

Scope: `apps/admin/app/[locale]/orders/page.tsx` and its read-only order workspace.
Mode: Operate. Frequent merchant reconciliation; scan orders, expand one row,
inspect frozen items, payment/work/fulfillment and receiving details, then continue.

User approved C 表格原位展开 on 2026-09-27 via the exact comp link.
Comp: `.impeccable/mocks/decision/merchant-orders-inline.png`, 1586×992.
THESIS: retain the order table context while revealing one order in place; no
side inspector, modal, KPI cards, export/shipping/refund/manual-paid affordances.
FIRST VIEWPORT: inherited navy rail and topbar, Orders title, one commercial
filter and refresh, full-width five-column table, selected light-blue row followed
by an inline region: items/totals left, delivery center, distinct states right.
FORM: approved surface option orders-inline, seed 549adef8; inherited ledger world.
Responsive: one order expansion, not duplicated desktop/mobile trees; mobile
prioritizes order and total/state, details stack; no page horizontal overflow.

Truth beats generated sample defects: use only MOR/MBT DTO fields/enums; show
per-page count, no fictional global totals; preserve test mode, leading-zero store
codes, unchanged currency across languages. No provider-ready claims.

Gate: contracts/merchant-orders-ui-v1.md MOU01–06. In particular no old PII after
session/store/selection/hide/pagehide; fresh authorized read required on restore.
No shipping raster assets (the approved comp is evidence, not shipped artwork).
Unresolved visual choices: none. Implementation/browser acceptance: NOT_RUN.
