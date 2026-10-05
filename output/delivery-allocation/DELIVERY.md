# delivery-allocation — DELIVERY

P0 backend unit: **a merchant who only enables a delivery service (宅配/超商/自取…) must produce a
buyer-selectable option**, without the merchant understanding "warehouse allocation". The pre-fix defect:
`internal/checkout/options.go` INNER JOINs `fulfillment.allocation_heads` (options.go:190), and
`fulfillment.SetAllocation` was only ever called from tests — no migration default, and no admin/BFF/CLI
path called it — so an enabled service with no allocation was invisible to buyers.

Author model: DeepSeek V4-Pro (backend worker). Base SHA `3a894ef2`. Worktree
`.worktrees/delivery-allocation`, branch `unit/delivery-allocation`.

## Tier

DESIGN + MODEL_ONLY. The domain code, migration and foundation tests compile and the DB-free unit gate
passes, but the **real-PG foundation gate and browser gates are NOT_RUN** in this session (disposable PG
provisioning required approval that was not granted — see NOT_RUN/BLOCKED). Nothing here is a product
pass.

## Implementation

- `internal/httpapi/settings.go:22-30` — the merchant settings PUT route
  (`PUT /v1/admin/stores/{store_id}/markets/{market_id}/countries/{country}/delivery-services/{code}`)
  now calls `fulfillment.SetServiceWithDefaultAllocation` instead of `fulfillment.SetService`. This is
  the same BFF/Go route the admin UI uses.
- `internal/fulfillment/service.go:78-82` — `Service.DefaultWarehouseID` field
  (`json:"default_warehouse_id,omitempty"`), set only by the settings-path wrapper so the UI can show
  the chosen default.
- `internal/fulfillment/service.go:207-232` — `SetServiceWithDefaultAllocation`: writes the service
  revision exactly as `SetService` (same CAS, `command.Run` idempotency, `expected_version`), then, when
  the resulting service is **enabled**, calls `ensureDefaultAllocation` in the **same transaction**.
  Disable/emergency-stop keeps the allocation (history) and simply stops listing the option. A replayed
  command result also re-reads the existing allocation, so replay responses carry the correct
  `default_warehouse_id`.
- `internal/fulfillment/allocation.go:147-214` — `ensureDefaultAllocation` (returns the first-position
  warehouse id): allocation advisory lock → `allocation_heads FOR UPDATE` → on first enable, pick the
  store's default warehouse (sole active, else first active by `created_at, id`), `lock_warehouse`
  re-check, insert `allocation_versions`(v1, `service_version`=current service version,
  `warehouse_count`=1) + `allocation_warehouses`(position 1) + `allocation_heads`, and audit
  `fulfillment.allocation.ensure`. Existing heads are never overwritten. No warehouse / inactive
  warehouse → `command.ErrConflict` (HTTP 409), and the service write rolls back with it — never a
  silent "enabled but no allocation". Lock order matches `SetAllocation`
  (service head → allocation advisory → allocation head → warehouse), so concurrent enable/update and
  explicit `SetAllocation` serialize on the same `fulfillment.allocation|…` advisory key.
- `migrations/0117_delivery_allocation_backfill.sql` — one-time backfill (draft number 0117; integrator
  assigns the final number): adds `inventory.warehouses.created_at` (IF NOT EXISTS), then backfills an
  enabled service that still lacks an allocation head with a single-warehouse head pointing at the
  store's first active warehouse. Forward-only, checksummed by the migration runner, repeatable no-op
  (`NOT EXISTS(allocation_heads)` guard). Disabled services are never backfilled.
- `tests/foundation/delivery_allocation_auto_test.go` — new foundation tests (real-PG):
  - `TestDeliveryAllocationAutoMerchantHTTPShowsBuyerOption` — real HTTP PUT via `httpapi.NewHandler`,
    then real buyer `/v1/buyer/checkout-options` shows the service with
    `service_version=1 / allocation_version=1`; response `default_warehouse_id` equals the persisted
    position-1 warehouse.
  - `TestDeliveryAllocationAutoConcurrentEnableSingleHead` — 4 concurrent same-key enables → all 200,
    exactly 1 service head/version and 1 allocation head/version/warehouse (runs under `-race`).
  - `TestDeliveryAllocationBackfillMigration` — enabled service without allocation → apply the raw
    migration file → one single-warehouse head; disabled service untouched; re-run is a no-op.
  - `TestDeliveryAllocationAutoWarehouseDefaultAndNoWarehouse` — fresh store: no active warehouse →
    HTTP 409 and the service write rolls back; two active warehouses with explicit `created_at` → the
    earlier one is chosen and returned.
