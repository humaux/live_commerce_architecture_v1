# PR394 unfinished manual-discount error contracts

Status: **E1 / STATIC_ONLY / first harness correction (1/2). Target product RED is NOT_RUN.**

- current task_id: `5a778f90-bd8e-443b-91fa-1498434dd622`; own canvas: `5a778-sub-1`; assigned role/route: test_worker / Sol high (runtime actual identifier remains UNKNOWN).
- current base_commit: `7591a8abd73db3066d8f1f3617c89a749c8ef9ce` (clean on entry); only the two paths below are authorized.
- The original author card/command record below is retained as history. The current correction changes only the added helper and this report.

- original author task_id: `94e28b6b-f3ed-4bc5-9372-ef7167a491f7`; sub-element: `94e28b6b-sub-1`.
- original preserved-test base_commit: `42548b76e1367c3b7474f4ee1dfd133761b58146` (clean on entry).
- branch: `codex/pr394-offline-diagnostic`.
- worktree: `.worktrees/codex-pr394-offline-diagnostic`.
- role: tests-only author; runtime does not expose an authoritative actual model/reasoning identifier: **UNKNOWN**.
- exclusive changed paths: `tests/foundation/manual_discount_test.go`, this report.
- Parent owns the claim and author lock; this author neither separately claimed nor delegated.

## Authority and implementation

Read `AGENTS.md`, architecture sections 0/1/2/26/27, task DAG and P0 invariants I01/I02/I03/I08/I11/I18/I21. Root owns the task-DAG scheduling. The specific error oracles come from the base commit's `contracts/storefront-v2.md:649,651` and `contracts/cart-quote-v1.md:192`. Source inspection: `internal/merchanttools/manual.go` pipeline and classifier, `discount.go`, migration 0220 authorization, plus existing real-PG HTTP harness and buyer capability registration. Humaux task decision `ccd20326-1fe1-4e8f-9349-ef6535384b27` points to these roots; it is not runtime evidence.

Two additive `t.Run` leaves hang from existing `mdManualContract`, already selected by `TestMerchantToolsManualOrder/FEAT-MANUAL-DISCOUNT`. Each owns a separate `mtOrderEnv`. Both principals are created through `e.member` with `inventory:reserve` and `catalog:read`; changing only `mtAdmin.token` retains the same handler, issuer and deployment secret. B's real options GET must return 200.

Both leaves first submit quantity 2, discount 2501 through real merchant HTTP. The fixture SKU is 1250, so the required response is 422 `discount_exceeds_subtotal`. Owner-pool **read-only** witnesses require exactly one A authorization with the independently encoded normalized request hash and NULL quote, one same-owner cart receipt, and quantity 2. There must be no quote/order/reservation for the new manual owner, no manual receipt/order audit, and no change to the pre-existing quote/order/hold/stock/ledger fixture tuples. Held and allocated quantities must equal their pre-POST baseline; the fixture already owns a legitimate quote and hold.

| Leaf | Refused resume | Required response | Base source predicts (not observed) |
|---|---|---|---|
| partial authorization principal conflict | B, same store/key/body, valid permissions | 409 `conflict` | SQL PT409 principal conflict falls through `classifyPipeline` to 503 `retry_later` |
| partial changed cart and discount idempotency conflict | A, same key, quantity 3 + discount 100 | 409 `idempotency_conflict` | `SetCart` receipt conflicts before authorization hash check; classifier emits 409 `conflict` |

Before/after byte equality covers full tuples from buyer owners/sessions, authorization, buyer and merchant receipts, carts/lines, quotes, reservations/lines, balances, orders and audit. The table identifiers are a closed literal list, tenant/store values are bound parameters, and tuple contents are never printed. This also guards reuse of the same synthetic owner and cart receipt.

Case 1 additionally calls the same authorization SQL using B through the real `commerce_runtime` pool and `platform.WithScope`, with the saved owner/hash. It checks the callback scope tuple and both permissions before setting the target-entry witness. Existing `sqlState` uses `errors.As` to extract an actual `*pgconn.PgError`; the witness requires exactly PT409. Returning that error makes `WithScope` roll back; a subsequent rollback must return `pgx.ErrTxClosed`, and all witnessed state must remain byte-identical. This source-classification control does not replace the merchant HTTP assertion and is itself runtime NOT_RUN. No owner-role impersonation or permission revocation is used.

