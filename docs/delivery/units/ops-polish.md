# Unit ops-polish — honest checkout options, live order feed, pay-at-pickup in finance, dead-copy cleanup

Role: commerce_worker (mid tier). Base `896bf24`. Worktree `.worktrees/ops-polish`, branch
`unit/ops-polish`. No delegation, no new dependency, **no migration** (if you believe one is needed,
stop and report; integrator assigns numbers).

**Why (output/r3-readiness/REPORT.md §2 UX trap, gaps 6, 8, 10):** in the pilot config (Stripe off, no
PSP) the only completable sale is CVS pay-at-pickup, yet checkout offers and pre-selects "card"
(`internal/checkout/options.go` ~268 always lists `card`; `apps/storefront/components/OrderFlow.tsx`
~128 defaults to it), so a buyer who does not flip the radio creates a stock-holding order nobody can
pay. Merchants get no live feel of new orders during a live session. Finance ignores pay-at-pickup
money. Studio still says "Local MOCK rehearsal only"; WorkspaceFrame nav has dead entries.

## Decisions (binding)
- OP1 `card` is listed in `PaymentModes` only when the store has at least one payment method that is
  actually available at runtime for this market (reuse the existing runtime availability evaluator —
  grep `available` / `runtime availability` in internal/checkout and internal/httpapi
  settings_discovery.go; do not write a second evaluator). When `card` is absent the storefront
  pre-selects the first mode the option offers; when an option has no payment mode at all it is not
  offered (existing "unavailable reason" pattern). Server stays the authority: BeginCheckout/order
  creation must refuse `card` when unavailable with the existing typed error (verify it does; if not,
  add the check at the shared function every caller routes through, not in the handler).
- OP2 Merchant orders page (`MerchantOrders.tsx`): poll the existing list route every 20 s while the
  tab is visible (`document.visibilityState`), pause when hidden, never overlap requests, keep the
  user's filters/scroll; new orders since the last view get a visible "new" marker and the tab title
  shows the count (`(3) 訂單`). No websockets, no notifications API. `ponytail:` comment naming the
  ceiling (polling; SSE when >50 concurrent merchant tabs).
- OP3 Finance (`contracts/customers-billing-v1.md` BD7, `internal/reporting` + `Finance.tsx`): add a
  separate "pay-at-pickup collected" column per day from `checkout.orders` where
  `payment_mode='pay_at_pickup' AND collection_state='COLLECTED'` (UTC+8 day of the collection
  transition — find where COLLECTED is set in 0073 and which timestamp/audit row records it; if no
  timestamp exists, use the shipment/collection event time the CVS status hook writes; do not add a
  column). Keep it out of "captured" (different money path: carrier remits, not PSP). Amend BD7 with one
  sentence. CSV export gets the column.
- OP4 Copy: `studio-copy.ts` subtitle → neutral production wording (three locales); WorkspaceFrame nav
  items that open "not connected in the current build" panels are removed (keep the code paths that
  real pages use). Do not touch Ledger.tsx (owned by unit catalog-media).

## Write paths
`internal/checkout/**` (options + the shared refusal only), `apps/storefront/components/OrderFlow.tsx`
(+ its lib), `apps/admin/components/{MerchantOrders,Finance,WorkspaceFrame}.tsx`, `apps/admin/lib/**`
(copy + client), `internal/reporting/**`, `internal/httpapi/**` only for the finance response field,
`contracts/customers-billing-v1.md` (BD7 sentence), `contracts/buyer-checkout-options-v1.md` (OP1 rule).

## Done when
Static set exit 0 (build, vet, gofmt, check-pkgdocs, depmap --check, admin + storefront typecheck,
test-node.sh, check_packet.py); Go unit tests for OP1 option filtering; author smoke via
`bash scripts/dev/test-focused.sh '<regex>'`. Evidence → `/Volumes/data/live_commerce_architecture_v1/
output/ops-polish/`. Commit on your branch; do not merge.
