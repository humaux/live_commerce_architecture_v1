# Payment worker — bounded runtime acceptance

2026-09-25. **PASS_BOUNDED_REAL_PG_SIGNED_MOCK** on source/test tree
`e6f052fc317b4206403329ed53275071d1fd7a2e`. Not provider SANDBOX/LIVE or production
deployment acceptance. T11 and the overall SaaS delivery remain in progress.

## Delivered boundary

- Separate, default-disabled `cmd/payment-worker`; strict worker-only database
  authority, shared account-key parser, fixed execution-profile queue, bounded
  startup and shutdown. No HTTP listener or production MOCK transport option.
- Query and reconcile jobs use `payment_mock_v1`, `payment_sandbox_v1` or
  `payment_live_v1`, derived from the immutable attempt. Existing default expiry
  and external-operation jobs are not consumed by this process.
- Checksummed post-River migration validates and moves only eligible active
  payment jobs. Deferred commit-time routing supports old producers inserting a
  job before its attempt; invalid linkage rolls back the transaction. Terminal
  history and unrelated jobs remain unchanged. Startup audits do not repair data.
- Existing query, signed observation and capture/order/stock transitions are
  reused. Process liveness, a provider return or successful query is never Paid.
- Fixed `payment_worker_ready` carries no fields/secrets and follows successful
  startup/watchdog disarm. It proves only local startup, not provider availability.

Contract: [PW01–PW05](../../contracts/payment-worker-runtime-v1.md).
Operations, authority, upgrade/rollback and recovery limits:
[runbook](payment-worker-runtime.md). Dependency graph: [dependencies.md](dependencies.md).

## Root-executed gates

All logs below are retained under `/Volumes/data/output/`.

| Gate / command | Observed result | Log |
| --- | --- | --- |
| `bash scripts/dev/test-local.sh` | Final `e6f052f`: 433 top-level PASS, 0 fail/skip; PG18 foundation 215.448s; `-race` and subsequent `go vet ./...`, exit 0 | `payment-worker-root-full-3.log` |
| `bash scripts/dev/test-local.sh --payment-worker` | `7152e23`: 8 top-level PASS, 43.831s, exit 0; final full3 reruns these with the ready-marker fix | `payment-worker-root-worker-2.log` |
| `bash scripts/dev/test-local.sh --browser-payment` | `7152e23`: production Next build and 11 actual Next → Go → PG → native mock-PSP browser checks, 7.60s / package 9.098s, exit 0 | `payment-worker-root-browser-1.log` |
| `node --test apps/storefront/tests/*.test.mjs` | `e6f052f`: 55 PASS, 0 fail/skip, exit 0 | `payment-worker-root-node-1.log` |
| `pnpm run typecheck:storefront` | `e6f052f`: strict TypeScript, exit 0 | `payment-worker-root-typecheck-1.log` |

The eight worker top-level entries include one child-process helper; seven are
substantive parent gates. The helper also runs as the real killed child. Do not
interpret the count as eight independent business features. Browser evidence is
`output/playwright/buyer-payment-1888143259`. After that browser run, only worker
startup logging and its process test changed; browser/BFF/API/SQL code was unchanged.
This is functional regression, not a new whole-surface visual or physical-phone audit.

PW coverage includes fresh/upgrade/repeated/checksum/unknown-version migration;
running/orphan/wrong-hash/unique-key/wrong-queue rollback; terminal preservation;
old producer late commit and default-notified reconcile consumption by polling;
eight malformed deferred-commit cases; role/router/drift admission failures;
two-tenant signed queries through actual River to one capture/stock commitment;
replayed reconcile attempts after restart; exact real expiry/external job rows
unchanged; mixed-profile zero claims/observations/facts; real binary ready →
SIGTERM → exit 0 with no pool connections; actual SIGKILL and River lease rescue.

## Failures retained and resolved

1. `payment-worker-root-full-1.log` failed the exact integration-function ACL
   registry after three deliberate functions were added. `c18fe54` updates the
   exact six-to-nine signature expectations, not a loose count; `b3d0dd1` adds
   explicit hosted-role denial. The private helpers remain inaccessible.
