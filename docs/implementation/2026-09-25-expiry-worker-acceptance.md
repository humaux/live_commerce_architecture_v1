# Checkout expiry worker — bounded runtime acceptance

2026-09-25. **PASS_BOUNDED_LOCAL_RUNTIME — not production deployment.** Go production implementation is
`42edf58` (author `38fa261`), post-River SQL is `25c44b2`. Final source/test tree
is `5cea8b91963e6c519d6a8e4621fe544389e63992`: independent expiry test corrections
are integrated and the final root full regression exited 0. No customer
production, provider network transaction or deployment is covered by this record.

## Delivered code boundary

- Separate default-off `cmd/expiry-worker`, ordinary worker-only pool, fixed
  `checkout_expiry_v1` queue and bounded boolean startup audit; no payment keyring,
  provider configuration, HTTP listener or arbitrary queue selection.
- Existing checkout `InsertTx`, `ExpiryWorker` and `expire_held` reused. Migration
  validates/backfills old active jobs atomically with a deferred AFTER INSERT
  router. It matches immutable job ID/initial generation, not payment-advanced
  current generation. No new stock writer or uncertain-payment release rule.
- Two actual commands share `internal/jobqueue.Run`; original payment startup
  watchdog, lifetime context and 15s/5s stop behavior retained. No new dependency.
- River's global leader JobScheduler may promote scheduled jobs in other queues
  to available; this is not execution by the expiry consumer. Isolation tests
  must prove no foreign-queue claims or business effects, not claim global
  maintenance never touches another row.

Contract: [EW01–EW05](../../contracts/checkout-expiry-runtime-v1.md).
Upgrade/rollback/stop lines: [runbook](expiry-worker-runtime.md).
Dependency and call mapping: [dependencies](dependencies.md).

## Root evidence already observed

| Gate | Result | `/Volumes/data/output/` log |
| --- | --- | --- |
| Existing checkout subset at `25c44b2` | PASS, actual PG/River, 12.613s | `expiry-root-checkout-1.log` |
| First EW subset at `ffd6914` | FAIL, 55.476s; admission/deferred and real CLI stop/crash recovery passed, four new fixture/assertion failures retained | `expiry-root-focused-1.log` |
| Existing payment worker after shared extraction | PASS, 8 top-level entries including child helper, actual PG/River, 45.947s | `expiry-root-payment-worker-1.log` |
| Five package race unit command, default package parallelism | FAIL, checkout process exceeded harness timeout and was quit at 90.094s; no test assertion or stack identified a product cause | `expiry-root-unit-1.log` |
| Same five packages with `-p 1 -race -count=1 -timeout=30s -v` | PASS, all test cases retained | `expiry-root-unit-serial-1.log` |
| Existing payment browser gate | PASS, production Next build + 11 actual Next/Go/PG/native local-mock-PSP cases, 7.38s / package 9.082s | `expiry-root-browser-payment-1.log` |
| Existing B order/history browser gate | PASS, 23 cases, six buyers/seven orders, exact durable facts, 14.56s / package 15.944s | `expiry-root-browser-order-1.log` |
| Frontend Node / strict TypeScript | PASS, 55 cases / exit 0 | `expiry-root-node-1.log`, `expiry-root-typecheck-1.log` |
| Final expiry-focused gate at `fefb1e4` | PASS, 9 top-level real PG/River/race tests, 53.271s | `expiry-root-focused-2.log` |
| First full regression at `13f56ff` | FAIL, 287.449s; one existing payment crash-test transient-state wait, query/report/capture 17/1/1 | `expiry-root-full-1.log` |
| Second full regression at `fefb1e4` | PASS, 444 top-level / 0 FAIL / 0 SKIP, PG18/race/vet, foundation 257.982s | `expiry-root-full-2.log` |
| Payment worker with corrected rescue wait at `5cea8b9` | PASS, 8 top-level entries including child helper, real PG/River/race, 44.108s | `expiry-root-payment-worker-2.log` |
| Final full regression at `5cea8b9` | PASS, 444 top-level / 0 FAIL / 0 SKIP, actual PG18/race/vet, foundation 274.541s, exit 0 | `expiry-root-full-3.log` |

