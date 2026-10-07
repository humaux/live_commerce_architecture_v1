# Unit: cancel-closes-work-item — root-cause fix for READY work on merchant-cancelled orders (backend) — Aliyun Qwen

(Copied verbatim from the delegating prompt; no separate brief file existed. Trunk 0617bc40,
worktree `.worktrees/cancel-closes-work-item`, branch `unit/cancel-closes-work-item`.)

- Worktree: current directory (`.worktrees/cancel-closes-work-item`, branch `unit/cancel-closes-work-item`, trunk 0617bc40). No brief file: this section is the brief; copy it to `docs/delivery/units/cancel-closes-work-item.md` first.
- Background (trunk c080f870, memory "[fix] 商家取消已付款订单后整个订单列表 503"): W3-08B merchant cancel (`fulfillment.merchant_cancel_order`, migration 0155) never closes the order's payment work item, so a cancelled captured order keeps `work_state = READY` in the merchant order projection. The Go validator (`internal/merchantorders/orders.go` validSummary) and the admin parser (`apps/admin/lib/orders-model.ts`) were relaxed to accept READY on CANCELLED/CANCELLED+captured so the list stops 503-ing. That is a symptom fix.
- Goal: make the DATA right. A merchant-cancelled order's work item should be closed (or projected as NONE) EXCEPT when money still needs a human: the cancel-refund gap (`GET /orders/cancel-refund-gaps`, an in-flight refund that later failed) must remain visible as needing work (decide: keep READY there, or a dedicated state if the contract allows — prefer the smallest change that keeps the gap list correct).
- Find: how work_state is derived (grep `work_state` in migrations/*.sql — the latest definition of the merchant order projection/definer, and the payment work item table/status), and the 0155 cancel definer. Copy CURRENT bodies (`grep -ln "FUNCTION <schema>.<fn>" migrations/*.sql | tail -1`) for any CREATE OR REPLACE; prefer the house in-place patch pattern (pg_get_functiondef + exactly-once anchor RAISE) only if a full copy is impractical.
- Migration **0162_cancel_closes_work_item.sql** (0161 = PAY-RM1 in parallel). Forward-only; also fix existing rows (cancelled + no outstanding refund gap → close/NONE), with a count assertion.
- Keep the relaxed validators (they are harmless and protect old data); add a note in their comments that 0162 closes the root cause.
- Contract: amend `contracts/returns-v1.md` (and manual-fulfilment / stripe-refund contract if they define work_state) — interface first.

## Gate discipline (mandatory)
- Red first: a PG test where a merchant cancels a fully refunded paid order → work_state must be NONE (fails on trunk); a cancel with a later-failed refund → still in the gap list and needs work; a never-paid cancel → NONE. Save red.log with SHA.
- UNFILTERED: `go build ./...`, `go vet ./...`, `go vet -tags browser ./tests/foundation`, `go test ./internal/... ./cmd/...`, `bash scripts/dev/check-gates.sh` — exit codes in DELIVERY.md.
- PG pins: `test-focused.sh '^(TestR2IntegrationUpgradeFromReleaseHead|TestReturns|TestMerchantCancel|TestMerchantOrders|TestRefund|TestStripeRF|TestWAS)'` (adjust to real names) on the COMMITTED tree, SHA at the top of green.log. R2 count on trunk 86 → +1 (PAY-RM1 also +1 in parallel; integrator unions).
- Do NOT claim anything you did not run. Write files in pieces ≤ ~250 lines per Write/Edit; commit in small steps.

FINAL STEP: write `output/cancel-closes-work-item/DELIVERY.md`, then `git add -A && git commit`. Return: SHA, ≤8-line summary, CI gates, NOT_RUN.

## Ruling recorded during the unit (design decision the brief asked for)

Smallest change that keeps the gap list correct: `merchant_cancel_order` DELETES the order's READY
`fulfillment.payment_work_items` row in the same transaction (the cancel already enforces refund
coverage, 409 refund_first). The gap list (`returns.list_cancel_refund_gaps`) is fact-based and does
not read work items, so it stays the single surface for "money needs a human" when an in-flight
refund counted at cancel time later FAILS. Conditional keeping (READY while a refund is in flight)
was rejected: the common flow refunds and cancels while the refund is still in flight, so it would
leave the READY row lingering after the refund succeeds — the very bug being fixed — unless the
frozen stripe-refund observation applier were hooked. The 0162 backfill originally kept READY for
legacy rows with an outstanding gap; the integrator's review ruling (P1-b) dropped that exception:
the backfill closes EVERY READY row of a CANCELLED order, because the gap is carried by
`returns.list_cancel_refund_gaps` (never reads the work item) and the new definer already gives the
same business state NONE.
