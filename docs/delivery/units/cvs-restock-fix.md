# Unit cvs-restock-fix — T21-01: pay-at-pickup restock after pickup (P1)

Role: commerce_worker (mid tier). Base `23185be`. Worktree `.worktrees/cvs-restock-fix`, branch
`unit/cvs-restock-fix`. No delegation, no new dependency. Migration number **0083** (integrator-assigned).
Source: `output/r3-t21-review/FINDINGS.md` T21-01 (read it fully: scenario, root cause, fix, failing test).
Contract: `contracts/taiwan-cvs-logistics-v1.md` §16 (pay-at-pickup collection state machine) — read §16 only.

## Decisions (binding)
- CR1 Restock guard becomes the positive rule its own comment states: restock is allowed only when the
  latest shipment attempt is `UNCLAIMED`, or when no attempt was ever handed to ECPay (manual /
  MERCHANT_SHIPPED path, i.e. only FAILED/ABANDONED history). Everything else → existing
  `parcel_not_returned`. No deny-list.
- CR2 Status ingest: a 2098 (re-delivery) report that moves the shipment UNCLAIMED → AT_STORE while the
  order's `collection_state='RETURNED'` (and not merchant-restocked) reverts it to `PENDING`, audited
  with the same audit/event pattern the block already uses; a later 2067 then reaches `COLLECTED`
  through the normal branch. A merchant-recorded "returned" (`:1744` in 0073) must also require the
  latest shipment state to be UNCLAIMED or no ECPay attempt (same predicate as CR1 — one SQL helper,
  used by both, not two copies).
- CR3 Forward-only: 0083 `CREATE OR REPLACE`s the affected functions with identical signatures, owners,
  `search_path`, grants and comments (copy the header/grant lines exactly; diff your body against 0073
  so only the guarded lines change). No data backfill: the pilot has no orders.
- CR4 Tests (author smoke; the independent tester adds the full gate): the sequence
  2030→2073→2074→2098→2067→restock must refuse and end `COLLECTED`; 2074→restock still allowed;
  manual path restock still allowed.

## Write paths
`migrations/0083_cvs_restock_guard.sql`, `tests/foundation/taiwan_cvs_pay_at_pickup_test.go` (append
cases only), `contracts/taiwan-cvs-logistics-v1.md` (one amendment paragraph in §16).

## Done when
Static set exit 0 (build, vet, gofmt, check_packet.py, depmap --check); author smoke
`bash scripts/dev/test-focused.sh '^TestTaiwanCvsPayAtPickup'` exit 0 (it serializes on the machine-wide
PG lock; wait for it). Evidence → `/Volumes/data/live_commerce_architecture_v1/output/cvs-restock-fix/`.
