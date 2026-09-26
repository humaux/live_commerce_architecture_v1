# Local seed-pool lifetime correction

Status: FROZEN candidate contract / NOT_RUN. Base `3e1d37c`.
Parent LRC06/WSD04 are blocked; the original same-capacity Meta maintenance
replay returned PostgreSQL `53300`. No production runtime fix is implied.

## Minimal cause and scope

Four `ewSetup` calls retain five newly owned pools each until top-level cleanup,
although that call site only needs a committed expiry job ID. Static inventory
counts pool objects, not actual PostgreSQL backends. Reuse the existing
`closeSeedPools` pattern in legacy isolation tests; close only these five owned
pools after seed use, never the shared owner/runtime pool or another harness.
Apply the same lifetime rule to sibling ID-only seed call sites. Do not close a
harness that subsequent operations still use. Prefer a small shared test helper
over duplicated cleanup or a connection-budget/configuration framework.

Author write paths: `tests/foundation/expiry_runtime_test.go`,
`tests/foundation/meta_isolation_maintenance_test.go`,
`tests/foundation/legacy_runtime_isolation_test.go`, and one focused
`tests/foundation/meta_seed_pools_test.go` if required for the independent gate.
No product source, migration, dependency, capacity, timeout, queue controls,
runtime flag or retry changes. Root owns this contract/evidence; PG work serial.

## Gates

- SPL01: real native SHOW values for max/reserved slots, scoped pg_stat_activity
  counts and pool Stat values establish actual seed connection ownership.
  After each close, those exact seed roles have zero backends within the
  existing bounded readiness envelope; shared/foreign pools remain usable.
  Output only safe counts/classifications, never DSN/config/query/key values.
- SPL02: the original `TestMetaRuntimeIsolationTwoWayRealMaintenance` passes
  at the unchanged 30-slot fixture with all scheduler/rescuer/cleaner positive
  controls and exact foreign snapshots intact. Retain prior 53300 RED evidence;
  do not label the old uninstrumented failure's exact occupancy as proven.
- SPL03: existing startup-failure, cancellation and shutdown pool cleanup gates
  remain. Independent review accepted the retained native 53300 RED plus SPL01/02
  as the minimum gate; no additional artificial capacity-negative is required
  for this test-only lifetime change. Do not claim coverage of every exhausted
  capacity cleanup path. Never force an intermittent failure until it occurs.
- SPL04: independent source/evidence review and complete root PG/race/vet,
  unchanged per-test deadlines and task-owned fixture cleanup before merge.

This corrects local test-resource ownership, not production capacity planning,
pool admission, HA/PITR or provider qualification. No customer system is touched.
