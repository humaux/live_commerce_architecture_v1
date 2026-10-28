# OC-09b pin candidate: buyerhttp.Routes (pending independent review)

Integrator grant (2026-10-28): the buyerhttp.Routes entry of contracts/route-table-dispatchers.json only. Branch unit/oc09b-cod-due,
PR #250, base r3/integration @ 79f95784c. Author: commerce_worker (Claude Opus 5.5). This file records a candidate, not approval.

## Hash
- old: c593c0be301721ca9917c2f2b4ea05acafc2bc9935c419c79d55f9b99da71f60 (reviewed 2026-10-15, output/buyer-route-table/review-p70-pin.md)
- new: bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a. This is the hash CI printed (Gates run 40615763623, plan job
  122589659476: "got bfbb4441… want c593c0be…"), and the local scanner run reproduced it.
- `reviewed`: "pending independent review (OC-09b, 2026-10-28)"

## Closure delta (functions 81 -> 82, declarations 150 -> 151)
- Added function: `handler.quoteWithCodDue` (internal/buyerhttp/handler.go, end of file). It projects the quote with the unchanged
  `projectQuote` and then sets `CodDueMinor` from `h.checkout.CodDue(ctx, token, storeID, quote.ID)`. checkout is a cross-package
  call, so its body is not frozen; reviewers should read internal/checkout/options.go `CodDue`/`codDue` separately.
- Added declaration: `func handler.quoteWithCodDue`
- Changed declaration: `func handler.dispatch`, only in the `quotesRoute` and `quoteRoute` cases.
  `out = projectQuote(out.(storefront.Quote))` becomes `out, err = h.quoteWithCodDue(ctx, token, storeID, out.(storefront.Quote))`.
  A CodDue error now fails the request through the existing error path.
- Changed declaration: `type quoteResponse`. One field is added: `CodDueMinor *int64 json:"cod_due_minor,omitempty"`.
- Removed: none. No other declaration in the closure moves (no route, const, var, routing or input-parsing change). cmd/api is
  untouched.

## Reviewer commands
```sh
cd /Volumes/data/live_commerce_architecture_v1/.claude/worktrees/oc09b
git diff 79f95784c -- internal/buyerhttp contracts/route-table-dispatchers.json   # the whole closure delta + pin delta
git diff 79f95784c -- internal/checkout/options.go                                # the cross-package CodDue it calls
go run ./scripts/dev/contractdrift -base origin/r3/integration | grep -E '^SUMMARY|\(NEW\)|dispatcher'
```
Local scanner result with the candidate pin: `SUMMARY ... errors=0`, 0 `(NEW)`. Before the pin edit: errors=130, 83 `(NEW)`, and
every buyerhttp row was UNRESOLVED with "route-table dispatcher changed or unpinned". The full frozen closure (82 functions, 151
declarations) is listed in the pin entry. To read all of it, open each `functions` entry in internal/buyerhttp/*.go
(catalogv2.go claims.go cvs.go design.go handler.go lookup.go media.go orderlink.go payment.go primary.go privacy.go projections.go routes.go transfer.go).
