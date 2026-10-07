# W6-05B operations ledger: independent review (Opus)

- Reviewer: Claude Opus 5.5, read-only. I changed no code and made no commits. This file is my only write.
- Reviewed: `git diff r3/integration...HEAD`. HEAD is `d9380a49` (author commit `c97a2845` plus a merge of trunk `93233a00`).
- Hashes: `migrations/0159_operations_ledger.sql` sha256 `af54cb74…eea6`, `internal/integrations/core/dispatcher.go` `5294cd9a…2de9`, `internal/integrations/core/ledger.go` `c3bf9b83…f0c`.
- Checks I ran without a database, on HEAD `d9380a49`:
  - `go build ./...` exited 0.
  - `go vet` on core, httpapi, pagination, httperror, cmd/api and tests/foundation exited 0.
  - `gofmt -l` was clean.
  - `go test -race -run 'Ledger|Operation|Normalize|Dispatcher|Budget'` on internal/integrations/core, httpapi, pagination and httperror exited 0.
- NOT_RUN by me: the REAL_PG gate `--operations-queue`, the mutation reds, and the full foundation/G07 suite. CI has to run them again on the merged SHA. `green.log` and `red*.log` were produced before `c97a2845` and the trunk merge, and they are not tied to a SHA.

## Verdict: MERGE-AFTER-FIX

- P0: none.
- P1: one, ads-lane `cancel` of a spend-stopping `meta.ads.pause`. The fix is one line.
- The core I06/I07 design holds:
  - UNKNOWN is never retried.
  - A query job cannot dispatch an effect kind.
  - Cancel is fenced against the claim.
  - The retry registry is enforced in SQL.
  - `generation_floor` changes no fence.

## P1

### P1-1 Ledger `cancel` can neutralise an ads pause, including the AD7 budget auto-pause. That is real money.

**Evidence**
- `0159:53-57`: `ledger_capability` allows `cancel` for every READY row before the ads-lane check at `:60`. So `meta.ads.pause` is cancellable.
- `tests/foundation/operations_queue_test.go:369,392-395` asserts exactly that ("cancel of a READY ads operation still works" for `meta.ads.pause`).
- `0074_meta_ads.sql:1443,1470`: the advance sweeper plans `pause` when spend ≥ the approved `lifetime_budget_minor` (AD7), when the ad is DISAPPROVED, or when the draft has ended.
- `0074:1479-1480`: a pause that is not SUCCEEDED is re-planned only after 15 min (`pause_retry`, next seq, at most 50 seqs). Each new READY pause shows up in the ledger with `cancel` available again.

**Why it matters**
- Ads-lane ops wait in READY exactly when ads-worker is down or backlogged. That is also when the ledger shows them as "attention".
- A merchant who "cleans up stuck ops" cancels a READY pause, and the live ad keeps spending past its approved budget, or after End.
- Each cancel buys 15 minutes, and this can repeat for up to about 12.5 h.
- Cancelling a stop-spend op is not "local, never dispatched, safe". Its effect is that remote spending continues.
- The approved budget is an `ads:approve` control. The ledger bypasses it through a generic `integration:execute` screen.
- AGENTS.md treats ad spend as a red line.

**Fix**
- In `ledger_capability`'s cancel branch, refuse spend-stopping kinds. For example, before `IF o.state='READY'`:
  `IF o.action='meta.ads.pause' THEN RETURN 'operation_closed'; END IF;` (or a new code such as `cancel_not_supported`, added to `refusalCodes` and `httperror`).
- Flip the `meta.ads.pause` cancel case in the test at `operations_queue_test.go:392-395` to expect 409, and add a mutation red for it.
- Alternative: make the ads lane read-only, with no cancel at all.

## P2 (non-blocking; fix in follow-ups or the integrator's discretion)

