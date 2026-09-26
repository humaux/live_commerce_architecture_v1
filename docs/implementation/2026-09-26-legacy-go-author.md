# Legacy family River Go wiring — author evidence

- Role: `commerce_worker`; actual model: `gpt-6-sol`, high reasoning.
- Frozen contract/base: `contracts/legacy-runtime-isolation-v1.md` at `9772380`.
- Branch/worktree: `commerce/legacy-family-go-20260926` at `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`.
- Write scope: the five production Go files below and this evidence file. No foundation tests, SQL, migration runner, platform code, dependencies, provider calls or production operations changed.

The checkout API producer and expiry consumer now use fixed `river_expiry`. The hosted-payment API producer, payment consumer and reconciliation-job producer now use fixed `river_payment`. Existing constructor signatures, profile queues, worker maps, retry/timeout defaults, pool authority checks and transaction boundaries are unchanged. The external producer remains on `river`; Meta remains on `river_meta`.

Changed Go paths: `cmd/api/buyer.go`, `cmd/api/buyer_payment.go`, `internal/checkout/runtime.go`, `internal/payments/runtime.go`, `internal/payments/query_worker.go`. The pre-existing unit suites exercise these packages and the two worker entrypoints; no unit test was added for a literal `Config.Schema` because constructor success requires a real authorized database and is assigned to the independent foundation/PG gate. A tautological source-string assertion would not prove the cross-schema transaction boundary.

Author checks on this branch before commit:

| Command | Exit | Result |
| --- | ---: | --- |
| `go test -count=1 ./cmd/api ./internal/checkout ./internal/payments` | 0 | Three packages passed |
| `go test -race -count=1 ./cmd/api ./internal/checkout ./internal/payments` | 0 | Three packages passed |
| `go test -race -count=1 ./cmd/payment-worker ./cmd/expiry-worker` | 0 | Both entrypoint packages passed |
| `go vet ./cmd/api ./internal/checkout ./internal/payments` | 0 | No diagnostics |
| `git diff --check` | 0 | No whitespace errors |

`LRI01`–`LRI06`, real PG18, cross-schema SQL admission, browser and full-repo regression are **NOT_RUN in this author lane**. The shared migration and independent foundation fixture changes must land before those gates can be evaluated. Unit/race/vet success is not product acceptance.