- `tests/foundation/browser_promotions_test.go:102-120` — the browser gate now enables its service
  through the real merchant settings route (task #4: replaced the `dsSet`+`daSet` Go fixture), and
  asserts `DefaultWarehouseID == e.p.stock.warehouse.ID`.

## Tests actually run (commands + exit codes)

| # | command | exit | note |
|---|---------|------|------|
| 1 | `go build ./...` | 0 | all Go packages compile |
| 2 | `go vet ./internal/fulfillment/... ./internal/httpapi/... ./internal/checkout/... ./tests/foundation/` | 0 | clean |
| 3 | `go vet -tags browser ./tests/foundation/` | 0 | browser gate change compiles |
| 4 | `go test -race -count=1 -run 'Allocation|DeliveryOption|Checkout' ./internal/fulfillment/... ./internal/checkout/...` | 0 | fulfillment `ok`; checkout `[no tests to run]` (no checkout test name matches that regex) |
| 5 | `go test -race -count=1 ./internal/fulfillment/... ./internal/checkout/...` | 0 | full DB-free unit suite green |
| 6 | `python3 scripts/check_packet.py` | 0 | `PASS_PACKET_STRUCTURE_ONLY` (structure only) |
| 7 | `go test -race -count=1 -run 'Allocation|DeliveryOption|Checkout' ./tests/foundation/` | 0 (SKIP) | **NOT a real pass**: `LC_TEST_DATABASE_ALLOWED` unset → fixture returns NOT_RUN and zero tests execute |

## Red → green evidence

**Not produced.** The four new foundation tests are the red→green artifact, but they require a real PG
18.6, and provisioning the disposable container (`bash scripts/dev/test-focused.sh …` and the `docker
run` it performs) required approval that was not granted in this session. So no red run and no green run
were recorded for `./tests/foundation`. The tests were written after the implementation (not strictly
red-first), and none of the new assertions have been executed. This is the single biggest gap and must be
closed by the integrator/acceptance before merge (see NOT_RUN/BLOCKED).

## NOT_RUN / BLOCKED

1. **Foundation gate (core red→green)** — `bash scripts/dev/test-focused.sh 'Allocation|DeliveryOption|Checkout' ./internal/fulfillment/... ./internal/checkout/... ./tests/foundation/` → BLOCKED: the harness (and any direct `docker run`) returned "requires approval"; no disposable PG could be started. An unrelated session also held the focused-test mkdir lock (`/tmp/lc-test-pg.lock`) during this work, so the serialized harness would have waited.
2. **Browser gates** — `--browser-checkout-offline`, `--browser-home-cod` (and the changed `--browser-promotions` gate) → NOT_RUN: browser build + the same PG provisioning approval, and the full-suite runtime (~25 min) was not available here.
3. **`bash scripts/dev/check-gates.sh`** → BLOCKED (exit 1) before reaching the Go/python checks: the worktree has no `node_modules`, so `scripts/dev/ui-architecture-gate.mjs` fails `Cannot find module 'typescript-api'`. Unrelated to this change (Node deps not installed in the worktree). The migration passes the two checks it gates: 0117 has no `GRANT … TO commerce_worker` (verified by grep), and every tracked `*.test.*|*.spec.*` file still has a runner (my new file is a `.go` test under `tests/foundation`, run by `go test`).
4. `python3 experiments/spec_models.py --out experiments/results` → NOT_RUN ("requires approval").

## Risks

- **Unverified DB behavior.** `ensureDefaultAllocation` inserts `allocation_versions` with
  `service_version=current service version` and `warehouse_count=1`, then `allocation_warehouses`(1 row),
  then `allocation_heads`; the deferred `check_allocation_complete` trigger (0011) must accept this on
  commit. The SQL mirrors `SetAllocation`, but it has not been executed against PG in this session.
- **Shared-store determinism.** `tests/foundation` fixtures are a `sync.Once` singleton; `storeA1`
  accumulates warehouses across tests. The auto tests therefore assert `default_warehouse_id != ""` and
  that it equals the persisted position-1 child (not a specific id); the deterministic
  "first-by-created_at" case is covered on a fresh store. If a future test inserts an inactive-flagged
  warehouse into `storeA1`, the auto tests still hold (they read the head's own child).
- **Migration numbering gap.** The repo's latest migration is `0113`; `0117` is a draft number from the
  brief and lexically applies last, which is correct here, but the integrator must renumber to the next
  sequential number on final merge (and it is forward-only/checksummed — do not edit after the runner
  has recorded it).
- **`inventory.warehouses.created_at` is a new shared column** (added by 0117) — a shared-schema change
  the integrator must coordinate (see below). `ALTER … IF NOT EXISTS` + `DEFAULT clock_timestamp()` is
  safe to re-run; the deterministic `id` tiebreak covers rows created before the column existed.
- **Replay surface.** `ensureDefaultAllocation` is intentionally not wrapped in `command.Run`; its
  idempotency is the head existence re-check under the allocation advisory lock. If a future caller
  writes a service and commits before `ensureDefaultAllocation`, that would break the "same transaction"
  invariant — the wrapper keeps them in one `pgx.Tx`, and the httpapi `bodyRoute` owns the transaction.

## Integrator items

- **Migration number**: `0117` is a draft; assign the final sequential number (next after 0113).
- **Shared schema**: `inventory.warehouses.created_at timestamptz NOT NULL DEFAULT clock_timestamp()`
  (added by the backfill migration) must be reflected in any shared JSON schema / OpenAPI / owner docs.
- **Permission list**: no new roles/grants. The route reuses `integration:manage` (write) and
  `integration:read` (read). Migration 0117 grants nothing (verified: no `GRANT`/`TO commerce_worker`).
- **OpenAPI / response shape**: `Service` gains `default_warehouse_id` (omitempty) on the settings
  delivery-service response; only the settings-path wrapper sets it. Shared JSON schema should be updated
  accordingly (integrator-only merge).
- **Non-author acceptance**: required — the author (this worker) is the only one who has reviewed this
  change; the real-PG gate and browser gates above must be re-run by the integrator/test_worker.

## Write paths used (all within allowance)

`internal/fulfillment/service.go`, `internal/fulfillment/allocation.go`,
`internal/httpapi/settings.go`, `migrations/0117_delivery_allocation_backfill.sql`,
`tests/foundation/delivery_allocation_auto_test.go`, `tests/foundation/browser_promotions_test.go`,
`output/delivery-allocation/DELIVERY.md`. No apps/UI, no go.mod/go.sum/OpenAPI/pnpm-lock changes.
