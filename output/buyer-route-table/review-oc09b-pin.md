**Verdict: APPROVE**

Independent read-only review of the PR #250 (unit/oc09b-cod-due) `buyerhttp.Routes` pin change, local commit
`a8bfc702469d6ca94e4f2e53db87085731a4e11a` (worktree `.claude/worktrees/oc09b`), base `origin/r3/integration` = merge-base
`79f95784c9dee419346fe3119d4d4f56720d4a3c`. Reviewer: Claude Opus 5.5 subagent, read-only (no repo edits, no commits, no push, no
`go test`). Evidence level: static checks plus scanner runs (CLI + diff). No Go or PG tests were run (NOT_RUN by instruction; CI only).

## Closure read (82 functions, 151 declarations, the whole set)

I read every non-test file in `internal/buyerhttp`, in full at HEAD: catalogv2.go claims.go cvs.go design.go handler.go lookup.go media.go
orderlink.go payment.go primary.go privacy.go projections.go routes.go transfer.go. Together they hold every `functions` entry and every
const, var and type declaration in the pin.
- `git diff --quiet 79f95784c -- catalogv2.go claims.go cvs.go design.go lookup.go media.go orderlink.go payment.go primary.go privacy.go transfer.go routes.go`
  exits 0, so those 12 files are byte-identical to the reviewed base.
- Only handler.go (+14/-2) and projections.go (+4) changed. cmd/api, routes.go and the row table `routeRows` are untouched.

## Findings

1. **The closure delta is exactly the declared delta.** PASS.
   - Added function `handler.quoteWithCodDue` (handler.go:787-795). It calls `projectQuote` (unchanged), then
     `h.checkout.CodDue(ctx, token, storeID, quote.ID)`, then sets `CodDueMinor`. On error it returns `(nil, err)`.
   - Changed `func handler.dispatch`. Only two lines changed, in the `quotesRoute` case (:589) and the `quoteRoute` case (:596):
     `out = projectQuote(...)` became `out, err = h.quoteWithCodDue(...)`. Nothing else in dispatch changed.
   - Changed `type quoteResponse`. It gains one field, `CodDueMinor *int64 json:"cod_due_minor,omitempty"` (projections.go:186).
   - The pin JSON diff touches only the `buyerhttp.Routes` entry: the hash (c593c0be… -> bfbb4441…), one added `functions` entry
     (`handler.quoteWithCodDue`), one added `declarations` entry (`func handler.quoteWithCodDue`), and `reviewed`. 82 functions and
     151 declarations, matching the candidate note. The other pinned tables are unchanged.
   - The new function adds no package-var reference and no new frozen-value use. `CodDue` is a cross-package method, so it is the
     documented unfrozen boundary (contractdrift.md, "Cross-package helpers … are NOT frozen"). It is reviewed separately in item 3.

2. **The hash and both lists match what the scanner computes, and each mismatch fails.** PASS. Logs are in scratchpad/oc09b/.
   - Worktree HEAD, `go run ./scripts/dev/contractdrift -base origin/r3/integration`: exit 0, `errors=0 … GO_ROUTE_TABLE=4 UNRESOLVED=51`.
   - Shared clone at a8bfc7024 with `CONTRACT_DRIFT_BASE=tmpbase` (79f95784c):
     - Control: exit 0 with the same SUMMARY.
     - Old pin file restored (hash c593c0be…, old lists): exit 1, `errors=130`, `GO_ROUTE_TABLE=1`, and 46
       `ERROR (dispatcher freeze) UNRESOLVED … buyerhttp.Routes bfbb4441… want c593c0be…`.
     - New hash, but `handler.quoteWithCodDue` removed from `functions`: exit 1, `errors=130`, freeze ERROR (got == want hash, so
       the list itself is enforced).
     - New hash, but `func handler.quoteWithCodDue` removed from `declarations`: exit 1, freeze ERROR.
   - Base commit 79f95784c run against the same base gives an identical SUMMARY.

3. **No route, method, path, auth or scope change.** PASS.
   - The full scanner log at base and at HEAD differs in one INFO line only: the buyer GO_ROUTE_TABLE source-line list gains
     handler.go:787 and projections.go lines shift by +4. The route inventory, methods, paths and counts are identical. No new route.
   - `routeRows`, `matchRoute`, `allowed`, `matchPattern`, `forbiddenInput`, `bearer`, `keyFor`, `ServeHTTP`, `authorizeView` and
     `viewAllowed` are unchanged. View-only guest sessions still cannot reach `quotesRoute` or `quoteRoute`.

4. **Tenant, store and owner isolation, and client steering.** PASS.
   - `storeID` is the resolver's (`h.resolver.Resolve(origin)`). `token` is the validated bearer. `quote.ID` comes from the quote the
     server just read or created in the caller's scope. No request field reaches `CodDue`, and the buyer sends no amount.
   - `CodDue` opens its own `buyer.WithScope(token, storeID)` and re-reads the quote with `storefront.GetQuote`
     (`WHERE tenant_id AND store_id AND owner_id AND id`, after `CheckScope`). `read_cod_offer` re-resolves the scope and refuses on a
     tenant or store GUC mismatch (0107:208-220). `checkCapability` compares `resolve_scope` with the scope.
   - Another owner's quote gives ErrNotFound and another store gives ErrUnauthorized. The test checks both (item 6).
   - **Fail closed.** Any `CodDue` error fails the whole request through the existing `classify` path (503, 401, 404 or 422). No
     quote body is written, so a wrong or partial due is never shown. This is stricter than "due absent on error". A quote POST that
     committed and then fails here is replayed by the same Idempotency-Key.

