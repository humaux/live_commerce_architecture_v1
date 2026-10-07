# DELIVERY — unit cancel-closes-work-item (backend, Aliyun Qwen)

## 1. Identity

| | |
|---|---|
| Branch / worktree | `unit/cancel-closes-work-item` / `.worktrees/cancel-closes-work-item` |
| Base (trunk) | `0617bc40` |
| Model | Aliyun Qwen |
| Commits (in order) | `e3534f25` brief copy + returns-v1 Amendment 0162 (interface first) → `84866c01` RED test + red.log → `8991abc5` GREEN: migration 0162 + R2 pin 87 → `f97eca08` validator comment note → `962d495c` faithful legacy fixture → final DELIVERY commit (this file) |
| Merged / pushed / deployed | **Never** — integrator only |

## 2. Root cause and fix

`fulfillment.merchant_cancel_order` (0155) never closed the order's `fulfillment.payment_work_items` row (nothing in the
codebase ever DELETEd from that table and its state CHECK has no closed value), so every merchant-cancelled captured order
projected `work_state=READY` forever (`identity.read_merchant_orders`/`_v2`: `coalesce(w.state,'NONE')`). The relaxed Go/TS
validators were only a symptom fix for the store-list 503.

**Fix (smallest change that keeps the gap list correct — ruling recorded in the brief file and the contract):**

* `migrations/0162_cancel_closes_work_item.sql`
  * `GRANT DELETE ON fulfillment.payment_work_items TO commerce_checkout_writer` (:23) — for this definer only; FORCE RLS
    `private_writer` (tenant/store GUCs set by `returns.authorize`) still scopes every row.
  * Full copy of the CURRENT 0155:664-798 body as `CREATE OR REPLACE FUNCTION fulfillment.merchant_cancel_order` (:34-174),
    verified by diff to differ ONLY by: `CREATE` → `CREATE OR REPLACE`, the inserted work-item close (:136-141), and the
    extended `COMMENT` (:174). The card branch, after the refund-coverage gate (`409 refund_first` proved held ≥ CAPTURED)
    and the guarded DEALLOCATE loop, now `DELETE`s the order's `READY` work item in the SAME transaction (I04). Only READY:
    a REVIEW_REQUIRED item belongs to a PAID_ALLOCATION_FAILED order, refused `422 not_cancellable` earlier. ACL/owner/
    EXECUTE re-stated identically (`manual_fulfilment_schema_test` pins them).
  * Forward-only backfill (:185-220): closes legacy `CANCELLED`+`READY` rows EXCEPT outstanding cancel-refund gaps
    (non-failed refunds below the CAPTURED amount of the row's own attempt — the same held/failed vocabulary as
    `returns.list_cancel_refund_gaps`, tied to `w.attempt_id`); gap rows keep READY (money needs a human, I24). Count
    assertion: the DELETE's own row count (one data-modifying CTE — count and delete share one snapshot) plus a post-count
    that must be zero, else `RAISE EXCEPTION`. Idempotent (the test re-runs the whole file twice).
