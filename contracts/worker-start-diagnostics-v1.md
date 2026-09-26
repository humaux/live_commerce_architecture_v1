# Worker start diagnostic witness (bounded local gate)

Status: FROZEN / NOT_RUN. Parent LRC06 is blocked at root `1fbd997` by one
`TestMetaRuntimeIsolationTwoWayRealMaintenance` native worker-start failure.
The generic error proves the constructor preflight passed, not why Start failed.
Keep the full1 failure and original process log; do not conflate it with the
earlier zero-log readiness timeout or call the underlying cause fixed.

## Minimal scope

Only `internal/jobqueue/run.go` and `run_test.go` for the author; root owns this
contract and integration/evidence. No new dependency, migration, runtime flag,
retry, timeout increase or provider/customer operation. `ErrStart`, worker
lifetime, watchdog join/disarm and successful readiness behavior stay unchanged.

On a failed start emit one structured diagnostic with fixed enumerated phase
and category; include only an explicitly allowlisted native PostgreSQL SQLSTATE
when discoverable through the error chain. Unknown values remain `other`/empty.
Never render the underlying error, its message/detail/hint/query, configuration,
DSN, environment, password, token or key. This is diagnostic hardening, not a
claimed fix for the original failure. Do not add a general logging framework.

## Acceptance gates

- WSD01: unit/race/vet prove invalid input, native Start error, watchdog and
  signal are distinguishable while the same `errors.Is(err, ErrStart)` behavior
  and original timing/lifetime tests remain intact.
- WSD02: secret-marker errors (including wrapping and forged SQLSTATE) never
  appear in captured output; every emitted value belongs to its allowed set.
  Known native SQLSTATE is available, unknown values fail closed.
- WSD03: one targeted original real-PG maintenance-process test after diagnostic
  deployment in the local fixture. Record source, exit/hash and process log on
  failure. Passing once is not evidence of the original root cause.
- WSD04: independent source review and full root PG/race/vet must close the
  parent acceptance. Any recurrence needs the new witness plus bounded PG/process
  evidence; do not rerun unchanged until a failing outcome happens to disappear.

Production deployment, public Meta qualification, customer workloads and whole
SaaS recovery remain excluded. No release guarantee comes from this witness.
