# LC-U3 A16R backend handoff

task_id: daceb64b-36de-499e-8567-b812902f001d; own canvas: daceb64b-sub-backend.
base_commit: 8a0ec7520b6b7b40ca992520c79ea3f17a2f82f7
checkpoint: 8065b007 (root integrated as bee91fa7); final delta: 6250cb5a53144006428af241cdcd09df04a1fb6c. Both use Implemented-by: Codex-3. NO push/merge.
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u3-create-order-drawer/.worktrees/lc-u3-recovery-seam; branch unit/lc-u3-recovery-seam.
Role: parent assigned backend commerce_worker equivalent; no recursive delegation. Actual model: Codex/GPT-6 system identity, exact service model identifier NOT_EXPOSED. Reasoning: high per task card. All 11 paths below authorized by root; httperror exact attempt_closed entry and typed normalization test explicitly approved. No migrations, grants, schema, lockfiles, go.mod or apps changes.

## Implementation and root causes

POST /v1/admin/stores/{store_id}/orders/for-buyer/recover accepts the identical frozen A16 body and SAME Idempotency-Key. It returns only {state:not_created} after a durable seal, or {state:created,order_id} after authenticating actual durable checkout receipt+order. It returns no buyer/customer data or links. Normalized body/kind conflicts reject409; late oldkey409attempt_closed alone does not authorize a client replacement attempt. Lost recovery responses replay original body/key.

Generic422 and absence of upper merchant receipts never prove checkout absence. Whole-attempt store/key serialization plus durable original canonical body binding protect replay. The independent review demonstrated outer connection loss lets original Go continue, so the final causal fence uses exact checkout.begin tenant/store/private-owner/step-key lock. Recovery's SAME merchant transaction holds it through closure+upper receipt COMMIT. Authenticated LookupBeginLocked reads durable facts without self-deadlocking on that held lock. Original BeginFenced retains its checkout transaction lock and checks durable ops closure using one scoped merchant connection before all money writes. Lost original outer lock cannot bypass seal; loss of seal-owning transaction rolls both receipts back. Fresh inventory:reserve/catalog:read/orders:read and session/store authority checks after wait/lookup and before final result prevent stale-auth response/order-ID leakage. Original private capability is never returned; absent attempts have no buyer links exposing their owner. Missing checkout receipt with actual owner order fails closed. Unbound pre-amendment partial order stays UNKNOWN.

Placement now clones and canonicalizes delivery/address/items before binding, then runs unchanged business validation. Recovery does not repeat static business validation, permitting recipient/payment422 bodies to close authoritatively without correcting/replacing the old body. This also fixes a causally observed shared HomeAddress input race. Successful normalized receipt hashes remain unchanged. Pool MaxConns<2 rejected without connecting; half-pool admission leaves room for guarded pipeline inner work. No order, stock, successful receipt or DM erased/changed by recovery.

REJECTED: direct checkout_runtime writes to ops were not authorized despite checkout_writer definer-owner grants. Actual runtime NOINHERIT ACL failed in real PG; preserved red-static-green-fence.log, no ACL/schema changes. Prior outer-lock-only PG11 green is superseded by lost-lock P1 and retained separately.

## Actual commands and exit codes

