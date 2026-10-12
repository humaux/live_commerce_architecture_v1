<!-- Purpose: W3-U3 exact BFF revert and conditional handoff to the canonical Go scope fix.
Depends on: integrator LCN03 ruling, rejected candidate58510545, preserved label/P2 source and SCOPE-404 merge.
Used by: integrator and this thread's quiet merge follow-up; no current browser READY claim. -->
# W3-U3 — WAITING_SCOPE404

- Revert/source: **e4e48ec581c8be7309521f20dc4497bee9ca08cc**, branch `unit/w3-u3-comment-label-print`.
- Before revert:3952ed32. Exact target:58510545f021d40dcc145c2f3531363e5aa47a81.
- No push/deploy; no Go/SQL/DTO edits, no duplicate backend repair.

## Decision and preserved work

K3 implementation review PASS does not approve the layer choice. LCN03 requires Go's shared authority to classify
mid-request loss consistently across domains. The BFF re-proof covers only inbox and can turn a genuine403 into404
when its second view lags; it also changes BuyerPanel's immediate403 vs local404 semantics. Therefore58510545 is reverted.

`git revert --no-commit 58510545`, followed by a co-authored commit, reversed exactly3 files:
`apps/admin/lib/auth.ts`, `apps/admin/app/api/stores/[store]/[...resource]/route.ts`, `tests/admin/inbox-bff.test.ts`.
Their bytes match58510545's parent and729afff9. Print/label components, print CSS, label specs, local denied-busy P2,
independently observed ledger P2 and the complete original live-console spec are unchanged from3952ed32.

`inbox-bff-test-design.patch` is byte-exact `git show 58510545 -- tests/admin/inbox-bff.test.ts`:
it preserves all14 new cases and their network-edge controls. They are retired as the rejected BFF contract,
not suppressed with skip/xfail or silently weakened. Original active BFF cases restore the baseline.

## Test ideas to retain at the correct seam

| Original insight | Canonical follow-up |
|---|---|
| Grant disappears between admission and private read | Deterministic REAL_PG Go race: store no longer visible =>404 |
| Genuine permission loss vs scope loss | Store still visible but permission missing =>403; no stale BFF opinion |
| Same identity/scope, no private read replay | Pin server-resolved tenant/store/principal and one read/command; do not change bearer or replay |
| Unknown/failed/malformed extra proof must not soften denial | No BFF proof remains; Go's re-resolution failure/remaining mismatch fails closed |
| Healthy reads and writes unchanged | Keep broad-domain read semantics, command keys/receipts and existing privacy fences |
| Timing/cancellation resource bound | Preserve as design evidence; no6s re-proof is now added by BFF |

SCOPE-404 implementation belongs to `unit/scope-revoke-404` (Kimi, reviewerQwen/K3). W3-U3 does not edit its code/tests.
Reviewer `w3_merge_audit` (gpt-6.1-sol/medium, E1 source-only) verified the exact three-file reverse, unchanged accepted
UI/specs and archive parity; it did not independently run gates. Archive SHA is in `parity.log`.

## Actual current checks (source e4e48ec5)

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` |0|1376 executions/1376 PASS/0 FAIL, `node.log` |
| `pnpm --filter admin exec tsc --noEmit` |0|`tsc.log` |
| `bash scripts/dev/check-gates.sh` |0|82 documented modes/1256 Go inventory/header ratchet, `gates.log` |
| Exact original-source/spec comparisons |0|`parity.log` |

The14-case count reduction is the exact explicit revert of the rejected BFF contract, not removal of unrelated tests.
No browser/PG mode was started here. Old full-mode gates on3952ed32 are retained but are **not acceptance of this source**.
Integrator has accepted W3-U3's other UI portions; this does not substitute for the new two-run condition.

## Conditional next step / NOT_RUN

At this check, fetched `origin/r3/integration` is9b738e0a (#29), and SCOPE-404 is **not yet confirmed merged**.
After the integrator's confirmed merge and verified ancestor SHA:

1. Check existing coordination ownership/runs, acquire the own-worktree lock; never start a duplicate gate.
2. Merge origin/r3/integration, retain the single mode registry and accepted print/label/P2 changes. Never bring58510545 back.
3. Freeze one source SHA and run `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console`
   **twice, strictly serial**, with original test actions, wait conditions, assertions and timeouts untouched.
4. Record both actual exits/counts/evidence; update DELIVERY and Humaux/canvas, commit only, report SHA for the integrator's PR.

Current two-run browser acceptance, current-head required CI/K3, global visual/click sweep and physical printing/LIVE:
**NOT_RUN**. No current full-mode READY is claimed.

Quiet thread heartbeat **w3-u3-scope-404** is ACTIVE every15min. It checks merge evidence without status spam;
after merge it resumes this exact worktree/task, observes execution policy, and pauses after the two-run handoff.
First create was rejected for missing thread destination; corrected create succeeded, not two automations.
The integrator can also notify this thread directly. No automation grants push/deploy, Go/DTO edits or test relaxation.

Humaux task: **dbd03f8b-e0f2-4f2e-8ad9-a6f7e03efd0c**. The label/P2 acceptance and all previous red/green evidence remain preserved.
