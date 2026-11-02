# PR394 database-boundary RED test candidate

Status: **SOURCE/E1, runtime NOT_RUN; PR394 remains HOLD.** This is a tests-only author handoff, not a reproduced PostgreSQL result, HTTP exploit claim, product fix, independent acceptance or production approval.

- task_id: `70bf282d-2a00-4a82-8db7-2905b4a05f27` (root owns coord; author did not claim again).
- author: `/root/manual_discount_db_red_0917`; role: independent tests-only author, not product implementer or final acceptor.
- requested model/reasoning: `gpt-6-sol` / high; actual provider/model ID: **UNKNOWN**.
- date: 2026-11-02 UTC; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr394-offline-diagnostic`; branch: `codex/pr394-offline-diagnostic`.
- base_commit: `1f3bf3d66bc893afd43fa0031503dd2ca142f477`, initially clean. No rebase/base movement.
- write_paths: `tests/foundation/manual_discount_test.go`, this report only. Local commit allowed; push/CI dispatch not performed.
- author lock: acquired literal `pr394:author` before changes; release reported in handoff.
- test source SHA256: `2c44a9fd310a50db3338cff002d5f198ce961f6f210200a3074beab1c94911f2`.

## Authority and scope

Read original `AGENTS.md`, architecture sections 0/1/2/26/27, task DAG and invariants (especially I01/I08/I18). The operative contract is `contracts/cart-quote-v1.md:188-210` at base: both `inventory:reserve` and `catalog:read`, immutable authorization, exactly one matching frozen quote. Read migration `0220_manual_order_discount.sql`, `internal/merchanttools/discount.go`, its pipeline callers, actual runtime login setup, scoped transactions and quote/cart fixture definitions. The entire independent source report `output/integrator/tools/logs/ci-evidence-1238/review394-D-validation-0904.md` in the root checkout was read. Root already enumerated rejected 9/9, no truncation.

The normal HTTP route already checks permissions; its existing 201/403, monetary, CAS, replay, cross-store/tenant and refund assertions are preserved. These additions exercise independently granted SQL privileges through the actual `f.runtime` login. They do not allege an ordinary HTTP bypass. P2 post-wait race #505 and the rejected JSON-null product-defect claim are outside this card.

## Added cases and expected current RED

Registered under existing ordinary-CI root `TestMerchantToolsManualOrder/FEAT-MANUAL-DISCOUNT/runtime database boundary`; no new top-level root or shard filtering. Static shard lookup finds existing root in `g05`. There are **12 negative leaves**, seven INSERT and five binder. These are authored counts, not executed test counts.

| Leaf | Positive/setup control | Expected result | Current base source expected to make RED |
|---|---|---|---|
| INSERT: neither permission / unbound | valid `store:read` session, exact resolved scope, both permission denials checked, real same-owner public quote and zero authorization rows | SQLSTATE 42501 or PT403, zero rows | 0220:23 table INSERT grant and :31-37 policy contain no permission fence |
| INSERT: neither permission / preset quote | same, genuine existing FK quote supplied | same | same grant/policy also allow non-null quote_id |
| INSERT: missing inventory reserve / unbound | same scope; catalog allowed, reserve denied | same | same missing permission fence |
| INSERT: missing inventory reserve / preset quote | same; genuine quote FK | same | same missing permission/initial-binding fence |
| INSERT: missing catalog read / unbound | same scope; reserve allowed, catalog denied | same | same missing permission fence |
| INSERT: missing catalog read / preset quote | same; genuine quote FK | same | same missing permission/initial-binding fence |
| INSERT: both permissions / preset quote | both permissions explicitly proved; genuine quote FK | same; authorized merchants must use binder rather than prebind INSERT | table-wide INSERT/policy do not require quote_id IS NULL |
| binder: another merchant principal | A matching genuine CreateManualQuote binds on independent owner; A/B are distinct valid merchants; B reserve scope plus catalog check succeeds | PT403 or PT409; full original authorization and quote unchanged | 0220:70-77 does not compare authorization principal to resolved principal |
| binder: missing frozen adjustment | same-principal matching adjusted quote control binds; negative is a real public no-adjustment quote with active authorization | PT403 or PT409; unchanged | binder reads owner, never frozen adjustment |
| binder: different frozen name | same builder matching control binds; separately inserted consistent quote differs only in adjustment name from authorization | same | same missing frozen-adjustment equality |
| binder: different frozen amount | same builder matching control binds; separate valid discounted money is generated with 200 versus authorized 100 | same | same missing frozen-adjustment equality |
| binder: no-discount tombstone with adjusted quote | matching no-discount authorization/public quote binds; negative quote carries valid 100 adjustment against tombstone | same | binder never compares absence/presence of adjustment |

A real `POST /orders/manual` 201 / total 2400 is an additional positive control before all negatives. The runtime pool is explicitly checked for current_user=session_user, inherited commerce_runtime, no superuser/BYPASSRLS/commerce_auth. Each INSERT uses a new owner, new principal/session and no authorization. Each binder leaf has separate positive-control and negative owners; all start unbound. Callback-entry witnesses reject failures that occur before the target SQL boundary. Binder callers prove both permissions before executing the definer. Only expected denial SQLSTATEs are accepted; FK, nonexistent-quote, malformed-input and setup errors cannot pass as safe refusals.

## Fixture boundary

Owner/admin pool is used only by existing fixture identity setup, initial synthetic authorization INSERTs, standard constrained INSERTs of new synthetic quote rows, and readback. It never performs the attack. No ACL change, SET ROLE, row-security disabling, session_replication_role, trigger disabling, existing authorization UPDATE/DELETE, existing quote UPDATE/DELETE, or snapshot backfill was introduced.

For mismatch fixtures, a genuine same-owner public quote supplies all relational headers, cart/session, versions, times and line provenance. A NEW quote ID is mirrored in its snapshot. Existing `pricing.CalculateAdjusted` generates consistent line/tax/total money for the specified valid adjustment. A standard INSERT SELECT copies the original headers while all FK/checks/triggers remain enabled. Positive matching authorization controls use this same builder. The negative's exact adjustment/amount/ID/cart readback is checked before the binder; full authorization JSON and frozen snapshot bytes are checked afterward. These are controlled database consistency fixtures, **not evidence that HTTP emits a mismatched quote**. The pricing function is used as fixture construction, not a replacement for the existing independent money assertions.

## Actual commands and exits

All below were run in the target worktree unless specified. No local Go/PG/browser runtime was started.

| Command | Exit/result |
|---|---|
| `git status --short` and `git rev-parse HEAD` | 0, clean at exact base |
| targeted `rg`, `sed`, `cat` original-source reads | successful corrected reads 0; early discovery included missing guessed filenames/glob errors, corrected using `rg --files`; no missing read is treated as evidence |
| Python edit of the allowed test file | 0 |
| `gofmt -w tests/foundation/manual_discount_test.go` | 0 |
| `git diff --check` | 0 |
| Python static preservation check (git-show base, subtract only new imports/root call/new function, require exact original byte equality; check only allowed tracked diff; inspect shard-plan JSON) | 0: original code preserved, root in g05, 12 authored leaves |
| `gofmt -l tests/foundation/manual_discount_test.go` | 0, empty output |
| final `git diff --check` | 0 |
| Python bounded final static check: exact base/hash, no fixture UPDATE/DELETE/ACL/trigger bypass, actual runtime scope call, auth/snapshot invariance assertions, no active commit hooks | 0 |
| `git status --short --untracked-files=all` | 0, only the two allowed files |

One initial shared-canvas update without a Mermaid payload returned a tool validation error and wrote nothing; corrected update is reported in handoff. Humaux operations are tool receipts, not shell exit codes. Two bounded static self-check passes only; no failure sample was removed or oracle weakened.

## Required root verification and unresolved items

Root reviews this diff, then pushes only after its author/reviewer checks. A focused **CI-only** invocation through the existing isolated-PG script can use:

```sh
LC_FOUNDATION_PKGS=./tests/foundation LC_FOUNDATION_RUN='^TestMerchantToolsManualOrder$/^FEAT-MANUAL-DISCOUNT$/^runtime_database_boundary$' bash scripts/dev/test-local.sh
```

This command is a proposed CI command, **NOT_RUN** here. Ordinary CI already discovers all leaves through the existing root. Keep raw RED log at the tests-only commit, including exact checkout SHA, actual command/exit and each leaf/control; distinguish target assertion failure from setup failure. Then integrator repairs production logic on a later commit and root obtains GREEN plus required whole-PR gates and independent review. Do not delete these failures, change thresholds, or use unrelated existing CI green evidence.

**NOT_RUN:** Go compilation/type checking, Go tests, race/vet, real PostgreSQL, HTTP execution, browser, migration execution, GitHub push/dispatch, RED/GREEN, independent acceptance, SANDBOX/LIVE. **UNKNOWN:** actual runtime RED leaf count until CI, compiler result, deployed grants, ordinary HTTP exploitability, eventual repair shape. No processes/containers/ports/temporary fixtures were started by this author; source and report are the only durable outputs. P1 product issues remain unresolved pending RED and integrator repair; author static E1 is not independent E3/E4 acceptance.
