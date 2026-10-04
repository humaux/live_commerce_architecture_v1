# Orders v2 — extension of approved C ledger

Authority: owner R5 A3 dispatch; comp `04-orders-batch-result.png` is the visual
reference, without its bulk-result drawer (M05 is separate). Existing approved C
inline expansion remains binding. The base does not contain the W0 shell; the
incumbent navy shell is retained, not reimplemented.

THESIS: scan server-authoritative task queues, filter a store's orders, and open
one frozen order in place without losing the ledger. OWN-WORLD: existing flat
merchant ledger from DESIGN.md. STORY: search/filter → SQL queue count → seven
factual fields → existing C detail. FIRST VIEWPORT: desktop filter grid and ledger;
mobile search, collapsed secondary filters, wrapped queues and first order.
FORM: extend approved C seed `549adef8`, not a new visual-world workshop.

## Observed implementation, 2026-10-03

- One semantic table; seven columns; one shared desktop/mobile expansion tree.
- Search is always available; payment, delivery, live session and Taipei dates
  collapse behind a native button on widths ≤680px. Values survive a toggle;
  applying/resetting returns to the collapsed mobile ledger. No duplicate forms.
- Controls have a 44px minimum height. Queues wrap without page overflow.
- Desktop amounts align right, with tabular numerals. Date inputs retain the
  platform calendar and show explicit year/month/day hints in all three locales.
- Existing tokens remain authoritative (`--ink`, `--line`, `--orders-muted`).
  The scoped orange apply/queue accents follow reference 04; these are not a new
  global token system. No assets, gradients, global font or shell changes.
- Only backend-returned values and masks are shown. No invented delivery,
  settlement or provider acceptance claims; demo screenshots are synthetic MOCK.

## Finish evidence

`output/orders-v2/` holds six document-top captures, six deliberately scrolled
ledger captures, and six C open-detail captures under `inline/`.
An independent fresh reviewer (substitution for the unavailable named Impeccable
role) initially found mobile form dominance, left-aligned totals and missing
English date guidance. One fix batch resolved all three; verdict **ship**, at the
scope of that fix-round review. Detector ran once: `[]`, exit 0. Existing
DESIGN.md and design.json were preserved; this surface note records the actual
extension without overwriting the project's design system.