Each leaf then replays A's original body and requires the same 422 and unchanged state. A fresh key with quantity 3/discount 100 must return 201 with total 3650 through the same caller/handler. **The exact error status/code assertion runs last**, after these controls, with milestone logs showing which assertions were reached. Setup/state failures still stop the affected leaf; no later assertion is claimed to have run. No blanket reinterpretation of `command.ErrConflict`, production seam, SQL change, fixture error injection, permission revocation, gate change or deletion was made.

## First harness correction 1/2 — current card

The prior controlled CI is retained: Gates `41479280477`, job `125395614907`, execution `5bbbe1094b30af938c34656dcc90d8e22e91a9e0`, with raw log `output/integrator/tools/logs/ci-evidence-1238/394-error-red-1630/job-125395614907.raw.log` SHA256 `03dce8e9fa3fffc0fbf576ab5e66e746b55b11864a4e3ab461f478dad093fcbc`. Both new leaves stopped at the old helper line 143 (`partial rejection wrote storefront.quotes`). They passed earlier options/422/auth/cart checks by code order, but **did not reach** runtime B/PT409, refused resume, replay, fresh-key 201 or the final 409 assertions. That failed run is setup RED, not target product RED.

Root accepted SOURCE diagnosis `output/integrator/tools/logs/ci-evidence-1238/394-error-setup-source-1639.md` SHA256 `adea4a85f4afbe555dcbf7c118a7188a1b45dd95bea33e974ddb671ef1c2692d`. Authority is the pinned fixture chain: `mtOrderEnv → tcvNew → psSetup → psSetupItemsOn → b.prepare/CreateQuote`, then `b.begin`; the checkout fixture intentionally retains the original quote/order/hold. The new test's whole-store absolute-zero precondition was wrong. No fixture is cleared or modified to make it empty.

The authorized correction reuses one small reader with two **closed literal** table sets. Immediately before the first tested POST it captures full tenant/store tuples for quotes, orders, reservations, reservation lines, balances and ledger. After the initial real 422 those six tables must be byte-identical, and the new manual owner must separately have zero quotes/orders (`owner_id`) and zero reservations (`buyer_owner_id`). Original manual receipt/audit zero and A principal/hash/NULL quote/cart assertions remain. Held/allocated equal their captured baseline, while full balance tuples also protect on-hand/version. Added milestone logs expose reaching these checks without printing tuples, tokens or bodies.

The same reader's other literal set is the unchanged later 13-table set. All subsequent SQL/HTTP refusal, rollback, replay, fresh-key and exact final error assertions remain byte-identical to 7591, as do body construction/normalization/hash/key code and everything outside the added helper. No product SQL, fixture, gate, source-hash registry, generator or legacy bundle/reader case is edited. Root must independently review and refresh its existing source-hash ownership evidence before controlled CI; this card does not alter that registry.

Actual current-card commands: inline Python helper-only refactor exit 0; `gofmt -w tests/foundation/manual_discount_test.go` exit 0; `git diff --check` exit 0; inline Python inverse/oracle-preservation verification exit 0; `shasum -a 256 tests/foundation/manual_discount_test.go` exit 0. No runtime or typecheck was invoked. Verifier output:

```text
STATIC_ONLY original_lines=1258 inverse_exact=true inserted_lines=153
outside_helper_exact=true postinitial_tail_exact=true body_hash_key_exact=true baseline_order=true
base_test_sha256=82fc3e623a43f865605c88c09fe1c31dcca5e0de4750993e559e481fe48aa256
current_test_sha256=289348f1b1b042717cc2ac33120434b1955c80eb5849c2d19fcacfbb05163da9
new_runtime=NOT_RUN; typecheck=NOT_RUN; CI_target_RED=NOT_RUN
```

Relative to current base 7591, test diff is +35/-23 within the added helper (net +12); relative to original 42548, all 1258 original lines remain exact and only 153 lines are added. Runtime gate remains: first prove both new baseline/owner checks and all subsequent applicable controls actually execute, then obtain the exact two target HTTP/code failures on unchanged product code. Any earlier failure remains harness/environment RED. No product fix or integration acceptance is claimed.

## Actual static commands and exit codes

Commands ran in the worktree above. No runtime test command was invoked.

