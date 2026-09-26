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

## Implemented surface (local, observed at `f70dc42`)

The approved composition is implemented in `MerchantOrders.tsx` and
`orders.css`: one semantic five-column table (order, created, total, order
state, payment state), with the selected order's detail row directly beneath
it. The row toggle exposes `aria-expanded`; the same list and expansion are
used at every viewport. Detail content separates frozen item lines and totals,
receiving data, and the independent order/payment/work/fulfillment states.
Store codes and identifiers retain their textual form; order values and totals
come from the read DTO rather than generated comp copy.

The responsive source behavior is explicit: at 1100px the detail region moves
from three columns to two, with states spanning the row; at 680px the order
rows become labeled two-column blocks and the detail sections stack. Only the
item table has its own bounded horizontal scroll (`min-width: 470px`); the
page itself does not gain horizontal overflow. The live desktop table columns
are 21/20/18/21/20 percent. The payment badge may wrap inside its cell.

Visual tokens remain owned by the inherited ledger world in `DESIGN.md` and
`apps/admin/app/globals.css`; this surface adds only `--orders-muted: #5c6e83`
and scoped order layout rules in `apps/admin/components/orders.css`. Do not
promote page-specific widths, badge colors, or spacing into global primitives.
The approved comp sidecar records seed `549adef8`; the comp is decision evidence,
not a shipped asset. No raster assets are shipped.

Truth beats generated sample defects: use only MOR/MBT DTO fields/enums; show
per-page count, no fictional global totals; preserve test mode, leading-zero store
codes, unchanged currency across languages. No provider-ready claims.

Gate: `contracts/merchant-orders-ui-v1.md` MOU01–06. The final local run is
PARTIAL: six functional browser cases passed; native visibility remained
NOT_RUN, so the aggregate command exited 1. Actual history returned
(`pageshow.persisted=false`) but native bfcache restoration is not proven.
The independent visual finish review resolved its single desktop payment-label
clipping P1 at `f70dc42`; that bounded verdict is not whole-surface or MOU
approval. See `.impeccable/review/merchant-orders-c-20260927/README.md` and
`docs/implementation/2026-09-27-merchant-orders-ui-acceptance.md` for capture
provenance and the gate record. No shipping raster assets (the approved comp is
evidence, not shipped artwork). Unresolved visual choices: none.