5. **`checkout.CodDue` and `codDue` (internal/checkout/options.go:269-311, cross-package, not frozen).** PASS.
   - **Equality with `cod_collect_minor`.** begin_hold (post_river/0021:239-249, the last definition) sets
     `v_total = v_quote.snapshot->amount->total_minor`, requires `p_snapshot->quote = v_quote.snapshot` (:72), and stores
     `total_minor` (:374) and `cod_surcharge_minor = surcharge_twd*100` (:375). `cod_collect_minor` is defined as
     `o.total_minor + cod_surcharge_minor` (0107:792, 916). `CodDue` uses `quote.Amount.TotalMinor` from the same stored snapshot JSON
     (`readQuote` unmarshals `snapshot`) plus `int64(surchargeTWD)*100` from the same settings row (`read_cod_offer`, defaults identical
     to an absent row, which begin_hold treats as disabled). The result is equal whenever the surcharge is unchanged between the quote
     read and Begin. If it changes, the `expected_cod_surcharge_minor` gate raises PT409 (see P2-1 for the residual gap).
   - **Absent exactly where begin_hold refuses on amount or switch.** Returns nil when: COD disabled, currency is not TWD, country is
     not TW, `total%100≠0`, total is outside 100..2,000,000, or `total+surcharge > max_twd*100`. This mirrors 0021:233-249 operator for
     operator, including the `>` cap at equality. The contract amendment documents that home-only, the recipient rule and the
     open-orders limit stay with `payment_modes` and Begin.
   - **No float arithmetic.** Integer `int64` only, and no overflow: surcharge ≤ 1000, cap ≤ 20000 by CHECK.
   - `go build` and `go vet` pass for `./internal/checkout/ ./internal/buyerhttp/ ./cmd/api/ ./tests/foundation/`. `gofmt -l` is clean.

6. **Tests.** Adequate (NOT_RUN here; CI owns them).
   - `tests/foundation/oc09b_cod_due_test.go`:
     - POST quote due == total + 5000.
     - GET quote returns the same due.
     - A real COD `Begin` from that quote produces an order whose `cod_collect_minor` == due, and a DB row with
       `total_minor+cod_surcharge_minor = due`.
     - Another owner gets 404 with no due in the body. `CodDue` returns ErrNotFound for another owner and ErrUnauthorized for another
       store, with an own-scope control.
     - The due is absent over the cap, and `Begin` refuses as a control. The due is absent with the switch off.
     - `output/oc-09/red-09b.log` records RED.
   - `internal/checkout/options_test.go` `TestCodDueMirrorsBeginHold`: the happy path, zero surcharge, equal-to-cap allowed,
     one-over-cap, switch off, not TWD, not TW, not whole TWD, zero total and above NT$20,000. All are nil where expected.

No P0/P1.

## Non-blocking (P2, for the issue batch; outside the frozen closure)

- **P2-1. The displayed due and Begin's surcharge guard use different sources.**
  - The bottom bar shows `quote.cod_due_minor`, which uses the surcharge at quote-read time.
  - The fee note and `expected_cod_surcharge_minor` sent to Begin use `option.cod_surcharge_minor`
    (apps/storefront/lib/purchase.ts:572, CheckoutFlow.tsx:694, OrderFlow.tsx:563).
  - Begin refuses only when the current surcharge ≠ the options value. So collect ≠ displayed due needs options value == now ≠ quote
    value: a merchant A→B→A change, or an options refetch after the quote was read.
  - This is narrow but not impossible. Suggested fix: send `expected_cod_surcharge_minor = cod_due_minor − amount.total_minor`, or
    expose the surcharge used on the quote, so Begin binds to what was shown.
- **P2-2. Boundary tests are missing.** `codDue` has no test for the inclusive boundaries `total = 100` and `total = 2,000,000`
  (allowed). There is also no transport test that a `CodDue` error yields an error response with no quote body.
- **P2-3. Availability coupling.** A COD-offer read failure now fails quote creation for card buyers too. This is acceptable because
  `ListOptions` already depends on the same `read_cod_offer`. It is noted only so the trade-off is on record.
- **Note.** The existing exact-recursive-key buyer quote tests (buyer-http-v1 §, "exact recursive JSON keys") must stay green in CI
  with COD enabled fixtures. The new key is `omitempty`, so stores with COD off keep the old shape.

The `reviewed` field may be updated to cite this review once this file is copied into `output/buyer-route-table/`.

## Commands (cwd = worktree unless noted; S = scratchpad/oc09b)

| Command | Exit |
|---|---|
| `git diff --stat origin/r3/integration...HEAD -- internal/buyerhttp cmd internal/feedhttp` (handler.go, projections.go only) | 0 |
| `git diff --quiet 79f95784c -- <12 other closure files>` | 0 |
| `go run ./scripts/dev/contractdrift -base origin/r3/integration` -> errors=0, GO_ROUTE_TABLE=4 | 0 |
| shared clone @a8bfc7024, `CONTRACT_DRIFT_BASE=tmpbase(79f95784c) cd` control -> errors=0 | 0 |
| same, old pin file -> errors=130, 46 dispatcher-freeze ERRORs (got bfbb4441 want c593c0be) | 1 (expected) |
| same, function entry removed -> errors=130 | 1 (expected) |
| same, declaration entry removed -> dispatcher-freeze ERROR | 1 (expected) |
| clone @79f95784c, same base -> SUMMARY identical; full log diff = one INFO source-line list | 0 |
| `go build ./internal/checkout/ ./internal/buyerhttp/ ./cmd/api/` | 0 |
| `go vet ./internal/checkout/ ./internal/buyerhttp/` ; `go vet ./tests/foundation/` | 0 ; 0 |
| `gofmt -l internal/checkout internal/buyerhttp tests/foundation/oc09b_cod_due_test.go` (no output) | 0 |

VERDICT: APPROVE