Browser artifacts remain in `output/playwright/buyer-payment-314492980` and
`output/playwright/buyer-order-837357204`. Root inspected desktop payment and
mobile order captures. Seven overwritten tracked reference captures were first
byte-matched against these retained fresh artifacts, then only those generated
copies restored to the accepted baseline. This is regression checking, not a new
whole-surface redesign or physical-phone acceptance.

## Test findings and execution limits

1. Old-producer tests reused existing cart/destination version zero. The real
   public business methods correctly rejected those stale versions; tests must
   obtain current versions and create a new quote/destination, not bypass guards.
2. `river.JobSnooze` decrements the attempt counter (first attempt 1 → 0), so an
   early scheduled job needs `attempted_at` and a DB-deadline scheduling witness.
   Requiring attempt > 0 would report a false failure. Completed jobs still need
   positive attempts; replay must increase the same durable job's attempt.
3. Original payment race had an already-expired order, with no lock overlap.
   Acceptance requires both real callers waiting through the same held order
   lock, including soft-blocker chains, and correlated payment return/state.
4. Global River scheduling changed a due payment job from scheduled to available
   without claiming it. Exact full-row nonmutation is checked from an available
   control-row baseline; it cannot be generalized to all scheduled rows.
5. Failed migration needs a second valid active default job as a witness that an
   invalid row did not cause partial movement. Only checking the invalid row is
   insufficient rollback evidence.
6. Independent author and root default-parallel package runs stalled on this
   host. Serial package execution passed; no product root cause was established.
   The full runner now uses `-p 1`, preserving `-race`, test cases, in-test
   concurrency and deadlines. Package envelope grows 300s → 360s solely for the
   added isolated migration and second crash-recovery suite. Original failures
   remain evidence; default-parallel runs are not counted as passing.
7. The first full run found an older payment crash gate waiting for both the
   durable JobRescuer error and current `retryable` state. That state can already
   have advanced between 100ms polls. The resulting extra 40s wait allowed the
   fixture's one-second retry policy to make 17 queries, while report/capture
   each remained one. `5cea8b9` waits for the durable marker on the same job and
   stops the real client immediately after capture. It retains exact one-query,
   one-report/one-capture, generation, stock and work assertions. This is a test
   causality correction, not a payment production-code fix. Full run 2 passed
   before this correction; that does not erase the earlier intermittent failure.

Final expiry tests additionally witness both real payment and expiry sessions
blocked on the same order through `pg_blocking_pids`, including transitive
blockers. The expired branch requires `command.ErrConflict`, not just any error;
the valid-payment branch requires its exact durable attempt. Crash recovery uses
the same job ID and River's own rescue marker, with no post-crash state rewrite.

## Independent ownership

- Root/integrator owns frozen contract `60db865`, SQL `25c44b2`, existing test
  adaptations, admission tests, shared-runner integration/acceptance,
  documentation and final reruns. Shared-runner implementation was authored by
  the Go worker below, not by the integrator.
- Go author `expiry_runtime_impl`, `gpt-6-sol/high`, base `60db865`, isolated
  `/Volumes/data/live-commerce-expiry-go`, branch `codex/expiry-runtime-go`;
  commit `38fa261`, nine assigned files. Focused unit/race/vet passed; author did
  not claim real-PG acceptance.
- Independent PG author `expiry_runtime_pg_gate`, `gpt-6-sol/high`, isolated
  `/Volumes/data/live-commerce-expiry-pg`, branch `codex/expiry-runtime-pg`;
  test-only ownership `tests/foundation/expiry_runtime_test.go`. Initial draft
  `94bd557`, causal correction `dc36fc0` and strict business-error assertion
  `5c62414`, integrated as `ffd6914`, `13f56ff`, `fefb1e4`. Independent focused
  gate passed 53.851s; root repeated the final stricter tree in 53.271s.