1. **`query` has no lifetime cap.**
   - Evidence: `0159:202` sets `generation_floor := generation`, which gives a full `MaxGenerations` budget (10, `dispatcher.go:148`) on every query. The only brakes are the 60 s spacing and the "no live job" check (`0159:69-72`).
   - Why: the steady state is about 10 reconcile claims per ≥60 s per operation, forever. For Meta kinds this costs no HTTP calls, because reconcile is a constant UNKNOWN (`routes.go:447`, `send_dm.go:379`, `live_videos.go:86`). For ECPay it means up to 10 Query/V5 calls per click (`ecpayroute/route.go:275`).
   - The base contract's "bounded" now means "bounded per merchant click". That is not a blind retry: reconcile is read-only, so I06/I07 are intact.
   - Fix: cap query, for example `count(query_requested) ≤ N per 24 h` in `ledger_capability`, or grant a smaller budget per query (`floor := generation - (Max - k)`).
2. **Retry re-open (FAILED_FINAL→READY) does not check for a live job.**
   - Evidence: `0159:80-83` checks only the registry and the binding.
   - Why: a stale retryable job plus the new job means two jobs on a READY row. `claim_operation`'s row lock still allows exactly one dispatch (`0096:156`), so the only result is a wasted read on read-only kinds.
   - Fix: add the same live-job check as the READY branch. That keeps "one job per READY" local. The query→dispatch impossibility currently rests on a global argument (only `0159:240` writes READY, and only for the registry), not a local one.
3. **Re-open bypasses the plan-time Page-budget de-duplication.**
   - Evidence: `plan_meta_audience` and `plan_meta_live_videos` treat READY as "in flight" only when `x.job_id` (the original job) is live (`0113:650`, `0118:257`). A re-opened op runs on a new job, so a concurrent plan mints a second Graph read for the same video or Page.
   - This is harmless for data, because latest-wins on `requested_at = op.created_at` (`0113:731`, `0118:347`). But it is an extra call against the Page budget.
4. **The attention list never shows an expired DISPATCHING op, although `query` accepts one.** `0159:18,151` lists READY/UNKNOWN/ACK/FAILED_FINAL/BLOCKED/STALE only. An in-doubt op whose job died is reachable only by id.
5. **The contract says binding ids are never exposed, but the DTO exposes one.**
   - Evidence: `contracts/external-operation-v1.md:113` lists "binding ids" as never exposed, yet `0159:169` maps `meta.live_videos` to `object {kind:"binding", id:<binding_id>}`. The DTO tests forbid `o.Binding.ID` (`operations_queue_test.go:177`) but never cover live_videos.
   - Fix: amend the contract, or drop the mapping, and add the case to `TestOperationsQueueObjectMapping`.
6. **No gate checks the SQL DTO itself (mutation M4 stays green).**
   - The SQL definer is minimal today: exact `jsonb_build_object` keys (`0159:178-180,184`), and the object is built from internal UUIDs only.
   - The only gate is the Go struct filter, and `commerce_runtime` can call `read_operation_ledger` directly.
   - Fix: add one assertion inside `WithScope` that checks the exact key set of the raw jsonb, so M4 turns red.
7. **The contract wording over-credits idempotency keys.**
   - The dispatcher always sends `lc:<operation_id>` (`dispatcher.go:576`). But Meta adapters ignore it. Only ECPay has natural provider dedupe, through `MerchantTradeNo(operation_id)` (`ecpayroute/route.go:192`).
   - What actually guarantees safety is that READY was never dispatched (only `0159:240` writes READY) and that the registry is read-only. The key is not what protects it.
   - Fix: reword the "Retry registry" paragraph of the amendment to say so.
8. **Cancelling a READY `meta.private_reply` burns the one-reply-per-comment key.** `0128:376` (K3 F1) treats CANCELLED as "not provably unsent". After a ledger cancel, a manual reply answers `used`. This fails safe, but it is a UX dead end. Document it in the W6-U2 copy, or amend 0128 now that ledger CANCELLED is provably READY-only.
9. **The cancel-vs-claim race test can flake.** `operations_queue_flow_test.go:378` fails if 20 timed iterations never produce both winners. Assert "never both" instead, and log the distribution.
10. **Migration locking.** `0159:13` validates the CHECK under ACCESS EXCLUSIVE, and `:17` builds a non-concurrent index. Both block the dispatcher during the migration. Acceptable at current table size; note it for the deploy window. Deploy order (0159 before claims-worker and ads-worker) is mandatory: the new dispatcher's `SELECT` names `generation_floor` (`dispatcher.go:548`).
11. **The evidence does not satisfy the SHA rule.**
    - `green.log` has no SHA or hash, and predates `c97a2845` and the merge.
    - The red runs say "base 6277731e + unit diff".
    - Of the 12 mutations, 11 are red. M4 is green by design.
    - The integrator's CI run on the merged SHA is the acceptance evidence.

