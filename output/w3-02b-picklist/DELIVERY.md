# w3-02b-picklist — delivery record

- **Unit brief**: `docs/delivery/units/w3-02b-picklist-export.md` (DRAFT; Integrator 裁决 overrides).
- **Author / role**: DeepSeek V4-Pro — backend only (no UI/`apps/`). No sub-agents spawned.
- **Actual model**: `deepseek-v4-pro`.
- **Reasoning**: see "Design deviation (integrator flag)" below — the batch label request cannot be a per-order savepoint because `plan_cvs_create` verifies the River job by `xmin`.
- **Base SHA**: `0c1dce66` — **branch**: `unit/w3-02b-picklist` — **worktree**: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-02b-picklist`.
- **Write paths (unique ownership)**:
  - `internal/merchantorders/picklist.go`
  - `internal/merchantorders/carrier_export.go`
  - `internal/fulfillment/cvs_batch.go`
  - `internal/httpapi/picklist.go`
  - `migrations/0130_pick_list.sql`
  - `tests/foundation/pick_list_test.go`
- **Supporting edits (shared files, minimal)**:
  - `internal/fulfillment/cvs.go` — comment only (reverted a temporary debug `fmt.Printf`; no behaviour change).
  - `internal/httpapi/handler.go` — route wiring for the three endpoints.
  - `internal/httperror/error.go` — `picklistClassify` 422 `too_many`.
  - `tests/foundation/manual_fulfilment_schema_test.go`, `merchant_orders_v2_acl_test.go`, `worker_authority_split_test.go` — ACL-pin additions for the two new SECURITY DEFINER functions.

## What was delivered

1. **拣货单 Pick list** — `POST /v1/admin/stores/{store_id}/orders/pick-list`, body `{order_ids:[≤500]}` XOR `{session_id}`, permission `orders:read`. Sorted by `sku_code`; result carries `orders`, `totals`, `skipped` (codes `not_pickable|order_not_found`). Only `CONFIRMED|AWAITING_COLLECTION` AND `fulfillment_state='MANUAL_UNASSIGNED'` rows are pickable.
2. **承运商导出 Carrier export** — `POST …/orders/export?template=black_cat|hsinchu|chunghwa_post|generic`, permission `orders:export`, audit `orders.carrier_export`. Go constant templates (≤15 columns), UTF-8 BOM, formula-injection guard (leading `=+-@` prefixed with `'`), `collect_minor = total_minor + coalesce(cod_surcharge_minor,0)`.
3. **超商批量建单 CVS batch** — `POST …/shipments/cvs-batch`, `Idempotency-Key` header, body `{order_ids:[≤100]}`, permission `fulfillment:write`. Per-order result `{order_id, outcome: queued|already|failed, code?}`.
4. **Migration 0130** — `fulfillment.read_pick_list(bytea,uuid,uuid[],uuid)` (owner `commerce_checkout_writer`, EXECUTE `commerce_runtime` only) and `claims.pick_list_session_orders(uuid,uuid,uuid)` (SECURITY DEFINER, `search_path=pg_catalog`, `REVOKE ALL FROM PUBLIC`, EXECUTE `commerce_checkout_writer`; `session = live_price_uses ∪ order_origins`). No new tables/columns/roles.

## Design deviation (integrator flag)

The brief worded the CVS batch as **one transaction with per-order savepoints**. That is impossible against the frozen `integration.plan_cvs_create` (0073:1046-1048): it verifies the freshly inserted River job with `r.xmin = pg_current_xact_id()::xid`. A row INSERTed inside a `SAVEPOINT` carries the **sub**-transaction XID as its `xmin`, while `pg_current_xact_id()` always returns the **top-level** XID, so every pickable order is refused `22023 invalid cvs plan` (the pre-fix red run in `red.log` shows exactly that). A per-order top-level transaction, by contrast, keeps the job's `xmin` top-level and lets a per-order refusal roll back orphan-free (`post_river/0014` refuses to commit an unlinked job).