- Read-only reviewer `hosted_contract_security_review` keeps its inherited
  existing session settings; SQL/runtime static review found no P0/P1. Exact ACL
  P2 closed by `e87bd6c`. Final expiry source/test review
  `0563015a-b497-4c58-be39-3850ffc709fe` has no open P0/P1/P2 in this bounded
  change. Independent payment rescue-wait adjudication is
  `cff2684c-5f75-46d3-a6c2-67f95ce0ebc5`. Final source/test/log review
  `d6a80595-844a-4dc5-8e04-c27c99f80860` independently verified `5cea8b9`,
  444/0/0, exit 0 and the final log hash, with no open P0/P1/P2. Final payment
  correction review is `98c52acb-b582-4e80-8b00-66037d3296ce`.

## Evidence hashes (SHA-256)

| Log | Hash |
| --- | --- |
| payment browser | `dad5da39793219ee90edd9dfbf9571a4911a737a90c94d0ba56538ecbea1a349` |
| order browser | `0b66e9c907973f249501aef1e4ef9488d5d1f2c2f813f9d53f1a29b9fc12a197` |
| Node | `9cd5da0cf9bb2c87674d2f8ce2ac06d104d4e6841ff8f49dfe69f842abf91fa2` |
| strict TS | `de46600b5fb7fd9cd4206a8374bb2562031f87e6f5f259353f2740f3def3cd6f` |
| serial unit/race | `9fa2144893443bf97ece55fb5551102306d914c72a2239c33c4f335ad090573f` |
| payment worker | `1ebefaf76b78f0181cac961c3d97f6a1b4c955edfe39fad2f04f00b6450d4d9a` |
| final focused expiry | `0a2f4be0a96ca75d5902d1db76998a1f0242539738d16af32438bc1be3b01be5` |
| full 1 (failed, retained) | `022d54e2ad7bb7c0f53e7a35bff199d84e0b3c3eb1551d9357e7a6384cf5a151` |
| full 2 | `f9689acd0cb065de050ece242bddb94cf4bc0ead74c18bdbc27cb477a03e8966` |
| corrected payment worker | `6767ae7a69b5a32b732947acd4098f1c897b41c568e97aaefebd501a1a5b7195` |
| final full 3 | `c08e8002080b6e64501443ba8399e91451f4de6c5d640ec585d242f226f6fa60` |

## Traceability

Final expiry and payment runtime test files were re-indexed after `5cea8b9`
(2 files, 95 entities, 0 rejected). The payment crash test is linked to its
independent causal review above; expiry construction/start and admission tests
are linked to frozen decision `90ac58cc-5c84-4ded-a96a-648ddf4da675`. Graph
indexing is maintenance provenance, not a replacement for the executable gates.

Root read back exit 0 from both final commands, counted the finished log and
confirmed no containers remained under the test fixture label. Logs, isolated
author worktrees and browser artifacts are retained; no customer service was
stopped. The architecture packet checker and local documentation links are
separate structural checks, not SaaS production acceptance.
Final structural checks passed: 6 documentation files / 80 existing local links,
`git diff --check`, packet structure, and an explicit JSON assertion that only
T11 owns this expiry acceptance while its overall status stays `IN_PROGRESS`.

## Retained release gates

River defaults remain one-hour crash-rescue age and 30-second scanning. Aging an
owned test lease does not prove real wall-clock expiry/recovery SLA. Terminal
failed-job reconciliation, capacity/alerts, real provider qualification and
notification protocol, refunds, trusted pickup/carrier mapping, deployment
DNS/TLS/rollback and complete SaaS acceptance remain open. T11/global goal remain
in progress; do not release uncertain payment stock merely because time elapsed.