## Point-by-point verification

### 1. `generation_floor` dispatcher change

- **Budget counting.** `service.go:110` defines `budgetUsed = Generation - GenerationFloor`. It is used at all four budget sites: `dispatcher.go:259` (pre-claim cancel), `:293` (cleanup-only), `:390` (post-callback) and `:437` (ambiguous). The post-claim sites use the row that was re-read inside the claim transaction (`dispatcher.go:484`), so the floor value is current.
- **Cannot reopen a terminal op.**
  - The terminal checks run before any budget logic: `dispatcher.go:256` (`terminalOperationState`, `:636`) and `claim_operation`, which returns `terminal` (`0096:143`).
  - `query` requires UNKNOWN, ACKNOWLEDGED or DISPATCHING (`0159:65`) and changes no state (`0159:202`).
  - The only terminal re-open is retry FAILED_FINAL→READY (`0159:240`). The amendment authorises it, and it is limited in SQL to the two registry kinds (`0159:82`).
- **No hard cap.** See P2-1. It is not an automatic loop: every round needs a click with an Idempotency-Key, 60 s spacing, and no live job (the previous budget must already have run out).
- **Races with claim/lease.**
  - Query and retry change the floor only under the operation row lock `FOR UPDATE` (`0159:111`). That is the same lock order (binding SHARE, then operation UPDATE) used by `claim_operation` and `complete_operation` (`0096`).
  - Query refuses an active lease and any live job (`0159:66,69-70`). Retry refuses DISPATCHING, UNKNOWN and ACKNOWLEDGED (`0159:84-85`), and READY or FAILED_FINAL never carry a lease.
  - Generation is untouched by query and retry, so every `complete_operation` generation+token fence (`0096`) is unchanged. Cancel adds 1 to generation only on READY, which has no outstanding lease.
- **Other generation-based budgets.** `internal/payments/{query_worker,stripe_query,stripe_refund}.go` use `claim.Generation` directly. Those are payment lanes, which the ledger cannot touch: `ledger_open` and the read locator filter to the default and ads lanes (`0159:107-108,147,154`). Ads lanes never get a floor because query and retry are refused there. The media 4096 caps are on an excluded lane. `settle_cvs_attempt` keys on `result_code`, not generation. So nothing disagrees.
- **Legacy upgrade.** The column is `NOT NULL DEFAULT 0`, checked by `CHECK 0≤floor≤generation` (`0159:12-13`). Workers already hold table-level SELECT (`0096:92`). `UPDATE(generation_floor)` is granted only to the definer owner (`0159:14`), and runtime and worker UPDATE is refused, as tested in `operations_queue_test.go` (SQLFences). Adding `generation_floor:0` to the legacy snapshot is correct.

### 2. Query is reconcile-only

- The job kind is the ordinary `external_operation_v1`. There is no special job: being reconcile-only comes from `claim_operation`'s `mode := CASE WHEN state='READY' THEN 'dispatch' ELSE 'reconcile'` (`0096:156`), combined with the dispatcher calling `Dispatch` only when `claim.Mode=="dispatch"` (`dispatcher.go:345,367`).
- Query starts only from UNKNOWN, ACKNOWLEDGED or expired DISPATCHING. The only SQL that writes READY anywhere is `0159:240` (found with grep across all migrations), and it is limited to the read-only registry. So an effect kind can never become READY again, and a query job can never dispatch one.
- **Concurrency.** Two queries serialise on the operation `FOR UPDATE`. `ledger_capability` is evaluated in a later statement with a fresh READ COMMITTED snapshot (`ledger_auth` enforces READ COMMITTED, `0159:29`). So the loser sees the winner's committed job and answers `query_in_progress`, and its own job rolls back with the exception.
- The test runs 8 concurrent queries and expects 1 to succeed and 7 to fail with `query_in_progress` (`operations_queue_flow_test.go`, ConcurrentActions). Mutations M10 (live job ignored) and M11 (row lock removed) turn it red.
- The 60 s spacing reads the committed `query_requested` event (`0159:71-72`).
- The job is checked to be this transaction's own exact job (`xmin`, args, queue, priority, `unique_key`; `0159:118-120`). M8 is red.

