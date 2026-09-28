# Stripe B1 independent test candidate evidence

Base SHA: `529cb0fa18bed5476516ee34fd6154f89aeb5c2b`.
Role: independent `test_worker`; model `gpt-6-sol` high; branch `unit/stripe-b1-tests-a`.

| Tier | Command | Exit | Result |
| --- | --- | ---: | --- |
| REAL_PG | `bash scripts/dev/test-focused.sh '^TestStripeSP(06|02|19)'` | 1 | PASS=0, FAIL=3, SKIP=0; expected red on clean base without 0061 |
| UNIT | `go test -race -count=1 ./internal/integrations/psp/stripe/stripetest` | 0 | fake self-tests passed |
| UNIT | `go test -count=1 ./internal/integrations/psp/stripe/...` | 0 | adapter plus fake passed |
| STATIC | `go vet ./internal/integrations/psp/stripe/... ./tests/foundation/` | 0 | passed |
| COMPILE | `go test -run '^$' ./tests/foundation` | 0 | SP02/SP06/SP19 compiled |

The raw REAL_PG log is `red.log` beside this file. It records missing `payments.stripe_amount_ok`, migration `0061_stripe_psp.sql`, and the obsolete registrar signature in the pre-amendment SP19 draft; the obsolete registrar assertion was removed after this run. No further PG run was made pending the frozen §0.2 amendment.

Current SP06 is a provisional catalog and CHECK-negative skeleton. Two-store success and cross-binding rejection, complete effective RLS, all CHECK negatives, set-once trigger behavior, and queue guard behavior are NOT_RUN. SP19 covers only the SQL `REAL_LIVE` CHECK-negative. No SANDBOX, BROWSER or LIVE call was made.

## §0.2 behavioral candidate (2026-09-29)

New tests in `tests/foundation/stripe_schema_test.go` compile against the frozen interfaces:
`TestStripeSP06RegistrarIsolation`, `TestStripeSP06EndpointCustodyAndRLS`,
`TestStripeSP13EndpointPrepareLocksRotation`, and
`TestStripeSP13AcceptedReceiptNeedsCommittedLink`.
`go test -run '^$' ./tests/foundation`, `go vet ./tests/foundation`, and
`go test -count=1 ./internal/integrations/psp/stripe/stripetest` each exited 0.
REAL_PG execution of these new tests is **NOT_RUN** while the SQL writer owns the machinewide
PG slot. The accepted-receipt test exercises the deferred final-row invariant using an owner
fixture; the full prepare → River InsertTx → commit positive path is NOT_RUN. Historical
attempt processing, every §6.1 CHECK negative, and full queue guard routing remain NOT_RUN.