The batch therefore reuses the existing single-order entry (`Request → requestOne → fulfillment.request_cvs_shipment`, the same SQL plan and the same `external_operation_v1` River kind) in **one top-level READ COMMITTED transaction per order**, serialised by an outer `command.Run` advisory lock; the aggregate `Idempotency-Key` is the replay authority. This needs **no new definer, grant, role or River kind** (respecting the brief's constraints).

Second deviation (idempotency, P0): the per-order idempotency key is **derived deterministically** from `(batch key, order id)` — `pb-` + first 16 bytes of `SHA-256(batchKey \x00 orderId)` hex — never random. A clean replay is answered by the outer `command.Run` without re-entering the loop; a **partial** replay (a fatal error mid-batch rolls the aggregate result back while some per-order transactions already committed) re-enters the loop but `request_cvs_shipment` recognises the stored per-order key (0073:1086-1088) and replays it instead of inserting a second River job. A genuinely new batch (different key) still becomes attempt N+1.

## Test commands, exit codes, evidence

| command | exit | result |
|---|---|---|
| `go build ./...` | 0 | pass |
| `go vet ./internal/fulfillment/... ./internal/merchantorders/... ./internal/httpapi/...` | 0 | pass |
| `gofmt -l <touched files>` | 0 | clean |
| `bash scripts/dev/test-focused.sh '^(TestCVSBatch\|TestCVSBatchUnknown\|TestPickList\|TestPickList500\|TestCarrierExport\|TestMerchantOrdersV2\|TestWAS0\|TestManualFulfilmentMF02)'` | 0 | **20 tests PASS, 0 FAIL** (final green — `output/w3-02b-picklist/green.log`) |
| `bash scripts/dev/test-local.sh --merchant-orders` | 0 | pass |
| `bash scripts/dev/test-local.sh --checkout` | 0 | pass |
| `bash scripts/dev/check-gates.sh` | 0 | pass (header ratchet + 68 gate modes) |
| `python3 scripts/check_packet.py` | 0 | `PASS_PACKET_STRUCTURE_ONLY` |
| `python3 experiments/spec_models.py --out experiments/results` | — | NOT_RUN (requires approval; validates executable spec models, not this backend unit) |

- **PL08 timing**: 500-row pick list answered in **87 ms** (< 10 s requirement).
- **Evidence files**: `output/w3-02b-picklist/red.log` (initial red: FK violation + 2× CVS `invalid_order`), `output/w3-02b-picklist/green.log` (final green, PASS=20 FAIL=0), `output/w3-02b-picklist/PLAN.md`. A scratch `debug.log` (the temporary `fmt.Printf` that produced the 22023 evidence) was deleted.

## Test table (PL01–PL08)

| case | coverage |
|---|---|
| PL01 | totals merge + per-order lines |
| PL02 | shipped/cancelled/foreign → `not_pickable`/`order_not_found` |
| PL03 | 501 ids → 422 `too_many` |
| PL04 | 4 export templates: exact headers, BOM, formula guard, collect/total cells |
| PL05 | `orders:read` vs `orders:export` permission split |
| PL06 | 3 CVS queue exactly 3 jobs, same-key replay adds none, home delivery → `failed:not_cvs` |
| PL07 | dispatcher → UNKNOWN, no requeue |
| PL08 | 500 rows < 10 s |

## Risks / NOT_RUN / BLOCKED

- **Risks**:
  - Partial-batch commit on a mid-batch fatal error (per-order transactions are independent of the outer aggregate result). Mitigated by the deterministic per-order keys so a same-key replay is idempotent — but the client must be told to replay the **same** `Idempotency-Key` to converge.
  - The `already` outcome code is reserved but not exercised by PL06/PL07 (it is only reachable through a partial-replay path); the single-order SQL's `not_shippable` refusal is the code a live re-request returns instead.
- **NOT_RUN** (by me, this session):
  - Full `go test -race ./...` entire suite — not run by me (out of this unit's scope; the focused green + `--merchant-orders` + `--checkout` gates cover the touched surface).
  - `python3 experiments/spec_models.py --out experiments/results` — requires interactive approval; validates executable spec models, unrelated to this backend unit.
  - Browser/UI acceptance — out of scope for a backend-only unit (owner's real-click requirement applies to UI units).
- **BLOCKED**: none. The two CVS batch tests were red at `invalid_order` (22023) until the per-order-transaction design landed; both are green.

## Environment designation

DESIGN + SANDBOX. The foundation tests run against a throwaway Docker PostgreSQL (real PG/River, no external provider call — the ECPay fake returns configured mock responses). No LIVE production action, no real refund, no label purchase, no replay marketing.