2. `payment-worker-root-worker-1.log` failed crash capture after 45s. PG showed
   River had rescued the same job but scheduled its next retry two minutes later.
   The final test sets only MOCK `QueryWorkerOptions.RetryDelay=1s`; it ages the
   owned crashed lease/attempt timestamp, then lets real River rescue and schedule
   the same row. No owner rewrite of post-rescue state/due time or duplicate job.
   This does **not** prove real wall-clock recovery SLO. Production retains the
   documented one-hour rescue age, 30s scan and two-minute query retry defaults.
3. Startup previously lacked a bounded synchronous River `Start`; `2649a5d`
   adds the disarmed/joined watchdog. A further test race used queue creation as
   readiness before `Start` returned; `fb0e021`/`e6f052f` use the fixed log and
   drain child stderr before exactly one process Wait.

A successful query returns River `JobSnooze`, which decrements the persisted job
attempt count. Crash proof therefore uses the same durable job ID, rescue marker,
advanced operation generation, signed query, one observation and capture/work;
requiring a final River attempt of two would be an incorrect assertion.

## Independent ownership and review

- Runtime author: `payment_runtime_author`, `gpt-6-sol/high`, isolated
  `/Volumes/data/live-commerce-worktrees/payment-worker-runtime-20260925`, branch
  `codex/payment-worker-runtime-20260925`; base `1c951f5`, final `9c9168b`.
  Queue contract was revised to `4a76c8b` before integration. Root owns SQL,
  shared modules, integration and final documentation.
- Independent PG author: `payment_runtime_pg_gate`, `gpt-6-sol/high`, isolated
  `/Volumes/data/worktrees/payment-runtime-pg-gate-20260925`, branch
  `codex/payment-runtime-pg-gate-20260925`; test base `f9c6b4c`, commits `b11a5bf`,
  `951dd26`, `a6e563c`, `b398f4c`. Write scope: the runtime and migration test files.
  Root additionally owns admission and deferred-routing tests and reran all gates.
- Read-only reviewer `hosted_contract_security_review` retained its existing
  inherited session configuration (no model override in this continuation).
  SQL/profile review `782c9306-446b-46c6-a6b4-18e3dc99e7d0`; startup P1 closure
  `98d20fa2-d6e4-44d7-8eef-7c40dd767131`; exact ACL review
  `340dc38d-a9cd-4536-b082-63be8669eb0f`; final source/test causal review
  `01972c3e-9f4a-4826-af77-234a6182c610` at `e6f052f`: no remaining P0/P1/P2
  in this bounded change. Final execution results above are root-observed.

## Evidence hashes (SHA-256)

| Log | Hash |
| --- | --- |
| full3 | `f17747319875246010bb324e652308528b59a9e57ab364215dfc87fd780edcdc` |
| worker2 | `25416670132419ff51d56fca73cfa5a3fa107a88e0c6a3a6a295c6297518f3d8` |
| browser1 | `5ae15e09cc983f4d707b6e8f68ea6e5ba84c26ed66aad9404f00b9fdc9e9065b` |
| Node1 | `17e5def4df9d31a5e43f7ab4ea5f25268ee161074c586dab1f750e2250d6a629` |
| typecheck1 | `de46600b5fb7fd9cd4206a8374bb2562031f87e6f5f259353f2740f3def3cd6f` |

## Retained gates

No customer production/live-broadcast change, provider network transaction,
merchant enablement or real payment. SANDBOX CLI signal tests use an empty isolated
database, not a sandbox provider call. Notification HTTP/ACK remains unconfirmed;
local form parsing is not the official notification transport contract. Trusted
account qualification, real-provider acceptance, expiry-only runtime, refunds,
DNS/TLS/deployment rollback, performance and operational alerting remain open.
Task-owned test containers/processes are cleaned by their fixtures; evidence and
author worktrees are retained. This record does not promote global G03/G05 or T11.