### 3. Cancel covers READY only

- `0159:53-57` allows READY only. DISPATCHING, UNKNOWN and ACKNOWLEDGED answer `already_dispatched`; anything else answers `operation_closed`.
- `0159:220` moves the row to CANCELLED, adds 1 to generation and records `cancelled_by_merchant`.
- **Fencing.** The claim takes the same row lock. If the claim commits first, the state is DISPATCHING and generation has gone up, so cancel fails with CAS `operation_changed` or `already_dispatched`. If cancel commits first, the claim sees CANCELLED and answers `terminal` (`0096:143`), and the dispatcher returns nil (`dispatcher.go:276-280`).
- `TestOperationsQueueCancelRacesDispatchClaim` checks this over 20 iterations, and M11 turns it red (flake risk: P2-9).
- The domain hooks fire on cancel: `wipe_send_secret` and `clear_terminal_capi_ip` both include CANCELLED, and `settle_cvs_attempt` treats READY→CANCELLED as `not_sent`, which allows a new attempt. The exception is the ads pause (P1-1).

### 4. Retry

- UNKNOWN, ACKNOWLEDGED and expired DISPATCHING answer `reconcile_first`; an active lease answers `lease_active` (`0159:84-85`). M2 is red.
- The registry is enforced in SQL (`0159:82`). There is no Go copy. M6 is red.
- **READY re-queue cannot double-dispatch.** A READY row has never had a dispatch callback, because the claim moves READY→DISPATCHING atomically and the callback runs only after the claim commits. Two jobs can still produce only one dispatch claim. The re-queued job behaves exactly like the original job running late, and the route `Check` (I07) runs again at dispatch.
- The ads lane is excluded (`lane_unsupported`).
- The registry kinds are safe to re-open: no generation=1 assumption in load/finish (`0113:707-731`, `0118:310-350`), latest-wins upserts, and retention does not touch them. The same-key claim and the idempotency limits are covered in P2-7.

### 5. DTO

- The SQL is minimal (P2-6 concerns the gate, not the SQL).
- Scope:
  - The store comes from `WithScope(path store_id)` plus the token. `ledger_auth` (`0159:24-40`) runs `resolve_access` and then requires `app.tenant/store/principal` to equal the session.
  - Every lookup is explicitly filtered by tenant and store (`0159:107-111,146-154,185`).
  - A cross-store id returns 404. M5 is red.
- Permissions:
  - Reads need `integration:read`; actions need `integration:execute`. Both are checked in Go (`ledger.go:126,171,223,267`) and in SQL (`0159:143,105`).
  - Bundles: owner and admin hold both (`0019`, `0065`, `0089:110`); live_operator holds neither.

### 6. Ads lane

- `post_river/0015` is not in the diff.
- `lane_unsupported` comes from the one function used for both the DTO and the POST (`0159:60`), so the two always agree. Tested in `operations_queue_test.go:367-397`. M9 is red.
- The cancel allowance is the P1.

### 7. Definer hygiene and pins

- All seven functions are `SECURITY DEFINER SET search_path=pg_catalog`, owned by `commerce_integration_writer`, with `REVOKE ALL FROM PUBLIC`.
- The three private helpers are granted to no login. The four public functions are granted to `commerce_runtime` only, and the test checks the worker, buyer and runtime roles.
- All references are schema-qualified.
- T06 pin 89→96 is correct: exactly 7 new `integration.*` functions.
- R2 pin 81→82 is correct: one migration. Trunk's 0157 will need summing by the integrator.
- The legacy snapshot is accurate.

### 8. Tests

- The mutations are meaningful: each targets one rule, and 11 of 12 turn red (M4 is the documented exception).
- The concurrency cases are covered: 8 retries produce 1 job and 1 provider effect under one `lc:` key; 8 queries produce 1 job; 6 same-key replays get the same body; and cancel-vs-claim is swept.
- Gaps: `meta.ads.pause` cancel (P1-1), live_videos object mapping (P2-5), the SQL-level DTO (P2-6), and timing flake (P2-9).
