# FIX-COD-422 (#422)

**Root cause**
1. Checkout bar label ignored the COD due: it always showed `v2.total` (最終應付) even when `codFooter` supplied `due.label`.
2. Payment-mode sessionStorage save failure set notice "failed" but left `confirmed` set, so the wrapper hid the alert and the buyer saw no reason.
3. Comment at the refreshCodQuote catch did not match the code.
4. (Relay 1) `tests/storefront/checkout-v2.mjs` still asserted the old `最終應付`/`Final amount due` label in the COD bar (stale expectation of the behaviour #422 changes).

**Change**
- apps/storefront/components/CheckoutFlow.tsx:697 label `due ? due.label : quote ? v2.total : copy.subtotal` (amount untouched, `due.note` kept).
- apps/storefront/components/OrderFlow.tsx:186 and :551 (same catch pattern in the mode onChange) add `setConfirmed(null)`.
- apps/storefront/components/OrderFlow.tsx:231-233 comment rewritten to 3 lines; behaviour unchanged (also fixes the shell-architecture line cap, was 855 > 854).
- tests/storefront/checkout-v2.mjs:399-401,479 COD mode now expects the due label (`codDue`, same strings as lib/cod-copy.ts); other modes still expect the final-total label.

**RED run id:** output/fix-cod-422/red.log (CI run id not available locally)
**GREEN run id:** NOT_RUN locally; CI pending on the next head (prior head failed in run 41229925438).
