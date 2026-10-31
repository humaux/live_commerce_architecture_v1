# FIX-COD-422 (#422)

**Root cause**
1. Checkout bar label ignored the COD due: it always showed `v2.total` (最終應付) even when `codFooter` supplied `due.label`.
2. Payment-mode sessionStorage save failure set notice "failed" but left `confirmed` set, so the wrapper hid the alert and the buyer saw no reason.
3. Comment at the refreshCodQuote catch did not match the code.

**Change**
- apps/storefront/components/CheckoutFlow.tsx:697 label `due ? due.label : quote ? v2.total : copy.subtotal` (amount untouched, `due.note` kept).
- apps/storefront/components/OrderFlow.tsx:186 and :551 (same catch pattern in the mode onChange) add `setConfirmed(null)`.
- apps/storefront/components/OrderFlow.tsx:231-235 comment rewritten; behaviour unchanged.

**RED run id:** output/fix-cod-422/red.log (CI run id not available locally; NOT_RUN locally)
**GREEN run id:** NOT_RUN locally; CI TestBrowserHomeCod pending
