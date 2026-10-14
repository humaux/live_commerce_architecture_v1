# CARD-PG-HEADROOM — delivery

Role: test-infra worker (write). Base 625544483, branch `unit/card-pg-headroom`. Evidence class: DESIGN (code reading) + compile only; nothing executed.

## Root cause
The guard (`harnessWatchCapacity`) counts every `client backend` in the cluster. At the guard start the buyer half still kept:
- the admin stack's identity pool (`cbbrStartAdmin`, never closed before test cleanup),
- the registrar pool (`e.reg`, sstEnv),
- the payment worker pool (`e.pool`) with its running River client, and the webhook ingress pool,
- every protected A/P pool at its setup-time high-water (pgx keeps idle backends up to MaxConns for 30 min).
All `platform.Open*Pool` pools are `MaxConns = 8` (`internal/platform/platform.go:159`). The sum of what could be open was far above 54, so the
observed 55–56 (RC run passed) is the nondeterministic idle/live mix, not a leak in one pool.

## Inventory (pool → MaxConns → open during buyer half, before → after this change)
N = `pgxpool.New` default `MaxConns = max(4, NumCPU)` (pwIsolatedFixture, `payment_runtime_test.go:~92`); 4 on a 4-vCPU runner.

| Pool | Opened at | MaxConns | Before | After |
|---|---|---|---|---|
| `e.f.owner` | `payment_runtime_test.go:92` | N | open | open (needed: count/mustExec) |
| `e.f.runtime` | `payment_runtime_test.go:~126` (OpenPool) | 8 | open | open (e.read, ensureStock) |
| `e.hosted` | `stripe_start_test.go:97` | 8 | open | open (buyerhttp payment view: `payment.go:98`) |
| store A `a.a.runtime`, `a.a.issuer`, `a.pool` | `payment_start_test.go:97,121` | 3×8 | open | open for nodes 1–2, **closed after node 2** |
| platform `plat.p.a.runtime`, `a.issuer`, `p.pool` | same | 3×8 | open | **Reset (0 backends) at guard start**, used by nodes 3–4 |
| `e.pool` (payment worker, River) | `stripe_flow_test.go:67` | 8 | open | **closed** (worker stopped; buyer half only reads) |
| `e.ingress` | `stripe_flow_test.go:73` | 8 | open | **closed** |
| registrar `e.reg` | `stripe_start_test.go:93` | 8 | open | **closed** (`e.reg.Close`) |
| admin identity `authority` | `browser_customers_billing_test.go:87` | 8 | open | **closed** (`stack.authority.Close`) |
| b, c, ox1 runtime/issuer/pool, all identity/worker/expiry | psSetupItemsOn | 8 each | closed (PR #56) | closed |
| observer connection | `harness_pool_capacity_test.go:104` | 1 | open | open |
| Node/Playwright/Next processes | — | 0 PG | — | — |

Request path per node (`internal/buyerhttp`): `scoped(h.pool)` = buyer runtime, `issuerPool` (domains/orderlink), checkout service pool (`p.pool`),
hosted starter (`h.payment`, `e.hosted`). collector-buyer.mjs clicks no pay button, so no job insert and no worker needed.

## Bound (MaxConns sum of every pool that can hold a backend)
Phase store-a (nodes 1–2): owner N + runtime 8 + hosted 8 + store A trio 24 + observer 1 = 41 + N. Platform trio is Reset and unused until node 3.
Phase platform-store (nodes 3–4, incl. concurrent-8): owner N + 8 + 8 + platform trio 24 + 1 = 41 + N. Store A trio is closed.
N = 4 → **45 ≤ 46** (limit 54, headroom 9). On hosts with NumCPU > 5 (not CI) the owner pool alone raises it; the log line shows the sum.
The bound is a MaxConns ceiling: concurrent-8 cannot exceed it, because a pool never holds more than MaxConns backends.

## Changes
- `browser_card_payments_test.go`: `cpbrReleaseAdminHalf` (control listener, worker stop, registrar, admin identity pool), `cpbrReleaseSetupPools` now also
  releases `e.pool`/`e.ingress`, `Reset()` of the platform trio, close of store A trio between node 2 and 3, `cpbrLogPools` →
  `HARNESS_CARD_POOLS open=<n> maxconns_sum=<m> observer=1 phase=<store-a|platform-store>` at the guard start and at the phase switch.
- `browser_refund_fulfilment_test.go`: `brfStack.authority` field (nil for other stacks). `browser_customers_billing_test.go`: `cbbrStartAdmin` sets it.
- No assertion, limit, margin or the harness files were changed; admin half, buyer half and concurrent-8 read are untouched.

## Verification
- `gofmt -l tests/foundation` clean; `go vet -tags browser ./tests/foundation/` exit 0 (compile only).

## Risks
- Stopping the worker and closing `e.pool`/`e.ingress` assumes the buyer half never enqueues or processes payment jobs (verified in collector-buyer.mjs: reads only).
- `pgxpool.Reset` closes idle backends and marks acquired ones for closure on release; the plat trio has no borrower at that point.
- The first-phase sum assumes the unused platform trio stays unused until node 3 (true in the test body).

## NOT_RUN
All behavioural verification: `scripts/dev/test-local.sh --browser-card-payments` and the CI browser job are NOT_RUN (CI only). No local test run. The
HARNESS_PG_CAPACITY sampled peak after the change is unmeasured; the evidence is the bound above, to be confirmed by the CI log (`HARNESS_CARD_POOLS`, `HARNESS_PG_CAPACITY`).