| Command | Exit | Result |
|---|---:|---|
| `git status --short && git rev-parse HEAD` | 0 | clean; expected base |
| `gofmt -w tests/foundation/manual_discount_test.go` | 0 | formatted; Go parser accepts syntax |
| `git diff --check` | 0 | no whitespace errors |
| first inline Python reverse-byte verifier | 1 | checker regex required comma/count; Git emitted `@@ -23,0 +24 @@ import (`. Preserved failure: `AssertionError` at checker line 12. No test oracle changed. |
| corrected inline Python reverse-byte verifier below | 0 | all 1258 old lines recovered byte-for-byte; exactly 141 added lines, zero deletions; both new leaves and positive-control order present |
| `git diff --numstat` | 0 | `117 0 tests/foundation/manual_discount_test.go` at initial checkpoint; final base diff is `141 0` |
| `gofmt -l tests/foundation/manual_discount_test.go` | 0 | empty output |
| `git add -- tests/foundation/manual_discount_test.go output/feat-manual-discount/ERROR-CONTRACT-RED.md` | 0 | initial local staging |
| `git commit -m 'test(manual-discount): pin unfinished pipeline error contracts'` | 0 | initial static commit `247d08d56ce2693810d5651d2b89f4a58c97b261` |
| `git diff --check HEAD^ HEAD` | 0 | initial committed diff; clean status verified |
| `gofmt -w tests/foundation/manual_discount_test.go` after runtime-SQL witness addition | 0 | additive 24-line control; no runtime invoked |
| corrected reverse-byte verifier after runtime-SQL witness addition | 0 | final base diff: 141 added lines; all 1258 original lines exact |

The first checker used `r'@@ -(\d+),(\d+) \+(\d+),(\d+) @@'`; the corrected checker accepts omitted single-line counts. Reproduce the corrected check before committing (or compare this commit's diff against the same base):

```sh
python3 - <<'PY'
import hashlib, pathlib, re, subprocess
base = '42548b76e1367c3b7474f4ee1dfd133761b58146'
path = 'tests/foundation/manual_discount_test.go'
original = subprocess.check_output(['git','show',base+':'+path])
current = pathlib.Path(path).read_bytes()
diff = subprocess.check_output(['git','diff','--unified=0',base,'--',path]).decode()
lines = current.splitlines(keepends=True)
insertions = []
for line in diff.splitlines():
    if line.startswith('@@'):
        match = re.match(r'@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@', line)
        assert match, line
        oldstart, oldcount, start, count = match.groups()
        assert int(oldcount or 1) == 0, 'old content changed'
        insertions.append((int(start),int(count or 1)))
    elif line.startswith('-') and not line.startswith('---'):
        raise AssertionError('old line deleted')
for start,count in reversed(insertions):
    del lines[start-1:start-1+count]
assert b''.join(lines) == original, 'reverse byte comparison failed'
text = current.decode()
assert text.count('t.Run("partial authorization principal conflict"') == 1
assert text.count('t.Run("partial changed cart and discount idempotency conflict"') == 1
assert text.index('fresh-key same-handler authority') < text.index('if code != 409 || out["code"] != wantCode')
assert 'mdPartialErrorContract(t, false)' in text and 'mdPartialErrorContract(t, true)' in text
print('STATIC_ONLY old_bytes_exact=true old_lines='+str(len(original.splitlines()))+' inserted_lines='+str(sum(n for _,n in insertions)))
print('base_test_sha256='+hashlib.sha256(original).hexdigest())
print('current_test_sha256='+hashlib.sha256(current).hexdigest())
print('Runtime Go/PG/HTTP/browser=NOT_RUN; no typecheck/compilation claimed')
PY
```

Historical author-card verifier output (retained; current output is above):

```text
STATIC_ONLY old_bytes_exact=true old_lines=1258 inserted_lines=141
base_test_sha256=82fc3e623a43f865605c88c09fe1c31dcca5e0de4750993e559e481fe48aa256
current_test_sha256=ac91916e36668cc79d723d219511fad5cbd46a9d62f7a676183dff63fbc972b0
Runtime Go/PG/HTTP/browser=NOT_RUN; no typecheck/compilation claimed
```

## Acceptance boundary and handoff

NOT_RUN for this correction: Go compilation/typecheck, `go test`, new real PG/HTTP execution, browser, new Foundation/Gates CI, target product RED, provider sandbox/live, mutation gate, independent acceptance. The prior setup-failing CI is preserved above. No dependency install, push, dispatch, runtime server, container or background job was started. Existing 20-case authorization matrix, fixtures, thresholds, old oracles and all other old lines remain byte-identical after stripping additions. The initial static checker failure is retained above.

Unresolved P1 candidates remain product defects until current-SHA CI establishes behavior and a later product-only fix receives independent acceptance. Root must inspect the full diff, control push/CI, verify both new leaves reach the controls and expected precise error assertions, retain actual failure logs, then commission the bounded product correction. Existing cart/service/allocation/policy 409 `conflict` semantics remain mandatory. Source predictions are not actual runtime failures and this document does not claim E3/E4 or integration completion.
