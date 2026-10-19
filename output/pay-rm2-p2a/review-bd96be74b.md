**Verdict: APPROVE**

Review of commit bd96be74b (PR #70, worktree .worktrees/pay-rm2-p2a, HEAD bd96be74b67eaa64db5f932ebf69a45d88e42ce7), checked against review-b9cfe42cd.md. This was a read-only review. I did not run any Go tests or browser tests, which is policy. Evidence level: E1 (static trace plus gofmt/vet). Runtime gates are NOT_RUN by me. PR checks `foundation` (run 38761930994) and `plan` (run 38761932865) were **pending** when I wrote this. The approval covers the diff. Merge still needs those checks to go green.

## Prior findings

| Prior | Status | Evidence |
|---|---|---|
| P1: whole-body equality in the rotate replay | **Closed, minimal** | `tests/admin/auth-real.spec.ts:431-433`. The test now asserts status 422 and compares with `request_id` blanked. This is the same pattern as :396-398, :526-528 and :550-552. |
| P2: decrypt claim in the restart test | **Closed** | `tests/foundation/account_process_test.go:217-218`. The comment now says the check decrypts with the fixture keys, and that the API process only parses the keyring at startup and reads `key_id`. |
| P2: lost guarantees not named | **Closed** | `output/pay-rm2-p2a/DELIVERY.md:549-564`. The commit lists the feature-only guarantees that were lost. Keyring reload is marked partial, with a pointer to `TestStripeSP15Process` (stripe_process_test.go:43, which exists). Restart replay of a successful write is marked **NOT_COVERED**, with a follow-up. |
| P3: durability checked only by counts | **Closed** | `account_process_test.go:186,193,214`. The test takes a full-row `maHistorySnapshot` before the first process starts and compares it after the refused create, and again after the refused replay and rotate in the second process. `m.facts` is kept, so the global `river.river_job` count is still checked. |

## Remaining equality scan: tests/admin/auth-real.spec.ts
I checked every `toEqual`/`toStrictEqual` call (no `toStrictEqual` exists). Two error bodies are compared at 4 sites: :398, :433, :528 and :552. All 4 blank `request_id` and assert status first. Every other `toEqual` compares a success body or a fixed literal: :131, :189, :217, :235 (store-create replay of a 200 receipt, which carries no envelope), :252, :289, :304, :325, :348, :442 and :614. **None remain.**

## Assertions checked against the retired writes
- Account create and rotate: `internal/integrations/accounts/service.go:70-76` returns `ErrInvalid` unconditionally, which maps to 422. So :363-382, :394-409, :419-438 (rotate on a synthetic missing ID gives 422, not 404) and :475 (originControl) all match. The 61-attempt loop at :588-603 matches because `internal/httpapi/accounts.go:111` limits at 60 per window and the loop breaks on 429.
- Method PUT: `internal/payments/methods.go:80-88` returns 422 when the input is valid with `enabled:false` (:518, :578-582, :586-593), and 409 when it is valid with `enabled:true` (:594-602). The method body passes `validMethodInput`: connection_id is a valid UUID and binding_version is 1.
- Inspect on a missing method: methods.go:118-123 returns `ErrNotFound`, which maps to 404 (:545-552).
- List and GET of accounts return empty or 404, because no account is ever created (:348, :442, :409, :603).
No assertion in the file depends on a write that is now retired.

## maHistorySnapshot coverage
Pre-removal `accounts.Create` and `Rotate` (`git show main:internal/integrations/accounts/service.go`) wrote to these tables:
- `integration.merchant_accounts` (INSERT/UPDATE)
- `integration.account_credentials` (INSERT)
- `integration.bindings` through `core.RegisterBinding` (`internal/integrations/core/service.go:148`)
- `ops.command_results` and `ops.audit_events` through `command.Run`/`Audit` (`internal/command/command.go:88,119,138`)

The snapshot (`tests/foundation/payuni_history_helpers_test.go:83-97`) covers all of these tables plus `integration.operations`, scoped to the tenant. River jobs are covered by the count in `facts[6]`. **Coverage is complete** for the rows the refused writes could touch.

## DELIVERY.md accuracy
The new section at :549-564 matches the code. One wording note: "GET and list are identical" is true for GET in both processes, but list is only compared in the second process (:204-207). This is accurate enough and needs no change.

## Nits (non-blocking)
- `account_process_test.go:218`: "Process-side keyring open/refuse is TestStripeSP15Process" is slightly stronger than DELIVERY.md:561, which says "partially covered; decrypt in-process". Optionally add "(partial)".
- `DELIVERY.md:564`: "addressed in the next commit" means bd96be74b itself. Optionally name the SHA.

## Commands run
| Command | Exit |
|---|---|
| `git show bd96be74b` | 0 |
| `gofmt -l tests/foundation/account_process_test.go` (no output) | 0 |
| `go vet ./tests/foundation/` | 0 |
| `go vet -tags browser ./tests/foundation/` | 0 |
| `git show main:internal/integrations/accounts/service.go` (pre-removal writes) | 0 |
| `gh pr checks 70` (foundation and plan pending) | 0 (pending reported) |
| grep/sed static reads (spec, helpers, settings.go, methods.go, accounts.go, command.go, core/service.go) | 0 |

Go tests and browser tests: NOT_RUN (forbidden locally). CI on bd96be74b: pending.