* `tests/foundation/r2_integration_upgrade_test.go` :73-77 — migration-count pin 86 → 87 with the house comment lines
  (PAY-RM1's 0161 lands in parallel; **integrator unions to 88**).
* `internal/merchantorders/orders.go` :412-419 — relaxed CANCELLED+READY clause KEPT (brief), comment now records that 0162
  closes the root cause and the clause protects exactly the legacy open-gap rows (and pre-0162 databases). Logic unchanged.
* `contracts/returns-v1.md` — §3 CONFIRMED-card row (:50) and gap paragraph (:55-59) amended; new "Amendment 0162" section
  (:128-143) with the ruling and the rejected alternative (keeping READY while a refund is in flight would linger after the
  refund SUCCEEDS — the very bug — unless the frozen stripe-refund applier were hooked).
* `docs/delivery/units/cancel-closes-work-item.md` — brief copy + ruling record.
* NOT touched: `apps/admin/lib/orders-model.ts` (Codex territory — see §7), projections, work-state vocabulary, MD6 ship
  eligibility (requires CONFIRMED), RF07 refund replay, go.mod/go.sum, OpenAPI, lockfiles, migration numbers not given.

Why the gap list survives the close: `returns.list_cancel_refund_gaps` is fact-based (`payments.facts` /
`payments.stripe_refunds` / `payments.refund_facts` + the `checkout.merchant_cancel` DEALLOCATE ledger row) and never reads
the work item — asserted by test subtests 3 and 4.

## 3. Tests (evidence class: REAL_PG with MOCK Stripe fakes; no LIVE/SANDBOX provider touched)

| Run | Tree | Result | Evidence |
|---|---|---|---|
| RED `bash scripts/dev/test-focused.sh '^TestMerchantCancelClosesWorkItem$'` | `e3534f25` (HEAD at run time) + the RED test itself, committed right after as `84866c01`; no 0162 in the tree — trunk behavior | **FAIL exit=1** as required: fully-refunded cancel leaves 1 READY row; in-flight cancel leaves 1 READY row; backfill subtest fails (no 0162 file). Never-paid subtest PASSES (regression guard). | `red.log` (SHA `e3534f25…` at top) |
| GREEN `bash scripts/dev/test-focused.sh '^TestMerchantCancelClosesWorkItem$'` | committed `962d495c` | **PASS=1 FAIL=0 SKIP=0 exit=0**, all 4 subtests (never-paid NONE regression guard; fully-refunded cancel closes; in-flight-then-failed gap stays listed with work_state NONE and re-refund clears it; 0162 backfill closes legacy A / keeps open-gap B READY, idempotent re-run) | `green.log` (SHA at top) |
| First green attempt | committed `f97eca08` | FAIL exit=1 — backfill subtest only: my legacy fixture flipped orders with a raw UPDATE, omitting the `checkout.merchant_cancel` DEALLOCATE rows every real cancel writes and the gap list keys on. Targeted fix 1 of 2: fixture now cancels through the real definer and re-inserts the READY row (`962d495c`); assertions unchanged, none weakened. | `green-attempt1-fixture-fail.log` (SHA at top; kept as failure evidence) |
| PG pins `bash scripts/dev/test-focused.sh '^(TestR2IntegrationUpgradeFromReleaseHead\|TestReturns\|TestMerchantCancel\|TestMerchantOrders\|TestWAS)'` (foreground) | committed `962d495c` | **PASS=29 FAIL=0 SKIP=0 exit=0** (`ok livecommerce/tests/foundation 90.381s`), incl. R2 pin 87, `TestMerchantCancelClosesWorkItem`, `TestMerchantCancelRefundGap`, `TestReturns`, all 15 `TestMerchantOrders*`, all 6 `TestWAS*` | `pins.log` (SHA at top) |

(`TestRefund*` does not exist in `tests/foundation` — those tests live in `internal/httpapi` and are covered by the
`go test ./internal/... ./cmd/...` gate below; regexes adjusted to real names as the brief allowed. `TestMerchantCancel`
prefix-matches `TestMerchantCancelRefundGap`, `TestMerchantCancelGroupRace` and `TestMerchantCancelClosesWorkItem` too.
Supplementary: an earlier pin run on the SAME committed SHA that included `TestStripeRF` was externally interrupted
(session teardown) after 26/26 top-level passes — `TestStripeRF05HappyMock`, `TestStripeRF06Unknown`, `TestStripeRF07Lifecycle`
all PASS — kept as `pins-interrupted.log`; it is NOT the authoritative pin evidence, `pins.log` is.)

## 4. Unfiltered gates — exact commands, all re-run on the final committed tree `962d495c` from this worktree

| Command | Exit | Evidence |
|---|---|---|
| `go build ./...` | **0** | `gate-go-build.log` (silent = clean) |
| `go vet ./...` | **0** | `gate-go-vet.log` (silent = clean) |
| `go vet -tags browser ./tests/foundation` | **0** | `gate-go-vet-browser.log` (silent = clean) |
| `go test ./internal/... ./cmd/...` | **0** | `gate-go-test-unit.log` (81 packages `ok`, 0 FAIL; includes `internal/httpapi` TestRefund*) |
| `bash scripts/dev/check-gates.sh` | **0** | `gate-check-gates.log` (`check-gates: ok (75 modes…)`; `check-headers: OK` incl. the new migration; the G-UI5 >500-line WARNs are pre-existing `apps/storefront` files, untouched by this unit) |
| `bash scripts/dev/test-focused.sh '^(TestR2IntegrationUpgradeFromReleaseHead\|TestReturns\|TestMerchantCancel\|TestMerchantOrders\|TestWAS)'` | **0** | `pins.log` (PASS=29 FAIL=0 SKIP=0, SHA `962d495c` on line 1) |

## 5. CI gates the integrator MUST run (not run locally — RAM-heavy / machine policy)

* **G07 full**: `release-gate.sh --strict --only G07` — mandatory for units touching migrations, GRANTs and a SECURITY
  DEFINER body.
* Full foundation suite (`tests/foundation`, all PG tests) on the unioned tree.
* Browser-mode gates (Playwright) on GitHub CI.
* R2 pin union: 0161 (PAY-RM1, upstream 87) + 0162 (this branch, 87) → **88** with both house comment lines kept
  (expected same-line conflict in `r2_integration_upgrade_test.go`).

## 6. Risks

* `commerce_checkout_writer` gains DELETE on `fulfillment.payment_work_items` — usable only through this definer; RLS
  (FORCE) still scopes rows to the acting tenant/store. No other code path deletes work items.
* The backfill is forward-only (house rule); its gap predicate mirrors `list_cancel_refund_gaps` — if that vocabulary ever
  changes, the two must change together (both now documented in returns-v1 Amendment 0162).
* Cancel with an in-flight refund that later FAILS leaves work_state NONE by design: the gap list is the single surface for
  that money (contract ruling; asserted by subtest 3, including the re-refund clearing the row).
* Replay safety: `merchant_cancel` idempotent replay returns the stored response before any write; the DELETE is a no-op on
  re-execution paths.

## 7. NOT_RUN / BLOCKED / integrator to-do

* NOT_RUN locally: full foundation suite, browser gates, G07 strict (→ GitHub CI, §5). No LIVE/SANDBOX payments anywhere
  (MOCK fakes only). No production host, real money, real key or buyer PII touched; no data deleted beyond the migration's
  own designed backfill.
* BLOCKED: none. (One blocker occurred — the unfaithful legacy fixture — resolved with 1 of the 2 allowed targeted fixes.)
* Integrator to-do:
  1. Union the R2 pin to 88 (§5) — same-line conflict expected.
  2. Have **Codex** add the matching 0162 note to the relaxed clause comment in `apps/admin/lib/orders-model.ts` (~:281-288)
     — I may not touch `apps/`; the Go twin (`orders.go` :412-419) carries the note already. Logic on both sides stays as-is.
  3. Evidence location: per the final orchestration instruction, all evidence (red.log, green.log,
     green-attempt1-fixture-fail.log, pins.log, pins-interrupted.log, gate-*.log, this DELIVERY.md) stays in THIS worktree
     under `output/cancel-closes-work-item/` and is COMMITTED with the final commit — nothing was copied to the main
     checkout. If the worktree is deleted before harvest, recover the evidence from the branch (`git show
     unit/cancel-closes-work-item:output/cancel-closes-work-item/DELIVERY.md` etc.).
