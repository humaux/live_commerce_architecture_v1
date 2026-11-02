# PR394 unfinished manual-discount error contracts

Status: **E1 / STATIC_ONLY / structure ready. Runtime RED is NOT_RUN.**

- task_id: `94e28b6b-f3ed-4bc5-9372-ef7167a491f7`; sub-element: `94e28b6b-sub-1`.
- base_commit: `42548b76e1367c3b7474f4ee1dfd133761b58146` (clean on entry).
- branch: `codex/pr394-offline-diagnostic`.
- worktree: `.worktrees/codex-pr394-offline-diagnostic`.
- role: tests-only author; runtime does not expose an authoritative actual model/reasoning identifier: **UNKNOWN**.
- exclusive changed paths: `tests/foundation/manual_discount_test.go`, this report.
- Parent owns the claim and author lock; this author neither separately claimed nor delegated.

## Authority and implementation

Read `AGENTS.md`, architecture sections 0/1/2/26/27, task DAG and P0 invariants I01/I02/I03/I08/I11/I18/I21. Root owns the task-DAG scheduling. The specific error oracles come from the base commit's `contracts/storefront-v2.md:649,651` and `contracts/cart-quote-v1.md:192`. Source inspection: `internal/merchanttools/manual.go` pipeline and classifier, `discount.go`, migration 0220 authorization, plus existing real-PG HTTP harness and buyer capability registration. Humaux task decision `ccd20326-1fe1-4e8f-9349-ef6535384b27` points to these roots; it is not runtime evidence.

Two additive `t.Run` leaves hang from existing `mdManualContract`, already selected by `TestMerchantToolsManualOrder/FEAT-MANUAL-DISCOUNT`. Each owns a separate `mtOrderEnv`. Both principals are created through `e.member` with `inventory:reserve` and `catalog:read`; changing only `mtAdmin.token` retains the same handler, issuer and deployment secret. B's real options GET must return 200.

Both leaves first submit quantity 2, discount 2501 through real merchant HTTP. The fixture SKU is 1250, so the required response is 422 `discount_exceeds_subtotal`. Owner-pool **read-only** witnesses require exactly one A authorization with the independently encoded normalized request hash and NULL quote, one same-owner cart receipt, and quantity 2. There must be no quote, manual receipt, order, reservation, order audit or held/allocated inventory.

| Leaf | Refused resume | Required response | Base source predicts (not observed) |
|---|---|---|---|
| partial authorization principal conflict | B, same store/key/body, valid permissions | 409 `conflict` | SQL PT409 principal conflict falls through `classifyPipeline` to 503 `retry_later` |
| partial changed cart and discount idempotency conflict | A, same key, quantity 3 + discount 100 | 409 `idempotency_conflict` | `SetCart` receipt conflicts before authorization hash check; classifier emits 409 `conflict` |

Before/after byte equality covers full tuples from buyer owners/sessions, authorization, buyer and merchant receipts, carts/lines, quotes, reservations/lines, balances, orders and audit. The table identifiers are a closed literal list, tenant/store values are bound parameters, and tuple contents are never printed. This also guards reuse of the same synthetic owner and cart receipt.

Each leaf then replays A's original body and requires the same 422 and unchanged state. A fresh key with quantity 3/discount 100 must return 201 with total 3650 through the same caller/handler. **The exact error status/code assertion runs last**, after these controls, with milestone logs showing which assertions were reached. Setup/state failures still stop the affected leaf; no later assertion is claimed to have run. No blanket reinterpretation of `command.ErrConflict`, production seam, SQL change, fixture error injection, permission revocation, gate change or deletion was made.

## Actual static commands and exit codes

Commands ran in the worktree above. No runtime test command was invoked.

| Command | Exit | Result |
|---|---:|---|
| `git status --short && git rev-parse HEAD` | 0 | clean; expected base |
| `gofmt -w tests/foundation/manual_discount_test.go` | 0 | formatted; Go parser accepts syntax |
| `git diff --check` | 0 | no whitespace errors |
| first inline Python reverse-byte verifier | 1 | checker regex required comma/count; Git emitted `@@ -23,0 +24 @@ import (`. Preserved failure: `AssertionError` at checker line 12. No test oracle changed. |
| corrected inline Python reverse-byte verifier below | 0 | all 1258 old lines recovered byte-for-byte; exactly 117 added lines, zero deletions; both new leaves and positive-control order present |
| `git diff --numstat` | 0 | `117 0 tests/foundation/manual_discount_test.go` before report addition |
| `gofmt -l tests/foundation/manual_discount_test.go` | 0 | empty output |

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

Actual verifier output:

```text
STATIC_ONLY old_bytes_exact=true old_lines=1258 inserted_lines=117
base_test_sha256=82fc3e623a43f865605c88c09fe1c31dcca5e0de4750993e559e481fe48aa256
current_test_sha256=9a3f390e827e37574af0074a286916b57387fe8684384595a001c11449b15d61
Runtime Go/PG/HTTP/browser=NOT_RUN; no typecheck/compilation claimed
```

## Acceptance boundary and handoff

NOT_RUN: Go compilation/typecheck, `go test`, real PG/HTTP execution, browser, Foundation/Gates CI, actual RED, provider sandbox/live, mutation gate, independent acceptance. No dependency install, push, dispatch, runtime server, container or background job was started. Existing 20-case authorization matrix, fixtures, thresholds, old oracles and all other old lines remain byte-identical after stripping additions. The initial static checker failure is retained above.

Unresolved P1 candidates remain product defects until current-SHA CI establishes behavior and a later product-only fix receives independent acceptance. Root must inspect the full diff, control push/CI, verify both new leaves reach the controls and expected precise error assertions, retain actual failure logs, then commission the bounded product correction. Existing cart/service/allocation/policy 409 `conflict` semantics remain mandatory. Source predictions are not actual runtime failures and this document does not claim E3/E4 or integration completion.