- `bash scripts/dev/test-focused.sh '^TestLiveConsoleRecovery'` => exit1, initial route404 PASS0 FAIL3 SKIP0: red.log.
- `GOTOOLCHAIN=go1.27.2 go test -count=1 ./internal/httpapi -run '^TestForBuyerRecoveryRouter$'` => exit1 route404: red-router.log.
- `GOTOOLCHAIN=go1.27.2 go test -race -count=1 ./internal/httpapi -run '^TestForBuyerRecoveryAttemptClosedCode$'` => exit1 coded409 sanitized as internal before exact registry fix: red-code.log.
- `GOTOOLCHAIN=go1.27.2 go test -race -count=1 ./internal/merchanttools -run '^TestManualOrdersAttemptPoolCapacity$'` => exit1 MaxConns1 accepted: red-pool.log. Final package race covers unchanged test green.
- `bash scripts/dev/test-focused.sh '^TestLiveConsoleRecoveryLostOuterLockFencesCheckout$'` => exit1 PASS0 FAIL1 SKIP0 original created after seal: red-lost-lock.log.
- `bash scripts/dev/test-focused.sh '^TestLiveConsoleRecoveryFreshAuthorityAfterCheckoutWait$'` => exit1 PASS0 FAIL1 SKIP0, created grant/store/session revoke leaks stale200: red-revoke.log. Final expands absent cases without weakening assertions.
- `bash scripts/dev/test-focused.sh '^TestLiveConsoleRecovery(LostOuterLockFencesCheckout|FreshAuthorityAfterCheckoutWait|Static422CanCloseFrozenBody)$'` => direct ops ACL rejected implementation exit1 PASS0 FAIL3 (red-static-green-fence.log); next no-SQL compile failure undefinedtoken exit1 PASS0 FAIL0 (r2.log); own queued runner interrupted143 before any test/runtime (r3.log empty, NOT_RUN).
- `bash scripts/dev/test-focused.sh '^TestLiveConsoleRecovery(LostOuterLockFencesCheckout|FreshAuthorityAfterCheckoutWait|Static422CanCloseFrozenBody|TwoConnectionPool|LostSealConnectionRollsBack)$'` on frozen8065b007 => exit1 PASS3 FAIL2 SKIP0, meaningful static422 recipient/payment red + existing address-pointer race; fence/auth/rollback green: red-static-green-fence-r4.log.
- `bash scripts/dev/test-focused.sh '^Test(LiveConsoleRecovery|LiveConsoleOrderForBuyer(Resume|ConcurrentStaff)|MerchantToolsManualOrder)$|^TestLiveConsoleRecovery'` final source => exit0 PASS16 FAIL0 SKIP0,128.846s: green-pg-final.log. Same pre-existing gate earlier PASS8 FAIL3 registry bug (green-pg-r1.log), then PASS11 FAIL0 outer-only (green-pg-before-fence.log, superseded).
- `GOTOOLCHAIN=go1.27.2 go test -race -count=1 ./internal/httpapi ./internal/merchanttools ./internal/checkout ./internal/httperror` => exit0 all4 packages: green-unit-final.log. Includes router/body rejection + safe typed attempt_closed + lazy constructor no-connection regression.
- `GOTOOLCHAIN=go1.27.2 go vet ./internal/httpapi ./internal/merchanttools ./internal/checkout ./internal/httperror` => exit0: vet-final.log.
- `bash scripts/dev/check-headers.sh` => exit0: headers-final.log.
- `GOTOOLCHAIN=go1.27.2 go test -run '^$' ./tests/foundation` => exit0 compile-only oncheckpoint tests (not runtime): compile-tests.log.
- `git diff --check` and `git diff --cached --check` => exit0 before both commits. gofmt applied only owned Go changes. Final git status no tracked source changes.
- `node scripts/dev/check-backend-coverage.mjs` isolatedWT => exit1 new HTTP testfile unclassified, root-owned integration gap. Root exact registry coverage updated; author did not edit shared classification.
- `bash scripts/dev/check-gates.sh` twice interrupted own processes143 while contractdrift compiling (before-code-registry.log/check-gates.log). No complete PASS. Root owns complete integrated gates; latest contract literal full endpoint added after final frozen runtime ended (documentation only).

## Final exact REAL_PG names

- TestLiveConsoleOrderForBuyerConcurrentStaff
- TestLiveConsoleOrderForBuyerResume
- TestLiveConsoleRecoveryFreshAuthorityAfterCheckoutWait
- TestLiveConsoleRecoveryTwoConnectionPool
- TestLiveConsoleRecoveryLostSealConnectionRollsBack
- TestLiveConsoleRecoveryLostOuterLockFencesCheckout
- TestLiveConsoleRecoveryStatic422CanCloseFrozenBody
- TestLiveConsoleRecoverySealLateAndLostReply
- TestLiveConsoleRecoveryCheckoutReceiptWithoutMerchantReceipt
- TestLiveConsoleRecoveryAuthority
- TestLiveConsoleRecoveryDynamic422AndKindConflict
- TestLiveConsoleRecoveryPlacementWinsConcurrentSeal
- TestLiveConsoleRecoverySealWinsConcurrentPlacement
- TestLiveConsoleRecoveryMissingCheckoutReceiptFailsClosed
- TestLiveConsoleRecoveryLegacyUnboundPartialStaysUnknown
- TestMerchantToolsManualOrder

## Evidence and limitations

E3 author-tested disposable REAL_PG actual checkout_runtime + race; current source/log hashes SHA256.json. This is not sole-author merge acceptance or E4. Root independent source review, integrated focused rerun and browser required. Full G07 REAL_PG mandatory CI because checkout runtime changed; NOT_RUN locally under heavy-mode rule. Browser, full integrated gates, CI, SANDBOX/LIVE/provider/production: NOT_RUN by this backend worker. Fleet upgrade/drain older manual/for-buyer writers required before enabling recovery. Legacy partial unbound original body remains UNKNOWN by contract. No known final author P0/P1 in tested scope; independent review still controls acceptance.

All logs preserved under canonical /Volumes/data/live_commerce_architecture_v1/output/lc-u3-create-order-drawer/r1-money-backend. Own disposable containers/runners terminated by runner cleanup; no foreign process terminated. Original two queued own runners stopped only known own wait PID with no fixture created. Files changed:

- contracts/live-console-v1.md
- internal/checkout/checkout.go
- internal/checkout/receipt_lookup.go
- internal/httpapi/merchanttools.go
- internal/httpapi/order_recovery_test.go
- internal/httperror/error.go
- internal/merchanttools/manual.go
- internal/merchanttools/order_attempt.go
- internal/merchanttools/order_attempt_test.go
- internal/merchanttools/order_for_buyer.go
- tests/foundation/live_console_order_recovery_test.go

Humaux fix: bce58c3e-3a39-42c5-bd22-51c1bb5448f2, supersedes runtime-pending checkpoint; own canvas updated. code_index submitted all10changedGo files, Recover code_memory_link succeeded. Final canonical copy byte/hash verified; only own untracked receipt directory removed after all runners exited. Worktree clean and owned PG container filter empty.
